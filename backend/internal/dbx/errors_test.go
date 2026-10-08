package dbx

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIsUniqueViolation(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, email TEXT UNIQUE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, email) VALUES (1, $1)`, "a@example.com"); err != nil {
		t.Fatal(err)
	}

	t.Run("unique index violation", func(t *testing.T) {
		_, err := db.Exec(ctx, `INSERT INTO t (id, email) VALUES (2, $1)`, "a@example.com")
		if err == nil {
			t.Fatal("expected a uniqueness error")
		}
		if !IsUniqueViolation(err) {
			t.Errorf("IsUniqueViolation(%v) = false, want true", err)
		}
	})

	t.Run("primary key violation", func(t *testing.T) {
		_, err := db.Exec(ctx, `INSERT INTO t (id, email) VALUES (1, $1)`, "b@example.com")
		if err == nil {
			t.Fatal("expected a primary key error")
		}
		if !IsUniqueViolation(err) {
			t.Errorf("IsUniqueViolation(%v) = false, want true", err)
		}
	})

	t.Run("unrelated error is not a unique violation", func(t *testing.T) {
		_, err := db.Exec(ctx, `INSERT INTO t (id, email) VALUES (3, $1)`, nil)
		if err != nil {
			t.Fatalf("unexpected error inserting NULL into a nullable column: %v", err)
		}
		if IsUniqueViolation(nil) {
			t.Error("IsUniqueViolation(nil) = true, want false")
		}
		if IsUniqueViolation(errors.New("boom")) {
			t.Error("IsUniqueViolation(plain error) = true, want false")
		}
	})

	t.Run("ErrNoRows is not a unique violation", func(t *testing.T) {
		err := db.QueryRow(ctx, `SELECT id FROM t WHERE id = 999`).Scan(new(int))
		if !errors.Is(err, ErrNoRows) {
			t.Fatalf("err = %v, want ErrNoRows", err)
		}
		if IsUniqueViolation(err) {
			t.Error("IsUniqueViolation(ErrNoRows) = true, want false")
		}
	})
}

// A CHECK failure is its own class: tests use it to prove a rejection came from
// the constraint under test, not from some other error on the same statement.
func TestIsCheckViolation(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (
		id INTEGER PRIMARY KEY,
		role TEXT NOT NULL CHECK (role IN ('admin', 'member')),
		email TEXT UNIQUE
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, role, email) VALUES (1, 'admin', 'a@example.com')`); err != nil {
		t.Fatal(err)
	}

	_, err := db.Exec(ctx, `INSERT INTO t (id, role) VALUES (2, 'owner')`)
	if err == nil {
		t.Fatal("expected a CHECK failure")
	}
	if !IsCheckViolation(err) {
		t.Errorf("IsCheckViolation(%v) = false, want true", err)
	}
	if IsUniqueViolation(err) {
		t.Errorf("IsUniqueViolation(%v) = true for a CHECK failure", err)
	}
	if wrapped := fmt.Errorf("insert: %w", err); !IsCheckViolation(wrapped) {
		t.Error("IsCheckViolation does not see through a wrapped error")
	}

	_, err = db.Exec(ctx, `INSERT INTO t (id, role, email) VALUES (3, 'member', 'a@example.com')`)
	if err == nil || IsCheckViolation(err) {
		t.Errorf("a UNIQUE failure must not read as a CHECK failure (err=%v)", err)
	}
	_, err = db.Exec(ctx, `INSERT INTO t (id, role) VALUES (4, NULL)`)
	if err == nil || IsCheckViolation(err) {
		t.Errorf("a NOT NULL failure must not read as a CHECK failure (err=%v)", err)
	}
	if IsCheckViolation(nil) || IsCheckViolation(errors.New("boom")) {
		t.Error("IsCheckViolation is true for nil or a plain error")
	}
}
