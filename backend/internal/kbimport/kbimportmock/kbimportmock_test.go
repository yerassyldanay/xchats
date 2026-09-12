package kbimportmock

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

func TestCheckURLAlwaysSucceeds(t *testing.T) {
	f := New()
	// A real safefetch.CheckURL would reject a private/loopback host unless
	// explicitly allowed — the mock never resolves DNS at all, so both a
	// public-looking and an obviously-private URL succeed identically.
	for _, u := range []string{"https://example.com/page", "http://169.254.169.254/latest/meta-data"} {
		if err := f.CheckURL(context.Background(), u, false); err != nil {
			t.Errorf("CheckURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestCheckURLRespectsCanceledContext(t *testing.T) {
	f := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.CheckURL(ctx, "https://example.com", false); err == nil {
		t.Fatal("expected an error for an already-canceled context")
	}
}

// TestGetReturnsAValidImage proves the mock's payload survives the EXACT
// downstream validation internal/kbimport's downloadOneImage runs it
// through — blob.MimeSanityCheck and kbstore.KindOfMime == "image" — so a
// mock-mode embedded-image import lands as a real, playable image material
// rather than being silently rejected.
func TestGetReturnsAValidImage(t *testing.T) {
	f := New()
	data, status, contentType, err := f.Get(context.Background(), "https://example.com/photo.png", false, time.Second, 1<<20)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if reason := blob.MimeSanityCheck(contentType, data); reason != "" {
		t.Fatalf("MimeSanityCheck rejected the mock payload: %s", reason)
	}
	if kind := kbstore.KindOfMime(contentType); kind != "image" {
		t.Fatalf("KindOfMime(%q) = %q, want image", contentType, kind)
	}
}

func TestGetRespectsMaxBytes(t *testing.T) {
	f := New()
	data, _, _, err := f.Get(context.Background(), "https://example.com/photo.png", false, time.Second, 10)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(data) > 10 {
		t.Fatalf("len(data) = %d, want <= 10 (maxBytes)", len(data))
	}
}

func TestGetRespectsCanceledContext(t *testing.T) {
	f := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := f.Get(ctx, "https://example.com", false, time.Second, 1<<20); err == nil {
		t.Fatal("expected an error for an already-canceled context")
	}
}
