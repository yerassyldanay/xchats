// Package workermock provides a drop-in for worker.Worker.DirectMediaHTTP
// for cmd/xchats' mock-externals mode — the one outbound call in
// internal/worker not already behind an interface of its own (a plain GET
// against an Instagram/Messenger CDN URL for inbound attachment bytes).
package workermock

import (
	"bytes"
	"io"
	"net/http"
)

// mockPNG is a minimal valid 1x1 transparent PNG.
var mockPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

// DirectMediaClient answers every request with a small valid image, never
// dialing out. It structurally satisfies worker's own unexported httpDoer
// interface (Do(*http.Request) (*http.Response, error)) — Go interface
// satisfaction needs no import of worker itself.
type DirectMediaClient struct{}

func (DirectMediaClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Proto:      "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:  http.Header{"Content-Type": []string{"image/png"}},
		Body:    io.NopCloser(bytes.NewReader(mockPNG)),
		Request: req,
	}, nil
}
