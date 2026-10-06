package migrations

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestCreateUTCAndRefuseCollision(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.FixedZone("offset", 3*3600))
	id, err := Create(dir, "contacts_index", now)
	if err != nil || id != "20260929090000_contacts_index" {
		t.Fatalf("%s: %v", id, err)
	}
	body, err := fs.ReadFile(os.DirFS(dir), id+".sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-- contacts_index", "Plain SQL", "SQLite and PostgreSQL", "idempotent"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("generated file lacks %q:\n%s", want, body)
		}
	}
	// There is no template step any more: a header that promises dialect macros misleads.
	if strings.Contains(strings.ToLower(string(body)), "macro") {
		t.Errorf("generated header still mentions macros:\n%s", body)
	}
	// One shared file: no per-dialect directories or copies.
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Fatalf("created %d entries (%v), want exactly one shared file", len(entries), err)
	}
	if _, err := Create(dir, "contacts_index", now); err == nil {
		t.Fatal("collision overwrote migration")
	}
	if _, err := Create(dir, "../bad", now); err == nil {
		t.Fatal("unsafe name accepted")
	}
	if _, err := Create(dir, "Not_Snake", now); err == nil {
		t.Fatal("non snake_case name accepted")
	}
	if _, err := Create(dir, "other_branch", now); err != nil {
		t.Fatalf("same-second migration with a different name: %v", err)
	}
}

func TestEmbeddedMigrationsAreTimestampedSQL(t *testing.T) {
	name := regexp.MustCompile(`^[0-9]{14}_[a-z][a-z0-9_]*\.sql$`)
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded migrations")
	}
	prev := ""
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("%s: per-dialect subdirectories are gone; migrations are shared files", e.Name())
			continue
		}
		if !name.MatchString(e.Name()) {
			t.Errorf("%s does not match YYYYMMDDHHMMSS_description.sql", e.Name())
			continue
		}
		if _, err := time.Parse("20060102150405", e.Name()[:14]); err != nil {
			t.Errorf("%s: invalid UTC timestamp: %v", e.Name(), err)
		}
		if e.Name() <= prev {
			t.Errorf("%s is not strictly after %s", e.Name(), prev)
		}
		prev = e.Name()
	}
}
