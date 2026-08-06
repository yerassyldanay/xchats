package httpapi_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/password"
)

// uploadRequest builds (but does not send) a POST /xchats/api/v1/media
// multipart request — the request-construction half of harness.upload
// (integration_test.go), factored out so a test can inspect the response
// of a REJECTED upload instead of upload's built-in fatal-on-non-200.
func (h *harness) uploadRequest(t *testing.T, name, ct string, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename=%q`, name)}
	hdr["Content-Type"] = []string{ct}
	pw, _ := mw.CreatePart(hdr)
	pw.Write(data)
	mw.Close()
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/xchats/api/v1/media", &buf)
	if err != nil {
		t.Fatalf("build upload request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// mediaResponse GETs a media id's raw (non-enveloped) HTTP response.
func (h *harness) mediaResponse(t *testing.T, mediaID string) *http.Response {
	t.Helper()
	resp, err := h.client.Get(h.srv.URL + "/xchats/api/v1/media/" + mediaID)
	if err != nil {
		t.Fatalf("GET /media/%s: %v", mediaID, err)
	}
	return resp
}

// TestUploadMedia_RejectsHTMLMasqueradingAsImage is A3's regression guard:
// the classic stored-XSS-via-mislabeled-upload vector. Before this fix,
// handleUploadMedia trusted the client's declared Content-Type verbatim and
// handleServeMedia echoed it back with no protective headers, on the same
// origin as the app's own session cookie.
func TestUploadMedia_RejectsHTMLMasqueradingAsImage(t *testing.T) {
	h := newHarness(t)
	html := []byte("<html><body><script>alert(document.cookie)</script></body></html>")
	req := h.uploadRequest(t, "innocuous.png", "image/png", html)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (HTML content rejected regardless of declared image/png)", resp.StatusCode)
	}
}

// TestServeMedia_SecurityHeaders proves both handleServeMedia paths — the
// org-scoped confirmed-media path and the pending-upload fallback — carry
// the same defense-in-depth headers (writeMediaResponse, inbox.go), reused
// from mcp_media.go's already-correct handling of the identical problem.
func TestServeMedia_SecurityHeaders(t *testing.T) {
	h := newHarness(t)

	// Path 1: a pending upload, not yet attached to any message.
	pendingID := h.upload("a.png", "image/png", []byte("\x89PNG\r\n\x1a\nfake"))
	assertMediaSecurityHeaders(t, h.mediaResponse(t, pendingID))

	// Path 2: confirmed, message-attached media.
	chatID, _ := h.inject(customerJID, "WA-HDRS-1", "hi", false)
	confirmedID := h.upload("b.png", "image/png", []byte("\x89PNG\r\n\x1a\nfake2"))
	resp, _ := h.postJSON("/xchats/api/v1/chats/"+chatID+"/messages", map[string]any{"media_ids": []string{confirmedID}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send media message: status=%d", resp.StatusCode)
	}
	assertMediaSecurityHeaders(t, h.mediaResponse(t, confirmedID))
}

func assertMediaSecurityHeaders(t *testing.T, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy header missing")
	}
	if got := resp.Header.Get("Content-Disposition"); got == "" {
		t.Error("Content-Disposition header missing")
	}
}

// TestUploadMedia_RejectsOversized is A9's regression guard: before
// http.MaxBytesReader wrapped the request body (inbox.go), gin's own
// MaxMultipartMemory only bounded the parser's memory/disk spill point, not
// the upload itself — a request could be arbitrarily large.
func TestUploadMedia_RejectsOversized(t *testing.T) {
	h := newHarness(t)
	oversized := make([]byte, 64<<20+1) // one byte past the 64 MiB limit
	req := h.uploadRequest(t, "big.bin", "application/octet-stream", oversized)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status=%d, want 413", resp.StatusCode)
	}
}

// TestServeMedia_CrossOrgIDOR is A4's regression guard: confirmed media
// belonging to a DIFFERENT organization must 404 for an authenticated user
// of this one, and the owning org's own access must keep working — the fix
// must not be a blanket lockout, just a scope.
func TestServeMedia_CrossOrgIDOR(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	chatID, _ := h.inject(customerJID, "WA-IDOR-1", "hi", false)
	mediaID := h.upload("secret.png", "image/png", []byte("\x89PNG\r\n\x1a\norg-a-bytes"))
	resp, _ := h.postJSON("/xchats/api/v1/chats/"+chatID+"/messages", map[string]any{"media_ids": []string{mediaID}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send media message: status=%d", resp.StatusCode)
	}

	// Confirm the owning org can still read it — the fix must not be a
	// blanket lockout.
	if r := h.mediaResponse(t, mediaID); r.StatusCode != http.StatusOK {
		r.Body.Close()
		t.Fatalf("owning org GET /media/%s: status=%d, want 200", mediaID, r.StatusCode)
	} else {
		r.Body.Close()
	}

	// A second, unrelated organization must not be able to read it.
	org2, err := h.store.SeedOrganization(ctx, "xchats-second-idor")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	hash, _ := password.Hash("second-org-password-1")
	if _, err := h.store.CreateUser(ctx, org2.ID, "second-org-admin@xchats.test", hash, "Second Org Admin", "admin"); err != nil {
		t.Fatalf("create second-org user: %v", err)
	}
	otherClient, status := attemptLogin(t, h.srv.URL, "second-org-admin@xchats.test", "second-org-password-1")
	if status != http.StatusOK {
		t.Fatalf("second-org login: status=%d", status)
	}
	resp2 := requestAs(t, otherClient, http.MethodGet, h.srv.URL+"/xchats/api/v1/media/"+mediaID, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("cross-org GET /media/%s: status=%d, want 404", mediaID, resp2.StatusCode)
	}
}
