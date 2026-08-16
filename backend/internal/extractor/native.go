package extractor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
)

// nativeProvider implements Provider using only the standard library plus
// golang.org/x/net/html — no vendored PDF/DOCX SDK (see package doc). It is
// always registered: unlike Firecrawl/LlamaParse it needs no credential.
type nativeProvider struct {
	client   *http.Client
	maxBytes int64
}

// NewNative builds the native provider. client should be built with
// safefetch.Client — native is the one provider that fetches a URL directly
// FROM THIS PROCESS, so it is the one provider the SSRF dialer guard
// actually protects (Firecrawl/LlamaParse fetch from their own network;
// see safefetch's package doc for that residual-risk boundary). maxBytes
// caps both a fetched page and a read file.
func NewNative(client *http.Client, maxBytes int64) Provider {
	return &nativeProvider{client: client, maxBytes: maxBytes}
}

func (n *nativeProvider) Name() string { return "native" }

// Supports implements the native row of the capability matrix
// (plan/playground.md): URL ✅, PDF ❌ (no vendored parser — see package
// doc), DOCX ✅ (archive/zip + encoding/xml), txt/md/csv ✅ (passthrough),
// image/* ✅ (no-op — the material itself is the artifact).
func (n *nativeProvider) Supports(src Source) bool {
	switch src.Kind {
	case SourceURL:
		return true
	case SourceFile:
		switch {
		case src.MimeType == MimePDF:
			return false
		case src.MimeType == MimeDOCX:
			return true
		case isTextlike(src.MimeType):
			return true
		case isImage(src.MimeType):
			return true
		}
	}
	return false
}

func (n *nativeProvider) Extract(ctx context.Context, src Source) (Result, error) {
	if !n.Supports(src) {
		return Result{}, ErrUnsupported
	}
	switch {
	case src.Kind == SourceURL:
		return n.extractURL(ctx, src.URL)
	case src.MimeType == MimeDOCX:
		text, err := extractDOCXText(src.Data)
		if err != nil {
			return Result{}, err
		}
		return Result{Text: text, Provider: n.Name()}, nil
	case isImage(src.MimeType):
		// The material IS the artifact — nothing to extract textually in
		// v1 (no vision call here; see plan/DECISIONS.md's "Vision gap").
		return Result{Provider: n.Name()}, nil
	default: // isTextlike
		return Result{Text: string(src.Data), Provider: n.Name()}, nil
	}
}

func (n *nativeProvider) extractURL(ctx context.Context, rawURL string) (Result, error) {
	data, resp, err := safefetch.Get(ctx, n.client, rawURL, n.maxBytes)
	if err != nil {
		return Result{}, fmt.Errorf("extractor: fetch %s: %w", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("extractor: fetch %s: unexpected status %d", rawURL, resp.StatusCode)
	}
	text, images, err := parseHTML(data, rawURL)
	if err != nil {
		return Result{}, err
	}
	return Result{Text: text, Images: images, Provider: n.Name()}, nil
}

// skipTextTags are elements whose text content is never page prose —
// script/style bodies are the classic false positive for a naive text
// walker, noscript/template bodies are markup fragments the browser never
// renders as-is.
var skipTextTags = map[string]bool{"script": true, "style": true, "noscript": true, "template": true}

// parseHTML walks doc (relative to baseURL, for resolving <img src>) and
// returns its visible text (script/style/etc. excluded) plus every <img>
// reference in document order.
func parseHTML(data []byte, baseURL string) (string, []Image, error) {
	base, _ := url.Parse(baseURL) // best-effort; a nil base just means relative img srcs are skipped
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("extractor: parse html: %w", err)
	}

	var text strings.Builder
	var images []Image
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipTextTags[n.Data] {
				return
			}
			if n.Data == "img" {
				if img, ok := imageFromAttrs(base, n.Attr); ok {
					images = append(images, img)
				}
			}
		}
		if n.Type == html.TextNode {
			if trimmed := strings.TrimSpace(n.Data); trimmed != "" {
				text.WriteString(trimmed)
				text.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(text.String()), images, nil
}

func imageFromAttrs(base *url.URL, attrs []html.Attribute) (Image, bool) {
	var src, alt string
	for _, a := range attrs {
		switch a.Key {
		case "src":
			src = a.Val
		case "alt":
			alt = a.Val
		}
	}
	if strings.TrimSpace(src) == "" {
		return Image{}, false
	}
	resolved := resolveURL(base, src)
	if resolved == "" {
		return Image{}, false
	}
	return Image{URL: resolved, Alt: alt}, true
}

// resolveURL resolves ref against base (an <img src> is very often
// relative to the page it came from); a ref that is already absolute
// resolves the same regardless of base. Returns "" for anything that ends
// up neither absolute nor resolvable (e.g. a bare "data:" URI, which
// images.go has no use for — it downloads by HTTP(S) URL only).
func resolveURL(base *url.URL, ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if u.IsAbs() {
		if u.Scheme != "http" && u.Scheme != "https" {
			return ""
		}
		return u.String()
	}
	if base == nil {
		return ""
	}
	resolved := base.ResolveReference(u)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

// extractDOCXText reads a .docx's word/document.xml (a DOCX is a ZIP
// archive of OOXML parts) and joins every <w:t> run's character data, with
// one newline per paragraph close (</w:p>) — legacy .doc is unsupported
// (plan/playground.md: "legacy .doc is unsupported").
func extractDOCXText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("extractor: open docx: %w", err)
	}
	var docXML *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}
	if docXML == nil {
		return "", errors.New("extractor: docx has no word/document.xml — not a valid .docx")
	}
	rc, err := docXML.Open()
	if err != nil {
		return "", fmt.Errorf("extractor: open docx document.xml: %w", err)
	}
	defer rc.Close()
	return extractDocumentXMLText(rc)
}

// extractDocumentXMLText walks document.xml's token stream directly
// (encoding/xml, standard library only — no vendored OOXML SDK) rather than
// unmarshalling into a typed struct: the schema has far more elements than
// the ones this cares about, and a token walk ignores everything else for
// free.
func extractDocumentXMLText(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var buf strings.Builder
	inRun := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("extractor: parse document.xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inRun = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inRun = false
			case "p":
				buf.WriteString("\n")
			}
		case xml.CharData:
			if inRun {
				buf.Write(t)
			}
		}
	}
	return strings.TrimSpace(buf.String()), nil
}
