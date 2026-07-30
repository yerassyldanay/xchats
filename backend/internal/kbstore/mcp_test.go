package kbstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }
func uuidpp(u uuid.UUID) **uuid.UUID {
	p := &u
	return &p
}

// TestMCPUpsertProduct_CreateRequiresInStock enforces plan/mcp.md §5's
// "in_stock is required on create".
func TestMCPUpsertProduct_CreateRequiresInStock(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	_, err := kb.MCPUpsertProduct(ctx, orgID, "", kbstore.ProductChanges{Name: strp("Кофемашина")}, nil, "")
	var missing *kbstore.ErrRequiredFieldMissing
	if !errors.As(err, &missing) {
		t.Fatalf("expected ErrRequiredFieldMissing, got %v", err)
	}
}

// TestMCPUpsertProduct_CreateDerivesSlugFromCyrillicTitle exercises the
// missing-key create path end to end: a Russian name becomes a stable ASCII
// ref, and the record is readable back with every field intact.
func TestMCPUpsertProduct_CreateDerivesSlugFromCyrillicTitle(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	res, err := kb.MCPUpsertProduct(ctx, orgID, "", kbstore.ProductChanges{
		Name: strp("Кофемашина DeLonghi"), Price: strp("129 900 ₸"), InStock: boolp(true),
	}, nil, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !res.Created || res.Key == "" {
		t.Fatalf("expected a created record with a derived key, got %+v", res)
	}
	for _, r := range res.Key {
		if r > 'z' && r != '-' {
			t.Fatalf("derived key %q is not ASCII-safe", res.Key)
		}
	}

	page, err := kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeProduct}, "draft", res.Key, "", 0, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly 1 draft product, got %d", len(page.Items))
	}
	row := page.Items[0].Data.(kbstore.ProductRow)
	if row.Name != "Кофемашина DeLonghi" || row.Price != "129 900 ₸" || !row.InStock {
		t.Fatalf("round-tripped row wrong: %+v", row)
	}
}

// TestMCPUpsertProduct_UpdateByKeyPreservesOmittedFields is the core partial-
// patch contract: "Omitted fields remain unchanged."
func TestMCPUpsertProduct_UpdateByKeyPreservesOmittedFields(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	res, err := kb.MCPUpsertProduct(ctx, orgID, "coffee-machine", kbstore.ProductChanges{
		Name: strp("Кофемашина"), Price: strp("100000"), Category: strp("Кухня"), InStock: boolp(true),
	}, nil, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Patch only price; name/category/in_stock must survive untouched.
	if _, err := kb.MCPUpsertProduct(ctx, orgID, res.Key, kbstore.ProductChanges{Price: strp("120000")}, nil, ""); err != nil {
		t.Fatalf("patch: %v", err)
	}
	page, err := kb.ReadRecords(ctx, orgID, nil, "draft", res.Key, "", 0, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	row := page.Items[0].Data.(kbstore.ProductRow)
	if row.Price != "120000" {
		t.Fatalf("price not updated: %+v", row)
	}
	if row.Name != "Кофемашина" || row.Category != "Кухня" || !row.InStock {
		t.Fatalf("omitted fields were clobbered: %+v", row)
	}
}

// TestMCPUpsertTariff_ExactTitleMatchUnderAnotherKeyIsConflict is plan/
// mcp.md §4 step 2.
func TestMCPUpsertTariff_ExactTitleMatchUnderAnotherKeyIsConflict(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "biz", kbstore.TariffChanges{Name: strp("Business"), PricingType: strp("fixed")}, nil, ""); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}
	_, err := kb.MCPUpsertTariff(ctx, orgID, "", kbstore.TariffChanges{Name: strp("  business  "), PricingType: strp("fixed")}, nil, "")
	var conflict *kbstore.ErrDuplicateConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ErrDuplicateConflict for a normalized-equal title, got %v", err)
	}
	if conflict.ExistingKey != "biz" {
		t.Fatalf("conflict should point at the existing key, got %q", conflict.ExistingKey)
	}
}

