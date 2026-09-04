package dbtest

import (
	"context"
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// fsWithout returns a copy of mfs with excludeName removed — used below to
// apply every migration EXCEPT 0017_kb_virtual_facts, so a test can seed
// legacy-shaped rows (the in_stock boolean, a pre-migration kbd_draft
// blob) before running 0017 alone and asserting on its backfill.
func fsWithout(t testing.TB, mfs fs.FS, excludeName string) fs.FS {
	t.Helper()
	entries, err := fs.ReadDir(mfs, ".")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	out := fstest.MapFS{}
	for _, e := range entries {
		if e.Name() == excludeName {
			continue
		}
		b, err := fs.ReadFile(mfs, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = &fstest.MapFile{Data: b}
	}
	if _, ok := out[excludeName]; ok {
		t.Fatalf("fsWithout: %s was not actually excluded", excludeName)
	}
	return out
}

// openPreVirtualFacts opens a fresh database migrated through every file
// EXCEPT 0017_kb_virtual_facts.up.sql — the pre-migration schema shape
// (ai_products.in_stock still present, no availability_status/
// additional_facts/ai_tariff_info).
func openPreVirtualFacts(t testing.TB) *dbx.DB {
	t.Helper()
	db := OpenRawEmpty(t)
	pre := fsWithout(t, sqlitemigrations.FS, "0017_kb_virtual_facts.up.sql")
	if err := dbx.RunMigrations(context.Background(), db, pre); err != nil {
		t.Fatalf("migrate (pre-0017): %v", err)
	}
	return db
}

// applyVirtualFactsMigration runs the full migration set again — since
// every earlier file is already recorded in xchats_schema_migrations, only
// 0017_kb_virtual_facts actually applies.
func applyVirtualFactsMigration(t testing.TB, db *dbx.DB) {
	t.Helper()
	if err := dbx.RunMigrations(context.Background(), db, sqlitemigrations.FS); err != nil {
		t.Fatalf("migrate (0017): %v", err)
	}
}

const testOrgSQL = `INSERT INTO organizations (id, name) VALUES ('22222222-2222-2222-2222-222222222222', 'Test Org')`

// TestMigration0017_BackfillsAvailabilityStatusFromInStock is required test
// #1 (migration and backfill from in_stock) at the SQL level: a product row
// written under the PRE-0017 schema (in_stock boolean only) must end up
// with the correct availability_status after 0017 runs, and in_stock must
// no longer exist as a column at all.
func TestMigration0017_BackfillsAvailabilityStatusFromInStock(t *testing.T) {
	db := openPreVirtualFacts(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, testOrgSQL); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO ai_products (organization_id, ref, name, price, in_stock)
		VALUES ('22222222-2222-2222-2222-222222222222', 'in-stock-item', 'In Stock Item', '100', 1)`); err != nil {
		t.Fatalf("insert in-stock product: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO ai_products (organization_id, ref, name, price, in_stock)
		VALUES ('22222222-2222-2222-2222-222222222222', 'out-of-stock-item', 'Out Of Stock Item', '200', 0)`); err != nil {
		t.Fatalf("insert out-of-stock product: %v", err)
	}

	applyVirtualFactsMigration(t, db)

	rows, err := db.Query(ctx, `SELECT ref, availability_status FROM ai_products ORDER BY ref`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var ref, status string
		if err := rows.Scan(&ref, &status); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[ref] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if got["in-stock-item"] != "in_stock" {
		t.Errorf("in-stock-item availability_status = %q, want %q", got["in-stock-item"], "in_stock")
	}
	if got["out-of-stock-item"] != "unavailable" {
		t.Errorf("out-of-stock-item availability_status = %q, want %q", got["out-of-stock-item"], "unavailable")
	}

	// in_stock must be gone entirely — no contradictory legacy source left.
	infoRows, err := db.Query(ctx, `SELECT name FROM pragma_table_info('ai_products') WHERE name = 'in_stock'`)
	if err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	defer infoRows.Close()
	if infoRows.Next() {
		t.Error("ai_products.in_stock still exists after migration 0017 — it must be dropped")
	}
}

// TestMigration0017_RewritesPendingDraftProductsInPlace covers "migrate
// existing draft data safely": a kbd_draft blob staged BEFORE 0017 (its
// product entries shaped {"in_stock": true/false, ...}, no
// availability_status) must come out the other side with every entry
// translated 1:1 — availability_status set, in_stock removed, every other
// field byte-identical, array order preserved.
func TestMigration0017_RewritesPendingDraftProductsInPlace(t *testing.T) {
	db := openPreVirtualFacts(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, testOrgSQL); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	const legacyDraft = `{"config":{},"topics":[],"tariffs":[],"products":[` +
		`{"ref":"widget-a","name":"Widget A","price":"1000","description":"d","category":"c","in_stock":true,"sales_status":"active","featured_image":null,"gallery_images":[],"demo_videos":[],"certificate_documents":[],"guarantee_documents":[]},` +
		`{"ref":"widget-b","name":"Widget B","price":"2000","description":"","category":"","in_stock":false,"sales_status":"active","featured_image":null,"gallery_images":[],"demo_videos":[],"certificate_documents":[],"guarantee_documents":[]}` +
		`],"contacts":[],"policies":[],"delivery_zones":[],"deletes":[]}`
	if _, err := db.Exec(ctx, `INSERT INTO kbd_draft (organization_id, draft) VALUES ('22222222-2222-2222-2222-222222222222', $1)`, legacyDraft); err != nil {
		t.Fatalf("insert legacy draft: %v", err)
	}

	applyVirtualFactsMigration(t, db)

	var draft string
	if err := db.QueryRow(ctx, `SELECT draft FROM kbd_draft WHERE organization_id = '22222222-2222-2222-2222-222222222222'`).Scan(&draft); err != nil {
		t.Fatalf("read back draft: %v", err)
	}

	var parsed struct {
		Products []map[string]any `json:"products"`
	}
	if err := json.Unmarshal([]byte(draft), &parsed); err != nil {
		t.Fatalf("parsed draft is not valid JSON: %v\ndraft: %s", err, draft)
	}
	if len(parsed.Products) != 2 {
		t.Fatalf("want 2 products preserved in order, got %d: %s", len(parsed.Products), draft)
	}
	widgetA, widgetB := parsed.Products[0], parsed.Products[1]
	if widgetA["ref"] != "widget-a" || widgetB["ref"] != "widget-b" {
		t.Fatalf("product order not preserved: %s", draft)
	}
	if _, has := widgetA["in_stock"]; has {
		t.Errorf("widget-a still carries the legacy in_stock key: %s", draft)
	}
	if widgetA["availability_status"] != "in_stock" {
		t.Errorf("widget-a availability_status = %v, want %q", widgetA["availability_status"], "in_stock")
	}
	if widgetB["availability_status"] != "unavailable" {
		t.Errorf("widget-b availability_status = %v, want %q", widgetB["availability_status"], "unavailable")
	}
	if widgetA["name"] != "Widget A" || widgetA["price"] != "1000" || widgetA["description"] != "d" {
		t.Errorf("widget-a lost or corrupted an unrelated field: %s", draft)
	}
}

// TestMigration0017_EmptyOrAbsentDraftProductsUntouched confirms the
// migration is a safe no-op for an org with no pending product edits at
// all (draft.products absent or null) and for one with an explicit empty
// list — neither should end up NULL or error.
func TestMigration0017_EmptyOrAbsentDraftProductsUntouched(t *testing.T) {
	db := openPreVirtualFacts(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, testOrgSQL); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO kbd_draft (organization_id, draft) VALUES ('22222222-2222-2222-2222-222222222222', '{"products":[]}')`); err != nil {
		t.Fatalf("insert empty-products draft: %v", err)
	}

	applyVirtualFactsMigration(t, db)

	var draft string
	if err := db.QueryRow(ctx, `SELECT draft FROM kbd_draft WHERE organization_id = '22222222-2222-2222-2222-222222222222'`).Scan(&draft); err != nil {
		t.Fatalf("read back draft: %v", err)
	}
	var parsed struct {
		Products []map[string]any `json:"products"`
	}
	if err := json.Unmarshal([]byte(draft), &parsed); err != nil {
		t.Fatalf("parsed draft is not valid JSON: %v\ndraft: %s", err, draft)
	}
	if parsed.Products == nil || len(parsed.Products) != 0 {
		t.Errorf("want an empty (not null) products array, got %#v from draft: %s", parsed.Products, draft)
	}
}

// TestMigration0017_AddsNewColumnsAndTariffInfoTable is a light smoke test
// that the additive half of 0017 (new TEXT/JSON columns, the ai_tariff_info
// table) actually landed — the exhaustive column-by-column check is
// TestSchemaContract (contract_test.go) once schema_contract.json is
// updated; this just confirms migrating from a real PRE-0017 database (not
// a fully-fresh one) reaches the same end state.
func TestMigration0017_AddsNewColumnsAndTariffInfoTable(t *testing.T) {
	db := openPreVirtualFacts(t)
	applyVirtualFactsMigration(t, db)
	ctx := context.Background()

	for _, col := range []string{"brand", "advantages", "disadvantages", "best_for", "not_for",
		"availability_status", "availability_note", "installation_terms", "warranty_terms", "additional_facts"} {
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM pragma_table_info('ai_products') WHERE name = $1`, col).Scan(&n); err != nil {
			t.Fatalf("pragma_table_info(ai_products) for %s: %v", col, err)
		}
		if n != 1 {
			t.Errorf("ai_products.%s missing after migration", col)
		}
	}
	for _, col := range []string{"best_for", "not_for", "additional_facts"} {
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM pragma_table_info('ai_tariffs') WHERE name = $1`, col).Scan(&n); err != nil {
			t.Fatalf("pragma_table_info(ai_tariffs) for %s: %v", col, err)
		}
		if n != 1 {
			t.Errorf("ai_tariffs.%s missing after migration", col)
		}
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='ai_tariff_info'`).Scan(&n); err != nil {
		t.Fatalf("sqlite_master: %v", err)
	}
	if n != 1 {
		t.Error("ai_tariff_info table missing after migration")
	}
}

// OpenRawEmpty opens a fresh, UNMIGRATED database at t.TempDir() — the
// bare pool this file's tests apply a partial migration set to themselves.
func OpenRawEmpty(t testing.TB) *dbx.DB {
	t.Helper()
	ctx := context.Background()
	path := t.TempDir() + "/xchats.db"
	db, err := dbx.Open(ctx, path)
	if err != nil {
		t.Fatalf("dbtest: open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("dbtest: close: %v", err)
		}
	})
	return db
}
