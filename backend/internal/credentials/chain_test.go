package credentials

import (
	"context"
	"errors"
	"testing"
)

// mapStore is a tiny CredentialStore double backed by a plain map — used
// here as Chain's primary so these tests don't depend on Keyring/FileStore
// specifics, only on Chain's own overlay/delegation logic.
type mapStore struct {
	values map[Key]string
}

func newMapStore() *mapStore { return &mapStore{values: map[Key]string{}} }

func (m *mapStore) Get(_ context.Context, key Key) (string, error) {
	v, ok := m.values[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *mapStore) Set(_ context.Context, key Key, value string) error {
	m.values[key] = value
	return nil
}

func (m *mapStore) Delete(_ context.Context, key Key) error {
	if _, ok := m.values[key]; !ok {
		return ErrNotFound
	}
	delete(m.values, key)
	return nil
}

func TestChainEnvOverlayWinsReads(t *testing.T) {
	env := newMapStore()
	primary := newMapStore()
	env.values["openrouter.api_key"] = "from-env"
	primary.values["openrouter.api_key"] = "from-primary"
	c := &Chain{env: env, primary: primary, primaryName: "file"}

	got, err := c.Get(context.Background(), "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "from-env" {
		t.Errorf("Get = %q, want the env overlay's value %q", got, "from-env")
	}
}

func TestChainFallsThroughToPrimaryWhenEnvHasNoValue(t *testing.T) {
	env := newMapStore()
	primary := newMapStore()
	primary.values["openrouter.api_key"] = "from-primary"
	c := &Chain{env: env, primary: primary, primaryName: "file"}

	got, err := c.Get(context.Background(), "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "from-primary" {
		t.Errorf("Get = %q, want %q", got, "from-primary")
	}
}

func TestChainNotFoundWhenNeitherHasIt(t *testing.T) {
	c := &Chain{env: newMapStore(), primary: newMapStore(), primaryName: "file"}
	if _, err := c.Get(context.Background(), "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: err = %v, want ErrNotFound", err)
	}
}

