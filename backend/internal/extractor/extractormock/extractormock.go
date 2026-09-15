// Package extractormock provides in-memory extractor.Provider
// implementations for cmd/xchats' mock-externals mode — one per real
// provider (native/firecrawl/llamaparse), each mirroring that real
// provider's own Supports() capability matrix exactly (see native.go/
// firecrawl.go/llamaparse.go), so internal/kbimport's own submit-time
// provider/mime validation (enqueue.go) behaves identically whether or not
// mock externals is on; only Extract's own network call is replaced.
package extractormock

import (
	"context"
	"fmt"
	"strings"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
)

// isTextlike mirrors extractor's own unexported isTextlike exactly
// (txt/md/csv-shaped mime types) — small enough that duplicating it here
// beats exporting an internal helper solely for this mock's sake.
func isTextlike(mime string) bool {
	return strings.HasPrefix(mime, "text/") || mime == "application/csv"
}

// NewRegistry returns an *extractor.Registry with all three mock providers
// registered — the cmd/xchats composition root hands this straight to
// kbimport.Deps.Extractors.
func NewRegistry() *extractor.Registry {
	return extractor.NewRegistry(nativeProvider{}, firecrawlProvider{}, llamaparseProvider{})
}

// --- native -----------------------------------------------------------

type nativeProvider struct{}

func (nativeProvider) Name() string { return "native" }

func (nativeProvider) Supports(src extractor.Source) bool {
	switch src.Kind {
	case extractor.SourceURL:
		return true
	case extractor.SourceFile:
		return src.MimeType == extractor.MimeDOCX || isTextlike(src.MimeType) || extractor.IsImage(src.MimeType)
	default:
		return false
	}
}

func (p nativeProvider) Extract(ctx context.Context, src extractor.Source) (extractor.Result, error) {
	if !p.Supports(src) {
		return extractor.Result{}, extractor.ErrUnsupported
	}
	if err := ctx.Err(); err != nil {
		return extractor.Result{}, err
	}
	if src.Kind == extractor.SourceURL {
		return extractor.Result{
			Text:     "Mock extracted page content for " + src.URL + " (mock-externals native provider).",
			Provider: "native",
		}, nil
	}
	return extractor.Result{
		Text:     fmt.Sprintf("Mock extracted content for uploaded file %q (mock-externals native provider).", src.Filename),
		Provider: "native",
	}, nil
}

// --- firecrawl ----------------------------------------------------------

type firecrawlProvider struct{}

func (firecrawlProvider) Name() string { return "firecrawl" }

func (firecrawlProvider) Supports(src extractor.Source) bool { return src.Kind == extractor.SourceURL }

func (p firecrawlProvider) Extract(ctx context.Context, src extractor.Source) (extractor.Result, error) {
	if !p.Supports(src) {
		return extractor.Result{}, extractor.ErrUnsupported
	}
	if err := ctx.Err(); err != nil {
		return extractor.Result{}, err
	}
	return extractor.Result{
		Text:     "# Mock Firecrawl Extraction\n\nMarkdown content for " + src.URL + " (mock-externals).",
		Provider: "firecrawl",
	}, nil
}

// --- llamaparse -----------------------------------------------------------

type llamaparseProvider struct{}

func (llamaparseProvider) Name() string { return "llamaparse" }

func (llamaparseProvider) Supports(src extractor.Source) bool {
	if src.Kind != extractor.SourceFile {
		return false
	}
	switch src.MimeType {
	case extractor.MimePDF, extractor.MimeDOCX:
		return true
	}
	return isTextlike(src.MimeType)
}

func (p llamaparseProvider) Extract(ctx context.Context, src extractor.Source) (extractor.Result, error) {
	if !p.Supports(src) {
		return extractor.Result{}, extractor.ErrUnsupported
	}
	if err := ctx.Err(); err != nil {
		return extractor.Result{}, err
	}
	return extractor.Result{
		Text:     fmt.Sprintf("Mock LlamaParse extraction for %q (mock-externals).", src.Filename),
		Provider: "llamaparse",
	}, nil
}
