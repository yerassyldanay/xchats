package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
)

// openTestStore is dbtest.Open's own shape, reproduced here directly: this
// file lives in package store itself (to reach the unexported demo* id
// vars below), and internal/dbtest already imports package store, so
// importing dbtest here would be a cycle.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "xchats.db")
	st, err := New(ctx, path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

// testBox returns a secretbox.Box suitable for a test's credential
// read/write round-trip.
func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatalf("secretbox.New: %v", err)
	}
	return box
}

func TestSeedDemoMockCredentials_BackfillsEveryNonWhatsAppDemoAccount(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("SeedOrganization: %v", err)
	}
	adminID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	if inserted, err := st.SeedDemoWorkspace(ctx, org.ID, adminID); err != nil || !inserted {
		t.Fatalf("SeedDemoWorkspace: inserted=%v err=%v", inserted, err)
	}

	st.UseCredentialsBox(testBox(t))
	if err := st.SeedDemoMockCredentials(ctx); err != nil {
		t.Fatalf("SeedDemoMockCredentials: %v", err)
	}

	tok, err := st.TelegramBotToken(ctx, demoTelegramAccountID)
	if err != nil || tok == "" {
		t.Errorf("TelegramBotToken(demo telegram) = %q, %v; want a resolvable token", tok, err)
	}
	for _, id := range []uuid.UUID{demoInstagramAccountID, demoMessengerAccountID, demoWhatsAppCloudAccountID} {
		secret, err := st.ChannelCredentialsSecret(ctx, id)
		if err != nil || secret == "" {
			t.Errorf("ChannelCredentialsSecret(%s) = %q, %v; want a resolvable secret", id, secret, err)
		}
	}
}

// TestSeedDemoMockCredentials_IsIdempotent guards the ON CONFLICT DO UPDATE
// path both writers rely on — running seed-demo (and this backfill) twice
// against the same database, as the Makefile's profile-server target does
// on every restart, must never error.
func TestSeedDemoMockCredentials_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("SeedOrganization: %v", err)
	}
	if _, err := st.SeedDemoWorkspace(ctx, org.ID, uuid.MustParse("00000000-0000-0000-0000-000000000002")); err != nil {
		t.Fatalf("SeedDemoWorkspace: %v", err)
	}
	st.UseCredentialsBox(testBox(t))
	if err := st.SeedDemoMockCredentials(ctx); err != nil {
		t.Fatalf("first SeedDemoMockCredentials: %v", err)
	}
	if err := st.SeedDemoMockCredentials(ctx); err != nil {
		t.Fatalf("second SeedDemoMockCredentials: %v", err)
	}
}

// TestSeedDemoMockCredentials_RequiresCredentialsBox matches every other
// credential write in this package: no box installed means ErrNoCredentialsKey,
// never a silent no-op or a plaintext write.
func TestSeedDemoMockCredentials_RequiresCredentialsBox(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("SeedOrganization: %v", err)
	}
	if _, err := st.SeedDemoWorkspace(ctx, org.ID, uuid.MustParse("00000000-0000-0000-0000-000000000002")); err != nil {
		t.Fatalf("SeedDemoWorkspace: %v", err)
	}
	if err := st.SeedDemoMockCredentials(ctx); err == nil {
		t.Fatal("expected an error with no credentials box installed")
	}
}
