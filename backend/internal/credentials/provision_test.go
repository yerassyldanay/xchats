package credentials

import (
	"context"
	"errors"
	"testing"
)

func clearLegacySecretEnv(t *testing.T) {
	t.Helper()
	for _, spec := range systemSecrets {
		t.Setenv(spec.legacyEnv, "")
	}
}

func TestProvisionGeneratesWhenNothingExists(t *testing.T) {
	clearLegacySecretEnv(t)
	store := newMapStore()
	secrets, err := Provision(context.Background(), store)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	for name, v := range map[string]string{
		"SessionSecret":       secrets.SessionSecret,
		"TGCredentialsEncKey": secrets.TGCredentialsEncKey,
		"MCPJWTSigningKey":    secrets.MCPJWTSigningKey,
		"WebhookSecret":       secrets.WebhookSecret,
	} {
		if len(v) != 64 {
			t.Errorf("%s = %q (len %d), want a 64-char hex string", name, v, len(v))
		}
	}
	// Every value must have actually been persisted, not just returned.
	for _, spec := range systemSecrets {
		if _, err := store.Get(context.Background(), spec.key); err != nil {
			t.Errorf("store.Get(%s) after Provision: %v", spec.key, err)
		}
	}
}

func TestProvisionIsIdempotentAcrossCalls(t *testing.T) {
	clearLegacySecretEnv(t)
	store := newMapStore()
	first, err := Provision(context.Background(), store)
	if err != nil {
		t.Fatalf("Provision (first): %v", err)
	}
	second, err := Provision(context.Background(), store)
	if err != nil {
		t.Fatalf("Provision (second): %v", err)
	}
	if first != second {
		t.Errorf("Provision returned different secrets on a second call over the same store:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestProvisionAdoptsLegacyEnvValueOnce(t *testing.T) {
	clearLegacySecretEnv(t)
	t.Setenv("SESSION_SECRET", "legacy-session-secret-value")
	store := newMapStore()

	secrets, err := Provision(context.Background(), store)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if secrets.SessionSecret != "legacy-session-secret-value" {
		t.Errorf("SessionSecret = %q, want the adopted legacy env value", secrets.SessionSecret)
	}
	stored, err := store.Get(context.Background(), KeySessionSecret)
	if err != nil || stored != "legacy-session-secret-value" {
		t.Errorf("store.Get(KeySessionSecret) = (%q, %v), want (%q, nil) — legacy value must be written into the store",
			stored, err, "legacy-session-secret-value")
	}

	// Unset the env var — the store must now be authoritative (this is what
	// "adopted" means: it survives the env var going away on a later boot).
	t.Setenv("SESSION_SECRET", "")
	secrets2, err := Provision(context.Background(), store)
	if err != nil {
		t.Fatalf("Provision (after unsetting env): %v", err)
	}
	if secrets2.SessionSecret != "legacy-session-secret-value" {
		t.Errorf("SessionSecret after unsetting the env var = %q, want the still-persisted %q",
			secrets2.SessionSecret, "legacy-session-secret-value")
	}
}

func TestProvisionStoreValueWinsOverLegacyEnv(t *testing.T) {
	clearLegacySecretEnv(t)
	store := newMapStore()
	if err := store.Set(context.Background(), KeySessionSecret, "already-provisioned-value"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SESSION_SECRET", "a-stale-env-value-that-should-be-ignored")

	secrets, err := Provision(context.Background(), store)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if secrets.SessionSecret != "already-provisioned-value" {
		t.Errorf("SessionSecret = %q, want the store's existing value %q (store wins over a stale legacy env var)",
			secrets.SessionSecret, "already-provisioned-value")
	}
}

// failingStore's Set always fails — how this test proves Provision surfaces
// (rather than swallows) a persistence error.
type failingStore struct{ *mapStore }

func (f failingStore) Set(context.Context, Key, string) error {
	return errors.New("disk full")
}

func TestProvisionPropagatesStoreErrors(t *testing.T) {
	clearLegacySecretEnv(t)
	store := failingStore{newMapStore()}
	if _, err := Provision(context.Background(), store); err == nil {
		t.Fatal("Provision with a failing store: want an error, got nil")
	}
}

func TestProvisionEachSecretIsDistinct(t *testing.T) {
	clearLegacySecretEnv(t)
	secrets, err := Provision(context.Background(), newMapStore())
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	values := []string{secrets.SessionSecret, secrets.TGCredentialsEncKey, secrets.MCPJWTSigningKey, secrets.WebhookSecret}
	seen := map[string]bool{}
	for _, v := range values {
		if seen[v] {
			t.Fatalf("Provision generated the same value for two different secrets: %+v", secrets)
		}
		seen[v] = true
	}
}
