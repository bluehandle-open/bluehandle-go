// Package client manages the connection to the phone, over TCP (Wi-Fi) or
// RFCOMM (Bluetooth). It merges WifiSocket.java, BluetoothSocket.java and
// ClientThread.java into one implementation.
package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluehandle-open/bluehandle-go/internal/bluetooth"
	"github.com/bluehandle-open/bluehandle-go/internal/consts"
	"github.com/bluehandle-open/bluehandle-go/internal/input"
	"github.com/bluehandle-open/bluehandle-go/internal/keymap"
	"github.com/bluehandle-open/bluehandle-go/internal/protocol"
)

var (
	ErrPhoneClosed      = errors.New("client: phone closed the connection")
	ErrNotConnected     = errors.New("client: not connected")
	ErrAlreadyConnected = errors.New("client: already connected or connecting")
)

const (
	// DefaultReadTimeout is how long we wait for any frame before declaring
	// the phone gone. The phone sends a heartbeat every second over Wi-Fi.
	DefaultReadTimeout = 5 * time.Second
	// dialTimeout covers two SYN retransmissions (Windows resends after 3 s
	// and 9 s). A phone whose Wi-Fi is dozing often drops the first SYN.
	dialTimeout = 12 * time.Second
	// btDialTimeout leaves room for paging the phone and the SDP lookup.
	btDialTimeout    = 20 * time.Second
	handshakeTimeout = 10 * time.Second
	writeTimeout     = 3 * time.Second
)

// Client is safe for concurrent use. One Client handles one connection at a
// time; after Close or a remote disconnect it can Connect again.
type Client struct {
	kb input.Keyboard
	ev Events

	// ReadTimeout may be changed before Connect. Zero disables the timeout.
	ReadTimeout time.Duration

	mu    sync.Mutex
	conn  net.Conn
	state State
	done  chan struct{}
	cause error // why the last connection ended; nil after Close

	paused    atomic.Bool
	lastFrame atomic.Int64 // UnixNano of the last frame received
}

// New returns a client that replays key events on kb and reports to ev.
func New(kb input.Keyboard, ev Events) *Client {
	if ev == nil {
		ev = NopEvents{}
	}
	done := make(chan struct{})
	close(done)
	return &Client{kb: kb, ev: ev, ReadTimeout: DefaultReadTimeout, done: done}
}

// Connect dials the phone. addr is an IP or host, with an optional port
// (consts.Port otherwise), or a Bluetooth address "bt:AA:BB:CC:DD:EE:FF".
// It returns once the connection is up and starts the read loop in the
// background.
func (c *Client) Connect(ctx context.Context, addr string) error {
	c.mu.Lock()
	if c.state != Disconnected {
		c.mu.Unlock()
		return ErrAlreadyConnected
	}
	c.state = Connecting
	c.mu.Unlock()
	c.ev.OnState(Connecting, addr)

	conn, r, err := dial(ctx, addr)
	if err != nil {
		c.mu.Lock()
		c.state = Disconnected
		c.mu.Unlock()
		c.ev.OnState(Disconnected, err.Error())
		return err
	}

	done := make(chan struct{})
	c.mu.Lock()
	c.conn, c.state, c.done, c.cause = conn, Connected, done, nil
	c.mu.Unlock()
	c.lastFrame.Store(time.Now().UnixNano())
	c.ev.OnState(Connected, conn.RemoteAddr().String())
	go c.readLoop(conn, r, done)
	return nil
}

// dial opens the transport. Over Bluetooth the phone first sends a
// handshake line, which is consumed here so a wrong service fails early.
func dial(ctx context.Context, addr string) (net.Conn, *bufio.Reader, error) {
	if !bluetooth.IsAddr(addr) {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(addr, strconv.Itoa(consts.Port))
		}
		d := net.Dialer{Timeout: dialTimeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, nil, err
		}
		return conn, bufio.NewReader(conn), nil
	}

	ctx, cancel := context.WithTimeout(ctx, btDialTimeout)
	defer cancel()
	conn, err := bluetooth.Dial(ctx, addr)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = os.ErrDeadlineExceeded
		}
		var op *net.OpError
		if !errors.As(err, &op) {
			err = &net.OpError{Op: "dial", Net: "bluetooth", Err: err}
		}
		return nil, nil, err
	}
	r := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	if err := protocol.ReadHandshake(r); err != nil {
		conn.Close()
		return nil, nil, &net.OpError{Op: "dial", Net: "bluetooth", Err: err}
	}
	return conn, r, nil
}

// Cause reports why the most recent connection ended: ErrPhoneClosed when
// the phone said goodbye, a read error when the link was lost, or nil when
// the connection was closed with Close or is still up.
func (c *Client) Cause() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cause
}

