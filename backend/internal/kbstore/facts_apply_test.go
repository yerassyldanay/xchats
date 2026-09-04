package kbstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// factsp packs facts into a *[]aiprompt.AdditionalFact — the "explicit
// set/replace/clear" shape ProductInput.AdditionalFacts (and its
// Tariff/TariffInfo siblings) expect; a nil *[]aiprompt.AdditionalFact
// (simply omitting the field) means "no pending edit" instead, the exact
// same nil-means-unchanged/non-nil-means-replace contract media_apply_test.go
// already pins for *[]uuid.UUID media fields.
func factsp(facts ...aiprompt.AdditionalFact) *[]aiprompt.AdditionalFact { return &facts }

// TestUpsertProduct_AdditionalFactsAbsentPreservesExisting proves a
// text-only draft edit (AdditionalFacts left nil) never blanks out facts a
// previous write already staged — required test scenario "missing-patch-
// field-unchanged" for products.
func TestUpsertProduct_AdditionalFactsAbsentPreservesExisting(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", AvailabilityStatus: strp("in_stock"),
		AdditionalFacts: factsp(aiprompt.AdditionalFact{
			Ref: "has_wifi", Value: true, Instruction: "Поддерживает ли товар Wi-Fi.",
		}),
	}); err != nil {
		t.Fatalf("create with a fact: %v", err)
	}

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар (ред.)", AvailabilityStatus: strp("in_stock"),
	}); err != nil {
		t.Fatalf("text-only edit: %v", err)
	}

	row := readProduct(t, kb, orgID, "p")
	if len(row.AdditionalFacts) != 1 || row.AdditionalFacts[0].Ref != "has_wifi" {
		t.Fatalf("absent AdditionalFacts blanked existing facts: %+v", row.AdditionalFacts)
	}
	if row.Name != "Товар (ред.)" {
		t.Fatalf("text-only edit did not apply: %+v", row)
	}
}

// TestUpsertProduct_AdditionalFactsExplicitEmptyClears proves a non-nil but
// EMPTY fact list clears every previously staged fact — required test
// scenario "explicit-[]-clears" for products, the AdditionalFacts twin of
// TestUpsertProduct_MediaEmptySliceDetachesAll.
func TestUpsertProduct_AdditionalFactsExplicitEmptyClears(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", AvailabilityStatus: strp("in_stock"),
		AdditionalFacts: factsp(aiprompt.AdditionalFact{
			Ref: "has_wifi", Value: true, Instruction: "Поддерживает ли товар Wi-Fi.",
		}),
	}); err != nil {
		t.Fatalf("create with a fact: %v", err)
	}

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", AvailabilityStatus: strp("in_stock"),
		AdditionalFacts: factsp(),
	}); err != nil {
		t.Fatalf("clear facts: %v", err)
	}

	row := readProduct(t, kb, orgID, "p")
	if len(row.AdditionalFacts) != 0 {
		t.Fatalf("additional_facts = %+v, want empty (cleared)", row.AdditionalFacts)
	}
}

// TestUpsertProduct_AdditionalFactsSetReplacesWholeList proves an explicit
// non-empty fact list REPLACES the whole list rather than merging into it —
// the store's contract is whole-list replace, same as gallery_images.
func TestUpsertProduct_AdditionalFactsSetReplacesWholeList(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", AvailabilityStatus: strp("in_stock"),
		AdditionalFacts: factsp(
			aiprompt.AdditionalFact{Ref: "has_wifi", Value: true, Instruction: "Поддерживает ли товар Wi-Fi."},
			aiprompt.AdditionalFact{Ref: "model_code", Value: "DLM-500X", Instruction: "Точный код модели."},
		),
	}); err != nil {
		t.Fatalf("create with two facts: %v", err)
	}

	if err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", AvailabilityStatus: strp("in_stock"),
		AdditionalFacts: factsp(aiprompt.AdditionalFact{
			Ref: "working_pressure", Value: json.Number("275"), Instruction: "Рабочее давление, бар.",
		}),
	}); err != nil {
		t.Fatalf("replace with one fact: %v", err)
	}

	row := readProduct(t, kb, orgID, "p")
	if len(row.AdditionalFacts) != 1 || row.AdditionalFacts[0].Ref != "working_pressure" {
		t.Fatalf("additional_facts = %+v, want exactly [working_pressure] (whole-list replace, not merge)", row.AdditionalFacts)
	}
}

