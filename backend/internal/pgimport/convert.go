package pgimport

import (
	"fmt"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// columnKind is how one column's Postgres value is read and rebound for
// SQLite — resolved once per column from its pg_type (see kindForPgType),
// not re-derived per row.
type columnKind int

const (
	// kindScalar covers every Postgres type that already comes back from
	// pgx (see selectExpr's casts below) as a value dbx can bind directly:
	// text/citext/uuid (cast to ::text — see kindForPgType), boolean,
	// integer, bigint, bytea, and numeric/time (cast to ::float8/::text
	// respectively, for the same reason).
	kindScalar columnKind = iota
	// kindTimestamp is "timestamp with time zone" — read as time.Time
	// (no cast; casting to text would require re-parsing Postgres's own
	// text format just to get back to a time.Time for dbx.FormatTime, so
	// this is the one type left uncast — see selectExpr).
	kindTimestamp
	// kindArray is a Postgres array column (uuid[] or text[] in this
	// schema) — cast to ::text[] so it always comes back from pgx as
	// []interface{} of string regardless of element type (see
	// selectExpr), then rebound as dbx.StringArray. SQLite's own CHECK on
	// these columns is only json_valid(...), not element-type-aware, so a
	// StringArray write is valid for what the SQLite schema calls a
	// UUIDArray column too — see internal/dbx/jsonarray.go.
	kindArray
)

// kindForPgType classifies a schema_contract.json pg_type string. The
// full, exhaustive set of pg_type values appearing anywhere in this
// schema was enumerated directly from the contract before writing this
// (13 distinct values) — an unrecognized one is a schema change this
// package has not been taught about yet, not a value to silently guess at.
func kindForPgType(pgType string) (columnKind, error) {
	switch pgType {
	case "uuid", "text", "citext", "boolean", "integer", "bigint", "bytea",
		"numeric", "time without time zone", "jsonb":
		return kindScalar, nil
	case "timestamp with time zone":
		return kindTimestamp, nil
	case "_uuid[]", "_text[]":
		return kindArray, nil
	default:
		return 0, fmt.Errorf("unrecognized pg_type %q", pgType)
	}
}

// selectExpr returns the column reference to put in the source SELECT's
// column list — a cast for every pg_type whose default pgx decoding would
// otherwise need extra Go-side handling this package deliberately avoids
// (see each case's comment; verified empirically against a live Postgres
// instance before relying on it):
//   - uuid -> ::text: pgx's default decode is [16]byte, not a hyphenated
//     string; casting sidesteps a manual uuid.UUID conversion entirely.
//   - numeric -> ::float8: pgx's default decode is pgtype.Numeric
//     (arbitrary-precision); this schema's one numeric column
//     (ai_drafts.confidence) maps to a SQLite REAL, so float64 is already
//     the target representation — no precision this app relies on is lost
//     by asking Postgres to do that conversion instead of pgtype.Numeric.
//   - time without time zone -> ::text: pgx's default decode is
//     pgtype.Time (microseconds since midnight); casting gives the exact
//     "HH:MM:SS" string the SQLite schema stores directly (see
//     migrations/sqlite/0001_core.up.sql's respond_window_start/end).
//   - jsonb -> ::text: pgx's default decode is a Go map (re-encoding it
//     with encoding/json is a real option, but asking Postgres for its
//     own canonical text form is simpler and cannot introduce a
//     json.Marshal-vs-Postgres formatting difference).
//   - uuid[]/text[] -> ::text[]: makes both array types decode identically
//     ([]interface{} of string) — see kindArray's comment.
//   - timestamptz, boolean, integer, bigint, bytea: no cast; pgx's default
//     decode (time.Time, bool, int32, int64, []byte respectively) is
//     already what gets bound on the SQLite side (time.Time via
//     internal/dbx's own time-aware bindArgs).
func selectExpr(col column) string {
	switch col.PgType {
	case "uuid", "numeric", "time without time zone", "jsonb":
		return col.Name + "::text"
	case "_uuid[]", "_text[]":
		return col.Name + "::text[]"
	default:
		return col.Name
	}
}

// bindValue converts one already-cast pgx value (see selectExpr) into
// what dbx.Tx.Exec should bind for column. v is nil for SQL NULL.
func bindValue(kind columnKind, v any) (any, error) {
	if v == nil {
		if kind == kindArray {
			// Every array column in this schema is NOT NULL DEFAULT '{}'
			// on both sides (see internal/dbx/jsonarray.go) — a NULL
			// source value should not occur, but binding a real empty
			// array instead of a literal SQL NULL keeps that guarantee
			// even if it somehow did, rather than crashing the whole
			// import on a NOT NULL violation over one defensive column.
			return dbx.StringArray(nil), nil
		}
		return nil, nil
	}

	switch kind {
	case kindTimestamp:
		t, ok := v.(time.Time)
		if !ok {
			return nil, fmt.Errorf("pgimport: expected time.Time, got %T", v)
		}
		return t.UTC(), nil
	case kindArray:
		raw, ok := v.([]interface{})
		if !ok {
			return nil, fmt.Errorf("pgimport: expected []interface{} for an array column, got %T", v)
		}
		strs := make([]string, len(raw))
		for i, e := range raw {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("pgimport: array element %d: expected string, got %T", i, e)
			}
			strs[i] = s
		}
		return dbx.StringArray(strs), nil
	default: // kindScalar
		return v, nil
	}
}
