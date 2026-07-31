package kbstore_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

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
	_, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "", kbstore.ProductChanges{Name: strp("Кофемашина")}, nil, kbstore.MCPProvenance{})
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
	res, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "", kbstore.ProductChanges{
		Name: strp("Кофемашина DeLonghi"), Price: strp("129 900 ₸"), InStock: boolp(true),
	}, nil, kbstore.MCPProvenance{})
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
	res, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "coffee-machine", kbstore.ProductChanges{
		Name: strp("Кофемашина"), Price: strp("100000"), Category: strp("Кухня"), InStock: boolp(true),
	}, nil, kbstore.MCPProvenance{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Patch only price; name/category/in_stock must survive untouched.
	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, res.Key, kbstore.ProductChanges{Price: strp("120000")}, nil, kbstore.MCPProvenance{}); err != nil {
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
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{Name: strp("Business"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}
	_, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "", kbstore.TariffChanges{Name: strp("  business  "), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{})
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
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{Name: strp("Business"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}
	_, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "", kbstore.TariffChanges{Name: strp("Business Pro"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{})
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
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{Name: strp("Business"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}
	res, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "", kbstore.TariffChanges{Name: strp("Growth"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{})
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
	_, err := kb.MCPUpsertDeliveryZone(ctx, orgID, uuid.Nil, "almaty", kbstore.DeliveryZoneChanges{
		Name: strp("Алматы"), ZoneLevel: strp("city"),
	}, nil, kbstore.MCPProvenance{})
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
	if _, err := kb.MCPUpsertDeliveryZone(ctx, orgID, uuid.Nil, "almaty", kbstore.DeliveryZoneChanges{
		Name: strp("Алматы"), ZoneLevel: strp("city"), DeliveryAvailable: boolp(true),
		// delivery_cost/delivery_in_days omitted — invalid for an available zone.
	}, nil, kbstore.MCPProvenance{}); err != nil {
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
	if _, err := kb.MCPUpsertPolicies(ctx, orgID, uuid.Nil, kbstore.PoliciesChanges{
		OutsideZonesNote: strp("Мы не доставляем за пределы списка зон."),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed policy: %v", err)
	}
	res, err := kb.MCPUpsertDeliveryZone(ctx, orgID, uuid.Nil, "almaty", kbstore.DeliveryZoneChanges{
		Name: strp("Алматы"), ZoneLevel: strp("city"), DeliveryAvailable: boolp(true),
		DeliveryCost: strp("5 000 ₸"), DeliveryInDays: strp("1"),
	}, nil, kbstore.MCPProvenance{})
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
			_, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "p-"+c.name, kbstore.ProductChanges{
				Name: strp("Товар"), InStock: boolp(true), FeaturedImage: uuidpp(c.id),
			}, nil, kbstore.MCPProvenance{})
			var mediaErr *kbstore.ErrMediaReference
			if !errors.As(err, &mediaErr) {
				t.Fatalf("expected ErrMediaReference, got %v", err)
			}
		})
	}

	if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "p-valid", kbstore.ProductChanges{
		Name: strp("Товар"), InStock: boolp(true), FeaturedImage: uuidpp(valid),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("expected the valid material to be accepted, got %v", err)
	}
}

// TestMCPUpsert_OptimisticConcurrency covers expected_draft_version.
func TestMCPUpsert_OptimisticConcurrency(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	res, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "how-to-order", kbstore.TopicChanges{
		Title: strp("Как заказать"), BodyMD: strp("Напишите нам."),
	}, nil, kbstore.MCPProvenance{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stale := res.DraftVersion // now stale — a write already advanced past this
	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "how-to-order", kbstore.TopicChanges{Title: strp("v2")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("advance version: %v", err)
	}
	_, err = kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "how-to-order", kbstore.TopicChanges{Title: strp("v3")}, &stale, kbstore.MCPProvenance{})
	if !errors.Is(err, kbstore.ErrStale) {
		t.Fatalf("expected ErrStale for a stale expected_draft_version, got %v", err)
	}
	current, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "how-to-order", kbstore.TopicChanges{Title: strp("v3")}, &current, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("expected success with the correct expected_draft_version, got %v", err)
	}
}

// TestIdentityIndex_StateTransitions exercises published/new/changed/
// to_delete across live+draft.
func TestIdentityIndex_StateTransitions(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	// live-only ("published"): write directly then approve.
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "published-only", kbstore.TariffChanges{Name: strp("Опубликован"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// draft-only ("new").
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "new-only", kbstore.TariffChanges{Name: strp("Новый"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed new: %v", err)
	}
	// both ("changed"): edit the already-published one.
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "published-only", kbstore.TariffChanges{Summary: strp("обновлено")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("edit published: %v", err)
	}
	// to_delete: mark the PUBLISHED entity for deletion — it still has a live
	// row, so it must keep showing up (as to_delete, not vanish).
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeTariff, "published-only", nil); err != nil {
		t.Fatalf("delete published: %v", err)
	}

	index, err := kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTariff}, "both", "")
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
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeTariff, "new-only", nil); err != nil {
		t.Fatalf("delete new-only: %v", err)
	}
	index2, err := kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTariff}, "both", "")
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
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{Name: strp("Business"), Price: strp("v1"), PricingType: strp("fixed")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{Price: strp("v2")}, nil, kbstore.MCPProvenance{}); err != nil {
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
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeAssistant, kbstore.NaturalKeyMain, nil); err == nil {
		t.Fatal("expected the assistant singleton to be non-deletable")
	}
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeContacts, "not-main", nil); err == nil {
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
	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "how-to-order", kbstore.TopicChanges{
		Title: strp("Как заказать"), BodyMD: strp("Текст."), FeaturedImage: uuidpp(materialID),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("mcp upsert: %v", err)
	}

	// The legacy whole-row Playground path edits only text fields.
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "how-to-order", Title: "Как заказать (ред.)", BodyMD: "Текст."}); err != nil {
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

// TestReadRecords_CrossOrgIsolation is plan/mcp.md §9's "tenant isolation"
// validation item at the read path: kb_summary (IdentityIndex) and kb_read
// (ReadRecords) for one organization must never surface another
// organization's draft or live records, even though both share the same
// kbd_draft/ai_* tables keyed only by organization_id.
func TestReadRecords_CrossOrgIsolation(t *testing.T) {
	kb, orgA, st := newTestKB(t)
	ctx := context.Background()
	orgBRow, err := st.SeedOrganization(ctx, "org-b")
	if err != nil {
		t.Fatalf("seed org b: %v", err)
	}
	orgB := orgBRow.ID

	if _, err := kb.MCPUpsertProduct(ctx, orgA, uuid.Nil, "a-product", kbstore.ProductChanges{
		Name: strp("Товар A"), InStock: boolp(true),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed org a product: %v", err)
	}
	if _, err := kb.MCPUpsertProduct(ctx, orgB, uuid.Nil, "b-product", kbstore.ProductChanges{
		Name: strp("Товар B"), InStock: boolp(true),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed org b product: %v", err)
	}

	idxA, err := kb.IdentityIndex(ctx, orgA, nil, "both", "")
	if err != nil {
		t.Fatalf("identity index a: %v", err)
	}
	foundOwn := false
	for _, id := range idxA {
		if id.Key == "b-product" {
			t.Fatalf("org A's identity index leaked org B's record: %+v", id)
		}
		if id.Key == "a-product" {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Fatalf("org A's own product missing from its identity index: %+v", idxA)
	}

	pageA, err := kb.ReadRecords(ctx, orgA, []string{"product"}, "both", "", "", 0, "")
	if err != nil {
		t.Fatalf("read records a: %v", err)
	}
	for _, rec := range pageA.Items {
		if row := rec.Data.(kbstore.ProductRow); row.Ref == "b-product" {
			t.Fatalf("org A's kb_read leaked org B's record: %+v", row)
		}
	}

	pageB, err := kb.ReadRecords(ctx, orgB, []string{"product"}, "both", "", "", 0, "")
	if err != nil {
		t.Fatalf("read records b: %v", err)
	}
	for _, rec := range pageB.Items {
		if row := rec.Data.(kbstore.ProductRow); row.Ref == "a-product" {
			t.Fatalf("org B's kb_read leaked org A's record: %+v", row)
		}
	}

	// A key-scoped read for the OTHER org's key must come back empty, not
	// the other org's record — key matching alone must never cross orgID.
	pageCrossKey, err := kb.ReadRecords(ctx, orgA, []string{"product"}, "both", "b-product", "", 0, "")
	if err != nil {
		t.Fatalf("read records cross-key: %v", err)
	}
	if len(pageCrossKey.Items) != 0 {
		t.Fatalf("org A's kb_read(key=%q) should find nothing, got %+v", "b-product", pageCrossKey.Items)
	}
}

// TestReadRecords_PaginationCoversAllItemsExactlyOnce is plan/mcp.md §9's
// "pagination" validation item: following next_cursor to the end must
// return every record exactly once with no gaps or repeats.
func TestReadRecords_PaginationCoversAllItemsExactlyOnce(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	const n = 7
	for i := 0; i < n; i++ {
		ref := fmt.Sprintf("product-%02d", i)
		if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, ref, kbstore.ProductChanges{
			Name: strp(fmt.Sprintf("Товар %02d", i)), InStock: boolp(true),
		}, nil, kbstore.MCPProvenance{}); err != nil {
			t.Fatalf("seed %s: %v", ref, err)
		}
	}

	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		page, err := kb.ReadRecords(ctx, orgID, []string{"product"}, "draft", "", "", 3, cursor)
		if err != nil {
			t.Fatalf("read page: %v", err)
		}
		pages++
		if pages > n {
			t.Fatalf("pagination did not terminate after %d pages", pages)
		}
		for _, rec := range page.Items {
			row := rec.Data.(kbstore.ProductRow)
			if seen[row.Ref] {
				t.Fatalf("record %s returned more than once across pages", row.Ref)
			}
			seen[row.Ref] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != n {
		t.Fatalf("expected %d distinct records across all pages, got %d: %v", n, len(seen), seen)
	}
	if pages < 2 {
		t.Fatalf("expected the small page size (3) to force at least 2 pages for %d records, got %d", n, pages)
	}
}

// TestMCPUpsert_ConcurrentWritesSerializeWithoutLostUpdates is plan/mcp.md
// §9's "concurrent draft writes" validation item: writeDraftBlobVersioned
// takes a row-level FOR UPDATE lock (draft.go), so N real concurrent writers
// to the SAME draft row must all serialize through it and every write must
// land — none silently lost to a lost-update race.
func TestMCPUpsert_ConcurrentWritesSerializeWithoutLostUpdates(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "faq", kbstore.TopicChanges{
		Title: strp("FAQ"), BodyMD: strp("v0"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "faq", kbstore.TopicChanges{
				BodyMD: strp(fmt.Sprintf("v%d", i+1)),
			}, nil, kbstore.MCPProvenance{})
			errs[i] = err
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent writer %d failed: %v", i, err)
		}
	}

	final, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	// 1 (seed write) + n concurrent writes: every write serialized through
	// the row lock must be counted, or this comes up short.
	if final != int64(1+n) {
		t.Fatalf("expected base_version %d after %d serialized writes, got %d", 1+n, n, final)
	}

	afterAll, err := kb.ReadRecords(ctx, orgID, []string{"topic"}, "draft", "faq", "", 0, "")
	if err != nil {
		t.Fatalf("final read: %v", err)
	}
	if len(afterAll.Items) != 1 {
		t.Fatalf("expected exactly one draft topic to survive concurrent writes, got %+v", afterAll.Items)
	}
}

// TestApproveVersioned_ConcurrentUpsertNeverSilentlyLost is the regression
// guard for the atomic-approval fix (draft.go's ApproveVersioned doc
// comment). The PRE-FIX Approve read the draft blob unlocked, materialized
// into live in one transaction, then cleared the approved entries from the
// blob BY KEY in a completely separate, later transaction. An MCP update to
// the SAME entity landing in the gap between the first read and the second
// commit was published as its now-stale value and then silently deleted
// from the draft too — vanishing from both live and draft with no trace.
//
// This test runs many rounds of a whole-draft approve racing a concurrent
// MCPUpsertTopic patch to the SAME topic, under a bounded overall timeout
// (never a hang), and asserts each round's new body is found somewhere
// afterward — live or draft — never neither. Confirmed to fail against the
// pre-fix code (reverting ApproveVersioned to the old two-phase Approve
// reproduces a lost body within a handful of rounds); passes here because
// the fix makes read-validate-materialize-clear one indivisible operation
// under the kbd_draft row lock, closing the gap the bug lived in.
func TestApproveVersioned_ConcurrentUpsertNeverSilentlyLost(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "faq", kbstore.TopicChanges{
		Title: strp("FAQ"), BodyMD: strp("v0"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const rounds = 25
	for i := 0; i < rounds; i++ {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out after %d/%d rounds: %v", i, rounds, ctx.Err())
		default:
		}
		body := fmt.Sprintf("v%d", i+1)
		var wg sync.WaitGroup
		var approveErr, upsertErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			approveErr = kb.Approve(ctx, orgID, kbstore.ApproveSelector{})
		}()
		go func() {
			defer wg.Done()
			_, upsertErr = kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "faq", kbstore.TopicChanges{BodyMD: strp(body)}, nil, kbstore.MCPProvenance{})
		}()
		wg.Wait()
		if approveErr != nil {
			t.Fatalf("round %d: approve failed: %v", i, approveErr)
		}
		if upsertErr != nil {
			t.Fatalf("round %d: concurrent upsert failed: %v", i, upsertErr)
		}

		page, err := kb.ReadRecords(ctx, orgID, []string{"topic"}, "both", "faq", "", 0, "")
		if err != nil {
			t.Fatalf("round %d: read: %v", i, err)
		}
		found := false
		for _, item := range page.Items {
			if item.Data.(kbstore.TopicRow).BodyMD == body {
				found = true
			}
		}
		if !found {
			t.Fatalf("round %d: body %q vanished — present in neither live nor draft after concurrent approve+upsert (items: %+v)", i, body, page.Items)
		}
	}
}

// TestMCPUpsert_ConcurrentMediaWritesDoNotDeadlock is the regression guard
// for the OTHER pool-exhaustion deadlock class c8c0774 fixed for
// identityIndex but left in validateMediaRefs (mcp_media.go): every
// MCPUpsert* that touches a media field calls it from inside
// writeDraftBlobVersioned's closure, which already holds the kbd_draft row
// lock via its own connection. Before this fix, validateMediaRefs re-entered
// s.pool for a FRESH connection to look up each material — under N
// concurrent media-carrying writers to the SAME org, writer A holds the row
// lock (via its transaction's connection) and blocks waiting for a free
// connection for its OWN media lookup, while every other writer is itself
// blocked on A's row lock while holding a connection of its own: a
// pool-exhaustion deadlock with no timeout once N exceeds the pool size.
//
// Confirmed against the pre-fix code: reverting mcp_media.go's dbtx
// threading reproduces the exact predicted stack (every writer blocked in
// pgxpool.Acquire, called from lookupMaterialRef, called from
// writeDraftBlobVersioned's closure) and never completes. The writer
// goroutines run under a bounded, CANCELLABLE context (not
// context.Background()) specifically so that outcome is a clean, bounded
// test failure instead of a permanent goroutine/connection leak that
// depends on an external `go test -timeout` to ever notice — a plain
// `time.After` around an uncancelled Background()-rooted call would let the
// leaked goroutines keep holding pool connections forever after this test
// function returns, which can wedge unrelated later tests in the same
// binary too.
func TestMCPUpsert_ConcurrentMediaWritesDoNotDeadlock(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	setupCtx := context.Background()

	const n = 20 // comfortably above any default pgxpool pool size
	materialIDs := make([]uuid.UUID, n)
	for i := 0; i < n; i++ {
		id, err := kb.CreateUploadMaterial(setupCtx, orgID, kbstore.UploadMaterialInput{
			Filename: fmt.Sprintf("hero-%d.jpg", i), MimeType: "image/jpeg", SizeBytes: 5, CustomerVisibility: "visible",
		})
		if err != nil {
			t.Fatalf("create material %d: %v", i, err)
		}
		if err := kb.CompleteMaterialUpload(setupCtx, id, "disk", "org/x/"+id.String(), 5, ""); err != nil {
			t.Fatalf("complete upload %d: %v", i, err)
		}
		materialIDs[i] = id
	}

	writeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan []error, 1)
	go func() {
		var wg sync.WaitGroup
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				defer wg.Done()
				_, err := kb.MCPUpsertTopic(writeCtx, orgID, uuid.Nil, fmt.Sprintf("topic-%d", i), kbstore.TopicChanges{
					Title: strp(fmt.Sprintf("Topic %d", i)), BodyMD: strp("v0"),
					FeaturedImage: uuidpp(materialIDs[i]),
				}, nil, kbstore.MCPProvenance{})
				errs[i] = err
			}()
		}
		wg.Wait()
		done <- errs
	}()

	select {
	case errs := <-done:
		for i, err := range errs {
			if err != nil {
				t.Fatalf("writer %d failed: %v", i, err)
			}
		}
	case <-writeCtx.Done():
		t.Fatal("concurrent media-carrying upserts deadlocked (pool exhaustion): writeCtx's own 15s bound expired before all writers finished — see this test's doc comment for the exact mechanism")
	}
}

