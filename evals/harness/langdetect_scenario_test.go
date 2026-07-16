package main

import (
	"path/filepath"
	"testing"
)

// TestDetectLang_AgreesWithV4CanarySplit is the real consistency guard between the
// hand-authored V4 routing split (scenarios/lang-canary-v4-kk and -v4-ru, Phase 2.1) and
// detectLang's actual behavior: every test placed in the kk scenario must be a message
// detectLang would route to "kk", and every test in the ru scenario must route to "ru".
// If detectLang is ever changed and stops agreeing with this hand-picked split, this
// test catches it — the canary scenarios would otherwise silently test the WRONG frame
// for a message a real production router would have sent elsewhere.
func TestDetectLang_AgreesWithV4CanarySplit(t *testing.T) {
	kkTests, err := resolveTests(filepath.Join("..", "scenarios", "lang-canary-v4-kk"), "tests.yaml")
	if err != nil {
		t.Fatalf("resolve kk canary tests: %v", err)
	}
	if len(kkTests) == 0 {
		t.Fatal("expected at least one kk canary test")
	}
	for _, tc := range kkTests {
		if got := detectLang(tc.Message, tc.History); got != "kk" {
			t.Errorf("test %q (message %q): detectLang = %q, but it's placed in the KK canary scenario", tc.ID, tc.Message, got)
		}
	}

	ruTests, err := resolveTests(filepath.Join("..", "scenarios", "lang-canary-v4-ru"), "tests.yaml")
	if err != nil {
		t.Fatalf("resolve ru canary tests: %v", err)
	}
	if len(ruTests) == 0 {
		t.Fatal("expected at least one ru canary test")
	}
	for _, tc := range ruTests {
		if got := detectLang(tc.Message, tc.History); got != "ru" {
			t.Errorf("test %q (message %q): detectLang = %q, but it's placed in the RU canary scenario", tc.ID, tc.Message, got)
		}
	}
}
