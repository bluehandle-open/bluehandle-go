package client

// State is the connection state shown in the UI. It replaces the
// CONN_TYPE_* / CONN_NONE integers in BlueHandle.java.
type State int

const (
	Disconnected State = iota
	Connecting
	Connected
)

func (s State) String() string {
	switch s {
	case Disconnected:
		return "disconnected"
	case Connecting:
		return "connecting"
	case Connected:
		return "connected"
	}
	return "unknown"
}