// TestMCPUpsertTariff_SimilarTitleUnderAnotherKeyIsAmbiguous is step 3.
func TestMCPUpsertTariff_SimilarTitleUnderAnotherKeyIsAmbiguous(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "biz", kbstore.TariffChanges{Name: strp("Business"), PricingType: strp("fixed")}, nil, ""); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}
	_, err := kb.MCPUpsertTariff(ctx, orgID, "", kbstore.TariffChanges{Name: strp("Business Pro"), PricingType: strp("fixed")}, nil, "")
	var ambiguous *kbstore.ErrAmbiguousMatch
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected ErrAmbiguousMatch for a similar title, got %v", err)
	}
	if len(ambiguous.Candidates) != 1 || ambiguous.Candidates[0].Key != "biz" {
		t.Fatalf("expected candidate 'biz', got %+v", ambiguous.Candidates)
	}
}

// TestMCPUpsertTariff_DistinctTitleCreatesCleanly ensures the duplicate
// checks don't false-positive on ordinary distinct names.
func TestMCPUpsertTariff_DistinctTitleCreatesCleanly(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "biz", kbstore.TariffChanges{Name: strp("Business"), PricingType: strp("fixed")}, nil, ""); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}
	res, err := kb.MCPUpsertTariff(ctx, orgID, "", kbstore.TariffChanges{Name: strp("Growth"), PricingType: strp("fixed")}, nil, "")
	if err != nil {
		t.Fatalf("expected clean create, got %v", err)
	}
	if !res.Created || res.Key == "biz" {
		t.Fatalf("expected a distinct new record, got %+v", res)
	}
}

// TestMCPUpsertZone_CreateRequiresDeliveryAvailable enforces the third
// create-required field.
func TestMCPUpsertZone_CreateRequiresDeliveryAvailable(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	_, err := kb.MCPUpsertDeliveryZone(ctx, orgID, "almaty", kbstore.DeliveryZoneChanges{
		Name: strp("Алматы"), ZoneLevel: strp("city"),
	}, nil, "")
	var missing *kbstore.ErrRequiredFieldMissing
	if !errors.As(err, &missing) {
		t.Fatalf("expected ErrRequiredFieldMissing for delivery_available, got %v", err)
	}
}

// TestMCPUpsertAndApprove_ZoneWorldStillEnforcedAtApprove confirms Approve
// rejects a zone that violates the delivery_available/cost/days invariant,
// even though it was accepted as a syntactically valid draft write.
func TestMCPUpsertAndApprove_ZoneWorldStillEnforcedAtApprove(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertDeliveryZone(ctx, orgID, "almaty", kbstore.DeliveryZoneChanges{
		Name: strp("Алматы"), ZoneLevel: strp("city"), DeliveryAvailable: boolp(true),
		// delivery_cost/delivery_in_days omitted — invalid for an available zone.
	}, nil, ""); err != nil {
		t.Fatalf("draft write: %v", err)
	}
	err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{})
	var gerr *kbstore.GateError
	if !errors.As(err, &gerr) {
		t.Fatalf("expected GateError from the zone-world invariant, got %v", err)
	}
}

