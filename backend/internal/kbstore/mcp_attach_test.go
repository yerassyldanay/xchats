package kbstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// mkAttachMaterial stages (and optionally completes) a kbd_materials row for
// attach tests, mirroring TestMCPUpsertProduct_MediaValidation_Rejects'
// mkMaterial helper.
func mkAttachMaterial(t *testing.T, kb *kbstore.Store, org uuid.UUID, mime, visibility string, uploaded bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id, err := kb.CreateUploadMaterial(ctx, org, kbstore.UploadMaterialInput{
		Filename: "f.bin", MimeType: mime, SizeBytes: 10, CustomerVisibility: visibility,
	})
	if err != nil {
		t.Fatalf("create upload material: %v", err)
	}
	if uploaded {
		if err := kb.CompleteMaterialUpload(ctx, id, "disk", "org/"+org.String()+"/"+id.String(), 10, ""); err != nil {
			t.Fatalf("complete upload: %v", err)
		}
	}
	return id
}

func mkImageMaterial(t *testing.T, kb *kbstore.Store, org uuid.UUID) uuid.UUID {
	return mkAttachMaterial(t, kb, org, "image/jpeg", "visible", true)
}

// TestMCPAttachMedia_RejectsInvalidFieldPairing covers both halves of the
// registry check: a field that is real but belongs to a DIFFERENT type, and
// featured_image, which belongs to no type (it is never an attachment
// target — see mediaAttachmentFields' doc comment).
func TestMCPAttachMedia_RejectsInvalidFieldPairing(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	material := mkImageMaterial(t, kb, orgID)

	cases := []struct {
		name  string
		kbTyp string
		field string
	}{
		{"field belongs to a different type", kbstore.KBTypeTopic, "gallery_images"},
		{"featured_image is never an attachment target", kbstore.KBTypeProduct, "featured_image"},
		{"unknown field", kbstore.KBTypeProduct, "not_a_real_field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
				MaterialID: material, Type: c.kbTyp, Key: "whatever", Field: c.field,
			}, nil)
			var attachErr *kbstore.ErrAttachTarget
			if !errors.As(err, &attachErr) {
				t.Fatalf("expected ErrAttachTarget, got %v", err)
			}
		})
	}
}

// TestMCPAttachMedia_RejectsMissingRecord confirms attaching never creates a
// record — an unknown key is rejected, not silently upserted (the exact
// behavior that used to surface as "pricing_type is required to create this
// record" when the widget's old read-modify-upsert flow guessed wrong).
func TestMCPAttachMedia_RejectsMissingRecord(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	material := mkImageMaterial(t, kb, orgID)

	_, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: material, Type: kbstore.KBTypeProduct, Key: "does-not-exist", Field: "gallery_images",
	}, nil)
	var attachErr *kbstore.ErrAttachTarget
	if !errors.As(err, &attachErr) {
		t.Fatalf("expected ErrAttachTarget, got %v", err)
	}

	page, rerr := kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeProduct}, "both", "does-not-exist", "", 0, "")
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	if len(page.Items) != 0 {
		t.Fatalf("expected the attach attempt to create nothing, got %+v", page.Items)
	}
}

// TestMCPAttachMedia_RejectsRecordStagedForDeletion covers a live record
// that has been marked for deletion in the draft — kb_media_attach must
// refuse it exactly like a nonexistent record, not silently resurrect it.
func TestMCPAttachMedia_RejectsRecordStagedForDeletion(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "doomed", kbstore.ProductChanges{
		Name: strp("Товар"), AvailabilityStatus: strp("in_stock"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeProduct, "doomed", nil); err != nil {
		t.Fatalf("stage delete: %v", err)
	}

	material := mkImageMaterial(t, kb, orgID)
	_, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: material, Type: kbstore.KBTypeProduct, Key: "doomed", Field: "gallery_images",
	}, nil)
	var attachErr *kbstore.ErrAttachTarget
	if !errors.As(err, &attachErr) {
		t.Fatalf("expected ErrAttachTarget, got %v", err)
	}
}

