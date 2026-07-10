package api

import (
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type applianceIPs struct {
	SourceIPs  []string `json:"sourceIPs"`
	LocalIPs   []string `json:"localIPs"`
	ExternalIP string   `json:"externalIP"`
}

func (s *Server) handleGetAppliance(w http.ResponseWriter, _ *http.Request) {
	dataDir := s.cfg.DataDir
	ips := detectApplianceIPs()
	writeJSON(w, http.StatusOK, map[string]any{
		"dataDir":       dataDir,
		"masterKeyPath": filepath.Join(dataDir, "secrets", "master.key"),
		"databasePath":  filepath.Join(dataDir, "app.db"),
		"backupsPath":   filepath.Join(dataDir, "backups"),
		"sourceIPs":     ips.SourceIPs,
		"localIPs":      ips.LocalIPs,
		"externalIP":    ips.ExternalIP,
	})
}

func detectApplianceIPs() applianceIPs {
	local := localInterfaceIPs()
	ext := fetchExternalIP()
	source := append([]string{}, local...)
	if ext != "" {
		seen := map[string]bool{}
		for _, ip := range source {
			seen[ip] = true
		}
		if !seen[ext] {
			source = append(source, ext)
		}
	}
	sort.Strings(source)
	return applianceIPs{SourceIPs: source, LocalIPs: local, ExternalIP: ext}
}

func localInterfaceIPs() []string {
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

func fetchExternalIP() string {
	client := &http.Client{Timeout: 3 * time.Second}
	for _, url := range []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
	} {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
}