// TestMCPUpsert_ConcurrentFirstWritesToNewOrgNeverLoseAnEntry is the
// regression guard for writeDraftBlobVersioned's advisory-lock fix
// (draft.go): "SELECT ... FOR UPDATE" locks nothing when the kbd_draft row
// does not exist yet, so — before that fix — two concurrent FIRST writers
// to a brand-new org's draft both read "no row, version 0", both mutate an
// independently-empty in-memory blob, and whichever's INSERT ... ON
// CONFLICT ran second silently overwrote the first's already-committed
// content: a lost update with no error surfaced on either side, and a
// returned draft_version that disagreed with what ended up actually stored.
//
// newTestKB's org has never had a kbd_draft row, so the concurrent writers
// below are genuinely racing to create that row, not just to update an
// existing one (TestMCPUpsert_ConcurrentWritesSerializeWithoutLostUpdates,
// above, seeds one write first specifically to exercise the
// already-exists path instead).
func TestMCPUpsert_ConcurrentFirstWritesToNewOrgNeverLoseAnEntry(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()

	const n = 10
	var wg sync.WaitGroup
	results := make([]kbstore.UpsertResult, n)
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			res, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "", kbstore.TopicChanges{
				Title: strp(fmt.Sprintf("Topic %d", i)), BodyMD: strp("v0"),
			}, nil, kbstore.MCPProvenance{})
			results[i], errs[i] = res, err
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d failed: %v", i, err)
		}
	}

	// Every writer must be reported under a DISTINCT draft_version. If two
	// writers ever reported the SAME version, or a version came out
	// duplicated/skipped, that is direct evidence one writer's write
	// silently clobbered another's without either side detecting it.
	seen := map[int64]int{}
	for _, r := range results {
		seen[r.DraftVersion]++
	}
	for v, count := range seen {
		if count > 1 {
			t.Fatalf("draft_version %d was reported by %d different writers — a lost update (see this test's doc comment)", v, count)
		}
	}

	final, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if final != int64(n) {
		t.Fatalf("expected base_version %d after %d concurrent first-writers, got %d", n, n, final)
	}

	// And confirm no writer's CONTENT was actually discarded, not just that
	// the base_version arithmetic came out right.
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("Topic %d", i)
		page, err := kb.ReadRecords(ctx, orgID, []string{"topic"}, "draft", "", title, 0, "")
		if err != nil {
			t.Fatalf("read topic %d: %v", i, err)
		}
		if len(page.Items) == 0 {
			t.Fatalf("topic %q vanished — lost to a concurrent first-write race", title)
		}
	}
}

