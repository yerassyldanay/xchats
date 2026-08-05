package dbx

import (
	"context"
	"errors"
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
