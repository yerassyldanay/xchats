package dbtest

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

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
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
