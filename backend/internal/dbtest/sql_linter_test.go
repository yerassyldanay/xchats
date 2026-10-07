package dbtest

// SQL portability linter.
//
// xchats ships ONE set of SQL — the migrations and every repository query — and runs it
// byte for byte on SQLite (the default) and on PostgreSQL. Nothing is rendered per engine,
// so a single engine-specific token is a bug on the other engine that no test of the first
// one can see. This file is the guard: it runs inside `go test` (and `make lint-sql`, which
// `make lint-backend` runs first), needs no database and opens no connection, and fails on
// the first SQL that would not run identically on both.
//
// What is scanned
//
//   - every file of migrations.FS (the bytes that ship), as a whole file;
//   - every .go file under internal/ and cmd/, tests included: any string expression whose
//     constant-folded text starts like a SQL statement is a statement, wherever it goes
//     (Query, QueryRow, Exec, a test helper, a closure handed to dbx.QueryInChunks, a slice
//     of statements). Text that Go only knows at run time (a joined WHERE, an IN list, a
//     table name) is replaced by a neutral filler so the rest can still be parsed.
//   - in a file that holds SQL, every other string literal is a fragment: it may be a piece
//     of a statement assembled at run time, so only the token rules look at it.
//
// Only string literals are read, never comments, so a comment may say "jsonb".
//
// Rules (the id appears in every finding):
//
//	E1  engine-specific token   JSONB, TIMESTAMPTZ, CITEXT, ->>, ->, xchats_now, {{...}}, FOR UPDATE,
//	                            COLLATE, ILIKE, ::, PRAGMA, CREATE FUNCTION/EXTENSION, json_*() ... (engineTokens)
//	S1  syntax                  the text must parse with PostgreSQL's own grammar (libpg_query, in memory);
//	                            a statement that has placeholders must be exactly one statement
//	P1  placeholders            $1, $2, ... only and gapless; never ?, ?N, :name, @name, $name;
//	                            a migration has none
//	T1  types                   text, bigint, integer, smallint, boolean, bytea, numeric, real
//	D1  defaults                a column DEFAULT is a constant: the repository supplies ids and times
//	M1  statement kinds         SELECT/INSERT/UPDATE/DELETE and the DDL both engines run identically
//	N1  engine-only constructs  DISTINCT ON, = ANY(...), ARRAY[...], CURRENT_TIMESTAMP, ROLLUP, LATERAL,
//	                            identity columns, operators other than = <> < <= > >= + - * / % ||,
//	                            LIKE on anything but lower(column) (it is case-insensitive on SQLite only) ...
//	F1  functions               count, sum, min, max, lower (one argument each); anything else is
//	                            unverified on one engine or the other
//
// E1 is the keyword blacklist; S1 uses the parser as an oracle for "is this SQL at all"; the
// tree rules (T1, D1, M1, N1, F1) read PostgreSQL's parse tree for what no blacklist can
// enumerate. The rule sets do not overlap: where E1 names a token, the tree rules stay silent
// about it, so a finding always points at one thing.
//
// libpg_query is PostgreSQL's real parser. It is used through github.com/wasilibs/go-pgquery,
// the same C library built as WebAssembly on the pure-Go wazero runtime and returning the same
// github.com/pganalyze/pg_query_go/v6 parse tree: pg_query_go's own Parse needs cgo, which
// would make this guard vanish from `CGO_ENABLED=0 go test`.
//
// Accepting a new construct: add it to the self-tests first (sql_linter_selftest_test.go),
// then to the allowlists below, only after running it on both engines. Exempting code that is
// engine-specific by design goes in sqlLintExempt, with a reason.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	pgquery "github.com/wasilibs/go-pgquery"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yerassyldanay/xchats/backend/migrations"
)

type lintKind int

const (
	// lintMigration is a whole .sql file: any number of statements, run without arguments.
	lintMigration lintKind = iota
	// lintStatement is SQL a repository (or a test) hands to the database.
	lintStatement
	// lintFragment is a string literal that may be part of a statement assembled at run time:
	// it cannot be parsed alone, so only the token rules look at it.
	lintFragment
)

type lintUnit struct {
	Where   string // file, or file:line of the call
	Kind    lintKind
	Text    string
	Dynamic bool // part of the statement is only known at run time; Text holds fillers there
}

type lintFinding struct {
	Rule  string
	Where string
	Msg   string
}

func (f lintFinding) String() string { return fmt.Sprintf("%s: [%s] %s", f.Where, f.Rule, f.Msg) }

type lintResult struct {
	Findings []lintFinding
	// Unverified is a dynamic statement the parser rejected. A filler can be the reason, so it is
	// not a failure, but sqlLintMaxUnverified keeps the number from growing unnoticed.
	Unverified bool
}

// ---------------------------------------------------------------------------------------
// E1: tokens
// ---------------------------------------------------------------------------------------

type engineToken struct {
	name        string
	re          *regexp.Regexp
	hint        string
	inCode      bool // statements and migrations
	inFragments bool // string literals that may be part of a statement: only text that is never prose
	call        bool // the match is a function call: the finding names the function
}

func newToken(name, pattern, hint string, inCode, inFragments bool) engineToken {
	return engineToken{name: name, re: regexp.MustCompile(`(?i)` + pattern), hint: hint, inCode: inCode, inFragments: inFragments}
}

func callToken(name, pattern, hint string, inCode, inFragments bool) engineToken {
	t := newToken(name, pattern, hint, inCode, inFragments)
	t.call = true
	return t
}

// engineFuncs are functions that exist on one engine only (or that the macro schema defined).
// Each has its own message; any other function outside portableFuncs is reported by F1.
var engineFuncs = []struct {
	name      string
	hint      string
	fragments bool // distinctive enough to check in any string literal
}{
	{"xchats_now", "bind a Go-supplied UTC Unix millisecond value", true},
	{"unicode_lower", "use lower(); dbx makes it Unicode-aware on SQLite", true},
	{"xchats_json_merge_patch", "read, change and write the JSON document in Go", true},
	{"gen_random_uuid", "bind uuid.New() from Go", true},
	{"uuid_generate_v4", "bind uuid.New() from Go", true},
	{"randomblob", "bind uuid.New() or random bytes from Go", true},
	{"strftime", "bind a value formatted in Go", true},
	{"julianday", "bind a value computed in Go", true},
	{"datetime", "bind a UTC Unix millisecond value from Go", false},
	{"now", "bind a UTC Unix millisecond value from Go", false},
	{"ifnull", "use COALESCE", true},
	{"iif", "use CASE WHEN", true},
	{"date_trunc", "compute the boundary in Go and bind it", true},
	{"to_char", "format in Go", true},
	{"string_agg", "aggregate in Go", true},
	{"group_concat", "aggregate in Go", true},
	{"generate_series", "generate the values in Go", true},
	{"unnest", "expand the list in Go and use IN ($1, $2, ...)", true},
}

