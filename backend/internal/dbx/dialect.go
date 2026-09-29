package dbx

import (
	"database/sql/driver"
	sqlite "modernc.org/sqlite"
	"time"
)

// Dialect selects SQL syntax only at the persistence boundary.
type Dialect string

const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
)

func (db *DB) Dialect() Dialect {
	if db.dialect == "" {
		return SQLite
	}
	return db.dialect
}
func (db *DB) bind(args []any) []any { return bindDialect(db.Dialect(), args) }
func bindDialect(d Dialect, args []any) []any {
	if d == Postgres {
		return args
	}
	return bindArgs(args)
}

// JSONValues returns a table expression with a TEXT value column. argument
// must be a trusted SQL placeholder, never user data. Bind a JSON array.
func (db *DB) JSONValues(argument string) string {
	if db.Dialect() == Postgres {
		return "(SELECT jsonb_array_elements_text(CAST(" + argument + " AS jsonb)) AS value)"
	}
	return "json_each(" + argument + ")"
}

func init() {
	// xchats_now is the shared SQL clock: UTC milliseconds in SQLite,
	// TIMESTAMPTZ in PostgreSQL (defined by the baseline).
	if err := sqlite.RegisterScalarFunction("xchats_now", 0, func(_ *sqlite.FunctionContext, _ []driver.Value) (driver.Value, error) {
		return FormatTime(time.Now()), nil
	}); err != nil {
		panic(err)
	}
}

// JSONMergePatch implements RFC 7396 object merge/delete semantics. Arguments
// are trusted SQL expressions; values must still be bound as parameters.
func (db *DB) JSONMergePatch(target, patch string) string {
	if db.Dialect() == Postgres {
		return "xchats_json_merge_patch(" + target + ", CAST(" + patch + " AS jsonb))"
	}
	return "json_patch(" + target + ", " + patch + ")"
}

// SkipLocked is appended to a candidate SELECT inside an atomic UPDATE.
// SQLite serializes the UPDATE; PostgreSQL locks only the selected rows.
func (db *DB) SkipLocked() string {
	if db.Dialect() == Postgres {
		return " FOR UPDATE SKIP LOCKED"
	}
	return ""
}
