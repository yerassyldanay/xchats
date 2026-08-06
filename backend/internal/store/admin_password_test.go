package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/password"
)

var sentinelAdminID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

// TestBootstrapSentinelAdminPassword_MintsOnceNotTwice pins
// BootstrapSentinelAdminPassword's idempotency: it must write the hash
// exactly once (on the row 0008_bootstrap_admin.up.sql left with an empty
// password_hash) and report minted=false on every call after — the
// property cmd/xchats' first-boot bootstrap depends on to never re-mint
// (and re-print) a fresh password on a normal restart.
func TestBootstrapSentinelAdminPassword_MintsOnceNotTwice(t *testing.T) {
	st, _ := dbtest.Open(t)
	ctx := context.Background()

	hash1, err := password.Hash("first-one-time-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	minted, err := st.BootstrapSentinelAdminPassword(ctx, hash1)
	if err != nil {
		t.Fatalf("first bootstrap call: %v", err)
	}
	if !minted {
		t.Fatal("first call on a freshly migrated DB: minted = false, want true")
	}

	got, err := st.UserByEmail(ctx, "admin@xchat.kz")
	if err != nil {
		t.Fatalf("lookup admin: %v", err)
	}
	if !password.Verify("first-one-time-password", got.PasswordHash) {
		t.Error("stored hash does not verify against the minted password")
	}
	if !got.MustChangePassword {
		t.Error("must_change_password = false after minting — bootstrap must leave it set")
	}

	// A second call (a restart) must be a no-op, even with a different
	// candidate password — proves it only ever mints once.
	hash2, err := password.Hash("second-candidate-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	minted, err = st.BootstrapSentinelAdminPassword(ctx, hash2)
	if err != nil {
		t.Fatalf("second bootstrap call: %v", err)
	}
	if minted {
		t.Error("second call: minted = true, want false (must not re-mint)")
	}
	got, err = st.UserByEmail(ctx, "admin@xchat.kz")
	if err != nil {
		t.Fatalf("lookup admin: %v", err)
	}
	if !password.Verify("first-one-time-password", got.PasswordHash) {
		t.Error("the first minted password no longer verifies — the second call overwrote it")
	}
}

// TestResetSentinelAdminPassword_RemintsOnNextBoot pins the
// "xchats reset-admin-password" recovery path end to end: reset blanks the
// hash and re-sets must_change_password, and the NEXT
// BootstrapSentinelAdminPassword call (the next boot) mints again.
func TestResetSentinelAdminPassword_RemintsOnNextBoot(t *testing.T) {
	st, _ := dbtest.Open(t)
	ctx := context.Background()

	hash1, _ := password.Hash("original-one-time-password")
	if minted, err := st.BootstrapSentinelAdminPassword(ctx, hash1); err != nil || !minted {
		t.Fatalf("initial mint: minted=%v err=%v", minted, err)
	}

	if err := st.ResetSentinelAdminPassword(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}
	got, err := st.UserByEmail(ctx, "admin@xchat.kz")
	if err != nil {
		t.Fatalf("lookup admin: %v", err)
	}
	if got.PasswordHash != "" {
		t.Errorf("password_hash after reset = %q, want \"\"", got.PasswordHash)
	}
	if !got.MustChangePassword {
		t.Error("must_change_password after reset = false, want true")
	}

	hash2, _ := password.Hash("re-minted-one-time-password")
	minted, err := st.BootstrapSentinelAdminPassword(ctx, hash2)
	if err != nil {
		t.Fatalf("re-mint after reset: %v", err)
	}
	if !minted {
		t.Error("re-mint after reset: minted = false, want true")
	}
	got, err = st.UserByEmail(ctx, "admin@xchat.kz")
	if err != nil {
		t.Fatalf("lookup admin: %v", err)
	}
	if !password.Verify("re-minted-one-time-password", got.PasswordHash) {
		t.Error("re-minted password does not verify")
	}
}

// TestSetUserPassword_ClearsMustChangeFlag proves the RUNTIME password
// change path (internal/httpapi's handleChangePassword) differs from the
// boot-time bootstrap path above in exactly the one way that matters: it
// clears must_change_password, since a user who just chose their own
// password has satisfied the forced-change requirement.
func TestSetUserPassword_ClearsMustChangeFlag(t *testing.T) {
	st, _ := dbtest.Open(t)
	ctx := context.Background()

	hash1, _ := password.Hash("one-time-password")
	if _, err := st.BootstrapSentinelAdminPassword(ctx, hash1); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	newHash, _ := password.Hash("a-real-chosen-password")
	if err := st.SetUserPassword(ctx, sentinelAdminID, newHash); err != nil {
		t.Fatalf("SetUserPassword: %v", err)
	}
	got, err := st.UserByEmail(ctx, "admin@xchat.kz")
	if err != nil {
		t.Fatalf("lookup admin: %v", err)
	}
	if got.MustChangePassword {
		t.Error("must_change_password after SetUserPassword = true, want false")
	}
	if !password.Verify("a-real-chosen-password", got.PasswordHash) {
		t.Error("stored hash does not verify against the new password")
	}
}

// TestDeleteOtherSessions_KeepsOnlyTheGivenOne proves the session-eviction
// half of a password change: every session for the user except the one
// explicitly kept is gone; a different user's session is untouched.
func TestDeleteOtherSessions_KeepsOnlyTheGivenOne(t *testing.T) {
	st, _ := dbtest.Open(t)
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "delete-sessions-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	hash, _ := password.Hash("password123")
	userA, err := st.CreateUser(ctx, org.ID, "a@xchats.test", hash, "A", "member")
	if err != nil {
		t.Fatalf("create user A: %v", err)
	}
	userB, err := st.CreateUser(ctx, org.ID, "b@xchats.test", hash, "B", "member")
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}

	sessions := map[string]uuid.UUID{
		"a-keep": userA.ID, "a-evict-1": userA.ID, "a-evict-2": userA.ID, "b-untouched": userB.ID,
	}
	for id, uid := range sessions {
		if err := st.CreateSession(ctx, id, uid, 24*time.Hour); err != nil {
			t.Fatalf("create session %s: %v", id, err)
		}
	}

	if err := st.DeleteOtherSessions(ctx, userA.ID, "a-keep"); err != nil {
		t.Fatalf("DeleteOtherSessions: %v", err)
	}

	if _, err := st.UserForSession(ctx, "a-keep"); err != nil {
		t.Errorf("kept session a-keep: %v, want it to still resolve", err)
	}
	for _, evicted := range []string{"a-evict-1", "a-evict-2"} {
		if _, err := st.UserForSession(ctx, evicted); err == nil {
			t.Errorf("session %s still resolves, want it evicted", evicted)
		}
	}
	if _, err := st.UserForSession(ctx, "b-untouched"); err != nil {
		t.Errorf("user B's unrelated session: %v, want it untouched", err)
	}
}