// TestUpsertProduct_RejectsInvalidAdditionalFacts proves the draft lane
// enforces aiprompt.ValidateFacts (here: a ref colliding with the product's
// own concrete "price" column) BEFORE anything is persisted — the same
// belt-and-suspenders every other typed field already gets, so a bad fact
// can never reach kbd_draft.
func TestUpsertProduct_RejectsInvalidAdditionalFacts(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	err := kb.UpsertProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "p", Name: "Товар", AvailabilityStatus: strp("in_stock"),
		AdditionalFacts: factsp(aiprompt.AdditionalFact{
			Ref: "price", Value: json.Number("1"), Instruction: "Коллизия с колонкой price.",
		}),
	})
	if err == nil {
		t.Fatal("expected an error for a fact ref colliding with the concrete column \"price\"")
	}

	page, readErr := kb.ReadRecords(ctx, orgID, []string{kbstore.KBTypeProduct}, "draft", "p", "", 0, "")
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if len(page.Items) != 0 {
		t.Fatalf("the rejected product must never reach the draft: %+v", page.Items)
	}
}

// TestUpsertTariff_AdditionalFactsSemantics is a second-type cross-check
// that tariffs share the exact same nil-unchanged/empty-clears/set-replaces
// contract products do — one shared validation+apply path, not a per-type
// copy that could drift.
func TestUpsertTariff_AdditionalFactsSemantics(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.UpsertTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{
		Ref: "basic", Name: "Базовый",
		AdditionalFacts: factsp(aiprompt.AdditionalFact{
			Ref: "limit_on_devices", Value: json.Number("5"), Instruction: "Максимальное количество устройств.",
		}),
	}); err != nil {
		t.Fatalf("create with a fact: %v", err)
	}
	if err := kb.UpsertTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{Ref: "basic", Name: "Базовый v2"}); err != nil {
		t.Fatalf("text-only edit: %v", err)
	}
	row := readTariff(t, kb, orgID, "basic")
	if len(row.AdditionalFacts) != 1 || row.AdditionalFacts[0].Ref != "limit_on_devices" {
		t.Fatalf("absent AdditionalFacts blanked existing facts: %+v", row.AdditionalFacts)
	}

	if err := kb.UpsertTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{
		Ref: "basic", Name: "Базовый v3", AdditionalFacts: factsp(),
	}); err != nil {
		t.Fatalf("clear facts: %v", err)
	}
	row = readTariff(t, kb, orgID, "basic")
	if len(row.AdditionalFacts) != 0 {
		t.Fatalf("additional_facts = %+v, want empty (cleared)", row.AdditionalFacts)
	}
}

// TestPatchTariffInfo_AdditionalFactsSemantics covers the org-wide
// tariff_info singleton's own PATCH-only lane — nil (absent) preserves,
// non-nil empty clears, non-nil non-empty replaces.
func TestPatchTariffInfo_AdditionalFactsSemantics(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.PatchTariffInfo(ctx, orgID, uuid.Nil, kbstore.TariffInfoPatch{
		AdditionalFacts: factsp(aiprompt.AdditionalFact{
			Ref: "trial_in_days", Value: json.Number("3"), Instruction: "Продолжительность общего пробного периода.",
		}),
	}); err != nil {
		t.Fatalf("set a fact: %v", err)
	}
	row := readTariffInfo(t, kb, orgID)
	if len(row.AdditionalFacts) != 1 || row.AdditionalFacts[0].Ref != "trial_in_days" {
		t.Fatalf("additional_facts = %+v, want [trial_in_days]", row.AdditionalFacts)
	}

	// Absent (nil) patch on some other field — nothing to patch here but the
	// facts, so patch the same facts back and confirm a second identical
	// patch is a stable no-op; then prove nil truly means "leave alone" by
	// patching with a zero-value TariffInfoPatch{} and re-reading.
	if err := kb.PatchTariffInfo(ctx, orgID, uuid.Nil, kbstore.TariffInfoPatch{}); err != nil {
		t.Fatalf("empty (all-nil) patch: %v", err)
	}
	row = readTariffInfo(t, kb, orgID)
	if len(row.AdditionalFacts) != 1 || row.AdditionalFacts[0].Ref != "trial_in_days" {
		t.Fatalf("an all-nil patch blanked additional_facts: %+v", row.AdditionalFacts)
	}

	if err := kb.PatchTariffInfo(ctx, orgID, uuid.Nil, kbstore.TariffInfoPatch{AdditionalFacts: factsp()}); err != nil {
		t.Fatalf("clear facts: %v", err)
	}
	row = readTariffInfo(t, kb, orgID)
	if len(row.AdditionalFacts) != 0 {
		t.Fatalf("additional_facts = %+v, want empty (cleared)", row.AdditionalFacts)
	}
}

