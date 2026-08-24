package chatkb

import (
	"strings"
	"testing"
)

// vitaminReal/vitaminDraft are the spec's own worked example (§5): one
// product whose price is lower in the draft than in the live KB.
func vitaminResult() Result {
	real := Snapshot{Source: SourceReal, Records: []Record{
		{Kind: KindProducts, Key: "vitamin-d", Title: "Vitamin D", Source: SourceReal, Fields: []Field{
			{Key: "name", Label: "Name", Value: "Vitamin D"},
			{Key: "price", Label: "Price", Value: "12 000 KZT"},
			{Key: "in_stock", Label: "In stock", Value: "yes"},
		}},
		{Kind: KindProducts, Key: "omega-3", Title: "Omega 3", Source: SourceReal, Fields: []Field{
			{Key: "name", Label: "Name", Value: "Omega 3"},
			{Key: "price", Label: "Price", Value: "8 000 KZT"},
		}},
	}}
	draft := Snapshot{Source: SourceDraft, Records: []Record{
		{Kind: KindProducts, Key: "vitamin-d", Title: "Vitamin D", Source: SourceDraft, Fields: []Field{
			{Key: "name", Label: "Name", Value: "Vitamin D"},
			{Key: "price", Label: "Price", Value: "10 800 KZT"},
			{Key: "in_stock", Label: "In stock", Value: "yes"},
		}},
		{Kind: KindProducts, Key: "omega-3", Title: "Omega 3", Source: SourceDraft, Fields: []Field{
			{Key: "name", Label: "Name", Value: "Omega 3"},
			{Key: "price", Label: "Price", Value: "8 000 KZT"},
		}},
		{Kind: KindProducts, Key: "magnesium", Title: "Magnesium", Source: SourceDraft, Fields: []Field{
			{Key: "name", Label: "Name", Value: "Magnesium"},
			{Key: "price", Label: "Price", Value: "6 500 KZT"},
		}},
	}}
	return Result{Real: real, Draft: draft}
}

func TestDifferencesReportsOnlyWhatChanged(t *testing.T) {
	diffs := vitaminResult().Differences()
	if len(diffs) != 2 {
		t.Fatalf("len(differences) = %d, want 2 (one update, one addition); got %+v", len(diffs), diffs)
	}

	updated := diffs[0]
	if updated.Key != "vitamin-d" || updated.Change != ChangeUpdated {
		t.Fatalf("differences[0] = %s/%s, want vitamin-d/updated", updated.Key, updated.Change)
	}
	if len(updated.Fields) != 1 || updated.Fields[0].Key != "price" {
		t.Fatalf("vitamin-d diff fields = %+v, want only price", updated.Fields)
	}
	if updated.Fields[0].Real != "12 000 KZT" || updated.Fields[0].Draft != "10 800 KZT" {
		t.Errorf("price diff = %q -> %q, want %q -> %q",
			updated.Fields[0].Real, updated.Fields[0].Draft, "12 000 KZT", "10 800 KZT")
	}
	if updated.Real == nil || updated.Draft == nil {
		t.Error("an updated difference must carry both sides in full")
	}

	added := diffs[1]
	if added.Key != "magnesium" || added.Change != ChangeAdded {
		t.Fatalf("differences[1] = %s/%s, want magnesium/added", added.Key, added.Change)
	}
	if added.Real != nil {
		t.Error("an addition must have no real side")
	}
	// Omega 3 is identical in both states and must not appear at all — the
	// whole point is that "nothing pending here" is expressible.
	for _, d := range diffs {
		if d.Key == "omega-3" {
			t.Errorf("omega-3 is identical in both states but was reported as %s", d.Change)
		}
	}
}

func TestDifferencesReportsRemovals(t *testing.T) {
	result := Result{
		Real: Snapshot{Source: SourceReal, Records: []Record{
			{Kind: KindTopics, Key: "returns", Title: "Returns", Source: SourceReal,
				Fields: []Field{{Key: "title", Label: "Title", Value: "Returns"}}},
		}},
		Draft: Snapshot{Source: SourceDraft},
	}
	diffs := result.Differences()
	if len(diffs) != 1 || diffs[0].Change != ChangeRemoved {
		t.Fatalf("differences = %+v, want one removal", diffs)
	}
	if diffs[0].Draft != nil {
		t.Error("a removal must have no draft side")
	}
}

