package playground

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The URL adapter must refuse to fetch private/loopback hosts by default (SSRF):
// an httptest server binds 127.0.0.1, so the dial is blocked → needs_human.
func TestExtractURL_BlocksPrivateByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html><body>secret internal</body></html>"))
	}))
	defer srv.Close()

	ex := NewExtractor(nil, nil) // AllowPrivateHosts defaults to false
	got := ex.extractURL(context.Background(), srv.URL)
	if got.Status != "needs_human" {
		t.Fatalf("loopback fetch should be blocked → needs_human, got %q (%s)", got.Status, got.Extraction)
	}
}

// Non-http(s) schemes (file://, ftp://, gopher://) are rejected before any dial.
func TestExtractURL_RejectsNonHTTPScheme(t *testing.T) {
	ex := NewExtractor(nil, nil)
	for _, u := range []string{"file:///etc/passwd", "ftp://example.com/x", "gopher://x"} {
		if got := ex.extractURL(context.Background(), u); got.Status != "needs_human" {
			t.Fatalf("scheme %q should be rejected, got %q", u, got.Status)
		}
	}
}

// With the guard opted out, a loopback fetch succeeds (self-hosted / test path).
func TestExtractURL_AllowPrivateFetches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html><body><p>Тарифы</p></body></html>"))
	}))
	defer srv.Close()

	ex := NewExtractor(nil, nil)
	ex.AllowPrivateHosts = true
	got := ex.extractURL(context.Background(), srv.URL)
	if got.Status != "ready" {
		t.Fatalf("allowed loopback fetch should be ready, got %q (%s)", got.Status, got.Extraction)
	}
}