var engineFuncSet = func() map[string]bool {
	m := map[string]bool{}
	for _, f := range engineFuncs {
		m[f.name] = true
	}
	return m
}()

// funcTokenName names the tokens built from engineFuncs.
const funcTokenName = "engine-specific function"

var engineTokens = func() []engineToken {
	toks := []engineToken{
		newToken("JSONB", `\bjsonb\b`, "PostgreSQL-only type; keep JSON in a TEXT column and read or merge it in Go", true, true),
		newToken("TIMESTAMPTZ", `\btimestamptz\b`, "PostgreSQL-only type; store time as BIGINT UTC Unix milliseconds", true, true),
		newToken("CITEXT", `\bcitext\b`, "needs CREATE EXTENSION; use TEXT and normalise in Go", true, true),
		newToken("->>", `->>`, "JSON operator; read the JSON column in Go", true, true),
		newToken("->", `->($|[^>])`, "JSON operator; read the JSON column in Go", true, false),
		newToken("{{ }}", `\{\{|\}\}`, "template macro; SQL is plain text, identical on both engines", true, false),
		newToken("FOR UPDATE", `\bfor\s+(?:update|share|no\s+key\s+update|key\s+share)\b`, "row locks differ per engine; dbx.Begin already serialises writers", true, false),
		newToken("SKIP LOCKED", `\b(?:skip\s+locked|nowait)\b`, "lock hints differ per engine; claim rows with a compare-and-swap UPDATE", true, false),
		newToken("COLLATE", `\bcollate\b`, "collations differ per engine; compare lower() values", true, false),
		newToken("ILIKE", `\bilike\b`, "PostgreSQL-only; use lower(x) LIKE lower(y)", true, false),
		newToken("::", `::`, "PostgreSQL cast syntax; use CAST(x AS type)", true, false),
		newToken("PRAGMA", `\bpragma\b`, "SQLite-only; engine settings belong in internal/dbx", true, false),
		newToken("$$", `\$\$`, "PostgreSQL dollar quoting", true, false),
		newToken("CREATE FUNCTION", `\bcreate\s+(?:or\s+replace\s+)?(?:function|extension)\b`, "no custom functions or extensions: PostgreSQL must need no superuser", true, false),
		callToken("json function", `\bjsonb?_[a-z_]+\s*\(`, "JSON functions differ per engine; handle JSON in Go", true, true),
		// A fragment such as "c.ref LIKE $" is half of a statement the tree rules never see; a column
		// (not a closing parenthesis) right before LIKE is the bare-column case N1 reports in a whole statement.
		newToken("LIKE on a bare column", `[\w."]\s+like\s+\$`, "SQLite's LIKE ignores ASCII case and PostgreSQL's does not: write lower(column) LIKE a pattern lowered in Go", false, true),
		// In a whole statement the parse tree reports these precisely; in a fragment it cannot.
		newToken("CURRENT_TIMESTAMP", `\bcurrent_(?:timestamp|date|time)\b`, "bind a UTC Unix millisecond value from Go", false, true),
	}
	var all, safe []string
	for _, f := range engineFuncs {
		all = append(all, f.name)
		if f.fragments {
			safe = append(safe, f.name)
		}
	}
	return append(toks,
		callToken(funcTokenName, `\b(?:`+strings.Join(all, "|")+`)\s*\(`, "", true, false),
		callToken(funcTokenName, `\b(?:`+strings.Join(safe, "|")+`)\s*\(`, "", false, true),
	)
}()

// matchedFunc extracts the function name from a match of the engine-specific function token.
func matchedFunc(match string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(match), "(")))
}

func engineFuncHint(name string) string {
	for _, f := range engineFuncs {
		if f.name == name {
			return f.hint
		}
	}
	return "see engineFuncs"
}

// ---------------------------------------------------------------------------------------
// P1: placeholders
// ---------------------------------------------------------------------------------------

var (
	// rePlaceholderStyle finds every placeholder style that is not $N, outside literals and comments.
	rePlaceholderStyle = regexp.MustCompile(`\?|(?:^|[^:]):[A-Za-z_]|@[A-Za-z_]|\$[A-Za-z_]`)
	// rePlaceholderFragment is the same for a fragment, where prose is possible: it needs the
	// operator context of a placeholder.
	rePlaceholderFragment = regexp.MustCompile(`(?i)(?:[=<>]|\blike|\bin\s*\(|\bvalues\s*\(|,)\s*(?:\?\d*|[:@][A-Za-z_]\w*|\$[A-Za-z_]\w*)`)
)

// ---------------------------------------------------------------------------------------
// the tree rules' allowlists
// ---------------------------------------------------------------------------------------

// lintPortableTypes are libpg_query's names for text, bigint, integer, smallint, boolean, bytea,
// numeric and real.
var lintPortableTypes = map[string]bool{"text": true, "int8": true, "int4": true, "int2": true, "bool": true, "bytea": true, "numeric": true, "float4": true}

// e1Types are named by E1 already.
var e1Types = map[string]bool{"jsonb": true, "timestamptz": true, "citext": true}

func typeAdvice(name string) string {
	switch name {
	case "blob":
		return "BYTEA (PostgreSQL has no BLOB; SQLite stores a Go []byte in a BYTEA column as a blob)"
	case "uuid", "varchar", "bpchar", "char", "name":
		return "TEXT (ids and strings are TEXT; Go supplies uuid.New().String())"
	case "timestamp", "date", "time", "timetz", "interval":
		return "BIGINT holding UTC Unix milliseconds (Go supplies and parses it)"
	case "json":
		return "TEXT holding the JSON document"
	case "serial", "bigserial", "smallserial":
		return "TEXT PRIMARY KEY with an id Go supplies, or BIGINT"
	case "float8":
		return "REAL or NUMERIC"
	}
	return "one of text, bigint, integer, smallint, boolean, bytea, numeric, real"
}

