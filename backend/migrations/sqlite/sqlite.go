// Package sqlitemigrations embeds xchats' SQLite migration files so the
// binary can apply them without the files being present at runtime (one
// self-contained artifact) — the SQLite-era sibling of the frozen
// migrations/postgres (pgschema) package. internal/store's New opens the
// database and applies these via internal/dbx.RunMigrations.
package sqlitemigrations

import "embed"

// FS holds the *.up.sql migration files. schema_contract.json and
// SCHEMA.snapshot.sql live alongside them but are deliberately NOT part of
// this glob — they are Phase 0/1 build-time artifacts (the contract test's
// source of truth), not something the runtime migrator should ever see as
// a migration file.
//
//go:embed *.up.sql
var FS embed.FS

// SchemaContractJSON is schema_contract.json's raw bytes — the same
// column/type/foreign-key metadata dbtest's contract test validates the
// SQLite schema against, embedded separately from FS (see FS's doc
// comment) for internal/pgimport's runtime use: cmd/xchats-import is a
// standalone binary with no access to the source tree at run time, so it
// needs this embedded rather than reading the file by path.
//
//go:embed schema_contract.json
var SchemaContractJSON []byte
