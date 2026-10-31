package platform

import (
	"strconv"
	"strings"
)

const (
	// WaitPIDFlag is passed to a restarted instance with the old PID.
	WaitPIDFlag = "--wait-pid"
	// HiddenFlag starts the app in the tray without showing the window; used
	// for launch at login.
	HiddenFlag = "--hidden"
)

// WaitPID returns the PID given with WaitPIDFlag, or 0.
func WaitPID(args []string) int {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, WaitPIDFlag+"="); ok {
			if pid, err := strconv.Atoi(v); err == nil {
				return pid
			}
		}
	}
	return 0
}

// HasFlag reports whether flag appears in args.
func HasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
