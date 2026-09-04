package kbstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// clearUUIDP returns a **uuid.UUID whose outer pointer is non-nil but whose
// inner value is nil — the explicit-clear shape *Media.FeaturedImage (and
// friends) expect, as distinct from a plain nil outer pointer (absent, no
// pending edit). uuidpp (mcp_test.go) is its "set to this id" counterpart.
func clearUUIDP() **uuid.UUID {
	var p *uuid.UUID
	return &p
}

// idsp packs ids into a *[]uuid.UUID — the "explicit set/replace/clear"
// shape every plural *Media field expects; a nil *[]uuid.UUID (simply
// omitting the field) means "no pending edit" instead.
func idsp(ids ...uuid.UUID) *[]uuid.UUID { return &ids }

// TestUpsertProduct_MediaAbsentPreservesMCPAuthored is Stage 3's own version
// of TestLegacyUpsertTopic_PreservesMCPAuthoredMedia for the PRODUCT typed
// lane: a draft-lane write that leaves ProductInput.Media at its zero value
// must not blank out gallery_images/featured_image an MCP tool already
// staged.
func TestUpsertProduct_MediaAbsentPreservesMCPAuthored(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	img := mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", true)

	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "coffee-machine", kbstore.ProductChanges{
		Name: strp("Кофемашина"), AvailabilityStatus: strp("in_stock"),
		FeaturedImage: uuidpp(img), GalleryImages: idsp(img),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed via MCP: %v", err)
	}

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "coffee-machine", Name: "Кофемашина (ред.)", Price: "150000",
	}); err != nil {
		t.Fatalf("legacy upsert: %v", err)
	}

	row := readProduct(t, kb, orgID, "coffee-machine")
	if row.FeaturedImage == nil || *row.FeaturedImage != img {
		t.Fatalf("legacy edit clobbered featured_image: %+v", row)
	}
	if len(row.GalleryImages) != 1 || row.GalleryImages[0] != img {
		t.Fatalf("legacy edit clobbered gallery_images: %+v", row)
	}
	if row.Name != "Кофемашина (ред.)" {
		t.Fatalf("legacy edit did not apply: %+v", row)
	}
}

// TestUpsertProduct_MediaSetReplacesWholeList proves an explicit non-nil
// GalleryImages replaces the whole slice (the "attach" semantics live at the
// picker level in the frontend; the store's contract is whole-slice
// replace).
func TestUpsertProduct_MediaSetReplacesWholeList(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	img1 := mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", true)
	img2 := mkAttachMaterial(t, kb, orgID, "image/png", "visible", true)

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", Media: kbstore.ProductMedia{GalleryImages: idsp(img1)},
	}); err != nil {
		t.Fatalf("create with gallery: %v", err)
	}
	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", Media: kbstore.ProductMedia{GalleryImages: idsp(img2)},
	}); err != nil {
		t.Fatalf("replace gallery: %v", err)
	}

	row := readProduct(t, kb, orgID, "p")
	if len(row.GalleryImages) != 1 || row.GalleryImages[0] != img2 {
		t.Fatalf("gallery_images = %v, want [%v]", row.GalleryImages, img2)
	}
}

// TestUpsertProduct_MediaEmptySliceDetachesAll proves an explicit EMPTY
// (non-nil) slice is detach-all, distinct from a nil (absent) slice.
func TestUpsertProduct_MediaEmptySliceDetachesAll(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	img := mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", true)

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", Media: kbstore.ProductMedia{GalleryImages: idsp(img)},
	}); err != nil {
		t.Fatalf("create with gallery: %v", err)
	}
	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", Media: kbstore.ProductMedia{GalleryImages: &[]uuid.UUID{}},
	}); err != nil {
		t.Fatalf("detach gallery: %v", err)
	}

	row := readProduct(t, kb, orgID, "p")
	if len(row.GalleryImages) != 0 {
		t.Fatalf("gallery_images = %v, want empty (detached)", row.GalleryImages)
	}
}

