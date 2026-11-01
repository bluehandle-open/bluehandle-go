//go:build !windows

package platform

import "errors"

// OpenBluetoothSettings is only implemented on Windows.
func OpenBluetoothSettings() error {
	return errors.New("platform: opening Bluetooth settings is not supported here")
}