// portableFuncs maps a function name to the number of arguments both engines agree on
// (-1: any). A function is added here only after it has run identically on SQLite and PostgreSQL.
var portableFuncs = map[string]int{
	"count": -1,
	"sum":   1,
	"min":   1, // SQLite's two-argument min/max is a different, scalar function
	"max":   1,
	"lower": 1, // dbx makes SQLite's lower() Unicode-aware
	// SQL-standard syntax that PostgreSQL rewrites to pg_catalog functions and SQLite also has:
	// LIKE ... ESCAPE, and TRIM(x), which strips spaces on both engines.
	"pg_catalog.like_escape":     2,
	"pg_catalog.not_like_escape": 2,
	"pg_catalog.btrim":           1,
}

var portableOps = map[string]bool{"=": true, "<>": true, "<": true, "<=": true, ">": true, ">=": true, "+": true, "-": true, "*": true, "/": true, "%": true, "||": true}

// e1Operators are named by E1 already.
var e1Operators = map[string]bool{"->": true, "->>": true}

var allowedStatements = map[string]bool{
	"SelectStmt": true, "InsertStmt": true, "UpdateStmt": true, "DeleteStmt": true,
	"CreateStmt": true, "IndexStmt": true, "ViewStmt": true, "DropStmt": true,
	"AlterTableStmt": true, "RenameStmt": true,
}

const rebuildHint = "SQLite cannot alter a column or constraint in place: create the new table, copy the rows, drop the old table and rename"

// pgOnlyNodes are parse-tree nodes that only PostgreSQL has (or that behave differently).
var pgOnlyNodes = map[string]string{
	"A_ArrayExpr":      "ARRAY[...] is PostgreSQL-only; bind each value and use IN ($1, $2, ...)",
	"A_Indirection":    "array subscripts and field selection are PostgreSQL-only",
	"SQLValueFunction": "CURRENT_TIMESTAMP and its relatives return different types on the two engines; bind a UTC Unix millisecond value from Go",
	"GroupingSet":      "GROUPING SETS, ROLLUP and CUBE are PostgreSQL-only",
	"GroupingFunc":     "GROUPING() is PostgreSQL-only",
	"NamedArgExpr":     "named function arguments are PostgreSQL-only",
	"MinMaxExpr":       "GREATEST and LEAST do not exist on SQLite; use CASE",
	"RangeTableSample": "TABLESAMPLE is PostgreSQL-only",
	"TableLikeClause":  "CREATE TABLE ... LIKE is PostgreSQL-only",
	"IntoClause":       "SELECT ... INTO is PostgreSQL-only",
	"SetToDefault":     "the DEFAULT keyword in VALUES is PostgreSQL-only; bind the value from Go",
	"CurrentOfExpr":    "WHERE CURRENT OF is PostgreSQL-only",
	"XmlExpr":          "XML functions are PostgreSQL-only",
	"XmlSerialize":     "XML functions are PostgreSQL-only",
}

// ---------------------------------------------------------------------------------------
// the rule engine
// ---------------------------------------------------------------------------------------

