// Package kbimportmock is an in-memory kbimport.URLFetcher for cmd/xchats'
// mock-externals mode — CheckURL always succeeds (no DNS resolution, no
// SSRF check — there is nothing to protect against when nothing is
// actually fetched) and Get always answers with a small valid PNG,
// regardless of the requested URL, so internal/kbimport's own downstream
// image validation (blob.MimeSanityCheck, kbstore.KindOfMime == "image")
// passes exactly as it would for a real image download.
package kbimportmock

import (
	"context"
	"net/http"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/kbimport"
)

// mockPNG is a minimal valid 1x1 transparent PNG.
var mockPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

// Fetcher is a ready-to-use, stateless mock — a zero value works.
type Fetcher struct{}

// New returns a ready-to-use mock URLFetcher.
func New() Fetcher { return Fetcher{} }

var _ kbimport.URLFetcher = Fetcher{}

func (Fetcher) CheckURL(ctx context.Context, rawURL string, allowPrivate bool) error {
	return ctx.Err()
}

func (Fetcher) Get(ctx context.Context, rawURL string, allowPrivate bool, timeout time.Duration, maxBytes int64) ([]byte, int, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, "", err
	}
	data := mockPNG
	if int64(len(data)) > maxBytes {
		data = data[:maxBytes]
	}
	return data, http.StatusOK, "image/png", nil
}
