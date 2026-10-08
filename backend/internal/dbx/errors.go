package dbx

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"

	sqlite "modernc.org/sqlite"
)

// SQLite extended result codes for constraint violations that mean "this
// write collided with a uniqueness constraint" — the two cases the old pgx
// "23505" string match (isUniqueViolation, internal/httpapi/auth.go) needs
// to keep detecting after the cutover. Both are SQLITE_CONSTRAINT (19)
// combined with an extended cause: PRIMARYKEY (19 | 6<<8 = 1555) and UNIQUE
// (19 | 8<<8 = 2067). See https://www.sqlite.org/rescode.html#constraint_unique.
const (
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintUnique     = 2067
	// SQLITE_CONSTRAINT_CHECK: SQLITE_CONSTRAINT (19) | 1<<8.
	sqliteConstraintCheck = 275
)

// IsUniqueViolation reports whether err is a uniqueness-constraint failure
// (a PRIMARY KEY or UNIQUE index collision) — the SQLite equivalent of pgx's
// "23505" check. Repositories translate this into domain.ErrDuplicate at
// their exported boundary; consumers never see a driver-specific error.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code()
	return code == sqliteConstraintPrimaryKey || code == sqliteConstraintUnique
}

// IsCheckViolation reports whether err is a CHECK-constraint failure — PostgreSQL
// SQLSTATE 23514, SQLite SQLITE_CONSTRAINT_CHECK. Tests use it to prove that a
// rejected write was rejected by the CHECK under test and not by some other
// error on the same statement.
func IsCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23514"
	}
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteConstraintCheck
}
