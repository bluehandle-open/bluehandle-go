package protocol

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/bluehandle-open/bluehandle-go/internal/consts"
)

// ReadHandshake consumes the first line sent by a Bluetooth server and checks
// it against consts.ServerSign (MessageUtil.validServer in Java). The Wi-Fi
// server does not send a handshake, so Wi-Fi callers must not call this.
func ReadHandshake(r *bufio.Reader) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimRight(line, "\r\n") != consts.ServerSign {
		return ErrBadHandshake
	}
	return nil
}

// ReadMessage reads one phone -> PC frame: [type, totalLen, body...].
//
// Unlike MessageUtil.processMessage in Java, it uses io.ReadFull so a body
// split across TCP segments is never truncated. A stream that ends inside a
// frame yields io.ErrUnexpectedEOF; a clean end before a frame yields io.EOF.
func ReadMessage(r *bufio.Reader) (Message, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Message{}, err
	}
	total := int(hdr[1])
	if total < 2 {
		return Message{}, ErrBadLength
	}
	body := make([]byte, total-2)
	if _, err := io.ReadFull(r, body); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return Message{}, err
	}
	return Message{Type: hdr[0], Body: body}, nil
}

// KeyEvent is a decoded TypeKey message (ReciveKey in Java).
type KeyEvent struct {
	Down bool  // press every key in Keys
	Up   bool  // release every key in Keys
	Keys []int // wire key codes as sent; zero bytes are dropped. See keymap.Normalize
}

// ParseKeyEvent decodes body[0] as press flags and body[1:] as key codes.
// Bytes are treated as unsigned, matching the `keyCode &= 0xff` in
// KeyProcessUtil.java.
func ParseKeyEvent(m Message) (KeyEvent, error) {
	if m.Type != consts.TypeKey {
		return KeyEvent{}, ErrNotKeyEvent
	}
	if len(m.Body) == 0 {
		return KeyEvent{}, ErrEmptyBody
	}
	flags := m.Body[0]
	ev := KeyEvent{
		Down: flags&consts.PressFlagDown != 0,
		Up:   flags&consts.PressFlagUp != 0,
	}
	for _, b := range m.Body[1:] {
		if b != 0 {
			ev.Keys = append(ev.Keys, int(b))
		}
	}
	return ev, nil
}

// TextEvent is a decoded TypeText message.
type TextEvent struct {
	Text  string // may be empty when the phone only wants Enter
	Enter bool   // press Enter after typing Text
}

// ParseText decodes body[0] as flags and body[1:] as UTF-8 text. The phone
// never splits a character across frames, so each frame must be valid UTF-8
// on its own.
func ParseText(m Message) (TextEvent, error) {
	if m.Type != consts.TypeText {
		return TextEvent{}, ErrNotText
	}
	if len(m.Body) == 0 {
		return TextEvent{}, ErrEmptyBody
	}
	text := m.Body[1:]
	if !utf8.Valid(text) {
		return TextEvent{}, ErrBadUTF8
	}
	return TextEvent{
		Text:  string(text),
		Enter: m.Body[0]&consts.TextFlagEnter != 0,
	}, nil
}
