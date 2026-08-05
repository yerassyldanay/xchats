package credentials

import (
	"context"
	"errors"

	zalandokeyring "github.com/zalando/go-keyring"
)

// keyringService is the single service name every credential is stored
// under in the OS keychain — entries are then distinguished by
// account=string(key) within it.
const keyringService = "xchats"

// systemKeyring is the exact slice of zalando/go-keyring's package-level API
// this file uses — a seam so tests can swap in an in-memory fake instead of
// touching a real OS keychain (or relying on zalando/go-keyring's own
// process-global MockInit, which would leak state across parallel tests).
type systemKeyring interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type realKeyring struct{}

func (realKeyring) Get(service, user string) (string, error) {
	return zalandokeyring.Get(service, user)
}

func (realKeyring) Set(service, user, password string) error {
	return zalandokeyring.Set(service, user, password)
}

func (realKeyring) Delete(service, user string) error {
	return zalandokeyring.Delete(service, user)
}

// Keyring stores credentials in the OS-native secret store — macOS
// Keychain, Windows Credential Manager, or a Linux Secret Service provider
// (gnome-keyring, KWallet, ...) via zalando/go-keyring. It is the preferred
// CredentialStore backend wherever one is actually usable; see Open and
// available.
type Keyring struct {
	backend systemKeyring
}

// NewKeyring returns a Keyring backed by the real OS keychain.
func NewKeyring() *Keyring {
	return &Keyring{backend: realKeyring{}}
}

var _ CredentialStore = (*Keyring)(nil)

func (k *Keyring) Get(_ context.Context, key Key) (string, error) {
	v, err := k.backend.Get(keyringService, string(key))
	if errors.Is(err, zalandokeyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func (k *Keyring) Set(_ context.Context, key Key, value string) error {
	return k.backend.Set(keyringService, string(key), value)
}

func (k *Keyring) Delete(_ context.Context, key Key) error {
	err := k.backend.Delete(keyringService, string(key))
	if errors.Is(err, zalandokeyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// probeAccount is the canary entry available() writes-then-deletes to test
// whether the keychain actually works — distinct from every real
// credentials.Key so it can never collide with (or get overwritten by) an
// actual stored secret.
const probeAccount = "xchats-keyring-probe"

// available reports whether the OS keychain is genuinely usable in this
// environment. A headless Linux container with no Secret Service provider
// running (no gnome-keyring, no libsecret) is the common failure, and it
// only surfaces at call time — construction never fails — so the only
// reliable way to know is a real round-trip probe: write a canary value,
// read/delete it, and see whether that actually worked. Open calls this to
// decide whether Keyring belongs in the returned Chain at all.
func available(k systemKeyring) bool {
	if err := k.Set(keyringService, probeAccount, "probe"); err != nil {
		return false
	}
	_ = k.Delete(keyringService, probeAccount)
	return true
}