// TestUpsertProduct_FeaturedImageInnerNilClearsVsOuterNilPreserves is the
// pointer-semantics regression this whole stage is built around: outer nil
// (absent) leaves featured_image untouched, while a non-nil outer pointer
// to a nil inner value is an EXPLICIT clear.
func TestUpsertProduct_FeaturedImageInnerNilClearsVsOuterNilPreserves(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	img := mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", true)

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", Media: kbstore.ProductMedia{FeaturedImage: uuidpp(img)},
	}); err != nil {
		t.Fatalf("create with featured_image: %v", err)
	}

	// Outer nil (Media zero value) — absent, must preserve.
	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{Ref: "p", Name: "Товар v2"}); err != nil {
		t.Fatalf("text-only edit: %v", err)
	}
	row := readProduct(t, kb, orgID, "p")
	if row.FeaturedImage == nil || *row.FeaturedImage != img {
		t.Fatalf("absent Media blanked featured_image: %+v", row)
	}

	// Non-nil outer, nil inner — explicit clear.
	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар v3", Media: kbstore.ProductMedia{FeaturedImage: clearUUIDP()},
	}); err != nil {
		t.Fatalf("clear featured_image: %v", err)
	}
	row = readProduct(t, kb, orgID, "p")
	if row.FeaturedImage != nil {
		t.Fatalf("featured_image = %v, want nil (cleared)", *row.FeaturedImage)
	}
}

// TestUpsertProduct_RejectsCrossOrgMediaReference proves the draft lane runs
// the SAME validateMediaRef checks the MCP lane does — a foreign org's
// material must be rejected with ErrMediaReference, not silently accepted.
func TestUpsertProduct_RejectsCrossOrgMediaReference(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	ctx := context.Background()
	otherOrg, err := st.SeedOrganization(ctx, "org-b-media")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	foreignImg := mkAttachMaterial(t, kb, otherOrg.ID, "image/jpeg", "visible", true)

	err = kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", Media: kbstore.ProductMedia{FeaturedImage: uuidpp(foreignImg)},
	})
	var mediaErr *kbstore.ErrMediaReference
	if !errors.As(err, &mediaErr) {
		t.Fatalf("expected ErrMediaReference, got %v", err)
	}
}

// TestUpsertTopic_MediaSemantics is a second-type cross-check that
// applyTopicMedia shares the exact same set/clear/preserve contract
// applyProductMedia does — one shared implementation, not a per-type copy
// that could drift.
func TestUpsertTopic_MediaSemantics(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	doc := mkAttachMaterial(t, kb, orgID, "application/pdf", "visible", true)

	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "faq", Title: "FAQ", BodyMD: "Текст.",
		Media: kbstore.TopicMedia{ReferenceDocuments: idsp(doc)},
	}); err != nil {
		t.Fatalf("create with reference_documents: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "faq", Title: "FAQ v2", BodyMD: "Текст."}); err != nil {
		t.Fatalf("text-only edit: %v", err)
	}
	row := readTopic(t, kb, orgID, "faq")
	if len(row.ReferenceDocuments) != 1 || row.ReferenceDocuments[0] != doc {
		t.Fatalf("absent Media blanked reference_documents: %+v", row)
	}

	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "faq", Title: "FAQ v3", BodyMD: "Текст.",
		Media: kbstore.TopicMedia{ReferenceDocuments: &[]uuid.UUID{}},
	}); err != nil {
		t.Fatalf("detach reference_documents: %v", err)
	}
	row = readTopic(t, kb, orgID, "faq")
	if len(row.ReferenceDocuments) != 0 {
		t.Fatalf("reference_documents = %v, want empty (detached)", row.ReferenceDocuments)
	}
}

