package migrations

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreatePairedUTCAndRefuseCollision(t *testing.T) {
	dir := t.TempDir()
	for _, dialect := range []string{"sqlite", "postgres"} {
		if err := os.Mkdir(filepath.Join(dir, dialect), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.FixedZone("offset", 3*3600))
	id, err := Create(dir, "contacts_index", now)
	if err != nil || id != "20260929090000_contacts_index" {
		t.Fatalf("%s: %v", id, err)
	}
	for _, dialect := range []string{"sqlite", "postgres"} {
		if _, err := os.Stat(filepath.Join(dir, dialect, id+".sql")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Create(dir, "contacts_index", now); err == nil {
		t.Fatal("collision overwrote migration")
	}
	if _, err := Create(dir, "../bad", now); err == nil {
		t.Fatal("unsafe name accepted")
	}
	if _, err := Create(dir, "other_branch", now); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedDialectsHaveSameIdentifiers(t *testing.T) {
	sqlite, err := fs.Glob(ForDialect("sqlite"), "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	postgres, err := fs.Glob(ForDialect("postgres"), "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(sqlite) == 0 || len(sqlite) != len(postgres) {
		t.Fatalf("migration pairs differ: %v / %v", sqlite, postgres)
	}
	for i := range sqlite {
		if sqlite[i] != postgres[i] {
			t.Fatalf("missing paired migration: %s / %s", sqlite[i], postgres[i])
		}
	}
}
