package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// seedKBMaterialVisibility is h.seedKBMaterial (kb_media_test.go) with a
// configurable customer_visibility — that helper hardcodes "visible", but
// the invisible-material rejection case below needs "invisible" specifically.
func (h *harness) seedKBMaterialVisibility(t *testing.T, orgID uuid.UUID, mime, visibility string, uploaded bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	data := []byte("fake-bytes-for-visibility-test")
	id, err := h.kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: "f.bin", MimeType: mime, SizeBytes: int64(len(data)), CustomerVisibility: visibility,
	})
	if err != nil {
		t.Fatalf("create upload material: %v", err)
	}
	if uploaded {
		key := "kb-vis-" + orgID.String() + "-" + id.String()
		if _, err := h.blob.Put(key, data, blob.Meta{Mimetype: mime, FileName: "f.bin", FileSize: int64(len(data))}); err != nil {
			t.Fatalf("put blob: %v", err)
		}
		if err := h.kb.CompleteMaterialUpload(ctx, id, "disk", key, int64(len(data)), ""); err != nil {
			t.Fatalf("complete upload: %v", err)
		}
	}
	return id
}

func envErrcode(t *testing.T, env map[string]json.RawMessage) string {
	t.Helper()
	var code string
	if err := json.Unmarshal(env["errcode"], &code); err != nil {
		t.Fatalf("decode errcode: %v", err)
	}
	return code
}

// TestPlaygroundUpsertProduct_MediaReferenceRejections422 pins the kbFail
// fix: every way validateMediaRef can reject a material_id must come back
// as 422 VALIDATION_ERROR with a domain message, never the default 500
// INTERNAL branch that would leak raw Go error text (mirrors
// media_security_test.go's style of one assertion per rejection reason).
func TestPlaygroundUpsertProduct_MediaReferenceRejections422(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	otherOrg, err := h.store.SeedOrganization(ctx, "xchats-media-reject-org")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	foreignImg := h.seedKBMaterial(t, otherOrg.ID, "foreign.png", "image/png", []byte("foreign"), true)
	notUploaded := h.seedKBMaterialVisibility(t, h.orgID, "video/mp4", "visible", false)
	invisible := h.seedKBMaterialVisibility(t, h.orgID, "image/png", "invisible", true)
	wrongKind := h.seedKBMaterial(t, h.orgID, "clip.mp3", "audio/mpeg", []byte("audio"), true)

	cases := []struct {
		name  string
		field string
		id    uuid.UUID
	}{
		{"foreign organization", "featured_image", foreignImg},
		{"upload not completed", "demo_videos", notUploaded},
		{"marked invisible", "featured_image", invisible},
		{"mime does not match field kind", "featured_image", wrongKind},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"ref": "reject-" + tc.name, "name": "Товар", "availability_status": "in_stock"}
			if tc.field == "demo_videos" {
				body[tc.field] = []string{tc.id.String()}
			} else {
				body[tc.field] = tc.id.String()
			}
			resp, env := h.postJSON("/xchats/api/v1/playground/draft/products", body)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d, want 422 (body=%s)", resp.StatusCode, env["message"])
			}
			if code := envErrcode(t, env); code != "VALIDATION_ERROR" {
				t.Errorf("errcode=%q, want VALIDATION_ERROR", code)
			}
		})
	}
}

