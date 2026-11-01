// Package session runs one phone connection for the desktop app: manual and
// automatic connecting, reconnecting after a dropped link, pausing, the
// activity log and counters. It knows nothing about the UI; the GUI layer
// forwards Sink callbacks to the frontend.
package session

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bluehandle-open/bluehandle-go/internal/bluetooth"
	"github.com/bluehandle-open/bluehandle-go/internal/client"
	"github.com/bluehandle-open/bluehandle-go/internal/config"
	"github.com/bluehandle-open/bluehandle-go/internal/input"
	"github.com/bluehandle-open/bluehandle-go/internal/protocol"
)

const (
	// MaxActivities bounds the in-memory activity log.
	MaxActivities = 200
	// holdThreshold separates a held key from a tap that arrived as two frames.
	holdThreshold = 400 * time.Millisecond
	// textMergeWindow joins the chunks of one long text into a single entry.
	textMergeWindow = time.Second
)

// Connection states reported in Status.State.
const (
	StateDisconnected = "disconnected"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
)

// Status is everything the UI needs to draw the connection card.
type Status struct {
	State       string    `json:"state"`
	Addr        string    `json:"addr"` // host:port being used or last used
	Paused      bool      `json:"paused"`
	ConnectedAt time.Time `json:"connected_at"`
	Retrying    bool      `json:"retrying"` // automatic connect/reconnect loop running
	Error       string    `json:"error"`    // client.ErrorKind of the last failure, "" if none
	Detail      string    `json:"detail"`   // raw error text for the tooltip
}

// Activity is one line of the "recently received" log. A text entry can be
// reported again with the same ID when more chunks of it arrive.
type Activity struct {
	ID      int64     `json:"id"`
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"` // key, combo, hold, text, lock, pause, resume
	Keys    []string  `json:"keys,omitempty"`
	HoldMs  int64     `json:"hold_ms,omitempty"`
	Text    string    `json:"text,omitempty"` // empty when Config.ShowText is off
	Chars   int       `json:"chars,omitempty"`
	Enter   bool      `json:"enter,omitempty"`
	Applied bool      `json:"applied"` // false when dropped while paused
}

// Stats feeds the counters on the connected card.
type Stats struct {
	Keys        int   `json:"keys"`
	Texts       int   `json:"texts"`
	LastFrameMs int64 `json:"last_frame_ms"` // -1 when not connected
}

// Sink receives updates. Calls come from several goroutines, never while
// the session lock is held, and must not block.
type Sink interface {
	OnStatus(Status)
	OnActivity(Activity)
}

// Session is safe for concurrent use.
type Session struct {
	c          *client.Client
	sink       Sink
	save       func(config.Config) error
	RetryEvery time.Duration // pause between automatic attempts

	mu     sync.Mutex
	cfg    config.Config
	status Status
	log    []Activity
	nextID int64
	keys   int
	texts  int
	held   map[int]time.Time
	cancel context.CancelFunc // current attempt or retry loop
	gen    int                // bumped by every Connect/Disconnect so stale loops stop
}

// New builds a session that replays input on kb. save persists config
// changes and may be nil.
func New(kb input.Keyboard, cfg config.Config, save func(config.Config) error, sink Sink) *Session {
	if save == nil {
		save = func(config.Config) error { return nil }
	}
	s := &Session{
		sink:       sink,
		save:       save,
		cfg:        cfg,
		RetryEvery: 3 * time.Second,
		held:       map[int]time.Time{},
		status:     Status{State: StateDisconnected},
	}
	s.c = client.New(kb, s)
	return s
}

// Client exposes the underlying client, e.g. to tune ReadTimeout in tests.
func (s *Session) Client() *client.Client { return s.c }

// Config returns a copy of the current settings.
func (s *Session) Config() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyConfig()
}

// UpdateConfig applies edit and saves the result.
func (s *Session) UpdateConfig(edit func(*config.Config)) error {
	s.mu.Lock()
	edit(&s.cfg)
	c := s.copyConfig()
	s.mu.Unlock()
	return s.save(c)
}

// Status returns the current connection status.
func (s *Session) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Activities returns the log, oldest first.
func (s *Session) Activities() []Activity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Activity(nil), s.log...)
}

// ClearActivities empties the log; counters are kept.
func (s *Session) ClearActivities() {
	s.mu.Lock()
	s.log = nil
	s.mu.Unlock()
}

// Stats returns the counters.
func (s *Session) Stats() Stats {
	s.mu.Lock()
	st := Stats{Keys: s.keys, Texts: s.texts, LastFrameMs: -1}
	connected := s.status.State == StateConnected
	s.mu.Unlock()
	if connected {
		st.LastFrameMs = time.Since(s.c.LastFrameAt()).Milliseconds()
	}
	return st
}

