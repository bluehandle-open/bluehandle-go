package platform

import (
	"net/netip"
	"testing"
)

func TestPick(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("10.8.0.2"),
		netip.MustParseAddr("192.168.6.226"),
	}
	cases := map[string]string{
		"192.168.6.144:8088": "192.168.6.226", // same /24 wins
		"192.168.6.144":      "192.168.6.226",
		"172.20.1.1":         "10.8.0.2", // no match: first address
		"":                   "10.8.0.2",
	}
	for phone, want := range cases {
		if got := pick(addrs, phone); got != want {
			t.Errorf("pick(%q) = %q, want %q", phone, got, want)
		}
	}
	if pick(nil, "192.168.6.1") != "" {
		t.Error("no addresses must give empty string")
	}
}

func TestIsPrivate(t *testing.T) {
	for s, want := range map[string]bool{
		"192.168.6.226": true, "10.1.2.3": true, "172.31.0.1": true,
		"172.32.0.1": false, "198.18.0.1": false, "8.8.8.8": false, "169.254.1.1": false,
	} {
		if got := isPrivate(netip.MustParseAddr(s)); got != want {
			t.Errorf("isPrivate(%s) = %v", s, got)
		}
	}
}
