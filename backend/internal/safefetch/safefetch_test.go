package safefetch

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCheckURL_RejectsLoopbackLiteral(t *testing.T) {
	if err := CheckURL(context.Background(), "http://127.0.0.1/", false); !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("CheckURL(127.0.0.1) = %v, want ErrBlockedHost", err)
	}
}

func TestCheckURL_RejectsIPv6Loopback(t *testing.T) {
	if err := CheckURL(context.Background(), "http://[::1]/", false); !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("CheckURL(::1) = %v, want ErrBlockedHost", err)
	}
}

func TestCheckURL_RejectsPrivateRanges(t *testing.T) {
	for _, host := range []string{"10.0.0.5", "192.168.1.10", "172.16.5.5"} {
		t.Run(host, func(t *testing.T) {
			if err := CheckURL(context.Background(), "https://"+host+"/", false); !errors.Is(err, ErrBlockedHost) {
				t.Fatalf("CheckURL(%s) = %v, want ErrBlockedHost", host, err)
			}
		})
	}
}

func TestCheckURL_RejectsLinkLocal(t *testing.T) {
	// 169.254.169.254 is the cloud-metadata endpoint — the single most
	// consequential SSRF target to block.
	if err := CheckURL(context.Background(), "http://169.254.169.254/latest/meta-data/", false); !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("CheckURL(169.254.169.254) = %v, want ErrBlockedHost", err)
	}
}

func TestCheckURL_RejectsDNSNameResolvingPrivate(t *testing.T) {
	// "localhost" resolves via the OS resolver (usually /etc/hosts) to
	// 127.0.0.1/::1 without needing real network access — this exercises
	// CheckURL's LookupIPAddr path, distinct from the IP-literal fast path
	// the tests above cover.
	err := CheckURL(context.Background(), "http://localhost:8080/", false)
	if err == nil {
		t.Fatal("CheckURL(localhost) = nil, want an error")
	}
	if !errors.Is(err, ErrBlockedHost) {
		t.Skipf("localhost did not resolve in this sandbox (got %v) — skipping, this environment has no usable local resolver", err)
	}
}

func TestCheckURL_AllowsPublicIPLiteral(t *testing.T) {
	// A well-known public IP literal — no DNS involved, so this needs no
	// network access to assert the "not blocked" branch.
	if err := CheckURL(context.Background(), "https://8.8.8.8/", false); err != nil {
		t.Fatalf("CheckURL(8.8.8.8) = %v, want nil", err)
	}
}

func TestCheckURL_AllowPrivateBypassesEverything(t *testing.T) {
	if err := CheckURL(context.Background(), "http://127.0.0.1/", true); err != nil {
		t.Fatalf("CheckURL with allowPrivate=true = %v, want nil", err)
	}
}

func TestCheckURL_RejectsNonHTTPScheme(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/x", "javascript:alert(1)"} {
		if err := CheckURL(context.Background(), raw, false); err == nil {
			t.Fatalf("CheckURL(%q) = nil, want a scheme rejection", raw)
		}
	}
}

func TestCheckURL_RejectsMalformedURL(t *testing.T) {
	if err := CheckURL(context.Background(), "://not a url", false); err == nil {
		t.Fatal("expected a parse error")
	}
}

// --- Client: dialer Control (the real SSRF boundary) -----------------------

func TestClient_BlocksLoopbackDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := Client(false, 2*time.Second)
	_, _, err := Get(context.Background(), client, srv.URL, 1024)
	if err == nil {
		t.Fatal("expected the dialer to refuse a loopback connection")
	}
}

func TestClient_AllowsLoopbackWhenAllowPrivate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := Client(true, 2*time.Second)
	data, resp, err := Get(context.Background(), client, srv.URL, 1024)
	if err != nil {
		t.Fatalf("Get with allowPrivate client = %v, want success", err)
	}
	if resp.StatusCode != http.StatusOK || string(data) != "ok" {
		t.Fatalf("Get body/status = %q/%d, want ok/200", data, resp.StatusCode)
	}
}

// --- checkRedirectPolicy: https-only + redirect cap -------------------------

func TestCheckRedirectPolicy_BlocksHTTPDowngrade(t *testing.T) {
	req := &http.Request{URL: mustParseURL(t, "http://example.com/next")}
	if err := checkRedirectPolicy(req, nil); err == nil {
		t.Fatal("expected a redirect to a non-https URL to be blocked")
	}
}

func TestCheckRedirectPolicy_AllowsHTTPSUnderTheCap(t *testing.T) {
	req := &http.Request{URL: mustParseURL(t, "https://example.com/next")}
	via := make([]*http.Request, maxRedirects-1)
	if err := checkRedirectPolicy(req, via); err != nil {
		t.Fatalf("expected an https redirect under the cap to be allowed: %v", err)
	}
}

func TestCheckRedirectPolicy_BlocksAtTheRedirectCap(t *testing.T) {
	req := &http.Request{URL: mustParseURL(t, "https://example.com/next")}
	via := make([]*http.Request, maxRedirects)
	if err := checkRedirectPolicy(req, via); err == nil {
		t.Fatal("expected the 6th hop (5 prior redirects) to be blocked")
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

// --- Get: byte caps ----------------------------------------------------------

func TestGet_RejectsOverCapDeclaredContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "999999999")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short body, never fully sent"))
	}))
	defer srv.Close()

	client := Client(true, 2*time.Second)
	_, _, err := Get(context.Background(), client, srv.URL, 1024)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Get() err = %v, want ErrTooLarge (rejected via declared Content-Length)", err)
	}
}

func TestGet_RejectsUnboundedChunkedBody(t *testing.T) {
	const maxBytes = 4096
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not support flushing — cannot force chunked encoding")
		}
		chunk := bytes.Repeat([]byte("a"), 1024)
		// No Content-Length is ever set, and multiple flushed writes force
		// chunked transfer-encoding — the "server lies about size" / unknown-
		// length case Get's io.LimitReader (not the Content-Length check) must
		// catch. Total exceeds maxBytes by a wide margin.
		for i := 0; i < 64; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			fl.Flush()
		}
	}))
	defer srv.Close()

	client := Client(true, 5*time.Second)
	_, _, err := Get(context.Background(), client, srv.URL, maxBytes)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Get() err = %v, want ErrTooLarge (rejected via streamed byte cap)", err)
	}
}

func TestGet_AllowsBodyUnderCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("small"))
	}))
	defer srv.Close()

	client := Client(true, 2*time.Second)
	data, _, err := Get(context.Background(), client, srv.URL, 1024)
	if err != nil {
		t.Fatalf("Get() = %v, want success", err)
	}
	if string(data) != "small" {
		t.Fatalf("Get() body = %q, want %q", data, "small")
	}
}
