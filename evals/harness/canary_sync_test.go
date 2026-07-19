package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// canarySyncFiles lists every hand-copied canary tests.yaml that must stay a byte-for-byte
// field match of its source test in common/shop-questions.yaml, plus the EXACT ordered ID
// list each file is expected to carry — pinning the list (not just iterating whatever the
// file happens to contain right now) means an accidentally emptied or truncated canary
// file fails loudly instead of vacuously passing every check below. lang-canary-v2/v3 and
// escalation-canary-v2 all point their scenario.yaml `tests:` field at v1's tests.yaml
// (see their scenario.yaml files), so guarding these four physical files transitively
// covers all seven canary scenarios — no separate entry is needed for the v2/v3 variants.
var canarySyncFiles = map[string][]string{
	filepath.Join("..", "scenarios", "escalation-canary-v1", "tests.yaml"): {
		"9. off-KB city coverage",
		"10. refund request",
		"18. wrong understanding, asks about a product we don't sell",
		"19. wrong understanding, asks about a service we don't offer",
		"21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim",
		"16. follow-up with history, needs delivery cost",
	},
	filepath.Join("..", "scenarios", "lang-canary-v1", "tests.yaml"): {
		"1. price question, Russian",
		"2. price question, Kazakh",
		"3. delivery cost + time, Kazakh",
		"9. off-KB city coverage",
		"20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh)",
		"21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim",
		"16. follow-up with history, needs delivery cost",
	},
	// Test 20 sits in the KK sibling (not RU): its question clause is Kazakh, so the
	// dominant-language rule — and detectLang, which mirrors it — routes it "kk". It
	// lived in the RU sibling under the old blanket mixed→Russian rule.
	filepath.Join("..", "scenarios", "lang-canary-v4-kk", "tests.yaml"): {
		"2. price question, Kazakh",
		"3. delivery cost + time, Kazakh",
		"20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh)",
		"21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim",
	},
	filepath.Join("..", "scenarios", "lang-canary-v4-ru", "tests.yaml"): {
		"1. price question, Russian",
		"9. off-KB city coverage",
		"16. follow-up with history, needs delivery cost",
	},
	// Phase 2.3 combo variant: the union of the lang + escalation canary sets, split by
	// detectLang routing exactly like the v4 siblings (TestDetectLang_AgreesWithRoutedCanarySplits
	// proves the split; this pins the membership).
	filepath.Join("..", "scenarios", "combo-canary-v1-kk", "tests.yaml"): {
		"2. price question, Kazakh",
		"3. delivery cost + time, Kazakh",
		"20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh)",
		"21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim",
	},
	filepath.Join("..", "scenarios", "combo-canary-v1-ru", "tests.yaml"): {
		"1. price question, Russian",
		"9. off-KB city coverage",
		"10. refund request",
		"16. follow-up with history, needs delivery cost",
		"18. wrong understanding, asks about a product we don't sell",
		"19. wrong understanding, asks about a service we don't offer",
	},
}

// loadTestCases reads a tests.yaml-shaped file's own `tests:` list (never following
// `include:` — every canarySyncFiles entry declares its tests directly, same as
// canary_holdout_test.go's loadRawTestMessages does for messages only).
func loadTestCases(t *testing.T, path string) []TestCase {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var tf TestsFile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return tf.Tests
}

// TestCanarySync_PinnedIDListMatchesFileExactly is the structural half of the sync
// guarantee: each canary file must contain EXACTLY the pinned ID list, in order — not a
// subset, not a superset, not merely "every pinned ID is present somewhere." A canary
// file that lost a test (e.g. an accidental deletion during a future edit) would still
// pass a mere subset check; comparing the full ordered list catches that instead.
func TestCanarySync_PinnedIDListMatchesFileExactly(t *testing.T) {
	for path, wantIDs := range canarySyncFiles {
		t.Run(path, func(t *testing.T) {
			tests := loadTestCases(t, path)
			gotIDs := make([]string, len(tests))
			for i, tc := range tests {
				gotIDs[i] = tc.ID
			}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("%s: want exactly these IDs in order:\n%v\ngot:\n%v", path, wantIDs, gotIDs)
			}
		})
	}
}

// TestCanarySync_FieldIdenticalWithShopQuestionsBank is the content half of the sync
// guarantee: every hand-copied canary test must be a byte-for-byte struct match
// (message, requires, media, escalate, language, must_not_contain, history — and any
// future TestCase field, since reflect.DeepEqual compares the whole struct) of the test
// carrying the same ID in common/shop-questions.yaml. These files are hand-maintained
// copies (their own header comments say "re-sync by hand") specifically so the
// escalation-wording and language bake-offs stay comparable against the SAME canary
// content — drift here would silently make the two bake-offs non-comparable, exactly
// the failure mode the scenario docs warn is the whole point of keeping them in sync.
func TestCanarySync_FieldIdenticalWithShopQuestionsBank(t *testing.T) {
	bankTests := loadTestCases(t, filepath.Join("..", "common", "shop-questions.yaml"))
	bankByID := map[string]TestCase{}
	for _, tc := range bankTests {
		bankByID[tc.ID] = tc
	}

	for path := range canarySyncFiles {
		t.Run(path, func(t *testing.T) {
			for _, tc := range loadTestCases(t, path) {
				bankTC, ok := bankByID[tc.ID]
				if !ok {
					t.Errorf("%s: test %q has no matching ID in common/shop-questions.yaml — it may have been renamed or removed from the bank without updating this canary copy", path, tc.ID)
					continue
				}
				if !reflect.DeepEqual(tc, bankTC) {
					t.Errorf("%s: test %q has drifted from common/shop-questions.yaml's version of the same ID (both files' header comments say \"re-sync by hand\" — this copy is now stale):\ncanary: %+v\nbank:   %+v", path, tc.ID, tc, bankTC)
				}
			}
		})
	}
}
