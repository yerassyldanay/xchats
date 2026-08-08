package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/password"
)

// kbUploadRequest builds (but does not send) a POST /xchats/api/v1/kb/materials
// multipart request — the KB counterpart of h.uploadRequest (media_security_test.go),
// which targets the chat-media route instead.
func (h *harness) kbUploadRequest(t *testing.T, name, ct string, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename=%q`, name)}
	hdr["Content-Type"] = []string{ct}
	pw, _ := mw.CreatePart(hdr)
	pw.Write(data)
	mw.Close()
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/xchats/api/v1/kb/materials", &buf)
	if err != nil {
		t.Fatalf("build kb upload request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// TestKBUploadMaterial_HappyPathProvesOneLineage uploads through the new
// browser endpoint and reads it back through GET /kb/materials/:id/content
// — the SAME route an MCP-uploaded material's bytes are served through —
// proving both authoring paths share one material lineage end to end
// (has_content, storage_key, the content endpoint) rather than two parallel
// implementations that could drift.
func TestKBUploadMaterial_HappyPathProvesOneLineage(t *testing.T) {
	h := newHarness(t)
	data := []byte("\x89PNG\r\n\x1a\nfake-browser-upload")

	resp, err := h.client.Do(h.kbUploadRequest(t, "hero.png", "image/png", data))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d, want 200 (body=%s)", resp.StatusCode, body)
	}
	var env struct {
		Payload struct {
			MaterialID       string `json:"material_id"`
			ProcessingStatus string `json:"processing_status"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if env.Payload.MaterialID == "" {
		t.Fatal("response carries no material_id")
	}
	if env.Payload.ProcessingStatus != "parsed" {
		t.Errorf("processing_status = %q, want parsed", env.Payload.ProcessingStatus)
	}

	contentResp := materialContentResponse(t, h.client, h.srv.URL, env.Payload.MaterialID)
	defer contentResp.Body.Close()
	if contentResp.StatusCode != http.StatusOK {
		t.Fatalf("GET content: status=%d", contentResp.StatusCode)
	}
	body, _ := io.ReadAll(contentResp.Body)
	if !bytes.Equal(body, data) {
		t.Errorf("content bytes = %q, want %q", body, data)
	}
	if got := contentResp.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
}

// TestKBUploadMaterial_RejectsHTMLMasqueradingAsImage is this route's
// version of A3 (media_security_test.go's TestUploadMedia_RejectsHTMLMasqueradingAsImage):
// declaring image/png over actual HTML bytes must not be trusted.
func TestKBUploadMaterial_RejectsHTMLMasqueradingAsImage(t *testing.T) {
	h := newHarness(t)
	html := []byte("<html><body><script>alert(document.cookie)</script></body></html>")
	resp, err := h.client.Do(h.kbUploadRequest(t, "innocuous.png", "image/png", html))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (HTML content rejected regardless of declared image/png)", resp.StatusCode)
	}
}

// TestKBUploadMaterial_RejectsUnattachableMimeType covers the KB-specific
// guard media_security_test.go's chat-media route has no equivalent of:
// kbstore.KindOfMime returning "" would make the material permanently
// un-attachable at validateMediaRef time, so it must be rejected here at
// upload time instead of leaving an orphaned row behind.
func TestKBUploadMaterial_RejectsUnattachableMimeType(t *testing.T) {
	h := newHarness(t)
	resp, err := h.client.Do(h.kbUploadRequest(t, "font.woff2", "font/woff2", []byte("not a real font, just bytes")))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (font/woff2 is not image/video/audio/document)", resp.StatusCode)
	}
}

// TestKBUploadMaterial_RejectsOversized mirrors A9's regression guard
// (TestUploadMedia_RejectsOversized) for this route's own cap:
// mcpUploadMaxBytes (50 MiB) — deliberately not maxUploadBytes (64 MiB).
func TestKBUploadMaterial_RejectsOversized(t *testing.T) {
	h := newHarness(t)
	oversized := make([]byte, 50<<20+1) // one byte past the 50 MiB limit
	resp, err := h.client.Do(h.kbUploadRequest(t, "big.bin", "application/octet-stream", oversized))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status=%d, want 413", resp.StatusCode)
	}
}

// TestKBUploadMaterial_CrossOrgCannotReadUploadedMaterial is this route's
// version of A4 (TestKBMaterialContent_CrossOrgIDOR): a material uploaded by
// one organization's session must 404 for every other organization's
// session, proving the browser-upload path lands in the same org-scoped
// kbd_materials row every other authoring path does.
func TestKBUploadMaterial_CrossOrgCannotReadUploadedMaterial(t *testing.T) {
	h := newHarness(t)
	resp, err := h.client.Do(h.kbUploadRequest(t, "secret.png", "image/png", []byte("\x89PNG\r\n\x1a\norg-a-only")))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	var env struct {
		Payload struct {
			MaterialID string `json:"material_id"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	ctx := context.Background()
	org2, err := h.store.SeedOrganization(ctx, "xchats-second-kb-upload-idor")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	hash, _ := password.Hash("second-org-password-1")
	if _, err := h.store.CreateUser(ctx, org2.ID, "second-org-upload@xchats.test", hash, "Second Org Admin", "admin"); err != nil {
		t.Fatalf("create second-org user: %v", err)
	}
	otherClient, status := attemptLogin(t, h.srv.URL, "second-org-upload@xchats.test", "second-org-password-1")
	if status != http.StatusOK {
		t.Fatalf("second-org login: status=%d", status)
	}
	resp2 := requestAs(t, otherClient, http.MethodGet, h.srv.URL+"/xchats/api/v1/kb/materials/"+env.Payload.MaterialID+"/content", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("cross-org GET content: status=%d, want 404", resp2.StatusCode)
	}
}

// TestKBUploadMaterial_RequiresAuthentication proves an unauthenticated
// request (no session cookie) is rejected before any upload logic runs.
func TestKBUploadMaterial_RequiresAuthentication(t *testing.T) {
	h := newHarness(t)
	req := h.kbUploadRequest(t, "hero.png", "image/png", []byte("\x89PNG\r\n\x1a\nfake"))
	resp, err := (&http.Client{}).Do(req) // no cookie jar — never logged in
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", resp.StatusCode)
	}
}
