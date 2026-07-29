package store_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

// newTestStoreThrough resets the schema and applies migrations up to and
// including upTo (a version stem like "0008_kb_response_demo_data") — no
// further. TestMigrations_0008IsIdempotentAndNeverOverwrites and its 0009
// sibling below manually re-apply a historical migration's raw SQL body to
// prove IT is idempotent; that body only makes sense against the schema shape
// that existed right after it first ran, not after later migrations (e.g.
// 0012, which drops ai_products/ai_tariffs.lang) changed the columns it
// references.
func newTestStoreThrough(t *testing.T, upTo string) (*store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping store DB test")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `DROP SCHEMA IF EXISTS xchats CASCADE; DROP TABLE IF EXISTS public.xchats_schema_migrations`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `CREATE TABLE IF NOT EXISTS public.xchats_schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatalf("create migrations table: %v", err)
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".up.sql")
		if version > upTo {
			break
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := st.Pool().Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if _, err := st.Pool().Exec(ctx, `INSERT INTO public.xchats_schema_migrations (version) VALUES ($1)`, version); err != nil {
			t.Fatalf("record %s: %v", name, err)
		}
	}
	return st, st.Close
}

// TestMigrations_FreshDatabaseNoOpsDemoData proves 0001-0008 apply cleanly on
// a database with no organization yet, and that 0008 leaves no KB rows behind
// (its RAISE NOTICE no-op path) — the exact sequence `serve` runs on a brand
// new deployment (migrations before the identity seed creates the org).
func TestMigrations_FreshDatabaseNoOpsDemoData(t *testing.T) {
	st, closeFn := newTestStoreForSimulator(t) // resets schema + runs every migration through the latest, no org seeded
	defer closeFn()
	ctx := context.Background()

	var orgs, assistants, accounts int
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.organizations`).Scan(&orgs); err != nil {
		t.Fatalf("count organizations: %v", err)
	}
	if orgs != 0 {
		t.Fatalf("want a truly fresh database (0 organizations), got %d", orgs)
	}
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_assistants`).Scan(&assistants); err != nil {
		t.Fatalf("count ai_assistants: %v", err)
	}
	if assistants != 0 {
		t.Fatalf("0008 must no-op on a fresh database (no org to attach demo data to), got %d ai_assistants rows", assistants)
	}
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.wa_accounts`).Scan(&accounts); err != nil {
		t.Fatalf("count wa_accounts: %v", err)
	}
	if accounts != 0 {
		t.Fatalf("want 0 accounts on a fresh database, got %d", accounts)
	}
}

// TestMigrations_WaAccountsChannelDefaultsToWhatsApp proves migration 0007's
// wa_accounts.channel column exists with the correct default for an account
// created without specifying it (every pre-0007 INSERT path).
func TestMigrations_WaAccountsChannelDefaultsToWhatsApp(t *testing.T) {
	st, closeFn := newTestStoreForSimulator(t)
	defer closeFn()
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "channel-default-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	var channel string
	err = st.Pool().QueryRow(ctx, `
		INSERT INTO xchats.wa_accounts (id, organization_id, display_name, owner_jid, connection_state)
		VALUES (uuid_generate_v4(), $1, 'x', 'unspecified-channel-jid@s.whatsapp.net', 'connected')
		RETURNING channel`, org.ID).Scan(&channel)
	if err != nil {
		t.Fatalf("insert account without channel: %v", err)
	}
	if channel != "whatsapp" {
		t.Fatalf("channel default = %q, want whatsapp", channel)
	}
}

// TestMigrations_0008IsIdempotentAndNeverOverwrites replays the exact dev
// workflow the 0008 migration's own header documents: boot once (org exists),
// then re-apply the file's body by hand. Demo rows must appear with their
// fixed UUIDs, a second application must be a complete no-op (same rows, same
// count), and a pre-existing (non-demo) KB row must never be touched.
func TestMigrations_0008IsIdempotentAndNeverOverwrites(t *testing.T) {
	st, closeFn := newTestStoreThrough(t, "0008_kb_response_demo_data")
	defer closeFn()
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "demo-data-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	body, err := migrations.FS.ReadFile("0008_kb_response_demo_data.up.sql")
	if err != nil {
		t.Fatalf("read 0008 up.sql: %v", err)
	}

	applyOnce := func() {
		t.Helper()
		if _, err := st.Pool().Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply 0008 body: %v", err)
		}
	}
	applyOnce()

	const demoAssistantID = "00000000-0000-4000-9000-000000000d01"
	var persona string
	if err := st.Pool().QueryRow(ctx, `SELECT persona FROM xchats.ai_assistants WHERE id = $1`, demoAssistantID).Scan(&persona); err != nil {
		t.Fatalf("demo assistant row missing: %v", err)
	}
	if persona == "" {
		t.Fatal("demo assistant persona is blank")
	}

	var productCount, zoneCount, topicCount int
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_products WHERE organization_id = $1 AND ref LIKE 'demo_%'`, org.ID).Scan(&productCount)
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_delivery_zones WHERE organization_id = $1`, org.ID).Scan(&zoneCount)
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_topics WHERE organization_id = $1 AND slug LIKE 'demo_%'`, org.ID).Scan(&topicCount)
	if productCount != 5 {
		t.Fatalf("demo products = %d, want 5", productCount)
	}
	if zoneCount != 3 {
		t.Fatalf("demo delivery zones = %d, want 3", zoneCount)
	}
	if topicCount != 3 {
		t.Fatalf("demo topics = %d, want 3", topicCount)
	}

	// A pre-existing, non-demo KB row (simulating operator-curated content)
	// must survive a re-application untouched.
	const customRef = "operator-added-product"
	if _, err := st.Pool().Exec(ctx, `
		INSERT INTO xchats.ai_products (organization_id, ref, lang, name, price, status, in_stock)
		VALUES ($1, $2, 'ru', 'Custom', '1 ₸', 'active', true)`, org.ID, customRef); err != nil { // schema as of 0008 — lang/status still present at this point in history
		t.Fatalf("insert operator product: %v", err)
	}

	applyOnce() // re-apply: must be a complete no-op

	var productCount2, customCount int
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_products WHERE organization_id = $1 AND ref LIKE 'demo_%'`, org.ID).Scan(&productCount2)
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_products WHERE organization_id = $1 AND ref = $2`, org.ID, customRef).Scan(&customCount)
	if productCount2 != 5 {
		t.Fatalf("re-apply: demo products = %d, want still 5 (no duplicates)", productCount2)
	}
	if customCount != 1 {
		t.Fatalf("re-apply: operator-added product was touched, count = %d, want 1", customCount)
	}
}