// stripSQL blanks comments and the contents of quoted strings and identifiers (keeping the
// quotes, the length and the newlines) so token rules never match text that is not SQL code.
func stripSQL(sql string) string {
	b := []byte(sql)
	n := len(b)
	blank := func(i int) {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
	for i := 0; i < n; {
		switch {
		case b[i] == '-' && i+1 < n && b[i+1] == '-':
			for i < n && b[i] != '\n' {
				blank(i)
				i++
			}
		case b[i] == '/' && i+1 < n && b[i+1] == '*':
			for depth := 0; i < n; {
				switch {
				case b[i] == '/' && i+1 < n && b[i+1] == '*':
					depth++
					blank(i)
					blank(i + 1)
					i += 2
				case b[i] == '*' && i+1 < n && b[i+1] == '/':
					depth--
					blank(i)
					blank(i + 1)
					i += 2
				default:
					blank(i)
					i++
				}
				if depth == 0 {
					break
				}
			}
		case b[i] == '\'' || b[i] == '"':
			q := b[i]
			for i++; i < n; i++ {
				if b[i] == q {
					if i+1 < n && b[i+1] == q { // doubled quote: still inside
						blank(i)
						blank(i + 1)
						i++
						continue
					}
					break
				}
				blank(i)
			}
			i++ // the closing quote
		default:
			i++
		}
	}
	return string(b)
}

func lintSQL(u lintUnit) lintResult {
	var res lintResult
	seen := map[string]bool{}
	lineAt := func(off int) int {
		if off > len(u.Text) {
			off = len(u.Text)
		}
		if off < 0 {
			off = 0
		}
		return 1 + strings.Count(u.Text[:off], "\n")
	}
	add := func(rule string, off int, format string, args ...any) {
		where, msg := u.Where, fmt.Sprintf(format, args...)
		if line := lineAt(off); u.Kind == lintMigration {
			where = fmt.Sprintf("%s:%d", u.Where, line)
		} else if line > 1 {
			msg += fmt.Sprintf(" (line %d of the SQL)", line)
		}
		if key := rule + where + msg; !seen[key] {
			seen[key] = true
			res.Findings = append(res.Findings, lintFinding{Rule: rule, Where: where, Msg: msg})
		}
	}
	defer func() {
		sort.SliceStable(res.Findings, func(i, j int) bool { return res.Findings[i].Where < res.Findings[j].Where })
	}()

	code := stripSQL(u.Text)
	for _, tk := range engineTokens {
		if u.Kind == lintFragment && !tk.inFragments || u.Kind != lintFragment && !tk.inCode {
			continue
		}
		for _, loc := range tk.re.FindAllStringIndex(code, 3) {
			name, hint := fmt.Sprintf("%q", tk.name), tk.hint
			if tk.call {
				fn := matchedFunc(code[loc[0]:loc[1]])
				name = fn + "()"
				if hint == "" {
					hint = engineFuncHint(fn)
				}
			}
			add("E1", loc[0], "%s is engine-specific: %s", name, hint)
		}
	}

	if u.Kind == lintFragment {
		if loc := rePlaceholderFragment.FindStringIndex(code); loc != nil {
			add("P1", loc[0], "placeholders are $1, $2, ... only (found %q)", strings.TrimSpace(code[loc[0]:loc[1]]))
		}
		return res
	}
	if loc := rePlaceholderStyle.FindStringIndex(code); loc != nil {
		add("P1", loc[0], "placeholders are $1, $2, ... only: ?, ?N, :name, @name and $name are SQLite-only or ambiguous (found %q)", strings.TrimSpace(code[loc[0]:loc[1]]))
	}

	parsed, err := pgquery.Parse(u.Text)
	if err != nil {
		if u.Dynamic {
			res.Unverified = true
			return res
		}
		add("S1", 0, "does not parse as PostgreSQL SQL: %v", err)
		return res
	}
	lintTree(u, parsed.ProtoReflect(), add)
	return res
}

type addFunc func(rule string, off int, format string, args ...any)

func lintTree(u lintUnit, tree protoreflect.Message, add addFunc) {
	l := &treeLint{add: add, params: map[int64]bool{}}
	stmts := listOf(tree, "stmts")
	for _, raw := range stmts {
		stmt := msgOf(raw, "stmt")
		if stmt == nil {
			continue
		}
		off := int(intOf(raw, "stmt_location"))
		kind := nameOf(stmt)
		if !allowedStatements[kind] {
			add("M1", off, "%s is not portable: only SELECT, INSERT, UPDATE, DELETE and CREATE/DROP of tables, indexes and views run identically on both engines", kind)
			continue
		}
		l.statement(kind, stmt, off)
		walkTree(stmt, l.visit)
	}
	if len(l.params) > 0 {
		switch {
		case u.Kind == lintMigration:
			add("P1", 0, "a migration is executed without arguments: it cannot contain $N placeholders")
		case len(stmts) > 1:
			add("S1", 0, "a statement with placeholders must be exactly one statement (PostgreSQL's extended protocol rejects several)")
		case !u.Dynamic:
			var max int64
			for n := range l.params {
				if n > max {
					max = n
				}
			}
			for n := int64(1); n <= max; n++ {
				if !l.params[n] {
					add("P1", 0, "placeholders must be gapless: $%d is used but $%d is not", max, n)
					break
				}
			}
		}
	}
}

type treeLint struct {
	add    addFunc
	params map[int64]bool // the $N numbers the statement uses
}

// statement checks what only the top of a statement shows.
func (l *treeLint) statement(kind string, stmt protoreflect.Message, off int) {
	switch kind {
	case "AlterTableStmt":
		for _, cmd := range listOf(stmt, "cmds") {
			if sub := enumOf(cmd, "subtype"); sub != "AT_AddColumn" {
				l.add("M1", off, "ALTER TABLE ... %s is not portable: only ADD COLUMN and RENAME run on both engines; %s", sub, rebuildHint)
			}
		}
	case "RenameStmt":
		if rt := enumOf(stmt, "rename_type"); rt != "OBJECT_TABLE" && rt != "OBJECT_COLUMN" {
			l.add("M1", off, "RENAME of %s is not portable; only tables and columns can be renamed on both engines", rt)
		}
	case "DropStmt":
		switch rt := enumOf(stmt, "remove_type"); {
		case rt != "OBJECT_TABLE" && rt != "OBJECT_INDEX" && rt != "OBJECT_VIEW":
			l.add("M1", off, "DROP of %s is not portable; only tables, indexes and views exist on both engines", rt)
		case enumOf(stmt, "behavior") == "DROP_CASCADE":
			l.add("M1", off, "DROP ... CASCADE is not portable; drop the dependents first")
		case boolOf(stmt, "concurrent"):
			l.add("M1", off, "DROP ... CONCURRENTLY is PostgreSQL-only")
		case len(listOf(stmt, "objects")) > 1:
			l.add("M1", off, "DROP of several objects in one statement is not portable; use one statement each")
		}
	}
}

// visit applies the tree rules to one node and reports whether to look inside it.
func (l *treeLint) visit(m protoreflect.Message) bool {
	name := nameOf(m)
	off := int(intOf(m, "location"))
	if why, bad := pgOnlyNodes[name]; bad {
		l.add("N1", off, "%s", why)
		return false
	}
	if strings.HasPrefix(name, "Json") {
		l.add("N1", off, "SQL/JSON constructs are engine-specific; handle JSON in Go")
		return false
	}
	switch name {
	case "ParamRef":
		l.params[intOf(m, "number")] = true

	case "TypeName":
		names := stringsOf(m, "names")
		if len(names) == 0 {
			break
		}
		last := strings.ToLower(names[len(names)-1])
		if len(listOf(m, "array_bounds")) > 0 {
			l.add("N1", off, "array types are PostgreSQL-only; store a list as TEXT (JSON) or in a child table")
		}
		if e1Types[last] {
			break
		}
		if qualified := len(names) > 1 && names[0] != "pg_catalog"; qualified || !lintPortableTypes[last] {
			l.add("T1", off, "type %s is not portable: use %s", strings.Join(names, "."), typeAdvice(last))
		}

	case "FuncCall":
		fn := strings.ToLower(strings.Join(stringsOf(m, "funcname"), "."))
		if engineFuncSet[fn] || strings.HasPrefix(fn, "json") {
			break // E1 names it
		}
		want, ok := portableFuncs[fn]
		switch got := len(listOf(m, "args")); {
		case !ok:
			l.add("F1", off, "function %s() is not on the portable list (%s): both engines must define it identically; add it to portableFuncs after running it on SQLite and PostgreSQL%s", fn, listKeys(portableFuncs), standardSyntaxNote(fn))
		case want >= 0 && got != want:
			l.add("F1", off, "%s() with %d argument(s) is not portable (both engines agree on %d); e.g. SQLite's two-argument max() is a different, scalar function: use CASE", fn, got, want)
		}

	case "A_Expr":
		op := strings.Join(stringsOf(m, "name"), ".")
		switch kind := enumOf(m, "kind"); kind {
		case "AEXPR_OP":
			if !portableOps[op] && !e1Operators[op] {
				l.add("N1", off, "operator %s does not exist on both engines (portable: = <> < <= > >= + - * / %% ||)", op)
			}
		case "AEXPR_LIKE":
			if !isLowerCall(msgOf(m, "lexpr")) {
				l.add("N1", off, "LIKE on a bare column behaves differently per engine (SQLite's ignores ASCII case, PostgreSQL's does not): write lower(column) LIKE a pattern lowered in Go")
			}
		case "AEXPR_IN", "AEXPR_NULLIF", "AEXPR_BETWEEN", "AEXPR_NOT_BETWEEN", "AEXPR_ILIKE":
			// ILIKE is E1's
		case "AEXPR_OP_ANY", "AEXPR_OP_ALL":
			l.add("N1", off, "= ANY(...) / ALL(...) over an array is PostgreSQL-only; expand the list into IN ($1, $2, ...)")
			return false
		case "AEXPR_SIMILAR":
			l.add("N1", off, "SIMILAR TO is PostgreSQL-only; use LIKE")
			return false
		default:
			l.add("N1", off, "%s is not portable; write the NULL cases out", kind)
			return false
		}

	case "SubLink":
		switch t := enumOf(m, "sub_link_type"); t {
		case "EXISTS_SUBLINK", "ANY_SUBLINK", "EXPR_SUBLINK", "MULTIEXPR_SUBLINK":
		default:
			l.add("N1", off, "%s is not portable; use EXISTS or IN", t)
			return false
		}

	case "SelectStmt":
		for _, d := range listOf(m, "distinct_clause") {
			if d != nil {
				l.add("N1", off, "DISTINCT ON is PostgreSQL-only; use GROUP BY or a subquery")
				break
			}
		}
		if op := enumOf(m, "op"); (op == "SETOP_INTERSECT" || op == "SETOP_EXCEPT") && boolOf(m, "all") {
			l.add("N1", off, "INTERSECT ALL and EXCEPT ALL are PostgreSQL-only; use UNION ALL, or INTERSECT/EXCEPT")
		}

	case "RangeSubselect", "RangeFunction":
		if boolOf(m, "lateral") {
			l.add("N1", off, "LATERAL is PostgreSQL-only; join on a subquery")
			return false
		}

	case "OnConflictClause":
		if c := msgOf(m, "infer"); c != nil && stringOf(c, "conname") != "" {
			l.add("N1", off, "ON CONFLICT ON CONSTRAINT is PostgreSQL-only; name the columns")
		}

	case "DeleteStmt":
		if len(listOf(m, "using_clause")) > 0 {
			l.add("N1", off, "DELETE ... USING is PostgreSQL-only; use a subquery in WHERE")
		}

	case "ViewStmt":
		if boolOf(m, "replace") {
			l.add("N1", off, "CREATE OR REPLACE VIEW is PostgreSQL-only; DROP VIEW IF EXISTS then CREATE VIEW")
		}
		if enumOf(m, "with_check_option") != "NO_CHECK_OPTION" && enumOf(m, "with_check_option") != "VIEW_CHECK_OPTION_UNDEFINED" {
			l.add("N1", off, "WITH CHECK OPTION is not portable")
		}

	case "IndexStmt":
		if am := stringOf(m, "access_method"); am != "" && am != "btree" {
			l.add("N1", off, "index method %q is PostgreSQL-only; indexes are btree", am)
		}
		if boolOf(m, "concurrent") || boolOf(m, "nulls_not_distinct") || len(listOf(m, "index_including_params")) > 0 || len(listOf(m, "options")) > 0 || stringOf(m, "table_space") != "" {
			l.add("N1", off, "CONCURRENTLY, INCLUDE, NULLS NOT DISTINCT, WITH (...) and TABLESPACE on an index are PostgreSQL-only")
		}

	case "IndexElem":
		if len(listOf(m, "collation")) > 0 || len(listOf(m, "opclass")) > 0 {
			l.add("N1", off, "index collations and operator classes are PostgreSQL-only")
		}
		if no := enumOf(m, "nulls_ordering"); no == "SORTBY_NULLS_FIRST" || no == "SORTBY_NULLS_LAST" {
			l.add("N1", off, "NULLS FIRST/LAST in an index definition is PostgreSQL-only")
		}

	case "CreateStmt":
		rel := msgOf(m, "relation")
		if p := stringOf(rel, "relpersistence"); p != "" && p != "p" {
			l.add("N1", off, "TEMPORARY and UNLOGGED tables are not portable")
		}
		if len(listOf(m, "inh_relations")) > 0 || msgOf(m, "partspec") != nil || msgOf(m, "partbound") != nil || len(listOf(m, "options")) > 0 ||
			stringOf(m, "tablespacename") != "" || msgOf(m, "of_typename") != nil {
			l.add("N1", off, "INHERITS, PARTITION BY, WITH (...), TABLESPACE and typed tables are PostgreSQL-only")
		}
		if oc := enumOf(m, "oncommit"); oc != "" && oc != "ONCOMMIT_NOOP" && oc != "ON_COMMIT_ACTION_UNDEFINED" {
			l.add("N1", off, "ON COMMIT is not portable")
		}

	case "ColumnDef":
		if stringOf(m, "identity") != "" || stringOf(m, "generated") != "" {
			l.add("N1", off, "identity and generated columns are engine-specific; ids come from uuid.New() in Go")
		}

	case "Constraint":
		switch enumOf(m, "contype") {
		case "CONSTR_DEFAULT":
			if raw := msgOf(m, "raw_expr"); raw == nil || nameOf(raw) != "A_Const" {
				l.add("D1", off, "a column DEFAULT must be a constant such as 0, TRUE or '{}': engines disagree on default expressions, so the repository supplies ids and timestamps")
			}
			return false
		case "CONSTR_IDENTITY", "CONSTR_GENERATED":
			l.add("N1", off, "identity and generated columns are engine-specific; ids come from uuid.New() in Go")
		case "CONSTR_EXCLUSION":
			l.add("N1", off, "EXCLUDE constraints are PostgreSQL-only")
		}
	}
	return true
}

// isLowerCall reports whether e is a call of lower(...).
func isLowerCall(e protoreflect.Message) bool {
	return e != nil && nameOf(e) == "FuncCall" && strings.EqualFold(strings.Join(stringsOf(e, "funcname"), "."), "lower")
}

// standardSyntaxNote explains a pg_catalog name: the SQL never spelled it that way.
func standardSyntaxNote(fn string) string {
	if strings.HasPrefix(fn, "pg_catalog.") {
		return fmt.Sprintf(" (PostgreSQL's name for SQL-standard syntax such as TRIM, SUBSTRING or EXTRACT: %s)", strings.TrimPrefix(fn, "pg_catalog."))
	}
	return ""
}

func listKeys(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, strings.TrimPrefix(k, "pg_catalog."))
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// ---- protoreflect helpers over libpg_query's parse tree ----

func nameOf(m protoreflect.Message) string {
	if m == nil {
		return ""
	}
	return string(m.Descriptor().Name())
}

// unwrapNode returns the concrete message inside a pg_query.Node, or nil for an empty Node.
func unwrapNode(m protoreflect.Message) protoreflect.Message {
	if m == nil || nameOf(m) != "Node" {
		return m
	}
	fd := m.WhichOneof(m.Descriptor().Oneofs().Get(0))
	if fd == nil {
		return nil
	}
	return m.Get(fd).Message()
}

func fieldOf(m protoreflect.Message, name string) protoreflect.FieldDescriptor {
	if m == nil {
		return nil
	}
	return m.Descriptor().Fields().ByName(protoreflect.Name(name))
}

func msgOf(m protoreflect.Message, name string) protoreflect.Message {
	fd := fieldOf(m, name)
	if fd == nil || fd.Message() == nil || fd.IsList() || !m.Has(fd) {
		return nil
	}
	return unwrapNode(m.Get(fd).Message())
}

func listOf(m protoreflect.Message, name string) []protoreflect.Message {
	fd := fieldOf(m, name)
	if fd == nil || !fd.IsList() {
		return nil
	}
	l := m.Get(fd).List()
	out := make([]protoreflect.Message, 0, l.Len())
	for i := 0; i < l.Len(); i++ {
		out = append(out, unwrapNode(l.Get(i).Message()))
	}
	return out
}

// stringsOf reads a list of String nodes (type and function names).
func stringsOf(m protoreflect.Message, name string) []string {
	var out []string
	for _, n := range listOf(m, name) {
		if n != nil && nameOf(n) == "String" {
			out = append(out, stringOf(n, "sval"))
		}
	}
	return out
}

func stringOf(m protoreflect.Message, name string) string {
	if fd := fieldOf(m, name); fd != nil && fd.Kind() == protoreflect.StringKind {
		return m.Get(fd).String()
	}
	return ""
}

func boolOf(m protoreflect.Message, name string) bool {
	if fd := fieldOf(m, name); fd != nil && fd.Kind() == protoreflect.BoolKind {
		return m.Get(fd).Bool()
	}
	return false
}

func intOf(m protoreflect.Message, name string) int64 {
	if fd := fieldOf(m, name); fd != nil && (fd.Kind() == protoreflect.Int32Kind || fd.Kind() == protoreflect.Int64Kind) {
		return m.Get(fd).Int()
	}
	return 0
}

func enumOf(m protoreflect.Message, name string) string {
	fd := fieldOf(m, name)
	if fd == nil || fd.Enum() == nil {
		return ""
	}
	if v := fd.Enum().Values().ByNumber(m.Get(fd).Enum()); v != nil {
		return string(v.Name())
	}
	return ""
}

// walkTree visits every message below m depth-first; visit returns false to skip a subtree.
func walkTree(m protoreflect.Message, visit func(protoreflect.Message) bool) {
	if m == nil || !m.IsValid() || !visit(m) {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Message() == nil || fd.IsMap():
		case fd.IsList():
			for l, i := v.List(), 0; i < l.Len(); i++ {
				walkTree(l.Get(i).Message(), visit)
			}
		default:
			walkTree(v.Message(), visit)
		}
		return true
	})
}

