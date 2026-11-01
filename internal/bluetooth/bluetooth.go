// Package bluetooth dials the phone over RFCOMM. The phone listens with the
// standard serial-port UUID (see docs/protocol.md); once connected it sends
// the handshake line and then the same frames as over Wi-Fi.
//
// Addresses are written "bt:AA:BB:CC:DD:EE:FF" so they can share the address
// field, the recent list and the config file with IP addresses.
package bluetooth

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Prefix marks an address as Bluetooth.
const Prefix = "bt:"

var (
	// ErrUnsupported is returned on platforms without an implementation.
	ErrUnsupported = errors.New("bluetooth: not supported on this platform")
	// ErrNoRadio means the computer has no Bluetooth adapter or it is off.
	ErrNoRadio = errors.New("bluetooth: no Bluetooth adapter is turned on")
)

// Device is a remembered (paired) device.
type Device struct {
	Name      string `json:"name"`
	Addr      string `json:"addr"`      // "bt:AA:BB:CC:DD:EE:FF"
	Phone     bool   `json:"phone"`     // device class says it is a phone
	Connected bool   `json:"connected"` // Windows currently has a link to it
}

// IsAddr reports whether s is a Bluetooth address in our notation.
func IsAddr(s string) bool {
	_, err := ParseAddr(s)
	return err == nil
}

// ParseAddr turns "bt:AA:BB:CC:DD:EE:FF" (case-insensitive, ":" or "-"
// between bytes) into the 48-bit value Windows uses.
func ParseAddr(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if len(s) < len(Prefix) || !strings.EqualFold(s[:len(Prefix)], Prefix) {
		return 0, fmt.Errorf("bluetooth: %q is not a bt: address", s)
	}
	parts := strings.FieldsFunc(s[len(Prefix):], func(r rune) bool { return r == ':' || r == '-' })
	if len(parts) != 6 {
		return 0, fmt.Errorf("bluetooth: %q needs six bytes", s)
	}
	var v uint64
	for _, p := range parts {
		b, err := strconv.ParseUint(p, 16, 8)
		if err != nil || len(p) != 2 {
			return 0, fmt.Errorf("bluetooth: bad byte %q in %q", p, s)
		}
		v = v<<8 | b
	}
	return v, nil
}

// FormatAddr is the inverse of ParseAddr.
func FormatAddr(v uint64) string {
	var b strings.Builder
	b.WriteString(Prefix)
	for i := 5; i >= 0; i-- {
		fmt.Fprintf(&b, "%02X", byte(v>>(8*i)))
		if i > 0 {
			b.WriteByte(':')
		}
	}
	return b.String()
}

// sortDevices puts phones first, then devices Windows is linked to, then
// alphabetical, so the phone the user wants is usually on top.
func sortDevices(ds []Device) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.Phone != b.Phone {
			return a.Phone
		}
		if a.Connected != b.Connected {
			return a.Connected
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

// addr is the net.Addr of an RFCOMM endpoint.
type addr uint64

func (addr) Network() string  { return "bluetooth" }
func (a addr) String() string { return FormatAddr(uint64(a)) }
