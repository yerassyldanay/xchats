package dbops_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbops"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// BenchmarkBackup gives an operator a concrete "how long does a backup
// take" answer at a few database sizes — Backup holds the database's one
// physical connection for VACUUM INTO's whole duration (see backup.go's
// package doc), so this is also the window of increased request latency a
// live backup costs the rest of the app.
func BenchmarkBackup(b *testing.B) {
	for _, rows := range []int{0, 1_000, 10_000} {
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			ctx := context.Background()
			path := filepath.Join(b.TempDir(), "live.db")
			db, err := dbx.Open(ctx, path)
			if err != nil {
				b.Fatalf("dbx.Open: %v", err)
			}
			defer db.Close()
			if _, err := db.Exec(ctx, `CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
				b.Fatalf("create table: %v", err)
			}
			for i := 0; i < rows; i++ {
				if _, err := db.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, fmt.Sprintf("widget-%d", i)); err != nil {
					b.Fatalf("seed row %d: %v", i, err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dest := filepath.Join(b.TempDir(), fmt.Sprintf("backup-%d.db", i))
				if err := dbops.Backup(ctx, db, dest); err != nil {
					b.Fatalf("Backup: %v", err)
				}
			}
		})
	}
}

// BenchmarkIntegrityCheck covers the same size range for the read-only
// consistency check — cheap enough to run on a schedule without the
// downtime-adjacent cost Backup has.
func BenchmarkIntegrityCheck(b *testing.B) {
	for _, rows := range []int{0, 1_000, 10_000} {
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			ctx := context.Background()
			path := filepath.Join(b.TempDir(), "live.db")
			db, err := dbx.Open(ctx, path)
			if err != nil {
				b.Fatalf("dbx.Open: %v", err)
			}
			defer db.Close()
			if _, err := db.Exec(ctx, `CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
				b.Fatalf("create table: %v", err)
			}
			for i := 0; i < rows; i++ {
				if _, err := db.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, fmt.Sprintf("widget-%d", i)); err != nil {
					b.Fatalf("seed row %d: %v", i, err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := dbops.IntegrityCheck(ctx, db); err != nil {
					b.Fatalf("IntegrityCheck: %v", err)
				}
			}
		})
	}
}
