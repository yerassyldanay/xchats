package dbtest

import (
	"sort"
	"strings"
	"testing"
)

// The linter in sql_linter_test.go decides what ships to both engines, so it is itself
// tested: every rule has a fixture that must trip exactly that rule, and a set of
// realistic portable statements must pass untouched. A rule that stops firing, or starts
// firing on good SQL, fails here before it can pass or block real code.

// rulesOf names the distinct rules a result violates, sorted and comma-joined.
func rulesOf(r lintResult) string {
	seen := map[string]bool{}
	for _, f := range r.Findings {
		seen[f.Rule] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func describeFindings(r lintResult) string {
	if len(r.Findings) == 0 {
		return "(no findings)"
	}
	lines := make([]string, len(r.Findings))
	for i, f := range r.Findings {
		lines[i] = "  " + f.String()
	}
	return strings.Join(lines, "\n")
}

func TestSQLLintRulesCatchViolations(t *testing.T) {
	const (
		mig = lintMigration
		stm = lintStatement
		frg = lintFragment
	)
	tests := []struct {
		name string
		kind lintKind
		sql  string
		want string // the exact set of rule ids, sorted: a fixture must not pass by tripping another rule
	}{
		// E1: engine-specific tokens.
		{"E1 jsonb type", stm, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, doc JSONB NOT NULL)`, "E1"},
		{"E1 timestamptz type", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, at TIMESTAMPTZ NOT NULL)`, "E1"},
		{"E1 citext type", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, email CITEXT NOT NULL UNIQUE)`, "E1"},
		{"E1 json text operator", stm, `SELECT doc ->> 'k' FROM t WHERE id = $1`, "E1"},
		{"E1 json operator", stm, `SELECT doc -> 'k' FROM t WHERE id = $1`, "E1"},
		{"E1 xchats_now", stm, `UPDATE t SET updated_at = xchats_now() WHERE id = $1`, "E1"},
		{"E1 template macro", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, at {{timestamp}} NOT NULL)`, "E1,S1"},
		{"E1 for update", stm, `SELECT id FROM t WHERE id = $1 FOR UPDATE`, "E1"},
		{"E1 for update skip locked", stm, "SELECT id FROM t WHERE state = $1\nFOR\tUPDATE SKIP LOCKED", "E1"},
		{"E1 collate", stm, `SELECT id FROM t ORDER BY name COLLATE "C"`, "E1"},
		{"E1 ilike", stm, `SELECT id FROM t WHERE name ILIKE $1`, "E1"},
		{"E1 pragma", stm, `PRAGMA foreign_keys = ON`, "E1,S1"},
		{"E1 cast operator", stm, `SELECT $1::text`, "E1"},
		{"E1 gen_random_uuid", stm, `INSERT INTO t (id) VALUES (gen_random_uuid())`, "E1"},
		{"E1 now", stm, `SELECT id FROM t WHERE at < now()`, "E1"},
		{"E1 ifnull", stm, `SELECT ifnull(a, 0) FROM t WHERE b = $1`, "E1"},
		{"E1 json function", stm, `SELECT json_extract(doc, '$.k') FROM t WHERE id = $1`, "E1"},
		{"E1 create function", mig, `CREATE FUNCTION f() RETURNS trigger AS $$ BEGIN RETURN NEW; END $$ LANGUAGE plpgsql`, "E1,M1"},
		{"E1 create extension", mig, `CREATE EXTENSION IF NOT EXISTS citext`, "E1,M1"},

		// S1: PostgreSQL's own grammar must accept the text.
		{"S1 typo", stm, `SELEC id FROM t`, "S1"},
		{"S1 unterminated", mig, `CREATE TABLE t (id TEXT`, "S1"},
		{"S1 two statements with arguments", stm, `UPDATE t SET a = $1; UPDATE t SET b = $2`, "S1"},
		{"S1 sqlite-only syntax", stm, `INSERT OR REPLACE INTO t (id) VALUES ($1)`, "S1"},

		// P1: $1, $2, ... only.
		{"P1 gap", stm, `SELECT id FROM t WHERE a = $1 AND b = $3`, "P1"},
		{"P1 first placeholder is not $1", stm, `SELECT id FROM t WHERE a = $2`, "P1"},
		{"P1 question mark", stm, `SELECT id FROM t WHERE a = ?`, "P1,S1"},
		{"P1 numbered question mark", stm, `SELECT id FROM t WHERE a = ?1`, "N1,P1"}, // PostgreSQL reads ?1 as an operator applied to 1
		{"P1 colon name", stm, `SELECT id FROM t WHERE a = :name`, "P1,S1"},
		{"P1 at name", stm, `SELECT id FROM t WHERE a = @name`, "N1,P1"}, // to PostgreSQL, @name is also the abs-value operator applied to a column
		{"P1 dollar name", stm, `SELECT id FROM t WHERE a = $name`, "P1,S1"},
		{"P1 placeholder in a migration", mig, `INSERT INTO t (id) VALUES ($1)`, "P1"},

		// T1: portable column and CAST types only.
		{"T1 blob", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, b BLOB)`, "T1"},
		{"T1 uuid", mig, `CREATE TABLE t (id UUID PRIMARY KEY NOT NULL)`, "T1"},
		{"T1 timestamp", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, at TIMESTAMP NOT NULL)`, "T1"},
		{"T1 varchar", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, name VARCHAR(40))`, "T1"},
		{"T1 date", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, d DATE)`, "T1"},
		{"T1 json", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, doc JSON)`, "T1"},
		{"T1 serial", mig, `CREATE TABLE t (id SERIAL PRIMARY KEY)`, "T1"},
		{"T1 double precision", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, n DOUBLE PRECISION)`, "T1"},
		{"T1 cast", stm, `SELECT CAST(a AS UUID) FROM t WHERE b = $1`, "T1"},

		// D1: a column DEFAULT is a constant.
		{"D1 function default", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, a TEXT NOT NULL DEFAULT lower('X'))`, "D1"},
		{"D1 current_timestamp default", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, at BIGINT NOT NULL DEFAULT CURRENT_TIMESTAMP)`, "D1"},
		{"D1 expression default", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, n BIGINT NOT NULL DEFAULT 1 + 1)`, "D1"},
		{"D1 generated id", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL DEFAULT gen_random_uuid())`, "D1,E1"},

		// M1: only the statements both engines run identically.
		{"M1 alter column type", mig, `ALTER TABLE t ALTER COLUMN a TYPE BIGINT`, "M1"},
		{"M1 add constraint", mig, `ALTER TABLE t ADD CONSTRAINT c CHECK (a > 0)`, "M1"},
		{"M1 drop column", mig, `ALTER TABLE t DROP COLUMN a`, "M1"},
		{"M1 trigger", mig, `CREATE TRIGGER trg BEFORE INSERT ON t FOR EACH ROW EXECUTE FUNCTION f()`, "M1"},
		{"M1 grant", mig, `GRANT SELECT ON t TO app`, "M1"},
		{"M1 transaction control", mig, `BEGIN`, "M1"},
		{"M1 sequence", mig, `CREATE SEQUENCE s`, "M1"},
		{"M1 truncate", stm, `TRUNCATE t`, "M1"},
		{"M1 drop cascade", mig, `DROP TABLE t CASCADE`, "M1"},

		// N1: constructs only one engine has.
		{"N1 distinct on", stm, `SELECT DISTINCT ON (a) a, b FROM t WHERE c = $1`, "N1"},
		{"N1 any array", stm, `SELECT id FROM t WHERE id = ANY($1)`, "N1"},
		{"N1 array constructor", stm, `SELECT ARRAY[1, 2]`, "N1"},
		{"N1 current_timestamp", stm, `UPDATE t SET at = CURRENT_TIMESTAMP WHERE id = $1`, "N1"},
		{"N1 similar to", stm, `SELECT id FROM t WHERE name SIMILAR TO $1`, "N1"},
		{"N1 regex operator", stm, `SELECT id FROM t WHERE name ~ $1`, "N1"},
		{"N1 on conflict on constraint", stm, `INSERT INTO t (id) VALUES ($1) ON CONFLICT ON CONSTRAINT t_pkey DO NOTHING`, "N1"},
		{"N1 delete using", stm, `DELETE FROM t USING u WHERE t.id = u.id AND u.a = $1`, "N1"},
		{"N1 rollup", stm, `SELECT a, count(*) FROM t GROUP BY ROLLUP (a)`, "N1"},
		{"N1 lateral", stm, `SELECT a FROM t, LATERAL (SELECT 1) x`, "N1"},
		{"N1 identity column", mig, `CREATE TABLE t (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY)`, "N1"},
		{"N1 array column", mig, `CREATE TABLE t (id TEXT PRIMARY KEY NOT NULL, ids BIGINT[])`, "N1"},
		{"N1 gin index", mig, `CREATE INDEX i ON t USING gin (a)`, "N1"},
		{"N1 concurrent index", mig, `CREATE INDEX CONCURRENTLY i ON t (a)`, "N1"},
		{"N1 intersect all", stm, `SELECT a FROM t1 WHERE x = $1 INTERSECT ALL SELECT a FROM t2 WHERE x = $1`, "N1"},
		{"N1 nulls last in an index", mig, `CREATE INDEX i ON t (a DESC NULLS LAST)`, "N1"},
		{"N1 greatest", stm, `SELECT GREATEST(a, b) FROM t WHERE c = $1`, "N1"},
		{"N1 is distinct from", stm, `SELECT 1 FROM t WHERE a IS DISTINCT FROM $1`, "N1"},
		{"N1 create or replace view", mig, `CREATE OR REPLACE VIEW v AS SELECT 1 FROM t`, "N1"},
		{"N1 insert default keyword", stm, `INSERT INTO t (id, a) VALUES ($1, DEFAULT)`, "N1"},

		// F1: only functions both engines share.
		{"F1 unknown function", stm, `SELECT abs(a) FROM t WHERE b = $1`, "F1"},
		{"F1 two-argument max", stm, `SELECT max(a, b) FROM t WHERE c = $1`, "F1"},
		{"F1 sqlite function", stm, `SELECT typeof(a) FROM t WHERE b = $1`, "F1"},
		{"F1 postgres function", stm, `SELECT char_length(a) FROM t WHERE b = $1`, "F1"},
		{"F1 trim with characters", stm, `SELECT TRIM(BOTH 'x' FROM a) FROM t WHERE b = $1`, "F1"},
		{"F1 random", stm, `SELECT id FROM t ORDER BY random() LIMIT $1`, "F1"},

		// Fragments (pieces of a statement assembled at run time) get the token rules only.
		{"fragment json operator", frg, `doc->>'k' = $`, "E1"},
		{"fragment xchats_now", frg, `updated_at = xchats_now()`, "E1"},
		{"fragment question mark", frg, `c.status_id = ?`, "P1"},
		{"fragment engine function", frg, `string_agg(name, ',')`, "E1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSQL(lintUnit{Where: "fixture", Kind: tc.kind, Text: tc.sql})
			if rulesOf(got) != tc.want {
				t.Fatalf("rules = %q, want %q\nsql: %s\n%s", rulesOf(got), tc.want, tc.sql, describeFindings(got))
			}
			if got.Unverified {
				t.Fatalf("a static fixture is never \"unverified\"")
			}
			for _, f := range got.Findings {
				if f.Msg == "" || !strings.HasPrefix(f.Where, "fixture") {
					t.Fatalf("finding without message or location: %+v", f)
				}
			}
		})
	}
}

