// Package publicendpoint owns the canonical public origin advertised by the
// MCP connector. A deployment may have a fixed HTTPS origin, an embedded
// ngrok tunnel, or both; tunnel state wins while it is active and falls back
// to the configured origin when it stops.
package publicendpoint

import (
	"net"
	"net/url"
	"strings"
	"sync"
)

// Endpoint is a concurrency-safe, runtime-updatable public MCP address.
type Endpoint struct {
	mu           sync.RWMutex
	configured   string
	tunnelOrigin string
}

// Snapshot is safe to expose to an authenticated UI. It deliberately carries
// no credentials or internal bind addresses.
type Snapshot struct {
	Ready  bool   `json:"ready"`
	Origin string `json:"origin,omitempty"`
	MCPURL string `json:"mcp_url,omitempty"`
	Source string `json:"source"` // configured | tunnel | unavailable
}

func New(configuredOrigin string) *Endpoint {
	return &Endpoint{configured: cleanOrigin(configuredOrigin)}
}

func (e *Endpoint) SetTunnelURL(publicURL string) {
	e.mu.Lock()
	e.tunnelOrigin = cleanOrigin(publicURL)
	e.mu.Unlock()
}

func (e *Endpoint) ClearTunnelURL() {
	e.mu.Lock()
	e.tunnelOrigin = ""
	e.mu.Unlock()
}

func (e *Endpoint) Origin() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.tunnelOrigin != "" {
		return e.tunnelOrigin
	}
	return e.configured
}

func (e *Endpoint) MCPURL() string {
	origin := e.Origin()
	if origin == "" {
		return ""
	}
	return origin + "/mcp"
}

func (e *Endpoint) Snapshot() Snapshot {
	e.mu.RLock()
	origin := e.configured
	source := "configured"
	if e.tunnelOrigin != "" {
		origin = e.tunnelOrigin
		source = "tunnel"
	}
	e.mu.RUnlock()
	if !publicHTTPS(origin) {
		return Snapshot{Source: "unavailable"}
	}
	return Snapshot{Ready: true, Origin: origin, MCPURL: origin + "/mcp", Source: source}
}

func cleanOrigin(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func publicHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()) {
		return false
	}
	return true
}