// TestPlaygroundUpsertProduct_FeaturedImageTriState proves the wire's three
// states for a nullable singular reference (absent/null/uuid) map onto
// kbstore's leave/clear/set contract through the browser JSON path.
func TestPlaygroundUpsertProduct_FeaturedImageTriState(t *testing.T) {
	h := newHarness(t)
	img := h.seedKBMaterial(t, h.orgID, "hero.png", "image/png", []byte("hero"), true)

	// Set.
	resp, env := h.postJSON("/xchats/api/v1/playground/draft/products", map[string]any{
		"ref": "tri", "name": "Товар", "availability_status": "in_stock", "featured_image": img.String(),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := draftProduct(t, env, "tri").FeaturedImage; got == nil || *got != img {
		t.Fatalf("featured_image after set = %v, want %v", got, img)
	}

	// Absent — leaves the value untouched.
	resp, env = h.postJSON("/xchats/api/v1/playground/draft/products", map[string]any{
		"ref": "tri", "name": "Товар v2", "availability_status": "in_stock",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("absent: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := draftProduct(t, env, "tri").FeaturedImage; got == nil || *got != img {
		t.Fatalf("featured_image after absent-field write = %v, want unchanged %v", got, img)
	}

	// Explicit null — clears it.
	resp, env = h.postJSON("/xchats/api/v1/playground/draft/products", map[string]any{
		"ref": "tri", "name": "Товар v3", "availability_status": "in_stock", "featured_image": nil,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := draftProduct(t, env, "tri").FeaturedImage; got != nil {
		t.Fatalf("featured_image after explicit null = %v, want nil", *got)
	}
}

// TestPlaygroundUpsertProduct_PluralEmptyArrayDetaches proves the plural
// contract: absent leaves the list untouched, an explicit [] detaches
// everything.
func TestPlaygroundUpsertProduct_PluralEmptyArrayDetaches(t *testing.T) {
	h := newHarness(t)
	img := h.seedKBMaterial(t, h.orgID, "g1.png", "image/png", []byte("g1"), true)

	resp, env := h.postJSON("/xchats/api/v1/playground/draft/products", map[string]any{
		"ref": "plural", "name": "Товар", "availability_status": "in_stock", "gallery_images": []string{img.String()},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := draftProduct(t, env, "plural").GalleryImages; len(got) != 1 || got[0] != img {
		t.Fatalf("gallery_images after set = %v, want [%v]", got, img)
	}

	resp, env = h.postJSON("/xchats/api/v1/playground/draft/products", map[string]any{
		"ref": "plural", "name": "Товар v2", "availability_status": "in_stock",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("absent: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := draftProduct(t, env, "plural").GalleryImages; len(got) != 1 || got[0] != img {
		t.Fatalf("gallery_images after absent-field write = %v, want unchanged [%v]", got, img)
	}

	resp, env = h.postJSON("/xchats/api/v1/playground/draft/products", map[string]any{
		"ref": "plural", "name": "Товар v3", "availability_status": "in_stock", "gallery_images": []string{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detach: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := draftProduct(t, env, "plural").GalleryImages; len(got) != 0 {
		t.Fatalf("gallery_images after [] = %v, want empty", got)
	}
}

// draftProduct pulls one product by ref out of a DraftChangeSet envelope
// payload (the response handlePlaygroundUpsertProduct returns).
func draftProduct(t *testing.T, env map[string]json.RawMessage, ref string) kbstore.ProductRow {
	t.Helper()
	var changes kbstore.DraftChangeSet
	if err := json.Unmarshal(env["payload"], &changes); err != nil {
		t.Fatalf("decode DraftChangeSet: %v", err)
	}
	for _, p := range changes.Products {
		if p.Ref == ref {
			return p
		}
	}
	t.Fatalf("product %q not in response payload: %s", ref, env["payload"])
	return kbstore.ProductRow{}
}

// TestKBLiveProduct_IgnoresMediaKeys proves the draft/live boundary Stage 3
// documents on productReq: POST /kb/products (the LIVE lane) shares the
// struct with media fields, but PutLiveProduct's ProductInput construction
// never reads them, so a request carrying media keys leaves live media
// completely untouched instead of corrupting it. The browser SPA never
// calls this route (only /playground/draft/* and GET /kb), but the
// contract must hold regardless of caller.
func TestKBLiveProduct_IgnoresMediaKeys(t *testing.T) {
	h := newHarness(t)
	img := h.seedKBMaterial(t, h.orgID, "live.png", "image/png", []byte("live"), true)

	resp, env := h.postJSON("/xchats/api/v1/kb/products", map[string]any{
		"ref": "live-p", "name": "Товар", "availability_status": "in_stock",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create live product: status=%d body=%s", resp.StatusCode, env["message"])
	}

	resp, env = h.postJSON("/xchats/api/v1/kb/products", map[string]any{
		"ref": "live-p", "name": "Товар (edit)", "availability_status": "in_stock",
		"featured_image": img.String(), "gallery_images": []string{img.String()},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("edit live product with media keys: status=%d body=%s", resp.StatusCode, env["message"])
	}

	var live kbstore.DraftView
	h.get("/xchats/api/v1/kb", &live)
	for _, p := range live.Products {
		if p.Ref != "live-p" {
			continue
		}
		if p.FeaturedImage != nil {
			t.Errorf("live featured_image = %v, want nil — media keys must be ignored on the live lane", *p.FeaturedImage)
		}
		if len(p.GalleryImages) != 0 {
			t.Errorf("live gallery_images = %v, want empty — media keys must be ignored on the live lane", p.GalleryImages)
		}
		if p.Name != "Товар (edit)" {
			t.Errorf("live name = %q, want the text edit to still apply", p.Name)
		}
		return
	}
	t.Fatalf("live product 'live-p' not found in GET /kb")
}
