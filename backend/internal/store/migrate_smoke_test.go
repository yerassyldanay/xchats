package store_test

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/migrations"
)

// TestMigrations_FreshDatabaseNoOpsDemoData proves 0001-0008 apply cleanly on
// a database with no organization yet, and that 0008 leaves no KB rows behind
// (its RAISE NOTICE no-op path) — the exact sequence `serve` runs on a brand
// new deployment (migrations before the identity seed creates the org).
func TestMigrations_FreshDatabaseNoOpsDemoData(t *testing.T) {
	st, closeFn := newTestStoreForSimulator(t) // resets schema + runs 0001-0008, no org seeded
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
	st, closeFn := newTestStoreForSimulator(t)
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
		VALUES ($1, $2, 'ru', 'Custom', '1 ₸', 'active', true)`, org.ID, customRef); err != nil {
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