// ---------------------------------------------------------------------------------------
// extracting SQL from Go source (AST only; nothing is executed)
// ---------------------------------------------------------------------------------------

// goSource is one Go file handed to the extractor. An exempt file still contributes its
// constants (the rest of its package may use them) but reports nothing.
type goSource struct {
	Path   string // slash-separated, relative to the module root
	Src    string
	Exempt bool
}

// dyn marks text Go only knows at run time.
const dyn = "\x00"

// stmtLike says whether folded text starts like a SQL statement. It is deliberately
// structural, not just a leading keyword, so a message such as "insert into the failing table"
// or "update failed" is prose.
var stmtLike = regexp.MustCompile(`(?is)^\s*(?:--[^\n]*\n\s*)*(?:` +
	`select\s+(?:distinct\s+)?(?:\*|[^;]*?\bfrom\s+[\w."(\x00]|[\w.]+\(|\d|'|\$\d)` +
	`|insert\s+into\s+[\w."\x00]+\s*(?:\(|values|select|default)` +
	`|update\s+[\w."\x00]+\s+set\s` +
	`|delete\s+from\s+[\w."\x00]+(?:\s|;|$)` +
	`|with\s+(?:recursive\s+)?\w+\s*(?:\([^)]*\)\s*)?as\s*\(` +
	`|(?:create|drop|alter)\s+(?:or\s+replace\s+)?(?:unique\s+)?(?:temp(?:orary)?\s+)?(?:table|index|view|function|trigger|extension|schema|sequence|type)\s+(?:if\s+(?:not\s+)?exists\s+)?[\w."\x00]+` +
	`|pragma\s+\w+` +
	`|truncate\s+(?:table\s+)?[\w."]+` +
	`)`)

