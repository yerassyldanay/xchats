package chatkb

// Structured KB components — the part of an answer the UI draws as a card
// rather than reading out of the model's prose.
//
// These are built HERE, from retrieved data, and only then handed to the
// frontend (spec §7: "the backend should return structured data rather than
// relying on the frontend to parse arbitrary LLM text"). Nothing in this file
// looks at what the model said: a price shown on a card is the price the KB
// holds, whether or not the model quoted it correctly in the sentence above
// it.

// Component types, matching the vocabulary the Vue renderer switches on.
const (
	// ComponentItem is one KB record, shown as a single card with a source
	// badge.
	ComponentItem = "kb_item"
	// ComponentList is several related records of one kind.
	ComponentList = "kb_list"
	// ComponentComparison is one entity's real and draft states side by
	// side, with the per-field differences called out.
	ComponentComparison = "kb_comparison"
)

// Component is one structured element attached to an assistant turn. Data's
// concrete type is determined by Type: ItemData for ComponentItem, ListData
// for ComponentList, and Difference (diff.go) for ComponentComparison — a
// comparison card IS one entity's real-vs-draft difference, so it carries
// that type rather than a copy of it under another name.
type Component struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// ItemData backs ComponentItem.
type ItemData struct {
	Record Record `json:"record"`
}

// ListData backs ComponentList. Kind and Source are hoisted out of the
// records so the card can title itself ("Products · REAL_KB") without
// inspecting the list.
type ListData struct {
	Kind    Kind     `json:"kind"`
	Source  Source   `json:"source"`
	Records []Record `json:"records"`
}

// Component limits. A chat answer with a dozen cards under it is a report,
// not an answer — past a handful, cards stop helping and the prose does the
// work instead.
const (
	maxComparisons = 4
	maxListRecords = 8
)

// Components picks the structured elements for one question against one
// retrieval, in this order:
//
//  1. If the question names entities that have pending changes, compare
//     those — the most specific, most useful answer available.
//  2. If the question is about the draft state generally but names no
//     entity, compare everything that is pending.
//  3. Otherwise show what the question named, from the real KB: one match
//     becomes an item card, several become a list.
//
// A question that points at nothing in the KB gets no components at all,
// which is correct: an unrelated question should not be answered with a
// random card.
func Components(result Result, query string) []Component {
	differences := result.Differences()

	// A named entity's pending change is the highest-value card there is.
	if named := matchingDifferences(differences, result.Real.Records, query); len(named) > 0 {
		return comparisonComponents(named)
	}
	// "What's in the draft?" with nothing named — show the whole pending set.
	if hasDraftIntent(query) && len(differences) > 0 {
		return comparisonComponents(differences)
	}
	relevant := rank(result.Real.Records, query, maxListRecords)
	switch {
	case len(relevant) == 1:
		return []Component{{Type: ComponentItem, Data: ItemData{Record: relevant[0]}}}
	case len(relevant) > 1:
		return []Component{{Type: ComponentList, Data: ListData{
			Kind:    listKind(relevant),
			Source:  result.Real.Source,
			Records: relevant,
		}}}
	}
	return nil
}

// matchingDifferences narrows the pending set to the entities the question
// actually NAMED (rankIdentity, not rank — see its doc comment for the
// distinction). Draft-only additions have no live counterpart to match
// against, so they are matched from the draft side separately.
func matchingDifferences(differences []Difference, realRecords []Record, query string) []Difference {
	if len(differences) == 0 {
		return nil
	}
	named := make(map[recordID]bool)
	for _, r := range rankIdentity(realRecords, query, maxListRecords) {
		named[recordID{Kind: r.Kind, Key: r.Key}] = true
	}
	var addedOnly []Record
	for _, d := range differences {
		if d.Change == ChangeAdded && d.Draft != nil {
			addedOnly = append(addedOnly, *d.Draft)
		}
	}
	for _, r := range rankIdentity(addedOnly, query, maxListRecords) {
		named[recordID{Kind: r.Kind, Key: r.Key}] = true
	}

	out := make([]Difference, 0, len(differences))
	for _, d := range differences {
		if named[recordID{Kind: d.Kind, Key: d.Key}] {
			out = append(out, d)
		}
	}
	return out
}

func comparisonComponents(differences []Difference) []Component {
	if len(differences) > maxComparisons {
		differences = differences[:maxComparisons]
	}
	out := make([]Component, 0, len(differences))
	for _, d := range differences {
		out = append(out, Component{Type: ComponentComparison, Data: d})
	}
	return out
}

// listKind is the kind a list card labels itself with: the single kind its
// records share, or "" when the matches span several (the card then falls
// back to a neutral title rather than mislabelling itself).
func listKind(records []Record) Kind {
	if len(records) == 0 {
		return ""
	}
	kind := records[0].Kind
	for _, r := range records[1:] {
		if r.Kind != kind {
			return ""
		}
	}
	return kind
}