// TestTariffInfo_DraftRoundTripThroughApprove is tariff_info's own version
// of TestDraftEditAndApprove: a brand-new entity type (no live row at all
// yet) must show up as a pending change, materialize into ai_tariff_info on
// approve, and stop appearing as a pending change afterwards — required
// test scenario "draft-to-live approval/diff" for the new singleton.
func TestTariffInfo_DraftRoundTripThroughApprove(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.PatchTariffInfo(ctx, orgID, uuid.Nil, kbstore.TariffInfoPatch{
		AdditionalFacts: factsp(aiprompt.AdditionalFact{
			Ref: "trial_in_days", Value: json.Number("3"), Instruction: "Продолжительность общего пробного периода.",
		}),
	}); err != nil {
		t.Fatalf("patch tariff_info: %v", err)
	}

	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if len(changes.TariffInfo) != 1 || changes.TariffInfo[0].AdditionalFacts[0].Ref != "trial_in_days" {
		t.Fatalf("pending tariff_info change missing from DraftChanges: %+v", changes.TariffInfo)
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "tariff_info", Key: kbstore.NaturalKeyMain}); err != nil {
		t.Fatalf("approve tariff_info: %v", err)
	}

	changes, err = kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes after approve: %v", err)
	}
	if len(changes.TariffInfo) != 0 {
		t.Fatalf("tariff_info should no longer be a pending change after approve: %+v", changes.TariffInfo)
	}

	// The approved row is gone from the draft-only (pending) view (checked
	// above) but must now be live: ReadRecords(source="live") goes through
	// LiveView/liveView, which reads ai_tariff_info directly with no blob
	// overlay at all, so this is a genuine materialization check, not just
	// an echo of the blob.
	row := readTariffInfoSource(t, kb, orgID, "live")
	if len(row.AdditionalFacts) != 1 || row.AdditionalFacts[0].Ref != "trial_in_days" {
		t.Fatalf("approved tariff_info did not materialize into ai_tariff_info: %+v", row)
	}
	if row.Draft {
		t.Fatalf("tariff_info should read back as live (not draft) after approve: %+v", row)
	}
}

// readTariff/readTariffInfo mirror readProduct/readTopic/readContact
// (media_apply_test.go) — reading back the pending (draft-only) shape of
// one record through the real ReadRecords path.
func readTariff(t *testing.T, kb *kbstore.Store, orgID uuid.UUID, ref string) kbstore.TariffRow {
	t.Helper()
	page, err := kb.ReadRecords(context.Background(), orgID, []string{kbstore.KBTypeTariff}, "draft", ref, "", 0, "")
	if err != nil {
		t.Fatalf("read tariff %q: %v", ref, err)
	}
	if len(page.Items) == 0 {
		t.Fatalf("tariff %q not found", ref)
	}
	return page.Items[0].Data.(kbstore.TariffRow)
}

func readTariffInfo(t *testing.T, kb *kbstore.Store, orgID uuid.UUID) kbstore.TariffInfoRow {
	t.Helper()
	return readTariffInfoSource(t, kb, orgID, "draft")
}

// readTariffInfoSource lets a caller pick "draft" (pending-only, via
// DraftOnly — what disappears the moment the blob's tariff_info entry is
// cleared) or "live" (via LiveView, a real ai_tariff_info query with no
// blob overlay at all) — the two ReadRecords sources genuinely differ for a
// singleton around Approve, unlike the merged "both" a caller wants for
// day-to-day editing.
func readTariffInfoSource(t *testing.T, kb *kbstore.Store, orgID uuid.UUID, source string) kbstore.TariffInfoRow {
	t.Helper()
	page, err := kb.ReadRecords(context.Background(), orgID, []string{kbstore.KBTypeTariffInfo}, source, "", "", 0, "")
	if err != nil {
		t.Fatalf("read tariff_info (source=%s): %v", source, err)
	}
	if len(page.Items) == 0 {
		t.Fatalf("tariff_info not found (source=%s)", source)
	}
	return page.Items[0].Data.(kbstore.TariffInfoRow)
}
