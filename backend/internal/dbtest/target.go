package dbtest

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// Target isolates each test in a local file or, when TEST_DATABASE_URL is set,
// a disposable PostgreSQL schema. Cleanup is registered before handle cleanup.
func Target(t testing.TB) string {
	t.Helper()
	target := os.Getenv("TEST_DATABASE_URL")
	if target == "" {
		return filepath.Join(t.TempDir(), "xchats.db")
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	ctx := context.Background()
	admin, err := dbx.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	// Install the shared extension once under the public-schema transaction
	// lock. Tests only create/drop their own schema, never the public schema.
	tx, err := admin.Begin(ctx)
	if err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public`); err != nil {
		_ = tx.Rollback(ctx)
		_ = admin.Close()
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	name := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+name); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer func() {
			if err := admin.Close(); err != nil {
				t.Error(err)
			}
		}()
		if _, err := admin.Exec(ctx, `DROP SCHEMA `+name+` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	q := u.Query()
	q.Set("search_path", name+",public")
	u.RawQuery = q.Encode()
	return u.String()
}
