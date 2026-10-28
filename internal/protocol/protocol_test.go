package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/bluehandle-open/bluehandle-go/internal/consts"
)

// Byte samples below are taken from the Java implementation
// (SendHandleSetting, AbstractSendMessage.getMessage, KeyCommunication.sendSelf).

func TestEncodeHandleSetting(t *testing.T) {
	keys := []byte{87, 83, 65, 68, 85, 73, 79, 74, 75, 76, 49, 53}
	got, err := EncodeToPhone(NewKeySetting(consts.TypeHandleKeySet, keys))
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{14, 0x10}, keys...)
	want = append(want, '\r', '\n')
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

func TestEncodeFinish(t *testing.T) {
	got, err := EncodeToPhone(NewFinish())
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{2, 0x40, '\r', '\n'}; !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

func TestEncodeRejectsUnframeableBody(t *testing.T) {
	_, err := EncodeToPhone(Message{Type: consts.TypeText, Body: []byte("a\nb")})
	if !errors.Is(err, ErrUnframeable) {
		t.Fatalf("got %v, want ErrUnframeable", err)
	}
	_, err = EncodeToPhone(Message{Type: consts.TypeText, Body: make([]byte, 126)})
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("got %v, want ErrTooLong", err)
	}
}

func TestReadKeyEvent(t *testing.T) {
	// phone -> PC: type 0x02, totalLen 5, flags press, keys W S
	r := bufio.NewReader(bytes.NewReader([]byte{0x02, 0x05, 0x01, 87, 83}))
	m, err := ReadMessage(r)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := ParseKeyEvent(m)
	if err != nil {
		t.Fatal(err)
	}
	want := KeyEvent{Down: true, Keys: []int{87, 83}}
	if !reflect.DeepEqual(ev, want) {
		t.Fatalf("got %+v, want %+v", ev, want)
	}
}

func TestReadStreamOfMessages(t *testing.T) {
	stream := []byte{
		0x41, 0x02, // heartbeat
		0x02, 0x04, 0x03, 0xE0, // press+release, key 0xE0 (negative in Java, must read as 224)
		0x40, 0x02, // finish
	}
	r := bufio.NewReader(bytes.NewReader(stream))

	m, err := ReadMessage(r)
	if err != nil || m.Type != consts.TypeHeartBeatReq || len(m.Body) != 0 {
		t.Fatalf("heartbeat: %+v %v", m, err)
	}
	m, err = ReadMessage(r)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := ParseKeyEvent(m)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.Down || !ev.Up || !reflect.DeepEqual(ev.Keys, []int{224}) {
		t.Fatalf("got %+v", ev)
	}
	m, err = ReadMessage(r)
	if err != nil || m.Type != consts.TypeFinish {
		t.Fatalf("finish: %+v %v", m, err)
	}
	if _, err = ReadMessage(r); err != io.EOF {
		t.Fatalf("got %v, want io.EOF", err)
	}
}

func TestReadTruncatedBody(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader([]byte{0x02, 0x05, 0x01, 87}))
	if _, err := ReadMessage(r); err != io.ErrUnexpectedEOF {
		t.Fatalf("got %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestReadBadLength(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader([]byte{0x02, 0x01}))
	if _, err := ReadMessage(r); !errors.Is(err, ErrBadLength) {
		t.Fatalf("got %v, want ErrBadLength", err)
	}
}

func TestParseKeyEventDropsZeroAndChecksType(t *testing.T) {
	ev, err := ParseKeyEvent(Message{Type: consts.TypeKey, Body: []byte{2, 0, 17, 0}})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Down || !ev.Up || !reflect.DeepEqual(ev.Keys, []int{17}) {
		t.Fatalf("got %+v", ev)
	}
	if _, err := ParseKeyEvent(Message{Type: consts.TypeFinish}); !errors.Is(err, ErrNotKeyEvent) {
		t.Fatalf("got %v, want ErrNotKeyEvent", err)
	}
	if _, err := ParseKeyEvent(Message{Type: consts.TypeKey}); !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("got %v, want ErrEmptyBody", err)
	}
}

func TestReadHandshake(t *testing.T) {
	if err := ReadHandshake(bufio.NewReader(bytes.NewBufferString("MyBluetoothApp\r\n"))); err != nil {
		t.Fatal(err)
	}
	err := ReadHandshake(bufio.NewReader(bytes.NewBufferString("something else\n")))
	if !errors.Is(err, ErrBadHandshake) {
		t.Fatalf("got %v, want ErrBadHandshake", err)
	}
}
