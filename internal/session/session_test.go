package session

import (
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluehandle-open/bluehandle-go/internal/client"
	"github.com/bluehandle-open/bluehandle-go/internal/config"
)

type nopKB struct{}

func (nopKB) Press(int) error       { return nil }
func (nopKB) Release(int) error     { return nil }
func (nopKB) TypeText(string) error { return nil }
func (nopKB) LockScreen() error     { return nil }

type sink struct {
	mu       sync.Mutex
	statuses []Status
}

func (k *sink) OnStatus(s Status)   { k.mu.Lock(); k.statuses = append(k.statuses, s); k.mu.Unlock() }
func (k *sink) OnActivity(Activity) {}
func (k *sink) last() Status {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.statuses) == 0 {
		return Status{}
	}
	return k.statuses[len(k.statuses)-1]
}

// phone accepts any number of connections on a loopback port.
func phone(t *testing.T) (addr string, conns <-chan net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
			ch <- c
		}
	}()
	return ln.Addr().String(), ch
}

func accept(t *testing.T, conns <-chan net.Conn) net.Conn {
	t.Helper()
	select {
	case c := <-conns:
		return c
	case <-time.After(3 * time.Second):
		t.Fatal("phone was not contacted")
		return nil
	}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func newSession(t *testing.T) (*Session, *sink, *[]config.Config) {
	var saved []config.Config
	var mu sync.Mutex
	k := &sink{}
	s := New(nopKB{}, config.Default(), func(c config.Config) error {
		mu.Lock()
		saved = append(saved, c)
		mu.Unlock()
		return nil
	}, k)
	s.RetryEvery = 50 * time.Millisecond
	t.Cleanup(func() { s.Disconnect() })
	return s, k, &saved
}

func describe(acts []Activity) []string {
	var out []string
	for _, a := range acts {
		d := a.Kind
		if len(a.Keys) > 0 {
			d += " " + strings.Join(a.Keys, "+")
		}
		if a.Kind == "text" {
			d += " " + a.Text
			if a.Enter {
				d += "↵"
			}
		}
		if a.Kind == "hold" && a.HoldMs >= 400 {
			d += " long"
		}
		if !a.Applied {
			d += " (dropped)"
		}
		out = append(out, d)
	}
	return out
}

func TestActivityLog(t *testing.T) {
	addr, conns := phone(t)
	s, _, saved := newSession(t)
	if err := s.Connect(addr); err != nil {
		t.Fatal(err)
	}
	p := accept(t, conns)

	p.Write([]byte{0x02, 0x04, 0x03, 'J'})        // tap J
	p.Write([]byte{0x02, 0x05, 0x03, 0x5B, 'D'})  // Win+D
	p.Write([]byte{0x02, 0x04, 0x01, 'K'})        // K down ...
	p.Write([]byte{0x02, 0x04, 0x02, 'K'})        // ... up at once: a tap
	p.Write([]byte{0x02, 0x04, 0x01, 'W'})        // W held
	time.Sleep(450 * time.Millisecond)            //
	p.Write([]byte{0x02, 0x04, 0x02, 'W'})        // released: a hold
	p.Write(append([]byte{0x50, 9, 0}, "你好"...))  // text, chunk 1
	p.Write(append([]byte{0x50, 6, 1}, " ab"...)) // chunk 2 with Enter
	p.Write([]byte{0x50, 3, 1})                   // a lone Enter starts a new entry
	p.Write([]byte{0x51, 0x02})                   // lock
	eventually(t, "lock entry", func() bool { return len(s.Activities()) == 7 })
	s.SetPaused(true)
	p.Write([]byte{0x02, 0x04, 0x03, 0x22}) // PgDn while paused
	eventually(t, "dropped entry", func() bool { return len(s.Activities()) == 9 })
	s.SetPaused(false)

	want := []string{
		"key J", "combo Win+D", "key K", "hold W long",
		"text 你好 ab↵", "text ↵", "lock",
		"pause", "key PgDn (dropped)", "resume",
	}
	if got := describe(s.Activities()); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if st := s.Stats(); st.Keys != 5 || st.Texts != 2 || st.LastFrameMs < 0 {
		t.Fatalf("stats %+v", st)
	}
	if len(*saved) == 0 || (*saved)[len(*saved)-1].LastAddr() != addr {
		t.Fatal("successful connection should be saved as the most recent phone")
	}
}

func TestTextHiddenWhenShowTextOff(t *testing.T) {
	addr, conns := phone(t)
	s, _, _ := newSession(t)
	s.UpdateConfig(func(c *config.Config) { c.ShowText = false })
	if err := s.Connect(addr); err != nil {
		t.Fatal(err)
	}
	p := accept(t, conns)
	p.Write(append([]byte{0x50, 9, 0}, "秘密"...))
	eventually(t, "text entry", func() bool { return len(s.Activities()) == 1 })
	if a := s.Activities()[0]; a.Text != "" || a.Chars != 2 {
		t.Fatalf("got %+v", a)
	}
}

func TestReconnectOnlyAfterUnexpectedDrop(t *testing.T) {
	addr, conns := phone(t)
	s, k, _ := newSession(t)
	if err := s.Connect(addr); err != nil {
		t.Fatal(err)
	}
	accept(t, conns).Close() // link drops without a goodbye
	second := accept(t, conns)
	eventually(t, "reconnected", func() bool { return s.Status().State == StateConnected && !s.Status().Retrying })

	second.Write([]byte{0x40, 0x02}) // phone disconnects on purpose
	eventually(t, "phone_closed", func() bool { return k.last().Error == string(client.ErrKindPhoneClosed) })
	select {
	case <-conns:
		t.Fatal("must not reconnect after the phone said goodbye")
	case <-time.After(200 * time.Millisecond):
	}
	if st := s.Status(); st.State != StateDisconnected || st.Retrying {
		t.Fatalf("status %+v", st)
	}
}

func TestNoReconnectWhenDisabledOrUserDisconnects(t *testing.T) {
	addr, conns := phone(t)
	s, _, _ := newSession(t)
	s.UpdateConfig(func(c *config.Config) { c.AutoReconnect = false })
	if err := s.Connect(addr); err != nil {
		t.Fatal(err)
	}
	accept(t, conns).Close()
	eventually(t, "lost", func() bool { return s.Status().Error == string(client.ErrKindLost) })

	s.UpdateConfig(func(c *config.Config) { c.AutoReconnect = true })
	if err := s.Connect(addr); err != nil {
		t.Fatal(err)
	}
	accept(t, conns)
	s.Disconnect()
	select {
	case <-conns:
		t.Fatal("must not reconnect after the user disconnects")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRetryUntilPhoneListensAndCancel(t *testing.T) {
	// Reserve a port, then free it so the first attempts are refused.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()

	s, k, _ := newSession(t)
	s.ConnectRetry(addr)
	eventually(t, "refused", func() bool { return k.last().Error == string(client.ErrKindRefused) })
	if !s.Status().Retrying {
		t.Fatal("should still be retrying")
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("port reused by someone else: %v", err)
	}
	defer ln.Close()
	eventually(t, "connected", func() bool { return s.Status().State == StateConnected })
	if st := s.Status(); st.Retrying || st.Error != "" {
		t.Fatalf("status %+v", st)
	}
	s.Disconnect()

	// Cancel stops a loop that can never succeed.
	ln.Close()
	s.ConnectRetry(addr)
	eventually(t, "retrying", func() bool { return k.last().Error == string(client.ErrKindRefused) })
	s.Cancel()
	if s.Status().Retrying {
		t.Fatal("Cancel should stop retrying")
	}
}

func TestConnectErrorsAndHostPort(t *testing.T) {
	s, _, _ := newSession(t)
	if got := s.HostPort(" 192.168.6.144 "); got != "192.168.6.144:8088" {
		t.Fatalf("HostPort = %q", got)
	}
	if got := s.HostPort("10.0.0.2:9000"); got != "10.0.0.2:9000" {
		t.Fatalf("HostPort = %q", got)
	}
	if err := s.Connect("1.2.3:abc"); err == nil || s.Status().Error != string(client.ErrKindBadAddress) {
		t.Fatalf("bad address: %v %+v", err, s.Status())
	}
}

func TestLabels(t *testing.T) {
	got := Labels([]int{0x5B, 'D', 0x22, 0xAF, 0x74, 0x61, 0xBC})
	want := []string{"Win", "D", "PgDn", "音量+", "F5", "小键盘1", ","}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
