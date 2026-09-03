package extractor_test

import (
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
)

// TestCapabilities_MatchesDoc pins Capabilities()'s output against
// the expected provider capability matrix (the provider per-input-type matrix)
// — if a provider's Supports() ever changes, this test forces the capability
// definition (or this test) to be updated in the same change, rather than
// silently drifting.
func TestCapabilities_MatchesDoc(t *testing.T) {
	want := map[string]map[extractor.Family]bool{
		"native": {
			extractor.FamilyURL: true, extractor.FamilyText: true, extractor.FamilyDOCX: true,
			extractor.FamilyPDF: false, extractor.FamilyImage: true,
		},
		"firecrawl": {
			extractor.FamilyURL: true, extractor.FamilyText: false, extractor.FamilyDOCX: false,
			extractor.FamilyPDF: false, extractor.FamilyImage: false,
		},
		"llamaparse": {
			extractor.FamilyURL: false, extractor.FamilyText: true, extractor.FamilyDOCX: true,
			extractor.FamilyPDF: true, extractor.FamilyImage: false,
		},
	}

	got := extractor.Capabilities()
	if len(got) != len(want) {
		t.Fatalf("Capabilities() returned %d providers, want %d", len(got), len(want))
	}
	for _, cp := range got {
		wantFam, ok := want[cp.Name]
		if !ok {
			t.Fatalf("Capabilities() returned unexpected provider %q", cp.Name)
		}
		for _, f := range extractor.Families {
			if cp.Families[f] != wantFam[f] {
				t.Errorf("%s.Families[%s] = %v, want %v (capability matrix disagrees)", cp.Name, f, cp.Families[f], wantFam[f])
			}
		}
	}
}

// TestCapabilities_ProbingNeverPanics guards the "construct with an empty
// key and a nil *http.Client purely to probe" contract Capabilities' own
// doc comment promises — a future provider whose Supports() dereferences
// its client/key would panic here first, not in production.
func TestCapabilities_ProbingNeverPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Capabilities() panicked: %v", r)
		}
	}()
	if got := extractor.Capabilities(); len(got) == 0 {
		t.Fatal("Capabilities() returned no providers")
	}
}
