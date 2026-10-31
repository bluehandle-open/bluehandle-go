package client

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
)

// ErrorKind groups connection errors by what the user should check.
type ErrorKind string

const (
	ErrKindNone        ErrorKind = ""
	ErrKindRefused     ErrorKind = "refused"      // phone is not waiting for a PC
	ErrKindTimeout     ErrorKind = "timeout"      // no answer: other network or blocked
	ErrKindUnreachable ErrorKind = "unreachable"  // no route to the phone
	ErrKindBadAddress  ErrorKind = "bad_address"  // address cannot be parsed or resolved
	ErrKindPhoneClosed ErrorKind = "phone_closed" // phone disconnected on purpose
	ErrKindLost        ErrorKind = "lost"         // established link dropped
	ErrKindCanceled    ErrorKind = "canceled"
	ErrKindOther       ErrorKind = "other"
)

// Classify maps a Connect error or a Cause to an ErrorKind.
func Classify(err error) ErrorKind {
	if err == nil {
		return ErrKindNone
	}
	if errors.Is(err, ErrPhoneClosed) {
		return ErrKindPhoneClosed
	}
	if errors.Is(err, context.Canceled) {
		return ErrKindCanceled
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case errRefused:
			return ErrKindRefused
		case errNetUnreach, errHostUnreach:
			return ErrKindUnreachable
		}
	}
	var addrErr *net.AddrError
	var dnsErr *net.DNSError
	if errors.As(err, &addrErr) || errors.As(err, &dnsErr) {
		return ErrKindBadAddress
	}
	var opErr *net.OpError
	isDial := errors.As(err, &opErr) && opErr.Op == "dial"
	var ne net.Error
	if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) ||
		(errors.As(err, &ne) && ne.Timeout()) {
		if isDial {
			return ErrKindTimeout
		}
		return ErrKindLost // read deadline: no heartbeat from the phone
	}
	if isDial {
		return ErrKindOther
	}
	return ErrKindLost
}
