package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
)

// promptPayload mirrors httpapi's (unexported) promptView JSON shape — this is
// an external test package, so it decodes GET /kb/prompt's response into its
// own copy of that shape rather than reaching into httpapi internals.
type promptPayload struct {
	PromptRef     string `json:"prompt_ref"`
	RenderedText  string `json:"rendered_text"`
	FrameText     string `json:"frame_text"`
	CharCount     int    `json:"char_count"`
	ApproxTokens  int    `json:"approx_tokens"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	SectionCounts struct {
		Topics   int `json:"topics"`
		Products int `json:"products"`
		Tariffs  int `json:"tariffs"`
		Zones    int `json:"zones"`
		Contacts int `json:"contacts"`
		Policies int `json:"policies"`
	} `json:"section_counts"`
}

// TestKBPrompt_OkWithSeededKB proves GET /kb/prompt renders the exact prompt
// the response engine would use for newHarness's seeded org (brain.SeedSnapshot,
// via kb.SeedLiveIfEmpty): status ok, the pinned prompt_ref, the ЗОНЫ ДОСТАВКИ
// section marker and a product block present, and section_counts matching the
// seeded row counts (3 tariffs, 2 products, 5 topics, 1 contacts row, 1
// policies row — brain/seed.go).
func TestKBPrompt_OkWithSeededKB(t *testing.T) {
	h := newHarness(t)
	var got promptPayload
	h.get("/xchats/api/v1/kb/prompt", &got)

	if got.Status != "ok" {
		t.Fatalf("status = %q, want ok (error=%q)", got.Status, got.Error)
	}
	if got.PromptRef != "shop-kb@v6" {
		t.Fatalf("prompt_ref = %q, want shop-kb@v6", got.PromptRef)
	}
	if !strings.Contains(got.RenderedText, "ЗОНЫ ДОСТАВКИ") {
		t.Fatalf("rendered_text missing the ЗОНЫ ДОСТАВКИ section marker:\n%s", got.RenderedText)
	}
	if !strings.Contains(got.RenderedText, "product: coffee-machine") {
		t.Fatalf("rendered_text missing a product block for the seeded coffee-machine:\n%s", got.RenderedText)
	}
	if got.SectionCounts.Products != 2 {
		t.Fatalf("section_counts.products = %d, want 2", got.SectionCounts.Products)
	}
	if got.SectionCounts.Tariffs != 3 {
		t.Fatalf("section_counts.tariffs = %d, want 3", got.SectionCounts.Tariffs)
	}
	if got.SectionCounts.Topics != 5 {
		t.Fatalf("section_counts.topics = %d, want 5", got.SectionCounts.Topics)
	}
	if got.SectionCounts.Contacts != 1 || got.SectionCounts.Policies != 1 {
		t.Fatalf("section_counts contacts/policies = %d/%d, want 1/1", got.SectionCounts.Contacts, got.SectionCounts.Policies)
	}
	if got.CharCount != len(got.RenderedText) {
		t.Fatalf("char_count = %d, want %d (len of rendered_text)", got.CharCount, len(got.RenderedText))
	}
	if got.ApproxTokens != got.CharCount/4 {
		t.Fatalf("approx_tokens = %d, want %d (char_count/4)", got.ApproxTokens, got.CharCount/4)
	}
	if !strings.Contains(got.FrameText, "%%PRODUCTS_AVAILABLE%%") {
		t.Fatalf("frame_text should be the raw frame with unfilled slots")
	}
}

// TestKBPrompt_InvalidationOnWrite proves the cache-invalidation architecture
// end to end: a /kb/products write must be visible on the VERY NEXT GET
// /kb/prompt, not up to 60s later (CachedKBRepo's TTL backstop) — because
// kbLiveChanged calls s.kbInvalidator.Invalidate synchronously. A distinctive
// marker in the product description proves this directly: absent before the
// write, present immediately after.
func TestKBPrompt_InvalidationOnWrite(t *testing.T) {
	h := newHarness(t)
	const marker = "МАРКЕР-ОБНОВЛЕНИЯ-92f1"

	var before promptPayload
	h.get("/xchats/api/v1/kb/prompt", &before)
	if before.Status != "ok" {
		t.Fatalf("status = %q, want ok (error=%q)", before.Status, before.Error)
	}
	if strings.Contains(before.RenderedText, marker) {
		t.Fatal("sanity: marker should not be present before the write")
	}

	resp, _ := h.postJSON("/xchats/api/v1/kb/products", map[string]any{
		"ref": "coffee-machine", "name": "Кофемашина DeLonghi", "price": "129 900 ₸",
		"description": marker, "category": "Техника", "availability_status": "in_stock",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upsert product status = %d, want 200", resp.StatusCode)
	}

	var after promptPayload
	h.get("/xchats/api/v1/kb/prompt", &after)
	if after.Status != "ok" {
		t.Fatalf("status = %q, want ok (error=%q)", after.Status, after.Error)
	}
	if !strings.Contains(after.RenderedText, marker) {
		t.Fatalf("rendered_text should reflect the just-written product immediately (cache not invalidated):\n%s", after.RenderedText)
	}
}

// TestKBPrompt_ErrorWhenNotConfigured proves an org with no ai_assistants row
// at all (never seeded, never kb-load-ed) gets an honest status:"error" —
// never a hardcoded or fabricated prompt — with the exact
// responsestore.ErrKBNotConfigured message surfaced.
func TestKBPrompt_ErrorWhenNotConfigured(t *testing.T) {
	h := newPromptHarness(t)
	var got promptPayload
	h.get("/xchats/api/v1/kb/prompt", &got)

	if got.Status != "error" {
		t.Fatalf("status = %q, want error (rendered_text=%q)", got.Status, got.RenderedText)
	}
	if !strings.Contains(got.Error, "not configured") {
		t.Fatalf("error = %q, want it to mention the KB is not configured", got.Error)
	}
	if got.RenderedText != "" {
		t.Fatalf("rendered_text should be empty on error, got %q", got.RenderedText)
	}
}

// --- promptHarness: a minimal server for the "KB not configured" case -----
//
// Unlike newHarness (integration_test.go), this deliberately never calls
// kb.SeedLiveIfEmpty — the org starts with NO ai_assistants row, the exact
// precondition GET /kb/prompt's status:"error" path needs. It wires only what
// that one read-only route (plus login) touches: store/auth and a CachedKBRepo,
// mirroring main.go's real production wiring.
type promptHarness struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
}

func newPromptHarness(t *testing.T) *promptHarness {
	t.Helper()
	ctx := context.Background()
	// Fresh, migrated database per test — see newHarnessWithLLM's own comment
	// in integration_test.go for why the DATABASE_URL gate is gone. This
	// harness deliberately does NOT call kb.SeedLiveIfEmpty: the org starting
	// with no ai_assistants row is this suite's whole precondition.
	st, db := dbtest.Open(t)

	cfg := &config.Config{
		System:   config.SystemConfig{SessionTTLHours: 1, MinPasswordLen: 8},
		Server:   config.ServerConfig{CORSOrigins: []string{"*"}},
		PageSize: 50,
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	org, err := st.SeedOrganization(ctx, "prompt-not-configured-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	hash, _ := password.Hash(adminPass)
	if _, err := st.SeedUser(ctx, org.ID, adminEmail, hash, "Admin"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	kb, err := kbstore.New(ctx, db.Path())
	if err != nil {
		t.Fatalf("kbstore: %v", err)
	}
	t.Cleanup(kb.Close)
	kbRepo, err := responsestore.NewKnowledgeBaseRepo(ctx, db.Path())
	if err != nil {
		t.Fatalf("kb repo: %v", err)
	}
	t.Cleanup(kbRepo.Close)
	cached := responsestore.NewCachedKBRepo(kbRepo)
	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Hub: realtime.NewHub(), KB: kb,
		KBRepo: cached, KBInvalidator: cached,
		OrgID: org.ID, Log: log,
	})
	ts := httptest.NewServer(srv.Router())
	jar, _ := cookiejar.New(nil)
	h := &promptHarness{t: t, srv: ts, client: &http.Client{Jar: jar}}
	// st/db are closed by dbtest's own t.Cleanup registrations.
	t.Cleanup(func() { ts.Close() })
	h.login()
	return h
}

func (h *promptHarness) login() {
	body, _ := json.Marshal(map[string]string{"email": adminEmail, "password": adminPass})
	resp, err := h.client.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", strings.NewReader(string(body)))
	if err != nil || resp.StatusCode != http.StatusOK {
		h.t.Fatalf("login: %v status=%v", err, statusOf(resp))
	}
}

func (h *promptHarness) get(path string, out any) {
	resp, err := h.client.Get(h.srv.URL + path)
	if err != nil {
		h.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	decodeEnvelope(h.t, resp, out)
}
