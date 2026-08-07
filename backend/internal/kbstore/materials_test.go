package kbstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// TestListLiveMaterials_ReturnsUploadMetadata proves the widened SELECT in
// listMaterials actually surfaces the upload-lineage columns (filename,
// mime_type, size_bytes, processing_status, has_content) that GET
// /kb/materials previously dropped — the frontend needs these to render a
// real thumbnail/player/download row instead of a bare paperclip count.
func TestListLiveMaterials_ReturnsUploadMetadata(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	completed := mkAttachMaterial(t, kb, orgID, "image/png", "visible", true)
	pending := mkAttachMaterial(t, kb, orgID, "application/pdf", "visible", false)

	materials, err := kb.ListLiveMaterials(ctx, orgID)
	if err != nil {
		t.Fatalf("ListLiveMaterials: %v", err)
	}
	byID := map[uuid.UUID]kbstore.Material{}
	for _, m := range materials {
		byID[m.ID] = m
	}

	got, ok := byID[completed]
	if !ok {
		t.Fatalf("completed material %s missing from list", completed)
	}
	if got.Filename != "f.bin" {
		t.Errorf("Filename = %q, want %q", got.Filename, "f.bin")
	}
	if got.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want %q", got.MimeType, "image/png")
	}
	if got.SizeBytes != 10 {
		t.Errorf("SizeBytes = %d, want 10", got.SizeBytes)
	}
	if got.ProcessingStatus != "parsed" {
		t.Errorf("ProcessingStatus = %q, want %q", got.ProcessingStatus, "parsed")
	}
	if got.CustomerVisibility != "visible" {
		t.Errorf("CustomerVisibility = %q, want %q", got.CustomerVisibility, "visible")
	}
	if !got.HasContent {
		t.Error("HasContent = false for a completed upload, want true")
	}

	gotPending, ok := byID[pending]
	if !ok {
		t.Fatalf("pending material %s missing from list", pending)
	}
	if gotPending.ProcessingStatus != "uploaded" {
		t.Errorf("pending ProcessingStatus = %q, want %q", gotPending.ProcessingStatus, "uploaded")
	}
	if gotPending.HasContent {
		t.Error("HasContent = true for an upload that never completed, want false")
	}
}

// TestListLiveMaterials_LegacyTextMaterialHasNoUploadMetadata proves the new
// columns degrade cleanly (empty/zero, not an error) for a material created
// through the older CreateMaterial(text) path, which never touches
// filename/mime_type/storage_key at all.
func TestListLiveMaterials_LegacyTextMaterialHasNoUploadMetadata(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if _, err := kb.CreateMaterial(ctx, orgID, kbstore.MaterialInput{
		SourceType: "text", SourceRef: "note", Text: "hello",
	}); err != nil {
		t.Fatalf("create text material: %v", err)
	}

	materials, err := kb.ListLiveMaterials(ctx, orgID)
	if err != nil {
		t.Fatalf("ListLiveMaterials: %v", err)
	}
	if len(materials) != 1 {
		t.Fatalf("len(materials) = %d, want 1", len(materials))
	}
	m := materials[0]
	if m.Filename != "" || m.MimeType != "" {
		t.Errorf("legacy text material has upload metadata: filename=%q mime=%q", m.Filename, m.MimeType)
	}
	if m.HasContent {
		t.Error("HasContent = true for a text material with no storage_key, want false")
	}
	if m.Status != "ready" {
		t.Errorf("Status = %q, want %q (text materials are born ready)", m.Status, "ready")
	}
}

// TestGetMaterialContentRef_ScopedToOrg is the content endpoint's core safety
// property: a material id from another organization must come back exactly
// like an id that does not exist at all — kbstore.ErrUnknownKind, nothing
// that would let a caller distinguish "not yours" from "not real".
func TestGetMaterialContentRef_ScopedToOrg(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	ctx := context.Background()

	otherOrg, err := st.SeedOrganization(ctx, "other-org")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}

	mine := mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", true)
	theirs := mkAttachMaterial(t, kb, otherOrg.ID, "image/jpeg", "visible", true)

	ref, err := kb.GetMaterialContentRef(ctx, orgID, mine)
	if err != nil {
		t.Fatalf("GetMaterialContentRef(mine): %v", err)
	}
	if ref.StorageKey == "" {
		t.Error("StorageKey is empty for a completed upload")
	}
	if ref.MimeType != "image/jpeg" {
		t.Errorf("MimeType = %q, want %q", ref.MimeType, "image/jpeg")
	}

	if _, err := kb.GetMaterialContentRef(ctx, orgID, theirs); !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Errorf("GetMaterialContentRef(theirs) err = %v, want ErrUnknownKind", err)
	}
	if _, err := kb.GetMaterialContentRef(ctx, orgID, uuid.New()); !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Errorf("GetMaterialContentRef(unknown) err = %v, want ErrUnknownKind", err)
	}
}

// TestGetMaterialContentRef_NoStoredBytesYet covers the "staged but the PUT
// never completed" case — storage_key stays empty, and the content endpoint
// (handleKBMaterialContent) is what turns that into a 404 rather than
// serving zero bytes.
func TestGetMaterialContentRef_NoStoredBytesYet(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	pending := mkAttachMaterial(t, kb, orgID, "image/jpeg", "visible", false)

	ref, err := kb.GetMaterialContentRef(ctx, orgID, pending)
	if err != nil {
		t.Fatalf("GetMaterialContentRef: %v", err)
	}
	if ref.StorageKey != "" {
		t.Errorf("StorageKey = %q, want empty for an incomplete upload", ref.StorageKey)
	}
}