// TestMCPAttachMedia_MediaValidation covers the material-side checks
// (validateMediaRef, reused unchanged from the typed-upsert path): foreign
// org, MIME mismatch, invisible, and incomplete upload must all be rejected
// as ErrMediaReference — not ErrAttachTarget, which is reserved for problems
// with the TARGET, not the material.
func TestMCPAttachMedia_MediaValidation(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	ctx := context.Background()
	otherOrg, err := st.SeedOrganization(ctx, "other-org")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "p", kbstore.ProductChanges{
		Name: strp("Товар"), AvailabilityStatus: strp("in_stock"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create product: %v", err)
	}

	cases := []struct {
		name string
		id   uuid.UUID
	}{
		{"cross-org", mkAttachMaterial(t, kb, otherOrg.ID, "image/jpeg", "visible", true)},
		{"wrong-mime", mkAttachMaterial(t, kb, orgID, "application/pdf", "visible", true)},
		{"invisible", mkAttachMaterial(t, kb, orgID, "image/jpeg", "invisible", true)},
		{"not-uploaded", mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", false)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
				MaterialID: c.id, Type: kbstore.KBTypeProduct, Key: "p", Field: "gallery_images",
			}, nil)
			var mediaErr *kbstore.ErrMediaReference
			if !errors.As(err, &mediaErr) {
				t.Fatalf("expected ErrMediaReference, got %v", err)
			}
		})
	}
}

// TestMCPAttachMedia_PluralAppendsWithoutDuplicates covers the first,
// subsequent, and duplicate attach to the same plural field.
func TestMCPAttachMedia_PluralAppendsWithoutDuplicates(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "p", kbstore.ProductChanges{
		Name: strp("Товар"), AvailabilityStatus: strp("in_stock"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create product: %v", err)
	}
	first := mkImageMaterial(t, kb, orgID)
	second := mkImageMaterial(t, kb, orgID)

	res, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: first, Type: kbstore.KBTypeProduct, Key: "p", Field: "gallery_images",
	}, nil)
	if err != nil {
		t.Fatalf("first attach: %v", err)
	}
	if res.AlreadyPresent {
		t.Fatalf("first attach should not report already_present")
	}
	gallery := readProductGallery(t, kb, orgID, "p")
	if len(gallery) != 1 || gallery[0] != first {
		t.Fatalf("expected gallery=[%s], got %v", first, gallery)
	}

	// Duplicate: attaching the SAME material again must be a no-op, not a
	// second entry.
	res, err = kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: first, Type: kbstore.KBTypeProduct, Key: "p", Field: "gallery_images",
	}, nil)
	if err != nil {
		t.Fatalf("duplicate attach: %v", err)
	}
	if !res.AlreadyPresent {
		t.Fatalf("expected already_present=true for a duplicate attach")
	}
	gallery = readProductGallery(t, kb, orgID, "p")
	if len(gallery) != 1 {
		t.Fatalf("duplicate attach must not append — expected 1 entry, got %v", gallery)
	}

	// Subsequent: a DIFFERENT material appends.
	res, err = kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: second, Type: kbstore.KBTypeProduct, Key: "p", Field: "gallery_images",
	}, nil)
	if err != nil {
		t.Fatalf("second attach: %v", err)
	}
	if res.AlreadyPresent {
		t.Fatalf("second attach should not report already_present")
	}
	gallery = readProductGallery(t, kb, orgID, "p")
	if len(gallery) != 2 || gallery[0] != first || gallery[1] != second {
		t.Fatalf("expected gallery=[%s,%s], got %v", first, second, gallery)
	}
}

