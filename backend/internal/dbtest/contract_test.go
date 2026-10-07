package dbtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// The shared contract checks both dialects against the same application schema.
type contractColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
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
	path := filepath.Join(moduleRoot(t), "migrations", "schema_contract.json")
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

// TestSchemaContract checks every application table, view, column (its declared
// type, nullability and whether it has a default), primary key, foreign
// key/delete action, and index shape on the selected database engine. The types
// are the portable ones, so SQLite's declared type and PostgreSQL's data_type
// must read the same. CHECK constraints are covered by checks_test.go.
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
			t.Errorf("table %q from the contract is missing from the migrated schema", name)
		}
	}
	for name := range gotTables {
		if !wantTables[name] {
			t.Errorf("schema has table %q that is not in the contract — renamed or added without updating schema_contract.json?", name)
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
				t.Fatalf("view %q from the contract is missing from the migrated schema", name)
			}
			gotCols := sqliteColumnNameSet(t, db, ctx, name)
			for _, c := range want.Columns {
				if !gotCols[c] {
					t.Errorf("view %s: contract column %q is missing", name, c)
				}
			}
			for c := range gotCols {
				if !containsStr(want.Columns, c) {
					t.Errorf("view %s: schema has column %q that is not in the contract", name, c)
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
		if gc.typ != wc.Type {
			t.Errorf("column %q: type = %s, want %s", name, gc.typ, wc.Type)
		}
		if gc.hasDefault != wc.HasDefault {
			t.Errorf("column %q: has a default = %v, want %v", name, gc.hasDefault, wc.HasDefault)
		}
	}
	for name := range cols {
		if _, ok := wantCols[name]; !ok {
			t.Errorf("schema has column %q that is not in the contract", name)
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
			t.Errorf("schema has a foreign key %v -> %s(%v) ON DELETE %s that is not in the contract",
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
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'`
	if db.Dialect() == dbx.Postgres {
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema=current_schema() AND table_type='BASE TABLE' AND table_name != 'schema_migrations'`
	}
	rows, err := db.Query(ctx, query)
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
	query := `SELECT name FROM sqlite_master WHERE type='view'`
	if db.Dialect() == dbx.Postgres {
		query = `SELECT table_name FROM information_schema.views WHERE table_schema=current_schema()`
	}
	rows, err := db.Query(ctx, query)
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
	nullable   bool
	typ        string // declared type, lower case; a view's computed columns have none
	hasDefault bool
}

// sqliteColumns reads PRAGMA table_info(table). It works for views too
// (SQLite reports a view's output columns the same way).
func sqliteColumns(t *testing.T, db *dbx.DB, ctx context.Context, table string) map[string]sqliteColumn {
	t.Helper()
	query := `SELECT name, "notnull", lower(type), CASE WHEN dflt_value IS NULL THEN 0 ELSE 1 END FROM pragma_table_info($1)`
	if db.Dialect() == dbx.Postgres {
		query = `SELECT column_name, CASE WHEN is_nullable='NO' THEN 1 ELSE 0 END, data_type, CASE WHEN column_default IS NULL THEN 0 ELSE 1 END FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1`
	}
	rows, err := db.Query(ctx, query, table)
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()
	out := map[string]sqliteColumn{}
	for rows.Next() {
		var name, typ string
		var notNull, hasDefault int
		if err := rows.Scan(&name, &notNull, &typ, &hasDefault); err != nil {
			t.Fatal(err)
		}
		out[name] = sqliteColumn{nullable: notNull == 0, typ: typ, hasDefault: hasDefault == 1}
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
	query := `SELECT name FROM pragma_table_info($1) WHERE pk > 0 ORDER BY pk`
	if db.Dialect() == dbx.Postgres {
		query = `SELECT a.attname FROM pg_index i
 JOIN pg_class t ON t.oid=i.indrelid JOIN pg_namespace n ON n.oid=t.relnamespace
 CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum, pos)
 JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.attnum
 WHERE n.nspname=current_schema() AND t.relname=$1 AND i.indisprimary ORDER BY k.pos`
	}
	rows, err := db.Query(ctx, query, table)
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
	if db.Dialect() == dbx.Postgres {
		return postgresForeignKeys(t, db, ctx, table)
	}
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
	if db.Dialect() == dbx.Postgres {
		return postgresIndexes(t, db, ctx, table)
	}
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

func postgresForeignKeys(t *testing.T, db *dbx.DB, ctx context.Context, table string) []sqliteFK {
	t.Helper()
	rows, err := db.Query(ctx, `SELECT
 to_json(ARRAY(SELECT a.attname FROM unnest(c.conkey) WITH ORDINALITY k(num,pos) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.num ORDER BY pos)),
 r.relname,
 to_json(ARRAY(SELECT a.attname FROM unnest(c.confkey) WITH ORDINALITY k(num,pos) JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.num ORDER BY pos)),
 CASE c.confdeltype WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN 'SET NULL' WHEN 'r' THEN 'RESTRICT' WHEN 'd' THEN 'SET DEFAULT' ELSE 'NO ACTION' END
 FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid
 JOIN pg_namespace n ON n.oid=t.relnamespace JOIN pg_class r ON r.oid=c.confrelid
 WHERE c.contype='f' AND n.nspname=current_schema() AND t.relname=$1`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []sqliteFK
	for rows.Next() {
		var fk sqliteFK
		if err := rows.Scan((*dbx.StringArray)(&fk.columns), &fk.refTable, (*dbx.StringArray)(&fk.refColumns), &fk.onDelete); err != nil {
			t.Fatal(err)
		}
		out = append(out, fk)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func postgresIndexes(t *testing.T, db *dbx.DB, ctx context.Context, table string) []sqliteIdx {
	t.Helper()
	rows, err := db.Query(ctx, `SELECT
 to_json(ARRAY(SELECT a.attname FROM unnest(i.indkey) WITH ORDINALITY k(num,pos) JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.num ORDER BY pos)),
 i.indisunique, i.indpred IS NOT NULL
 FROM pg_index i JOIN pg_class t ON t.oid=i.indrelid JOIN pg_namespace n ON n.oid=t.relnamespace
 WHERE n.nspname=current_schema() AND t.relname=$1 AND NOT i.indisprimary`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []sqliteIdx
	for rows.Next() {
		var idx sqliteIdx
		if err := rows.Scan((*dbx.StringArray)(&idx.columns), &idx.unique, &idx.partial); err != nil {
			t.Fatal(err)
		}
		out = append(out, idx)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
