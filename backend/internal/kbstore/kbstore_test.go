package kbstore_test

import (
	"context"
	"errors"
	"os"
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
	org, err := st.SeedOrganization(ctx, "XChats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(st.Close)
	return kbstore.New(st.Pool()), org.ID, st
}

// The brain's snapshot must round-trip through the DB unchanged: seed → load →
// every topic renders identically, INCLUDING unit-bearing values that the old
// typed PriceBook/formatTenge path could not represent.
func TestRoundTrip_RendersIdentically(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	seed := brain.SeedSnapshot()
	if err := kb.SeedIfEmpty(ctx, orgID, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	loaded, err := kb.LoadPublished(ctx, orgID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Config.Persona != seed.Config.Persona {
		t.Fatalf("persona lost: %q", loaded.Config.Persona)
	}
	if len(loaded.Topics) != len(seed.Topics) {
		t.Fatalf("topics: want %d got %d", len(seed.Topics), len(loaded.Topics))
	}
	// Every seed topic must render the same from the DB snapshot as from the literal.
	for _, topic := range seed.Topics {
		want, werr := seed.Values.Render(topic.BodyMD, topic.Language)
		got, gerr := loaded.Values.Render(topic.BodyMD, topic.Language)
		if werr != nil || gerr != nil {
			t.Fatalf("render %q: seed=%v db=%v", topic.Slug, werr, gerr)
		}
		if want != got {
			t.Fatalf("topic %q render mismatch:\n seed: %q\n db:   %q", topic.Slug, want, got)
		}
	}
}

// The lossy-bridge fix: a value carrying a unit ("/мес") survives the DB round-trip
// verbatim — the regression the old int-tenge PriceBook silently dropped.
func TestRoundTrip_UnitBearingValue(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := kb.OpenDraft(ctx, orgID); err != nil {
		t.Fatalf("open draft: %v", err)
	}
	if err := kb.UpsertValue(ctx, orgID, kbstore.ValueInput{
		Token: "price.growth", Lang: "ru", ValueText: "25 000 ₸/мес",
	}); err != nil {
		t.Fatalf("upsert value: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{
		Slug: "growth", Lang: "ru", BodyMD: "Тариф Рост — {{price.growth}}.",
	}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	if _, err := kb.Publish(ctx, orgID, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	loaded, err := kb.LoadPublished(ctx, orgID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, err := loaded.Values.Render("Тариф Рост — {{price.growth}}.", "ru")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if want := "Тариф Рост — 25 000 ₸/мес."; got != want {
		t.Fatalf("unit dropped:\n want %q\n got  %q", want, got)
	}
}

// Open clones the published baseline into the draft; publishing bumps the version
// and the new snapshot becomes the one the brain loads.
func TestDraftCloneAndPublishBumpsVersion(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	view, err := kb.GetDraft(ctx, orgID) // opens + clones
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if len(view.Topics) == 0 {
		t.Fatal("draft should clone published topics")
	}
	if view.Config.Version != 0 {
		t.Fatalf("draft version should be 0, got %d", view.Config.Version)
	}
	v, err := kb.Publish(ctx, orgID, nil)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if v != 2 { // seed was version 1
		t.Fatalf("want new version 2, got %d", v)
	}
	loaded, _ := kb.LoadPublished(ctx, orgID)
	if loaded.Config.Version != 2 {
		t.Fatalf("brain should load version 2, got %d", loaded.Config.Version)
	}
}

// The deterministic gate blocks publish on an undescribed asset and an unresolved
// token, listing every reason.
func TestPublishGateBlocks(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := kb.OpenDraft(ctx, orgID); err != nil {
		t.Fatalf("open: %v", err)
	}
	// An asset with no description + a topic with an unresolved token.
	if err := kb.UpsertAsset(ctx, orgID, kbstore.AssetInput{Ref: "loose_img", Kind: "image", Description: ""}); err != nil {
		t.Fatalf("asset: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{Slug: "broken", Lang: "ru", BodyMD: "Цена {{price.unknown}}"}); err != nil {
		t.Fatalf("topic: %v", err)
	}
	_, err := kb.Publish(ctx, orgID, nil)
	var ge *kbstore.GateError
	if !errors.As(err, &ge) {
		t.Fatalf("want GateError, got %T: %v", err, err)
	}
	if len(ge.Reasons) < 2 {
		t.Fatalf("want >=2 gate reasons, got %v", ge.Reasons)
	}
}
