package client

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/bluehandle-open/bluehandle-go/internal/protocol"
)

type recorder struct {
	mu  sync.Mutex
	log []string
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	r.log = append(r.log, s)
	r.mu.Unlock()
}
func (r *recorder) Press(c int) error       { r.add(fmt.Sprintf("press %d", c)); return nil }
func (r *recorder) Release(c int) error     { r.add(fmt.Sprintf("release %d", c)); return nil }
func (r *recorder) TypeText(s string) error { r.add("type " + s); return nil }
func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.log...)
}

type states struct {
	mu sync.Mutex
	s  []State
}

func (s *states) OnState(st State, _ string) {
	s.mu.Lock()
	s.s = append(s.s, st)
	s.mu.Unlock()
}
func (s *states) OnError(error) {}
func (s *states) snapshot() []State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]State(nil), s.s...)
}

// fakePhone listens on a loopback port and hands over the first connection.
func fakePhone(t *testing.T) (addr string, accepted <-chan net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			ch <- conn
		}
	}()
	return ln.Addr().String(), ch
}

func waitDone(t *testing.T, c *Client) {
	t.Helper()
	select {
	case <-c.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("read loop did not exit")
	}
}

func TestKeyEventsReachKeyboardAndFinishDisconnects(t *testing.T) {
	addr, accepted := fakePhone(t)
	kb := &recorder{}
	st := &states{}
	c := New(kb, st)
	if err := c.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	phone := <-accepted
	defer phone.Close()

	phone.Write([]byte{0x41, 0x02})               // heartbeat
	phone.Write([]byte{0x02, 0x05, 0x01, 87, 83}) // press W S
	phone.Write([]byte{0x02, 0x05, 0x02, 87, 83}) // release W S
	phone.Write([]byte{0x40, 0x02})               // finish
	waitDone(t, c)

	want := []string{"press 87", "press 83", "release 87", "release 83"}
	if got := kb.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("keyboard got %v, want %v", got, want)
	}
	if c.State() != Disconnected {
		t.Fatalf("state %v, want disconnected", c.State())
	}
	if got := st.snapshot(); !reflect.DeepEqual(got, []State{Connecting, Connected, Disconnected}) {
		t.Fatalf("states %v", got)
	}
}

func TestCloseSendsFinishAndKeySettingIsFramed(t *testing.T) {
	addr, accepted := fakePhone(t)
	c := New(&recorder{}, nil)
	if err := c.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	phone := <-accepted
	defer phone.Close()
	r := bufio.NewReader(phone)

	if err := c.SendKeySetting(0x20); err != nil {
		t.Fatal(err)
	}
	line, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	wantSetting := []byte{9, 0x20, 38, 40, 37, 39, 27, 16, 116, '\r', '\n'}
	if !bytes.Equal(line, wantSetting) {
		t.Fatalf("setting frame % x, want % x", line, wantSetting)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	line, err = r.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{2, 0x40, '\r', '\n'}; !bytes.Equal(line, want) {
		t.Fatalf("finish frame % x, want % x", line, want)
	}
	waitDone(t, c)
	if err := c.Send(protocol.NewHeartBeatReq()); err != ErrNotConnected {
		t.Fatalf("send after close: %v", err)
	}
}

func TestReadTimeoutDisconnects(t *testing.T) {
	addr, accepted := fakePhone(t)
	c := New(&recorder{}, nil)
	c.ReadTimeout = 200 * time.Millisecond
	if err := c.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	phone := <-accepted
	defer phone.Close()
	waitDone(t, c)
	if c.State() != Disconnected {
		t.Fatal("silent phone should be treated as gone")
	}
}

func TestConnectRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	c := New(&recorder{}, nil)
	if err := c.Connect(context.Background(), addr); err == nil {
		t.Fatal("expected dial error")
	}
	if c.State() != Disconnected {
		t.Fatal("failed dial must leave the client disconnected")
	}
}
