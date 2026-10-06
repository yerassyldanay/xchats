package dbx

import "time"

// Every timestamp column is a BIGINT holding UTC Unix milliseconds, on both
// engines: integers compare, order and aggregate identically everywhere, and no
// column default or SQL clock is involved — the caller binds the instant.
//
// Domain code keeps working in time.Time. bindArgs converts a bound time.Time or
// *time.Time to its millisecond count, and Row.Scan / Rows.Scan turn the stored
// integer back into a UTC time.Time (see scan.go). Sub-millisecond precision is
// dropped on the way in, which is also what makes a value written and read back
// compare equal.

// bindArgs rewrites time.Time / *time.Time query arguments to Unix milliseconds
// before they reach the driver — centrally, so a call site just passes the
// time.Time it already has. Every other argument type passes through unchanged,
// including types that already implement driver.Valuer (uuid.UUID, the
// JSON-array helpers).
func bindArgs(args []any) []any {
	var out []any // allocated lazily; the overwhelmingly common case has no time.Time args at all
	for i, a := range args {
		var replacement any
		switch v := a.(type) {
		case time.Time:
			replacement = v.UnixMilli()
		case *time.Time:
			if v == nil {
				replacement = nil
			} else {
				replacement = v.UnixMilli()
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