// LastFrameAt is when the last frame (including heartbeats) arrived on the
// current or most recent connection.
func (c *Client) LastFrameAt() time.Time {
	return time.Unix(0, c.lastFrame.Load())
}

// SetPaused stops (or resumes) replaying the phone's input. The connection
// stays up and observers still see every input. Pausing lifts any key that
// is still held so nothing stays stuck down.
func (c *Client) SetPaused(p bool) error {
	if c.paused.Swap(p) == p || !p {
		return nil
	}
	if r, ok := c.kb.(input.Releaser); ok {
		return r.ReleaseAll()
	}
	return nil
}

// Paused reports whether input replay is paused.
func (c *Client) Paused() bool { return c.paused.Load() }

// State reports the current connection state.
func (c *Client) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Done returns a channel that is closed when the current connection's read
// loop has exited. Before the first Connect it is already closed.
func (c *Client) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.done
}

// SendKeySetting pushes one of the three key tables to the phone.
func (c *Client) SendKeySetting(typ byte) error {
	keys, ok := keymap.ByType(typ)
	if !ok {
		return fmt.Errorf("client: unknown key set 0x%02x", typ)
	}
	return c.Send(protocol.NewKeySetting(typ, keys))
}

// Send frames and writes m to the phone.
func (c *Client) Send(m protocol.Message) error {
	frame, err := protocol.EncodeToPhone(m)
	if err != nil {
		return err
	}
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return ErrNotConnected
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err = conn.Write(frame)
	return err
}

// Close tells the phone we are leaving, then closes the socket. It does not
// wait for the read loop; use Done for that. Closing a disconnected client
// is a no-op.
func (c *Client) Close() error {
	c.mu.Lock()
	conn := c.conn
	if conn == nil {
		c.mu.Unlock()
		return nil
	}
	c.conn = nil
	c.state = Disconnected
	c.cause = nil
	c.mu.Unlock()

	frame, _ := protocol.EncodeToPhone(protocol.NewFinish())
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, werr := conn.Write(frame)
	cerr := conn.Close()
	c.ev.OnState(Disconnected, "closed by user")
	return errors.Join(werr, cerr)
}

func (c *Client) readLoop(conn net.Conn, r *bufio.Reader, done chan struct{}) {
	defer close(done)
	if r, ok := c.kb.(input.Releaser); ok {
		defer func() {
			if err := r.ReleaseAll(); err != nil {
				c.ev.OnError(err)
			}
		}()
	}
	for {
		if c.ReadTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
		}
		m, err := protocol.ReadMessage(r)
		if err != nil {
			c.teardown(conn, err)
			return
		}
		c.lastFrame.Store(time.Now().UnixNano())
		switch m.Type {
		case consts.TypeKey:
			ev, err := protocol.ParseKeyEvent(m)
			if err != nil {
				c.ev.OnError(err)
				continue
			}
			c.handle(Input{Kind: InputKey, Key: ev}, func() error { return input.Apply(c.kb, ev) })
		case consts.TypeText:
			ev, err := protocol.ParseText(m)
			if err != nil {
				c.ev.OnError(err)
				continue
			}
			c.handle(Input{Kind: InputText, Text: ev}, func() error { return input.ApplyText(c.kb, ev) })
		case consts.TypeLockScreen:
			c.handle(Input{Kind: InputLock}, c.kb.LockScreen)
		case consts.TypeFinish:
			c.teardown(conn, ErrPhoneClosed)
			return
		case consts.TypeHeartBeatReq, consts.TypeHeartBeatRsp:
			// Liveness only. The Java client never answered and the phone
			// does not expect a reply; the read deadline does the rest.
		default:
			// Newer phones may send types we do not know yet. ReadMessage has
			// already consumed the whole frame, so report it and keep going.
			c.ev.OnError(fmt.Errorf("client: unknown message type 0x%02x", m.Type))
		}
	}
}

// handle replays one input unless paused, then tells the observer.
func (c *Client) handle(in Input, apply func() error) {
	applied := !c.paused.Load()
	if applied {
		if err := apply(); err != nil {
			c.ev.OnError(err)
		}
	}
	if o, ok := c.ev.(InputObserver); ok {
		o.OnInput(in, applied)
	}
}

// teardown handles a connection ending from the read side. If Close already
// detached conn it does nothing, so the UI sees exactly one Disconnected.
func (c *Client) teardown(conn net.Conn, cause error) {
	c.mu.Lock()
	if c.conn != conn {
		c.mu.Unlock()
		return
	}
	c.conn = nil
	c.state = Disconnected
	c.cause = cause
	c.mu.Unlock()
	_ = conn.Close()
	detail := ""
	if cause != nil {
		detail = cause.Error()
	}
	c.ev.OnState(Disconnected, detail)
}
