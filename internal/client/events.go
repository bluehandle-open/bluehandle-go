package client

import "github.com/bluehandle-open/bluehandle-go/internal/protocol"

// Events receives connection notifications. The UI implements it; this is
// what replaces the BlueHandle reference that WifiSocket and ClientThread
// used to hold.
//
// Callbacks run on the client's goroutines. Implementations must not block
// and must not call Client.Close from inside a callback.
type Events interface {
	OnState(s State, detail string)
	OnError(err error)
}

// NopEvents discards every notification.
type NopEvents struct{}

func (NopEvents) OnState(State, string) {}
func (NopEvents) OnError(error)         {}

// InputKind tells which kind of input the phone sent.
type InputKind int

const (
	InputKey InputKind = iota
	InputText
	InputLock
)

// Input is one decoded input message from the phone.
type Input struct {
	Kind InputKind
	Key  protocol.KeyEvent  // set for InputKey
	Text protocol.TextEvent // set for InputText
}

// InputObserver may be implemented by an Events value to see every input
// the phone sends. applied is false when the client is paused and the input
// was dropped. It runs on the read goroutine and must not block.
type InputObserver interface {
	OnInput(in Input, applied bool)
}