// fillers give a neutral, parseable stand-in for run-time text, chosen by what precedes it.
var fillers = []struct {
	re   *regexp.Regexp
	fill string
}{
	{regexp.MustCompile(`\$$`), "1"}, // "... = $" + itoa(n)
	{regexp.MustCompile(`(?i)\b(?:limit|offset)\s*$`), "1"},
	{regexp.MustCompile(`(?i)\bin\s*$`), "(NULL)"}, // dbx.InList
	{regexp.MustCompile(`(?i)\bset\s*$`), "dyn_col = 1"},
	{regexp.MustCompile(`(?i)(?:\b(?:where|and|or|on|not|having)\b|\()\s*$`), "1 = 1"},
}

func fillDynamic(text string) string {
	var out strings.Builder
	for {
		i := strings.Index(text, dyn)
		if i < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:i])
		fill := "dyn_name"
		for _, f := range fillers {
			if f.re.MatchString(out.String()) {
				fill = f.fill
				break
			}
		}
		out.WriteString(fill)
		text = text[i+len(dyn):]
	}
}

// folded is constant-folded text and the string literals it came from.
type folded struct {
	s    string
	lits []token.Pos
}

type foldEnv struct {
	pkg   map[string]ast.Expr // package-level const/var of the directory
	local map[string]folded   // function-local strings, as assigned so far
}

