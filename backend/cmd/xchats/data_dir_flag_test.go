package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestApplyDataDirFlag_Empty proves an absent --data-dir is a no-op: an
// existing XCHATS_DATA_DIR in the environment must survive untouched, so
// the flag never silently overrides a value the operator set another way.
func TestApplyDataDirFlag_Empty(t *testing.T) {
	t.Setenv("XCHATS_DATA_DIR", "/already/set")
	if err := applyDataDirFlag(""); err != nil {
		t.Fatalf("applyDataDirFlag(\"\") = %v, want nil", err)
	}
	if got := os.Getenv("XCHATS_DATA_DIR"); got != "/already/set" {
		t.Errorf("XCHATS_DATA_DIR = %q, want untouched %q", got, "/already/set")
	}
}

// TestApplyDataDirFlag_RelativeRejected proves --data-dir must be absolute —
// a relative path is ambiguous the same way a bare config.yaml lookup would
// be for a packaged app (see desktop.ConfigPath's own doc comment).
func TestApplyDataDirFlag_RelativeRejected(t *testing.T) {
	t.Setenv("XCHATS_DATA_DIR", "")
	if err := applyDataDirFlag("relative/path"); err == nil {
		t.Fatal("applyDataDirFlag(relative) = nil, want an error")
	}
	if got := os.Getenv("XCHATS_DATA_DIR"); got != "" {
		t.Errorf("XCHATS_DATA_DIR = %q after a rejected flag, want untouched empty", got)
	}
}

// TestApplyDataDirFlag_ValidAbsoluteWinsOverExistingEnv proves the flag
// outranks a pre-existing XCHATS_DATA_DIR — the documented precedence is
// --data-dir, then $XCHATS_DATA_DIR, then the OS default.
func TestApplyDataDirFlag_ValidAbsoluteWinsOverExistingEnv(t *testing.T) {
	t.Setenv("XCHATS_DATA_DIR", "/some/other/path")
	dir := filepath.Join(t.TempDir(), "data-root")
	if err := applyDataDirFlag(dir); err != nil {
		t.Fatalf("applyDataDirFlag(%q) = %v, want nil", dir, err)
	}
	if got := os.Getenv("XCHATS_DATA_DIR"); got != dir {
		t.Errorf("XCHATS_DATA_DIR = %q, want the flag value %q", got, dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s was not created as a directory", dir)
	}
}

// TestApplyDataDirFlag_NotWritable proves a directory the process cannot
// write into fails with an actionable error rather than being silently
// accepted and surfacing as a confusing failure deep in the store layer.
func TestApplyDataDirFlag_NotWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	t.Setenv("XCHATS_DATA_DIR", "")
	parent := t.TempDir()
	locked := filepath.Join(parent, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatalf("mkdir locked dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	if err := applyDataDirFlag(locked); err == nil {
		t.Fatal("applyDataDirFlag(unwritable dir) = nil, want an error")
	}
	if got := os.Getenv("XCHATS_DATA_DIR"); got != "" {
		t.Errorf("XCHATS_DATA_DIR = %q after a rejected flag, want untouched empty", got)
	}
}
