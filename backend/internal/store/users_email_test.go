package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/domain"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// users.email is plain TEXT UNIQUE: no CITEXT, no COLLATE NOCASE. Case-insensitive
// identity is the store's job on every engine, so every write stores the
// lower-cased, trimmed address and every lookup normalises its input the same way.
func TestUserEmailsAreNormalisedInGo(t *testing.T) {
	st, db := dbtest.Open(t)
	ctx := context.Background()

	org := uuid.MustParse("00000000-0000-0000-0000-000000000001") // the seeded default organization

	created, err := st.CreateUser(ctx, org, "  Alice.Smith@Example.COM ", "hash", "Alice", "member")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.Email != "alice.smith@example.com" {
		t.Errorf("returned email = %q, want it lower-cased and trimmed", created.Email)
	}
	var stored string
	if err := db.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, created.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "alice.smith@example.com" {
		t.Errorf("stored email = %q, want alice.smith@example.com", stored)
	}

	for _, probe := range []string{"alice.smith@example.com", "ALICE.SMITH@EXAMPLE.COM", " Alice.Smith@example.com\t"} {
		got, err := st.UserByEmail(ctx, probe)
		if err != nil || got.ID != created.ID {
			t.Errorf("UserByEmail(%q) = %v, %v; want user %s", probe, got.ID, err, created.ID)
		}
	}

	// The same address in another case is the same account, not a second one.
	if _, err := st.CreateUser(ctx, org, "ALICE.SMITH@example.com", "hash", "Dup", "member"); !errors.Is(err, domain.ErrDuplicate) {
		t.Errorf("CreateUser with a case variant: err = %v, want domain.ErrDuplicate", err)
	}

	// SeedUser upserts by address: a case variant updates the existing row.
	seeded, err := st.SeedUser(ctx, org, "Alice.Smith@EXAMPLE.com", "new-hash", "Alice")
	if err != nil {
		t.Fatalf("SeedUser: %v", err)
	}
	if seeded.ID != created.ID {
		t.Errorf("SeedUser created user %s, want the existing %s", seeded.ID, created.ID)
	}
	if _, err := st.UserByEmail(ctx, "nobody@example.com"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("UserByEmail(unknown) err = %v, want ErrNotFound", err)
	}
}
