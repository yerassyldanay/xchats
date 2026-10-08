package dbx

// Dialect names the engine behind a *DB. It exists for the few places that must
// know which engine they are on — file backup is SQLite-only, and the write lock
// is taken differently — never for SQL text: every statement is the same bytes
// on both engines.
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
