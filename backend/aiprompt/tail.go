package aiprompt

import "strings"

// HistoryTurn is one prior message in a conversation, in send order. Text is
// authored prose (never a {{token}} — history is what was ALREADY sent to the
// customer), rendered verbatim into the conversation tail.
type HistoryTurn struct {
	Role string // "client" | "assistant"
	Text string
}

// RenderHistory renders turns into the plain-text block the conversation tail's
// history line fills — an exact port of the eval harness's renderHistory
// (evals/harness/render.go:527-540), so production and the eval pipeline format
// prior turns identically.
func RenderHistory(turns []HistoryTurn) string {
	if len(turns) == 0 {
		return "(пусто — начало разговора)"
	}
	var lines []string
	for _, t := range turns {
		label := "Клиент"
		if t.Role == "assistant" {
			label = "Ассистент"
		}
		lines = append(lines, label+": "+t.Text)
	}
	return strings.Join(lines, "\n")
}

// ConversationTail appends the history block and the customer's current message
// to the rendered prompt prefix, in the exact shape the eval harness's
// wrapPromptfooTemplate produces (evals/harness/render.go:514-522) once its
// promptfoo {{history}}/{{message}} placeholders are substituted — so production
// and the eval pipeline send the model byte-identical tails.
func ConversationTail(historyBlock, message string) string {
	return "История переписки (может быть пустой — тогда это начало разговора):\n" +
		historyBlock + "\nКлиент пишет: " + message + "\n"
}
