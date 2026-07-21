package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

// kbQuestionsRUBankPath is common/kb-questions-ru.yaml, relative to this package dir —
// the shop-kb-v1 family's sole question bank (pipeline: schema_kb_v1).
var kbQuestionsRUBankPath = filepath.Join("..", "common", "kb-questions-ru.yaml")

// kbQuestionsRUPinnedIDs is the bank's exact, ordered ID list. Pinning the full list (not
// just checking membership) means an accidental deletion or truncation during a future
// edit fails loudly here instead of silently shrinking shop-kb-v1's coverage — same
// doctrine as canary_sync_test.go's TestCanarySync_PinnedIDListMatchesFileExactly.
var kbQuestionsRUPinnedIDs = []string{
	"1. exact price — coffee machine",
	"2. in-stock availability — coffee machine",
	"3. out-of-stock honesty with in-stock alternative",
	"4. photo request — media-ful product",
	"5. photo and certificate combined (all_of)",
	"6. photo request — media-less product",
	"7. video request — product with no video",
	"8. partial-media probe — asks for the one missing kind",
	"9. delivery cost",
	"10. delivery days",
	"11. minimum order amount",
	"12. working hours and phone",
	"13. missing exact value — return period escalates",
	"14. warranty duration escalates (prose-only, no fact column)",
	"15. off-KB city escalates",
	"16. history follow-up — coffee machine price",
}

func TestKBQuestionsRUBank_PinnedIDList(t *testing.T) {
	tests := loadTestCases(t, kbQuestionsRUBankPath)
	gotIDs := make([]string, len(tests))
	for i, tc := range tests {
		gotIDs[i] = tc.ID
	}
	if !reflect.DeepEqual(gotIDs, kbQuestionsRUPinnedIDs) {
		t.Fatalf("%s: want exactly these IDs in order:\n%v\ngot:\n%v", kbQuestionsRUBankPath, kbQuestionsRUPinnedIDs, gotIDs)
	}
}

// TestDetectLang_KBQuestionsRUBankIsAllRussian confirms the shop-kb-v1 family's own bank
// never accidentally drifts into Kazakh-looking text. The schema_kb_v1 render path never
// calls detectLang itself (frame-ru.txt is the only frame this family ever renders — there
// is no language routing to do), but other harness tooling (e.g. blind-export's routing-
// accuracy report) runs it generically over every scenario's messages, and this family is
// Russian-only by contract (aiprompt.ValidateResponse rejects any reply_language other
// than "ru") — a message that reads as Kazakh here would be a silent contradiction.
func TestDetectLang_KBQuestionsRUBankIsAllRussian(t *testing.T) {
	for _, tc := range loadTestCases(t, kbQuestionsRUBankPath) {
		if got := detectLang(tc.Message, tc.History); got != "ru" {
			t.Errorf("test %q: detectLang(message)=%q, want ru — message: %q", tc.ID, got, tc.Message)
		}
		for _, turn := range tc.History {
			if got := detectLang(turn.Text, nil); got != "ru" {
				t.Errorf("test %q: detectLang(history turn)=%q, want ru — text: %q", tc.ID, got, turn.Text)
			}
		}
	}
}