func (en *foldEnv) fold(e ast.Expr, depth int) folded {
	if depth > 12 {
		return folded{s: dyn}
	}
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return folded{s: s, lits: []token.Pos{v.Pos()}}
			}
		}
	case *ast.ParenExpr:
		return en.fold(v.X, depth+1)
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			a, b := en.fold(v.X, depth+1), en.fold(v.Y, depth+1)
			return folded{s: a.s + b.s, lits: append(append([]token.Pos(nil), a.lits...), b.lits...)}
		}
	case *ast.Ident:
		if f, ok := en.local[v.Name]; ok {
			return f
		}
		if x, ok := en.pkg[v.Name]; ok {
			return en.fold(x, depth+1)
		}
	}
	return folded{s: dyn}
}

// stringish reports whether e could be a string built from literals and names.
func stringish(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.BasicLit:
		return v.Kind == token.STRING
	case *ast.Ident:
		return true
	case *ast.ParenExpr:
		return stringish(v.X)
	case *ast.BinaryExpr:
		return v.Op == token.ADD
	}
	return false
}

// packageStrings gathers the package-level constants and variables of every directory, so a
// statement can be assembled from declarations spread over files.
func packageStrings(parsed []*ast.File, files []goSource) map[string]map[string]ast.Expr {
	pkgs := map[string]map[string]ast.Expr{}
	for i, af := range parsed {
		dir := dirOf(files[i].Path)
		if pkgs[dir] == nil {
			pkgs[dir] = map[string]ast.Expr{}
		}
		for _, d := range af.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST && gd.Tok != token.VAR {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				for j, n := range vs.Names {
					if j < len(vs.Values) {
						pkgs[dir][n.Name] = vs.Values[j]
					}
				}
			}
		}
	}
	return pkgs
}

// shadow makes names unknown in the current function: a parameter, a loop variable or a
// value from a call hides a package-level declaration of the same name.
func (en *foldEnv) shadow(names ...*ast.Ident) {
	for _, n := range names {
		if n != nil && n.Name != "_" {
			en.local[n.Name] = folded{s: dyn}
		}
	}
}

func (en *foldEnv) shadowFields(fl *ast.FieldList) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		en.shadow(f.Names...)
	}
}

func extractGoSQL(files []goSource) ([]lintUnit, error) {
	fset := token.NewFileSet()
	parsed := make([]*ast.File, len(files))
	for i, f := range files {
		af, err := parser.ParseFile(fset, f.Path, f.Src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		parsed[i] = af
	}
	pkgs := packageStrings(parsed, files)
	where := func(i int, p token.Pos) string { return fmt.Sprintf("%s:%d", files[i].Path, fset.Position(p).Line) }

	var units []lintUnit
	stmtLits := map[token.Pos]bool{} // literals that are part of a statement
	sqlFiles := map[int]bool{}       // files that hold at least one statement

	// Statements: a string expression handed to a call, returned, or put in a literal, whose
	// folded text reads as SQL, whatever receives it.
	for i, f := range files {
		en := &foldEnv{pkg: pkgs[dirOf(f.Path)], local: map[string]folded{}}
		root := func(e ast.Expr) {
			if !stringish(e) {
				return
			}
			fd := en.fold(e, 0)
			if !stmtLike.MatchString(fd.s) {
				return
			}
			for _, p := range fd.lits {
				stmtLits[p] = true
			}
			sqlFiles[i] = true
			if f.Exempt {
				return
			}
			u := lintUnit{Where: where(i, e.Pos()), Kind: lintStatement, Text: fd.s}
			if strings.Contains(fd.s, dyn) {
				u.Text, u.Dynamic = fillDynamic(fd.s), true
			}
			units = append(units, u)
		}
		for _, decl := range parsed[i].Decls {
			en.local = map[string]folded{} // locals live as long as their declaration
			ast.Inspect(decl, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.FuncDecl:
					en.shadowFields(v.Type.Params)
					en.shadowFields(v.Type.Results)
				case *ast.FuncLit:
					en.shadowFields(v.Type.Params)
					en.shadowFields(v.Type.Results)
				case *ast.RangeStmt:
					if v.Tok == token.DEFINE {
						k, _ := v.Key.(*ast.Ident)
						val, _ := v.Value.(*ast.Ident)
						en.shadow(k, val)
					}
				case *ast.DeclStmt:
					if gd, ok := v.Decl.(*ast.GenDecl); ok && (gd.Tok == token.CONST || gd.Tok == token.VAR) {
						for _, sp := range gd.Specs {
							vs := sp.(*ast.ValueSpec)
							for j, name := range vs.Names {
								if j < len(vs.Values) && stringish(vs.Values[j]) {
									en.local[name.Name] = en.fold(vs.Values[j], 0)
								} else {
									en.shadow(name)
								}
							}
						}
					}
				case *ast.AssignStmt:
					if len(v.Lhs) != len(v.Rhs) { // a, b := f()
						for _, lhs := range v.Lhs {
							id, _ := lhs.(*ast.Ident)
							en.shadow(id)
						}
						break
					}
					for j, lhs := range v.Lhs {
						id, isIdent := lhs.(*ast.Ident)
						switch {
						case !isIdent:
							root(v.Rhs[j]) // x.sql = "..."
						case !stringish(v.Rhs[j]):
							en.shadow(id) // q = rewrite(q): unknown from here on
						default:
							rhs := en.fold(v.Rhs[j], 0)
							if v.Tok == token.ADD_ASSIGN {
								old := en.local[id.Name]
								rhs = folded{s: old.s + rhs.s, lits: append(append([]token.Pos(nil), old.lits...), rhs.lits...)}
							}
							en.local[id.Name] = rhs
						}
					}
				case *ast.CallExpr:
					for _, a := range v.Args {
						root(a)
					}
				case *ast.ReturnStmt:
					for _, r := range v.Results {
						root(r)
					}
				case *ast.CompositeLit:
					for _, e := range v.Elts {
						if kv, ok := e.(*ast.KeyValueExpr); ok {
							e = kv.Value
						}
						root(e)
					}
				}
				return true
			})
		}
	}

	// Leftovers: statement-like literals no call, return or element took (a statement kept in a
	// variable that is only returned, say). Linted alone; a partial one is "unverified".
	for i, f := range files {
		if f.Exempt {
			continue
		}
		ast.Inspect(parsed[i], func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING || stmtLits[bl.Pos()] {
				return true
			}
			s, err := strconv.Unquote(bl.Value)
			if err != nil || !stmtLike.MatchString(s) {
				return true
			}
			stmtLits[bl.Pos()] = true
			sqlFiles[i] = true
			u := lintUnit{Where: where(i, bl.Pos()), Kind: lintStatement, Text: s}
			if _, err := pgquery.Parse(s); err != nil {
				u.Text, u.Dynamic = fillDynamic(s+dyn), true
			}
			units = append(units, u)
			return true
		})
	}

	// Fragments: every other literal of a file that holds SQL.
	for i, f := range files {
		if f.Exempt || !sqlFiles[i] {
			continue
		}
		ast.Inspect(parsed[i], func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING || stmtLits[bl.Pos()] {
				return true
			}
			if s, err := strconv.Unquote(bl.Value); err == nil {
				units = append(units, lintUnit{Where: where(i, bl.Pos()), Kind: lintFragment, Text: s})
			}
			return true
		})
	}
	return units, nil
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

