package store

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/internal/dbops"
)

// Backup writes a consistent, compacted snapshot of the database to destPath.
// destPath must not already exist. Thin wrapper over dbops.Backup — kept here
// (rather than exposing the underlying *dbx.DB) so callers outside the
// persistence boundary, like cmd/xchats, never import internal/dbx directly.
func (s *Store) Backup(ctx context.Context, destPath string) error {
	return dbops.Backup(ctx, s.db, destPath)
}

// IntegrityCheck runs SQLite's own consistency check and returns every
// problem found, if any. A nil, nil result means the database is fully
// consistent.
func (s *Store) IntegrityCheck(ctx context.Context) ([]string, error) {
	return dbops.IntegrityCheck(ctx, s.db)
}
