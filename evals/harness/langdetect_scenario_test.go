package main

import (
	"path/filepath"
	"testing"
)

// TestDetectLang_AgreesWithRoutedCanarySplits is the real consistency guard between
// every hand-authored routed split (the V4 siblings, Phase 2.1, and the combo siblings,
// Phase 2.3) and detectLang's actual behavior: every test placed in a kk scenario must
// be a message detectLang would route to "kk", and every test in a ru scenario must
// route to "ru". If detectLang is ever changed and stops agreeing with a hand-picked
// split, this test catches it — the canary scenarios would otherwise silently test the
// WRONG frame for a message a real production router would have sent elsewhere.
func TestDetectLang_AgreesWithRoutedCanarySplits(t *testing.T) {
	routedScenarios := map[string]string{
		"lang-canary-v4-kk":  "kk",
		"lang-canary-v4-ru":  "ru",
		"combo-canary-v1-kk": "kk",
		"combo-canary-v1-ru": "ru",
	}
	for scenario, wantLang := range routedScenarios {
		t.Run(scenario, func(t *testing.T) {
			tests, err := resolveTests(filepath.Join("..", "scenarios", scenario), "tests.yaml")
			if err != nil {
				t.Fatalf("resolve %s canary tests: %v", scenario, err)
			}
			if len(tests) == 0 {
				t.Fatalf("expected at least one canary test in %s", scenario)
			}
			for _, tc := range tests {
				if got := detectLang(tc.Message, tc.History); got != wantLang {
					t.Errorf("test %q (message %q): detectLang = %q, but it's placed in the %s canary scenario", tc.ID, tc.Message, got, wantLang)
				}
			}
		})
	}
}
