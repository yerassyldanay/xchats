package ngrokapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientListReservedDomains(t *testing.T) {
	const apiKey = "test-api-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/reserved_domains" {
			t.Errorf("path = %q, want /reserved_domains", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Errorf("limit = %q, want 100", r.URL.Query().Get("limit"))
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Errorf("Authorization = %q, want bearer test key", got)
		}
		if got := r.Header.Get("ngrok-version"); got != "2" {
			t.Errorf("ngrok-version = %q, want 2", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reserved_domains":[{"id":"rd_1","domain":"stable.ngrok-free.app","created_at":"2024-08-23T12:48:24Z"}]}`))
	}))
	defer srv.Close()

	client := newClient(srv.URL, srv.Client())
	domains, err := client.ListReservedDomains(context.Background(), apiKey)
	if err != nil {
		t.Fatalf("ListReservedDomains: %v", err)
	}
	if len(domains) != 1 || domains[0].ID != "rd_1" || domains[0].Domain != "stable.ngrok-free.app" {
		t.Fatalf("domains = %#v, want stable.ngrok-free.app", domains)
	}
}

func TestClientListReservedDomainsClassifiesFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       error
	}{
		{name: "unauthorized key", statusCode: http.StatusUnauthorized, body: `{"error":"no"}`, want: ErrInvalidAPIKey},
		{name: "forbidden key", statusCode: http.StatusForbidden, body: `{"error":"no"}`, want: ErrInvalidAPIKey},
		{name: "server error", statusCode: http.StatusInternalServerError, body: `{"error":"down"}`, want: ErrUnavailable},
		{name: "invalid response", statusCode: http.StatusOK, body: `{`, want: ErrUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := newClient(srv.URL, srv.Client()).ListReservedDomains(context.Background(), "secret-not-for-errors")
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), "secret-not-for-errors") {
				t.Fatal("error contains the API key")
			}
		})
	}
}

func TestSelectDefaultDomain(t *testing.T) {
	tests := []struct {
		name    string
		domains []ReservedDomain
		want    string
		wantErr error
	}{
		{
			name:    "single assigned domain",
			domains: []ReservedDomain{{Domain: "stable.ngrok-free.app"}},
			want:    "stable.ngrok-free.app",
		},
		{
			name: "assigned development domain wins",
			domains: []ReservedDomain{
				{Domain: "custom.example.com", CreatedAt: "2023-01-01T00:00:00Z"},
				{Domain: "assigned.ngrok-free.app", CreatedAt: "2024-01-01T00:00:00Z"},
			},
			want: "assigned.ngrok-free.app",
		},
		{
			name: "oldest domain is deterministic fallback",
			domains: []ReservedDomain{
				{Domain: "new.ngrok.app", CreatedAt: "2025-01-01T00:00:00Z"},
				{Domain: "old.ngrok.app", CreatedAt: "2024-01-01T00:00:00Z"},
			},
			want: "old.ngrok.app",
		},
		{name: "empty account", wantErr: ErrNoReservedDomains},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectDefaultDomain(tt.domains)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("domain = %q, want %q", got, tt.want)
			}
		})
	}
}
