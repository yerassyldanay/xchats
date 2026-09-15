package extractormock

import (
	"context"
	"errors"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
)

func TestRegistryHasAllThreeProviders(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{"native", "firecrawl", "llamaparse"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("registry missing provider %q", name)
		}
	}
}

// TestSupportsMatrixMatchesRealProviders locks the mock's capability matrix
// to the exact same cases the real providers' own tests assert (see
// native_test.go/firecrawl_test.go/llamaparse_test.go) — kbimport's own
// submit-time validation depends on Supports() agreeing with the real
// provider it stands in for.
func TestSupportsMatrixMatchesRealProviders(t *testing.T) {
	reg := NewRegistry()
	native, _ := reg.Get("native")
	firecrawl, _ := reg.Get("firecrawl")
	llamaparse, _ := reg.Get("llamaparse")

	cases := []struct {
		provider extractor.Provider
		src      extractor.Source
		want     bool
	}{
		{native, extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com"}, true},
		{native, extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF}, false},
		{native, extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimeDOCX}, true},
		{native, extractor.Source{Kind: extractor.SourceFile, MimeType: "text/plain"}, true},
		{native, extractor.Source{Kind: extractor.SourceFile, MimeType: "image/png"}, true},
		{native, extractor.Source{Kind: extractor.SourceFile, MimeType: "application/octet-stream"}, false},

		{firecrawl, extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com"}, true},
		{firecrawl, extractor.Source{Kind: extractor.SourceFile, MimeType: "application/pdf"}, false},

		{llamaparse, extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF}, true},
		{llamaparse, extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimeDOCX}, true},
		{llamaparse, extractor.Source{Kind: extractor.SourceFile, MimeType: "text/csv"}, true},
		{llamaparse, extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com"}, false},
		{llamaparse, extractor.Source{Kind: extractor.SourceFile, MimeType: "image/png"}, false},
	}
	for _, c := range cases {
		if got := c.provider.Supports(c.src); got != c.want {
			t.Errorf("%s.Supports(%+v) = %v, want %v", c.provider.Name(), c.src, got, c.want)
		}
	}
}

func TestExtractReturnsValidNonEmptyText(t *testing.T) {
	reg := NewRegistry()
	native, _ := reg.Get("native")
	res, err := native.Extract(context.Background(), extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com/page"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.Text == "" || res.Provider != "native" {
		t.Errorf("got %+v, want non-empty Text and Provider=native", res)
	}
}

func TestExtractRejectsUnsupportedSource(t *testing.T) {
	reg := NewRegistry()
	firecrawl, _ := reg.Get("firecrawl")
	_, err := firecrawl.Extract(context.Background(), extractor.Source{Kind: extractor.SourceFile, MimeType: "application/pdf"})
	if !errors.Is(err, extractor.ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestExtractRespectsCanceledContext(t *testing.T) {
	reg := NewRegistry()
	llamaparse, _ := reg.Get("llamaparse")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := llamaparse.Extract(ctx, extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF}); err == nil {
		t.Fatal("expected an error for an already-canceled context")
	}
}
