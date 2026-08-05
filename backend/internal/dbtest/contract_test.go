package dbtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// The types below mirror backend/migrations/sqlite/schema_contract.json's
// shape exactly (produced by the Phase 0 pg_catalog introspection tool —
// see that file's own header). This is the Phase 0 checkpoint's contract:
// every relation captured from a fully-migrated Postgres database, which a
// freshly-migrated SQLite database must still satisfy column-for-column.

type contractColumn struct {
	Name       string `json:"name"`
	Nullable   bool   `json:"nullable"`
	HasDefault bool   `json:"has_default"`
}

type contractIndex struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Where   string   `json:"where"`
}

type contractFK struct {
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`
	OnDelete   string   `json:"on_delete"`
}

type contractCheck struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

type contractTable struct {
	Columns     []contractColumn `json:"columns"`
	PrimaryKey  []string         `json:"primary_key"`
	Indexes     []contractIndex  `json:"indexes"`
	Checks      []contractCheck  `json:"checks"`
	ForeignKeys []contractFK     `json:"foreign_keys"`
}

type contractView struct {
	Columns []string `json:"columns"`
}

type schemaContract struct {
	Tables map[string]contractTable `json:"tables"`
	Views  map[string]contractView  `json:"views"`
}

func loadContract(t *testing.T) schemaContract {
	t.Helper()
	path := filepath.Join(moduleRoot(t), "migrations", "sqlite", "schema_contract.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema contract %s: %v", path, err)
	}
	var c schemaContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse schema contract: %v", err)
	}
	if len(c.Tables) == 0 {
		t.Fatal("schema contract has zero tables — is schema_contract.json empty or malformed?")
	}
	return c
}

// TestSchemaContract asserts a freshly migrated SQLite database has every
// table, view, column (with matching nullability), primary key, index
// (including partial-index predicates), and foreign key (including its ON
// DELETE action) that migrations/sqlite/schema_contract.json captured from
// the equivalent, fully-migrated Postgres database — the Phase 0 checkpoint
// this whole cutover is not allowed to silently regress. It intentionally
// does NOT compare default-value or CHECK-constraint SQL text verbatim:
// those are expected to read differently across engines (now() vs
// strftime(), = ANY(ARRAY[...]) vs IN (...), and SQLite's json_valid CHECKs
// have no Postgres equivalent at all since jsonb is natively validated
// there) — see TestEnumChecksEnforced for a behavioral check of the four
// enum-shaped CHECK constraints instead.
func TestSchemaContract(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	contract := loadContract(t)

	gotTables := sqliteTableNames(t, db, ctx)
	wantTables := make(map[string]bool, len(contract.Tables))
	for name := range contract.Tables {
		wantTables[name] = true
	}
	for name := range wantTables {
		if !gotTables[name] {
			t.Errorf("table %q from the contract is missing from the migrated SQLite schema", name)
		}
	}
	for name := range gotTables {
		if !wantTables[name] {
			t.Errorf("SQLite schema has table %q that is not in the contract — renamed or added without updating schema_contract.json?", name)
		}
	}

	for name, want := range contract.Tables {
		if !gotTables[name] {
			continue // already reported above; avoid a second, noisier failure per missing table
		}
		t.Run(name, func(t *testing.T) {
			checkTable(t, db, ctx, name, want)
		})
	}

	gotViews := sqliteViewNames(t, db, ctx)
	for name, want := range contract.Views {
		t.Run("view_"+name, func(t *testing.T) {
			if !gotViews[name] {
				t.Fatalf("view %q from the contract is missing from the migrated SQLite schema", name)
			}
			gotCols := sqliteColumnNameSet(t, db, ctx, name)
			for _, c := range want.Columns {
				if !gotCols[c] {
					t.Errorf("view %s: contract column %q is missing", name, c)
				}
			}
			for c := range gotCols {
				if !containsStr(want.Columns, c) {
					t.Errorf("view %s: SQLite has column %q that is not in the contract", name, c)
				}
			}
		})
	}
}

func checkTable(t *testing.T, db *dbx.DB, ctx context.Context, table string, want contractTable) {
	cols := sqliteColumns(t, db, ctx, table)

	wantCols := make(map[string]contractColumn, len(want.Columns))
	for _, c := range want.Columns {
		wantCols[c.Name] = c
	}
	for name, wc := range wantCols {
		gc, ok := cols[name]
		if !ok {
			t.Errorf("column %q is missing", name)
			continue
		}
		if gc.nullable != wc.Nullable {
			t.Errorf("column %q: nullable = %v, want %v", name, gc.nullable, wc.Nullable)
		}
	}
	for name := range cols {
		if _, ok := wantCols[name]; !ok {
			t.Errorf("SQLite has column %q that is not in the contract", name)
		}
	}

	gotPK := sqlitePrimaryKey(t, db, ctx, table)
	if !equalStringSlices(gotPK, want.PrimaryKey) {
		t.Errorf("primary key = %v, want %v", gotPK, want.PrimaryKey)
	}

	gotFKs := sqliteForeignKeys(t, db, ctx, table)
	for _, wfk := range want.ForeignKeys {
		if !hasMatchingFK(gotFKs, wfk) {
			t.Errorf("missing (or mismatched) foreign key %v -> %s(%v) ON DELETE %s",
				wfk.Columns, wfk.RefTable, wfk.RefColumns, wfk.OnDelete)
		}
	}
	for _, gfk := range gotFKs {
		found := false
		for _, wfk := range want.ForeignKeys {
			if equalStringSlices(gfk.columns, wfk.Columns) &&
				gfk.refTable == wfk.RefTable &&
				equalStringSlices(gfk.refColumns, wfk.RefColumns) &&
				gfk.onDelete == wfk.OnDelete {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SQLite has a foreign key %v -> %s(%v) ON DELETE %s that is not in the contract",
				gfk.columns, gfk.refTable, gfk.refColumns, gfk.onDelete)
		}
	}

	gotIdx := sqliteIndexes(t, db, ctx, table)
	for _, wi := range want.Indexes {
		if !hasMatchingIndex(gotIdx, wi) {
			t.Errorf("missing (or mismatched) index on %v (unique=%v, partial=%v)",
				wi.Columns, wi.Unique, wi.Where != "")
		}
	}
}

// --- SQLite introspection -----------------------------------------------

func sqliteTableNames(t *testing.T, db *dbx.DB, ctx context.Context) map[string]bool {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != 'xchats_schema_migrations'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out[name] = true
	}
	return out
}

func sqliteViewNames(t *testing.T, db *dbx.DB, ctx context.Context) map[string]bool {
	t.Helper()
	rows, err := db.Query(ctx, `SELECT name FROM sqlite_master WHERE type = 'view'`)
	if err != nil {
		t.Fatalf("list views: %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out[name] = true
	}
	return out
}

func sqliteColumnNameSet(t *testing.T, db *dbx.DB, ctx context.Context, table string) map[string]bool {
	t.Helper()
	cols := sqliteColumns(t, db, ctx, table)
	out := make(map[string]bool, len(cols))
	for name := range cols {
		out[name] = true
	}
	return out
}

type sqliteColumn struct {
	nullable bool
}

// sqliteColumns reads PRAGMA table_info(table). It works for views too
// (SQLite reports a view's output columns the same way).
func sqliteColumns(t *testing.T, db *dbx.DB, ctx context.Context, table string) map[string]sqliteColumn {
	t.Helper()
	rows, err := db.Query(ctx, `SELECT name, "notnull" FROM pragma_table_info($1)`, table)
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()
	out := map[string]sqliteColumn{}
	for rows.Next() {
		var name string
		var notNull int
		if err := rows.Scan(&name, &notNull); err != nil {
			t.Fatal(err)
		}
		out[name] = sqliteColumn{nullable: notNull == 0}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("table_info(%s) returned zero columns — table missing?", table)
	}
	return out
}

// sqlitePrimaryKey returns the table's primary-key columns in declaration
// order (pragma_table_info's pk field is the column's 1-based ordinal
// WITHIN the primary key, 0 if it is not part of one).
func sqlitePrimaryKey(t *testing.T, db *dbx.DB, ctx context.Context, table string) []string {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT name FROM pragma_table_info($1)
		WHERE pk > 0
		ORDER BY pk`, table)
	if err != nil {
		t.Fatalf("table_info(%s) pk: %v", table, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	return out
}

type sqliteFK struct {
	columns    []string
	refTable   string
	refColumns []string
	onDelete   string
}

// sqliteForeignKeys groups pragma_foreign_key_list(table) rows by id — a
// single multi-column FK spans several rows sharing one id, ordered by seq.
func sqliteForeignKeys(t *testing.T, db *dbx.DB, ctx context.Context, table string) []sqliteFK {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT id, seq, "table", "from", "to", on_delete
		FROM pragma_foreign_key_list($1)
		ORDER BY id, seq`, table)
	if err != nil {
		t.Fatalf("foreign_key_list(%s): %v", table, err)
	}
	defer rows.Close()

	type group struct {
		refTable, onDelete  string
		columns, refColumns []string
	}
	groups := map[int]*group{}
	var order []int
	for rows.Next() {
		var id, seq int
		var refTable, from, to, onDelete string
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onDelete); err != nil {
			t.Fatal(err)
		}
		g, ok := groups[id]
		if !ok {
			g = &group{refTable: refTable, onDelete: onDelete}
			groups[id] = g
			order = append(order, id)
		}
		g.columns = append(g.columns, from)
		g.refColumns = append(g.refColumns, to)
	}
	out := make([]sqliteFK, 0, len(order))
	for _, id := range order {
		g := groups[id]
		out = append(out, sqliteFK{columns: g.columns, refTable: g.refTable, refColumns: g.refColumns, onDelete: g.onDelete})
	}
	return out
}

type sqliteIdx struct {
	columns []string
	unique  bool
	partial bool
}

// sqliteIndexes reads pragma_index_list(table), excluding the PRIMARY KEY's
// own implicit index (origin='pk' — the contract tracks that separately as
// primary_key), then pragma_index_info for each index's column list.
func sqliteIndexes(t *testing.T, db *dbx.DB, ctx context.Context, table string) []sqliteIdx {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT name, "unique", partial FROM pragma_index_list($1)
		WHERE origin != 'pk'`, table)
	if err != nil {
		t.Fatalf("index_list(%s): %v", table, err)
	}
	defer rows.Close()

	type idxMeta struct {
		name    string
		unique  bool
		partial bool
	}
	var metas []idxMeta
	for rows.Next() {
		var name string
		var unique, partial int
		if err := rows.Scan(&name, &unique, &partial); err != nil {
			t.Fatal(err)
		}
		metas = append(metas, idxMeta{name: name, unique: unique != 0, partial: partial != 0})
	}
	rows.Close()

	out := make([]sqliteIdx, 0, len(metas))
	for _, m := range metas {
		colRows, err := db.Query(ctx, `SELECT name FROM pragma_index_info($1) ORDER BY seqno`, m.name)
		if err != nil {
			t.Fatalf("index_info(%s): %v", m.name, err)
		}
		var cols []string
		for colRows.Next() {
			var c string
			if err := colRows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			cols = append(cols, c)
		}
		colRows.Close()
		out = append(out, sqliteIdx{columns: cols, unique: m.unique, partial: m.partial})
	}
	return out
}

// --- comparison helpers ---------------------------------------------------

func hasMatchingFK(fks []sqliteFK, want contractFK) bool {
	for _, fk := range fks {
		if equalStringSlices(fk.columns, want.Columns) &&
			fk.refTable == want.RefTable &&
			equalStringSlices(fk.refColumns, want.RefColumns) &&
			fk.onDelete == want.OnDelete {
			return true
		}
	}
	return false
}
func hasMatchingIndex(idxs []sqliteIdx, want contractIndex) bool {
	for _, idx := range idxs {
		if equalStringSlices(idx.columns, want.Columns) &&
			idx.unique == want.Unique &&
			idx.partial == (want.Where != "") {
			return true
		}
	}
	return false
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