// TestMCPAttachMedia_SingularReplaces covers a singular field (contacts'
// contact_card_image): a second attach replaces rather than appending, and
// works on a fresh org that has never written a contacts row before — the
// singleton is not required to pre-exist (see MCPAttachMedia's doc comment).
func TestMCPAttachMedia_SingularReplaces(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	first := mkImageMaterial(t, kb, orgID)
	second := mkImageMaterial(t, kb, orgID)

	res, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: first, Type: kbstore.KBTypeContacts, Key: kbstore.NaturalKeyMain, Field: "contact_card_image",
	}, nil)
	if err != nil {
		t.Fatalf("first attach on a never-before-written singleton: %v", err)
	}
	if res.Key != kbstore.NaturalKeyMain {
		t.Fatalf("expected key=main, got %q", res.Key)
	}

	page, err := kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeContacts}, "draft", kbstore.NaturalKeyMain, "", 0, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one draft contacts record, got %d", len(page.Items))
	}
	row := page.Items[0].Data.(kbstore.ContactRow)
	if row.ContactCardImage == nil || *row.ContactCardImage != first {
		t.Fatalf("expected contact_card_image=%s, got %+v", first, row.ContactCardImage)
	}

	if _, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: second, Type: kbstore.KBTypeContacts, Key: kbstore.NaturalKeyMain, Field: "contact_card_image",
	}, nil); err != nil {
		t.Fatalf("replace attach: %v", err)
	}
	page, err = kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeContacts}, "draft", kbstore.NaturalKeyMain, "", 0, "")
	if err != nil {
		t.Fatalf("read after replace: %v", err)
	}
	row = page.Items[0].Data.(kbstore.ContactRow)
	if row.ContactCardImage == nil || *row.ContactCardImage != second {
		t.Fatalf("expected contact_card_image replaced with %s, got %+v", second, row.ContactCardImage)
	}
}

// TestMCPAttachMedia_PoliciesSingletonFirstWrite is the same "no prior row"
// case as TestMCPAttachMedia_SingularReplaces, for policies' one plural
// field — the singleton exemption applies regardless of cardinality.
func TestMCPAttachMedia_PoliciesSingletonFirstWrite(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	material := mkAttachMaterial(t, kb, orgID, "application/pdf", "visible", true)

	if _, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: material, Type: kbstore.KBTypePolicies, Key: kbstore.NaturalKeyMain, Field: "commerce_policy_documents",
	}, nil); err != nil {
		t.Fatalf("first attach on a never-before-written policies singleton: %v", err)
	}
}

// TestMCPAttachMedia_WritesOnlyDraftAndReturnsNewVersion confirms the write
// lands in kbd_draft only (the live row is untouched until Approve) and the
// returned draft_version matches the store's own counter.
func TestMCPAttachMedia_WritesOnlyDraftAndReturnsNewVersion(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "p", kbstore.ProductChanges{
		Name: strp("Товар"), AvailabilityStatus: strp("in_stock"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	material := mkImageMaterial(t, kb, orgID)

	res, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
		MaterialID: material, Type: kbstore.KBTypeProduct, Key: "p", Field: "gallery_images",
	}, nil)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	wantVersion, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if res.DraftVersion != wantVersion {
		t.Fatalf("expected draft_version=%d, got %d", wantVersion, res.DraftVersion)
	}

	livePage, err := kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeProduct}, "live", "p", "", 0, "")
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	liveRow := livePage.Items[0].Data.(kbstore.ProductRow)
	if len(liveRow.GalleryImages) != 0 {
		t.Fatalf("expected the LIVE row untouched by an attach, got %v", liveRow.GalleryImages)
	}

	draftPage, err := kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeProduct}, "draft", "p", "", 0, "")
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	draftRow := draftPage.Items[0].Data.(kbstore.ProductRow)
	if len(draftRow.GalleryImages) != 1 || draftRow.GalleryImages[0] != material {
		t.Fatalf("expected the DRAFT row to carry the attach, got %v", draftRow.GalleryImages)
	}
}

func readProductGallery(t *testing.T, kb *kbstore.Store, orgID uuid.UUID, ref string) []uuid.UUID {
	t.Helper()
	page, err := kb.ReadRecords(context.Background(), orgID, []string{kbstore.KBTypeProduct}, "draft", ref, "", 0, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one draft product record, got %d", len(page.Items))
	}
	row := page.Items[0].Data.(kbstore.ProductRow)
	return row.GalleryImages
}

