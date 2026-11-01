package client

import (
	"context"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"
)

// observer records every input with its applied flag.
type observer struct {
	NopEvents
	mu  sync.Mutex
	got []string
}

func (o *observer) OnInput(in Input, applied bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := map[InputKind]string{InputKey: "key", InputText: "text", InputLock: "lock"}[in.Kind]
	if in.Kind == InputText {
		s += " " + in.Text.Text
	}
	if !applied {
		s += " (dropped)"
	}
	o.got = append(o.got, s)
}

func (o *observer) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.got...)
}

func TestPauseDropsInputButObserverSeesIt(t *testing.T) {
	addr, accepted := fakePhone(t)
	kb := &releasingRecorder{}
	obs := &observer{}
	c := New(kb, obs)
	if err := c.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	phone := <-accepted
	defer phone.Close()

	phone.Write([]byte{0x02, 0x04, 0x01, 'W'}) // press W, still held when pausing
	waitFor(t, func() bool { return len(obs.snapshot()) == 1 })
	if err := c.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	if !c.Paused() {
		t.Fatal("Paused() should report true")
	}
	phone.Write([]byte{0x02, 0x04, 0x03, 'J'})
	phone.Write([]byte{0x50, 0x04, 0x00, 'x'})
	phone.Write([]byte{0x51, 0x02})
	waitFor(t, func() bool { return len(obs.snapshot()) == 4 })
	c.SetPaused(false)
	phone.Write([]byte{0x02, 0x04, 0x03, 'K'})
	phone.Write([]byte{0x40, 0x02})
	waitDone(t, c)

	wantKB := []string{"press 87", "release all", "press 75", "release 75", "release all"}
	if got := kb.snapshot(); !reflect.DeepEqual(got, wantKB) {
		t.Fatalf("keyboard got %v, want %v", got, wantKB)
	}
	wantObs := []string{"key", "key (dropped)", "text x (dropped)", "lock (dropped)", "key"}
	if got := obs.snapshot(); !reflect.DeepEqual(got, wantObs) {
		t.Fatalf("observer got %v, want %v", got, wantObs)
	}
}

func TestCauseAndClassify(t *testing.T) {
	// Phone says goodbye.
	addr, accepted := fakePhone(t)
	c := New(&recorder{}, nil)
	if err := c.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	phone := <-accepted
	if got := Classify(c.Cause()); got != ErrKindNone {
		t.Fatalf("cause while connected: %q", got)
	}
	phone.Write([]byte{0x40, 0x02})
	waitDone(t, c)
	if got := Classify(c.Cause()); got != ErrKindPhoneClosed {
		t.Fatalf("finish: got %q", got)
	}
	phone.Close()

	// Phone goes silent.
	addr, accepted = fakePhone(t)
	c = New(&recorder{}, nil)
	c.ReadTimeout = 150 * time.Millisecond
	if err := c.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	phone = <-accepted
	defer phone.Close()
	waitDone(t, c)
	if got := Classify(c.Cause()); got != ErrKindLost {
		t.Fatalf("silence: got %q (%v)", got, c.Cause())
	}

	// Nobody listening.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := ln.Addr().String()
	ln.Close()
	if got := Classify(New(&recorder{}, nil).Connect(context.Background(), closed)); got != ErrKindRefused {
		t.Fatalf("refused: got %q", got)
	}

	// Garbage address and cancellation.
	if got := Classify(New(&recorder{}, nil).Connect(context.Background(), "1.2.3:abc")); got != ErrKindBadAddress {
		t.Fatalf("bad address: got %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := Classify(New(&recorder{}, nil).Connect(ctx, closed)); got != ErrKindCanceled {
		t.Fatalf("canceled: got %q", got)
	}
}

func TestCloseClearsCause(t *testing.T) {
	addr, accepted := fakePhone(t)
	c := New(&recorder{}, nil)
	if err := c.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	phone := <-accepted
	defer phone.Close()
	before := c.LastFrameAt()
	time.Sleep(30 * time.Millisecond) // Windows clock ticks are coarse
	phone.Write([]byte{0x41, 0x02})
	waitFor(t, func() bool { return c.LastFrameAt().After(before) })
	c.Close()
	waitDone(t, c)
	if c.Cause() != nil {
		t.Fatalf("Close should leave no cause, got %v", c.Cause())
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
