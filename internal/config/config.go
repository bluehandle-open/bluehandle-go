// Package config stores the desktop app's settings and recent phones in a
// JSON file under the user's config directory.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bluehandle-open/bluehandle-go/internal/consts"
)

// MaxRecent bounds the "recent phones" list.
const MaxRecent = 5

// Recent is one phone the PC has connected to.
type Recent struct {
	Addr     string    `json:"addr"`
	LastAt   time.Time `json:"last_at"`
	Duration int64     `json:"duration_seconds"` // length of the last session
}

// Config is the whole settings file. Zero values are not meaningful; start
// from Default and let Load fill in what the file has.
type Config struct {
	Port          int      `json:"port"`
	AutoConnect   bool     `json:"auto_connect"`   // reconnect to the last phone on start
	AutoReconnect bool     `json:"auto_reconnect"` // retry after an unexpected drop
	StartOnBoot   bool     `json:"start_on_boot"`
	CloseToTray   bool     `json:"close_to_tray"`
	ShowText      bool     `json:"show_text"` // show typed text in the activity log
	Notify        bool     `json:"notify"`    // system notification on connect/disconnect
	Recent        []Recent `json:"recent"`
}

// Default returns the settings used before the user changes anything.
func Default() Config {
	return Config{
		Port:          consts.Port,
		AutoConnect:   true,
		AutoReconnect: true,
		CloseToTray:   true,
		ShowText:      true,
		Notify:        true,
	}
}

// DefaultPath is <user config dir>/bluehandle/config.json, which is
// %AppData%\bluehandle\config.json on Windows.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bluehandle", "config.json"), nil
}

// Load reads path on top of Default. A missing file is not an error.
func Load(path string) (Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Default(), err
	}
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = consts.Port
	}
	if len(c.Recent) > MaxRecent {
		c.Recent = c.Recent[:MaxRecent]
	}
	return c, nil
}

// Save writes c to path atomically: a crash never leaves a half-written file.
func Save(path string, c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Touch moves addr to the front of the recent list, adding it if needed.
func (c *Config) Touch(addr string, at time.Time) {
	r := Recent{Addr: addr}
	if i := c.indexOf(addr); i >= 0 {
		r = c.Recent[i]
		c.Recent = append(c.Recent[:i], c.Recent[i+1:]...)
	}
	r.LastAt = at
	c.Recent = append([]Recent{r}, c.Recent...)
	if len(c.Recent) > MaxRecent {
		c.Recent = c.Recent[:MaxRecent]
	}
}

// SetDuration records how long the latest session with addr lasted.
func (c *Config) SetDuration(addr string, d time.Duration) {
	if i := c.indexOf(addr); i >= 0 {
		c.Recent[i].Duration = int64(d / time.Second)
	}
}

// Forget removes addr from the recent list.
func (c *Config) Forget(addr string) {
	if i := c.indexOf(addr); i >= 0 {
		c.Recent = append(c.Recent[:i], c.Recent[i+1:]...)
	}
}

// LastAddr is the most recently used phone, or "" if there is none.
func (c *Config) LastAddr() string {
	if len(c.Recent) == 0 {
		return ""
	}
	return c.Recent[0].Addr
}

func (c *Config) indexOf(addr string) int {
	for i, r := range c.Recent {
		if r.Addr == addr {
			return i
		}
	}
	return -1
}
