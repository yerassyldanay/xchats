// Package migrations selects the embedded, timestamp-versioned SQL for a database.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed sqlite/*.sql postgres/*.sql
var files embed.FS

// ForDialect returns the migration directory. Unsupported dialects fail when
// the runner reads it; callers use dbx.DB.Dialect(), never user-provided paths.
func ForDialect(dialect string) fs.FS {
	if dialect != "sqlite" && dialect != "postgres" {
		panic("unsupported migration dialect: " + dialect)
	}
	f, err := fs.Sub(files, dialect)
	if err != nil {
		panic(err)
	}
	return f
}
