package kbstore_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// newTestKB opens a fresh, migrated SQLite database and seeds an org.
func newTestKB(t *testing.T) (*kbstore.Store, uuid.UUID, *store.Store, *dbx.DB) {
	t.Helper()
	kb, st, db := dbtest.NewKB(t)
	org, err := st.SeedOrganization(context.Background(), "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return kb, org.ID, st, db
}

// The brain's snapshot must round-trip through the DB unchanged: seed → load →
// topic bodies are byte-identical (pure prose) and every fact in the [F] catalog
// resolves to the SAME verbatim value from the DB snapshot as from the literal —
// INCLUDING unit-bearing amounts the old typed PriceBook/formatTenge path dropped.
func TestRoundTrip_RendersIdentically(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	seed := brain.SeedSnapshot()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	loaded, err := kb.LoadLive(ctx, orgID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Config.Persona != seed.Config.Persona {
		t.Fatalf("persona lost: %q", loaded.Config.Persona)
	}
	if len(loaded.Topics) != len(seed.Topics) {
		t.Fatalf("topics: want %d got %d", len(seed.Topics), len(loaded.Topics))
	}
	// Topic bodies are pure prose — they survive the round-trip byte-for-byte.
	seedBodies := map[string]string{}
	for _, topic := range seed.Topics {
		seedBodies[topic.Slug] = topic.BodyMD
	}
	for _, topic := range loaded.Topics {
		if want := seedBodies[topic.Slug]; want != topic.BodyMD {
			t.Fatalf("topic %q body mismatch:\n seed: %q\n db:   %q", topic.Slug, want, topic.BodyMD)
		}
	}
	// Every advertised fact resolves to the same value from either snapshot —
	// EXCEPT product.*.availability: that column is retired (plan/database-
	// schema.md: not part of the target canonical schema) and kbstore no
	// longer writes it, so it never round-trips through the DB even though
	// the in-memory seed literal (brain/seed.go) still sets flavor text for
	// it. This is expected, not a regression.
	facts := seed.Facts.List()
	if len(facts) == 0 {
		t.Fatal("seed advertises no facts to round-trip")
	}
	for _, f := range facts {
		if strings.HasSuffix(f.Token, ".availability}}") {
			continue
		}
		want, werr := seed.Facts.Render(f.Token, "ru")
		got, gerr := loaded.Facts.Render(f.Token, "ru")
		if werr != nil || gerr != nil {
			t.Fatalf("render %q: seed=%v db=%v", f.Token, werr, gerr)
		}
		if want != got {
			t.Fatalf("fact %q render mismatch:\n seed: %q\n db:   %q", f.Token, want, got)
		}
	}
}

// The lossy-bridge fix: a fact carrying a unit ("/мес") survives the DB round-trip
// verbatim — the regression the old int-tenge PriceBook silently dropped. The
// amount now lives in a typed ai_tariffs.price column, quoted by its 3-part token.
func TestRoundTrip_UnitBearingValue(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.UpsertTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{
		Ref: "growth", Name: "Рост", Price: "25 000 ₸/мес",
	}); err != nil {
		t.Fatalf("upsert tariff: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	loaded, err := kb.LoadLive(ctx, orgID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, err := loaded.Facts.Render("Тариф Рост — {{tariff.growth.price}}.", "ru")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if want := "Тариф Рост — 25 000 ₸/мес."; got != want {
		t.Fatalf("unit dropped:\n want %q\n got  %q", want, got)
	}
}

// Editing the draft blob overlays live rows with pending entries (flagged
// Draft: true); approving materializes them and the blob is emptied.
func TestDraftEditAndApprove(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	view, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if len(view.Topics) == 0 {
		t.Fatal("draft should show the seeded live topics")
	}
	for _, tp := range view.Topics {
		if tp.Draft {
			t.Fatalf("freshly-seeded topic %q should not be flagged draft", tp.Slug)
		}
	}

	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "new_topic", Title: "Новая тема", BodyMD: "Просто текст.",
	}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	view, err = kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	var found bool
	for _, tp := range view.Topics {
		if tp.Slug == "new_topic" {
			found = true
			if !tp.Draft {
				t.Fatalf("new_topic should be flagged draft (pending)")
			}
		}
	}
	if !found {
		t.Fatal("new_topic missing from the merged draft view")
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "topics", Key: "new_topic"}); err != nil {
		t.Fatalf("approve entity: %v", err)
	}
	view, err = kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	for _, tp := range view.Topics {
		if tp.Slug == "new_topic" && tp.Draft {
			t.Fatalf("new_topic should be live (not draft) after approve")
		}
	}
	live, err := kb.LoadLive(ctx, orgID)
	if err != nil {
		t.Fatalf("load live: %v", err)
	}
	liveHasIt := false
	for _, tp := range live.Topics {
		if tp.Slug == "new_topic" {
			liveHasIt = true
		}
	}
	if !liveHasIt {
		t.Fatal("approved topic did not materialize into the live table")
	}
}