// TestMCPUpsertZone_ApprovePublishesCompleteRow is the full happy path: a
// valid zone draft materializes into ai_delivery_zones with every field.
func TestMCPUpsertZone_ApprovePublishesCompleteRow(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()
	// A zone world needs a zone-compatible policy first (blank flat delivery
	// fields + a non-blank outside_zones_note) — same invariant zones.go
	// documents.
	if _, err := kb.MCPUpsertPolicies(ctx, orgID, kbstore.PoliciesChanges{
		OutsideZonesNote: strp("Мы не доставляем за пределы списка зон."),
	}, nil, ""); err != nil {
		t.Fatalf("seed policy: %v", err)
	}
	res, err := kb.MCPUpsertDeliveryZone(ctx, orgID, "almaty", kbstore.DeliveryZoneChanges{
		Name: strp("Алматы"), ZoneLevel: strp("city"), DeliveryAvailable: boolp(true),
		DeliveryCost: strp("5 000 ₸"), DeliveryInDays: strp("1"),
	}, nil, "")
	if err != nil {
		t.Fatalf("draft zone: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var name, cost string
	var available bool
	if err := st.Pool().QueryRow(ctx, `SELECT name, delivery_available, delivery_cost FROM xchats.ai_delivery_zones
		WHERE organization_id=$1 AND ref=$2`, orgID, res.Key).Scan(&name, &available, &cost); err != nil {
		t.Fatalf("read back live zone: %v", err)
	}
	if name != "Алматы" || !available || cost != "5 000 ₸" {
		t.Fatalf("live zone row wrong: name=%q available=%v cost=%q", name, available, cost)
	}
}

// TestMCPUpsertProduct_MediaValidation_RejectsCrossOrgAndWrongMimeAndInvisible
// covers plan/mcp.md §9's "same-organization media references" backstop.
func TestMCPUpsertProduct_MediaValidation_Rejects(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()
	otherOrg, err := st.SeedOrganization(ctx, "other-org")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}

	mkMaterial := func(org uuid.UUID, mime, visibility string, uploaded bool) uuid.UUID {
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

	crossOrg := mkMaterial(otherOrg.ID, "image/jpeg", "visible", true)
	wrongMime := mkMaterial(orgID, "application/pdf", "visible", true)
	invisible := mkMaterial(orgID, "image/jpeg", "invisible", true)
	notUploaded := mkMaterial(orgID, "image/jpeg", "visible", false)
	valid := mkMaterial(orgID, "image/jpeg", "visible", true)

	cases := []struct {
		name string
		id   uuid.UUID
	}{
		{"cross-org", crossOrg},
		{"wrong-mime", wrongMime},
		{"invisible", invisible},
		{"not-uploaded", notUploaded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := kb.MCPUpsertProduct(ctx, orgID, "p-"+c.name, kbstore.ProductChanges{
				Name: strp("Товар"), InStock: boolp(true), FeaturedImage: uuidpp(c.id),
			}, nil, "")
			var mediaErr *kbstore.ErrMediaReference
			if !errors.As(err, &mediaErr) {
				t.Fatalf("expected ErrMediaReference, got %v", err)
			}
		})
	}

	if _, err := kb.MCPUpsertProduct(ctx, orgID, "p-valid", kbstore.ProductChanges{
		Name: strp("Товар"), InStock: boolp(true), FeaturedImage: uuidpp(valid),
	}, nil, ""); err != nil {
		t.Fatalf("expected the valid material to be accepted, got %v", err)
	}
}

// TestMCPUpsert_OptimisticConcurrency covers expected_draft_version.
func TestMCPUpsert_OptimisticConcurrency(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	res, err := kb.MCPUpsertTopic(ctx, orgID, "how-to-order", kbstore.TopicChanges{
		Title: strp("Как заказать"), BodyMD: strp("Напишите нам."),
	}, nil, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stale := res.DraftVersion // now stale — a write already advanced past this
	if _, err := kb.MCPUpsertTopic(ctx, orgID, "how-to-order", kbstore.TopicChanges{Title: strp("v2")}, nil, ""); err != nil {
		t.Fatalf("advance version: %v", err)
	}
	_, err = kb.MCPUpsertTopic(ctx, orgID, "how-to-order", kbstore.TopicChanges{Title: strp("v3")}, &stale, "")
	if !errors.Is(err, kbstore.ErrStale) {
		t.Fatalf("expected ErrStale for a stale expected_draft_version, got %v", err)
	}
	current, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if _, err := kb.MCPUpsertTopic(ctx, orgID, "how-to-order", kbstore.TopicChanges{Title: strp("v3")}, &current, ""); err != nil {
		t.Fatalf("expected success with the correct expected_draft_version, got %v", err)
	}
}

// TestIdentityIndex_StateTransitions exercises published/new/changed/
// to_delete across live+draft.
func TestIdentityIndex_StateTransitions(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	// live-only ("published"): write directly then approve.
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "published-only", kbstore.TariffChanges{Name: strp("Опубликован"), PricingType: strp("fixed")}, nil, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// draft-only ("new").
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "new-only", kbstore.TariffChanges{Name: strp("Новый"), PricingType: strp("fixed")}, nil, ""); err != nil {
		t.Fatalf("seed new: %v", err)
	}
	// both ("changed"): edit the already-published one.
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "published-only", kbstore.TariffChanges{Summary: strp("обновлено")}, nil, ""); err != nil {
		t.Fatalf("edit published: %v", err)
	}
	// to_delete: mark the PUBLISHED entity for deletion — it still has a live
	// row, so it must keep showing up (as to_delete, not vanish).
	if _, err := kb.MCPDelete(ctx, orgID, kbstore.KBTypeTariff, "published-only", nil); err != nil {
		t.Fatalf("delete published: %v", err)
	}

	index, err := kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTariff})
	if err != nil {
		t.Fatalf("identity index: %v", err)
	}
	states := map[string]string{}
	for _, id := range index {
		states[id.Key] = id.State()
	}
	if states["published-only"] != "to_delete" {
		t.Fatalf("expected 'to_delete' for a live entity marked for deletion, got %q", states["published-only"])
	}
	if states["new-only"] != "new" {
		t.Fatalf("expected 'new', got %q", states["new-only"])
	}

	// Deleting a draft-only (never-published) entity removes it outright —
	// there is nothing left to represent once its only draft entry is gone.
	if _, err := kb.MCPDelete(ctx, orgID, kbstore.KBTypeTariff, "new-only", nil); err != nil {
		t.Fatalf("delete new-only: %v", err)
	}
	index2, err := kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTariff})
	if err != nil {
		t.Fatalf("identity index: %v", err)
	}
	for _, id := range index2 {
		if id.Key == "new-only" {
			t.Fatalf("expected 'new-only' to vanish after deleting a draft-only entity, got %+v", id)
		}
	}
}

