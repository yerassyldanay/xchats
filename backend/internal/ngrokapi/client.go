// Package ngrokapi contains the small account-API client xchats uses to
// discover the static domain assigned to an ngrok account. The API key used
// here is distinct from the agent authtoken used by internal/tunnel.
package ngrokapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.ngrok.com"
	requestTimeout = 10 * time.Second
	maxBodyBytes   = 1 << 20
)

var (
	ErrInvalidAPIKey     = errors.New("ngrok API key was rejected")
	ErrUnavailable       = errors.New("ngrok account API is unavailable")
	ErrNoReservedDomains = errors.New("ngrok account has no reserved domains")
)

// ReservedDomain is the account data needed to select a stable public
// hostname. No credential or certificate material is retained.
type ReservedDomain struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	CreatedAt string `json:"created_at"`
}

// Client lists domains from ngrok's account API. Production always uses the
// fixed ngrok API origin; newClient exists only for same-package HTTP tests.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return newClient(defaultBaseURL, &http.Client{Timeout: requestTimeout})
}

func newClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

// ListReservedDomains retrieves up to 100 account domains. ngrok plans expose
// far fewer than that in the normal xchats deployment; the explicit cap also
// bounds memory and response processing.
func (c *Client) ListReservedDomains(ctx context.Context, apiKey string) ([]ReservedDomain, error) {
	if c == nil || c.httpClient == nil || strings.TrimSpace(apiKey) == "" {
		return nil, ErrInvalidAPIKey
	}
	u, err := url.Parse(c.baseURL + "/reserved_domains")
	if err != nil {
		return nil, fmt.Errorf("%w: construct request", ErrUnavailable)
	}
	query := u.Query()
	query.Set("limit", "100")
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: construct request", ErrUnavailable)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("ngrok-version", "2")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed", ErrUnavailable)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrInvalidAPIKey
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > maxBodyBytes {
		return nil, fmt.Errorf("%w: read response", ErrUnavailable)
	}
	var payload struct {
		ReservedDomains []ReservedDomain `json:"reserved_domains"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: decode response", ErrUnavailable)
	}
	return payload.ReservedDomains, nil
}

// SelectDefaultDomain prefers ngrok's assigned free development domain and
// otherwise chooses the oldest reserved domain deterministically. The API has
// no explicit "default" flag, so this mirrors the account/product semantics.
func SelectDefaultDomain(domains []ReservedDomain) (string, error) {
	candidates := make([]ReservedDomain, 0, len(domains))
	for _, domain := range domains {
		domain.Domain = strings.TrimSpace(domain.Domain)
		if domain.Domain != "" {
			candidates = append(candidates, domain)
		}
	}
	if len(candidates) == 0 {
		return "", ErrNoReservedDomains
	}
	sort.Slice(candidates, func(i, j int) bool {
		iFree := strings.HasSuffix(strings.ToLower(candidates[i].Domain), ".ngrok-free.app")
		jFree := strings.HasSuffix(strings.ToLower(candidates[j].Domain), ".ngrok-free.app")
		if iFree != jFree {
			return iFree
		}
		if candidates[i].CreatedAt != candidates[j].CreatedAt {
			return candidates[i].CreatedAt < candidates[j].CreatedAt
		}
		return candidates[i].Domain < candidates[j].Domain
	})
	return candidates[0].Domain, nil
}