// TestUpsertZone_DraftRoundTripThroughApprove is Task 13's kbstore-level
// regression guard: the delivery-zone draft path (UpsertZone/DeleteZone) must
// behave exactly like every other typed fact — pending until approved, live
// (LiveView) only staged-not-yet-approved for a deletion, gone from live only
// once the deletion itself is approved.
func TestUpsertZone_DraftRoundTripThroughApprove(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// zoneGateReasons requires blank flat delivery fields + a non-blank
	// outside_zones_note on every policy row the moment any zone exists —
	// the seed starts with the opposite, so fix that up first.
	if err := kb.PatchPolicies(ctx, orgID, uuid.Nil, kbstore.PolicyPatch{
		DeliveryCost: strp(""), DeliveryInDays: strp(""), OutsideZonesNote: strp("Только в зонах доставки."),
	}); err != nil {
		t.Fatalf("patch policies: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve policy edit: %v", err)
	}

	if err := kb.UpsertZone(ctx, orgID, uuid.Nil, kbstore.DeliveryZoneInput{
		Ref: "almaty", Name: "Алматы", ZoneLevel: "city",
		DeliveryAvailable: true, DeliveryCost: "1500", DeliveryInDays: "1-2",
	}); err != nil {
		t.Fatalf("upsert zone: %v", err)
	}
	view, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	z := findZoneRow(view.Zones, "almaty")
	if z == nil || !z.Draft {
		t.Fatalf("new zone should be pending in the merged draft view, got %+v", z)
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "delivery_zones", Key: "almaty"}); err != nil {
		t.Fatalf("approve zone: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if z := findZoneRow(live.Zones, "almaty"); z == nil {
		t.Fatal("approved zone did not materialize into the live table")
	}

	if err := kb.DeleteZone(ctx, orgID, uuid.Nil, "almaty"); err != nil {
		t.Fatalf("delete zone: %v", err)
	}
	view, err = kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if findZoneRow(view.Zones, "almaty") != nil {
		t.Fatal("a zone staged for deletion should be suppressed from the merged draft view")
	}
	live, err = kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if findZoneRow(live.Zones, "almaty") == nil {
		t.Fatal("the zone must still be live — only staged for deletion, not yet approved")
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "delivery_zones", Key: "almaty"}); err != nil {
		t.Fatalf("approve deletion: %v", err)
	}
	live, err = kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if findZoneRow(live.Zones, "almaty") != nil {
		t.Fatal("zone should be gone from the live view after the deletion is approved")
	}
}

func findZoneRow(zones []kbstore.ZoneRow, ref string) *kbstore.ZoneRow {
	for i := range zones {
		if zones[i].Ref == ref {
			return &zones[i]
		}
	}
	return nil
}

// TestApprove_ConfigKindMaterializesJustConfig is Task 14a's regression
// guard: assistant config previously had no natural key, so it only ever
// materialized on a whole-draft approve (sel.Kind == "") — an entity-scoped
// approve of kind "config" (keyed by NaturalKeyMain, the same singleton
// convention contacts/policies use) did nothing at all. This drives it end to
// end: stage a config edit, approve it individually, confirm it's live and no
// longer pending — then confirm a second entity-scoped approve is a
// no-op (nothing pending), never an error.
func TestApprove_ConfigKindMaterializesJustConfig(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: strp("Дружелюбный помощник")}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	view, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if !view.Config.Draft || view.Config.Persona != "Дружелюбный помощник" {
		t.Fatalf("expected a pending config edit, got %+v", view.Config)
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "config", Key: kbstore.NaturalKeyMain}); err != nil {
		t.Fatalf("approve config entity: %v", err)
	}
	view, err = kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if view.Config.Draft {
		t.Fatal("config should no longer be flagged draft (pending) after its entity approve")
	}
	if view.Config.Persona != "Дружелюбный помощник" {
		t.Fatalf("approved persona did not materialize, got %q", view.Config.Persona)
	}

	// Nothing pending now — a second entity-scoped approve must be a silent
	// no-op (errApproveNothingPending), not an error.
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "config", Key: kbstore.NaturalKeyMain}); err != nil {
		t.Fatalf("approve with nothing pending should be a no-op, got: %v", err)
	}
}

