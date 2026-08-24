package chatkb

// Real-vs-draft comparison. This is computed in Go, never asked of the
// model: "the draft price is 1,200 ₸ lower" is arithmetic on two retrieved
// values, and a model that got it wrong would be confidently wrong in a way
// nobody downstream could catch.

// ChangeType says how an entity differs between the two states.
type ChangeType string

const (
	// ChangeAdded — staged in the draft, does not exist live yet.
	ChangeAdded ChangeType = "added"
	// ChangeRemoved — live today, staged for deletion in the draft.
	ChangeRemoved ChangeType = "removed"
	// ChangeUpdated — exists in both, with at least one field differing.
	ChangeUpdated ChangeType = "updated"
)

// FieldDiff is one field whose value is not the same in both states. An
// empty Real or Draft means the field is unset (or the record absent) on
// that side — never "unknown".
type FieldDiff struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Real  string `json:"real"`
	Draft string `json:"draft"`
}

// Difference is one entity that is not identical across the two states,
// carrying both sides in full so a caller can render either without a second
// lookup. Real is nil for an addition, Draft is nil for a removal.
type Difference struct {
	Kind   Kind        `json:"kind"`
	Key    string      `json:"key"`
	Title  string      `json:"title"`
	Change ChangeType  `json:"change"`
	Real   *Record     `json:"real"`
	Draft  *Record     `json:"draft"`
	Fields []FieldDiff `json:"fields"`
}

// Differences reports every entity that differs between the retrieved real
// and draft states, in a deterministic order: entities that exist live
// first (in retrieval order), then draft-only additions. An entity present
// in both with identical fields does not appear at all — the whole point is
// that "nothing pending here" is expressible.
func (r Result) Differences() []Difference {
	draftByID := r.Draft.index()
	seen := make(map[recordID]bool, len(r.Draft.Records))

	var out []Difference
	for _, real := range r.Real.Records {
		id := recordID{Kind: real.Kind, Key: real.Key}
		draft, ok := draftByID[id]
		if !ok {
			removed := real
			out = append(out, Difference{
				Kind: real.Kind, Key: real.Key, Title: real.Title, Change: ChangeRemoved,
				Real: &removed, Fields: fieldDiffs(real.Fields, nil),
			})
			continue
		}
		seen[id] = true
		diffs := fieldDiffs(real.Fields, draft.Fields)
		if len(diffs) == 0 {
			continue
		}
		realCopy, draftCopy := real, draft
		out = append(out, Difference{
			Kind: real.Kind, Key: real.Key, Title: orKey(draft.Title, real.Title), Change: ChangeUpdated,
			Real: &realCopy, Draft: &draftCopy, Fields: diffs,
		})
	}
	// Everything left is draft-only: the loop above visited every real
	// record, so an unseen id had no live counterpart to pair with.
	for _, draft := range r.Draft.Records {
		if seen[recordID{Kind: draft.Kind, Key: draft.Key}] {
			continue
		}
		added := draft
		out = append(out, Difference{
			Kind: draft.Kind, Key: draft.Key, Title: draft.Title, Change: ChangeAdded,
			Draft: &added, Fields: fieldDiffs(nil, draft.Fields),
		})
	}
	return out
}

// fieldDiffs pairs two field lists by key and returns only the pairs that
// disagree. Field order follows the real side first (the order
// recordsFromView produced), then any key that exists only in the draft, so
// a diff reads in the same order as the record it describes.
func fieldDiffs(real, draft []Field) []FieldDiff {
	draftByKey := make(map[string]Field, len(draft))
	for _, f := range draft {
		draftByKey[f.Key] = f
	}
	seen := make(map[string]bool, len(real))

	var out []FieldDiff
	for _, rf := range real {
		seen[rf.Key] = true
		df := draftByKey[rf.Key] // zero Field when the draft dropped this field
		if rf.Value == df.Value {
			continue
		}
		out = append(out, FieldDiff{Key: rf.Key, Label: rf.Label, Real: rf.Value, Draft: df.Value})
	}
	for _, df := range draft {
		if seen[df.Key] {
			continue
		}
		out = append(out, FieldDiff{Key: df.Key, Label: df.Label, Real: "", Draft: df.Value})
	}
	return out
}
