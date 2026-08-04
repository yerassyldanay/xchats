package dbtest

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// persistenceBoundary is every package allowed to import internal/dbx — the
// repository layer the sqlite-cutover plan's Layering section names
// explicitly. Nothing else may import dbx, and NOTHING (not even these
// packages) may import database/sql or modernc.org/sqlite directly: that
// stays confined to internal/dbx alone, so there is exactly one place in
// the whole module that knows which database engine this is.
const modulePrefix = "github.com/yerassyldanay/xchats/backend/"

var persistenceBoundary = map[string]bool{
	modulePrefix + "internal/store":         true,
	modulePrefix + "internal/kbstore":       true,
	modulePrefix + "internal/responsestore": true,
	modulePrefix + "internal/mcpauth":       true,
	modulePrefix + "internal/dbtest":        true,
	// dbops is durability tooling (backup/restore/integrity-check), not a
	// repository package — but it is the one other place in this module
	// that legitimately holds a *dbx.DB and runs raw SQL against it, so it
	// belongs on this list too.
	modulePrefix + "internal/dbops": true,
	// pgimport is the standalone Postgres->SQLite data migration tool
	// (cmd/xchats-import). It is also pgx-linked (see driverOnlyPackages'
	// own doc comment) — that's a SEPARATE axis from this one: pgx is
	// never restricted by this test at all, only database/sql/
	// modernc.org/sqlite and dbx are. pgimport writes its SQLite side
	// through dbx like dbops does, rather than duplicating dbx's own
	// time/array type-conversion helpers (dbx.FormatTime, dbx.UUIDArray/
	// StringArray) a second time.
	modulePrefix + "internal/pgimport": true,
	// cmd/xchats-import is pgimport's own standalone binary (never
	// cmd/xchats — the packaged server never carries a pgx dependency): it
	// calls dbx.Open/dbx.RunMigrations itself to prepare the destination
	// SQLite file before handing it to pgimport.Import.
	modulePrefix + "cmd/xchats-import": true,
}

// dbtestConsumers may import internal/dbx from their TEST files only — never
// from production code. These are packages whose tests legitimately assert on
// database state that no exported method surfaces, and internal/dbtest's
// fixtures hand them a *dbx.DB for exactly that (see NewKB/Open's own doc
// comments). Their production code is still held to the full rule above: an
// import of dbx from a non-test file in any of these fails the test.
//
// The distinction is the point. The boundary exists so that PRODUCTION code
// cannot know which database engine is underneath it. A test asserting "this
// row landed in wa_messages" is a different concern, and forcing it through an
// exported method would mean growing the production API surface to serve tests.
var dbtestConsumers = map[string]bool{
	modulePrefix + "internal/httpapi":   true,
	modulePrefix + "internal/mcpserver": true,
	modulePrefix + "cmd/xchats":         true,
}

const dbxImportPath = modulePrefix + "internal/dbx"

// driverOnlyPackages may import database/sql and modernc.org/sqlite
// directly — internal/dbx itself is the only one today (the whole point of
// the facade). Nothing else needs to: even internal/pgimport, which reads
// from Postgres via pgx (a driver this test does not restrict at all —
// only the SQLite-side database/sql and modernc.org/sqlite, and dbx
// itself, are), writes its SQLite side through dbx like every other
// persistenceBoundary package instead of opening its own connection.
var driverOnlyPackages = map[string]bool{
	dbxImportPath: true,
}

type goListPackage struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
	DepOnly      bool
	Error        *struct {
		Err string
	}
}

// TestArchitectureBoundary enforces the persistence boundary from the
// sqlite-cutover plan's Layering section at build-graph level: no package
// outside internal/store, internal/kbstore, internal/responsestore,
// internal/mcpauth, and internal/dbtest may import internal/dbx, and NO
// package anywhere (including those five) may import database/sql or
// modernc.org/sqlite directly — that stays confined to internal/dbx. This
// is the same shape of check that, had it existed earlier, would have flagged
// the stray backend/force-user.go — a root package main holding its own pgx
// pool and a hardcoded local DSN, outside the persistence layer of its era.
// That file is now deleted; migration 0006_init_admin does its job.
func TestArchitectureBoundary(t *testing.T) {
	root := moduleRoot(t)

	// -e: keep going and still emit a JSON record (with Error set) for a
	// package that fails to load, instead of aborting the whole command.
	// cmd/xchats intentionally does not compile against this branch alone
	// (Phase 6 rewires it against the ported store) — this check only needs
	// whatever import list each package DID resolve, not a clean build.
	cmd := exec.Command("go", "list", "-e", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list -e -json ./...: %v\nstderr:\n%s", err, ee.Stderr)
		}
		t.Fatalf("go list -e -json ./...: %v", err)
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	checked := 0
	for {
		var pkg goListPackage
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		if !strings.HasPrefix(pkg.ImportPath, modulePrefix) {
			continue // a dependency, not one of this module's own packages
		}
		if driverOnlyPackages[pkg.ImportPath] {
			continue
		}
		checked++

		// Production imports and test-only imports are checked separately: the
		// dbx rule is relaxed for the test files of dbtestConsumers, the
		// database/sql and modernc.org/sqlite rules never are.
		testOnlyImports := append(append([]string{}, pkg.TestImports...), pkg.XTestImports...)
		for _, imp := range pkg.Imports {
			checkDriverImports(t, pkg.ImportPath, imp, "")
			if imp == dbxImportPath && !persistenceBoundary[pkg.ImportPath] {
				t.Errorf("%s imports %s from production code but is outside the persistence boundary (%v) — dbx is internal plumbing of the repository packages only",
					pkg.ImportPath, dbxImportPath, boundaryList())
			}
		}
		for _, imp := range testOnlyImports {
			checkDriverImports(t, pkg.ImportPath, imp, " (test files)")
			if imp == dbxImportPath && !persistenceBoundary[pkg.ImportPath] && !dbtestConsumers[pkg.ImportPath] {
				t.Errorf("%s's test files import %s, but it is neither inside the persistence boundary nor a declared dbtest consumer — get the handle from internal/dbtest's fixtures instead",
					pkg.ImportPath, dbxImportPath)
			}
		}
	}
	if checked == 0 {
		t.Fatal("architecture check inspected zero packages — go list produced no output, the check is not actually running")
	}
}

// checkDriverImports flags a direct database/sql or modernc.org/sqlite import.
// This rule has no exceptions anywhere in the module — not for the persistence
// packages, not for test files — so that exactly one package knows which
// database engine this is.
func checkDriverImports(t *testing.T, pkgPath, imp, where string) {
	t.Helper()
	switch imp {
	case "database/sql":
		t.Errorf("%s%s imports database/sql directly — only internal/dbx may; route through the dbx facade instead", pkgPath, where)
	case "modernc.org/sqlite":
		t.Errorf("%s%s imports modernc.org/sqlite directly — only internal/dbx may; route through the dbx facade instead", pkgPath, where)
	}
}

func boundaryList() []string {
	out := make([]string, 0, len(persistenceBoundary))
	for p := range persistenceBoundary {
		out = append(out, strings.TrimPrefix(p, modulePrefix))
	}
	return out
}
