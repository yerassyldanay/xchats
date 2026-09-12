package workermock

import (
	"io"
	"net/http"
	"testing"
)

func TestDirectMediaClientReturnsAValidImage(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://cdn.example/attachment.jpg", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := (DirectMediaClient{}).Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty body")
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}