// TestMCPUpsert_SourceURLProvenanceCreatesDurableMaterial is Task 16's
// regression guard: provenance.source_url must land as a durable
// kbd_materials row (source_type='url'), tagged via
// extraction_metadata.mcp_target with the record it was cited for — never
// as a field on the draft/live row itself (plan/DECISIONS.md's "business
// columns only"). Confirms the material survives Approve (materialization
// must not delete it) and that it belongs to the authoring organization.
func TestMCPUpsert_SourceURLProvenanceCreatesDurableMaterial(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()

	res, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "coffee-machine", kbstore.ProductChanges{
		Name: strp("Кофемашина"), Price: strp("100000"), InStock: boolp(true),
	}, nil, kbstore.MCPProvenance{SourceURL: "https://shop.example/coffee-machine"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var count int
	var materialOrgID uuid.UUID
	var extractionMetadata []byte
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.kbd_materials
		WHERE source_type = 'url' AND source_ref = $1`, "https://shop.example/coffee-machine").Scan(&count); err != nil {
		t.Fatalf("count materials: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 kbd_materials row for the source_url, got %d", count)
	}
	if err := st.Pool().QueryRow(ctx, `SELECT organization_id, extraction_metadata FROM xchats.kbd_materials
		WHERE source_type = 'url' AND source_ref = $1`, "https://shop.example/coffee-machine").
		Scan(&materialOrgID, &extractionMetadata); err != nil {
		t.Fatalf("read material: %v", err)
	}
	if materialOrgID != orgID {
		t.Fatalf("material belongs to org %s, want %s", materialOrgID, orgID)
	}
	var meta struct {
		McpTarget struct {
			Type string `json:"type"`
			Key  string `json:"key"`
		} `json:"mcp_target"`
	}
	if err := json.Unmarshal(extractionMetadata, &meta); err != nil {
		t.Fatalf("unmarshal extraction_metadata: %v", err)
	}
	if meta.McpTarget.Type != "product" || meta.McpTarget.Key != res.Key {
		t.Fatalf("extraction_metadata.mcp_target = %+v, want {product %s}", meta.McpTarget, res.Key)
	}

	// Publishing must not touch kbd_materials at all — the association
	// survives Approve exactly like any other durable material record.
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.kbd_materials
		WHERE source_type = 'url' AND source_ref = $1`, "https://shop.example/coffee-machine").Scan(&count); err != nil {
		t.Fatalf("count materials after approve: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the material to survive Approve, got count=%d", count)
	}

	// The live row itself carries no provenance leakage — only canonical
	// business columns exist on ai_products at all (schema-enforced), so
	// this just confirms the row reads back cleanly with the right content.
	page, err := kb.ReadRecords(ctx, orgID, []string{"product"}, "live", res.Key, "", 0, "")
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly 1 live product, got %d", len(page.Items))
	}
	row := page.Items[0].Data.(kbstore.ProductRow)
	if row.Name != "Кофемашина" {
		t.Fatalf("live row wrong: %+v", row)
	}
}

