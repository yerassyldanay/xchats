package store

import (
	"context"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies every *.up.sql in the embedded FS in lexical order,
// recording applied versions so re-runs are no-ops. The DDL itself is also
// IF NOT EXISTS, so this is safe even on a partially-migrated database.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, mfs fs.FS) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.xchats_schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(mfs, ".")
	if err != nil {
		return err
	}
	var versions []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			versions = append(versions, e.Name())
		}
	}
	sort.Strings(versions)

	for _, name := range versions {
		version := strings.TrimSuffix(name, ".up.sql")
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM public.xchats_schema_migrations WHERE version=$1)`, version).
			Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		body, err := fs.ReadFile(mfs, name)
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO public.xchats_schema_migrations(version) VALUES($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
