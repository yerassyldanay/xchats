package dbx

import (
	"database/sql"
	"fmt"
	"time"
)

// Scanner is satisfied by both *Row and *Rows — for helpers that scan one
// row's columns regardless of whether it came from QueryRow or mid-loop from
// a Query's *Rows (mirroring pgx.Row, an interface pgx.Rows also satisfies;
// dbx.Row and dbx.Rows are distinct concrete types instead, so callers that
// need to accept either — e.g. a scan helper shared between a single-row
// lookup and a list query's loop — take Scanner rather than either concrete
// type).
type Scanner interface {
	Scan(dest ...any) error
}

// Row wraps *sql.Row. Its only extra behavior over the stdlib type is
// Scan's time-awareness — see timeTarget.
type Row struct {
	r *sql.Row
}

// Scan copies the columns from the matched row into dest, exactly like
// sql.Row.Scan, except a *time.Time or **time.Time destination is parsed
// from the column's canonical on-disk text form (see TimeLayout) instead of
// requiring a driver-level time.Time conversion — the corresponding write
// path (bindArgs) is what put that text there in the first place.
func (row *Row) Scan(dest ...any) error {
	targets := findTimeTargets(dest)
	if len(targets) == 0 {
		return row.r.Scan(dest...)
	}
	scanDest, placeholders := substituteTimeTargets(dest, targets)
	if err := row.r.Scan(scanDest...); err != nil {
		return err
	}
	return applyTimeTargets(targets, placeholders)
}

// Rows wraps *sql.Rows with the same time-aware Scan as Row.
type Rows struct {
	r *sql.Rows
}

func (rows *Rows) Next() bool   { return rows.r.Next() }
func (rows *Rows) Close() error { return rows.r.Close() }
func (rows *Rows) Err() error   { return rows.r.Err() }

// Scan behaves exactly like Row.Scan (see above) for the current row.
func (rows *Rows) Scan(dest ...any) error {
	targets := findTimeTargets(dest)
	if len(targets) == 0 {
		return rows.r.Scan(dest...)
	}
	scanDest, placeholders := substituteTimeTargets(dest, targets)
	if err := rows.r.Scan(scanDest...); err != nil {
		return err
	}
	return applyTimeTargets(targets, placeholders)
}

// timeTarget records one Scan destination that needs canonical-text ->
// time.Time conversion after the underlying driver Scan runs. Exactly one
// of direct/nullable is set, matching which of the two shapes ported call
// sites use: `CreatedAt time.Time` scanned via &x.CreatedAt (*time.Time), or
// `DeletedAt *time.Time` scanned via &x.DeletedAt (**time.Time, nil for SQL
// NULL).
type timeTarget struct {
	idx      int
	direct   *time.Time
	nullable **time.Time
}

func findTimeTargets(dest []any) []timeTarget {
	var out []timeTarget
	for i, d := range dest {
		switch v := d.(type) {
		case *time.Time:
			out = append(out, timeTarget{idx: i, direct: v})
		case **time.Time:
			out = append(out, timeTarget{idx: i, nullable: v})
		}
	}
	return out
}

// substituteTimeTargets returns a copy of dest with every timeTarget index
// replaced by a fresh *sql.NullString placeholder (in the same order as
// targets), so the driver scans canonical text rather than failing to
// convert it to a time.Time itself.
func substituteTimeTargets(dest []any, targets []timeTarget) ([]any, []sql.NullString) {
	scanDest := append([]any(nil), dest...)
	placeholders := make([]sql.NullString, len(targets))
	for i, t := range targets {
		scanDest[t.idx] = &placeholders[i]
	}
	return scanDest, placeholders
}

func applyTimeTargets(targets []timeTarget, placeholders []sql.NullString) error {
	for i, t := range targets {
		p := placeholders[i]
		if !p.Valid {
			if t.nullable != nil {
				*t.nullable = nil
				continue
			}
			return fmt.Errorf("dbx: scanning NULL into non-nullable time.Time (column index %d)", t.idx)
		}
		parsed, err := ParseTime(p.String)
		if err != nil {
			return fmt.Errorf("dbx: parse time %q (column index %d): %w", p.String, t.idx, err)
		}
		if t.direct != nil {
			*t.direct = parsed
		} else {
			*t.nullable = &parsed
		}
	}
	return nil
}
