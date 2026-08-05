// Package domain holds the driver-free error vocabulary the persistence
// layer (internal/store, internal/kbstore, internal/responsestore,
// internal/mcpauth) translates driver errors into at its exported boundary.
// Consumers use errors.Is against these sentinels — never a driver-specific
// error or error-code string — so the database engine underneath the
// persistence layer stays an implementation detail.
//
// This package intentionally has zero dependencies (not even on internal/dbx):
// every package in the module, on either side of the persistence boundary,
// can depend on it without pulling in a database driver.
package domain

import "errors"

var (
	// ErrNotFound is returned when a lookup matches no row.
	ErrNotFound = errors.New("not found")

	// ErrDuplicate is returned when a write would violate a uniqueness
	// constraint (the old code's pgx "23505" check, and later modernc's
	// SQLite result codes 1555/2067 — see dbx.IsUniqueViolation).
	ErrDuplicate = errors.New("duplicate")

	// ErrVersionConflict is returned when an optimistic-concurrency check
	// (an If-Match / base_version guard) fails because the row moved since
	// the caller last read it.
	ErrVersionConflict = errors.New("version conflict")
)