func TestChainSetAndDeleteGoToPrimaryOnly(t *testing.T) {
	env := newMapStore()
	primary := newMapStore()
	c := &Chain{env: env, primary: primary, primaryName: "file"}
	ctx := context.Background()

	if err := c.Set(ctx, "openrouter.api_key", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok := env.values["openrouter.api_key"]; ok {
		t.Error("Set wrote into the env overlay — it must only ever go to primary")
	}
	if primary.values["openrouter.api_key"] != "v" {
		t.Error("Set did not reach primary")
	}

	if err := c.Delete(ctx, "openrouter.api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := primary.values["openrouter.api_key"]; ok {
		t.Error("Delete did not remove the value from primary")
	}
}

func TestChainSource(t *testing.T) {
	ctx := context.Background()

	t.Run("env", func(t *testing.T) {
		env := newMapStore()
		env.values["k"] = "v"
		c := &Chain{env: env, primary: newMapStore(), primaryName: "file"}
		src, err := c.Source(ctx, "k")
		if err != nil || src != "env" {
			t.Errorf("Source = (%q, %v), want (%q, nil)", src, err, "env")
		}
	})

	t.Run("primary", func(t *testing.T) {
		primary := newMapStore()
		primary.values["k"] = "v"
		c := &Chain{env: newMapStore(), primary: primary, primaryName: "keyring"}
		src, err := c.Source(ctx, "k")
		if err != nil || src != "keyring" {
			t.Errorf("Source = (%q, %v), want (%q, nil)", src, err, "keyring")
		}
	})

	t.Run("neither", func(t *testing.T) {
		c := &Chain{env: newMapStore(), primary: newMapStore(), primaryName: "keyring"}
		if _, err := c.Source(ctx, "k"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Source: err = %v, want ErrNotFound", err)
		}
	})
}

func TestOpenChoosesKeyringWhenAvailable(t *testing.T) {
	fake := newFakeSystemKeyring()
	c, err := Open(OpenOptions{
		keyring:   fake,
		available: func(systemKeyring) bool { return true },
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if c.primaryName != "keyring" {
		t.Errorf("primaryName = %q, want %q", c.primaryName, "keyring")
	}
	// The Chain actually works end to end against the fake.
	if err := c.Set(context.Background(), "openrouter.api_key", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, err := c.Get(context.Background(), "openrouter.api_key"); err != nil || got != "v" {
		t.Errorf("Get = (%q, %v), want (%q, nil)", got, err, "v")
	}
}

func TestOpenFallsBackToFileWhenAllowed(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(OpenOptions{
		AllowFile: true,
		DataDir:   dir,
		keyring:   newFakeSystemKeyring(),
		available: func(systemKeyring) bool { return false },
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if c.primaryName != "file" {
		t.Errorf("primaryName = %q, want %q", c.primaryName, "file")
	}
	if err := c.Set(context.Background(), "openrouter.api_key", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, err := c.Get(context.Background(), "openrouter.api_key"); err != nil || got != "v" {
		t.Errorf("Get = (%q, %v), want (%q, nil)", got, err, "v")
	}
}

// ForceFile is what keeps every out-of-package test isolated to its own
// t.TempDir(). Without it those tests silently bind to the developer's real
// OS keychain — reading their live API keys and overwriting them with the
// fixtures they seed (see OpenOptions.ForceFile). An available keyring must
// therefore lose to ForceFile, and the resulting store must be the file one.
func TestOpenForceFileIgnoresAnAvailableKeyring(t *testing.T) {
	fake := newFakeSystemKeyring()
	if err := fake.Set("xchats", "openrouter.api_key", "the-developers-real-key"); err != nil {
		t.Fatalf("seed keyring: %v", err)
	}
	c, err := Open(OpenOptions{
		AllowFile: true,
		ForceFile: true,
		DataDir:   t.TempDir(),
		keyring:   fake,
		available: func(systemKeyring) bool { return true },
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if c.primaryName != "file" {
		t.Fatalf("primaryName = %q, want %q", c.primaryName, "file")
	}
	// The isolated store starts empty — the keyring's value is not visible...
	if _, err := c.Get(context.Background(), "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: err = %v, want ErrNotFound (keyring value leaked into an isolated store)", err)
	}
	// ...and writing a fixture does not reach back into the keyring.
	if err := c.Set(context.Background(), "openrouter.api_key", "sk-unknown"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, _ := fake.Get("xchats", "openrouter.api_key"); got != "the-developers-real-key" {
		t.Errorf("keyring value = %q, want it untouched — a test fixture overwrote a real credential", got)
	}
}

func TestOpenReturnsErrNoSecureStoreWhenNeitherAvailable(t *testing.T) {
	_, err := Open(OpenOptions{
		AllowFile: false,
		keyring:   newFakeSystemKeyring(),
		available: func(systemKeyring) bool { return false },
	})
	if !errors.Is(err, ErrNoSecureStore) {
		t.Errorf("Open: err = %v, want ErrNoSecureStore", err)
	}
}

func TestOpenAllowFileWithoutDataDirErrors(t *testing.T) {
	_, err := Open(OpenOptions{
		AllowFile: true,
		DataDir:   "",
		keyring:   newFakeSystemKeyring(),
		available: func(systemKeyring) bool { return false },
	})
	if err == nil {
		t.Fatal("Open with AllowFile and no DataDir: want an error, got nil")
	}
}

func TestAllowFileFromEnv(t *testing.T) {
	t.Setenv("XCHATS_ALLOW_FILE_CREDENTIALS", "1")
	if !AllowFileFromEnv() {
		t.Error("AllowFileFromEnv() = false with the env var set to \"1\", want true")
	}
	t.Setenv("XCHATS_ALLOW_FILE_CREDENTIALS", "")
	if AllowFileFromEnv() {
		t.Error("AllowFileFromEnv() = true with the env var unset, want false")
	}
	t.Setenv("XCHATS_ALLOW_FILE_CREDENTIALS", "true")
	if AllowFileFromEnv() {
		t.Error(`AllowFileFromEnv() = true for the value "true" (only the literal "1" opts in), want false`)
	}
}
