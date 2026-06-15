// Package migrations embeds the SQL migration files so the binary can apply them
// without the files being present at runtime (one self-contained artifact).
package migrations

import "embed"

// FS holds the *.sql migration files.
//
//go:embed *.sql
var FS embed.FS