// HostPort adds the configured port to addr unless it already has one.
// Bluetooth addresses are normalised and have no port.
func (s *Session) HostPort(addr string) string {
	addr = strings.TrimSpace(addr)
	if v, err := bluetooth.ParseAddr(addr); err == nil {
		return bluetooth.FormatAddr(v)
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	s.mu.Lock()
	port := s.cfg.Port
	s.mu.Unlock()
	return net.JoinHostPort(addr, strconv.Itoa(port))
}

// Connect makes one attempt to reach the phone at addr (IP, optionally with
// a port), cancelling any attempt or retry loop already running. It blocks
// until the attempt finishes; the outcome is also reported through Status.
func (s *Session) Connect(addr string) error {
	ctx, gen := s.begin(false)
	return s.attempt(ctx, gen, s.HostPort(addr))
}

// ConnectRetry keeps trying addr every RetryEvery until it connects or is
// cancelled. It returns immediately.
func (s *Session) ConnectRetry(addr string) {
	ctx, gen := s.begin(true)
	go s.retryLoop(ctx, gen, s.HostPort(addr))
}

// AutoStart reconnects to the last phone if the user enabled it. It reports
// whether a retry loop was started.
func (s *Session) AutoStart() bool {
	s.mu.Lock()
	last, on := s.cfg.LastAddr(), s.cfg.AutoConnect
	s.mu.Unlock()
	if !on || last == "" {
		return false
	}
	ctx, gen := s.begin(true)
	go s.retryLoop(ctx, gen, last)
	return true
}

// Cancel stops a pending attempt or retry loop. An established connection
// is left alone.
func (s *Session) Cancel() {
	s.mu.Lock()
	s.stopLocked()
	s.status.Retrying = false
	st := s.status
	s.mu.Unlock()
	s.sink.OnStatus(st)
}

// Disconnect cancels any attempt and closes the connection, telling the
// phone. It does not trigger automatic reconnection.
func (s *Session) Disconnect() error {
	s.mu.Lock()
	s.stopLocked()
	s.status.Retrying = false
	s.status.Error, s.status.Detail = "", ""
	s.mu.Unlock()
	return s.c.Close()
}

// SetPaused stops or resumes replaying the phone's input.
func (s *Session) SetPaused(p bool) error {
	err := s.c.SetPaused(p)
	kind := "resume"
	if p {
		kind = "pause"
	}
	s.mu.Lock()
	if s.status.Paused == p {
		s.mu.Unlock()
		return err
	}
	s.status.Paused = p
	if p {
		s.held = map[int]time.Time{} // the client released everything
	}
	a := s.appendLocked(Activity{At: time.Now(), Kind: kind, Applied: true})
	st := s.status
	s.mu.Unlock()
	s.sink.OnActivity(a)
	s.sink.OnStatus(st)
	return err
}

// begin cancels whatever is running and starts a new generation.
func (s *Session) begin(retrying bool) (context.Context, int) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.stopLocked()
	s.cancel = cancel
	s.status.Retrying = retrying
	gen := s.gen
	s.mu.Unlock()
	return ctx, gen
}

func (s *Session) stopLocked() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.gen++
}

func (s *Session) retryLoop(ctx context.Context, gen int, hostport string) {
	for {
		if s.attempt(ctx, gen, hostport) == nil || ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.RetryEvery):
		}
	}
}

// attempt dials once and, on success, records the phone and watches the
// connection for the reconnect policy.
func (s *Session) attempt(ctx context.Context, gen int, hostport string) error {
	s.mu.Lock()
	s.status.Addr = hostport
	s.status.Error, s.status.Detail = "", ""
	s.mu.Unlock()

	err := s.c.Connect(ctx, hostport)

	s.mu.Lock()
	stale := gen != s.gen
	if err != nil {
		kind := client.Classify(err)
		if err == client.ErrAlreadyConnected {
			kind = client.ErrKindNone
		}
		if kind == client.ErrKindCanceled {
			kind = client.ErrKindNone
		}
		if !stale {
			s.status.Error, s.status.Detail = string(kind), err.Error()
		}
		st := s.status
		s.mu.Unlock()
		if !stale {
			s.sink.OnStatus(st)
		}
		return err
	}
	if stale {
		// Disconnect or a newer Connect ran while we were dialing.
		s.mu.Unlock()
		_ = s.c.Close()
		return context.Canceled
	}
	s.status.Retrying = false
	s.cancel = nil
	start := time.Now()
	s.cfg.Touch(hostport, start)
	c := s.copyConfig()
	st := s.status
	s.mu.Unlock()
	_ = s.save(c)
	s.sink.OnStatus(st)
	go s.watch(gen, hostport, start)
	return nil
}