// TestMCPUpsert_CrossOrgMaterialIDProvenanceRejected confirms
// provenance.material_ids gets the SAME same-organization discipline as an
// actual media attachment (validateMediaRef) — citing another organization's
// material as provenance is rejected, not silently ignored, since silently
// accepting it would let one organization tag/probe another's rows.
func TestMCPUpsert_CrossOrgMaterialIDProvenanceRejected(t *testing.T) {
	kb, orgA, st := newTestKB(t)
	ctx := context.Background()
	orgBRow, err := st.SeedOrganization(ctx, "org-b")
	if err != nil {
		t.Fatalf("seed org b: %v", err)
	}
	orgB := orgBRow.ID

	materialB, err := kb.CreateUploadMaterial(ctx, orgB, kbstore.UploadMaterialInput{
		Filename: "spec.pdf", MimeType: "application/pdf", SizeBytes: 10, CustomerVisibility: "auto",
	})
	if err != nil {
		t.Fatalf("create material under org B: %v", err)
	}

	_, err = kb.MCPUpsertProduct(ctx, orgA, uuid.Nil, "a-product", kbstore.ProductChanges{
		Name: strp("Товар A"), InStock: boolp(true),
	}, nil, kbstore.MCPProvenance{MaterialIDs: []uuid.UUID{materialB}})
	var mediaErr *kbstore.ErrMediaReference
	if !errors.As(err, &mediaErr) {
		t.Fatalf("expected ErrMediaReference for a cross-org material_id, got %v", err)
	}
}

