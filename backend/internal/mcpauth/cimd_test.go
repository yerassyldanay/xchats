package mcpauth

import (
	"context"
	"errors"
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
