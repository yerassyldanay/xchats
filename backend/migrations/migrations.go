// Package migrations embeds the timestamp-versioned SQL executed verbatim by SQLite and PostgreSQL.
package migrations

import "embed"

// FS holds every migration, one file per change at its root. A file is plain SQL, the
// same bytes on both engines: no templates, no per-engine variants, no extensions or
// custom functions. The portable types and rules are in docs/database.md.
//
//go:embed *.sql
var FS embed.FS