func TestSQLLintAcceptsPortableSQL(t *testing.T) {
	const (
		mig = lintMigration
		stm = lintStatement
		frg = lintFragment
	)
	tests := []struct {
		name string
		kind lintKind
		sql  string
	}{
		{"upsert returning", stm, `INSERT INTO crm_customers (id, organization_id, display_name, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			ON CONFLICT (organization_id, normalized_name) DO UPDATE SET updated_at = EXCLUDED.updated_at
			RETURNING id`},
		{"not exists", stm, `SELECT c.id FROM chats c WHERE c.org = $1 AND NOT EXISTS (
			SELECT 1 FROM chat_members m WHERE m.chat_id = c.id AND m.user_id = $2)`},
		{"in list", stm, `SELECT id FROM t WHERE org = $1 AND id IN ($2, $3, $4)`},
		{"lower like paging", stm, `SELECT id FROM t WHERE lower(name) LIKE $1 ORDER BY name LIMIT $2 OFFSET $3`},
		{"coalesce case cast nullif", stm, `SELECT COALESCE(a, ''), CASE WHEN b > 0 THEN 'y' ELSE 'n' END,
			CAST(NULL AS BIGINT), NULLIF(c, '') FROM t WHERE d = $1`},
		{"aggregates", stm, `SELECT status, count(*), max(updated_at), min(created_at), sum(n)
			FROM t WHERE org = $1 GROUP BY status HAVING count(*) > 1`},
		{"cte left join nulls last", stm, `WITH recent AS (SELECT id FROM t WHERE created_at > $1)
			SELECT r.id, u.email FROM recent r LEFT JOIN users u ON u.id = r.id ORDER BY r.id DESC NULLS LAST`},
		{"update", stm, `UPDATE t SET a = $1, b = NULL, updated_at = $2 WHERE id = $3 AND status = 'open'`},
		{"delete", stm, `DELETE FROM t WHERE id = $1`},
		{"keywords inside a string literal", stm, `SELECT id FROM t WHERE note = 'wait for update ->> {{x}} ? $$' AND a = $1`},
		{"comment that mentions jsonb", stm, "-- stored as jsonb on the old schema\nSELECT 1 FROM t WHERE a = $1"},
		{"select one", stm, `SELECT 1`},
		{"concatenation", stm, `SELECT count(*) FROM t WHERE x = 'a' || $1`},
		{"trim", stm, `SELECT COALESCE(NULLIF(TRIM(a || ' ' || b), ''), c) FROM t WHERE d = $1`},
		{"scalar subquery", stm, `SELECT (SELECT count(*) FROM u WHERE u.t = t.id) FROM t WHERE t.id = $1`},
		{"on conflict do nothing", stm, `INSERT INTO t (id, a) VALUES ($1, $2) ON CONFLICT DO NOTHING`},
		{"union all", stm, `SELECT a FROM t1 WHERE x = $1 UNION ALL SELECT a FROM t2 WHERE x = $1`},
		{"two statements without arguments", stm, `DELETE FROM a; DELETE FROM b`},

		{"table with every portable type", mig, `CREATE TABLE IF NOT EXISTS orgs (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			created_at BIGINT NOT NULL,
			secret BYTEA,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			attrs TEXT NOT NULL DEFAULT '{}',
			n INTEGER NOT NULL DEFAULT -1,
			ratio REAL,
			price NUMERIC,
			tiny SMALLINT,
			org_id TEXT REFERENCES orgs (id) ON DELETE CASCADE,
			CHECK (n >= -1)
		);`},
		{"indexes", mig, `CREATE UNIQUE INDEX IF NOT EXISTS orgs_name_uq ON orgs (name) WHERE active = TRUE;
			CREATE INDEX orgs_lower_idx ON orgs (lower(name));`},
		{"view", mig, `CREATE VIEW org_names AS SELECT id, name FROM orgs`},
		{"seed insert", mig, `INSERT INTO orgs (id, name, created_at) VALUES ('00000000-0000-0000-0000-000000000001', 'Default', 1791244800000) ON CONFLICT DO NOTHING;`},
		{"add column and rename", mig, `ALTER TABLE orgs ADD COLUMN slug TEXT; ALTER TABLE orgs RENAME TO organizations;`},
		{"drops", mig, `DROP VIEW IF EXISTS org_names; DROP INDEX IF EXISTS orgs_lower_idx; DROP TABLE IF EXISTS junk;`},
		{"comment-only file", mig, "-- nothing to do in this release\n"},
		{"check in and composite foreign key", mig, `CREATE TABLE t (
			a TEXT NOT NULL, b TEXT NOT NULL, status TEXT NOT NULL CHECK (status IN ('a', 'b')),
			FOREIGN KEY (a, b) REFERENCES u (a, b))`},

		{"fragment predicate", frg, `c.organization_id = $1`},
		{"fragment trailing placeholder", frg, `AND lower(c.name) LIKE $`},
		{"fragment set clause", frg, `SET a = $2, updated_at = $3 WHERE id = $1`},
		{"fragment prose about locking", frg, `waiting for update of the form`},
		{"fragment prose with braces and arrow", frg, `hello {{name}} -> ok`},
		{"fragment prose question", frg, `what? who knows`},
		{"fragment password hash", frg, `$argon2id$v=19$m=65536,t=1,p=4$eZE9z7aFgeOEeYVAUCJTxg$3x3PW6uhMxX+nhuXZZZ79JQOKAoImKMB`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSQL(lintUnit{Where: "fixture", Kind: tc.kind, Text: tc.sql})
			if len(got.Findings) != 0 || got.Unverified {
				t.Fatalf("portable SQL was rejected\nsql: %s\n%s", tc.sql, describeFindings(got))
			}
		})
	}
}

