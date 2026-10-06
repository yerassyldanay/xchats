package dbx

import (
	"context"
	"database/sql"
)

// Tx is a pgx-shaped wrapper over *sql.Tx: Commit/Rollback take a ctx
// (unused by database/sql, kept so ported call sites — tx.Commit(ctx),
// tx.Rollback(ctx) — don't change shape) and Query/QueryRow/Exec mirror DB's.
type Tx struct {
	tx *sql.Tx
}

// Query runs a query expected to return rows.
func (tx *Tx) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	r, err := tx.tx.QueryContext(ctx, query, bindArgs(args)...)
	if err != nil {
		return nil, err
	}
	return &Rows{r: r}, nil
}

// QueryRow runs a query expected to return at most one row.
func (tx *Tx) QueryRow(ctx context.Context, query string, args ...any) *Row {
	return &Row{r: tx.tx.QueryRowContext(ctx, query, bindArgs(args)...)}
}

// Exec runs a query that doesn't return rows.
func (tx *Tx) Exec(ctx context.Context, query string, args ...any) (CommandTag, error) {
	res, err := tx.tx.ExecContext(ctx, query, bindArgs(args)...)
	if err != nil {
		return CommandTag{}, err
	}
	return newCommandTag(res), nil
}

// Commit commits the transaction. ctx is accepted for pgx call-site
// compatibility and otherwise unused — database/sql's Tx.Commit takes none.
func (tx *Tx) Commit(ctx context.Context) error { return tx.tx.Commit() }

// Rollback aborts the transaction. Calling Rollback after Commit (or a
// second time) is a documented no-op error (sql.ErrTxDone) that callers
// conventionally discard in a defer, exactly as with pgx.Tx.
func (tx *Tx) Rollback(ctx context.Context) error { return tx.tx.Rollback() }