func TestDifferencesIsEmptyWhenStatesMatch(t *testing.T) {
	same := []Record{{Kind: KindProducts, Key: "a", Title: "A", Fields: []Field{{Key: "price", Value: "1"}}}}
	result := Result{
		Real:  Snapshot{Source: SourceReal, Records: same},
		Draft: Snapshot{Source: SourceDraft, Records: same},
	}
	if diffs := result.Differences(); len(diffs) != 0 {
		t.Errorf("differences = %+v, want none", diffs)
	}
}

// The prompt's whole safety property is that the two states are separately
// labelled and never interleaved.
func TestRenderKeepsStatesSeparateAndLabelled(t *testing.T) {
	out := Render(vitaminResult(), RenderOptions{})

	realAt := strings.Index(out, "=== REAL (LIVE) KNOWLEDGE BASE ===")
	draftAt := strings.Index(out, "=== DRAFT (PENDING CHANGES APPLIED) KNOWLEDGE BASE ===")
	pendingAt := strings.Index(out, "=== PENDING DRAFT CHANGES (REAL vs DRAFT) ===")
	if pendingAt != 0 {
		t.Errorf("the pending-changes block must come first, found at index %d", pendingAt)
	}
	if realAt < 0 || draftAt < 0 || realAt > draftAt {
		t.Fatalf("expected a REAL section before a DRAFT section; real=%d draft=%d\n%s", realAt, draftAt, out)
	}

	// Every record line carries its own source tag, so a line quoted out of
	// the middle of a long section still says which state it belongs to.
	realSection, draftSection := out[realAt:draftAt], out[draftAt:]
	if strings.Contains(realSection, string(SourceDraft)) {
		t.Error("the REAL section mentions DRAFT_KB — the two states must never interleave")
	}
	if strings.Contains(draftSection, string(SourceReal)) {
		t.Error("the DRAFT section mentions REAL_KB — the two states must never interleave")
	}
	if !strings.Contains(realSection, "12 000 KZT") {
		t.Error("the REAL section is missing the live price")
	}
	if !strings.Contains(draftSection, "10 800 KZT") {
		t.Error("the DRAFT section is missing the draft price")
	}
	// The difference is handed over as a finding, not left for the model to
	// spot by comparing two long lists.
	if !strings.Contains(out[:realAt], "10 800 KZT") || !strings.Contains(out[:realAt], "12 000 KZT") {
		t.Errorf("the pending-changes block must state both values:\n%s", out[:realAt])
	}
}

func TestRenderSaysSoWhenNothingIsPending(t *testing.T) {
	same := []Record{{Kind: KindProducts, Key: "a", Title: "A", Fields: []Field{{Key: "price", Value: "1"}}}}
	out := Render(Result{
		Real:  Snapshot{Source: SourceReal, Records: same},
		Draft: Snapshot{Source: SourceDraft, Records: same},
	}, RenderOptions{})
	if !strings.Contains(out, "identical") {
		t.Errorf("expected an explicit 'draft is identical' statement:\n%s", out)
	}
}

func TestRenderIsEmptyForAnEmptyKnowledgeBase(t *testing.T) {
	if out := Render(Result{}, RenderOptions{}); out != "" {
		t.Errorf("Render of an empty KB = %q, want \"\" so the caller can say so explicitly", out)
	}
}

// A truncated view must announce itself: a model that believes a partial KB
// is the whole one will confidently report things as missing.
func TestRenderMarksTruncation(t *testing.T) {
	var records []Record
	for _, key := range []string{"a", "b", "c", "d"} {
		records = append(records, Record{Kind: KindProducts, Key: key, Title: key, Fields: []Field{
			{Key: "description", Label: "Description", Value: strings.Repeat("x", 200)},
		}})
	}
	out := Render(Result{Real: Snapshot{Source: SourceReal, Records: records}}, RenderOptions{MaxChars: 250})
	if !strings.Contains(out, "records omitted") {
		t.Errorf("a truncated section must say so:\n%s", out)
	}
}

