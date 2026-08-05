package dbx

import "time"

// TimeLayout is the one canonical on-disk representation for every
// timestamp column: UTC, always exactly 3 fractional digits, so lexical
// string ordering equals chronological ordering and the stored text is
// directly comparable to strftime('%Y-%m-%d %H:%M:%f','now') (SQLite's %f
// is likewise always exactly 3 digits). See dbtime_test.go for the pinned
// round-trip/ordering/SQL-comparability contract.
const TimeLayout = "2006-01-02 15:04:05.000"

// FormatTime renders t in the canonical on-disk representation.
func FormatTime(t time.Time) string {
	return t.UTC().Format(TimeLayout)
}

// ParseTime parses the canonical on-disk representation back into a Time in
// UTC.
func ParseTime(s string) (time.Time, error) {
	return time.Parse(TimeLayout, s)
}

// bindArgs rewrites time.Time / *time.Time query arguments to their
// canonical string form before they reach the driver — centrally, so a
// ported call site can keep passing time.Time values exactly as it did
// against pgx (pgx has its own equivalent internal encoding; this is dbx's).
// Every other argument type passes through unchanged, including types that
// already implement driver.Valuer (uuid.UUID, the JSON-array helpers).
func bindArgs(args []any) []any {
	var out []any // allocated lazily; the overwhelmingly common case has no time.Time args at all
	for i, a := range args {
		var replacement any
		switch v := a.(type) {
		case time.Time:
			replacement = FormatTime(v)
		case *time.Time:
			if v == nil {
				replacement = nil
			} else {
				replacement = FormatTime(*v)
			}
		default:
			continue
		}
		if out == nil {
			out = make([]any, len(args))
			copy(out, args)
		}
		out[i] = replacement
	}
	if out == nil {
		return args
	}
	return out
}
