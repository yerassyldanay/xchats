package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// Chain layers an env overlay over one writable primary backend (Keyring or
// FileStore, chosen by Open). Reads always check the overlay first: an
// operator's env var or mounted secret file always wins over whatever is
// saved through the Settings UI, so a container that injects
// OPENROUTER_API_KEY at deploy time never has a stale Settings-UI value
// silently take precedence. Writes always go straight to the primary —
// EnvStore itself refuses them (ErrReadOnly), and "succeeding" a write by
// putting it somewhere the overlay doesn't even read from would be
// misleading.
type Chain struct {
	env         CredentialStore
	primary     CredentialStore
	primaryName string // "keyring" | "file" — Source's answer when env has no value
}

var _ CredentialStore = (*Chain)(nil)

func (c *Chain) Get(ctx context.Context, key Key) (string, error) {
	if v, err := c.env.Get(ctx, key); err == nil {
		return v, nil
	} else if !errors.Is(err, ErrNotFound) {
		return "", err
	}
	return c.primary.Get(ctx, key)
}

func (c *Chain) Set(ctx context.Context, key Key, value string) error {
	return c.primary.Set(ctx, key, value)
}

func (c *Chain) Delete(ctx context.Context, key Key) error {
	return c.primary.Delete(ctx, key)
}

// Source reports which backend actually answers Get(key) right now: "env"
// (the overlay has a value — a Settings UI edit for this key would be inert
// while that's true), or this Chain's primary name ("keyring"/"file").
// ErrNotFound means neither has a value. Track 2F's GET
// /settings/integrations uses this to show "managed by environment" instead
// of a misleadingly-editable field.
func (c *Chain) Source(ctx context.Context, key Key) (string, error) {
	if _, err := c.env.Get(ctx, key); err == nil {
		return "env", nil
	} else if !errors.Is(err, ErrNotFound) {
		return "", err
	}
	if _, err := c.primary.Get(ctx, key); err == nil {
		return c.primaryName, nil
	} else if !errors.Is(err, ErrNotFound) {
		return "", err
	}
	return "", ErrNotFound
}

// OpenOptions configures Open's backend selection.
type OpenOptions struct {
	// DataDir is where FileStore/FileKey read and write when the file
	// backend is selected — required whenever AllowFile is true.
	DataDir string
	// AllowFile opts into the file-backed fallback when the OS keychain is
	// not usable — see AllowFileFromEnv for the Docker default.
	AllowFile bool

	// ForceFile skips the OS-keychain probe entirely and selects FileStore
	// directly (AllowFile + DataDir are still required). Set it in EVERY
	// test outside this package.
	//
	// Without it, Open PREFERS a real OS keychain whenever one happens to be
	// usable, so a test that passes DataDir: t.TempDir() expecting an
	// isolated, empty store silently gets the developer's real keychain
	// instead: it reads their live API keys, and — because these tests seed
	// fixtures with Set — OVERWRITES them with dummies like "fc-key". That
	// is not hypothetical; it is why this field exists.
	ForceFile bool

	// keyring/available are test seams (see keyring_test.go/chain_test.go);
	// production always leaves them nil, which defaults to the real OS
	// keychain and a live availability probe.
	keyring   systemKeyring
	available func(systemKeyring) bool
}

// AllowFileFromEnv reports whether $XCHATS_ALLOW_FILE_CREDENTIALS=1 — the
// explicit opt-in Open requires before it will ever fall back to FileStore.
// The Docker image sets this by default (there is no desktop session for an
// OS keychain to run in), so a container gets a working, writable Settings
// UI out of the box; a bare-metal install must opt in deliberately.
func AllowFileFromEnv() bool {
	return os.Getenv("XCHATS_ALLOW_FILE_CREDENTIALS") == "1"
}

// Open selects the best available CredentialStore backend and returns it
// wrapped in a Chain (env always overlaid on top, per Chain's own doc):
//
//  1. The OS keychain (Keyring), if a real round-trip probe proves it is
//     usable in this environment — the preferred store wherever a desktop
//     session or a proper Secret Service provider exists. Skipped entirely
//     when opts.ForceFile is set (see that field: this preference is what
//     makes an un-opted-out test clobber real developer credentials).
//  2. FileStore, only when opts.AllowFile is set — the weaker, file-backed
//     fallback, never chosen silently.
//
// ErrNoSecureStore means neither applies: no usable keychain, and the file
// fallback was not opted into. main.go treats that as "credentials cannot
// be written from the Settings UI yet" and logs a warning rather than
// failing to boot — every existing env-var-configured credential keeps
// working regardless, since EnvStore is consulted directly by main.go's own
// legacy config fallbacks, independent of whether Open itself succeeded.
func Open(opts OpenOptions) (*Chain, error) {
	kr := opts.keyring
	if kr == nil {
		kr = realKeyring{}
	}
	probe := opts.available
	if probe == nil {
		probe = available
	}
	env := EnvStore{}

	if !opts.ForceFile && probe(kr) {
		return &Chain{env: env, primary: &Keyring{backend: kr}, primaryName: "keyring"}, nil
	}
	if opts.AllowFile {
		if opts.DataDir == "" {
			return nil, errors.New("credentials: AllowFile requires DataDir")
		}
		box, _, err := FileKey(opts.DataDir)
		if err != nil {
			return nil, fmt.Errorf("credentials: file key: %w", err)
		}
		return &Chain{env: env, primary: NewFileStore(opts.DataDir, box), primaryName: "file"}, nil
	}
	return nil, ErrNoSecureStore
}