// TestMCPUpsert_InvalidEnumValueRejected is Task 6's enum-validation
// regression guard: pricing_type/sales_status/zone_level are declared as a
// JSON-Schema enum (tools.go) but, before this fix, nothing checked that
// server-side — the enum was advisory only.
func TestMCPUpsert_InvalidEnumValueRejected(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()

	_, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{
		Name: strp("Business"), PricingType: strp("not-a-real-type"),
	}, nil, kbstore.MCPProvenance{})
	var enumErr *kbstore.ErrInvalidEnumValue
	if !errors.As(err, &enumErr) {
		t.Fatalf("expected ErrInvalidEnumValue for an invalid pricing_type, got %v", err)
	}
	if enumErr.Field != "pricing_type" {
		t.Fatalf("expected the error to name pricing_type, got %+v", enumErr)
	}

	_, err = kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "widget", kbstore.ProductChanges{
		Name: strp("Widget"), InStock: boolp(true), SalesStatus: strp("discontinued"),
	}, nil, kbstore.MCPProvenance{})
	if !errors.As(err, &enumErr) {
		t.Fatalf("expected ErrInvalidEnumValue for an invalid sales_status, got %v", err)
	}
}

// TestMCPUpsert_RenameOntoAnotherKeysTitleIsConflict is Task 6's
// rename-duplicate regression guard: resolveUpsertKey used to return
// immediately on an exact key match (the update path) without re-checking
// whether the NEW title collides with a DIFFERENT key — silently letting a
// rename merge two records' display identity.
func TestMCPUpsert_RenameOntoAnotherKeysTitleIsConflict(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()

	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{
		Name: strp("Business"), PricingType: strp("fixed"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed biz: %v", err)
	}
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "growth", kbstore.TariffChanges{
		Name: strp("Growth"), PricingType: strp("fixed"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed growth: %v", err)
	}

	// Renaming "growth" onto the EXACT normalized title "biz" already has —
	// this must be rejected exactly like a create would be, not silently
	// applied.
	_, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "growth", kbstore.TariffChanges{Name: strp("Business")}, nil, kbstore.MCPProvenance{})
	var conflict *kbstore.ErrDuplicateConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ErrDuplicateConflict for a rename colliding with another key's title, got %v", err)
	}
	if conflict.ExistingKey != "biz" {
		t.Fatalf("conflict should point at 'biz', got %q", conflict.ExistingKey)
	}

	// An unrelated rename (no collision) must still work.
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "growth", kbstore.TariffChanges{Name: strp("Growth Plus")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("expected a clean rename with no collision, got %v", err)
	}
}

