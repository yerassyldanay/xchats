package mcpauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
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
// clientIDURL. Unlike operator-configured knowledge-base imports, an OAuth
// client_id is controlled by an unauthenticated remote client, so CIMD never
// permits private, loopback, link-local, or otherwise non-public targets.
func FetchCIMD(ctx context.Context, clientIDURL string) (Client, error) {
	u, err := url.Parse(clientIDURL)
	if err != nil || u.Scheme != "https" {
		return Client{}, fmt.Errorf("mcpauth: client_id must be an https:// URL for CIMD, got %q", clientIDURL)
	}
	if err := safefetch.CheckURL(ctx, clientIDURL, false); err != nil {
		return Client{}, fmt.Errorf("mcpauth: unsafe client_id URL: %w", err)
	}

	client := safefetch.Client(false, 10*time.Second)
	body, resp, err := safefetch.GetJSON(ctx, client, clientIDURL, cimdMaxBodyBytes)
	if err != nil {
		if errors.Is(err, safefetch.ErrTooLarge) {
			return Client{}, errors.New("mcpauth: client metadata document too large")
		}
		return Client{}, fmt.Errorf("mcpauth: fetch client metadata: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Client{}, fmt.Errorf("mcpauth: client metadata fetch returned %d", resp.StatusCode)
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

// looksLikeCIMDClientID reports whether a client_id should be resolved via
// CIMD fetch rather than a DB lookup only — i.e. it is itself an https URL.
func looksLikeCIMDClientID(clientID string) bool {
	return strings.HasPrefix(clientID, "https://")
}
