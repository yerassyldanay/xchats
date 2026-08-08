package httpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/password"
)

// seedKBMaterial stages (and optionally completes) a kbd_materials row for
// the given org, mirroring kbstore_test.go's mkAttachMaterial helper — the
// content endpoint has no write path of its own, so tests seed directly
// through the store and blob backend the harness already exposes.
func (h *harness) seedKBMaterial(t *testing.T, orgID uuid.UUID, filename, mime string, data []byte, uploaded bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id, err := h.kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: filename, MimeType: mime, SizeBytes: int64(len(data)), CustomerVisibility: "visible",
	})
	if err != nil {
		t.Fatalf("create upload material: %v", err)
	}
	if uploaded {
		key := "kb-" + orgID.String() + "-" + id.String()
		if _, err := h.blob.Put(key, data, blob.Meta{Mimetype: mime, FileName: filename, FileSize: int64(len(data))}); err != nil {
			t.Fatalf("put blob: %v", err)
		}
		sum := sha256.Sum256(data)
		checksum := hex.EncodeToString(sum[:])
		if err := h.kb.CompleteMaterialUpload(ctx, id, "disk", key, int64(len(data)), checksum); err != nil {
			t.Fatalf("complete upload: %v", err)
		}
	}
	return id
}

func materialContentResponse(t *testing.T, client *http.Client, srvURL, materialID string) *http.Response {
	t.Helper()
	resp, err := client.Get(srvURL + "/xchats/api/v1/kb/materials/" + materialID + "/content")
	if err != nil {
		t.Fatalf("GET material content: %v", err)
	}
	return resp
}

// TestKBMaterialContent_HappyPathServesBytesWithSecurityHeaders proves the
// session-authenticated content route actually returns the stored bytes and
// carries the same defence-in-depth headers as handleMCPMediaRead
// (nosniff/CSP/Content-Disposition) and handleServeMedia.
func TestKBMaterialContent_HappyPathServesBytesWithSecurityHeaders(t *testing.T) {
	h := newHarness(t)
	data := []byte("\x89PNG\r\n\x1a\nfake-kb-material")
	id := h.seedKBMaterial(t, h.orgID, "product-photo.png", "image/png", data, true)

	resp := materialContentResponse(t, h.client, h.srv.URL, id.String())
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != string(data) {
		t.Errorf("body = %q, want %q", body, data)
	}

	// A second, fresh response for the header assertions below —
	// assertMediaSecurityHeaders closes the body itself, so it must not
	// share the response the read above already consumed and closed.
	resp2 := materialContentResponse(t, h.client, h.srv.URL, id.String())
	assertMediaSecurityHeaders(t, resp2)
}

// TestKBMaterialContent_ETagShortCircuitsOnMatch mirrors handleMCPMediaRead's
// checksum-as-ETag behavior: a matching If-None-Match must 304 without the
// handler needing to touch the blob store.
func TestKBMaterialContent_ETagShortCircuitsOnMatch(t *testing.T) {
	h := newHarness(t)
	id := h.seedKBMaterial(t, h.orgID, "cert.pdf", "application/pdf", []byte("%PDF-1.4 fake"), true)

	first := materialContentResponse(t, h.client, h.srv.URL, id.String())
	etag := first.Header.Get("ETag")
	first.Body.Close()
	if etag == "" {
		t.Fatal("first response carries no ETag")
	}

	req, _ := http.NewRequest(http.MethodGet, h.srv.URL+"/xchats/api/v1/kb/materials/"+id.String()+"/content", nil)
	req.Header.Set("If-None-Match", etag)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("conditional GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("status=%d, want 304", resp.StatusCode)
	}
}

// TestKBMaterialContent_RangeRequestHonored proves the endpoint uses
// http.ServeContent (not c.Data) — a <video> tag depends on partial-content
// support to seek.
func TestKBMaterialContent_RangeRequestHonored(t *testing.T) {
	h := newHarness(t)
	data := []byte("0123456789")
	id := h.seedKBMaterial(t, h.orgID, "clip.mp4", "video/mp4", data, true)

	req, _ := http.NewRequest(http.MethodGet, h.srv.URL+"/xchats/api/v1/kb/materials/"+id.String()+"/content", nil)
	req.Header.Set("Range", "bytes=2-4")
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("range GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status=%d, want 206", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "234" {
		t.Errorf("range body = %q, want %q", body, "234")
	}
}

// TestKBMaterialContent_UnknownAndNoBytesYet404s covers the two "nothing to
// serve" cases: an id that was never created, and one that was staged but
// never completed an upload (storage_key still empty).
func TestKBMaterialContent_UnknownAndNoBytesYet404s(t *testing.T) {
	h := newHarness(t)

	if resp := materialContentResponse(t, h.client, h.srv.URL, uuid.New().String()); resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Errorf("unknown id: status=%d, want 404", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	pending := h.seedKBMaterial(t, h.orgID, "still-uploading.bin", "application/octet-stream", []byte("x"), false)
	if resp := materialContentResponse(t, h.client, h.srv.URL, pending.String()); resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Errorf("no-bytes-yet: status=%d, want 404", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

// TestKBMaterialContent_CrossOrgIDOR is this route's version of A4
// (media_security_test.go's TestServeMedia_CrossOrgIDOR): a material must be
// readable by its OWN organization's session, and 404 — indistinguishable
// from an unknown id — for every other organization's session.
func TestKBMaterialContent_CrossOrgIDOR(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	org2, err := h.store.SeedOrganization(ctx, "xchats-second-kb-idor")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	hash, _ := password.Hash("second-org-password-1")
	if _, err := h.store.CreateUser(ctx, org2.ID, "second-org-kb@xchats.test", hash, "Second Org Admin", "admin"); err != nil {
		t.Fatalf("create second-org user: %v", err)
	}
	theirs := h.seedKBMaterial(t, org2.ID, "their-secret.png", "image/png", []byte("secret bytes"), true)

	// org2's own session can read its own material.
	otherClient, status := attemptLogin(t, h.srv.URL, "second-org-kb@xchats.test", "second-org-password-1")
	if status != http.StatusOK {
		t.Fatalf("second-org login: status=%d", status)
	}
	if resp := materialContentResponse(t, otherClient, h.srv.URL, theirs.String()); resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("owning org: status=%d, want 200", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// h's org (a different organization) must not be able to, even though
	// the id is real.
	resp := requestAs(t, h.client, http.MethodGet, h.srv.URL+"/xchats/api/v1/kb/materials/"+theirs.String()+"/content", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-org GET: status=%d, want 404", resp.StatusCode)
	}
}
