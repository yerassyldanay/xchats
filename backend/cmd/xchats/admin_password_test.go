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
	if !minted {
		t.Fatal("first bootstrap did not mint the sentinel admin password")
	}
	if path != filepath.Join(dataDir, bootstrapAdminCredentialFilename) {
		t.Fatalf("credential path = %q", path)
	}
	plaintext, err := readBootstrapAdminCredential(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) < cfg.System.MinPasswordLen {
		t.Fatalf("generated credential length = %d", len(plaintext))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("credential permissions = %o, want 600", got)
	}
	u, err := st.UserByEmail(context.Background(), "admin@xchat.kz")
	if err != nil {
		t.Fatal(err)
	}
	if !password.Verify(plaintext, u.PasswordHash) {
		t.Fatal("persisted credential does not verify against the admin hash")
	}
	if !u.MustChangePassword {
		t.Fatal("bootstrap unexpectedly cleared must_change_password")
	}

	_, minted, err = ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if minted {
		t.Fatal("second bootstrap replaced an existing admin password")
	}
	again, err := readBootstrapAdminCredential(path)
	if err != nil {
		t.Fatal(err)
	}
	if again != plaintext {
		t.Fatal("second bootstrap replaced the one-time credential file")
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

	path, _, err := ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := readBootstrapAdminCredential(path)
	if err := st.ResetSentinelAdminPassword(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	_, minted, err := ensureBootstrapAdminPassword(context.Background(), cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if !minted {
		t.Fatal("reset admin password was not re-minted")
	}
	second, _ := readBootstrapAdminCredential(path)
	if second == first {
		t.Fatal("reset reused the previous one-time credential")
	}
}
