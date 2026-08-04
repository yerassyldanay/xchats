// Package pgimport is a standalone, one-time data migration tool: it reads
// every row of every base table out of the frozen Postgres schema
// (migrations/postgres) and writes it into an already-migrated SQLite
// database (migrations/sqlite), preserving every primary key exactly, so
// cross-table references stay intact across the cutover. It is pgx-linked
// (the only place in this module allowed to be, alongside
// internal/dbx — see internal/dbtest's architecture test) and never linked
// into the packaged xchats binary: only cmd/xchats-import imports it.
//
// This package is schema-driven, not table-by-table hand-written: it reads
// migrations/sqlite's embedded schema_contract.json (the same column/type
// metadata dbtest's own contract test validates the SQLite schema
// against) to learn every table's columns and each column's Postgres type,
// and generates both the source SELECT and the destination INSERT from
// that — so a schema change updates the contract once (already required
// for the contract test to keep passing) and the importer picks it up
// automatically, rather than needing a second, easily-forgotten hand
// edit.
package pgimport

import (
	"encoding/json"
	"fmt"
	"sort"

	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// contract is schema_contract.json's shape, narrowed to the fields
// pgimport actually needs (column name/type/position, primary key for a
// stable read order) — the file carries more (indexes, unique
// constraints, foreign keys) that json.Unmarshal simply leaves unparsed.
type contract struct {
	Tables map[string]contractTable `json:"tables"`
}

type contractTable struct {
	Columns    []contractColumn `json:"columns"`
	PrimaryKey []string         `json:"primary_key"`
}

type contractColumn struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
	PgType   string `json:"pg_type"`
	Nullable bool   `json:"nullable"`
}

// table is one contractTable resolved into the shape the rest of this
// package works with: columns in stable position order, each carrying its
// resolvedKind (how to read it from Postgres and write it to SQLite).
type table struct {
	Name       string
	Columns    []column
	PrimaryKey []string
}

type column struct {
	Name string
	// PgType is the raw schema_contract.json type string (e.g. "uuid",
	// "numeric", "_uuid[]") — selectExpr needs the exact type, not just
	// Kind, since e.g. uuid and numeric are both kindScalar but need
	// different casts (::text vs ::float8).
	PgType string
	Kind   columnKind
}

// loadContract parses the embedded schema contract and returns every base
// table (never a view — schema_contract.json's own "views" section is a
// separate top-level key this type never unmarshals into, so views are
// structurally excluded) in a stable, deterministic order. That order does
// NOT need to respect foreign keys — see pgimport.go's use of
// PRAGMA defer_foreign_keys, which makes cross-table insert order
// irrelevant within one transaction.
func loadContract() ([]table, error) {
	var c contract
	if err := json.Unmarshal(sqlitemigrations.SchemaContractJSON, &c); err != nil {
		return nil, fmt.Errorf("pgimport: parse schema_contract.json: %w", err)
	}

	names := make([]string, 0, len(c.Tables))
	for name := range c.Tables {
		names = append(names, name)
	}
	sort.Strings(names)

	tables := make([]table, 0, len(names))
	for _, name := range names {
		ct := c.Tables[name]
		cols := append([]contractColumn(nil), ct.Columns...)
		sort.Slice(cols, func(i, j int) bool { return cols[i].Position < cols[j].Position })

		cs := make([]column, len(cols))
		for i, cc := range cols {
			kind, err := kindForPgType(cc.PgType)
			if err != nil {
				return nil, fmt.Errorf("pgimport: table %s column %s: %w", name, cc.Name, err)
			}
			cs[i] = column{Name: cc.Name, PgType: cc.PgType, Kind: kind}
		}
		tables = append(tables, table{Name: name, Columns: cs, PrimaryKey: ct.PrimaryKey})
	}
	return tables, nil
}
