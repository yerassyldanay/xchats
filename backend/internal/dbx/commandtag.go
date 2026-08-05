package dbx

import "database/sql"

// CommandTag mirrors the one method call sites ported from pgconn.CommandTag
// actually use: RowsAffected() with no error return. modernc's sql.Result
// never fails RowsAffected in practice (unlike some drivers that can't report
// it for certain statements), so the error from the underlying sql.Result is
// folded into 0 rather than threaded through a second return value.
type CommandTag struct {
	rowsAffected int64
}

// RowsAffected returns the number of rows the statement affected.
func (c CommandTag) RowsAffected() int64 { return c.rowsAffected }

func newCommandTag(res sql.Result) CommandTag {
	n, _ := res.RowsAffected()
	return CommandTag{rowsAffected: n}
}
