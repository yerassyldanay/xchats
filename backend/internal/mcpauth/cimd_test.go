package mcpauth

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
)

func TestFetchCIMDRejectsNonPublicTargets(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "IPv4 loopback", url: "https://127.0.0.1/client.json"},
		{name: "IPv6 loopback", url: "https://[::1]/client.json"},
		{name: "private network", url: "https://10.0.0.1/client.json"},
		{name: "cloud metadata", url: "https://169.254.169.254/latest/meta-data/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := FetchCIMD(context.Background(), tt.url)
			if !errors.Is(err, safefetch.ErrBlockedHost) {
				t.Fatalf("FetchCIMD(%q) error = %v, want ErrBlockedHost", tt.url, err)
			}
		})
	}
}

func TestFetchCIMDRejectsNonHTTPSURL(t *testing.T) {
	t.Parallel()
	if _, err := FetchCIMD(context.Background(), "http://example.com/client.json"); err == nil {
		t.Fatal("FetchCIMD accepted an http URL")
	}
}

func TestValidateCIMDRedirectURI(t *testing.T) {
	t.Parallel()
	clientURL, err := url.Parse("https://client.example.com/oauth/client.json")
	if err != nil {
		t.Fatalf("parse client URL: %v", err)
	}

	tests := []struct {
		name        string
		clientURL   string
		redirectURI string
		wantErr     bool
	}{
		{
			name:        "exact origin match",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "https://client.example.com/oauth/callback",
			wantErr:     false,
		},
		{
			name:        "case-insensitive host match",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "https://CLIENT.EXAMPLE.COM/oauth/callback",
			wantErr:     false,
		},
		{
			name:        "explicit default port matches omitted port",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "https://client.example.com:443/oauth/callback",
			wantErr:     false,
		},
		{
			name:        "explicit custom port matches",
			clientURL:   "https://client.example.com:8443/oauth/client.json",
			redirectURI: "https://client.example.com:8443/oauth/callback",
			wantErr:     false,
		},
		{
			name:        "loopback localhost http allowed",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "http://localhost:8080/callback",
			wantErr:     false,
		},
		{
			name:        "loopback IPv4 http allowed",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "http://127.0.0.1:3000/callback",
			wantErr:     false,
		},
		{
			name:        "loopback IPv6 http allowed",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "http://[::1]:9000/callback",
			wantErr:     false,
		},
		{
			name:        "different origin rejected",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "https://attacker.example.com/callback",
			wantErr:     true,
		},
		{
			name:        "different port rejected",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "https://client.example.com:8443/callback",
			wantErr:     true,
		},
		{
			name:        "plain http on public host rejected",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "http://client.example.com/callback",
			wantErr:     true,
		},
		{
			name:        "invalid URI rejected",
			clientURL:   "https://client.example.com/oauth/client.json",
			redirectURI: "::not a uri::",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cURL := clientURL
			if tt.clientURL != "" {
				var parseErr error
				cURL, parseErr = url.Parse(tt.clientURL)
				if parseErr != nil {
					t.Fatalf("parse test clientURL: %v", parseErr)
				}
			}
			err := validateCIMDRedirectURI(cURL, tt.redirectURI)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateCIMDRedirectURI(%q, %q) err = %v, wantErr = %v", cURL.String(), tt.redirectURI, err, tt.wantErr)
			}
		})
	}
}