// ---------------------------------------------------------------------------------------
// the real tree
// ---------------------------------------------------------------------------------------

// sqlLintExempt is code that is engine-specific BY DESIGN. A path ending in "/" is a directory.
// Every entry needs a reason, and TestSQLLintExemptionsExist fails when one no longer exists.
var sqlLintExempt = map[string]string{
	"internal/dbx/":                               "the engine boundary: SQLite pragmas, the PostgreSQL advisory lock, the lower() override and the migration runner",
	"internal/dbops/":                             "SQLite file backup and restore (VACUUM INTO, PRAGMA): refuses PostgreSQL by design",
	"internal/dbtest/failinserts.go":              "per-engine trigger DDL for fault injection",
	"internal/dbtest/target.go":                   "creates and drops the per-test PostgreSQL schema",
	"internal/dbtest/contract_test.go":            "catalog introspection: PRAGMA on SQLite, information_schema and pg_catalog on PostgreSQL",
	"internal/dbtest/schema_neutral_test.go":      "catalog introspection: PRAGMA on SQLite, information_schema and pg_catalog on PostgreSQL",
	"internal/dbtest/sql_linter_test.go":          "the linter itself",
	"internal/dbtest/sql_linter_selftest_test.go": "its fixtures are deliberately engine-specific SQL",
}

// sqlLintMaxUnverified caps the dynamic statements the parser rejected even with fillers (see
// lintResult.Unverified). It is a ratchet: lower it when a statement is made verifiable, never
// raise it without finding out why a new one cannot be parsed.
const sqlLintMaxUnverified = 0

func lintExempt(p string) bool {
	for e := range sqlLintExempt {
		if p == e || strings.HasSuffix(e, "/") && strings.HasPrefix(p, e) {
			return true
		}
	}
	return false
}

func TestSQLLintExemptionsExist(t *testing.T) {
	root := os.DirFS(moduleRoot(t))
	for p, reason := range sqlLintExempt {
		if reason == "" {
			t.Errorf("exemption %s has no reason", p)
		}
		if _, err := fs.Stat(root, strings.TrimSuffix(p, "/")); err != nil {
			t.Errorf("exemption %s no longer exists (%v): remove it from sqlLintExempt", p, err)
		}
	}
}

func TestSQLLintMigrations(t *testing.T) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := migrations.FS.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		files++
		res := lintSQL(lintUnit{Where: "migrations/" + e.Name(), Kind: lintMigration, Text: string(body)})
		for _, f := range res.Findings {
			t.Error(f)
		}
	}
	if files == 0 {
		t.Fatal("no migrations found: the linter scanned nothing")
	}
	t.Logf("linted %d migration files", files)
}

func readGoSources(t testing.TB, dirs ...string) []goSource {
	t.Helper()
	root := os.DirFS(moduleRoot(t))
	var out []goSource
	for _, dir := range dirs {
		err := fs.WalkDir(root, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if n := d.Name(); n == "testdata" || n == "vendor" || strings.HasPrefix(n, "_") || strings.HasPrefix(n, ".") {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") {
				return nil
			}
			b, err := fs.ReadFile(root, p)
			if err != nil {
				return err
			}
			out = append(out, goSource{Path: p, Src: string(b), Exempt: lintExempt(p)})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestSQLLintGoSQL(t *testing.T) {
	files := readGoSources(t, "internal", "cmd")
	units, err := extractGoSQL(files)
	if err != nil {
		t.Fatal(err)
	}
	var statements, dynamic, fragments, unverified int
	for _, u := range units {
		switch {
		case u.Kind == lintFragment:
			fragments++
		case u.Dynamic:
			statements++
			dynamic++
		default:
			statements++
		}
		res := lintSQL(u)
		for _, f := range res.Findings {
			t.Error(f)
		}
		if res.Unverified {
			unverified++
			t.Logf("unverified (dynamic statement the parser rejected): %s: %s", u.Where, squash(u.Text))
		}
	}
	// A linter that silently scans nothing is worse than none.
	if statements < 300 {
		t.Fatalf("found only %d SQL statements in %d Go files: the extractor has stopped seeing the repositories", statements, len(files))
	}
	if unverified > sqlLintMaxUnverified {
		t.Errorf("%d dynamic statements could not be parsed, more than the %d allowed (sqlLintMaxUnverified)", unverified, sqlLintMaxUnverified)
	}
	t.Logf("scanned %d Go files: %d statements (%d with run-time parts), %d other literals in SQL-bearing files, %d unverified", len(files), statements, dynamic, fragments, unverified)
}
