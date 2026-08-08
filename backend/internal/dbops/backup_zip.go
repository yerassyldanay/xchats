package dbops

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// BackupManifest records what one BackupZip run found/produced — written
// into the archive itself as manifest.json, so a downloaded backup is
// self-describing without needing to open xchats.db first.
type BackupManifest struct {
	CreatedAt time.Time `json:"created_at"`
	// IntegrityCheck is nil/omitted when the database is fully consistent —
	// the same "ok" vs. problem-list shape IntegrityCheck itself returns.
	IntegrityCheck []string `json:"integrity_check,omitempty"`
}

// BackupZip writes a complete disaster-recovery archive to w: a consistent
// SQLite snapshot (Backup) as xchats.db, every file under blobDir (media)
// under blob/, settingsJSON verbatim as settings.json when non-empty, and a
// manifest.json recording when the backup was taken and the result of an
// integrity check run against db beforehand.
//
// The credential store is deliberately never included: a leaked backup
// (downloaded to a laptop, attached to an email, dropped in cloud storage)
// must never also leak every configured integration's API key or xchats'
// own system secrets. Restoring those is a much smaller inconvenience
// (system secrets auto-provision fresh on next boot; integration keys are a
// few clicks in the Settings UI) than a leaked key ever is. settingsJSON
// carries no secrets — internal/settings.Settings is base URLs, model
// choices, and flags only — so it is safe to include, and worth including:
// an operator gets their provider/model preferences back on restore.
func BackupZip(ctx context.Context, db *dbx.DB, blobDir string, settingsJSON []byte, w io.Writer) (BackupManifest, error) {
	manifest := BackupManifest{CreatedAt: time.Now().UTC()}

	problems, err := IntegrityCheck(ctx, db)
	if err != nil {
		return manifest, fmt.Errorf("dbops: backup zip: integrity check: %w", err)
	}
	manifest.IntegrityCheck = problems

	tmpDir, err := os.MkdirTemp("", "xchats-backup-*")
	if err != nil {
		return manifest, fmt.Errorf("dbops: backup zip: temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "xchats.db")
	if err := Backup(ctx, db, dbPath); err != nil {
		return manifest, fmt.Errorf("dbops: backup zip: %w", err)
	}

	zw := zip.NewWriter(w)

	mw, err := zw.Create("manifest.json")
	if err != nil {
		return manifest, fmt.Errorf("dbops: backup zip: create manifest.json: %w", err)
	}
	enc := json.NewEncoder(mw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return manifest, fmt.Errorf("dbops: backup zip: write manifest.json: %w", err)
	}

	if err := addFileToZip(zw, "xchats.db", dbPath); err != nil {
		return manifest, fmt.Errorf("dbops: backup zip: %w", err)
	}

	if len(settingsJSON) > 0 {
		sw, err := zw.Create("settings.json")
		if err != nil {
			return manifest, fmt.Errorf("dbops: backup zip: create settings.json: %w", err)
		}
		if _, err := sw.Write(settingsJSON); err != nil {
			return manifest, fmt.Errorf("dbops: backup zip: write settings.json: %w", err)
		}
	}

	if blobDir != "" {
		if err := addDirToZip(zw, "blob", blobDir); err != nil {
			return manifest, fmt.Errorf("dbops: backup zip: %w", err)
		}
	}

	if err := zw.Close(); err != nil {
		return manifest, fmt.Errorf("dbops: backup zip: finalize: %w", err)
	}
	return manifest, nil
}

func addFileToZip(zw *zip.Writer, name, srcPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", srcPath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("failed to close backup source file", "path", srcPath, "err", err)
		}
	}()
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create %q in zip: %w", name, err)
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("write %q into zip: %w", name, err)
	}
	return nil
}

// addDirToZip walks srcDir and adds every regular file under it to zw,
// namespaced under prefix/ using srcDir-relative paths — a missing srcDir
// (nothing has been uploaded yet) is not an error, just an empty prefix/.
func addDirToZip(zw *zip.Writer, prefix, srcDir string) error {
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		return addFileToZip(zw, prefix+"/"+filepath.ToSlash(rel), path)
	})
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("walk %q: %w", srcDir, err)
	}
	return nil
}
