// Package input turns decoded key events into keystrokes on this machine.
// It replaces KeyProcessUtil.java. The robotgo-backed implementation is
// added in a later phase; Printer exists so the rest of the program can be
// exercised without cgo.
package input

import (
	"fmt"
	"io"

	"github.com/bluehandle-open/bluehandle-go/internal/keymap"
	"github.com/bluehandle-open/bluehandle-go/internal/protocol"
)

// Keyboard injects input into the local desktop. Key codes are Java VK codes
// exactly as they arrive on the wire; implementations translate them.
type Keyboard interface {
	Press(javaKeyCode int) error
	Release(javaKeyCode int) error
	// TypeText types a Unicode string. Reserved for voice input.
	TypeText(text string) error
}

// Apply replays ev on kb in the same order as KeyProcessUtil.fireKeyEvent:
// every key is pressed first, then every key is released.
func Apply(kb Keyboard, ev protocol.KeyEvent) error {
	if ev.Down {
		for _, k := range ev.Keys {
			if err := kb.Press(k); err != nil {
				return err
			}
		}
	}
	if ev.Up {
		for _, k := range ev.Keys {
			if err := kb.Release(k); err != nil {
				return err
			}
		}
	}
	return nil
}

// Printer writes what it would have typed. Used by the CLI and by tests.
type Printer struct {
	W io.Writer
}

func (p Printer) Press(code int) error {
	_, err := fmt.Fprintf(p.W, "press %s\n", keymap.KeyName(code))
	return err
}

func (p Printer) Release(code int) error {
	_, err := fmt.Fprintf(p.W, "release %s\n", keymap.KeyName(code))
	return err
}

func (p Printer) TypeText(text string) error {
	_, err := fmt.Fprintf(p.W, "type %q\n", text)
	return err
}
