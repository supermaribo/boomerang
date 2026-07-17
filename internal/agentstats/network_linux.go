//go:build linux

package agentstats

import (
	"os"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

// readDefaultRouteNet fills netIface / cumulative RX+TX for the default-route interface.
func readDefaultRouteNet(s *metrics.Sample) {
	iface := defaultRouteIface()
	if iface == "" {
		return
	}
	b, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return
	}
	rx, tx, ok := parseNetDevCounters(b, iface)
	if !ok {
		return
	}
	s.NetIface = iface
	s.NetRxBytes = rx
	s.NetTxBytes = tx
}

func defaultRouteIface() string {
	if b, err := os.ReadFile("/proc/net/route"); err == nil {
		if iface := parseProcRoute(b); iface != "" {
			return iface
		}
	}
	if b, err := os.ReadFile("/proc/net/ipv6_route"); err == nil {
		return parseProcRoute6(b)
	}
	return ""
}
