package extractor_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
)

func testClient() *http.Client { return safefetch.Client(true, 5*time.Second) }

// --- Supports matrix ---------------------------------------------------------

func TestNative_Supports(t *testing.T) {
	n := extractor.NewNative(testClient(), 1<<20)
	cases := []struct {
		name string
		src  extractor.Source
		want bool
	}{
		{"url", extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com"}, true},
		{"pdf", extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF}, false},
		{"docx", extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimeDOCX}, true},
		{"text/plain", extractor.Source{Kind: extractor.SourceFile, MimeType: "text/plain"}, true},
		{"text/markdown", extractor.Source{Kind: extractor.SourceFile, MimeType: "text/markdown"}, true},
		{"text/csv", extractor.Source{Kind: extractor.SourceFile, MimeType: "text/csv"}, true},
		{"image/png", extractor.Source{Kind: extractor.SourceFile, MimeType: "image/png"}, true},
		{"image/jpeg", extractor.Source{Kind: extractor.SourceFile, MimeType: "image/jpeg"}, true},
		{"unknown binary", extractor.Source{Kind: extractor.SourceFile, MimeType: "application/octet-stream"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := n.Supports(c.src); got != c.want {
				t.Errorf("Supports(%+v) = %v, want %v", c.src, got, c.want)
			}
		})
	}
}

func TestNative_Extract_PDF_ErrUnsupported(t *testing.T) {
	n := extractor.NewNative(testClient(), 1<<20)
	_, err := n.Extract(context.Background(), extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF, Data: []byte("%PDF-1.4")})
	if !errors.Is(err, extractor.ErrUnsupported) {
		t.Fatalf("Extract(pdf) err = %v, want ErrUnsupported", err)
	}
}

// --- URL extraction: text + images -------------------------------------------

func TestNative_ExtractURL_TextAndImages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>ignored title text is not body</title>
<style>.x { color: red; }</style>
<script>var x = "should not appear";</script>
</head><body>
<h1>Магнитный сверлильный станок ZT-40H</h1>
<p>Цена: 180 000 ₸</p>
<img src="/photos/front.jpg" alt="Фронтальное фото">
<img src="https://cdn.example.com/gallery/side.jpg" alt="Вид сбоку">
</body></html>`))
	}))
	defer srv.Close()

	n := extractor.NewNative(testClient(), 1<<20)
	res, err := n.Extract(context.Background(), extractor.Source{Kind: extractor.SourceURL, URL: srv.URL})
	if err != nil {
		t.Fatalf("Extract() = %v, want success", err)
	}
	if !contains(res.Text, "Магнитный сверлильный станок ZT-40H") || !contains(res.Text, "180 000 ₸") {
		t.Fatalf("Text = %q, missing expected page content", res.Text)
	}
	if contains(res.Text, "should not appear") || contains(res.Text, "color: red") {
		t.Fatalf("Text = %q, script/style content leaked into extracted text", res.Text)
	}
	if len(res.Images) != 2 {
		t.Fatalf("Images = %+v, want 2", res.Images)
	}
	if res.Images[0].URL != srv.URL+"/photos/front.jpg" {
		t.Errorf("Images[0].URL = %q, want the relative src resolved against the page URL", res.Images[0].URL)
	}
	if res.Images[0].Alt != "Фронтальное фото" {
		t.Errorf("Images[0].Alt = %q", res.Images[0].Alt)
	}
	if res.Images[1].URL != "https://cdn.example.com/gallery/side.jpg" {
		t.Errorf("Images[1].URL = %q, want the absolute src unchanged", res.Images[1].URL)
	}
	if res.Provider != "native" {
		t.Errorf("Provider = %q, want native", res.Provider)
	}
}

// --- text/csv/markdown passthrough -------------------------------------------

func TestNative_ExtractTextPassthrough(t *testing.T) {
	n := extractor.NewNative(testClient(), 1<<20)
	res, err := n.Extract(context.Background(), extractor.Source{
		Kind: extractor.SourceFile, MimeType: "text/csv", Data: []byte("name,price\nWidget,100"),
	})
	if err != nil {
		t.Fatalf("Extract() = %v", err)
	}
	if res.Text != "name,price\nWidget,100" {
		t.Errorf("Text = %q, want verbatim passthrough", res.Text)
	}
}

// --- image source: no-op, not an error ---------------------------------------

func TestNative_ExtractImage_NoOp(t *testing.T) {
	n := extractor.NewNative(testClient(), 1<<20)
	res, err := n.Extract(context.Background(), extractor.Source{
		Kind: extractor.SourceFile, MimeType: "image/png", Data: []byte{0x89, 0x50, 0x4E, 0x47},
	})
	if err != nil {
		t.Fatalf("Extract(image) = %v, want success (no-op)", err)
	}
	if res.Text != "" || len(res.Images) != 0 {
		t.Fatalf("Extract(image) = %+v, want an empty no-op result — the material itself is the artifact", res)
	}
}

// --- DOCX (archive/zip + encoding/xml only) ----------------------------------

func TestNative_ExtractDOCX(t *testing.T) {
	data := buildMinimalDocx(t, []string{"Первый абзац.", "Второй абзац с ценой 50 000 ₸."})
	n := extractor.NewNative(testClient(), 1<<20)
	res, err := n.Extract(context.Background(), extractor.Source{
		Kind: extractor.SourceFile, MimeType: extractor.MimeDOCX, Data: data,
	})
	if err != nil {
		t.Fatalf("Extract(docx) = %v", err)
	}
	if !contains(res.Text, "Первый абзац.") || !contains(res.Text, "Второй абзац с ценой 50 000 ₸.") {
		t.Fatalf("Text = %q, missing expected paragraphs", res.Text)
	}
}

func TestNative_ExtractDOCX_NotAZip(t *testing.T) {
	n := extractor.NewNative(testClient(), 1<<20)
	_, err := n.Extract(context.Background(), extractor.Source{
		Kind: extractor.SourceFile, MimeType: extractor.MimeDOCX, Data: []byte("not a zip file"),
	})
	if err == nil {
		t.Fatal("expected an error for malformed docx bytes")
	}
}

// buildMinimalDocx constructs the smallest ZIP a .docx reader needs to find
// word/document.xml — package parts like [Content_Types].xml are omitted
// since extractDOCXText never looks at them.
func buildMinimalDocx(t *testing.T, paragraphs []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	body.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, p := range paragraphs {
		body.WriteString(`<w:p><w:r><w:t>`)
		body.WriteString(p)
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	body.WriteString(`</w:body></w:document>`)
	if _, err := f.Write(body.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func contains(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}
