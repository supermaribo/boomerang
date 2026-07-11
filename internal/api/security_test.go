package api

import (
	"net/http/httptest"
	"testing"

	"github.com/boomerang-backup/boomerang/internal/config"
)

func TestSameOriginRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/login", nil)
	req.Host = "backup.local:8080"
	req.Header.Set("Origin", "http://backup.local:8080")
	if !sameOriginRequest(req) {
		t.Fatal("expected same origin")
	}
	req.Header.Set("Origin", "http://evil.example")
	if sameOriginRequest(req) {
		t.Fatal("expected different origin rejected")
	}
	req.Header.Del("Origin")
	if !sameOriginRequest(req) {
		t.Fatal("expected missing origin allowed")
	}
}

func TestClientIPWithoutTrustProxy(t *testing.T) {
	s := &Server{cfg: &config.Config{TrustProxy: false}}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.10:54321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := s.clientIP(req); got != "192.168.1.10" {
		t.Fatalf("got %q", got)
	}
}
