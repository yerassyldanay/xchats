package kbstore_test

import (
	"context"
	"testing"
)

func TestSeedDemoKB_InsertsFullDataset(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	inserted, err := kb.SeedDemoKB(ctx, orgID)
	if err != nil {
		t.Fatalf("SeedDemoKB: %v", err)
	}
	if !inserted {
		t.Fatalf("SeedDemoKB: want inserted=true on an empty org")
	}

	count := func(table string) int {
		t.Helper()
		var n int
		if err := db.QueryRow(ctx,
			"SELECT count(*) FROM "+table+" WHERE organization_id = $1", orgID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}

	cases := []struct {
		table string
		want  int
	}{
		{"ai_assistants", 1},
		{"ai_contacts", 1},
		{"ai_policies", 1},
		{"ai_topics", 3},
		{"ai_products", 5},
		{"ai_tariffs", 2},
		{"ai_delivery_zones", 3},
	}
	for _, c := range cases {
		if got := count(c.table); got != c.want {
			t.Errorf("%s count = %d, want %d", c.table, got, c.want)
		}
	}

	var inStock int
	if err := db.QueryRow(ctx,
		"SELECT count(*) FROM ai_products WHERE organization_id = $1 AND in_stock", orgID).Scan(&inStock); err != nil {
		t.Fatalf("count in-stock products: %v", err)
	}
	if inStock != 3 {
		t.Errorf("in-stock products = %d, want 3 (3 in stock, 2 not, per the fixed dataset)", inStock)
	}

	var baikonurAvailable bool
	if err := db.QueryRow(ctx,
		"SELECT delivery_available FROM ai_delivery_zones WHERE organization_id = $1 AND ref = 'demo_baikonur'",
		orgID).Scan(&baikonurAvailable); err != nil {
		t.Fatalf("read demo_baikonur zone: %v", err)
	}
	if baikonurAvailable {
		t.Errorf("demo_baikonur delivery_available = true, want false")
	}
}

// A repeat call, and a call against an org that already has REAL (non-demo)
// content, must both be safe no-ops — SeedDemoKB is reachable only via an
// explicit CLI command an operator can run more than once, or against the
// wrong org by accident, and it must never clobber or duplicate data either way.
func TestSeedDemoKB_NoopWhenOrgAlreadyHasContent(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	if inserted, err := kb.SeedDemoKB(ctx, orgID); err != nil || !inserted {
		t.Fatalf("first SeedDemoKB: inserted=%v err=%v", inserted, err)
	}

	inserted, err := kb.SeedDemoKB(ctx, orgID)
	if err != nil {
		t.Fatalf("second SeedDemoKB: %v", err)
	}
	if inserted {
		t.Fatalf("second SeedDemoKB: want inserted=false (org already has topics), got true")
	}

	var topics int
	if err := db.QueryRow(ctx,
		"SELECT count(*) FROM ai_topics WHERE organization_id = $1", orgID).Scan(&topics); err != nil {
		t.Fatalf("count topics: %v", err)
	}
	if topics != 3 {
		t.Errorf("topics after second call = %d, want 3 (no duplication)", topics)
	}
}

func TestSeedDemoKB_NoopWhenOrgHasPreexistingRealTopic(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx,
		`INSERT INTO ai_topics (organization_id, slug, title, body_md) VALUES ($1, 'real_topic', 'Real', 'Real content.')`,
		orgID); err != nil {
		t.Fatalf("seed a real topic: %v", err)
	}

	inserted, err := kb.SeedDemoKB(ctx, orgID)
	if err != nil {
		t.Fatalf("SeedDemoKB: %v", err)
	}
	if inserted {
		t.Fatalf("SeedDemoKB: want inserted=false against an org with existing (non-demo) content")
	}

	var products int
	if err := db.QueryRow(ctx,
		"SELECT count(*) FROM ai_products WHERE organization_id = $1", orgID).Scan(&products); err != nil {
		t.Fatalf("count products: %v", err)
	}
	if products != 0 {
		t.Errorf("products = %d, want 0 — demo data must never be inserted alongside real seller content", products)
	}
}