// Every DraftView collection must serialize as a JSON array ([]), never null —
// even when a table + the blob are empty. The client reads d.<coll>.length
// directly, so a nil slice (→ null) would crash the /playground /knowledge-base
// pages. (Regression: the typed-facts rewrite left empty slices nil.)
func TestDraft_CollectionsNeverNull(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	// No seed → every table AND the blob is empty; the worst case for nil slices.
	view, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if view.Topics == nil || view.Tariffs == nil ||
		view.Products == nil || view.Contacts == nil || view.Policies == nil ||
		view.Materials == nil || view.Requests == nil {
		t.Fatalf("a collection is nil: topics=%v tariffs=%v products=%v contacts=%v policies=%v materials=%v requests=%v",
			view.Topics == nil, view.Tariffs == nil, view.Products == nil,
			view.Contacts == nil, view.Policies == nil, view.Materials == nil, view.Requests == nil)
	}
	blob, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, k := range []string{"topics", "tariffs", "products", "contacts", "policies", "materials", "requests"} {
		if strings.Contains(string(blob), `"`+k+`":null`) {
			t.Fatalf("collection %q serialized as null (must be []): %s", k, blob)
		}
	}
}

// The deterministic gate blocks approve on every non-prose topic body it
// finds, listing every reason — not just the first. Two topics, each
// violating a different rule (a fact token; a literal currency amount),
// exercises the gate aggregating across multiple rows via the full DB path
// (Approve → LoadLive → mergeForGate → gate) — the pure-unit
// gate_internal_test.go does not go through the DB.
func TestApproveGateBlocks(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "broken_token", BodyMD: "Цена {{tariff.unknown.price}}",
	}); err != nil {
		t.Fatalf("topic: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "broken_amount", BodyMD: "Доставка стоит 1 500 ₸ по городу.",
	}); err != nil {
		t.Fatalf("topic: %v", err)
	}
	err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{})
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError, got %T: %v", err, err)
	}
	msgs := make([]string, len(ge.Reasons))
	for i, r := range ge.Reasons {
		msgs[i] = r.Message
		if r.Kind != "topics" {
			t.Errorf("reason %q should name kind=topics, got %q", r.Message, r.Kind)
		}
	}
	joined := strings.Join(msgs, "; ")
	if !strings.Contains(joined, "pure prose") || !strings.Contains(joined, "literal amount") {
		t.Fatalf("want both a token reason and a literal-amount reason, got %v", ge.Reasons)
	}
}

// LiveView must NEVER see pending Playground TEXT/FACT work — it ignores the
// draft blob entirely, so a live edit (/knowledge-base) can never mix with a
// draft edit (/playground). Every row it returns is Draft:false. Materials
// are the one exception to "playground work stays out of LiveView": a
// kbd_materials row has no draft/live split at all (it's either uploaded or
// it isn't), so LiveView DOES carry them — see liveView's own doc comment.
// Requests remains empty; a KbRequest is a Playground review-queue concept
// with no live-page meaning.
func TestLiveView_IgnoresDraftBlob(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Stage a PENDING topic in the draft blob — never approved.
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "pending_only", Title: "Черновик", BodyMD: "Только в черновике.",
	}); err != nil {
		t.Fatalf("upsert draft topic: %v", err)
	}
	materialID, err := kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: "hero.jpg", MimeType: "image/jpeg", SizeBytes: 5, CustomerVisibility: "visible",
	})
	if err != nil {
		t.Fatalf("create upload material: %v", err)
	}
	if err := kb.CompleteMaterialUpload(ctx, materialID, "disk", "org/x/"+materialID.String(), 5, ""); err != nil {
		t.Fatalf("complete upload: %v", err)
	}

	draft, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	foundInDraft := false
	for _, tp := range draft.Topics {
		if tp.Slug == "pending_only" {
			foundInDraft = true
		}
	}
	if !foundInDraft {
		t.Fatal("sanity: the pending topic should be visible in Draft()")
	}

	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	for _, tp := range live.Topics {
		if tp.Slug == "pending_only" {
			t.Fatalf("LiveView must not see a pending draft-only topic: %+v", tp)
		}
		if tp.Draft {
			t.Fatalf("every LiveView row must be Draft:false, got %+v", tp)
		}
	}
	if len(live.Requests) != 0 {
		t.Fatalf("LiveView must not carry requests (playground-only), got requests=%d", len(live.Requests))
	}
	foundMaterial := false
	for _, m := range live.Materials {
		if m.ID == materialID {
			foundMaterial = true
		}
	}
	if !foundMaterial {
		t.Fatalf("LiveView must carry materials (no draft/live split) — material %s missing from %+v", materialID, live.Materials)
	}
}

