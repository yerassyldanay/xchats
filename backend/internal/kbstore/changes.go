package kbstore

// changeKinds is THE mapping between the HTTP-facing plural entity vocabulary
// (ApproveSelector.Kind, the Playground draft-write URLs, the review payload)
// and the blob's singular DraftDelete.Kind. Every consumer — selectApproved,
// CancelChange, DraftChanges, audit summaries and error text — goes through
// SingularDeleteKind/PluralChangeKind rather than manipulating the string
// (the previous strings.TrimSuffix(kind, "s") turned "policies" into
// "policie"), so a future entity kind can never silently inherit that.
// "config" is deliberately absent: it has no DraftDelete.Kind — it is never
// deletable, only ever patched or approved as a whole pending edit.
var changeKinds = map[string]string{
	"topics": "topic", "tariffs": "tariff", "products": "product",
	"contacts": "contact", "policies": "policy", "delivery_zones": "delivery_zone",
}

// singularToPlural is changeKinds inverted — built once at init so
// PluralChangeKind is a plain map lookup, not a linear scan.
var singularToPlural = func() map[string]string {
	m := make(map[string]string, len(changeKinds))
	for plural, singular := range changeKinds {
		m[singular] = plural
	}
	return m
}()

// SingularDeleteKind maps an HTTP-facing plural kind (as used by
// ApproveSelector.Kind and the /playground/draft/* URLs) to the blob's
// singular DraftDelete.Kind vocabulary. ok is false for "config" (never
// deletable) and any unrecognized kind.
func SingularDeleteKind(kind string) (string, bool) {
	s, ok := changeKinds[kind]
	return s, ok
}

// PluralChangeKind is SingularDeleteKind's inverse — the vocabulary
// DraftChangeSet.Deletes and the frontend-facing review payload use. Returns
// singular unchanged if it is not one of changeKinds' values (defensive; every
// real DraftDelete.Kind in the blob was itself written via one of the
// singular values above).
func PluralChangeKind(singular string) string {
	if p, ok := singularToPlural[singular]; ok {
		return p
	}
	return singular
}

// IsSingletonKind reports whether the singular kind identifies a true
// singleton (contact | policy) — Key is ignored on the read/match side
// entirely for these: there is exactly one row per org, so the natural key
// is the org, not a value carried on the row. Writers keep writing whatever
// key they write (the Playground path's own "", MCPDelete's NaturalKeyMain
// "main", the approve route's domain.ContactSlug/PolicySlug) — this just
// means matching never compares it.
func IsSingletonKind(singular string) bool {
	return singular == "contact" || singular == "policy"
}

// deleteMatches reports whether a DraftDelete entry addresses the given
// singular kind + key — the one comparison every consumer of
// DraftBlob.Deletes should use instead of reimplementing the singleton-key
// exception inline.
func deleteMatches(d DraftDelete, singular, key string) bool {
	if d.Kind != singular {
		return false
	}
	if IsSingletonKind(singular) {
		return true
	}
	return d.Key == key
}

// deletedSingleton reports whether the blob carries a delete marker for the
// given singleton kind, regardless of which key spelling wrote it. Replaces
// the old deleted["contact:"]/deleted["policy:"] map lookups, which only
// matched a literal empty key and so never saw a marker MCPDelete wrote
// (key = NaturalKeyMain, "main").
func deletedSingleton(blob DraftBlob, singular string) bool {
	for _, d := range blob.Deletes {
		if d.Kind == singular {
			return true
		}
	}
	return false
}
