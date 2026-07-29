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
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("org: %v", err)
	}
	kb := kbstore.New(st.Pool())
	if err := kb.SeedLiveIfEmpty(ctx, org.ID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed kb: %v", err)
	}
	t.Cleanup(st.Close)
	return kb, org.ID
}

// The flagship loop: drop a text material with a price → the builder makes ONE
// PURE-PROSE topic (the amount excised, never a digit or token in the body) + a
// confirm_fact popup → confirm the fact onto a typed tariff column + approve the
// whole draft → the brain's live KB renders the confirmed price via its typed
// {{tariff.slug.price}} token. This is "build KB from raw materials, then answer".
func TestBuildFromMaterialsToApprove(t *testing.T) {
	kb, orgID := newKB(t)
	ctx := context.Background()

	// 1. Drop raw material (text). Born ready (no extraction needed).
	if _, err := kb.CreateMaterial(ctx, orgID, kbstore.MaterialInput{
		SourceType: "text", SourceRef: "chat",
		Text: "Тариф Рост стоит 25 000 ₸/мес и включает поддержку.",
	}); err != nil {
		t.Fatalf("material: %v", err)
	}

	// 2. Build: synthesize the materials into the draft blob.
	b := playground.NewBuilder(nil, nil)
	res, err := b.RunTurn(ctx, kb, orgID, "Вот мой тариф, собери базу знаний")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if res.TopicsUpserted != 1 {
		t.Fatalf("want 1 topic, got %d", res.TopicsUpserted)
	}
	if res.Confirms != 1 {
		t.Fatalf("want 1 confirm_fact popup (the detected price), got %d", res.Confirms)
	}

	view, _ := kb.Draft(ctx, orgID)
	// The body is pure prose: the amount was EXCISED (no digit) and NOT re-inserted
	// as a token — a fact token in stored knowledge would be gated (14 D3).
	for _, tp := range view.Topics {
		if strings.Contains(tp.BodyMD, "25 000") {
			t.Fatalf("raw price digit leaked into body: %q", tp.BodyMD)
		}
		if strings.Contains(tp.BodyMD, "{{") {
			t.Fatalf("fact token leaked into a topic body (must be pure prose): %q", tp.BodyMD)
		}
	}

	// 3. Confirm the detected fact onto its typed tariff column (what the resolve
	// endpoint does via SetFactField) + resolve the popup — both are pending in the
	// draft blob until approve.
	if err := kb.SetFactField(ctx, orgID, "tariff", "item_1", "price", "25 000 ₸/мес"); err != nil {
		t.Fatalf("confirm fact: %v", err)
	}
	for _, r := range view.Requests {
		if r.ReqType == "confirm_fact" {
			if err := kb.ResolveRequest(ctx, r.ID, "resolved", `{}`); err != nil {
				t.Fatalf("resolve request: %v", err)
			}
		}
	}

	// 4. Approve the whole draft → the gate passes and every pending entry
	// materializes into the live tables (topic + the typed tariff row).
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// 5. The brain's LIVE KB now renders the confirmed price verbatim (unit kept),
	// resolved from the typed ai_tariffs column via its 3-part token.
	snap, err := kb.LoadLive(ctx, orgID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, err := snap.Facts.Render("Цена — {{tariff.item_1.price}}.", "ru")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != "Цена — 25 000 ₸/мес." {
		t.Fatalf("live KB render: %q", got)
	}
}

// The URL adapter does a best-effort fetch → readable text (status ready).
func TestExtractURL_Ready(t *testing.T) {
	kb, orgID := newKB(t)
	ctx := context.Background()
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
// describe_file popup (the human fallback IS the pipeline).
func TestExtractMedia_FallsBackToDescribePopup(t *testing.T) {
	kb, orgID := newKB(t)
	ctx := context.Background()
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
	view, _ := kb.Draft(ctx, orgID)
	hasDescribe := false
	for _, r := range view.Requests {
		if r.ReqType == "describe_file" && r.State == "pending" {
			hasDescribe = true
		}
	}
	if !hasDescribe {
		t.Fatalf("expected a pending describe_file popup, got %+v", view.Requests)
	}
}

// The counterpart to TestExtractMedia_FallsBackToDescribePopup: an operator
// description staged WITH the upload substitutes for auto-extraction — the
// material is born ready immediately (no extraction job needed, no popup).
// The builder folds that description into the SAME topic-body prose as any
// other ready material (there is no generic asset section any more — see
// Plan's doc comment in builder.go) and raises no describe_file popup for it.
func TestCreateMaterial_WithDescription_BornReadyNoPopup(t *testing.T) {
	kb, orgID := newKB(t)
	ctx := context.Background()

	m, err := kb.CreateMaterial(ctx, orgID, kbstore.MaterialInput{
		SourceType: "image", SourceRef: "tariff.png", BlobID: uuid.NewString(), MediaKind: "image",
		Description: "Скриншот тарифов: Базовый 9900, Стандарт 19900.",
	})
	if err != nil {
		t.Fatalf("material: %v", err)
	}
	if m.Status != "ready" {
		t.Fatalf("described material should be born ready, got %q", m.Status)
	}
	if m.ExtractedText != "Скриншот тарифов: Базовый 9900, Стандарт 19900." {
		t.Fatalf("extracted_text should be the description verbatim, got %q", m.ExtractedText)
	}

	b := playground.NewBuilder(nil, nil)
	res, err := b.RunTurn(ctx, kb, orgID, "Собери базу знаний по скриншоту")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if res.Describes != 0 {
		t.Fatalf("a described material must not raise describe_file, got %d", res.Describes)
	}

	view, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	for _, r := range view.Requests {
		if r.ReqType == "describe_file" {
			t.Fatalf("a described material must not raise describe_file, got %+v", r)
		}
	}
	foundInTopicBody := false
	for _, top := range view.Topics {
		if strings.Contains(top.BodyMD, "Базовый 9900") {
			foundInTopicBody = true
		}
	}
	if !foundInTopicBody {
		t.Fatalf("expected the material's description to land in a topic body, got %+v", view.Topics)
	}
}
