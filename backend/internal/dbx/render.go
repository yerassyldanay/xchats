package dbx

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// sqliteUUIDv4Expr is the SQLite column-default expression behind {{uuid}}: a
// random, well-formed UUIDv4 string, matching PostgreSQL's gen_random_uuid()::text
// so "INSERT ... RETURNING id" call sites that omit id work on both engines.
// uuiddefault_test.go pins its correctness (version nibble '4', variant nibble
// one of 8/9/a/b, no duplicates) before it is trusted in DDL.
const sqliteUUIDv4Expr = `(lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))))`

var jsonColumnName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// RenderMigration expands the dialect macros of one shared migration file for
// d. Migrations are written once, as plain SQL plus these macros, and the
// runner renders them per engine before executing them:
//
//	{{uuid}}           UUIDv4 column default
//	{{now}}            UTC clock column default
//	{{timestamp}}      timestamp column type
//	{{json "col"}}     JSON column type; col is the column being declared
//	{{citext}}         case-insensitive text column type
//	{{bytes}}          binary column type
//	{{if sqlite}}…{{end}} / {{if postgres}}…{{end}}   engine-only statements
//
// The macros are frozen once a migration using them ships: an applied file is
// immutable, so changing what a macro expands to would silently diverge new
// databases from existing ones. Add a new macro instead. Literal double braces
// in SQL must be written {{"{{"}}. A file whose render is blank (everything
// inside an engine block for the other engine) is valid and a no-op.
//
// Errors name the file. name is only used for messages.
func RenderMigration(d Dialect, name, src string) (string, error) {
	if d != SQLite && d != Postgres {
		return "", fmt.Errorf("dbx: render migration %s: unsupported dialect %q", name, d)
	}
	t, err := template.New(name).Option("missingkey=error").Funcs(macroFuncs(d)).Parse(src)
	if err != nil {
		return "", fmt.Errorf("dbx: render migration %s: %w", name, err)
	}
	var out strings.Builder
	if err := t.Execute(&out, nil); err != nil {
		return "", fmt.Errorf("dbx: render migration %s: %w", name, err)
	}
	return out.String(), nil
}

func macroFuncs(d Dialect) template.FuncMap {
	pg := d == Postgres
	pick := func(sqliteSQL, postgresSQL string) func() string {
		if pg {
			return func() string { return postgresSQL }
		}
		return func() string { return sqliteSQL }
	}
	return template.FuncMap{
		"uuid":      pick(sqliteUUIDv4Expr, `(gen_random_uuid()::text)`),
		"now":       pick(`(strftime('%Y-%m-%d %H:%M:%f','now'))`, `(xchats_now())`),
		"timestamp": pick(`TEXT`, `TIMESTAMPTZ`),
		"citext":    pick(`TEXT COLLATE NOCASE`, `CITEXT`),
		"bytes":     pick(`BLOB`, `BYTEA`),
		"json": func(col string) (string, error) {
			if !jsonColumnName.MatchString(col) {
				return "", fmt.Errorf("json: invalid column name %q (use lower_snake_case)", col)
			}
			if pg {
				return "JSONB", nil
			}
			return "TEXT CHECK (" + col + " IS NULL OR json_valid(" + col + "))", nil
		},
		"sqlite":   func() bool { return !pg },
		"postgres": func() bool { return pg },
	}
}
