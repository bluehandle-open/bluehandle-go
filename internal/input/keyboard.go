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

// Keyboard injects input into the local desktop. Key codes are Windows
// virtual-key codes exactly as they arrive on the wire; other platforms
// translate them.
type Keyboard interface {
	Press(vk int) error
	Release(vk int) error
	// TypeText types a Unicode string independent of the active input method
	// (SendInput with KEYEVENTF_UNICODE on Windows).
	TypeText(text string) error
	// LockScreen locks the workstation (LockWorkStation on Windows; a
	// synthesized Win+L is swallowed by the system).
	LockScreen() error
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

// ApplyText types ev.Text, then taps Enter if requested.
func ApplyText(kb Keyboard, ev protocol.TextEvent) error {
	if ev.Text != "" {
		if err := kb.TypeText(ev.Text); err != nil {
			return err
		}
	}
	if ev.Enter {
		if err := kb.Press(keymap.VKEnter); err != nil {
			return err
		}
		return kb.Release(keymap.VKEnter)
	}
	return nil
}

// Printer writes what it would have typed. Used by the CLI and by tests.
type Printer struct {
	W io.Writer
}

func (p Printer) Press(vk int) error {
	_, err := fmt.Fprintf(p.W, "press %s\n", keymap.KeyName(vk))
	return err
}

func (p Printer) Release(vk int) error {
	_, err := fmt.Fprintf(p.W, "release %s\n", keymap.KeyName(vk))
	return err
}

func (p Printer) TypeText(text string) error {
	_, err := fmt.Fprintf(p.W, "type %q\n", text)
	return err
}

func (p Printer) LockScreen() error {
	_, err := fmt.Fprintln(p.W, "lock screen")
	return err
}