// TestReadRecords_SourceBothKeepsLiveAndDraftSeparate is plan/mcp.md §5's
// explicit requirement: "There is no `effective` source."
func TestReadRecords_SourceBothKeepsLiveAndDraftSeparate(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "biz", kbstore.TariffChanges{Name: strp("Business"), Price: strp("v1"), PricingType: strp("fixed")}, nil, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := kb.MCPUpsertTariff(ctx, orgID, "biz", kbstore.TariffChanges{Price: strp("v2")}, nil, ""); err != nil {
		t.Fatalf("edit: %v", err)
	}

	page, err := kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeTariff}, "both", "biz", "", 0, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected both a live and a draft record, got %d", len(page.Items))
	}
	bySource := map[string]kbstore.TariffRow{}
	for _, item := range page.Items {
		bySource[item.Source] = item.Data.(kbstore.TariffRow)
	}
	if bySource["live"].Price != "v1" {
		t.Fatalf("live record should still show v1, got %+v", bySource["live"])
	}
	if bySource["draft"].Price != "v2" {
		t.Fatalf("draft record should show v2, got %+v", bySource["draft"])
	}
}

// TestMCPDelete_RejectsAssistantAndWrongSingletonKey covers kb_delete's
// validation ("blocks a delete that would make the publishable KB invalid").
func TestMCPDelete_RejectsAssistantAndWrongSingletonKey(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPDelete(ctx, orgID, kbstore.KBTypeAssistant, kbstore.NaturalKeyMain, nil); err == nil {
		t.Fatal("expected the assistant singleton to be non-deletable")
	}
	if _, err := kb.MCPDelete(ctx, orgID, kbstore.KBTypeContacts, "not-main", nil); err == nil {
		t.Fatal("expected a wrong singleton key to be rejected")
	}
}

// TestLegacyUpsertTopic_PreservesMCPAuthoredMedia is the regression guard for
// the draft.go/live.go merge-instead-of-replace fix: an old-UI-style plain
// text edit must not blank out media an MCP tool already staged.
func TestLegacyUpsertTopic_PreservesMCPAuthoredMedia(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	materialID, err := kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: "hero.jpg", MimeType: "image/jpeg", SizeBytes: 5, CustomerVisibility: "visible",
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	if err := kb.CompleteMaterialUpload(ctx, materialID, "disk", "org/x/"+materialID.String(), 5, ""); err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if _, err := kb.MCPUpsertTopic(ctx, orgID, "how-to-order", kbstore.TopicChanges{
		Title: strp("Как заказать"), BodyMD: strp("Текст."), FeaturedImage: uuidpp(materialID),
	}, nil, ""); err != nil {
		t.Fatalf("mcp upsert: %v", err)
	}

	// The legacy whole-row Playground path edits only text fields.
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{Slug: "how-to-order", Title: "Как заказать (ред.)", BodyMD: "Текст."}); err != nil {
		t.Fatalf("legacy upsert: %v", err)
	}

	page, err := kb.ReadRecords(ctx, orgID, nil, "draft", "how-to-order", "", 0, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	row := page.Items[0].Data.(kbstore.TopicRow)
	if row.FeaturedImage == nil || *row.FeaturedImage != materialID {
		t.Fatalf("legacy edit clobbered MCP-authored media: %+v", row)
	}
	if row.Title != "Как заказать (ред.)" {
		t.Fatalf("legacy edit did not apply: %+v", row)
	}
}
