package aiprompt

import (
	"strings"
)

// CustomerContext is what the assistant is allowed to know about the person it
// is answering: the CRM profile a manager curated, and nothing else.
//
// It is a deliberately narrow projection, not a customer row. No ids, no
// storage locators, no custom fields, no raw profile columns — the same
// "internal ids never enter a prompt" rule the knowledge base is held to (see
// plan/overview.md's invariants). store.CustomerSummary is its only producer.
//
// This block is READ-ONLY context. Nothing the model emits can change a
// status, a tag, an assignee or a follow-up: the response contract
// (aiprompt.Contract) has no CRM fields at all, so a generated reply has no
// vocabulary in which to ask for a CRM write. Changing CRM data stays an
// explicit operator action in the UI.
type CustomerContext struct {
	Name       string
	Status     string
	Tags       []string
	LatestNote string
	// NextAction/NextDueOn describe the customer's next open follow-up:
	// the action type ("call") and its date ("2026-08-20").
	NextAction string
	NextDueOn  string
}

// RenderCustomerContext renders the block that goes between the knowledge-base
// prefix and the conversation tail.
//
// It returns "" for a nil or empty context, and that is load-bearing, not a
// convenience: every eval fixture runs without a CRM customer, so an empty
// context must leave the assembled prompt byte-identical to what
// evals/harness produces (TestAipromptSync_ConversationTailMatchesHarnessTemplate
// pins the tail's own shape). A conversation with no customer therefore sends
// exactly the prompt it sent before this feature existed.
func RenderCustomerContext(c *CustomerContext) string {
	if c == nil {
		return ""
	}
	var lines []string
	if n := strings.TrimSpace(c.Name); n != "" {
		lines = append(lines, "Имя: "+n)
	}
	if st := strings.TrimSpace(c.Status); st != "" {
		lines = append(lines, "Статус: "+st)
	}
	if tags := joinNonEmpty(c.Tags, ", "); tags != "" {
		lines = append(lines, "Теги: "+tags)
	}
	if note := collapseWhitespace(c.LatestNote); note != "" {
		lines = append(lines, "Последняя заметка менеджера: "+note)
	}
	if due := strings.TrimSpace(c.NextDueOn); due != "" {
		next := due
		if act := strings.TrimSpace(c.NextAction); act != "" {
			next = due + " (" + act + ")"
		}
		lines = append(lines, "Следующий шаг: "+next)
	}
	if len(lines) == 0 {
		return ""
	}
	// The trailing framing sentence is a guardrail, not decoration: without it
	// a model that sees an internal note tends to quote it back to the
	// customer. Notes are internal by definition (see crm_customer_notes).
	return "Что мы знаем об этом клиенте (внутренние данные — не пересказывай их клиенту, " +
		"используй только чтобы точнее ответить):\n" + strings.Join(lines, "\n") + "\n\n"
}

func joinNonEmpty(items []string, sep string) string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, sep)
}

// collapseWhitespace flattens a multi-line note onto one line so it cannot
// break the block's "one fact per line" shape.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
