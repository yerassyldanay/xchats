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

// TestKBLiveProduct_AppliesMediaKeys proves POST /kb/products (the LIVE lane)
// applies and persists media keys (featured_image, gallery_images, etc.) into
// the live ai_products table, and supports tri-state featured_image and
// detach-all gallery_images.
func TestKBLiveProduct_AppliesMediaKeys(t *testing.T) {
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
	var found bool
	for _, p := range live.Products {
		if p.Ref != "live-p" {
			continue
		}
		found = true
		if p.FeaturedImage == nil || *p.FeaturedImage != img {
			t.Errorf("live featured_image = %v, want %v", p.FeaturedImage, img)
		}
		if len(p.GalleryImages) != 1 || p.GalleryImages[0] != img {
			t.Errorf("live gallery_images = %v, want [%v]", p.GalleryImages, img)
		}
		if p.Name != "Товар (edit)" {
			t.Errorf("live name = %q, want 'Товар (edit)'", p.Name)
		}
		break
	}
	if !found {
		t.Fatalf("live product 'live-p' not found in GET /kb")
	}

	// Absent media keys leave existing media unchanged.
	resp, env = h.postJSON("/xchats/api/v1/kb/products", map[string]any{
		"ref": "live-p", "name": "Товар (edit 2)", "availability_status": "in_stock",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("edit live product absent media: status=%d body=%s", resp.StatusCode, env["message"])
	}
	h.get("/xchats/api/v1/kb", &live)
	for _, p := range live.Products {
		if p.Ref == "live-p" {
			if p.FeaturedImage == nil || *p.FeaturedImage != img {
				t.Errorf("live featured_image after absent write = %v, want unchanged %v", p.FeaturedImage, img)
			}
			if len(p.GalleryImages) != 1 || p.GalleryImages[0] != img {
				t.Errorf("live gallery_images after absent write = %v, want unchanged [%v]", p.GalleryImages, img)
			}
			break
		}
	}

	// Explicit null / empty array detaches media.
	resp, env = h.postJSON("/xchats/api/v1/kb/products", map[string]any{
		"ref": "live-p", "name": "Товар (edit 3)", "availability_status": "in_stock",
		"featured_image": nil, "gallery_images": []string{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("edit live product clear media: status=%d body=%s", resp.StatusCode, env["message"])
	}
	h.get("/xchats/api/v1/kb", &live)
	for _, p := range live.Products {
		if p.Ref == "live-p" {
			if p.FeaturedImage != nil {
				t.Errorf("live featured_image after null = %v, want nil", *p.FeaturedImage)
			}
			if len(p.GalleryImages) != 0 {
				t.Errorf("live gallery_images after empty = %v, want empty", p.GalleryImages)
			}
			break
		}
	}
}

// TestKBLiveOtherEntities_AppliesMediaKeys verifies topics, tariffs, contacts,
// and policies live endpoints persist media references into the database.
func TestKBLiveOtherEntities_AppliesMediaKeys(t *testing.T) {
	h := newHarness(t)
	img := h.seedKBMaterial(t, h.orgID, "live.png", "image/png", []byte("live"), true)
	doc := h.seedKBMaterial(t, h.orgID, "doc.pdf", "application/pdf", []byte("pdf"), true)

	// Topic
	resp, env := h.postJSON("/xchats/api/v1/kb/topics", map[string]any{
		"slug": "live-topic", "title": "Заголовок", "body_md": "Текст темы.",
		"featured_image": img.String(), "reference_documents": []string{doc.String()},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create live topic: status=%d body=%s", resp.StatusCode, env["message"])
	}

	// Tariff
	resp, env = h.postJSON("/xchats/api/v1/kb/tariffs", map[string]any{
		"ref": "live-tariff", "name": "Тариф", "pricing_images": []string{img.String()},
		"terms_documents": []string{doc.String()},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create live tariff: status=%d body=%s", resp.StatusCode, env["message"])
	}

	// Contacts
	resp, env = h.patchJSON("/xchats/api/v1/kb/contacts", map[string]any{
		"contact_card_image": img.String(), "company_legal_documents": []string{doc.String()},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch live contacts: status=%d body=%s", resp.StatusCode, env["message"])
	}

	// Policies
	resp, env = h.patchJSON("/xchats/api/v1/kb/policies", map[string]any{
		"commerce_policy_documents": []string{doc.String()},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch live policies: status=%d body=%s", resp.StatusCode, env["message"])
	}

	var live kbstore.DraftView
	h.get("/xchats/api/v1/kb", &live)

	// Verify topic
	var topicFound bool
	for _, top := range live.Topics {
		if top.Slug == "live-topic" {
			topicFound = true
			if top.FeaturedImage == nil || *top.FeaturedImage != img {
				t.Errorf("live topic featured_image = %v, want %v", top.FeaturedImage, img)
			}
			if len(top.ReferenceDocuments) != 1 || top.ReferenceDocuments[0] != doc {
				t.Errorf("live topic reference_documents = %v, want [%v]", top.ReferenceDocuments, doc)
			}
			break
		}
	}
	if !topicFound {
		t.Errorf("live topic 'live-topic' not found in GET /kb")
	}

	// Verify tariff
	var tariffFound bool
	for _, tr := range live.Tariffs {
		if tr.Ref == "live-tariff" {
			tariffFound = true
			if len(tr.PricingImages) != 1 || tr.PricingImages[0] != img {
				t.Errorf("live tariff pricing_images = %v, want [%v]", tr.PricingImages, img)
			}
			if len(tr.TermsDocuments) != 1 || tr.TermsDocuments[0] != doc {
				t.Errorf("live tariff terms_documents = %v, want [%v]", tr.TermsDocuments, doc)
			}
			break
		}
	}
	if !tariffFound {
		t.Errorf("live tariff 'live-tariff' not found in GET /kb")
	}

	// Verify contacts
	if len(live.Contacts) != 1 {
		t.Fatalf("len(live.Contacts) = %d, want 1", len(live.Contacts))
	}
	if live.Contacts[0].ContactCardImage == nil || *live.Contacts[0].ContactCardImage != img {
		t.Errorf("live contacts contact_card_image = %v, want %v", live.Contacts[0].ContactCardImage, img)
	}
	if len(live.Contacts[0].CompanyLegalDocuments) != 1 || live.Contacts[0].CompanyLegalDocuments[0] != doc {
		t.Errorf("live contacts company_legal_documents = %v, want [%v]", live.Contacts[0].CompanyLegalDocuments, doc)
	}

	// Verify policies
	if len(live.Policies) != 1 {
		t.Fatalf("len(live.Policies) = %d, want 1", len(live.Policies))
	}
	if len(live.Policies[0].CommercePolicyDocuments) != 1 || live.Policies[0].CommercePolicyDocuments[0] != doc {
		t.Errorf("live policies commerce_policy_documents = %v, want [%v]", live.Policies[0].CommercePolicyDocuments, doc)
	}
}
