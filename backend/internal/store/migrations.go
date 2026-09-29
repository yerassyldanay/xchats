package store

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

// Migrate applies pending migrations or explicitly replays a full identifier
// (or "all") in development. Unlike New it can repair an intentional checksum
// change without first attempting ordinary startup migration checks.
func Migrate(ctx context.Context, target, force string) error {
	db, err := dbx.Open(ctx, target)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	return dbx.RunMigrationsWithOptions(ctx, db, migrations.ForDialect(string(db.Dialect())), dbx.MigrationOptions{Force: force})
}
