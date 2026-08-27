package tsdial

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

var (
	mu        sync.RWMutex
	socksAddr string // e.g. 127.0.0.1:1055; empty = direct
)

// SetSOCKS configures an optional SOCKS5 proxy used for Tailscale destinations.
// Pass empty to clear (kernel Tailscale / direct dial).
func SetSOCKS(addr string) {
	mu.Lock()
	socksAddr = strings.TrimSpace(addr)
	mu.Unlock()
}

// SOCKS returns the configured SOCKS5 address, if any.
func SOCKS() string {
	mu.RLock()
	defer mu.RUnlock()
	return socksAddr
}

// IsTailnetHost reports whether host is a Tailscale IP or MagicDNS name.
func IsTailnetHost(host string) bool {
	host = strings.TrimSpace(host)
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	lower := strings.ToLower(host)
	if strings.HasSuffix(lower, ".ts.net") {
		return true
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		// Tailscale CGNAT 100.64.0.0/10
		prefix, _ := netip.ParsePrefix("100.64.0.0/10")
		if prefix.Contains(ip) {
			return true
		}
		// Unique Local fd7a:115c:a1e0::/48 used by Tailscale IPv6
		v6, _ := netip.ParsePrefix("fd7a:115c:a1e0::/48")
		if v6.Contains(ip) {
			return true
		}
	}
	return false
}

// DialTimeout dials network/address, using SOCKS5 for Tailnet destinations when configured.
func DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return DialContext(ctx, network, address)
}

// DialContext dials with optional Tailscale SOCKS proxy.
func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	d := &net.Dialer{Timeout: 15 * time.Second}
	if !IsTailnetHost(host) {
		return d.DialContext(ctx, network, address)
	}
	socks := SOCKS()
	if socks == "" {
		// Kernel Tailscale (tailscale0) or no Tailscale — direct dial.
		return d.DialContext(ctx, network, address)
	}
	dialer, err := proxy.SOCKS5("tcp", socks, nil, proxy.FromEnvironment())
	if err != nil {
		return nil, err
	}
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, address)
	}
	type result struct {
		c   net.Conn
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := dialer.Dial(network, address)
		ch <- result{c, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.c, r.err
	}
}

// LocalForward listens on 127.0.0.1:0 and forwards to remoteAddr via DialContext.
// Used so mysql/mysqldump CLI can reach Tailnet hosts without SOCKS awareness.
func LocalForward(remoteAddr string) (localHost string, localPort int, cleanup func(), err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", 0, func() {}, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	done := make(chan struct{})
	go func() {
		for {
			local, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				rmt, err := DialTimeout("tcp", remoteAddr, 20*time.Second)
				if err != nil {
					return
				}
				defer rmt.Close()
				errc := make(chan struct{}, 2)
				go func() { _, _ = netCopy(rmt, c); errc <- struct{}{} }()
				go func() { _, _ = netCopy(c, rmt); errc <- struct{}{} }()
				<-errc
			}(local)
		}
	}()
	cleanup = func() {
		close(done)
		_ = ln.Close()
	}
	return "127.0.0.1", port, cleanup, nil
}

func netCopy(dst net.Conn, src net.Conn) (int64, error) {
	buf := make([]byte, 32*1024)
	var n int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			n += int64(nw)
			if ew != nil {
				return n, ew
			}
		}
		if er != nil {
			return n, er
		}
	}
}
