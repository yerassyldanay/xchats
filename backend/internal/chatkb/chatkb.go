// Package chatkb is the Knowledge Base retrieval service behind the chat
// assistant (/chat): it turns the org's KB into a provenance-tagged,
// prompt-ready form and into the structured components the chat UI renders
// as cards.
//
// The one rule this package exists to enforce is that REAL and DRAFT never
// merge. Every Record carries the Source it came from, the two states are
// retrieved through separate calls and rendered into separate, explicitly
// labelled prompt sections, and the real-vs-draft difference is computed
// HERE, in Go, rather than left to the model to notice. An assistant that
// quoted a pending price as the current one would be worse than useless, so
// "which state is this?" is never an inference.
//
// Retrieval itself sits behind the Service interface. v1's implementation
// (StoreService) returns the whole KB for both states — full-context
// injection, correct and trivially auditable at the sizes a single business's
// KB actually reaches. The `query` parameter is already part of the contract
// so that a semantic/vector implementation can replace StoreService without
// any caller changing: internal/chat depends on this interface, never on
// kbstore.
package chatkb

import (
	"context"

	"github.com/google/uuid"
)

// Source names which KB state a record was read from. These are the exact
// labels the prompt uses, so a model quoting its source quotes one of these.
type Source string

const (
	// SourceReal is the live KB — what the assistant actually answers
	// customers from today.
	SourceReal Source = "REAL_KB"
	// SourceDraft is the pending KB — edits staged in the Playground draft
	// that no one has approved yet.
	SourceDraft Source = "DRAFT_KB"
)

// Kind is a KB entity kind, in the SAME plural vocabulary the rest of
// xchats already addresses these entities by: kbstore.PluralChangeKind's
// values, the /playground/draft/* URLs, and the frontend's ENTITY_META keys.
// Reusing it (rather than minting a chat-only vocabulary) is what lets a chat
// card render with the same icon and localized entity name the KB pages use.
type Kind string

const (
	KindTopics   Kind = "topics"
	KindProducts Kind = "products"
	KindTariffs  Kind = "tariffs"
	KindZones    Kind = "delivery_zones"
	KindContacts Kind = "contacts"
	KindPolicies Kind = "policies"
	// KindConfig is the assistant's own configuration (persona, mission,
	// guardrails, ...) — "config" in the same vocabulary, and the one kind
	// that is a true singleton with no natural key of its own.
	KindConfig Kind = "config"
)

// Field is one labelled value on a record. Label is human-facing (it is what
// a card row and a prompt line both show); Key is stable and machine-facing,
// so a comparison can line two records' fields up across states.
type Field struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Record is one KB entity flattened to labelled fields, tagged with the
// state it came from. Key is the entity's natural key (a topic slug, a
// product/tariff/zone ref, or the singleton's fixed name) — the join key
// between a real record and its draft counterpart.
type Record struct {
	Kind   Kind    `json:"kind"`
	Key    string  `json:"key"`
	Title  string  `json:"title"`
	Source Source  `json:"source"`
	Fields []Field `json:"fields"`
}

// Snapshot is everything retrieved from ONE state.
type Snapshot struct {
	Source  Source   `json:"source"`
	Records []Record `json:"records"`
}

// Service retrieves KB records for a query. The two states are separate
// methods rather than one call with a source argument so that a caller
// cannot accidentally ask for "the KB" and get an ambiguous mixture — asking
// for real and asking for draft are different questions with different
// answers.
//
// v1's implementation ignores query (see the package doc); a semantic
// implementation would use it to rank. Either way the caller's contract is
// the same: what comes back is scoped to orgID and tagged with its source.
type Service interface {
	SearchReal(ctx context.Context, orgID uuid.UUID, query string) (Snapshot, error)
	SearchDraft(ctx context.Context, orgID uuid.UUID, query string) (Snapshot, error)
}

// Result is both states retrieved for one question, never merged.
type Result struct {
	Real  Snapshot `json:"real"`
	Draft Snapshot `json:"draft"`
}

// Retrieve runs both halves of a Service for one query. A failure on either
// side fails the whole retrieval: answering from real while silently
// omitting draft (or the reverse) is exactly the mixing this package exists
// to prevent.
func Retrieve(ctx context.Context, svc Service, orgID uuid.UUID, query string) (Result, error) {
	real, err := svc.SearchReal(ctx, orgID, query)
	if err != nil {
		return Result{}, err
	}
	draft, err := svc.SearchDraft(ctx, orgID, query)
	if err != nil {
		return Result{}, err
	}
	return Result{Real: real, Draft: draft}, nil
}

// index maps a snapshot's records by their (kind, key) identity — the join
// used by every real-vs-draft comparison in this package.
func (s Snapshot) index() map[recordID]Record {
	out := make(map[recordID]Record, len(s.Records))
	for _, r := range s.Records {
		out[recordID{Kind: r.Kind, Key: r.Key}] = r
	}
	return out
}

type recordID struct {
	Kind Kind
	Key  string
}