// TestMaterialPreviews_ScopedToOrg confirms the org scoping is real: another
// organization's material must simply not come back, so the widget renders
// "preview unavailable" rather than leaking a filename across a tenant
// boundary.
func TestMaterialPreviews_ScopedToOrg(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	ctx := context.Background()

	otherOrg, err := st.SeedOrganization(ctx, "other-org")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}

	mine := mkAttachMaterial(t, kb, orgID, "image/png", "visible", true)
	theirs := mkAttachMaterial(t, kb, otherOrg.ID, "image/png", "visible", true)
	pending := mkAttachMaterial(t, kb, orgID, "image/png", "visible", false) // no bytes yet

	got, perr := kb.MaterialPreviews(ctx, orgID, []uuid.UUID{mine, theirs, pending})
	if perr != nil {
		t.Fatalf("previews: %v", perr)
	}
	if _, ok := got[mine]; !ok {
		t.Error("own completed material must be present")
	}
	if _, ok := got[theirs]; ok {
		t.Error("another organization's material must NOT be present")
	}
	if _, ok := got[pending]; ok {
		t.Error("a material whose upload never completed must NOT be present")
	}
	if p := got[mine]; p.Kind != "image" || p.MimeType != "image/png" {
		t.Errorf("unexpected preview: %+v", p)
	}
}

// TestMediaIDsIn_CoversEveryMediaColumn drives real records through the real
// read path and asserts every recognised media column is collected.
//
// It deliberately does NOT hand-build structs: KBRecord.Data holds DraftView's
// *Row types (TopicRow, ProductRow, …), not the same-named Draft* blob types,
// and an earlier version of this test constructed the blob types — so it
// passed green against a MediaIDsIn that in fact collected nothing at all.
// Going through ReadRecords is what makes the test able to fail.
func TestMediaIDsIn_CoversEveryMediaColumn(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	// One material per kind — validateMediaRef enforces the pairing, so a
	// document cannot be parked in an image column just to make a test pass.
	byKind := map[string]uuid.UUID{
		"image":    mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", true),
		"video":    mkAttachMaterial(t, kb, orgID, "video/mp4", "visible", true),
		"audio":    mkAttachMaterial(t, kb, orgID, "audio/mpeg", "visible", true),
		"document": mkAttachMaterial(t, kb, orgID, "application/pdf", "visible", true),
	}

	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "t", kbstore.TopicChanges{
		Title: strp("Тема"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "p", kbstore.ProductChanges{
		Name: strp("Товар"), AvailabilityStatus: strp("in_stock"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create product: %v", err)
	}
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "tar", kbstore.TariffChanges{
		Name: strp("Тариф"), PricingType: strp("fixed"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create tariff: %v", err)
	}

	// Every attachable column, via the real attach path.
	want := map[uuid.UUID]bool{}
	for kbType, key := range map[string]string{
		kbstore.KBTypeTopic: "t", kbstore.KBTypeProduct: "p", kbstore.KBTypeTariff: "tar",
		kbstore.KBTypeContacts: kbstore.NaturalKeyMain, kbstore.KBTypePolicies: kbstore.NaturalKeyMain,
	} {
		for _, def := range kbstore.MediaAttachmentFieldsByType()[kbType] {
			id := byKind[def.Kind]
			if _, err := kb.MCPAttachMedia(ctx, orgID, uuid.Nil, kbstore.MediaAttachInput{
				MaterialID: id, Type: kbType, Key: key, Field: def.Field,
			}, nil); err != nil {
				t.Fatalf("attach %s.%s: %v", kbType, def.Field, err)
			}
			want[id] = true
		}
	}

	// featured_image is not attachable, so set it the only way it can be set —
	// through the typed upsert. It must still be COLLECTED for preview.
	featured := mkAttachMaterial(t, kb, orgID, "image/png", "visible", true)
	fp := &featured
	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "p", kbstore.ProductChanges{
		FeaturedImage: &fp,
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("set featured_image: %v", err)
	}
	want[featured] = true

	page, err := kb.ReadRecords(ctx, orgID, nil, "draft", "", "", 100, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := map[uuid.UUID]bool{}
	for _, id := range kbstore.MediaIDsIn(page) {
		got[id] = true
	}
	for id := range want {
		if !got[id] {
			t.Errorf("MediaIDsIn missed material %s — some media column is not collected", id)
		}
	}
	if len(got) == 0 {
		t.Fatal("MediaIDsIn collected nothing at all — it is almost certainly switching on the wrong types")
	}
}
