package mcpauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// cimdMaxBodyBytes bounds a fetched Client ID Metadata Document — this is a
// small JSON object, never a reason to stream an unbounded response.
const cimdMaxBodyBytes = 64 * 1024

// cimdDocument is a Client ID Metadata Document (plan/mcp.md §3: "Support
// Client ID Metadata Documents") — an RFC 7591-shaped client metadata object
// served AT the client_id URL itself, so a host can self-register by simply
// naming an https:// URL as its client_id instead of calling
// POST /oauth/register first. Field names mirror RFC 7591 §2 exactly; this
// is the newest piece of the MCP auth ecosystem, so treat the exact shape as
// best-effort pending a real interop pass against ChatGPT/Claude (plan/mcp.md
// §9's MCP Inspector step).
type cimdDocument struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

// FetchCIMD retrieves and validates the Client ID Metadata Document at
// clientIDURL. allowPrivateHosts mirrors internal/playground's URL-adapter
// SSRF guard (KBAllowPrivateFetch) — false in every real deployment, true
// only for a trusted self-hosted setup fetching its own loopback tooling.
func FetchCIMD(ctx context.Context, clientIDURL string, allowPrivateHosts bool) (Client, error) {
	u, err := url.Parse(clientIDURL)
	if err != nil || u.Scheme != "https" {
		return Client{}, fmt.Errorf("mcpauth: client_id must be an https:// URL for CIMD, got %q", clientIDURL)
	}

	client := safeHTTPClient(allowPrivateHosts, 10*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientIDURL, nil)
	if err != nil {
		return Client{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return Client{}, fmt.Errorf("mcpauth: fetch client metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Client{}, fmt.Errorf("mcpauth: client metadata fetch returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, cimdMaxBodyBytes+1))
	if err != nil {
		return Client{}, err
	}
	if len(body) > cimdMaxBodyBytes {
		return Client{}, errors.New("mcpauth: client metadata document too large")
	}
	var doc cimdDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return Client{}, fmt.Errorf("mcpauth: parse client metadata: %w", err)
	}
	if len(doc.RedirectURIs) == 0 {
		return Client{}, errors.New("mcpauth: client metadata document has no redirect_uris")
	}
	for _, ru := range doc.RedirectURIs {
		if err := validateRedirectURI(ru); err != nil {
			return Client{}, fmt.Errorf("mcpauth: client metadata redirect_uris: %w", err)
		}
	}
	return Client{
		ClientID: clientIDURL, ClientName: doc.ClientName, RedirectURIs: doc.RedirectURIs, Source: "cimd",
	}, nil
}

// validateRedirectURI enforces https, except for loopback http (127.0.0.1,
// ::1, localhost) — the native-app/CLI/MCP-Inspector pattern OAuth 2.1
// explicitly still allows over plain http, since "localhost" never leaves
// the user's own machine.
func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid redirect_uri %q", raw)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("redirect_uri %q must be https, or http on a loopback host", raw)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// safeHTTPClient builds an SSRF-hardened client: a net.Dialer.Control hook
// vets the *resolved* IP of every connection (initial and each redirect), so
// a hostname resolving to a private/loopback/link-local address — or
// rebinding to one mid-fetch — is refused before any bytes flow. This is the
// same guard internal/playground's URL adapter applies to KB material
// fetches, reimplemented here rather than imported: an OAuth client
// registration fetch and a KB material fetch are different trust boundaries
// that happen to want the same primitive.
func safeHTTPClient(allowPrivateHosts bool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if allowPrivateHosts {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("blocked non-public address %s", address)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			if req.URL.Scheme != "https" {
				return fmt.Errorf("blocked redirect to %q scheme", req.URL.Scheme)
			}
			return nil
		},
	}
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast())
}

// looksLikeCIMDClientID reports whether a client_id should be resolved via
// CIMD fetch rather than a DB lookup only — i.e. it is itself an https URL.
func looksLikeCIMDClientID(clientID string) bool {
	return strings.HasPrefix(clientID, "https://")
}