func TestComponentsComparesTheNamedEntity(t *testing.T) {
	components := Components(vitaminResult(), "What is the current and draft price of Vitamin D?")
	if len(components) != 1 {
		t.Fatalf("len(components) = %d, want 1; got %+v", len(components), components)
	}
	if components[0].Type != ComponentComparison {
		t.Fatalf("component type = %s, want %s", components[0].Type, ComponentComparison)
	}
	data, okData := components[0].Data.(Difference)
	if !okData {
		t.Fatalf("component data is %T, want Difference", components[0].Data)
	}
	if data.Key != "vitamin-d" {
		t.Errorf("compared %q, want vitamin-d", data.Key)
	}
}

// "What changed in the draft?" names no entity — the answer is the whole
// pending set.
func TestComponentsComparesEverythingPendingForADraftQuestion(t *testing.T) {
	for _, query := range []string{
		"What changes are pending in the draft?",
		"Что изменилось в черновике?",
	} {
		components := Components(vitaminResult(), query)
		if len(components) != 2 {
			t.Fatalf("%q: len(components) = %d, want 2 (both pending changes)", query, len(components))
		}
		for _, c := range components {
			if c.Type != ComponentComparison {
				t.Errorf("%q: component type = %s, want %s", query, c.Type, ComponentComparison)
			}
		}
	}
}

func TestComponentsShowsAnItemForOneUnchangedMatch(t *testing.T) {
	components := Components(vitaminResult(), "how much is Omega 3?")
	if len(components) != 1 || components[0].Type != ComponentItem {
		t.Fatalf("components = %+v, want a single kb_item", components)
	}
	data, okData := components[0].Data.(ItemData)
	if !okData {
		t.Fatalf("component data is %T, want ItemData", components[0].Data)
	}
	if data.Record.Key != "omega-3" {
		t.Errorf("item = %q, want omega-3", data.Record.Key)
	}
	if data.Record.Source != SourceReal {
		t.Errorf("item source = %s, want %s — an unqualified question is answered from the live KB", data.Record.Source, SourceReal)
	}
}

func TestComponentsShowsAListForSeveralMatches(t *testing.T) {
	components := Components(vitaminResult(), "show me the price of everything")
	if len(components) != 1 || components[0].Type != ComponentList {
		t.Fatalf("components = %+v, want a single kb_list", components)
	}
	data, okData := components[0].Data.(ListData)
	if !okData {
		t.Fatalf("component data is %T, want ListData", components[0].Data)
	}
	if len(data.Records) < 2 {
		t.Errorf("list holds %d records, want several", len(data.Records))
	}
	if data.Kind != KindProducts {
		t.Errorf("list kind = %q, want %q", data.Kind, KindProducts)
	}
}

// A question the KB says nothing about gets no card. Showing an unrelated
// record would be worse than showing nothing: it reads as an answer.
func TestComponentsAreEmptyForAnUnrelatedQuestion(t *testing.T) {
	if components := Components(vitaminResult(), "hello, how are you today?"); len(components) != 0 {
		t.Errorf("components = %+v, want none", components)
	}
}

func TestComponentsAreCapped(t *testing.T) {
	var real, draft []Record
	for _, key := range []string{"a", "b", "c", "d", "e", "f"} {
		real = append(real, Record{Kind: KindProducts, Key: key, Title: key, Source: SourceReal,
			Fields: []Field{{Key: "price", Label: "Price", Value: "1"}}})
		draft = append(draft, Record{Kind: KindProducts, Key: key, Title: key, Source: SourceDraft,
			Fields: []Field{{Key: "price", Label: "Price", Value: "2"}}})
	}
	result := Result{Real: Snapshot{Source: SourceReal, Records: real}, Draft: Snapshot{Source: SourceDraft, Records: draft}}
	components := Components(result, "what is pending in the draft?")
	if len(components) != maxComparisons {
		t.Errorf("len(components) = %d, want the cap of %d", len(components), maxComparisons)
	}
}

func TestTokenizeIgnoresShortAndNonWordTokens(t *testing.T) {
	got := tokenize("Что с ценой Vitamin D? — 12,000 ₸")
	want := map[string]bool{"что": true, "ценой": true, "vitamin": true, "000": true}
	for _, tok := range got {
		if !want[tok] {
			t.Errorf("unexpected token %q in %v", tok, got)
		}
	}
	for tok := range want {
		if !containsString(got, tok) {
			t.Errorf("missing token %q in %v", tok, got)
		}
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
