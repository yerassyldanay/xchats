package store

import (
	"sync/atomic"
	"time"
)

// rowStamp is the timestamp of the i-th row a method writes in one go: one millisecond after
// the one before. Timestamps are Unix milliseconds, so rows sharing one value would tie under
// ORDER BY created_at, and PostgreSQL returns ties in no particular order; spacing them keeps
// the order the rows were given in (recipients are listed and sent in the order they were
// imported). The skew is bounded by the number of rows in the batch.
func rowStamp(base time.Time, i int) time.Time {
	return base.Add(time.Duration(i) * time.Millisecond)
}

// lastEventMs is the last millisecond eventStamp handed out.
var lastEventMs atomic.Int64

// eventStamp is the timestamp of an entry appended to a log that is read newest first (the
// campaign timeline): strictly later than the previous entry this process wrote, so two entries
// written inside one millisecond keep their order. SQLite finishes a write in well under a
// millisecond. Entries arrive a handful at a time, so the stamp stays within a few milliseconds of
// the wall clock.
func eventStamp() time.Time {
	for {
		prev := lastEventMs.Load()
		next := time.Now().UnixMilli()
		if next <= prev {
			next = prev + 1
		}
		if lastEventMs.CompareAndSwap(prev, next) {
			return time.UnixMilli(next).UTC()
		}
	}
}
