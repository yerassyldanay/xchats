package aiprompt

import (
	"strings"
	"testing"
)

// TestRenderCustomerContext_EmptyIsByteIdentical is the eval-parity guarantee.
// Every eval fixture runs without a CRM customer, and the harness's own
// template has no customer block at all (see
// evals/harness/aiprompt_sync_test.go). If an absent or empty customer
// rendered anything — even a heading, even a newline — production would send a
// prompt the eval suite never scores, and every recorded result would describe
// a prompt that no longer exists.
func TestRenderCustomerContext_EmptyIsByteIdentical(t *testing.T) {
	cases := map[string]*CustomerContext{
		"nil":                   nil,
		"zero value":            {},
		"whitespace only":       {Name: "   ", Status: "\t", LatestNote: "\n "},
		"empty tag entries":     {Tags: []string{"", "  "}},
		"followup with no date": {NextAction: "call"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := RenderCustomerContext(c); got != "" {
				t.Fatalf("RenderCustomerContext(%s) = %q, want \"\" — an empty customer must not change the prompt at all", name, got)
			}
		})
	}

	// And the assembled prompt is unchanged end to end, not just this block.
	const prefix = "KNOWLEDGE BASE\n"
	tail := ConversationTail(RenderHistory(nil), "Сколько стоит?")
	if got := prefix + RenderCustomerContext(nil) + tail; got != prefix+tail {
		t.Fatalf("a nil customer changed the assembled prompt:\n%q", got)
	}
}

func TestRenderCustomerContext_RendersCuratedFieldsOnly(t *testing.T) {
	got := RenderCustomerContext(&CustomerContext{
		Name:       "Иван Смирнов",
		Status:     "Интересуется",
		Tags:       []string{"VIP", "xpayment", ""},
		LatestNote: "Просил перезвонить\nпозже.",
		NextAction: "call",
		NextDueOn:  "2026-08-20",
	})

	for _, want := range []string{
		"Имя: Иван Смирнов",
		"Статус: Интересуется",
		"Теги: VIP, xpayment",
		"Последняя заметка менеджера: Просил перезвонить позже.",
		"Следующий шаг: 2026-08-20 (call)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("block is missing %q:\n%s", want, got)
		}
	}
	// The note is flattened onto one line so it cannot break the block's shape.
	if strings.Contains(got, "перезвонить\nпозже") {
		t.Errorf("a multi-line note was not flattened:\n%s", got)
	}
	// An internal note reaching a customer would be a product failure, so the
	// block always carries the do-not-repeat framing.
	if !strings.Contains(got, "не пересказывай их клиенту") {
		t.Errorf("block is missing the internal-only framing:\n%s", got)
	}
	// The block must end with a blank line so the conversation tail that
	// follows it starts cleanly.
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("block does not end with a blank separator:\n%q", got)
	}
}

// TestRenderCustomerContext_PartialContext: a customer with only some fields
// filled in renders only those, with no empty labels.
func TestRenderCustomerContext_PartialContext(t *testing.T) {
	got := RenderCustomerContext(&CustomerContext{Name: "Иван", Status: "Новый"})
	if strings.Contains(got, "Теги:") || strings.Contains(got, "Следующий шаг:") ||
		strings.Contains(got, "Последняя заметка") {
		t.Fatalf("empty fields produced labels:\n%s", got)
	}
	if !strings.Contains(got, "Имя: Иван") || !strings.Contains(got, "Статус: Новый") {
		t.Fatalf("set fields are missing:\n%s", got)
	}
}
