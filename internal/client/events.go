package client

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
