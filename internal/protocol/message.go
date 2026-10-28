// Package protocol encodes and decodes the frames exchanged with the phone.
//
// The two directions use different layouts, inherited from the Java code:
//
//	phone -> PC : [type, totalLen, body...]
//	PC -> phone : [totalLen, type, body..., CR, LF]
//
// totalLen always counts the two header bytes plus the body and excludes the
// trailing CRLF. See docs/protocol.md for the full description.
package protocol

import (
	"bytes"
	"errors"

	"github.com/bluehandle-open/bluehandle-go/internal/consts"
)

// Message is a decoded frame without framing bytes.
type Message struct {
	Type byte
	Body []byte
}

var (
	ErrTooLong      = errors.New("protocol: message longer than 127 bytes")
	ErrUnframeable  = errors.New("protocol: body contains CR or LF")
	ErrBadLength    = errors.New("protocol: totalLen smaller than header")
	ErrNotKeyEvent  = errors.New("protocol: not a key event message")
	ErrEmptyBody    = errors.New("protocol: key event without body")
	ErrBadHandshake = errors.New("protocol: unexpected server sign")
)

// EncodeToPhone frames m as [totalLen, type, body..., CR, LF].
//
// The phone reads PC frames with BufferedReader.readLine, so the body must
// not contain CR or LF and the frame must end with CRLF.
func EncodeToPhone(m Message) ([]byte, error) {
	total := 2 + len(m.Body)
	if total > consts.MaxMessageLen {
		return nil, ErrTooLong
	}
	if bytes.ContainsAny(m.Body, "\r\n") {
		return nil, ErrUnframeable
	}
	out := make([]byte, 0, total+2)
	out = append(out, byte(total), m.Type)
	out = append(out, m.Body...)
	return append(out, '\r', '\n'), nil
}

// NewKeySetting builds a key table message (SendHandleSetting and friends in Java).
func NewKeySetting(typ byte, keys []byte) Message {
	return Message{Type: typ, Body: append([]byte(nil), keys...)}
}

// NewFinish builds the message telling the phone that the PC is leaving.
func NewFinish() Message { return Message{Type: consts.TypeFinish} }

// NewHeartBeatReq builds a heartbeat request.
func NewHeartBeatReq() Message { return Message{Type: consts.TypeHeartBeatReq} }