// PutLiveTopic is held to the same content bar as the approve gate: a body
// carrying a fact token is rejected immediately (no silent bad write).
func TestPutLiveTopic_RejectsNonProseBody(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	err := kb.PutLiveTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "broken", BodyMD: "Цена {{tariff.unknown.price}}",
	})
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError for a token in the body, got %T: %v", err, err)
	}
}

// A live write is immediately final — no draft, no approve step: it must be
// visible via LoadLive (the brain's own read path) right away.
func TestPutLiveTariff_VisibleViaLoadLiveImmediately(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.PutLiveTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{
		Ref: "instant", Name: "Мгновенный", Price: "5 000 ₸",
	}); err != nil {
		t.Fatalf("put live tariff: %v", err)
	}
	snap, err := kb.LoadLive(ctx, orgID)
	if err != nil {
		t.Fatalf("load live: %v", err)
	}
	got, err := snap.Facts.Render("{{tariff.instant.price}}", "ru")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != "5 000 ₸" {
		t.Fatalf("want '5 000 ₸', got %q", got)
	}
}

// TestPutLiveTariffAndProduct_SalesStatusIsWired guards a gap Task 14 (frontend
// record components) surfaced: PutLiveTariff/PutLiveProduct read+preserved
// sales_status via currentLiveTariffTx/currentLiveProductTx's SELECT, but
// never actually applied in.SalesStatus onto cur before writing it back — the
// /kb/tariffs and /kb/products live editor silently could not change it,
// mirroring the OutsideZonesNote gap PatchPolicies had before Task 13.
func TestPutLiveTariffAndProduct_SalesStatusIsWired(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.PutLiveTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{
		Ref: "biz", Name: "Business", SalesStatus: "inactive",
	}); err != nil {
		t.Fatalf("put live tariff: %v", err)
	}
	if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "kettle2", Name: "Чайник", SalesStatus: "inactive",
	}); err != nil {
		t.Fatalf("put live product: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	var gotTariff, gotProduct string
	for _, tr := range live.Tariffs {
		if tr.Ref == "biz" {
			gotTariff = tr.SalesStatus
		}
	}
	for _, p := range live.Products {
		if p.Ref == "kettle2" {
			gotProduct = p.SalesStatus
		}
	}
	if gotTariff != "inactive" {
		t.Fatalf("tariff sales_status should be 'inactive', got %q", gotTariff)
	}
	if gotProduct != "inactive" {
		t.Fatalf("product sales_status should be 'inactive', got %q", gotProduct)
	}
}

// A per-entity approve must not be held hostage by an unrelated unanswered
// popup — only the whole-draft approve is blocked by pending requests.
func TestApprove_PerEntitySkipsPendingRequestsGate(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "unrelated_ok", Title: "OK", BodyMD: "Обычный текст без фактов.",
	}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	// An unrelated pending request — nothing to do with the topic above.
	if _, err := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{ReqType: "describe_file", Prompt: "?"}); err != nil {
		t.Fatalf("create request: %v", err)
	}

	// Per-entity approve of the unrelated topic must succeed despite the pending request.
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "topics", Key: "unrelated_ok"}); err != nil {
		t.Fatalf("per-entity approve should ignore the unrelated pending request: %v", err)
	}
	live, err := kb.LoadLive(ctx, orgID)
	if err != nil {
		t.Fatalf("load live: %v", err)
	}
	found := false
	for _, tp := range live.Topics {
		if tp.Slug == "unrelated_ok" {
			found = true
		}
	}
	if !found {
		t.Fatal("per-entity approve did not materialize the topic")
	}

	// The WHOLE-draft approve must still be blocked while the request is pending.
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "second", Title: "Ещё", BodyMD: "Ещё текст.",
	}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	err = kb.Approve(ctx, orgID, kbstore.ApproveSelector{})
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("whole-draft approve should still be blocked by the pending request, got %T: %v", err, err)
	}
}

