package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// This file guards the two prompt copies the multichannel response-service
// milestone depends on: the evaluated frame/tail/retry-feedback text lives both
// here (evals/) and embedded in backend/aiprompt (production). A previous
// divergence between the two copies was production blocker #1 for that
// milestone — these tests fail loudly the moment either copy drifts from the
// other, instead of silently shipping an unevaluated prompt.

// TestAipromptSync_FrameByteIdentical pins evals/scenarios/shop-kb-v1/frame-ru.txt
// to backend/aiprompt's embedded copy (aiprompt/frames/shop-kb-v5-ru.txt, exposed
// via aiprompt.FrameShopKBV5RU) — the exact frame production renders.
//
// Tracks v5 since 2026-08: v4 had no way to render tariffs, so production moved
// to v5 (v4 plus the ТАРИФЫ block) and this copy had to move with it. What this
// test guarantees is unchanged — the graded prompt and the shipped prompt are
// the same bytes — and it is exactly what would have caught the move silently
// leaving the eval corpus behind.
func TestAipromptSync_FrameByteIdentical(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "scenarios", "shop-kb-v1", "frame-ru.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != aiprompt.FrameShopKBV5RU() {
		t.Fatal("evals/scenarios/shop-kb-v1/frame-ru.txt has drifted from backend/aiprompt's embedded copy " +
			"(aiprompt/frames/shop-kb-v5-ru.txt) — production and the evaluated prompt must stay byte-identical")
	}
}

// TestAipromptSync_ConversationTailMatchesHarnessTemplate pins
// aiprompt.ConversationTail's history/message tail to the literal text
// wrapPromptfooTemplate appends (render.go) — the same history/message framing
// promptfoo substitutes {{history}}/{{message}} into.
func TestAipromptSync_ConversationTailMatchesHarnessTemplate(t *testing.T) {
	wrapped := wrapPromptfooTemplate("")
	wantTail := strings.TrimPrefix(wrapped, "{% raw %}\n\n{% endraw %}\n")
	if wantTail == wrapped {
		t.Fatal("test setup: wrapPromptfooTemplate's {% raw %} wrapper prefix did not match — harness template changed shape")
	}
	got := aiprompt.ConversationTail("{{history}}", "{{message}}")
	if got != wantTail {
		t.Fatalf("aiprompt.ConversationTail(...) diverged from the harness tail:\ngot:  %q\nwant: %q", got, wantTail)
	}
}

// TestAipromptSync_RenderHistoryMatchesHarnessRenderHistory pins
// aiprompt.RenderHistory to the harness's own renderHistory (render.go:527).
func TestAipromptSync_RenderHistoryMatchesHarnessRenderHistory(t *testing.T) {
	cases := [][]HistoryTurn{
		nil,
		{{Role: "client", Text: "Сколько стоит?"}},
		{{Role: "client", Text: "Привет"}, {Role: "assistant", Text: "Здравствуйте!"}},
	}
	for _, turns := range cases {
		want := renderHistory(turns)
		converted := make([]aiprompt.HistoryTurn, len(turns))
		for i, turn := range turns {
			converted[i] = aiprompt.HistoryTurn{Role: turn.Role, Text: turn.Text}
		}
		got := aiprompt.RenderHistory(converted)
		if got != want {
			t.Fatalf("aiprompt.RenderHistory(%+v) = %q, want %q (harness renderHistory)", turns, got, want)
		}
	}
}

// TestAipromptSync_RetryFeedbackMatchesHarnessCorrectiveFeedback pins
// aiprompt.RetryFeedback to the harness's correctiveFeedbackTextSchemaKB
// (retry.go:148) on an identical media_not_found response.
func TestAipromptSync_RetryFeedbackMatchesHarnessCorrectiveFeedback(t *testing.T) {
	kb := &aiprompt.KB{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		Products: []aiprompt.Product{
			{Ref: "widget", Name: "Виджет", Price: "1 000 ₸", AvailabilityStatus: "in_stock", SalesStatus: "active"},
		},
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"reply_text":"Хорошо.","reply_language":"ru","media_files_to_send":["products.widget.nonexistent"],"escalate":false}`

	want := correctiveFeedbackTextSchemaKB(raw, kb, cat)
	if want == "" {
		t.Fatal("test setup produced no corrective feedback from the harness function — fixture is wrong")
	}
	got := aiprompt.RetryFeedback(raw, kb, cat)
	if got != want {
		t.Fatalf("aiprompt.RetryFeedback diverged from the harness's correctiveFeedbackTextSchemaKB:\ngot:  %q\nwant: %q", got, want)
	}
}
