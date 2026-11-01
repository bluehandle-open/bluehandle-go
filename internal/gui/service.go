// Package gui is the Wails layer of the desktop app: it exposes the session
// to the frontend as bound methods and pushes its updates as events. Window
// and tray handling live in cmd/bluehandle.
package gui

import (
	"time"

	"github.com/bluehandle-open/bluehandle-go/internal/bluetooth"
	"github.com/bluehandle-open/bluehandle-go/internal/config"
	"github.com/bluehandle-open/bluehandle-go/internal/platform"
	"github.com/bluehandle-open/bluehandle-go/internal/session"
)

// Event names emitted to the frontend.
const (
	EventStatus   = "status"   // session.Status
	EventActivity = "activity" // session.Activity
	EventStats    = "stats"    // session.Stats, once a second while connected
	EventNavigate = "navigate" // page name, e.g. "settings"
)

// Settings is the part of config.Config the settings page edits.
type Settings struct {
	Port          int  `json:"port"`
	AutoConnect   bool `json:"auto_connect"`
	AutoReconnect bool `json:"auto_reconnect"`
	StartOnBoot   bool `json:"start_on_boot"`
	CloseToTray   bool `json:"close_to_tray"`
	ShowText      bool `json:"show_text"`
	Notify        bool `json:"notify"`
}

// AppInfo is static information for the footer and the elevation banner.
type AppInfo struct {
	Version         string `json:"version"`
	Elevated        bool   `json:"elevated"`
	ElevationNeeded bool   `json:"elevation_needed"`
}

// Host is what the service needs from the application shell.
type Host interface {
	SetStartOnBoot(on bool) error
	RestartElevated() error
}

// Service is bound to the frontend; every exported method becomes a call.
type Service struct {
	s       *session.Session
	host    Host
	version string
}

// NewService wraps s for the frontend.
func NewService(s *session.Session, host Host, version string) *Service {
	return &Service{s: s, host: host, version: version}
}

// Status returns the current connection status.
func (v *Service) Status() session.Status { return v.s.Status() }

// Stats returns the counters.
func (v *Service) Stats() session.Stats { return v.s.Stats() }

// Activities returns the activity log, oldest first.
func (v *Service) Activities() []session.Activity { return v.s.Activities() }

// ClearActivities empties the activity log.
func (v *Service) ClearActivities() { v.s.ClearActivities() }

// Connect makes one attempt to reach the phone; failures are also reported
// through the status event, so the frontend may ignore the error.
func (v *Service) Connect(addr string) error { return v.s.Connect(addr) }

// Cancel stops a pending attempt or retry loop.
func (v *Service) Cancel() { v.s.Cancel() }

// Disconnect closes the connection and tells the phone.
func (v *Service) Disconnect() error { return v.s.Disconnect() }

// SetPaused stops or resumes replaying the phone's input.
func (v *Service) SetPaused(p bool) error { return v.s.SetPaused(p) }

// PairedDevices lists the Bluetooth devices paired with this computer,
// phones first. An error means Bluetooth is unavailable here.
func (v *Service) PairedDevices() ([]bluetooth.Device, error) { return bluetooth.Paired() }

// OpenBluetoothSettings shows the system page for pairing the phone.
func (v *Service) OpenBluetoothSettings() error { return platform.OpenBluetoothSettings() }

// Recent lists recently used phones, newest first.
func (v *Service) Recent() []config.Recent { return v.s.Config().Recent }

// ForgetRecent removes a phone from the recent list.
func (v *Service) ForgetRecent(addr string) error {
	return v.s.UpdateConfig(func(c *config.Config) { c.Forget(addr) })
}

// LastAddr is the address to prefill: the most recent phone's IP without
// the default port.
func (v *Service) LastAddr() string { return v.s.Config().LastAddr() }

// LocalIP is this PC's LAN address, preferring the phone's subnet.
func (v *Service) LocalIP() string {
	phone := v.s.Status().Addr
	if phone == "" {
		phone = v.s.Config().LastAddr()
	}
	return platform.PreferredLANAddr(phone)
}

// Settings returns the editable settings.
func (v *Service) Settings() Settings {
	c := v.s.Config()
	return Settings{
		Port: c.Port, AutoConnect: c.AutoConnect, AutoReconnect: c.AutoReconnect,
		StartOnBoot: c.StartOnBoot, CloseToTray: c.CloseToTray, ShowText: c.ShowText, Notify: c.Notify,
	}
}

// SaveSettings stores st. Changing StartOnBoot registers or removes the
// login item first so a failure leaves the setting unchanged.
func (v *Service) SaveSettings(st Settings) error {
	if st.Port <= 0 || st.Port > 65535 {
		st.Port = v.s.Config().Port
	}
	if st.StartOnBoot != v.s.Config().StartOnBoot {
		if err := v.host.SetStartOnBoot(st.StartOnBoot); err != nil {
			return err
		}
	}
	return v.s.UpdateConfig(func(c *config.Config) {
		c.Port, c.AutoConnect, c.AutoReconnect = st.Port, st.AutoConnect, st.AutoReconnect
		c.StartOnBoot, c.CloseToTray, c.ShowText, c.Notify = st.StartOnBoot, st.CloseToTray, st.ShowText, st.Notify
	})
}

// Info returns the version and elevation state.
func (v *Service) Info() AppInfo {
	return AppInfo{Version: v.version, Elevated: platform.IsElevated(), ElevationNeeded: platform.ElevationNeeded}
}

// RestartAsAdmin relaunches the app elevated; the current process quits if
// the user accepts the UAC prompt.
func (v *Service) RestartAsAdmin() error { return v.host.RestartElevated() }

// Now returns the backend clock, so durations shown by the frontend match
// the timestamps the backend produced.
func (v *Service) Now() time.Time { return time.Now() }
