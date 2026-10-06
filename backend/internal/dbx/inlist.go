package dbx

import (
	"strconv"
	"strings"
)

// MaxInList is the most values one IN (...) list should carry. Callers whose
// input is unbounded split it with slices.Chunk(values, MaxInList) and run one
// statement per chunk: far below either engine's bound-parameter limit, and the
// statement text stays short.
const MaxInList = 500

// InList builds the placeholder list for `x IN <list>`: one $n per value,
// numbered after the arguments the statement has already bound. It returns the
// list text, "($3, $4, $5)", and args extended with the values — a fresh slice,
// never the caller's backing array.
//
//	list, args := dbx.InList([]any{orgID}, ids)
//	rows, err := db.Query(ctx, `SELECT ... WHERE organization_id = $1 AND id IN `+list, args...)
//
// An empty values yields "(NULL)", which is valid SQL on both engines and
// matches nothing, so a caller that forgot to return early gets no rows instead
// of the syntax error `IN ()`. (`NOT IN (NULL)` is also never true: a caller that
// wants "everything outside the list" must handle the empty case itself.) The
// list text contains only placeholders, never a value, so concatenating it into
// a statement is injection-safe.
func InList[T any](args []any, values []T) (string, []any) {
	if len(values) == 0 {
		return "(NULL)", args
	}
	out := make([]any, len(args), len(args)+len(values))
	copy(out, args)
	var b strings.Builder
	b.WriteByte('(')
	for i, v := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		out = append(out, v)
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(len(out)))
	}
	b.WriteByte(')')
	return b.String(), out
}
