package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These are the core, one-shot-history, and delivery-zones banks for pipeline:schema_kb_v1.
var kbQuestionsRUBankPath = filepath.Join("..", "common", "kb-questions-ru.yaml")
var kbHistoryRUBankPath = filepath.Join("..", "common", "kb-history-ru.yaml")
var kbDeliveryRUBankPath = filepath.Join("..", "common", "kb-delivery-ru.yaml")

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
	"9. delivery cost (general, zones present)",
	"10. delivery days (general, zones present)",
	"11. minimum order amount",
	"12. working hours and phone",
	"13. missing exact value — return period escalates",
	"14. warranty duration escalates (prose-only, no fact column)",
	"15. off-KB city resolves via country zone fallback",
}

var kbHistoryRUPinnedIDs = []string{
	"h1. pronoun price follow-up",
	"h2. purchase intent resolves discussed product",
	"h3. repeat prior literal through placeholder",
	"h4. pronoun photo request",
	"h5. out-of-stock purchase intent stays honest",
}

var kbDeliveryRUPinnedIDs = []string{
	"d1. listed city — Astana",
	"d2. listed city, different declension — Astane (dative)",
	"d3. explicit deny zone — Baikonur beats its parent's allow",
	"d4. unlisted country — China refuses via outside_zones_note",
	"d5. unplaceable location escalates",
	"d6. negotiation for an excluded direction escalates",
	"d7. refund demand escalates",
	"d8. repair service question escalates",
	"d9. off-catalog product escalates",
	"d10. history follow-up resolves the zone already discussed",
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

func TestKBHistoryRUBank_PinnedIDList(t *testing.T) {
	tests := loadTestCases(t, kbHistoryRUBankPath)
	gotIDs := make([]string, len(tests))
	for i, tc := range tests {
		gotIDs[i] = tc.ID
	}
	if !reflect.DeepEqual(gotIDs, kbHistoryRUPinnedIDs) {
		t.Fatalf("%s: want exactly these IDs in order:\n%v\ngot:\n%v", kbHistoryRUBankPath, kbHistoryRUPinnedIDs, gotIDs)
	}
}

func TestKBDeliveryRUBank_PinnedIDList(t *testing.T) {
	tests := loadTestCases(t, kbDeliveryRUBankPath)
	gotIDs := make([]string, len(tests))
	for i, tc := range tests {
		gotIDs[i] = tc.ID
	}
	if !reflect.DeepEqual(gotIDs, kbDeliveryRUPinnedIDs) {
		t.Fatalf("%s: want exactly these IDs in order:\n%v\ngot:\n%v", kbDeliveryRUBankPath, kbDeliveryRUPinnedIDs, gotIDs)
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
	for _, bankPath := range []string{kbQuestionsRUBankPath, kbHistoryRUBankPath, kbDeliveryRUBankPath} {
		for _, tc := range loadTestCases(t, bankPath) {
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
}

func TestKBHistoryRUBank_RendersAsOneShotPromptfooVariables(t *testing.T) {
	tests := loadTestCases(t, kbHistoryRUBankPath)
	dir := t.TempDir()
	scenario := &ScenarioConfig{Name: "history-one-shot", Description: "test"}
	models := &ModelsFile{Providers: []ModelProvider{{ID: "openrouter:test/model"}}}
	if err := writePromptfooConfig(dir, scenario, tests, models); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "promptfooconfig.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Tests []struct {
			Description string            `yaml:"description"`
			Vars        map[string]string `yaml:"vars"`
		} `yaml:"tests"`
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tests) != len(tests) {
		t.Fatalf("rendered tests = %d, want %d", len(cfg.Tests), len(tests))
	}
	for i, tc := range tests {
		if cfg.Tests[i].Description != tc.ID || cfg.Tests[i].Vars["message"] != tc.Message {
			t.Fatalf("rendered test %d does not carry its final message: %+v", i, cfg.Tests[i])
		}
		if cfg.Tests[i].Vars["history"] != renderHistory(tc.History) {
			t.Fatalf("rendered test %q does not carry its complete history", tc.ID)
		}
	}
	prompt := wrapPromptfooTemplate("stable prefix")
	historyAt := strings.Index(prompt, "{{history}}")
	messageAt := strings.Index(prompt, "{{message}}")
	if historyAt < 0 || messageAt <= historyAt {
		t.Fatalf("one-shot template must contain history before final message:\n%s", prompt)
	}
}
