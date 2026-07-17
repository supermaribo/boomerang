package agentstats

import (
	"bufio"
	"strconv"
	"strings"
)

// parseProcRoute selects the lowest-metric IPv4 default route (destination 00000000).
func parseProcRoute(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	bestIface := ""
	bestMetric := uint64(^uint64(0))
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			if strings.HasPrefix(line, "Iface") {
				continue
			}
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		iface, dest, metricRaw := fields[0], fields[1], fields[6]
		if dest != "00000000" {
			continue
		}
		if iface == "" || iface == "*" || iface == "lo" {
			continue
		}
		metric, err := strconv.ParseUint(metricRaw, 10, 64)
		if err != nil {
			continue
		}
		if bestIface == "" || metric < bestMetric {
			bestIface = iface
			bestMetric = metric
		}
	}
	return bestIface
}

// parseProcRoute6 selects the lowest-metric IPv6 default route (destination ::/0).
// Format: dest(32hex) dest_prefix src(32hex) src_prefix gateway(32hex) metric refcnt use flags iface
func parseProcRoute6(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	bestIface := ""
	bestMetric := uint64(^uint64(0))
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		dest, prefixRaw, metricRaw, iface := fields[0], fields[1], fields[5], fields[9]
		if !isZeroIPv6Hex(dest) {
			continue
		}
		prefix, err := strconv.ParseUint(prefixRaw, 16, 8)
		if err != nil || prefix != 0 {
			continue
		}
		if iface == "" || iface == "*" || iface == "lo" {
			continue
		}
		metric, err := strconv.ParseUint(metricRaw, 16, 64)
		if err != nil {
			continue
		}
		if bestIface == "" || metric < bestMetric {
			bestIface = iface
			bestMetric = metric
		}
	}
	return bestIface
}

func isZeroIPv6Hex(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range s {
		if r != '0' {
			return false
		}
	}
	return true
}

func parseNetDevCounters(data []byte, iface string) (rx, tx uint64, ok bool) {
	if len(data) == 0 || iface == "" {
		return 0, 0, false
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		if name != iface {
			continue
		}
		rest := strings.TrimSpace(line[colon+1:])
		fields := strings.Fields(rest)
		// rx bytes, packets, errs, drop, fifo, frame, compressed, multicast,
		// tx bytes, packets, ...
		if len(fields) < 9 {
			return 0, 0, false
		}
		rx, err1 := strconv.ParseUint(fields[0], 10, 64)
		tx, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return rx, tx, true
	}
	return 0, 0, false
}
