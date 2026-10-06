package dbx

import (
	"strings"
	"testing"
)

const renderTestFile = "20260101000000_x.sql"

func TestRenderMigrationMacros(t *testing.T) {
	cases := []struct {
		name, src, sqlite, postgres string
	}{
		{"uuid", `{{uuid}}`, sqliteUUIDv4Expr, `(gen_random_uuid()::text)`},
		{"now", `{{now}}`, `(strftime('%Y-%m-%d %H:%M:%f','now'))`, `(xchats_now())`},
		{"timestamp", `{{timestamp}}`, `TEXT`, `TIMESTAMPTZ`},
		{"json", `{{json "meta"}}`, `TEXT CHECK (meta IS NULL OR json_valid(meta))`, `JSONB`},
		{"citext", `{{citext}}`, `TEXT COLLATE NOCASE`, `CITEXT`},
		{"bytes", `{{bytes}}`, `BLOB`, `BYTEA`},
		{"engine blocks", `{{if sqlite}}S{{end}}{{if postgres}}P{{end}}`, `S`, `P`},
		{"sqlite-only block", `{{if sqlite}}PRAGMA x;{{end}}`, `PRAGMA x;`, ``},
		{"postgres-only block", `{{if postgres}}CREATE EXTENSION IF NOT EXISTS citext;{{end}}`, ``, `CREATE EXTENSION IF NOT EXISTS citext;`},
		{
			"inside a column definition",
			`created_at {{timestamp}} NOT NULL DEFAULT {{now}}`,
			`created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))`,
			`created_at TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for d, want := range map[Dialect]string{SQLite: tc.sqlite, Postgres: tc.postgres} {
				got, err := RenderMigration(d, renderTestFile, tc.src)
				if err != nil {
					t.Fatalf("%s: %v", d, err)
				}
				if got != want {
					t.Errorf("%s: rendered %q, want %q", d, got, want)
				}
			}
		})
	}
}

func TestRenderMigrationPlainSQLUnchanged(t *testing.T) {
	const plain = "-- plain SQL shared by both dialects { not a macro }\n" +
		"CREATE TABLE IF NOT EXISTS t (\n" +
		"    meta TEXT NOT NULL DEFAULT '{}',\n" +
		"    list TEXT NOT NULL DEFAULT '[]',\n" +
		"    nested TEXT NOT NULL DEFAULT '{\"a\":{\"b\":1}}'\n" +
		");\n" +
		"CREATE FUNCTION f(text) RETURNS text AS $$ SELECT lower($1) $$ LANGUAGE sql;\n" +
		"-- a trailing comment with 100% and 'quotes'\n"
	for _, d := range []Dialect{SQLite, Postgres} {
		got, err := RenderMigration(d, renderTestFile, plain)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if got != plain {
			t.Errorf("%s: plain SQL changed:\n got %q\nwant %q", d, got, plain)
		}
	}
}

func TestRenderMigrationErrors(t *testing.T) {
	cases := []struct {
		name    string
		d       Dialect
		src     string
		wantErr string
	}{
		{"unknown macro", SQLite, `{{nope}}`, "nope"},
		{"json column with uppercase and dash", SQLite, `{{json "Bad-Col"}}`, "Bad-Col"},
		{"json without a column", SQLite, `{{json}}`, "json"},
		{"json with two columns", Postgres, `{{json "a" "b"}}`, "json"},
		{"unclosed action", Postgres, `{{uuid`, "unclosed"},
		{"unsupported dialect", Dialect("mysql"), `SELECT 1`, "mysql"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderMigration(tc.d, renderTestFile, tc.src)
			if err == nil {
				t.Fatalf("rendered %q, want an error", got)
			}
			if !strings.Contains(err.Error(), renderTestFile) {
				t.Errorf("error %q does not name the file %s", err, renderTestFile)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

// Literal double braces in SQL are written {{"{{"}}; RenderMigration's doc
// comment promises this escape, so it is pinned here.
func TestRenderMigrationEscapesLiteralBraces(t *testing.T) {
	for _, d := range []Dialect{SQLite, Postgres} {
		got, err := RenderMigration(d, renderTestFile, `SELECT '{{"{{"}}x';`)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if want := `SELECT '{{x';`; got != want {
			t.Errorf("%s: rendered %q, want %q", d, got, want)
		}
	}
}
