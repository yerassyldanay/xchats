// Package appdirs resolves the per-platform, per-user directories xchats
// stores its configuration and application data in when neither is pinned
// explicitly — the OS-appropriate final leg of config.ResolveConfigPath's
// resolution chain, and the base every credential store and settings store
// (internal/credentials, internal/settings) writes under.
//
// Every path is override-first: an explicit XCHATS_CONFIG_DIR/XCHATS_DATA_DIR
// always wins over the platform default, so a container image or a user with
// unusual storage needs can point xchats anywhere without touching code.
package appdirs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the directory xchats stores its per-user configuration
// in, for the named app ("xchats" in production — parameterized so a test
// can use a throwaway name and never collide with a real install).
//
// Resolution order:
//  1. $XCHATS_CONFIG_DIR, used verbatim — no "<app>" suffix appended, since
//     the caller who set it asked for exactly that directory.
//  2. linux/other unix: $XDG_CONFIG_HOME/<app>, else ~/.config/<app>
//     darwin:           ~/Library/Application Support/<app>
//     windows:          %APPDATA%\<app> (roaming — this is user
//     configuration, reasonable to follow a roaming profile)
func ConfigDir(app string) (string, error) {
	if v := os.Getenv("XCHATS_CONFIG_DIR"); v != "" {
		return v, nil
	}
	return configDirFor(runtime.GOOS, app)
}

// DataDir returns the directory xchats stores its per-user application data
// in (the encrypted credential file store, settings.json, ...), for the
// named app.
//
// Resolution order:
//  1. $XCHATS_DATA_DIR, used verbatim.
//  2. linux/other unix: $XDG_DATA_HOME/<app>, else ~/.local/share/<app>
//     darwin:           ~/Library/Application Support/<app> — the same
//     directory ConfigDir resolves to; macOS conventionally does not split
//     config from data the way XDG does.
//     windows:          %LOCALAPPDATA%\<app> — LOCAL, not roaming: secrets
//     and settings should not silently follow a roaming profile between
//     machines the way plain configuration arguably could.
func DataDir(app string) (string, error) {
	if v := os.Getenv("XCHATS_DATA_DIR"); v != "" {
		return v, nil
	}
	return dataDirFor(runtime.GOOS, app)
}

// EnsureDir creates dir, and any missing parents, with owner-only
// permissions (0700) if it does not already exist — every file
// internal/credentials or internal/settings writes lives under a directory
// only the owning user can even list. MkdirAll leaves an already-existing
// directory's mode untouched, and the requested mode is itself subject to
// the process umask, exactly like every other Go program's directory
// creation.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o700)
}

// ValidateOverrideDir checks a user-supplied directory override (a --data-dir
// or --config-dir style flag) before anything is resolved against it: it must
// be an absolute path, it must be possible to create it (owner-only
// permissions, via EnsureDir), and it must actually be writable. An existing
// directory that MkdirAll happily no-ops on but that the process cannot
// write into (wrong owner, read-only filesystem) would otherwise surface as
// a confusing failure deep inside the store/blob layer at first use — this
// probes it once, up front, with a message that names the actual path.
func ValidateOverrideDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("must be an absolute path, got %q", dir)
	}
	if err := EnsureDir(dir); err != nil {
		return fmt.Errorf("create directory %q: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".xchats-write-test-*")
	if err != nil {
		return fmt.Errorf("directory %q is not writable: %w", dir, err)
	}
	name := probe.Name()
	_ = probe.Close()
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("directory %q is not writable: %w", dir, err)
	}
	return nil
}

// configDirFor/dataDirFor take goos explicitly (rather than reading
// runtime.GOOS themselves) so tests can drive every platform's branch from
// a single host — see appdirs_test.go.

func configDirFor(goos, app string) (string, error) {
	switch goos {
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			return "", errors.New("appdirs: %APPDATA% is not set")
		}
		return filepath.Join(base, app), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", app), nil
	default:
		if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
			return filepath.Join(base, app), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", app), nil
	}
}

func dataDirFor(goos, app string) (string, error) {
	switch goos {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("appdirs: %LOCALAPPDATA% is not set")
		}
		return filepath.Join(base, app), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", app), nil
	default:
		if base := os.Getenv("XDG_DATA_HOME"); base != "" {
			return filepath.Join(base, app), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", app), nil
	}
}
