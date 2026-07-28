package aiprompt

import "testing"

func TestRenderHistory_Empty(t *testing.T) {
	got := RenderHistory(nil)
	want := "(пусто — начало разговора)"
	if got != want {
		t.Fatalf("RenderHistory(nil) = %q, want %q", got, want)
	}
	if got := RenderHistory([]HistoryTurn{}); got != want {
		t.Fatalf("RenderHistory([]) = %q, want %q", got, want)
	}
}

func TestRenderHistory_ClientAndAssistantLabels(t *testing.T) {
	got := RenderHistory([]HistoryTurn{
		{Role: "client", Text: "Сколько стоит кофемашина?"},
		{Role: "assistant", Text: "129 900 ₸."},
		{Role: "client", Text: "Спасибо!"},
	})
	want := "Клиент: Сколько стоит кофемашина?\nАссистент: 129 900 ₸.\nКлиент: Спасибо!"
	if got != want {
		t.Fatalf("RenderHistory() = %q, want %q", got, want)
	}
}

func TestRenderHistory_UnknownRoleTreatedAsClient(t *testing.T) {
	got := RenderHistory([]HistoryTurn{{Role: "system", Text: "x"}})
	want := "Клиент: x"
	if got != want {
		t.Fatalf("RenderHistory() = %q, want %q (only \"assistant\" gets the Ассистент label)", got, want)
	}
}

func TestConversationTail_Format(t *testing.T) {
	got := ConversationTail("(пусто — начало разговора)", "Есть кофемашина?")
	want := "История переписки (может быть пустой — тогда это начало разговора):\n" +
		"(пусто — начало разговора)\n" +
		"Клиент пишет: Есть кофемашина?\n"
	if got != want {
		t.Fatalf("ConversationTail() =\n%q\nwant\n%q", got, want)
	}
}

func TestConversationTail_WithRenderedHistory(t *testing.T) {
	history := RenderHistory([]HistoryTurn{{Role: "client", Text: "Привет"}})
	got := ConversationTail(history, "Сколько стоит?")
	want := "История переписки (может быть пустой — тогда это начало разговора):\n" +
		"Клиент: Привет\n" +
		"Клиент пишет: Сколько стоит?\n"
	if got != want {
		t.Fatalf("ConversationTail() =\n%q\nwant\n%q", got, want)
	}
}
