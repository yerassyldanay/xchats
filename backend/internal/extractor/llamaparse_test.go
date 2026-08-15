package extractor_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
)

func TestLlamaParse_Supports(t *testing.T) {
	p := extractor.NewLlamaParseForTest("key", "", testClient(), time.Millisecond)
	cases := []struct {
		name string
		src  extractor.Source
		want bool
	}{
		{"pdf", extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF}, true},
		{"docx", extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimeDOCX}, true},
		{"text/plain", extractor.Source{Kind: extractor.SourceFile, MimeType: "text/plain"}, true},
		{"text/csv", extractor.Source{Kind: extractor.SourceFile, MimeType: "text/csv"}, true},
		{"url", extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com"}, false},
		{"image", extractor.Source{Kind: extractor.SourceFile, MimeType: "image/png"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := p.Supports(c.src); got != c.want {
				t.Errorf("Supports(%+v) = %v, want %v", c.src, got, c.want)
			}
		})
	}
}

func TestLlamaParse_Extract_URLSource_ErrUnsupported(t *testing.T) {
	p := extractor.NewLlamaParseForTest("key", "", testClient(), time.Millisecond)
	_, err := p.Extract(context.Background(), extractor.Source{Kind: extractor.SourceURL, URL: "https://example.com"})
	if !errors.Is(err, extractor.ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

// TestLlamaParse_Extract_UploadPollFetch exercises the full upload -> poll
// (PENDING once, then SUCCESS) -> markdown fetch flow against a test double
// serving all three LlamaParse endpoints, and asserts the auth header and
// upload's multipart shape.
func TestLlamaParse_Extract_UploadPollFetch(t *testing.T) {
	const jobID = "job-abc123"
	var polls int32
	var uploadAuth, statusAuth, resultAuth string
	var uploadContentType string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/parsing/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadAuth = r.Header.Get("Authorization")
		uploadContentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("upload: not valid multipart: %v", err)
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Errorf("upload: missing file part: %v", err)
		} else {
			defer f.Close()
			if hdr.Filename != "manual.pdf" {
				t.Errorf("upload filename = %q, want manual.pdf", hdr.Filename)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": jobID})
	})
	mux.HandleFunc("/api/v1/parsing/job/"+jobID, func(w http.ResponseWriter, r *http.Request) {
		statusAuth = r.Header.Get("Authorization")
		n := atomic.AddInt32(&polls, 1)
		status := "PENDING"
		if n >= 2 {
			status = "SUCCESS"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status})
	})
	mux.HandleFunc("/api/v1/parsing/job/"+jobID+"/result/markdown", func(w http.ResponseWriter, r *http.Request) {
		resultAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"markdown": "# Инструкция\n\n![Схема](https://cdn.example.com/schema.png)\n",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := extractor.NewLlamaParseForTest("lp-secret", srv.URL, testClient(), 5*time.Millisecond)
	res, err := p.Extract(context.Background(), extractor.Source{
		Kind: extractor.SourceFile, MimeType: extractor.MimePDF, Filename: "manual.pdf", Data: []byte("%PDF-1.4 fake"),
	})
	if err != nil {
		t.Fatalf("Extract() = %v", err)
	}
	if !strings.HasPrefix(uploadContentType, "multipart/form-data") {
		t.Errorf("upload Content-Type = %q, want multipart/form-data", uploadContentType)
	}
	for _, got := range []string{uploadAuth, statusAuth, resultAuth} {
		if got != "Bearer lp-secret" {
			t.Errorf("Authorization = %q, want Bearer lp-secret on every call", got)
		}
	}
	if atomic.LoadInt32(&polls) < 2 {
		t.Fatalf("expected at least 2 status polls (PENDING then SUCCESS), got %d", polls)
	}
	if res.Provider != "llamaparse" || res.Text == "" {
		t.Fatalf("res = %+v, want non-empty markdown text and provider=llamaparse", res)
	}
	if len(res.Images) != 1 || res.Images[0].URL != "https://cdn.example.com/schema.png" {
		t.Fatalf("Images = %+v, want one image parsed from the markdown result", res.Images)
	}
}

func TestLlamaParse_Extract_401_TypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := extractor.NewLlamaParseForTest("bad-key", srv.URL, testClient(), time.Millisecond)
	_, err := p.Extract(context.Background(), extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF, Data: []byte("x")})
	if !errors.Is(err, extractor.ErrProviderAuth) {
		t.Fatalf("err = %v, want ErrProviderAuth", err)
	}
}

func TestLlamaParse_Extract_JobFailure(t *testing.T) {
	const jobID = "job-fail"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/parsing/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": jobID})
	})
	mux.HandleFunc("/api/v1/parsing/job/"+jobID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ERROR"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := extractor.NewLlamaParseForTest("key", srv.URL, testClient(), time.Millisecond)
	_, err := p.Extract(context.Background(), extractor.Source{Kind: extractor.SourceFile, MimeType: extractor.MimePDF, Data: []byte("x")})
	if err == nil {
		t.Fatal("expected an error when the job status is ERROR")
	}
	if errors.Is(err, extractor.ErrProviderAuth) {
		t.Fatal("a job failure must not look like an auth error")
	}
}
