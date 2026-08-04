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
}

const dbxImportPath = modulePrefix + "internal/dbx"

// driverOnlyPackages may import database/sql and modernc.org/sqlite
// directly — internal/dbx itself (the whole point of the facade) and the
// standalone Postgres importer, which is pgx-based and never linked into
// the packaged xchats binary, so it falls outside this boundary entirely
// (it doesn't even import dbx).
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
// is the same shape of check that, run before this branch, would have
// flagged the stray backend/force-user.go (root package main, a direct pgx
// dependency outside the persistence layer of its era).
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

		allImports := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)
		for _, imp := range allImports {
			switch {
			case imp == "database/sql":
				t.Errorf("%s imports database/sql directly — only internal/dbx may; route through the dbx facade instead", pkg.ImportPath)
			case imp == "modernc.org/sqlite":
				t.Errorf("%s imports modernc.org/sqlite directly — only internal/dbx may; route through the dbx facade instead", pkg.ImportPath)
			case imp == dbxImportPath && !persistenceBoundary[pkg.ImportPath]:
				t.Errorf("%s imports %s but is outside the persistence boundary (%v) — dbx is internal plumbing of the repository packages only",
					pkg.ImportPath, dbxImportPath, boundaryList())
			}
		}
	}
	if checked == 0 {
		t.Fatal("architecture check inspected zero packages — go list produced no output, the check is not actually running")
	}
}

func boundaryList() []string {
	out := make([]string, 0, len(persistenceBoundary))
	for p := range persistenceBoundary {
		out = append(out, strings.TrimPrefix(p, modulePrefix))
	}
	return out
}