// TestMCPDelete_TransitionMatrix is Task 7's regression guard for the
// kb_delete transition matrix (plan/mcp.md §5): a draft-only (never
// published) record's delete cancels the creation and stages NO delete
// marker (there is nothing live for one to ever apply to — a marker here
// would be a permanent phantom, showing "to_delete" in kb_summary forever
// with nothing for Approve to actually remove); a LIVE record's delete
// stages a real marker; repeating the delete on an already-marked live
// record is an idempotent no-op; and a nonexistent key is rejected outright
// instead of silently accepted.
func TestMCPDelete_TransitionMatrix(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()

	// --- nonexistent key: rejected, not silently accepted -----------------
	_, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeTopic, "never-existed", nil)
	var cannotDelete *kbstore.ErrCannotDelete
	if !errors.As(err, &cannotDelete) {
		t.Fatalf("expected ErrCannotDelete for a nonexistent key, got %v", err)
	}

	// --- draft-only: delete cancels the creation, no phantom marker -------
	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "draft-only", kbstore.TopicChanges{
		Title: strp("Draft Only"), BodyMD: strp("v0"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed draft-only: %v", err)
	}
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeTopic, "draft-only", nil); err != nil {
		t.Fatalf("delete draft-only: %v", err)
	}
	index, err := kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTopic}, "both", "")
	if err != nil {
		t.Fatalf("identity index: %v", err)
	}
	for _, id := range index {
		if id.Key == "draft-only" {
			t.Fatalf("draft-only topic should vanish entirely (no phantom to_delete marker), got %+v", id)
		}
	}

	// --- live: delete stages a real marker, survives to Approve -----------
	if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "live-topic", kbstore.TopicChanges{
		Title: strp("Live Topic"), BodyMD: strp("v0"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed live-topic: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve live-topic: %v", err)
	}
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeTopic, "live-topic", nil); err != nil {
		t.Fatalf("delete live-topic: %v", err)
	}
	index, err = kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTopic}, "both", "")
	if err != nil {
		t.Fatalf("identity index: %v", err)
	}
	var liveTopicState string
	for _, id := range index {
		if id.Key == "live-topic" {
			liveTopicState = id.State()
		}
	}
	if liveTopicState != "to_delete" {
		t.Fatalf("expected live-topic to be marked to_delete, got state=%q", liveTopicState)
	}

	// --- repeat delete on the same (already-marked) live entity: idempotent
	// (the write itself still happens — base_version bumps — but the
	// MARKER must not duplicate; addDelete's own de-dup is what this proves
	// still holds through MCPDelete's rewritten existence check).
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeTopic, "live-topic", nil); err != nil {
		t.Fatalf("repeat delete: %v", err)
	}
	index, err = kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTopic}, "both", "")
	if err != nil {
		t.Fatalf("identity index: %v", err)
	}
	toDeleteCount := 0
	for _, id := range index {
		if id.Key == "live-topic" && id.State() == "to_delete" {
			toDeleteCount++
		}
	}
	if toDeleteCount != 1 {
		t.Fatalf("expected exactly one to_delete entry for live-topic after a repeat delete, got %d", toDeleteCount)
	}

	// --- Approve actually removes the live row -----------------------------
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve delete: %v", err)
	}
	page, err := kb.ReadRecords(ctx, orgID, []string{"topic"}, "live", "live-topic", "", 0, "")
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("expected live-topic to actually be gone from live after approve, got %+v", page.Items)
	}
}

