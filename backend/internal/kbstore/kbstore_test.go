package kbstore_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

// newTestKB resets the schema, migrates, seeds an org, and returns a KBStore.
func newTestKB(t *testing.T) (*kbstore.Store, uuid.UUID, *store.Store) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping kbstore DB test")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `DROP SCHEMA IF EXISTS xchats CASCADE; DROP TABLE IF EXISTS public.xchats_schema_migrations`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(st.Close)
	return kbstore.New(st.Pool()), org.ID, st
}

// The brain's snapshot must round-trip through the DB unchanged: seed → load →
// topic bodies are byte-identical (pure prose) and every fact in the [F] catalog
// resolves to the SAME verbatim value from the DB snapshot as from the literal —
// INCLUDING unit-bearing amounts the old typed PriceBook/formatTenge path dropped.
func TestRoundTrip_RendersIdentically(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
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
	// Every advertised fact resolves to the same value from either snapshot.
	facts := seed.Facts.List()
	if len(facts) == 0 {
		t.Fatal("seed advertises no facts to round-trip")
	}
	for _, f := range facts {
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
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.UpsertTariff(ctx, orgID, kbstore.TariffInput{
		Ref: "growth", Lang: "ru", Name: "Рост", Price: "25 000 ₸/мес",
	}); err != nil {
		t.Fatalf("upsert tariff: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}, nil); err != nil {
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
	kb, orgID, _ := newTestKB(t)
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

	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{
		Slug: "new_topic", Lang: "ru", Title: "Новая тема", BodyMD: "Просто текст.",
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

	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "topics", Key: "new_topic"}, nil); err != nil {
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

// Every DraftView collection must serialize as a JSON array ([]), never null —
// even when a table + the blob are empty. The client reads d.<coll>.length
// directly, so a nil slice (→ null) would crash the /playground /knowledge-base
// pages. (Regression: the typed-facts rewrite left empty slices nil.)
func TestDraft_CollectionsNeverNull(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	// No seed → every table AND the blob is empty; the worst case for nil slices.
	view, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if view.Topics == nil || view.Assets == nil || view.Tariffs == nil ||
		view.Products == nil || view.Contacts == nil || view.Policies == nil ||
		view.Materials == nil || view.Requests == nil {
		t.Fatalf("a collection is nil: topics=%v assets=%v tariffs=%v products=%v contacts=%v policies=%v materials=%v requests=%v",
			view.Topics == nil, view.Assets == nil, view.Tariffs == nil, view.Products == nil,
			view.Contacts == nil, view.Policies == nil, view.Materials == nil, view.Requests == nil)
	}
	blob, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, k := range []string{"topics", "assets", "tariffs", "products", "contacts", "policies", "materials", "requests"} {
		if strings.Contains(string(blob), `"`+k+`":null`) {
			t.Fatalf("collection %q serialized as null (must be []): %s", k, blob)
		}
	}
}

// The deterministic gate blocks approve on an undescribed asset and a topic body
// that is not pure prose (carries a fact token), listing every reason.
func TestApproveGateBlocks(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// An asset with no description + a topic whose body carries a fact token
	// (bodies must be pure prose — 14 D3).
	if err := kb.UpsertAsset(ctx, orgID, kbstore.AssetInput{Ref: "loose_img", Kind: "image", Description: ""}); err != nil {
		t.Fatalf("asset: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{Slug: "broken", Lang: "ru", BodyMD: "Цена {{tariff.unknown.price}}"}); err != nil {
		t.Fatalf("topic: %v", err)
	}
	err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}, nil)
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError, got %T: %v", err, err)
	}
	if len(ge.Reasons) < 2 {
		t.Fatalf("want >=2 gate reasons, got %v", ge.Reasons)
	}
}

// LiveView must NEVER see pending Playground work — it ignores the draft blob
// entirely, so a live edit (/knowledge-base) can never mix with a draft edit
// (/playground). Every row it returns is Draft:false; materials/requests are
// always empty (they are playground-only concepts).
func TestLiveView_IgnoresDraftBlob(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Stage a PENDING topic in the draft blob — never approved.
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{
		Slug: "pending_only", Lang: "ru", Title: "Черновик", BodyMD: "Только в черновике.",
	}); err != nil {
		t.Fatalf("upsert draft topic: %v", err)
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
	if len(live.Materials) != 0 || len(live.Requests) != 0 {
		t.Fatalf("LiveView must not carry materials/requests (playground-only), got materials=%d requests=%d",
			len(live.Materials), len(live.Requests))
	}
}

// PutLiveTopic is held to the same content bar as the approve gate: a body
// carrying a fact token is rejected immediately (no silent bad write).
func TestPutLiveTopic_RejectsNonProseBody(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	err := kb.PutLiveTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
		Slug: "broken", Lang: "ru", BodyMD: "Цена {{tariff.unknown.price}}",
	})
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError for a token in the body, got %T: %v", err, err)
	}
}

