package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func TestEnsureBootstrapAdminPasswordMintsOnceAndPersists(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XCHATS_DATA_DIR", dataDir)
	t.Setenv(bootstrapAdminPasswordEnv, "")

	st, err := store.New(context.Background(), filepath.Join(t.TempDir(), "xchats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	cfg := &config.Config{System: config.SystemConfig{MinPasswordLen: 8}}

	path, minted, err := ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if minted {
		t.Fatal("bootstrap minted on a freshly migrated DB — 0011 already restores the default password")
	}
	if path != filepath.Join(dataDir, bootstrapAdminCredentialFilename) {
		t.Fatalf("credential path = %q", path)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unused bootstrap credential was left on disk: %v", err)
	}
	u, err := st.UserByEmail(context.Background(), "admin@xchat.kz")
	if err != nil {
		t.Fatal(err)
	}
	if !password.Verify(defaultBootstrapAdminPassword, u.PasswordHash) {
		t.Fatal("the default password does not verify against the sentinel admin's hash")
	}
	// 0014_force_default_admin_password_change leaves this set on a fresh
	// install — bootstrap here is a no-op (0011 already restored the hash, so
	// BootstrapSentinelAdminPassword's password_hash='' guard never fires) and
	// never touches the flag itself.
	if !u.MustChangePassword {
		t.Fatal("sentinel admin's must_change_password = false, want true")
	}

	_, minted, err = ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if minted {
		t.Fatal("second bootstrap minted on an already-bootstrapped DB")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unused bootstrap credential was left on disk after the second call: %v", err)
	}
}

func TestEnsureBootstrapAdminPasswordUsesConfiguredFirstBootValue(t *testing.T) {
	t.Setenv("XCHATS_DATA_DIR", t.TempDir())
	const configured = "operator-supplied-one-time-password"
	t.Setenv(bootstrapAdminPasswordEnv, configured)

	st, err := store.New(context.Background(), filepath.Join(t.TempDir(), "xchats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := st.ResetSentinelAdminPassword(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{System: config.SystemConfig{MinPasswordLen: 8}}

	path, minted, err := ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if !minted {
		t.Fatal("configured first-boot password was not minted")
	}
	got, err := readBootstrapAdminCredential(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != configured {
		t.Fatalf("credential = %q, want configured value", got)
	}
}

func TestEnsureBootstrapAdminPasswordDoesNotLeaveUnrelatedCredential(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XCHATS_DATA_DIR", dataDir)
	t.Setenv(bootstrapAdminPasswordEnv, "")

	st, err := store.New(context.Background(), filepath.Join(t.TempDir(), "xchats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	u, err := st.UserByEmail(context.Background(), "admin@xchat.kz")
	if err != nil {
		t.Fatal(err)
	}
	existingHash, err := password.Hash("already-configured-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserPassword(context.Background(), u.ID, existingHash); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{System: config.SystemConfig{MinPasswordLen: 8}}
	path, minted, err := ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if minted {
		t.Fatal("bootstrap replaced an already-configured admin password")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unused bootstrap credential was left on disk: %v", err)
	}
}

func TestResetSentinelAdminPasswordRemintsOnNextBootstrap(t *testing.T) {
	t.Setenv("XCHATS_DATA_DIR", t.TempDir())
	t.Setenv(bootstrapAdminPasswordEnv, "")
	st, err := store.New(context.Background(), filepath.Join(t.TempDir(), "xchats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	cfg := &config.Config{System: config.SystemConfig{MinPasswordLen: 8}}

	// A freshly migrated DB already has the restored default password (0011),
	// so nothing mints until the hash is blanked again — exactly what "xchats
	// reset-admin-password" does.
	if err := st.ResetSentinelAdminPassword(context.Background()); err != nil {
		t.Fatal(err)
	}
	path, minted, err := ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if !minted {
		t.Fatal("reset admin password was not re-minted")
	}
	reminted, err := readBootstrapAdminCredential(path)
	if err != nil {
		t.Fatal(err)
	}
	if reminted != defaultBootstrapAdminPassword {
		t.Fatalf("re-minted credential = %q, want the static default %q", reminted, defaultBootstrapAdminPassword)
	}
	u, err := st.UserByEmail(context.Background(), "admin@xchat.kz")
	if err != nil {
		t.Fatal(err)
	}
	if !password.Verify(reminted, u.PasswordHash) {
		t.Fatal("re-minted credential does not verify against the stored hash")
	}
}
