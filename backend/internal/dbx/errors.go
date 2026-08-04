package dbx

import (
	"errors"

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
)

// IsUniqueViolation reports whether err is a uniqueness-constraint failure
// (a PRIMARY KEY or UNIQUE index collision) — the SQLite equivalent of pgx's
// "23505" check. Repositories translate this into domain.ErrDuplicate at
// their exported boundary; consumers never see a driver-specific error.
func IsUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code()
	return code == sqliteConstraintPrimaryKey || code == sqliteConstraintUnique
}