// squash makes extracted text comparable regardless of the Go source's indentation.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func unitsOf(t *testing.T, kind lintKind, files ...goSource) []lintUnit {
	t.Helper()
	all, err := extractGoSQL(files)
	if err != nil {
		t.Fatalf("extractGoSQL: %v", err)
	}
	var out []lintUnit
	for _, u := range all {
		if u.Kind == kind {
			out = append(out, u)
		}
	}
	return out
}

func statementTexts(t *testing.T, files ...goSource) []string {
	t.Helper()
	var out []string
	for _, u := range unitsOf(t, lintStatement, files...) {
		out = append(out, squash(u.Text))
	}
	return out
}

func wantTexts(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("statements:\n got  %q\n want %q", got, want)
	}
}

func TestSQLLintExtractsGoQueries(t *testing.T) {
	src := func(path, body string) goSource { return goSource{Path: path, Src: "package x\n" + body} }

	t.Run("a literal argument", func(t *testing.T) {
		got := statementTexts(t, src("internal/x/a.go", `func f(db DB, ctx C) { db.Exec(ctx, "DELETE FROM t WHERE id = $1", 1) }`))
		wantTexts(t, got, "DELETE FROM t WHERE id = $1")
	})

	t.Run("constants concatenated across files of one package only", func(t *testing.T) {
		files := []goSource{
			src("internal/x/a.go", `const base = "SELECT id FROM t "`),
			src("internal/x/b.go", "const q = base + \"WHERE a = $1\"\nfunc f(db DB, ctx C) { db.Query(ctx, q) }"),
			src("internal/y/c.go", `func g(db DB, ctx C) { db.Query(ctx, q) }`), // q belongs to package x, not y
		}
		wantTexts(t, statementTexts(t, files...), "SELECT id FROM t WHERE a = $1")
	})

	t.Run("a statement assembled in a local variable", func(t *testing.T) {
		got := statementTexts(t, src("internal/x/a.go", `
func f(db DB, ctx C, wide bool) {
	q := "SELECT id FROM t WHERE a = $1"
	if wide {
		q += " AND b = $2"
	}
	q = q + " ORDER BY id"
	db.Query(ctx, q, 1)
}`))
		wantTexts(t, got, "SELECT id FROM t WHERE a = $1 AND b = $2 ORDER BY id")
	})

	t.Run("a variable reused for two statements", func(t *testing.T) {
		got := statementTexts(t, src("internal/x/a.go", `
func f(db DB, ctx C) {
	q := "SELECT 1 FROM a"
	db.Query(ctx, q)
	q = "SELECT 1 FROM b"
	db.Query(ctx, q)
}`))
		wantTexts(t, got, "SELECT 1 FROM a", "SELECT 1 FROM b")
	})

	t.Run("function-local constants", func(t *testing.T) {
		got := statementTexts(t, src("internal/x/a.go", `
func f(db DB, ctx C) {
	const q = "SELECT 1 FROM t"
	var d = "DELETE FROM t WHERE id = $1"
	db.Exec(ctx, q)
	db.Exec(ctx, d, 1)
}`))
		wantTexts(t, got, "SELECT 1 FROM t", "DELETE FROM t WHERE id = $1")
	})

	t.Run("any callee that receives SQL counts", func(t *testing.T) {
		got := statementTexts(t, src("internal/x/a_test.go", `
func f(t T, db DB, ctx C) {
	countRows(t, db, "SELECT count(*) FROM t")
	mustExec(t, db, ctx, "INSERT INTO t (id) VALUES ($1)", 1)
}`))
		wantTexts(t, got, "SELECT count(*) FROM t", "INSERT INTO t (id) VALUES ($1)")
	})

	t.Run("literals in a slice or a table-driven struct", func(t *testing.T) {
		got := statementTexts(t, src("internal/x/a.go", `
func f(tx TX, ctx C) {
	for _, q := range []string{
		"UPDATE a SET x = $1 WHERE id = $2",
		"UPDATE b SET x = $1 WHERE id = $2",
	} {
		tx.Exec(ctx, q, 1, 2)
	}
	cases := []struct{ name, sql string }{{"c", "INSERT INTO t (id) VALUES ($1)"}}
	_ = cases
}`))
		wantTexts(t, got, "UPDATE a SET x = $1 WHERE id = $2", "UPDATE b SET x = $1 WHERE id = $2", "INSERT INTO t (id) VALUES ($1)")
	})

	t.Run("a parameter, a loop variable or a call result hides a package-level statement", func(t *testing.T) {
		files := []goSource{
			src("internal/x/a.go", "const q = \"SELECT 1 FROM t\""), // line 2
			src("internal/x/b.go", `
func f(db DB, ctx C, q string) { db.Exec(ctx, q) }
func g(db DB, ctx C, ids []string) {
	for _, q := range ids {
		db.Exec(ctx, q)
	}
}
func h(db DB, ctx C) {
	q, err := build()
	db.Exec(ctx, q)
	_ = err
}`),
		}
		units := unitsOf(t, lintStatement, files...)
		// The constant itself is linted once, where it is declared; none of the three calls resolves to it.
		if len(units) != 1 || units[0].Where != "internal/x/a.go:2" {
			t.Fatalf("units = %+v", units)
		}
	})

	t.Run("a variable rewritten by a call is unknown afterwards", func(t *testing.T) {
		units := unitsOf(t, lintStatement, src("internal/x/a.go", `
func f(db DB, ctx C) {
	q := "SELECT 1 FROM a"
	q = rewrite(q)
	db.Exec(ctx, q)
}`))
		// Only the literal itself, linted where it is written: the call sees no statement.
		if len(units) != 1 || units[0].Where != "internal/x/a.go:4" {
			t.Fatalf("units = %+v", units)
		}
	})

	t.Run("a placeholder numbered at run time is accepted", func(t *testing.T) {
		units := unitsOf(t, lintStatement, src("internal/x/a.go", `func f(db DB, ctx C, n int) { db.Query(ctx, "SELECT id FROM t WHERE a = $"+itoa(n)) }`))
		if len(units) != 1 || !units[0].Dynamic || squash(units[0].Text) != "SELECT id FROM t WHERE a = $1" {
			t.Fatalf("units = %+v", units)
		}
		if r := lintSQL(units[0]); len(r.Findings) != 0 || r.Unverified {
			t.Fatalf("a run-time numbered placeholder must pass: %s", describeFindings(r))
		}
	})

	t.Run("a dynamic where clause is filled and still parsed", func(t *testing.T) {
		units := unitsOf(t, lintStatement, src("internal/x/a.go", `
func f(db DB, ctx C, where []string) {
	q := "SELECT id FROM t WHERE " + strings.Join(where, " AND ") + " ORDER BY id"
	db.Query(ctx, q)
}`))
		if len(units) != 1 || !units[0].Dynamic || !strings.Contains(squash(units[0].Text), "WHERE 1 = 1 ORDER BY id") {
			t.Fatalf("units = %+v", units)
		}
		if r := lintSQL(units[0]); len(r.Findings) != 0 || r.Unverified {
			t.Fatalf("the filled statement must parse: %s", describeFindings(r))
		}
	})

	t.Run("a closure that builds the statement around an IN list", func(t *testing.T) {
		units := unitsOf(t, lintStatement, src("internal/x/a.go", `
func f(db DB, ctx C, ids []string) error {
	return dbx.QueryInChunks(ctx, db, nil, ids, func(list string) string {
		return "SELECT id FROM t WHERE id IN " + list
	}, scan)
}`))
		if len(units) != 1 || !units[0].Dynamic || !strings.Contains(squash(units[0].Text), "IN (NULL)") {
			t.Fatalf("units = %+v", units)
		}
		if r := lintSQL(units[0]); len(r.Findings) != 0 || r.Unverified {
			t.Fatalf("the filled statement must parse: %s", describeFindings(r))
		}
	})

	t.Run("a fragment with an engine operator is flagged", func(t *testing.T) {
		files := []goSource{src("internal/x/a.go", `
func f(db DB, ctx C, n int) {
	where := []string{}
	where = append(where, "doc->>'k' = $"+itoa(n))
	db.Query(ctx, "SELECT id FROM t WHERE "+strings.Join(where, " AND "))
}`)}
		var flagged []string
		for _, u := range unitsOf(t, lintFragment, files...) {
			if rulesOf(lintSQL(u)) == "E1" {
				flagged = append(flagged, u.Text)
			}
		}
		if len(flagged) != 1 || flagged[0] != "doc->>'k' = $" {
			t.Fatalf("flagged fragments = %q", flagged)
		}
	})

	t.Run("prose is not SQL", func(t *testing.T) {
		files := []goSource{src("internal/x/a_test.go", `
func f(t T, db DB, ctx C) {
	db.Exec(ctx, "DELETE FROM t WHERE id = $1", 1)
	t.Fatalf("what? {{name}} is unknown for update -> now")
	t.Fatalf("insert into the failing table: %v", err)
	log.Printf("update failed: %v", err)
}`)}
		wantTexts(t, statementTexts(t, files...), "DELETE FROM t WHERE id = $1")
		all, _ := extractGoSQL(files)
		for _, u := range all {
			if r := lintSQL(u); len(r.Findings) != 0 {
				t.Fatalf("%q was flagged: %s", u.Text, describeFindings(r))
			}
		}
	})

	t.Run("a file with no SQL is not scanned for tokens", func(t *testing.T) {
		all, err := extractGoSQL([]goSource{src("internal/x/a.go", `func f() string { return "jsonb citext xchats_now()" }`)})
		if err != nil || len(all) != 0 {
			t.Fatalf("units = %+v, err = %v", all, err)
		}
	})

	t.Run("an exempt file reports nothing but still declares constants", func(t *testing.T) {
		files := []goSource{
			{Path: "internal/dbx/engine.go", Exempt: true, Src: "package dbx\nconst shared = \"SELECT id FROM t\"\nconst engineOnly = \"SELECT a ->> 'x' FROM t\"\nfunc f(db DB, ctx C) { db.Query(ctx, engineOnly) }"},
			{Path: "internal/dbx/use.go", Src: "package dbx\nfunc g(db DB, ctx C) { db.Query(ctx, shared) }"},
		}
		wantTexts(t, statementTexts(t, files...), "SELECT id FROM t")
		for _, u := range unitsOf(t, lintFragment, files...) {
			if strings.Contains(u.Text, "->>") {
				t.Fatalf("a literal of the exempt file leaked: %+v", u)
			}
		}
	})

	t.Run("units say where they are", func(t *testing.T) {
		units := unitsOf(t, lintStatement, src("internal/x/a.go", "\n\nfunc f(db DB, ctx C) {\n\tdb.Exec(ctx, \"DELETE FROM t\")\n}"))
		if len(units) != 1 || units[0].Where != "internal/x/a.go:5" {
			t.Fatalf("units = %+v", units)
		}
	})

	t.Run("source that does not parse is an error", func(t *testing.T) {
		if _, err := extractGoSQL([]goSource{{Path: "internal/x/bad.go", Src: "package x\nfunc ("}}); err == nil {
			t.Fatal("expected a parse error")
		}
	})
}
