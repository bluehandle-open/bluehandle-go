//go:build !windows

package platform

import (
	"errors"
	"time"
)

// IsElevated is only meaningful on Windows.
func IsElevated() bool { return false }

// ElevationNeeded is false where input injection is not blocked by
// integrity levels.
const ElevationNeeded = false

// RestartElevated is not supported outside Windows.
func RestartElevated([]string) error {
	return errors.New("platform: restarting elevated is only supported on Windows")
}

// WaitForPID is a no-op outside Windows.
func WaitForPID(int, time.Duration) {}
