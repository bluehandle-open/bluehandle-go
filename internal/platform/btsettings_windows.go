package platform

import "os/exec"

// OpenBluetoothSettings shows the Windows Bluetooth page, where the user
// pairs the phone.
func OpenBluetoothSettings() error {
	return exec.Command("explorer.exe", "ms-settings:bluetooth").Start()
}
