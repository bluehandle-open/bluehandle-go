package platform

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether this process runs as administrator.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// ElevationNeeded reports whether elevation can matter on this platform:
// Windows blocks input into elevated windows from a normal process (UIPI).
const ElevationNeeded = true

// RestartElevated starts this executable again as administrator with args,
// adding WaitPIDFlag so the new process waits for this one to exit before
// claiming the single-instance lock. The caller should quit afterwards. The
// user can refuse the UAC prompt, which is reported as an error.
func RestartElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args = append(args, WaitPIDFlag+"="+strconv.Itoa(os.Getpid()))
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	return windows.ShellExecute(0, verb, file, params, nil, windows.SW_SHOWNORMAL)
}

// WaitForPID blocks until process pid exits or timeout passes.
func WaitForPID(pid int, timeout time.Duration) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return // already gone
	}
	defer windows.CloseHandle(h)
	windows.WaitForSingleObject(h, uint32(timeout/time.Millisecond))
}
