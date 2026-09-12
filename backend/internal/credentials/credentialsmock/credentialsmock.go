// Package credentialsmock provides an *http.Client for cmd/xchats'
// mock-externals mode that answers every credential-probe request
// (internal/credentials' Validate hooks: OpenRouter, OpenAI, Gemini, Groq,
// ngrok, Langfuse, Meta, Firecrawl, LlamaParse — see provider.go's
// providers table) with a 200 OK, never dialing out. classify (provider.go)
// treats status 200 alone as success regardless of body content, so this
// needs no per-vendor response shaping — every "Test connection" click in
// Settings just succeeds instantly.
package credentialsmock

import (
	"io"
	"net/http"
	"strings"
)

// roundTripFunc adapts a plain function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// Client returns an *http.Client whose Transport answers every request with
// 200 OK and an empty JSON body — pass it to
// credentials.SetValidateHTTPClient in mock mode.
func Client() *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Proto:      "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header:  make(http.Header),
			Body:    io.NopCloser(strings.NewReader("{}")),
			Request: req,
		}, nil
	})}
}
