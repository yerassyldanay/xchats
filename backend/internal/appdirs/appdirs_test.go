package appdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirOverrideWinsVerbatim(t *testing.T) {
	t.Setenv("XCHATS_CONFIG_DIR", "/custom/config/path")
	got, err := ConfigDir("xchats")
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if got != "/custom/config/path" {
		t.Errorf("ConfigDir override = %q, want %q (no app suffix appended)", got, "/custom/config/path")
	}
}

func TestDataDirOverrideWinsVerbatim(t *testing.T) {
	t.Setenv("XCHATS_DATA_DIR", "/custom/data/path")
	got, err := DataDir("xchats")
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if got != "/custom/data/path" {
		t.Errorf("DataDir override = %q, want %q (no app suffix appended)", got, "/custom/data/path")
	}
}

func TestConfigDirForPlatforms(t *testing.T) {
	t.Run("linux with XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/home/op/.xdgconfig")
		got, err := configDirFor("linux", "xchats")
		if err != nil {
			t.Fatalf("configDirFor: %v", err)
		}
		if want := filepath.Join("/home/op/.xdgconfig", "xchats"); got != want {
			t.Errorf("configDirFor(linux) = %q, want %q", got, want)
		}
	})

	t.Run("linux without XDG_CONFIG_HOME falls back to ~/.config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/home/op")
		got, err := configDirFor("linux", "xchats")
		if err != nil {
			t.Fatalf("configDirFor: %v", err)
		}
		if want := filepath.Join("/home/op", ".config", "xchats"); got != want {
			t.Errorf("configDirFor(linux) = %q, want %q", got, want)
		}
	})

	t.Run("linux with neither XDG_CONFIG_HOME nor HOME errors", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
		if _, err := configDirFor("linux", "xchats"); err == nil {
			t.Error("configDirFor(linux) with no HOME: want error, got nil")
		}
	})

	t.Run("darwin uses Application Support", func(t *testing.T) {
		t.Setenv("HOME", "/Users/op")
		got, err := configDirFor("darwin", "xchats")
		if err != nil {
			t.Fatalf("configDirFor: %v", err)
		}
		if want := filepath.Join("/Users/op", "Library", "Application Support", "xchats"); got != want {
			t.Errorf("configDirFor(darwin) = %q, want %q", got, want)
		}
	})

	t.Run("windows uses APPDATA", func(t *testing.T) {
		t.Setenv("APPDATA", `C:\Users\op\AppData\Roaming`)
		got, err := configDirFor("windows", "xchats")
		if err != nil {
			t.Fatalf("configDirFor: %v", err)
		}
		if want := filepath.Join(`C:\Users\op\AppData\Roaming`, "xchats"); got != want {
			t.Errorf("configDirFor(windows) = %q, want %q", got, want)
		}
	})

	t.Run("windows without APPDATA errors", func(t *testing.T) {
		t.Setenv("APPDATA", "")
		if _, err := configDirFor("windows", "xchats"); err == nil {
			t.Error("configDirFor(windows) with no APPDATA: want error, got nil")
		}
	})
}

func TestDataDirForPlatforms(t *testing.T) {
	t.Run("linux with XDG_DATA_HOME", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/home/op/.xdgdata")
		got, err := dataDirFor("linux", "xchats")
		if err != nil {
			t.Fatalf("dataDirFor: %v", err)
		}
		if want := filepath.Join("/home/op/.xdgdata", "xchats"); got != want {
			t.Errorf("dataDirFor(linux) = %q, want %q", got, want)
		}
	})

	t.Run("linux without XDG_DATA_HOME falls back to ~/.local/share", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", "/home/op")
		got, err := dataDirFor("linux", "xchats")
		if err != nil {
			t.Fatalf("dataDirFor: %v", err)
		}
		if want := filepath.Join("/home/op", ".local", "share", "xchats"); got != want {
			t.Errorf("dataDirFor(linux) = %q, want %q", got, want)
		}
	})

	t.Run("darwin uses Application Support, same as config", func(t *testing.T) {
		t.Setenv("HOME", "/Users/op")
		got, err := dataDirFor("darwin", "xchats")
		if err != nil {
			t.Fatalf("dataDirFor: %v", err)
		}
		if want := filepath.Join("/Users/op", "Library", "Application Support", "xchats"); got != want {
			t.Errorf("dataDirFor(darwin) = %q, want %q", got, want)
		}
	})

	t.Run("windows uses LOCALAPPDATA, distinct from config's APPDATA", func(t *testing.T) {
		t.Setenv("LOCALAPPDATA", `C:\Users\op\AppData\Local`)
		got, err := dataDirFor("windows", "xchats")
		if err != nil {
			t.Fatalf("dataDirFor: %v", err)
		}
		if want := filepath.Join(`C:\Users\op\AppData\Local`, "xchats"); got != want {
			t.Errorf("dataDirFor(windows) = %q, want %q", got, want)
		}
	})

	t.Run("windows without LOCALAPPDATA errors", func(t *testing.T) {
		t.Setenv("LOCALAPPDATA", "")
		if _, err := dataDirFor("windows", "xchats"); err == nil {
			t.Error("dataDirFor(windows) with no LOCALAPPDATA: want error, got nil")
		}
	})
}

// TestConfigDirAndDataDirDivergeOnRealHost is a light integration check of
// the public, override-free entry points (ConfigDir/DataDir over the actual
// runtime.GOOS) rather than the goos-parameterized internals above: both
// must resolve to a NON-empty path ending in the app name, and on every
// platform except darwin the two must differ (a shared directory would mean
// credentials and config got mixed under the same tree).
func TestConfigDirAndDataDirOnRealHost(t *testing.T) {
	t.Setenv("XCHATS_CONFIG_DIR", "")
	t.Setenv("XCHATS_DATA_DIR", "")
	cfg, err := ConfigDir("xchats-appdirs-test")
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	data, err := DataDir("xchats-appdirs-test")
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if filepath.Base(cfg) != "xchats-appdirs-test" {
		t.Errorf("ConfigDir = %q, want it to end in the app name", cfg)
	}
	if filepath.Base(data) != "xchats-appdirs-test" {
		t.Errorf("DataDir = %q, want it to end in the app name", data)
	}
}

func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "app", "dir")
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat after EnsureDir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("permissions = %o, want %o", perm, 0o700)
	}

	// Idempotent: calling it again on an existing directory is not an error.
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir on existing dir: %v", err)
	}
}