// The draft blob's optimistic-concurrency token (base_version) advances on
// every write, and a concurrent writer is serialized (not lost) by the row
// lock in writeDraftBlob.
func TestDraftBaseVersionAdvances(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	v0, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if v0 != 0 {
		t.Fatalf("want 0 before any draft write, got %d", v0)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "a", BodyMD: "x"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	v1, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if v1 <= v0 {
		t.Fatalf("base_version should advance: v0=%d v1=%d", v0, v1)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "b", BodyMD: "y"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	v2, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if v2 <= v1 {
		t.Fatalf("base_version should advance again: v1=%d v2=%d", v1, v2)
	}
}

// queryInStock reads ai_products.in_stock directly, bypassing kbstore's own
// view builders — the tests below are asserting on the COLUMN upsertProductRow
// actually wrote, not on a read path that could hide the same bug.
func queryInStock(t *testing.T, db *dbx.DB, orgID uuid.UUID, ref string) bool {
	t.Helper()
	var v bool
	if err := db.QueryRow(context.Background(),
		`SELECT in_stock FROM ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref).Scan(&v); err != nil {
		t.Fatalf("query in_stock: %v", err)
	}
	return v
}

// PutLiveProduct's InStock is nil-able: nil means "no opinion" (schema
// default on insert, PRESERVE the existing value on update); only an explicit
// pointer overwrites it.
func TestPutLiveProduct_InStockDefaultInsertPreserveUpdateExplicitFalse(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	// insert, InStock nil -> schema default true.
	if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "kettle", Name: "Чайник", Price: "1 ₸",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !queryInStock(t, db, orgID, "kettle") {
		t.Fatal("insert with nil InStock should default to true")
	}

	// explicit false persists.
	f := false
	if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "kettle", Name: "Чайник", Price: "1 ₸", InStock: &f,
	}); err != nil {
		t.Fatalf("update explicit false: %v", err)
	}
	if queryInStock(t, db, orgID, "kettle") {
		t.Fatal("explicit false InStock should persist as false")
	}

	// a later write with InStock nil PRESERVES the existing (false) value —
	// never silently resets it back to true.
	if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "kettle", Name: "Чайник (переименован)", Price: "2 ₸",
	}); err != nil {
		t.Fatalf("update nil InStock: %v", err)
	}
	if queryInStock(t, db, orgID, "kettle") {
		t.Fatal("update with nil InStock should preserve the existing false value, not reset to true")
	}
}

// PatchLiveConfig fixes the historical silent no-op: a bare UPDATE against a
// fresh org with no ai_assistants row yet touched zero rows and reported
// success anyway. upsertConfigRow's ON CONFLICT DO UPDATE creates the row.
func TestPatchLiveConfig_UpsertsMissingRow(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	var before int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_assistants WHERE organization_id=$1`, orgID).Scan(&before); err != nil {
		t.Fatalf("count: %v", err)
	}
	if before != 0 {
		t.Fatalf("sanity: fresh org should have no ai_assistants row yet, got %d", before)
	}

	persona := "Новый ассистент магазина"
	if err := kb.PatchLiveConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	var got string
	if err := db.QueryRow(ctx, `SELECT persona FROM ai_assistants WHERE organization_id=$1`, orgID).Scan(&got); err != nil {
		t.Fatalf("PatchLiveConfig should have created the ai_assistants row: %v", err)
	}
	if got != persona {
		t.Fatalf("persona = %q, want %q", got, persona)
	}
}

// ApproveVersioned's config branch had the exact same historical bug
// PatchLiveConfig above was already fixed for, in a second, never-updated
// copy: a bare UPDATE against a fresh org (no ai_assistants row yet) touches
// zero rows, but the pending patch is unconditionally cleared from the draft
// right after — approving a first-ever assistant config silently discarded
// it instead of publishing it. Covers both call shapes that hit the same
// set.config branch: an entity-scoped approve (Kind:"config") and a
// whole-draft approve (Kind:"") that happens to include a pending config edit.
func TestApproveVersioned_ConfigUpsertsMissingRow_EntityScoped(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	var before int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_assistants WHERE organization_id=$1`, orgID).Scan(&before); err != nil {
		t.Fatalf("count: %v", err)
	}
	if before != 0 {
		t.Fatalf("sanity: fresh org should have no ai_assistants row yet, got %d", before)
	}

	persona := "Ассистент xPayment — эксперт по Kaspi Pay API"
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona}); err != nil {
		t.Fatalf("stage config patch: %v", err)
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "config", Key: kbstore.NaturalKeyMain}); err != nil {
		t.Fatalf("approve config: %v", err)
	}

	var got string
	if err := db.QueryRow(ctx, `SELECT persona FROM ai_assistants WHERE organization_id=$1`, orgID).Scan(&got); err != nil {
		t.Fatalf("approving config should have created the ai_assistants row: %v", err)
	}
	if got != persona {
		t.Fatalf("persona = %q, want %q", got, persona)
	}

	// The pending patch must actually be gone from the draft now that it
	// truly made it live — not cleared-but-lost.
	view, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if view.Config.Draft {
		t.Fatalf("draft config still marked pending after a successful approve")
	}
}

func TestApproveVersioned_ConfigUpsertsMissingRow_WholeDraft(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	persona := "Ассистент xPayment — эксперт по Kaspi Pay API"
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona}); err != nil {
		t.Fatalf("stage config patch: %v", err)
	}

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("whole-draft approve: %v", err)
	}

	var got string
	if err := db.QueryRow(ctx, `SELECT persona FROM ai_assistants WHERE organization_id=$1`, orgID).Scan(&got); err != nil {
		t.Fatalf("whole-draft approve should have created the ai_assistants row: %v", err)
	}
	if got != persona {
		t.Fatalf("persona = %q, want %q", got, persona)
	}
}

// zoneCompatiblePolicy makes orgID's '*' policy row zone-compatible (blank
// flat delivery fields, a non-blank outside_zones_note) — the precondition
// PutLiveZone's validateZoneWorld requires before any zone can be added.
func zoneCompatiblePolicy(t *testing.T, kb *kbstore.Store, orgID uuid.UUID) {
	t.Helper()
	blank, note := "", "не доставляем за пределами списка зон"
	if err := kb.PatchLivePolicies(context.Background(), orgID, uuid.Nil, kbstore.PolicyPatch{
		DeliveryCost: &blank, DeliveryInDays: &blank, OutsideZonesNote: &note,
	}); err != nil {
		t.Fatalf("seed zone-compatible policy: %v", err)
	}
}

// PutLiveZone/DeleteLiveZone re-validate the org's whole zone/policy world
// (validateZoneWorld) on every write: a contradictory zone, a dangling
// parent_ref, and deleting a zone another zone's parent_ref still points to
// are all rejected with a *GateError.
func TestZoneWrites_EnforceInvariant(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	zoneCompatiblePolicy(t, kb, orgID)

	var ge *kbstore.GateError

	// available=true with blank cost/days is rejected.
	err := kb.PutLiveZone(ctx, orgID, uuid.Nil, kbstore.ZoneInput{
		Ref: "almaty", Name: "Алматы", ZoneLevel: "city", DeliveryAvailable: true,
	})
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError for an available zone with no cost/days, got %T: %v", err, err)
	}

	// a fully-specified zone succeeds.
	if err := kb.PutLiveZone(ctx, orgID, uuid.Nil, kbstore.ZoneInput{
		Ref: "almaty", Name: "Алматы", ZoneLevel: "city",
		DeliveryAvailable: true, DeliveryCost: "5 000 ₸", DeliveryInDays: "1",
	}); err != nil {
		t.Fatalf("valid zone should succeed: %v", err)
	}

	// a dangling parent_ref is rejected.
	err = kb.PutLiveZone(ctx, orgID, uuid.Nil, kbstore.ZoneInput{
		Ref: "orphan", Name: "Ниоткуда", ZoneLevel: "city", ParentRef: "does-not-exist",
		DeliveryAvailable: true, DeliveryCost: "1 ₸", DeliveryInDays: "1",
	})
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError for a dangling parent_ref, got %T: %v", err, err)
	}

	// a valid child zone succeeds…
	if err := kb.PutLiveZone(ctx, orgID, uuid.Nil, kbstore.ZoneInput{
		Ref: "baikonur", Name: "Байконур", ZoneLevel: "city", ParentRef: "almaty", DeliveryAvailable: false,
	}); err != nil {
		t.Fatalf("child zone should succeed: %v", err)
	}
	// …and now deleting its parent is rejected (would dangle baikonur.parent_ref).
	err = kb.DeleteLiveZone(ctx, orgID, uuid.Nil, "almaty")
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError deleting a zone another zone's parent_ref points to, got %T: %v", err, err)
	}

	// deleting the child, then the parent, both succeed.
	if err := kb.DeleteLiveZone(ctx, orgID, uuid.Nil, "baikonur"); err != nil {
		t.Fatalf("delete leaf zone: %v", err)
	}
	if err := kb.DeleteLiveZone(ctx, orgID, uuid.Nil, "almaty"); err != nil {
		t.Fatalf("delete now-childless zone: %v", err)
	}
}

// PatchLivePolicies runs the same validateZoneWorld check before commit: once
// a zone exists, setting a non-blank flat delivery_cost/delivery_time is
// rejected, and the rejected write must not land (atomic with the gate check).
func TestPatchLivePolicies_BlockedWhileZonesExist(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()
	zoneCompatiblePolicy(t, kb, orgID)
	if err := kb.PutLiveZone(ctx, orgID, uuid.Nil, kbstore.ZoneInput{
		Ref: "almaty", Name: "Алматы", ZoneLevel: "city",
		DeliveryAvailable: true, DeliveryCost: "5 000 ₸", DeliveryInDays: "1",
	}); err != nil {
		t.Fatalf("zone: %v", err)
	}

	flat := "1 500 ₸ по Алматы"
	err := kb.PatchLivePolicies(ctx, orgID, uuid.Nil, kbstore.PolicyPatch{DeliveryCost: &flat})
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError setting a flat delivery_cost while zones exist, got %T: %v", err, err)
	}

	var gotCost string
	if err := db.QueryRow(ctx, `SELECT delivery_cost FROM ai_policies WHERE organization_id=$1`, orgID).Scan(&gotCost); err != nil {
		t.Fatalf("read policy: %v", err)
	}
	if gotCost != "" {
		t.Fatalf("rejected write must not land: delivery_cost = %q, want still blank", gotCost)
	}
}

// Every /kb/* live write commits an ai_audit_log row with the acting user's
// id, atomically with the write itself (auditRow runs in the same
// transaction as PutLiveTopic/DeleteLiveTopic's own statement).
func TestLiveWrites_AuditActor(t *testing.T) {
	kb, orgID, st, db := newTestKB(t)
	ctx := context.Background()
	user, err := st.SeedUser(ctx, orgID, "editor@xchats.test", "hash", "Editor")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if err := kb.PutLiveTopic(ctx, orgID, user.ID, kbstore.TopicInput{Slug: "t1", BodyMD: "Просто текст."}); err != nil {
		t.Fatalf("put topic: %v", err)
	}
	if err := kb.DeleteLiveTopic(ctx, orgID, user.ID, "t1"); err != nil {
		t.Fatalf("delete topic: %v", err)
	}

	rows, err := db.Query(ctx, `SELECT action, actor_user_id, note FROM ai_audit_log
		WHERE organization_id=$1 ORDER BY created_at`, orgID)
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var action, note string
		var actorID uuid.UUID
		if err := rows.Scan(&action, &actorID, &note); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if actorID != user.ID {
			t.Fatalf("audit row actor = %s, want %s", actorID, user.ID)
		}
		if note == "" {
			t.Fatal("audit row note should not be blank")
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(actions) != 2 || actions[0] != "edit" || actions[1] != "delete" {
		t.Fatalf("audit actions = %v, want [edit delete]", actions)
	}
}

// DeleteLiveConfig/DeleteLiveContacts/DeleteLivePolicies remove the org's one
// row from each singleton live table and, like every other /kb/* live write,
// record an audit row in the same transaction. "xchats kb-load -remove" is the
// caller these exist for; before they existed it reached past the store with
// raw SQL against a pgx pool.
func TestDeleteLiveSingletons(t *testing.T) {
	kb, orgID, st, db := newTestKB(t)
	ctx := context.Background()
	user, err := st.SeedUser(ctx, orgID, "remover@xchats.test", "hash", "Remover")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	persona, whatsapp, cost := "Помощник", "+7 700 000 00 00", "1000 ₸"
	if err := kb.PatchLiveConfig(ctx, orgID, user.ID, kbstore.ConfigPatch{Persona: &persona}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	if err := kb.PatchLiveContacts(ctx, orgID, user.ID, kbstore.ContactPatch{WhatsApp: &whatsapp}); err != nil {
		t.Fatalf("patch contacts: %v", err)
	}
	if err := kb.PatchLivePolicies(ctx, orgID, user.ID, kbstore.PolicyPatch{DeliveryCost: &cost}); err != nil {
		t.Fatalf("patch policies: %v", err)
	}

	countRows := func(table string) int {
		t.Helper()
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE organization_id=$1`, orgID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	for _, table := range []string{"ai_assistants", "ai_contacts", "ai_policies"} {
		if got := countRows(table); got != 1 {
			t.Fatalf("setup: %s has %d rows, want 1", table, got)
		}
	}

	if err := kb.DeleteLiveConfig(ctx, orgID, user.ID); err != nil {
		t.Fatalf("delete config: %v", err)
	}
	if err := kb.DeleteLiveContacts(ctx, orgID, user.ID); err != nil {
		t.Fatalf("delete contacts: %v", err)
	}
	if err := kb.DeleteLivePolicies(ctx, orgID, user.ID); err != nil {
		t.Fatalf("delete policies: %v", err)
	}
	for _, table := range []string{"ai_assistants", "ai_contacts", "ai_policies"} {
		if got := countRows(table); got != 0 {
			t.Fatalf("%s has %d rows after delete, want 0", table, got)
		}
	}

	// Deleting again is not an error: kb-load -remove is expected to be
	// re-runnable, and a DELETE matching zero rows is not a failure.
	if err := kb.DeleteLiveConfig(ctx, orgID, user.ID); err != nil {
		t.Fatalf("second delete config: %v", err)
	}

	var deletes int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_audit_log
		WHERE organization_id=$1 AND action='delete' AND actor_user_id=$2`, orgID, user.ID).Scan(&deletes); err != nil {
		t.Fatalf("count audit deletes: %v", err)
	}
	if deletes != 4 {
		t.Fatalf("audit delete rows = %d, want 4 (three deletes plus the idempotent repeat)", deletes)
	}
}

// TestApproveVersioned_WholeDraftClearsEveryEntry is the slice-aliasing
// regression: selectApproved's whole-draft branch handed ApproveVersioned's
// clear loop slice headers that ALIASED the blob's own backing arrays
// (`products: b.Products` — a by-value DraftBlob copies slice headers, not the
// arrays behind them), while removeProduct/removeTopic/removeTariff compact
// IN PLACE via `out := b.Products[:0]`. Iterating set.products while
// b.Products was being rewritten underneath made the loop read shifted data
// and skip every other entry: 4 staged left 1 behind, 7 left 3, 12 left 5.
//
// User-visible effect: "Опубликовать всё" published every record to live but
// never emptied Черновик, so the pending counter never reached zero and a
// stale phantom entry could later republish over a newer live edit.
//
// The bug is INVISIBLE at n<=2 (nothing is left behind), which is why every
// other approve test in this package passed against the broken code — none of
// them stages more than one row of a single kind before a successful
// whole-draft approve. Hence 5 of each kind here, covering all three of the
// slice loops that iterate an aliased set.
func TestApproveVersioned_WholeDraftClearsEveryEntry(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	const n = 5
	for i := 0; i < n; i++ {
		suffix := string(rune('a' + i))
		if _, err := kb.MCPUpsertProduct(ctx, orgID, uuid.Nil, "product-"+suffix, kbstore.ProductChanges{
			Name: strp("Товар " + suffix), InStock: boolp(true),
		}, nil, kbstore.MCPProvenance{}); err != nil {
			t.Fatalf("stage product %d: %v", i, err)
		}
		if _, err := kb.MCPUpsertTopic(ctx, orgID, uuid.Nil, "topic-"+suffix, kbstore.TopicChanges{
			Title: strp("Тема " + suffix), BodyMD: strp("Обычный текст без токенов и без сумм."),
		}, nil, kbstore.MCPProvenance{}); err != nil {
			t.Fatalf("stage topic %d: %v", i, err)
		}
		if _, err := kb.MCPUpsertTariff(ctx, orgID, uuid.Nil, "tariff-"+suffix, kbstore.TariffChanges{
			Name: strp("Тариф " + suffix), PricingType: strp("fixed"),
		}, nil, kbstore.MCPProvenance{}); err != nil {
			t.Fatalf("stage tariff %d: %v", i, err)
		}
	}

	if err := kb.ApproveVersioned(ctx, orgID, kbstore.ApproveSelector{}, nil, uuid.Nil); err != nil {
		t.Fatalf("approve whole draft: %v", err)
	}

	// Half one: the draft must be EMPTY. This is what failed before the fix.
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if got := len(changes.Products); got != 0 {
		t.Fatalf("draft still holds %d product(s) after a whole-draft approve, want 0 (aliasing regression)", got)
	}
	if got := len(changes.Topics); got != 0 {
		t.Fatalf("draft still holds %d topic(s) after a whole-draft approve, want 0 (aliasing regression)", got)
	}
	if got := len(changes.Tariffs); got != 0 {
		t.Fatalf("draft still holds %d tariff(s) after a whole-draft approve, want 0 (aliasing regression)", got)
	}

	// Half two: everything actually reached live — so a fix that empties the
	// draft without publishing cannot pass this test either.
	for _, tc := range []struct{ table, label string }{
		{"ai_products", "product"},
		{"ai_topics", "topic"},
		{"ai_tariffs", "tariff"},
	} {
		var live int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM `+tc.table+` WHERE organization_id = $1`, orgID).Scan(&live); err != nil {
			t.Fatalf("count %s: %v", tc.table, err)
		}
		if live != n {
			t.Fatalf("%s has %d live row(s) after approving %d staged %s(s), want %d", tc.table, live, n, tc.label, n)
		}
	}
}
