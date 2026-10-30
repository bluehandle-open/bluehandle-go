//go:build !windows

package input

import (
	"errors"
	"runtime"
)

// NewSystem reports that real input injection is not implemented on this
// platform yet; callers fall back to Printer.
func NewSystem() (Keyboard, error) {
	return nil, errors.New("input: keyboard injection is not supported on " + runtime.GOOS + " yet")
}
