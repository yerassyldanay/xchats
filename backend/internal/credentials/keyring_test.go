package credentials

import (
	"context"
	"errors"
	"testing"

	zalandokeyring "github.com/zalando/go-keyring"
)

// fakeSystemKeyring is an in-memory systemKeyring double — used instead of
// zalando/go-keyring's own MockInit so these tests never touch that
// package's process-global state (and so a "keyring unavailable" scenario
// is trivial to construct, unlike with a real OS keychain).
type fakeSystemKeyring struct {
	values map[[2]string]string
	// failSet/failDelete, if non-nil, are returned by every Set/Delete call
	// regardless of arguments — how tests simulate an unusable keychain.
	failSet    error
	failDelete error
}

func newFakeSystemKeyring() *fakeSystemKeyring {
	return &fakeSystemKeyring{values: map[[2]string]string{}}
}

func (f *fakeSystemKeyring) Get(service, user string) (string, error) {
	v, ok := f.values[[2]string{service, user}]
	if !ok {
		return "", zalandokeyring.ErrNotFound
	}
	return v, nil
}

func (f *fakeSystemKeyring) Set(service, user, password string) error {
	if f.failSet != nil {
		return f.failSet
	}
	f.values[[2]string{service, user}] = password
	return nil
}

func (f *fakeSystemKeyring) Delete(service, user string) error {
	if f.failDelete != nil {
		return f.failDelete
	}
	k := [2]string{service, user}
	if _, ok := f.values[k]; !ok {
		return zalandokeyring.ErrNotFound
	}
	delete(f.values, k)
	return nil
}

func TestKeyringRoundTrip(t *testing.T) {
	fake := newFakeSystemKeyring()
	kr := &Keyring{backend: fake}
	ctx := context.Background()

	if _, err := kr.Get(ctx, "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get before Set: err = %v, want ErrNotFound", err)
	}
	if err := kr.Set(ctx, "openrouter.api_key", "sk-test"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := kr.Get(ctx, "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if got != "sk-test" {
		t.Errorf("Get = %q, want %q", got, "sk-test")
	}

	if err := kr.Delete(ctx, "openrouter.api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := kr.Get(ctx, "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotFound", err)
	}
	if err := kr.Delete(ctx, "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete of an already-deleted key: err = %v, want ErrNotFound", err)
	}
}

func TestKeyringIsolatesKeysByAccount(t *testing.T) {
	fake := newFakeSystemKeyring()
	kr := &Keyring{backend: fake}
	ctx := context.Background()
	if err := kr.Set(ctx, "openrouter.api_key", "a"); err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(ctx, "openai.api_key", "b"); err != nil {
		t.Fatal(err)
	}
	got, err := kr.Get(ctx, "openrouter.api_key")
	if err != nil || got != "a" {
		t.Errorf("openrouter.api_key = (%q, %v), want (%q, nil)", got, err, "a")
	}
	got, err = kr.Get(ctx, "openai.api_key")
	if err != nil || got != "b" {
		t.Errorf("openai.api_key = (%q, %v), want (%q, nil)", got, err, "b")
	}
}

var _ CredentialStore = (*Keyring)(nil)

func TestAvailableProbesRoundTrip(t *testing.T) {
	fake := newFakeSystemKeyring()
	if !available(fake) {
		t.Error("available() = false for a working fake keyring, want true")
	}
	// The probe cleans up after itself — no residual canary entry.
	if _, ok := fake.values[[2]string{keyringService, probeAccount}]; ok {
		t.Error("available() left its probe canary entry behind")
	}
}

func TestAvailableFalseWhenSetFails(t *testing.T) {
	fake := newFakeSystemKeyring()
	fake.failSet = errors.New("no secret service provider running")
	if available(fake) {
		t.Error("available() = true despite Set failing, want false")
	}
}

func TestAvailableTrueEvenIfDeleteFails(t *testing.T) {
	// A keychain that can Set but not immediately Delete a fresh entry is
	// still "available" for storing real credentials — the probe's own
	// cleanup failing should not itself flip the verdict.
	fake := newFakeSystemKeyring()
	fake.failDelete = errors.New("transient")
	if !available(fake) {
		t.Error("available() = false when only the probe's own cleanup Delete fails, want true")
	}
}
