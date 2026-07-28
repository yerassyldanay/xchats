package main

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

// newKBLoadTestStore resets the schema, migrates, and returns a config
// pointing at the same DSN plus the store this test uses for verification —
// runKBLoad itself opens its own separate pool (mirroring how a real CLI
// invocation would run against an already-migrated database).
func newKBLoadTestStore(t *testing.T) (*config.Config, *store.Store) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping kb-load DB test")
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
	t.Cleanup(st.Close)
	return &config.Config{OrgName: "kb-load-test-org", DatabaseURL: dsn}, st
}

func kbCounts(t *testing.T, st *store.Store, orgID any) (assistants, contacts, policies, topics, products, tariffs, zones int) {
	t.Helper()
	ctx := context.Background()
	must := func(dst *int, table string) {
		if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.`+table+` WHERE organization_id=$1`, orgID).Scan(dst); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
	}
	must(&assistants, "ai_assistants")
	must(&contacts, "ai_contacts")
	must(&policies, "ai_policies")
	must(&topics, "ai_topics")
	must(&products, "ai_products")
	must(&tariffs, "ai_tariffs")
	must(&zones, "ai_delivery_zones")
	return
}

// TestKBLoad_RoundTrip drives `xchats kb-load` exactly as the runbook does:
// load testdata/demo_kb.json → every row exists (content gates and the zones
// invariant all apply, since this runs through the SAME kbstore live-write
// functions the /kb/* HTTP editor uses) → loading again is idempotent (no
// duplicates) → -remove takes the org's KB fully back to blank, proving the
// demo dataset is completely removable (nothing hand-authored is left behind
// for a real operator's KB to collide with).
func TestKBLoad_RoundTrip(t *testing.T) {
	cfg, st := newKBLoadTestStore(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := context.Background()

	runKBLoad(cfg, log, []string{"-file", "../../testdata/demo_kb.json"})

	org, err := st.SeedOrganization(ctx, cfg.OrgName)
	if err != nil {
		t.Fatalf("resolve org: %v", err)
	}

	assistants, contacts, policies, topics, products, tariffs, zones := kbCounts(t, st, org.ID)
	if assistants != 1 || contacts != 1 || policies != 1 || topics != 3 || products != 5 || tariffs != 2 || zones != 3 {
		t.Fatalf("after load: assistants=%d contacts=%d policies=%d topics=%d products=%d tariffs=%d zones=%d",
			assistants, contacts, policies, topics, products, tariffs, zones)
	}
	var kettleInStock bool
	if err := st.Pool().QueryRow(ctx, `SELECT in_stock FROM xchats.ai_products WHERE organization_id=$1 AND ref='demo_kettle'`, org.ID).Scan(&kettleInStock); err != nil {
		t.Fatalf("read demo_kettle: %v", err)
	}
	if kettleInStock {
		t.Fatal("demo_kettle should load with in_stock=false per testdata/demo_kb.json")
	}

	// idempotent: loading the same file again must not duplicate anything.
	runKBLoad(cfg, log, []string{"-file", "../../testdata/demo_kb.json"})
	assistants2, contacts2, policies2, topics2, products2, tariffs2, zones2 := kbCounts(t, st, org.ID)
	if assistants2 != assistants || contacts2 != contacts || policies2 != policies || topics2 != topics ||
		products2 != products || tariffs2 != tariffs || zones2 != zones {
		t.Fatalf("re-load changed row counts: got (%d,%d,%d,%d,%d,%d,%d), want (%d,%d,%d,%d,%d,%d,%d)",
			assistants2, contacts2, policies2, topics2, products2, tariffs2, zones2,
			assistants, contacts, policies, topics, products, tariffs, zones)
	}

	// -remove: the KB goes fully blank again.
	runKBLoad(cfg, log, []string{"-file", "../../testdata/demo_kb.json", "-remove"})
	assistants3, contacts3, policies3, topics3, products3, tariffs3, zones3 := kbCounts(t, st, org.ID)
	if assistants3 != 0 || contacts3 != 0 || policies3 != 0 || topics3 != 0 || products3 != 0 || tariffs3 != 0 || zones3 != 0 {
		t.Fatalf("after -remove: assistants=%d contacts=%d policies=%d topics=%d products=%d tariffs=%d zones=%d, want all 0",
			assistants3, contacts3, policies3, topics3, products3, tariffs3, zones3)
	}
}