// TestMCPUpsert_CancelsPendingDeleteMarker is Task 7's regression guard for
// the OTHER half of the transition matrix: re-upserting a key that is
// CURRENTLY staged for deletion means the caller is choosing to keep/update
// it, not delete it. Before this fix, the patch and the delete marker
// coexisted in the draft, and a whole-draft Approve materialized the patch
// and then IMMEDIATELY deleted it again (deletes apply after upserts in
// Approve's materialization order) — the update was silently discarded.
func TestMCPUpsert_CancelsPendingDeleteMarker(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()

	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{
		Name: strp("Business"), PricingType: strp("fixed"),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve seed: %v", err)
	}
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeTariff, "biz", nil); err != nil {
		t.Fatalf("stage delete: %v", err)
	}

	// The model changes its mind and updates the SAME key instead.
	if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "biz", kbstore.TariffChanges{Summary: strp("kept after all")}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("upsert over a pending delete: %v", err)
	}

	index, err := kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeTariff}, "both", "")
	if err != nil {
		t.Fatalf("identity index: %v", err)
	}
	for _, id := range index {
		if id.Key == "biz" && id.State() == "to_delete" {
			t.Fatalf("expected the pending delete to be cancelled by the upsert, got %+v", id)
		}
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	page, err := kb.ReadRecords(ctx, orgID, []string{"tariff"}, "live", "biz", "", 0, "")
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 'biz' to SURVIVE approve (the delete was cancelled), got %+v", page.Items)
	}
	row := page.Items[0].Data.(kbstore.TariffRow)
	if row.Summary != "kept after all" {
		t.Fatalf("expected the update to have applied, got %+v", row)
	}
}

