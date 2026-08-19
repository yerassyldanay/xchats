package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/yerassyldanay/xchats/backend/internal/appdirs"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

const (
	bootstrapAdminCredentialFilename = "bootstrap-admin-password"
	bootstrapAdminPasswordEnv        = "XCHATS_BOOTSTRAP_ADMIN_PASSWORD"
	defaultBootstrapAdminPassword    = "xchat-admin-change-me"
)

// ensureBootstrapAdminPassword completes the sentinel admin's bootstrap.
// Migration 0006_init_admin ships that account with a static default
// password (defaultBootstrapAdminPassword, restored by
// 0011_restore_default_admin_password after 0008/0006 briefly retired it),
// so on a fresh install Store.BootstrapSentinelAdminPassword's guard — it
// only writes when password_hash is still the "" blanked sentinel — makes
// this whole function a no-op: minted comes back false and nothing is
// written. It only actually mints again on the recovery path, after "xchats
// reset-admin-password" has re-blanked the hash, or when an operator sets
// XCHATS_BOOTSTRAP_ADMIN_PASSWORD to override the default. Either way, it
// persists the plaintext in an owner-only file before installing its hash.
//
// The file is created before the database update. That ordering is deliberate:
// a crash can leave a reusable credential file for the next boot, but can
// never leave a newly-hashed password whose plaintext was lost before it was
// durably written.
func ensureBootstrapAdminPassword(ctx context.Context, cfg *config.Config, st *store.Store) (path string, minted bool, err error) {
	path, err = bootstrapAdminCredentialPath()
	if err != nil {
		return "", false, err
	}

	plaintext, created, err := loadOrCreateBootstrapAdminCredential(path, cfg.System.MinPasswordLen)
	if err != nil {
		return "", false, err
	}
	hash, err := password.Hash(plaintext)
	if err != nil {
		return "", false, fmt.Errorf("hash one-time credential: %w", err)
	}
	minted, err = st.BootstrapSentinelAdminPassword(ctx, hash)
	if err != nil {
		return "", false, err
	}

	// The database already had a real password and there was no prior
	// bootstrap file. Do not leave the newly generated, unrelated candidate
	// behind where `admin-credential show` could misleadingly display it.
	if !minted && created {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return "", false, fmt.Errorf("remove unused bootstrap credential: %w", removeErr)
		}
	}
	return path, minted, nil
}

func bootstrapAdminCredentialPath() (string, error) {
	dataDir, err := appdirs.DataDir("xchats")
	if err != nil {
		return "", fmt.Errorf("resolve application data directory: %w", err)
	}
	return filepath.Join(dataDir, bootstrapAdminCredentialFilename), nil
}

func loadOrCreateBootstrapAdminCredential(path string, minLength int) (plaintext string, created bool, err error) {
	if existing, readErr := readBootstrapAdminCredential(path); readErr == nil {
		return existing, false, nil
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return "", false, readErr
	}

	if err := appdirs.EnsureDir(filepath.Dir(path)); err != nil {
		return "", false, fmt.Errorf("create bootstrap credential directory: %w", err)
	}
	plaintext = os.Getenv(bootstrapAdminPasswordEnv)
	if plaintext == "" {
		plaintext = defaultBootstrapAdminPassword
	}
	if len(plaintext) < minLength {
		return "", false, fmt.Errorf("%s must contain at least %d characters", bootstrapAdminPasswordEnv, minLength)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: path is bootstrapAdminCredentialPath()'s fixed app-data location, never user input
	if errors.Is(err, os.ErrExist) {
		// Another process won the create race. Its fully persisted credential
		// is authoritative; read that instead of overwriting it.
		existing, readErr := readBootstrapAdminCredential(path)
		return existing, false, readErr
	}
	if err != nil {
		return "", false, fmt.Errorf("create bootstrap credential: %w", err)
	}
	removePartial := true
	defer func() {
		_ = f.Close()
		if removePartial {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.WriteString(plaintext); err != nil {
		return "", false, fmt.Errorf("write bootstrap credential: %w", err)
	}
	if err := f.Sync(); err != nil {
		return "", false, fmt.Errorf("sync bootstrap credential: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", false, fmt.Errorf("close bootstrap credential: %w", err)
	}
	removePartial = false
	return plaintext, true, nil
}

func readBootstrapAdminCredential(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: path is bootstrapAdminCredentialPath()'s fixed app-data location, never user input
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("bootstrap admin credential file is empty")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("secure bootstrap admin credential: %w", err)
	}
	return string(b), nil
}

func runAdminCredential(args []string) {
	if len(args) != 1 || args[0] != "show" {
		fatal("admin credential", errString("usage: xchats admin-credential show"))
	}
	path, err := bootstrapAdminCredentialPath()
	if err != nil {
		fatal("admin credential", err)
	}
	plaintext, err := readBootstrapAdminCredential(path)
	if errors.Is(err, os.ErrNotExist) {
		fatal("admin credential", errString("no bootstrap credential is available; it may already have been changed"))
	}
	if err != nil {
		fatal("admin credential", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, plaintext); err != nil {
		fatal("admin credential", err)
	}
}

func runResetAdminPassword(cfg *config.Config, log *slog.Logger, args []string) {
	if len(args) != 0 {
		fatal("reset admin password", errString("usage: xchats reset-admin-password"))
	}
	st := mustStore(cfg, log)
	defer st.Close()
	if err := st.ResetSentinelAdminPassword(context.Background()); err != nil {
		fatal("reset admin password", err)
	}
	path, err := bootstrapAdminCredentialPath()
	if err != nil {
		fatal("reset admin password", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		fatal("reset admin password", err)
	}
	log.Info("admin password reset; restart xchats to mint a new one-time credential")
}
