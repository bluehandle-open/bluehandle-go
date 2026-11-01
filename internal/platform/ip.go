// Package platform holds the small OS integrations the desktop app needs:
// local addresses, elevation, and waiting for a previous instance to exit.
package platform

import (
	"net"
	"net/netip"
)

var (
	private = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	// VPN clients such as Cloudflare WARP use the benchmarking range; a phone
	// is never reachable there.
	benchmark = netip.MustParsePrefix("198.18.0.0/15")
)

// LANAddrs lists this machine's private IPv4 addresses on interfaces that
// are up, in interface order.
func LANAddrs() []netip.Addr {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []netip.Addr
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP)
			if !ok {
				continue
			}
			ip = ip.Unmap()
			if ip.Is4() && isPrivate(ip) {
				out = append(out, ip)
			}
		}
	}
	return out
}

// PreferredLANAddr picks the address to show the user: one in the same /24
// as phone if there is one, otherwise the first private address. It returns
// "" when the machine has no private IPv4 address.
func PreferredLANAddr(phone string) string {
	return pick(LANAddrs(), phone)
}

func pick(addrs []netip.Addr, phone string) string {
	if len(addrs) == 0 {
		return ""
	}
	if host, _, err := net.SplitHostPort(phone); err == nil {
		phone = host
	}
	if p, err := netip.ParseAddr(phone); err == nil {
		subnet, _ := p.Prefix(24)
		for _, a := range addrs {
			if subnet.Contains(a) {
				return a.String()
			}
		}
	}
	return addrs[0].String()
}

func isPrivate(ip netip.Addr) bool {
	if benchmark.Contains(ip) {
		return false
	}
	for _, p := range private {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
