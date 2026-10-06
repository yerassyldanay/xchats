package dbtest

import (
	"io/fs"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

var (
	createTableRE = regexp.MustCompile(`(?m)^CREATE TABLE IF NOT EXISTS (\w+) \(`)
	createIndexRE = regexp.MustCompile(`(?m)^CREATE (?:UNIQUE )?INDEX IF NOT EXISTS (\w+)`)
	createViewRE  = regexp.MustCompile(`(?m)^CREATE VIEW (\w+) AS`)
	// {{json "col"}} renders on SQLite as `TEXT CHECK (col IS NULL OR json_valid(col))`;
	// a check that names another column would silently validate the wrong one.
	jsonCheckRE = regexp.MustCompile(`(\w+)\s+TEXT CHECK \((\w+) IS NULL OR json_valid\((\w+)\)\)`)
)

func names(re *regexp.Regexp, sql string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(sql, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestEmbeddedMigrationsRenderForBothDialects renders every shared file for both
// engines and checks that they describe the same objects, with no macro left
// behind and every JSON check naming the column it is declared on.
func TestEmbeddedMigrationsRenderForBothDialects(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		src, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		t.Run(e.Name(), func(t *testing.T) {
			rendered := map[dbx.Dialect]string{}
			for _, d := range []dbx.Dialect{dbx.SQLite, dbx.Postgres} {
				out, err := dbx.RenderMigration(d, e.Name(), string(src))
				if err != nil {
					t.Fatalf("%s: %v", d, err)
				}
				if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
					t.Errorf("%s: macro left in rendered SQL", d)
				}
				rendered[d] = out
			}
			sqlite, postgres := rendered[dbx.SQLite], rendered[dbx.Postgres]
			for what, re := range map[string]*regexp.Regexp{"tables": createTableRE, "indexes": createIndexRE, "views": createViewRE} {
				if a, b := names(re, sqlite), names(re, postgres); !reflect.DeepEqual(a, b) {
					t.Errorf("%s differ between engines:\n sqlite   %v\n postgres %v", what, a, b)
				}
			}
			for _, m := range jsonCheckRE.FindAllStringSubmatch(sqlite, -1) {
				if m[1] != m[2] || m[2] != m[3] {
					t.Errorf("JSON check names %q, %q and %q; all three must be the declared column", m[1], m[2], m[3])
				}
			}
			if strings.Contains(sqlite, "JSONB") || strings.Contains(sqlite, "TIMESTAMPTZ") || strings.Contains(sqlite, "gen_random_uuid") {
				t.Error("PostgreSQL-only syntax leaked into the SQLite render")
			}
			if strings.Contains(postgres, "json_valid") || strings.Contains(postgres, "randomblob") || strings.Contains(postgres, "strftime") {
				t.Error("SQLite-only syntax leaked into the PostgreSQL render")
			}
		})
	}
}
