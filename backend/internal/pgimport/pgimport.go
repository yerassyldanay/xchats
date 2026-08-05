package pgimport

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// bootstrapOrgID/bootstrapUserID are migrations/sqlite/0006_init_admin.up.sql's
// fixed sentinel rows — the "admin@xchat.kz" org/user a freshly migrated
// SQLite database always starts with (see that migration's own doc
// comment: "no seed command, only migration 0006 for bootstrap"). They
// exist only to make a brand-new database immediately usable; once real
// data is being imported, the placeholder no longer belongs, so Import
// removes it first — see Import's own doc comment.
const (
	bootstrapOrgID  = "00000000-0000-0000-0000-000000000001"
	bootstrapUserID = "00000000-0000-0000-0000-000000000002"
)

// TableResult reports one table's imported row count.
type TableResult struct {
	Table string
	Rows  int
}

// Result is Import's whole-run summary.
type Result struct {
	Tables []TableResult
}

// TotalRows sums every table's row count.
func (r Result) TotalRows() int {
	n := 0
	for _, t := range r.Tables {
		n += t.Rows
	}
	return n
}

// Import reads every row of every base table out of the Postgres database
// at pgConnString and writes it into dest, preserving every primary key
// exactly. dest must already be migrated (internal/dbx.RunMigrations
// against migrations/sqlite) — Import only writes rows, it does not
// create schema.
//
// Every table is read from Postgres inside ONE repeatable-read, read-only
// transaction, so the whole multi-table read sees one consistent
// cross-table snapshot even if the source database is still being written
// to during the (potentially multi-second, many-table) import — a chat
// row read from wa_chats can never end up referencing a message that
// "did not exist yet" from wa_messages's own read, or vice versa, the way
// 31 independent unsynchronized SELECTs against a live database could.
// Running the import against a already-quiesced/read-only source (the
// normal cutover runbook: stop the app, then import) makes this moot, but
// costs nothing to get right regardless.
//
// The whole import — every table — runs inside ONE SQLite transaction
// with PRAGMA defer_foreign_keys ON (verified empirically: SQLite permits
// a temporarily-violated foreign key within a transaction as long as it
// is satisfied by COMMIT), which makes table import order irrelevant:
// there is no need to topologically sort 31 tables by their foreign keys,
// and no risk of a self-referencing or circular one having no valid
// order at all. It also makes the import atomic — dest either ends up
// with the complete dataset or (on any error, including a genuinely
// unsatisfied foreign key) is rolled back to exactly the freshly
// migrated, bootstrap-admin-only state it started at, never a partial
// import silently mistaken for a complete one.
//
// Import first deletes the migration 0006 bootstrap organization/user/
// membership row (see bootstrapOrgID/bootstrapUserID) — real imported
// data should never ship alongside the synthetic "admin@xchat.kz"
// placeholder — then proceeds unconditionally; a Postgres row that
// happens to share one of those sentinel ids (vanishingly unlikely: they
// are all-zero UUIDs no real id-generation path produces) simply takes
// its place.
func Import(ctx context.Context, pgConnString string, dest *dbx.DB) (Result, error) {
	tables, err := loadContract()
	if err != nil {
		return Result{}, err
	}

	pgConn, err := pgx.Connect(ctx, pgConnString)
	if err != nil {
		return Result{}, fmt.Errorf("pgimport: connect to postgres: %w", err)
	}
	defer pgConn.Close(ctx)

	pgTx, err := pgConn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Result{}, fmt.Errorf("pgimport: begin postgres read transaction: %w", err)
	}
	// Read-only: nothing to commit, and rolling back is the correct way to
	// end any transaction — including a successful one — that never wrote.
	defer pgTx.Rollback(ctx)

	tx, err := dest.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("pgimport: begin destination transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return Result{}, fmt.Errorf("pgimport: enable defer_foreign_keys: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM organization_users WHERE organization_id = $1 AND user_id = $2`,
		bootstrapOrgID, bootstrapUserID); err != nil {
		return Result{}, fmt.Errorf("pgimport: remove bootstrap membership: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, bootstrapUserID); err != nil {
		return Result{}, fmt.Errorf("pgimport: remove bootstrap user: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, bootstrapOrgID); err != nil {
		return Result{}, fmt.Errorf("pgimport: remove bootstrap organization: %w", err)
	}

	var result Result
	for _, t := range tables {
		n, err := importTable(ctx, pgTx, tx, t)
		if err != nil {
			return Result{}, fmt.Errorf("pgimport: table %s: %w", t.Name, err)
		}
		result.Tables = append(result.Tables, TableResult{Table: t.Name, Rows: n})
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("pgimport: commit: %w", err)
	}
	return result, nil
}

// importTable streams t's rows from pgTx and inserts each into tx,
// returning the row count.
func importTable(ctx context.Context, pgTx pgx.Tx, tx *dbx.Tx, t table) (int, error) {
	selectSQL := buildSelect(t)
	insertSQL := buildInsert(t)

	rows, err := pgTx.Query(ctx, selectSQL)
	if err != nil {
		return 0, fmt.Errorf("select from postgres (%s): %w", selectSQL, err)
	}
	defer rows.Close()

	args := make([]any, len(t.Columns))
	n := 0
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return 0, fmt.Errorf("read row %d: %w", n, err)
		}
		for i, col := range t.Columns {
			bound, err := bindValue(col.Kind, vals[i])
			if err != nil {
				return 0, fmt.Errorf("row %d column %s: %w", n, col.Name, err)
			}
			args[i] = bound
		}
		if _, err := tx.Exec(ctx, insertSQL, args...); err != nil {
			return 0, fmt.Errorf("insert row %d: %w", n, err)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate rows: %w", err)
	}
	return n, nil
}

// buildSelect returns "SELECT <col1-expr>, <col2-expr>, ... FROM
// xchats.<table> ORDER BY <primary key>" — the schema's own xchats prefix
// (see internal/dbx's per-package-of-callers translation table: every
// OTHER ported package strips this prefix because SQLite has no schema
// concept, but pgimport's SOURCE side is still the real Postgres database,
// where the prefix is real and required). ORDER BY the primary key makes
// row order deterministic across repeated runs (handy for diffing import
// output during testing) — it is not required for correctness, since
// every row carries its own id regardless of insert order.
func buildSelect(t table) string {
	exprs := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		exprs[i] = selectExpr(col)
	}
	q := "SELECT " + strings.Join(exprs, ", ") + " FROM xchats." + t.Name
	if len(t.PrimaryKey) > 0 {
		q += " ORDER BY " + strings.Join(t.PrimaryKey, ", ")
	}
	return q
}

// buildInsert returns "INSERT INTO <table> (col1,col2,...) VALUES
// ($1,$2,...)" against the SQLite destination — no schema prefix (SQLite
// has none).
func buildInsert(t table) string {
	names := make([]string, len(t.Columns))
	placeholders := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		names[i] = col.Name
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return "INSERT INTO " + t.Name + " (" + strings.Join(names, ",") + ") VALUES (" + strings.Join(placeholders, ",") + ")"
}
