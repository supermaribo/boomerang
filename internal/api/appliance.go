package api

import (
	"net"
	"net/http"
	"path/filepath"
	"sort"
)

func (s *Server) handleGetAppliance(w http.ResponseWriter, _ *http.Request) {
	dataDir := s.cfg.DataDir
	writeJSON(w, http.StatusOK, map[string]any{
		"dataDir":      dataDir,
		"masterKeyPath": filepath.Join(dataDir, "secrets", "master.key"),
		"databasePath": filepath.Join(dataDir, "app.db"),
		"backupsPath":  filepath.Join(dataDir, "backups"),
		"sourceIPs":    applianceSourceIPs(),
	})
}

// applianceSourceIPs returns non-loopback addresses targets should allow in their firewall.
func applianceSourceIPs() []string {
	seen := map[string]bool{}
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			s := ip.String()
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sort.Strings(out)
	return out
}
