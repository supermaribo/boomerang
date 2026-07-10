package api

import (
	"net"
	"net/http"
	"strings"
	"time"
)

func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) allowSetup(ip string) bool {
	const max = 10
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cut := now.Add(-15 * time.Minute)
	var kept []time.Time
	for _, t := range s.setupN[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		s.setupN[ip] = kept
		return false
	}
	s.setupN[ip] = append(kept, now)
	return true
}