// TestPatchContacts_MediaSemantics covers the double-singular shape
// (contact_card_image, location_map_image) alongside a plural field, on the
// PATCH-only singleton lane.
func TestPatchContacts_MediaSemantics(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	card := mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", true)
	mapImg := mkAttachMaterial(t, kb, orgID, "image/png", "visible", true)

	if err := kb.PatchContacts(ctx, orgID, uuid.Nil, kbstore.ContactPatch{
		Media: kbstore.ContactsMedia{ContactCardImage: uuidpp(card), LocationMapImage: uuidpp(mapImg)},
	}); err != nil {
		t.Fatalf("set both images: %v", err)
	}
	// Absent Media on a text-only patch preserves both.
	phone := "+7 700 000 00 00"
	if err := kb.PatchContacts(ctx, orgID, uuid.Nil, kbstore.ContactPatch{Phone: &phone}); err != nil {
		t.Fatalf("text-only patch: %v", err)
	}
	row := readContact(t, kb, orgID)
	if row.ContactCardImage == nil || *row.ContactCardImage != card {
		t.Fatalf("absent Media blanked contact_card_image: %+v", row)
	}
	if row.LocationMapImage == nil || *row.LocationMapImage != mapImg {
		t.Fatalf("absent Media blanked location_map_image: %+v", row)
	}
	if row.Phone != phone {
		t.Fatalf("phone = %q, want %q", row.Phone, phone)
	}

	// Clear just contact_card_image; location_map_image must survive.
	if err := kb.PatchContacts(ctx, orgID, uuid.Nil, kbstore.ContactPatch{
		Media: kbstore.ContactsMedia{ContactCardImage: clearUUIDP()},
	}); err != nil {
		t.Fatalf("clear contact_card_image: %v", err)
	}
	row = readContact(t, kb, orgID)
	if row.ContactCardImage != nil {
		t.Fatalf("contact_card_image = %v, want nil (cleared)", *row.ContactCardImage)
	}
	if row.LocationMapImage == nil || *row.LocationMapImage != mapImg {
		t.Fatalf("clearing contact_card_image blanked location_map_image: %+v", row)
	}
}

// readProduct/readTopic/readContact read back the merged draft view of one
// record — the same ReadRecords path the MCP kb_read tool and the widget
// use, so these tests exercise the real merge instead of poking DraftBlob
// internals. types is passed explicitly (not left nil) so a singleton
// (contacts) doesn't come back as the assistant record — flattenRecords
// special-cases NaturalKeyMain for BOTH assistant and contacts/policies, so
// key alone cannot disambiguate them (mcp_read.go's flattenRecords).
func readProduct(t *testing.T, kb *kbstore.Store, orgID uuid.UUID, ref string) kbstore.ProductRow {
	t.Helper()
	page, err := kb.ReadRecords(context.Background(), orgID, []string{kbstore.KBTypeProduct}, "draft", ref, "", 0, "")
	if err != nil {
		t.Fatalf("read product %q: %v", ref, err)
	}
	if len(page.Items) == 0 {
		t.Fatalf("product %q not found", ref)
	}
	return page.Items[0].Data.(kbstore.ProductRow)
}

func readTopic(t *testing.T, kb *kbstore.Store, orgID uuid.UUID, slug string) kbstore.TopicRow {
	t.Helper()
	page, err := kb.ReadRecords(context.Background(), orgID, []string{kbstore.KBTypeTopic}, "draft", slug, "", 0, "")
	if err != nil {
		t.Fatalf("read topic %q: %v", slug, err)
	}
	if len(page.Items) == 0 {
		t.Fatalf("topic %q not found", slug)
	}
	return page.Items[0].Data.(kbstore.TopicRow)
}

func readContact(t *testing.T, kb *kbstore.Store, orgID uuid.UUID) kbstore.ContactRow {
	t.Helper()
	page, err := kb.ReadRecords(context.Background(), orgID, []string{kbstore.KBTypeContacts}, "draft", "", "", 0, "")
	if err != nil {
		t.Fatalf("read contacts: %v", err)
	}
	if len(page.Items) == 0 {
		t.Fatalf("contacts not found")
	}
	return page.Items[0].Data.(kbstore.ContactRow)
}