// TestMigrations_0001Through0009ApplyCleanOnFreshDatabase proves the full
// current migration sequence — through 0009, this PR's own addition — still
// applies cleanly on a database with no organization yet (newTestStoreForSimulator
// already ran it) and is recorded as applied, so `serve`'s normal migrate step
// (main.go's runServe, before the identity seed creates the org) never trips
// on 0009.
func TestMigrations_0001Through0009ApplyCleanOnFreshDatabase(t *testing.T) {
	st, closeFn := newTestStoreForSimulator(t) // resets schema + runs every migration through the latest, no org seeded
	defer closeFn()
	ctx := context.Background()

	var applied bool
	if err := st.Pool().QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM public.xchats_schema_migrations WHERE version = '0009_remove_kb_response_demo_data')`,
	).Scan(&applied); err != nil {
		t.Fatalf("check migration recorded: %v", err)
	}
	if !applied {
		t.Fatal("0009_remove_kb_response_demo_data did not run as part of the normal migrate step")
	}
}

// TestMigrations_0009IsIdempotentAndRemovesDemoData is 0009's removal-side
// mirror of TestMigrations_0008IsIdempotentAndNeverOverwrites: after 0008
// inserts its demo rows, applying 0009's body deletes exactly those fixed
// UUIDs, and a second application is a harmless no-op (nothing left to delete).
func TestMigrations_0009IsIdempotentAndRemovesDemoData(t *testing.T) {
	st, closeFn := newTestStoreThrough(t, "0009_remove_kb_response_demo_data")
	defer closeFn()
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "demo-removal-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	body8, err := migrations.FS.ReadFile("0008_kb_response_demo_data.up.sql")
	if err != nil {
		t.Fatalf("read 0008 up.sql: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, string(body8)); err != nil {
		t.Fatalf("apply 0008 body: %v", err)
	}

	const demoAssistantID = "00000000-0000-4000-9000-000000000d01"
	var persona string
	if err := st.Pool().QueryRow(ctx, `SELECT persona FROM xchats.ai_assistants WHERE id = $1`, demoAssistantID).Scan(&persona); err != nil {
		t.Fatalf("demo assistant row missing after 0008: %v", err)
	}

	body9, err := migrations.FS.ReadFile("0009_remove_kb_response_demo_data.up.sql")
	if err != nil {
		t.Fatalf("read 0009 up.sql: %v", err)
	}
	applyOnce9 := func() {
		t.Helper()
		if _, err := st.Pool().Exec(ctx, string(body9)); err != nil {
			t.Fatalf("apply 0009 body: %v", err)
		}
	}
	applyOnce9()

	err = st.Pool().QueryRow(ctx, `SELECT persona FROM xchats.ai_assistants WHERE id = $1`, demoAssistantID).Scan(&persona)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("demo assistant row should be gone after 0009, err=%v persona=%q", err, persona)
	}
	var productCount, zoneCount, topicCount, tariffCount int
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_products WHERE organization_id = $1 AND ref LIKE 'demo_%'`, org.ID).Scan(&productCount)
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_delivery_zones WHERE organization_id = $1`, org.ID).Scan(&zoneCount)
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_topics WHERE organization_id = $1 AND slug LIKE 'demo_%'`, org.ID).Scan(&topicCount)
	st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.ai_tariffs WHERE organization_id = $1 AND ref LIKE 'demo_%'`, org.ID).Scan(&tariffCount)
	if productCount != 0 || zoneCount != 0 || topicCount != 0 || tariffCount != 0 {
		t.Fatalf("demo rows remain after 0009: products=%d zones=%d topics=%d tariffs=%d",
			productCount, zoneCount, topicCount, tariffCount)
	}

	applyOnce9() // re-apply: must be a harmless no-op — nothing left to delete
}
