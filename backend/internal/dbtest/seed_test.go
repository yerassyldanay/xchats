package dbtest

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

// TestInitAdminMigration pins the "Seeding & bootstrap" decision: required
// initial state comes 100% from migrations 0006_init_admin +
// 0008_bootstrap_admin, never from Go code. A fresh, freshly-migrated
// database — nothing else — must already have the default organization,
// the default admin user, and their membership link.
//
// The admin's password_hash must be BLANK and must_change_password set —
// 0008 retires 0006's committed default-password hash immediately, so
// nothing (not even the old documented default password) verifies against
// it. cmd/xchats' first-boot bootstrap is what mints a real, unique
// password on top of this — see TestBootstrapAdminPassword in
// cmd/xchats for that half.
func TestInitAdminMigration(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()

	const (
		orgID   = "00000000-0000-0000-0000-000000000001"
		adminID = "00000000-0000-0000-0000-000000000002"
		email   = "admin@xchat.kz"
		// The password 0006's precomputed hash used to open with, before
		// 0008 retired it — kept only to prove it no longer verifies.
		formerDefaultPassword = "xchat-admin-change-me"
	)

	var orgName string
	if err := db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1`, orgID).Scan(&orgName); err != nil {
		t.Fatalf("default organization missing: %v", err)
	}
	if orgName == "" {
		t.Error("default organization has an empty name")
	}

	var gotEmail, hash string
	var mustChangePassword bool
	if err := db.QueryRow(ctx, `SELECT email, password_hash, must_change_password FROM users WHERE id = $1`, adminID).
		Scan(&gotEmail, &hash, &mustChangePassword); err != nil {
		t.Fatalf("default admin user missing: %v", err)
	}
	if gotEmail != email {
		t.Errorf("admin email = %q, want %q", gotEmail, email)
	}
	if hash != "" {
		t.Errorf("sentinel admin's password_hash = %q, want \"\" (0008 must blank 0006's committed hash)", hash)
	}
	if !mustChangePassword {
		t.Error("sentinel admin's must_change_password = false, want true")
	}
	if verifyArgon2id(formerDefaultPassword, hash) {
		t.Error("the former default password still verifies — 0008 failed to retire the committed hash")
	}
	if verifyArgon2id("", hash) {
		t.Error("an empty password verified against the blanked hash — must fail closed")
	}

	var membershipCount int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM organization_users WHERE organization_id = $1 AND user_id = $2`,
		orgID, adminID).Scan(&membershipCount); err != nil {
		t.Fatal(err)
	}
	if membershipCount != 1 {
		t.Errorf("organization_users membership rows = %d, want 1", membershipCount)
	}

	var role string
	if err := db.QueryRow(ctx, `
		SELECT role FROM organization_users WHERE organization_id = $1 AND user_id = $2`,
		orgID, adminID).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "admin" {
		t.Errorf("sentinel admin's membership role = %q, want %q", role, "admin")
	}

	// Re-running migrations (the runner's own idempotency, exercised again
	// here specifically against 0006) must not create a second org/user/
	// membership row nor error on the fixed-PK re-insert.
	if err := reapplyMigrations(t, db); err != nil {
		t.Fatalf("re-running migrations: %v", err)
	}
	var orgCount, userCount int
	db.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgCount)
	db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&userCount)
	if orgCount != 1 {
		t.Errorf("organizations count after re-migration = %d, want 1", orgCount)
	}
	if userCount != 1 {
		t.Errorf("users count after re-migration = %d, want 1", userCount)
	}
}

// verifyArgon2id reimplements internal/httpapi/auth.go's VerifyPassword
// against the encoded argon2id format HashPassword produces — duplicated
// rather than imported so this package never depends on httpapi (outside
// the persistence boundary).
func verifyArgon2id(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, argonTime uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &argonTime, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, argonTime, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
