// Package pgschema embeds xchats' frozen PostgreSQL migration history
// (0001-0016, applied through the SQLite cutover). Nothing in the packaged
// xchats binary runs these anymore — internal/store now migrates a SQLite
// database from migrations/sqlite instead. This FS exists so
// cmd/xchats-import (the standalone pgx-linked import tool) and anyone
// standing up a throwaway Postgres to compare against SCHEMA.snapshot.sql
// have the exact historical DDL without digging through git history.
package pgschema

import "embed"

// FS holds the frozen *.up.sql / *.down.sql migration files.
//
//go:embed *.up.sql *.down.sql
var FS embed.FS
