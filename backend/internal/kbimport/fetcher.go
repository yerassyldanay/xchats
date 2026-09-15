package kbimport

import (
	"context"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
)

// URLFetcher is the narrow seam behind every operator-submitted-URL network
// call this package makes directly (Submit's own SSRF precheck, and
// downloadOneImage's fetch of an embedded image) — as opposed to
// resolveProvider's own extractor.Provider seam, which covers a
// vendor-hosted extraction call instead. Deps.Fetcher, when set (cmd/xchats'
// mock-externals composition root), replaces realURLFetcher below with an
// in-memory fake that never resolves DNS or dials out.
type URLFetcher interface {
	// CheckURL validates rawURL is safe to fetch (scheme, DNS resolution,
	// no private/loopback target unless allowPrivate) without fetching it —
	// the real implementation is exactly safefetch.CheckURL.
	CheckURL(ctx context.Context, rawURL string, allowPrivate bool) error
	// Get fetches rawURL, capped at maxBytes, using timeout as the
	// underlying client's own timeout — the real implementation wraps
	// safefetch.Client + safefetch.Get.
	Get(ctx context.Context, rawURL string, allowPrivate bool, timeout time.Duration, maxBytes int64) (data []byte, statusCode int, contentType string, err error)
}

// realURLFetcher is the production URLFetcher — a thin adapter over
// safefetch, preserving its exact SSRF-safe behavior unchanged.
type realURLFetcher struct{}

func (realURLFetcher) CheckURL(ctx context.Context, rawURL string, allowPrivate bool) error {
	return safefetch.CheckURL(ctx, rawURL, allowPrivate)
}

func (realURLFetcher) Get(ctx context.Context, rawURL string, allowPrivate bool, timeout time.Duration, maxBytes int64) ([]byte, int, string, error) {
	client := safefetch.Client(allowPrivate, timeout)
	data, resp, err := safefetch.Get(ctx, client, rawURL, maxBytes)
	if err != nil {
		return nil, 0, "", err
	}
	return data, resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

var _ URLFetcher = realURLFetcher{}

// fetcher returns Deps.Fetcher when set, else the real safefetch-backed
// implementation — the one place this package decides which to use.
func (s *Service) fetcher() URLFetcher {
	if s.deps.Fetcher != nil {
		return s.deps.Fetcher
	}
	return realURLFetcher{}
}