// TestOperatorAttribution_UpdatedByAndActorUserID is Task 8's regression
// guard: kbd_draft.updated_by (DECISIONS.md: "Last operator who changed the
// draft") and ai_audit_log.actor_user_id both existed in the schema but
// nothing ever wrote them — every draft write (MCP or the legacy Playground
// editor) left updated_by NULL forever, and Approve's own audit row never
// named who approved. Confirms both are now populated end to end: an
// MCP-authenticated write's verified Principal.UserID lands in updated_by,
// and the human who approves lands in the audit row.
func TestOperatorAttribution_UpdatedByAndActorUserID(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()

	writer, err := st.SeedUser(ctx, orgID, "writer@example.com", "hash", "Writer")
	if err != nil {
		t.Fatalf("seed writer: %v", err)
	}
	approver, err := st.SeedUser(ctx, orgID, "approver@example.com", "hash", "Approver")
	if err != nil {
		t.Fatalf("seed approver: %v", err)
	}

	if _, err := kb.MCPUpsertTopic(ctx, orgID, writer.ID, "how-to-order", kbstore.TopicChanges{
		Title: strp("Как заказать"), BodyMD: strp("Текст."),
	}, nil, kbstore.MCPProvenance{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var updatedBy uuid.UUID
	if err := st.Pool().QueryRow(ctx, `SELECT updated_by FROM xchats.kbd_draft WHERE organization_id = $1`, orgID).Scan(&updatedBy); err != nil {
		t.Fatalf("read kbd_draft.updated_by: %v", err)
	}
	if updatedBy != writer.ID {
		t.Fatalf("kbd_draft.updated_by = %s, want the writer %s", updatedBy, writer.ID)
	}

	if err := kb.ApproveVersioned(ctx, orgID, kbstore.ApproveSelector{}, nil, approver.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var actorUserID uuid.UUID
	var note string
	if err := st.Pool().QueryRow(ctx, `SELECT actor_user_id, note FROM xchats.ai_audit_log
		WHERE organization_id = $1 AND action = 'approve' ORDER BY created_at DESC LIMIT 1`, orgID).
		Scan(&actorUserID, &note); err != nil {
		t.Fatalf("read ai_audit_log: %v", err)
	}
	if actorUserID != approver.ID {
		t.Fatalf("ai_audit_log.actor_user_id = %s, want the approver %s", actorUserID, approver.ID)
	}

	// And the post-approve draft-clearing write must NOT clobber updated_by
	// back to NULL/some earlier value in a way that loses the last writer's
	// identity for whatever remains pending — reuse the approver's own id
	// here since ApproveVersioned's internal cleanup write is attributed to
	// whoever approved.
	if err := st.Pool().QueryRow(ctx, `SELECT updated_by FROM xchats.kbd_draft WHERE organization_id = $1`, orgID).Scan(&updatedBy); err != nil {
		t.Fatalf("read kbd_draft.updated_by after approve: %v", err)
	}
	if updatedBy != approver.ID {
		t.Fatalf("kbd_draft.updated_by after approve = %s, want the approver %s", updatedBy, approver.ID)
	}
}
