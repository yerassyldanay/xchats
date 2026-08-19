package extractor_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
)

func TestFirecrawl_Supports(t *testing.T) {
	p := extractor.NewFirecrawl("key", "", testClient())
	if !p.Supports(extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com"}) {
		t.Error("expected Supports(url) = true")
	}
	if p.Supports(extractor.Source{Kind: extractor.SourceFile, MimeType: "application/pdf"}) {
		t.Error("expected Supports(file) = false — firecrawl is URL-only")
	}
}

func TestFirecrawl_Extract_FileSource_ErrUnsupported(t *testing.T) {
	p := extractor.NewFirecrawl("key", "", testClient())
	_, err := p.Extract(context.Background(), extractor.Source{Kind: extractor.SourceFile, MimeType: "text/plain", Data: []byte("x")})
	if !errors.Is(err, extractor.ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestFirecrawl_Extract_RequestShapeAndMarkdownImages(t *testing.T) {
	var gotAuth, gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"markdown": "# ZT-40H\n\nЦена: 180 000 ₸\n\n![Фото станка](https://cdn.example.com/zt40h.jpg)\n",
			},
		})
	}))
	defer srv.Close()

	p := extractor.NewFirecrawl("fc-secret-key", srv.URL, testClient())
	res, err := p.Extract(context.Background(), extractor.Source{Kind: extractor.SourceURL, URL: "https://vendor.example/zt40h"})
	if err != nil {
		t.Fatalf("Extract() = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/scrape" {
		t.Errorf("path = %q, want /v2/scrape", gotPath)
	}
	if gotAuth != "Bearer fc-secret-key" {
		t.Errorf("Authorization = %q, want Bearer fc-secret-key", gotAuth)
	}
	if gotBody["url"] != "https://vendor.example/zt40h" {
		t.Errorf("request body url = %v, want the source URL", gotBody["url"])
	}

	if res.Text == "" || res.Provider != "firecrawl" {
		t.Fatalf("res = %+v, want non-empty markdown text and provider=firecrawl", res)
	}
	if len(res.Images) != 1 || res.Images[0].URL != "https://cdn.example.com/zt40h.jpg" || res.Images[0].Alt != "Фото станка" {
		t.Fatalf("Images = %+v, want one image parsed from the markdown", res.Images)
	}
}

func TestFirecrawl_Extract_401_TypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"error":"invalid api key"}`))
	}))
	defer srv.Close()

	p := extractor.NewFirecrawl("bad-key", srv.URL, testClient())
	_, err := p.Extract(context.Background(), extractor.Source{Kind: extractor.SourceURL, URL: "https://vendor.example/x"})
	if !errors.Is(err, extractor.ErrProviderAuth) {
		t.Fatalf("err = %v, want ErrProviderAuth", err)
	}
}

func TestFirecrawl_Extract_ScrapeFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "page not renderable"})
	}))
	defer srv.Close()

	p := extractor.NewFirecrawl("key", srv.URL, testClient())
	_, err := p.Extract(context.Background(), extractor.Source{Kind: extractor.SourceURL, URL: "https://vendor.example/x"})
	if err == nil {
		t.Fatal("expected an error when firecrawl reports success:false")
	}
	if errors.Is(err, extractor.ErrProviderAuth) {
		t.Fatal("a content-level scrape failure must not look like an auth error")
	}
}
