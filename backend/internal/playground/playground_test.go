package playground_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/playground"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

func newKB(t *testing.T) (*kbstore.Store, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping playground DB test")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `DROP SCHEMA IF EXISTS xchats CASCADE; DROP TABLE IF EXISTS public.xchats_schema_migrations`); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	org, err := st.SeedOrganization(ctx, "XChats")
	if err != nil {
		t.Fatalf("org: %v", err)
	}
	kb := kbstore.New(st.Pool())
	if err := kb.SeedIfEmpty(ctx, org.ID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed kb: %v", err)
	}
	t.Cleanup(st.Close)
	return kb, org.ID
}

// The flagship loop: drop a text material with a price → the builder makes ONE
// topic with the price tokenized (never a digit) + a confirm_value popup → confirm
// the value + approve the topic → publish → the brain's published KB renders the
// confirmed price. This is "build KB from raw materials, then answer from it".
func TestBuildFromMaterialsToPublish(t *testing.T) {
	kb, orgID := newKB(t)
	ctx := context.Background()
	if _, err := kb.OpenDraft(ctx, orgID); err != nil {
		t.Fatalf("open draft: %v", err)
	}

	// 1. Drop raw material (text). Born ready (no extraction needed).
	if _, err := kb.CreateMaterial(ctx, orgID, kbstore.MaterialInput{
		SourceType: "text", SourceRef: "chat",
		Text: "Тариф Рост стоит 25 000 ₸/мес и включает поддержку.",
	}); err != nil {
		t.Fatalf("material: %v", err)
	}

	// 2. Build: synthesize the materials into a draft.
	b := playground.NewBuilder(nil, nil)
	res, err := b.RunTurn(ctx, kb, orgID, "Вот мой тариф, собери базу знаний")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if res.TopicsUpserted != 1 {
		t.Fatalf("want 1 topic, got %d", res.TopicsUpserted)
	}
	if res.Confirms != 1 {
		t.Fatalf("want 1 confirm_value popup (the detected price), got %d", res.Confirms)
	}

	view, _ := kb.GetDraft(ctx, orgID)
	// The price must be a token, never a digit in the body (the core guardrail).
	var topicID uuid.UUID
	foundToken := false
	for _, tp := range view.Topics {
		if strings.Contains(tp.BodyMD, "{{price.item_1}}") {
			foundToken = true
			topicID = tp.ID
		}
		if strings.Contains(tp.BodyMD, "25 000") {
			t.Fatalf("raw price digit leaked into body: %q", tp.BodyMD)
		}
	}
	if !foundToken {
		t.Fatalf("price was not tokenized into the body: %+v", view.Topics)
	}

	// 3. Confirm the detected value + resolve its popup (what the resolve endpoint
	// does) + approve the proposed topic.
	if err := kb.UpsertValue(ctx, orgID, kbstore.ValueInput{
		Token: "price.item_1", Lang: "ru", ValueText: "25 000 ₸/мес", ReviewState: "approved",
	}); err != nil {
		t.Fatalf("confirm value: %v", err)
	}
	for _, r := range view.Requests {
		if r.ReqType == "confirm_value" {
			if err := kb.ResolveRequest(ctx, r.ID, "resolved", `{}`); err != nil {
				t.Fatalf("resolve request: %v", err)
			}
		}
	}
	if err := kb.SetReviewState(ctx, orgID, "topics", topicID, "approved"); err != nil {
		t.Fatalf("approve topic: %v", err)
	}

	// 4. Publish → the gate passes and the version bumps.
	version, err := kb.Publish(ctx, orgID, nil)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if version != 2 {
		t.Fatalf("want version 2, got %d", version)
	}

	// 5. The brain's published KB now renders the confirmed price verbatim (unit kept).
	snap, err := kb.LoadPublished(ctx, orgID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, err := snap.Values.Render("Цена — {{price.item_1}}.", "ru")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != "Цена — 25 000 ₸/мес." {
		t.Fatalf("published KB render: %q", got)
	}
}

// The URL adapter does a best-effort fetch → readable text (status ready).
func TestExtractURL_Ready(t *testing.T) {
	kb, orgID := newKB(t)
	ctx := context.Background()
	if _, err := kb.OpenDraft(ctx, orgID); err != nil {
		t.Fatalf("open: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>Тарифы</h1><p>Рост — 25 000 ₸</p><script>x()</script></body></html>`))
	}))
	defer srv.Close()

	m, err := kb.CreateMaterial(ctx, orgID, kbstore.MaterialInput{SourceType: "url", SourceRef: srv.URL})
	if err != nil {
		t.Fatalf("material: %v", err)
	}
	ex := playground.NewExtractor(nil, nil)
	ex.AllowPrivateHosts = true // httptest binds loopback; opt past the SSRF guard
	bs, _ := blob.NewDisk(t.TempDir())
	if err := ex.Extract(ctx, kb, bs, nil, orgID, m.ID); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, _ := kb.GetMaterial(ctx, m.ID)
	if got.Status != "ready" {
		t.Fatalf("want ready, got %q (%s)", got.Status, got.Extraction)
	}
	if !strings.Contains(got.ExtractedText, "Тарифы") || strings.Contains(got.ExtractedText, "x()") {
		t.Fatalf("html not cleaned: %q", got.ExtractedText)
	}
}

// With no vision client, a media drop can't auto-extract → needs_human + a
// describe_media popup (the human fallback IS the pipeline).
func TestExtractMedia_FallsBackToDescribePopup(t *testing.T) {
	kb, orgID := newKB(t)
	ctx := context.Background()
	if _, err := kb.OpenDraft(ctx, orgID); err != nil {
		t.Fatalf("open: %v", err)
	}
	bs, _ := blob.NewDisk(t.TempDir())
	blobID := uuid.NewString()
	_, _ = bs.Put(blobID, []byte("\x89PNG..."), blob.Meta{MediaType: "image", Mimetype: "image/png"})
	m, err := kb.CreateMaterial(ctx, orgID, kbstore.MaterialInput{
		SourceType: "image", SourceRef: "tariff.png", BlobID: blobID, MediaKind: "image",
	})
	if err != nil {
		t.Fatalf("material: %v", err)
	}
	ex := playground.NewExtractor(nil, nil) // no vision
	if err := ex.Extract(ctx, kb, bs, nil, orgID, m.ID); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, _ := kb.GetMaterial(ctx, m.ID)
	if got.Status != "needs_human" {
		t.Fatalf("want needs_human, got %q", got.Status)
	}
	view, _ := kb.GetDraft(ctx, orgID)
	hasDescribe := false
	for _, r := range view.Requests {
		if r.ReqType == "describe_media" && r.State == "pending" {
			hasDescribe = true
		}
	}
	if !hasDescribe {
		t.Fatalf("expected a pending describe_media popup, got %+v", view.Requests)
	}
}
