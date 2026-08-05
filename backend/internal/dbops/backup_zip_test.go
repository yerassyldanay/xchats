package dbops_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbops"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// zipEntries reads the whole archive and returns its file names -> contents,
// for tests to assert against without repeating archive/zip boilerplate.
func zipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open produced zip: %v", err)
	}
	out := make(map[string][]byte, len(r.File))
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %q: %v", f.Name, err)
		}
		out[f.Name] = b
	}
	return out
}

func TestBackupZip_ContainsManifestAndDB(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")

	var buf bytes.Buffer
	manifest, err := dbops.BackupZip(ctx, db, "", nil, &buf)
	if err != nil {
		t.Fatalf("BackupZip: %v", err)
	}
	if len(manifest.IntegrityCheck) != 0 {
		t.Errorf("manifest.IntegrityCheck = %v, want none for a healthy database", manifest.IntegrityCheck)
	}
	if manifest.CreatedAt.IsZero() {
		t.Error("manifest.CreatedAt is zero")
	}

	entries := zipEntries(t, buf.Bytes())
	if _, ok := entries["manifest.json"]; !ok {
		t.Fatal("zip missing manifest.json")
	}
	var gotManifest dbops.BackupManifest
	if err := json.Unmarshal(entries["manifest.json"], &gotManifest); err != nil {
		t.Fatalf("parse manifest.json: %v", err)
	}
	if !gotManifest.CreatedAt.Equal(manifest.CreatedAt) {
		t.Errorf("manifest.json's created_at = %v, want %v", gotManifest.CreatedAt, manifest.CreatedAt)
	}

	dbBytes, ok := entries["xchats.db"]
	if !ok {
		t.Fatal("zip missing xchats.db")
	}
	dbPath := filepath.Join(t.TempDir(), "extracted.db")
	if err := os.WriteFile(dbPath, dbBytes, 0o600); err != nil {
		t.Fatalf("write extracted db: %v", err)
	}
	check, err := dbx.OpenReadOnly(ctx, dbPath)
	if err != nil {
		t.Fatalf("open extracted xchats.db: %v", err)
	}
	defer check.Close()
	rows, err := check.Query(ctx, `SELECT name FROM widgets ORDER BY id`)
	if err != nil {
		t.Fatalf("query extracted db: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, name)
	}
	if len(names) != 2 || names[0] != "first" || names[1] != "second" {
		t.Errorf("extracted db rows = %v, want [first second]", names)
	}
}

func TestBackupZip_IncludesBlobFilesUnderPrefix(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")

	blobDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(blobDir, "a.jpg"), []byte("image-bytes"), 0o600); err != nil {
		t.Fatalf("seed blob file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(blobDir, "sub"), 0o755); err != nil {
		t.Fatalf("seed blob subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, "sub", "b.pdf"), []byte("doc-bytes"), 0o600); err != nil {
		t.Fatalf("seed nested blob file: %v", err)
	}

	var buf bytes.Buffer
	if _, err := dbops.BackupZip(ctx, db, blobDir, nil, &buf); err != nil {
		t.Fatalf("BackupZip: %v", err)
	}

	entries := zipEntries(t, buf.Bytes())
	if string(entries["blob/a.jpg"]) != "image-bytes" {
		t.Errorf("blob/a.jpg = %q, want %q", entries["blob/a.jpg"], "image-bytes")
	}
	if string(entries["blob/sub/b.pdf"]) != "doc-bytes" {
		t.Errorf("blob/sub/b.pdf = %q, want %q", entries["blob/sub/b.pdf"], "doc-bytes")
	}
}

func TestBackupZip_MissingBlobDirIsNotAnError(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")

	var buf bytes.Buffer
	if _, err := dbops.BackupZip(ctx, db, filepath.Join(t.TempDir(), "never-created"), nil, &buf); err != nil {
		t.Fatalf("BackupZip with a nonexistent blob dir: %v", err)
	}
	entries := zipEntries(t, buf.Bytes())
	for name := range entries {
		if len(name) >= 5 && name[:5] == "blob/" {
			t.Errorf("unexpected blob entry %q for a blob dir that was never created", name)
		}
	}
}

func TestBackupZip_SettingsJSONIncludedOnlyWhenNonEmpty(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")

	var withSettings bytes.Buffer
	if _, err := dbops.BackupZip(ctx, db, "", []byte(`{"llm":{"default_provider":"openrouter"}}`), &withSettings); err != nil {
		t.Fatalf("BackupZip: %v", err)
	}
	entries := zipEntries(t, withSettings.Bytes())
	if string(entries["settings.json"]) != `{"llm":{"default_provider":"openrouter"}}` {
		t.Errorf("settings.json = %q, want the exact bytes passed in", entries["settings.json"])
	}

	var withoutSettings bytes.Buffer
	if _, err := dbops.BackupZip(ctx, db, "", nil, &withoutSettings); err != nil {
		t.Fatalf("BackupZip: %v", err)
	}
	entries = zipEntries(t, withoutSettings.Bytes())
	if _, ok := entries["settings.json"]; ok {
		t.Error("settings.json present despite passing nil settingsJSON")
	}
}

// TestBackupZip_NeverIncludesCredentialFiles is a documentation-as-test
// guard for BackupZip's core security property: whatever ends up in the
// archive comes ONLY from the three explicit inputs (db, blobDir,
// settingsJSON) — there is no code path that reaches for a credential
// store file by convention/well-known-path. This test doesn't need a real
// credential store; it just enumerates every entry BackupZip actually
// produced and asserts each one is accounted for by name/prefix.
func TestBackupZip_NeverIncludesCredentialFiles(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	blobDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(blobDir, "a.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blob file: %v", err)
	}

	var buf bytes.Buffer
	if _, err := dbops.BackupZip(ctx, db, blobDir, []byte(`{}`), &buf); err != nil {
		t.Fatalf("BackupZip: %v", err)
	}
	for name := range zipEntries(t, buf.Bytes()) {
		switch {
		case name == "manifest.json", name == "xchats.db", name == "settings.json":
		case len(name) >= 5 && name[:5] == "blob/":
		default:
			t.Errorf("unexpected zip entry %q — BackupZip must only ever produce manifest.json/xchats.db/settings.json/blob/*", name)
		}
	}
}

// failAfterWriter errors once more than `remaining` bytes have been written
// through it — simulates an HTTP client disconnecting mid-download (the
// real-world way BackupZip's destination writer fails, since
// handleDownloadBackup streams straight to c.Writer) or a full disk.
type failAfterWriter struct{ remaining int }

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errors.New("simulated write failure")
	}
	if len(p) <= w.remaining {
		w.remaining -= len(p)
		return len(p), nil
	}
	n := w.remaining
	w.remaining = 0
	return n, errors.New("simulated write failure")
}

func TestBackupZip_DestinationWriteFailureIsReturnedAsError(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")

	if _, err := dbops.BackupZip(ctx, db, "", nil, &failAfterWriter{remaining: 10}); err == nil {
		t.Fatal("want an error when the destination writer fails partway through")
	}
}
