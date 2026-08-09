// Package extractor is the pass-1 provider seam (plan/playground.md "Per
// input type"): turning one submitted URL or file into plain evidence text
// plus a list of embedded image references, without deciding anything about
// KB structure (that is pass 2's job — internal/kbimport/synth.go). Every
// Provider emits the exact same Result shape regardless of source type, so
// pass 2 stays type-blind and upgrading one provider never touches
// synthesis.
//
// No PDF or DOCX library is vendored here — DOCX is read with only
// archive/zip + encoding/xml (both standard library), and PDF text
// extraction is deliberately UNSUPPORTED by the native provider (see
// native.go): vendoring a PDF library would trigger the `make notices` /
// THIRD_PARTY_LICENSES.txt pipeline for a v1 feature that already has a
// working alternative (LlamaParse). golang.org/x/net/html is already an
// indirect module dependency; this package is what promotes it to direct.
package extractor

import (
	"context"
	"errors"
	"strings"
)

// SourceKind classifies what a Source names.
type SourceKind string

const (
	// SourceURL means Source.URL names a page to fetch.
	SourceURL SourceKind = "url"
	// SourceFile means Source.Data already holds the material's bytes
	// (staged via the same upload path every KB file upload uses).
	SourceFile SourceKind = "file"
)

// The two file MIME types this package has explicit, named handling for —
// every other MIME type is matched structurally (isTextlike/isImage) rather
// than enumerated.
const (
	MimePDF  = "application/pdf"
	MimeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// Source is one material to extract: either a URL to fetch, or a file's
// already-staged bytes with a known MIME type. Kind determines which fields
// apply.
type Source struct {
	Kind SourceKind

	// URL is the page to fetch — set iff Kind == SourceURL.
	URL string

	// Filename/MimeType/Data describe an already-staged file's bytes — set
	// iff Kind == SourceFile. MimeType is the material's detected/declared
	// MIME type (kbd_materials.mime_type), the value every Provider's
	// Supports/Extract dispatch on.
	Filename string
	MimeType string
	Data     []byte
}

// Image is one image reference discovered during extraction — a page's
// <img src>, or a ![alt](url) reference in a provider's markdown output.
// This is a REFERENCE only, not yet downloaded: internal/kbimport/images.go
// resolves it (against the source URL when relative), fetches it through
// safefetch, and stages it as its own kbd_materials row.
type Image struct {
	URL string
	Alt string
}

// Result is one material's extracted evidence — the common shape every
// Provider emits, so pass 2 never special-cases a source type
// (plan/playground.md: "All extractors emit the same evidence shape").
type Result struct {
	// Text is the extracted plain text or markdown, ready for pass 2's
	// prompt. Empty for an image source: the material itself IS the
	// artifact (native.go), there is nothing to extract textually — v1 has
	// no vision call here (plan/DECISIONS.md's "Vision gap" risk).
	Text string
	// Images are the embedded images this extraction discovered, in
	// document order. Never populated for a file source that is itself an
	// image.
	Images []Image
	// Provider records which Provider produced this Result (its Name()) —
	// carried into extraction_metadata for the run's audit trail.
	Provider string
}

// ErrUnsupported means a Provider cannot extract the given Source AT ALL —
// e.g. the native provider asked to read a PDF. A caller either tries a
// different provider or, at submit time (internal/httpapi/kb_import.go's
// precheck), rejects the request outright with a 400 rather than an async
// job failure.
var ErrUnsupported = errors.New("extractor: unsupported source for this provider")

// ErrProviderAuth means a vendor (Firecrawl/LlamaParse) rejected the
// request as unauthorized (401/403) — an invalid or revoked API key, as
// opposed to a transient network error or a content problem. Callers use
// errors.Is(err, ErrProviderAuth) to surface "this integration needs a new
// key" distinctly, mirroring llm.ErrProviderAuth's exact role for model
// calls.
var ErrProviderAuth = errors.New("extractor: provider rejected the request as unauthorized")

// Provider extracts a Result from a Source. Implementations must be safe
// for concurrent use — internal/kbimport runs several extraction jobs in
// parallel against the same Registry.
type Provider interface {
	// Name is the provider id used in extraction_metadata and in the
	// operator-facing provider dropdown ("native", "firecrawl",
	// "llamaparse").
	Name() string
	// Supports reports whether this provider can extract src at all — the
	// capability matrix (plan/playground.md's per-input-type table). Cheap
	// and side-effect-free: called at submit time for every candidate
	// provider, before any network activity.
	Supports(src Source) bool
	// Extract performs the extraction. Returns ErrUnsupported if Supports
	// would have reported false — callers should still check Supports
	// first to fail fast without a wasted round trip.
	Extract(ctx context.Context, src Source) (Result, error)
}

// Registry resolves a configured Provider by name — built once at the
// composition root (internal/kbimport's Service) from whichever providers
// have a usable configuration (native is always present; firecrawl/
// llamaparse only when their credential resolves).
type Registry struct {
	byName map[string]Provider
	order  []string
}

// NewRegistry builds a Registry from providers, in the given order —
// duplicate names overwrite earlier entries (last one wins), which is
// never expected to happen in practice but is not itself an error.
func NewRegistry(providers ...Provider) *Registry {
	r := &Registry{byName: make(map[string]Provider, len(providers))}
	for _, p := range providers {
		if _, exists := r.byName[p.Name()]; !exists {
			r.order = append(r.order, p.Name())
		}
		r.byName[p.Name()] = p
	}
	return r
}

// Get resolves a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.byName[name]
	return p, ok
}

// Names returns every registered provider's name, in registration order.
func (r *Registry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// isTextlike reports whether mime is a plain-text-ish format the native
// provider passes through verbatim (txt/md/csv — plan/playground.md's
// "txt/md/csv" row). Deliberately structural (any text/* subtype) rather
// than an exhaustive enum, plus the one application/* alias csv sometimes
// arrives as.
func isTextlike(mime string) bool {
	return strings.HasPrefix(mime, "text/") || mime == "application/csv"
}

// isImage reports whether mime is any image/* subtype.
func isImage(mime string) bool { return strings.HasPrefix(mime, "image/") }

// truncate bounds an error/log snippet to n bytes, appending an ellipsis
// marker when it cuts — used when echoing a vendor's raw response body into
// an error message, so a misbehaving vendor can never inflate an error
// string without bound.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
