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
	err := kb.PutLiveTopic(ctx, orgID, kbstore.TopicInput{
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
	if err := kb.PutLiveTariff(ctx, orgID, kbstore.TariffInput{
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