// watch waits for the connection to end, records its duration and decides
// whether to reconnect: only an unexpected drop does, and only if enabled.
func (s *Session) watch(gen int, hostport string, start time.Time) {
	<-s.c.Done()
	cause := s.c.Cause()
	kind := client.Classify(cause)

	s.mu.Lock()
	s.cfg.SetDuration(hostport, time.Since(start))
	c := s.copyConfig()
	s.held = map[int]time.Time{}
	current := gen == s.gen
	if current {
		s.status.Error = string(kind)
		s.status.Detail = ""
		if cause != nil {
			s.status.Detail = cause.Error()
		}
	}
	reconnect := current && kind == client.ErrKindLost && s.cfg.AutoReconnect
	st := s.status
	s.mu.Unlock()
	_ = s.save(c)
	if !current {
		return
	}
	if reconnect {
		ctx, g := s.begin(true)
		st.Retrying = true
		s.sink.OnStatus(st)
		go s.retryLoop(ctx, g, hostport)
		return
	}
	s.sink.OnStatus(st)
}

// OnState implements client.Events.
func (s *Session) OnState(st client.State, _ string) {
	s.mu.Lock()
	switch st {
	case client.Connecting:
		s.status.State = StateConnecting
	case client.Connected:
		s.status.State = StateConnected
		s.status.ConnectedAt = time.Now()
	default:
		s.status.State = StateDisconnected
		s.status.ConnectedAt = time.Time{}
	}
	out := s.status
	s.mu.Unlock()
	s.sink.OnStatus(out)
}

// OnError implements client.Events. Per-message errors are not shown.
func (s *Session) OnError(error) {}

// OnInput implements client.InputObserver and turns raw input into log
// entries.
func (s *Session) OnInput(in client.Input, applied bool) {
	now := time.Now()
	s.mu.Lock()
	var out []Activity
	switch in.Kind {
	case client.InputKey:
		out = s.keyLocked(in.Key, applied, now)
	case client.InputText:
		out = []Activity{s.textLocked(in.Text, applied, now)}
	case client.InputLock:
		out = []Activity{s.appendLocked(Activity{At: now, Kind: "lock", Applied: applied})}
	}
	s.mu.Unlock()
	for _, a := range out {
		s.sink.OnActivity(a)
	}
}

func (s *Session) keyLocked(ev protocol.KeyEvent, applied bool, now time.Time) []Activity {
	if len(ev.Keys) == 0 {
		return nil
	}
	labels := Labels(ev.Keys)
	tap := func() Activity {
		kind := "key"
		if len(ev.Keys) > 1 {
			kind = "combo"
		}
		return s.appendLocked(Activity{At: now, Kind: kind, Keys: labels, Applied: applied})
	}
	switch {
	case ev.Down && ev.Up:
		s.keys++
		return []Activity{tap()}
	case ev.Down:
		s.keys++
		for _, k := range ev.Keys {
			s.held[k] = now
		}
		return nil
	default: // release only
		var since time.Time
		for _, k := range ev.Keys {
			if t, ok := s.held[k]; ok && (since.IsZero() || t.Before(since)) {
				since = t
			}
			delete(s.held, k)
		}
		if since.IsZero() || now.Sub(since) < holdThreshold {
			return []Activity{tap()}
		}
		return []Activity{s.appendLocked(Activity{
			At: now, Kind: "hold", Keys: labels, HoldMs: now.Sub(since).Milliseconds(), Applied: applied,
		})}
	}
}

func (s *Session) textLocked(ev protocol.TextEvent, applied bool, now time.Time) Activity {
	chars := utf8.RuneCountInString(ev.Text)
	if n := len(s.log); n > 0 {
		last := &s.log[n-1]
		if last.Kind == "text" && !last.Enter && last.Applied == applied && now.Sub(last.At) < textMergeWindow {
			last.At = now
			last.Chars += chars
			last.Enter = ev.Enter
			if s.cfg.ShowText {
				last.Text += ev.Text
			}
			return *last
		}
	}
	s.texts++
	a := Activity{At: now, Kind: "text", Chars: chars, Enter: ev.Enter, Applied: applied}
	if s.cfg.ShowText {
		a.Text = ev.Text
	}
	return s.appendLocked(a)
}

func (s *Session) appendLocked(a Activity) Activity {
	s.nextID++
	a.ID = s.nextID
	s.log = append(s.log, a)
	if len(s.log) > MaxActivities {
		s.log = append([]Activity(nil), s.log[len(s.log)-MaxActivities:]...)
	}
	return a
}

func (s *Session) copyConfig() config.Config {
	c := s.cfg
	c.Recent = append([]config.Recent(nil), s.cfg.Recent...)
	return c
}