// A live write is immediately final — no draft, no approve step: it must be
// visible via LoadLive (the brain's own read path) right away.
func TestPutLiveTariff_VisibleViaLoadLiveImmediately(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.PutLiveTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{
		Ref: "instant", Lang: "ru", Name: "Мгновенный", Price: "5 000 ₸",
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

// A per-entity approve must not be held hostage by an unrelated unanswered
// popup — only the whole-draft approve is blocked by pending requests.
func TestApprove_PerEntitySkipsPendingRequestsGate(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{
		Slug: "unrelated_ok", Lang: "ru", Title: "OK", BodyMD: "Обычный текст без фактов.",
	}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	// An unrelated pending request — nothing to do with the topic above.
	if _, err := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{ReqType: "describe_media", Prompt: "?"}); err != nil {
		t.Fatalf("create request: %v", err)
	}

	// Per-entity approve of the unrelated topic must succeed despite the pending request.
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "topics", Key: "unrelated_ok"}, nil); err != nil {
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
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{
		Slug: "second", Lang: "ru", Title: "Ещё", BodyMD: "Ещё текст.",
	}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	err = kb.Approve(ctx, orgID, kbstore.ApproveSelector{}, nil)
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("whole-draft approve should still be blocked by the pending request, got %T: %v", err, err)
	}
}

// The draft blob's optimistic-concurrency token (base_version) advances on
// every write, and a concurrent writer is serialized (not lost) by the row
// lock in writeDraftBlob.
func TestDraftBaseVersionAdvances(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	v0, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if v0 != 0 {
		t.Fatalf("want 0 before any draft write, got %d", v0)
	}
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{Slug: "a", Lang: "ru", BodyMD: "x"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	v1, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	if v1 <= v0 {
		t.Fatalf("base_version should advance: v0=%d v1=%d", v0, v1)
	}
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{Slug: "b", Lang: "ru", BodyMD: "y"}); err != nil {
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
func queryInStock(t *testing.T, st *store.Store, orgID uuid.UUID, ref string) bool {
	t.Helper()
	var v bool
	if err := st.Pool().QueryRow(context.Background(),
		`SELECT in_stock FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref).Scan(&v); err != nil {
		t.Fatalf("query in_stock: %v", err)
	}
	return v
}

// PutLiveProduct's InStock is nil-able: nil means "no opinion" (schema
// default on insert, PRESERVE the existing value on update); only an explicit
// pointer overwrites it.
func TestPutLiveProduct_InStockDefaultInsertPreserveUpdateExplicitFalse(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()

	// insert, InStock nil -> schema default true.
	if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "kettle", Lang: "ru", Name: "Чайник", Price: "1 ₸",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !queryInStock(t, st, orgID, "kettle") {
		t.Fatal("insert with nil InStock should default to true")
	}

	// explicit false persists.
	f := false
	if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "kettle", Lang: "ru", Name: "Чайник", Price: "1 ₸", InStock: &f,
	}); err != nil {
		t.Fatalf("update explicit false: %v", err)
	}
	if queryInStock(t, st, orgID, "kettle") {
		t.Fatal("explicit false InStock should persist as false")
	}

	// a later write with InStock nil PRESERVES the existing (false) value —
	// never silently resets it back to true.
	if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
		Ref: "kettle", Lang: "ru", Name: "Чайник (переименован)", Price: "2 ₸",
	}); err != nil {
		t.Fatalf("update nil InStock: %v", err)
	}
	if queryInStock(t, st, orgID, "kettle") {
		t.Fatal("update with nil InStock should preserve the existing false value, not reset to true")
	}
}

// PatchLiveConfig fixes the historical silent no-op: a bare UPDATE against a
// fresh org with no ai_assistants row yet touched zero rows and reported
// success anyway. upsertConfigRow's ON CONFLICT DO UPDATE creates the row.
func TestPatchLiveConfig_UpsertsMissingRow(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()

	var before int
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_assistants WHERE organization_id=$1`, orgID).Scan(&before); err != nil {
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
	if err := st.Pool().QueryRow(ctx, `SELECT persona FROM xchats.ai_assistants WHERE organization_id=$1`, orgID).Scan(&got); err != nil {
		t.Fatalf("PatchLiveConfig should have created the ai_assistants row: %v", err)
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
		DeliveryCost: &blank, DeliveryTime: &blank, OutsideZonesNote: &note,
	}); err != nil {
		t.Fatalf("seed zone-compatible policy: %v", err)
	}
}

// PutLiveZone/DeleteLiveZone re-validate the org's whole zone/policy world
// (validateZoneWorld) on every write: a contradictory zone, a dangling
// parent_ref, and deleting a zone another zone's parent_ref still points to
// are all rejected with a *GateError.
func TestZoneWrites_EnforceInvariant(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
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
	kb, orgID, st := newTestKB(t)
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
	if err := st.Pool().QueryRow(ctx, `SELECT delivery_cost FROM xchats.ai_policies WHERE organization_id=$1 AND lang='*'`, orgID).Scan(&gotCost); err != nil {
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
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()
	user, err := st.SeedUser(ctx, orgID, "editor@xchats.test", "hash", "Editor")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if err := kb.PutLiveTopic(ctx, orgID, user.ID, kbstore.TopicInput{Slug: "t1", Lang: "ru", BodyMD: "Просто текст."}); err != nil {
		t.Fatalf("put topic: %v", err)
	}
	if err := kb.DeleteLiveTopic(ctx, orgID, user.ID, "t1"); err != nil {
		t.Fatalf("delete topic: %v", err)
	}

	rows, err := st.Pool().Query(ctx, `SELECT action, actor_user_id, note FROM xchats.ai_audit_log
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
