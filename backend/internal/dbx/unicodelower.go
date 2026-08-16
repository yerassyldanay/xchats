package dbx

import (
	"database/sql/driver"
	"fmt"
	"strings"

	sqlite "modernc.org/sqlite"
)

// SQLite's built-in lower()/upper() and its NOCASE collation only fold ASCII
// A–Z. That is a documented SQLite limitation, not a driver quirk: without the
// optional ICU extension, «Алия» lowercases to «Алия», unchanged.
//
// For this product that is not a corner case. Operators and customers write
// Russian, so a case-insensitive search built on lower() silently fails on
// exactly the names it exists to find: searching "али" would never match a
// stored "Алия", while "abc" would happily match "ABC".
//
// unicode_lower closes that gap with Go's own Unicode-aware strings.ToLower.
// It lives here because internal/dbx is the one package in the module allowed
// to know which database engine is underneath (enforced by
// dbtest.TestArchitectureBoundary) — a repository package registering driver
// functions would be exactly the leak that boundary exists to prevent.
//
// Registration is process-wide and applies to connections opened AFTER it
// runs, so it happens in this package's init rather than in Open: a caller
// that opens a database before some other package's init got around to
// registering would otherwise get a connection without the function.
const unicodeLowerFunc = "unicode_lower"

func init() {
	err := sqlite.RegisterDeterministicScalarFunction(unicodeLowerFunc, 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("%s: expected 1 argument, got %d", unicodeLowerFunc, len(args))
			}
			switch v := args[0].(type) {
			case nil:
				// NULL in, NULL out — the same contract SQLite's own lower() has.
				return nil, nil
			case string:
				return strings.ToLower(v), nil
			case []byte:
				return strings.ToLower(string(v)), nil
			default:
				// A number or blob compared against a text pattern: stringify
				// rather than error, so a query cannot fail on a column whose
				// affinity surprised us.
				return strings.ToLower(fmt.Sprint(v)), nil
			}
		})
	if err != nil {
		panic("dbx: register " + unicodeLowerFunc + ": " + err.Error())
	}
}
