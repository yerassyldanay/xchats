// Package migrations embeds the timestamp-versioned SQL shared by SQLite and PostgreSQL.
package migrations

import "embed"

// FS holds every migration, one file per change at its root. A file is plain SQL plus
// the dialect macros that internal/dbx renders for the open engine; see docs/database.md.
//
//go:embed *.sql
var FS embed.FS
