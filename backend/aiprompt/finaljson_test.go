package aiprompt

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustResponseJSON(t *testing.T, text string) string {
	t.Helper()
	b, err := json.Marshal(Response{
		ReplyText:        text,
		ReplyLanguage:    "ru",
		MediaFilesToSend: []string{},
		Escalate:         false,
		EscalationReason: "",
		Confidence:       0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestExtractFinalOutput_PureJSON: the whole output is one JSON object -> direct.
func TestExtractFinalOutput_PureJSON(t *testing.T) {
	final := mustResponseJSON(t, "Готово.")
	got, ok := ExtractFinalOutput(final)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Method != ExtractDirect {
		t.Errorf("Method = %q, want %q", got.Method, ExtractDirect)
	}
	if got.Final != final {
		t.Errorf("Final = %q, want %q", got.Final, final)
	}
	if got.NonFinal != "" {
		t.Errorf("NonFinal = %q, want empty", got.NonFinal)
	}
}

// TestExtractFinalOutput_OneOuterMarkdownFence: final JSON wrapped in exactly one
// outer fence is accepted directly (transport noise, not combined reasoning).
func TestExtractFinalOutput_OneOuterMarkdownFence(t *testing.T) {
	final := mustResponseJSON(t, "Готово.")
	raw := "```json\n" + final + "\n```"
	got, ok := ExtractFinalOutput(raw)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Method != ExtractOuterFence {
		t.Errorf("Method = %q, want %q", got.Method, ExtractOuterFence)
	}
	if got.Final != final {
		t.Errorf("Final = %q, want %q", got.Final, final)
	}
}

// TestExtractFinalOutput_ReasoningThenOneCompleteFinalBlock: in-band reasoning prose
// followed by exactly one complete final JSON object at the end of the output.
func TestExtractFinalOutput_ReasoningThenOneCompleteFinalBlock(t *testing.T) {
	final := mustResponseJSON(t, "Кофемашина в наличии.")
	raw := "Thinking: let me check the knowledge base for stock status of the coffee machine before answering the customer.\n\n" + final
	got, ok := ExtractFinalOutput(raw)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Final != final {
		t.Errorf("Final = %q, want %q", got.Final, final)
	}
	if got.Method != ExtractBalancedObject {
		t.Errorf("Method = %q, want %q", got.Method, ExtractBalancedObject)
	}
	if !strings.Contains(got.NonFinal, "Thinking:") {
		t.Errorf("NonFinal = %q, want it to retain the reasoning prose", got.NonFinal)
	}
}

// TestExtractFinalOutput_PartialDiagnosticThenCompleteFinal: a partial
// diagnostic-only object like {"confidence": 1} must be ignored in favor of the
// later complete operational object.
func TestExtractFinalOutput_PartialDiagnosticThenCompleteFinal(t *testing.T) {
	final := mustResponseJSON(t, "Ответ готов.")
	raw := `Thinking about confidence: {"confidence": 1}` + "\n\nFinal answer: " + final
	got, ok := ExtractFinalOutput(raw)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Final != final {
		t.Errorf("Final = %q, want %q", got.Final, final)
	}
	if strings.Contains(got.Final, "confidence\": 1") {
		t.Errorf("Final must not be the partial diagnostic-only object, got %q", got.Final)
	}
}

// TestExtractFinalOutput_MultipleCandidatesSelectsLast: when several complete
// operational JSON objects appear, the LAST one is the final answer.
func TestExtractFinalOutput_MultipleCandidatesSelectsLast(t *testing.T) {
	first := mustResponseJSON(t, "Черновик один.")
	second := mustResponseJSON(t, "Черновик два, финальный.")
	raw := "Draft 1: " + first + "\n\nActually, let me revise. Draft 2: " + second
	got, ok := ExtractFinalOutput(raw)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Final != second {
		t.Errorf("Final = %q, want the LAST candidate %q", got.Final, second)
	}
}

// TestExtractFinalOutput_ReasoningWithNoFinalJSON: pure reasoning prose, no JSON
// object anywhere -> extraction fails.
func TestExtractFinalOutput_ReasoningWithNoFinalJSON(t *testing.T) {
	raw := "Thinking: I am considering the price of the coffee machine and need to look it up in the knowledge base before I can answer the customer's question about availability."
	_, ok := ExtractFinalOutput(raw)
	if ok {
		t.Fatal("expected extraction to fail: no JSON object present at all")
	}
}

// TestExtractFinalOutput_TruncatedFinalJSON: reasoning consumed the completion
// budget and the final JSON never closes -> extraction fails. Never repaired or
// reconstructed.
func TestExtractFinalOutput_TruncatedFinalJSON(t *testing.T) {
	final := mustResponseJSON(t, "Кофемашина стоит 129900 тенге и доставка")
	truncated := final[:len(final)-15] // cut off mid-object, never closes
	raw := "Thinking: let me check the price.\n\n" + truncated
	_, ok := ExtractFinalOutput(raw)
	if ok {
		t.Fatal("expected extraction to fail on a truncated/incomplete final object")
	}
}

// TestExtractFinalOutput_MissingOrWrongTypedOperationalFieldsStillFail: an embedded
// candidate missing an operational field, or with a wrong-typed one, must not be
// accepted as the final answer even when it's the only/last candidate.
func TestExtractFinalOutput_MissingOrWrongTypedOperationalFieldsStillFail(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing escalate", body: `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[]}`},
		{name: "wrong-typed escalate", body: `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":"false"}`},
		{name: "wrong-typed media_files_to_send", body: `{"reply_text":"ok","reply_language":"ru","media_files_to_send":"none","escalate":false}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := "Thinking about it.\n\n" + tt.body
			_, ok := ExtractFinalOutput(raw)
			if ok {
				t.Fatalf("expected extraction to fail for a candidate with missing/wrong-typed operational fields: %s", tt.body)
			}
		})
	}
}

// TestExtractFinalOutput_RawNeverMutated: extraction only SELECTS a span already
// present verbatim in the input — it never rewrites or repairs the caller's raw
// string. This is the byte-for-byte immutability guarantee the caller relies on to
// keep RawOutput untouched.
func TestExtractFinalOutput_RawNeverMutated(t *testing.T) {
	final := mustResponseJSON(t, "Ответ.")
	raw := "Thinking: something.\n\n" + final
	rawCopy := raw

	got, ok := ExtractFinalOutput(raw)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if raw != rawCopy {
		t.Fatalf("ExtractFinalOutput must never mutate its input; raw changed from %q to %q", rawCopy, raw)
	}
	if !strings.Contains(raw, got.Final) {
		t.Fatalf("Final %q must be an exact substring of the original raw input", got.Final)
	}
}

// TestExtractFinalOutput_ValidatesThroughContractAfterExtraction proves the
// extracted final JSON is exactly what ValidateResponse accepts — extraction and
// contract validation must never disagree about which object is "the answer".
func TestExtractFinalOutput_ValidatesThroughContractAfterExtraction(t *testing.T) {
	cat := validCatalog(t)
	final := mustResponseJSON(t, "Готово.")
	raw := "Thinking: reviewing the rules before answering.\n\n" + final
	got, ok := ExtractFinalOutput(raw)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	resp, issues := ValidateResponse(got.Final, baseKB(), cat)
	if resp == nil || len(issues) != 0 {
		t.Fatalf("ValidateResponse(extracted final) = (%+v, %+v), want valid with no issues", resp, issues)
	}
}

// --- ExtractJSONObject: the contract-agnostic core other callers (e.g.
// internal/kbimport's synthesis parser) use directly, with their own single
// acceptance predicate instead of the customer-response contract's fields.

func hasCallsField(m map[string]json.RawMessage) bool {
	_, ok := m["calls"]
	return ok
}

func TestExtractJSONObject_PureJSON(t *testing.T) {
	final := `{"calls":[{"tool":"kb_product_upsert"}],"notes":"","unmapped":[]}`
	got, ok := ExtractJSONObject(final, hasCallsField)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Method != ExtractDirect {
		t.Errorf("Method = %q, want %q", got.Method, ExtractDirect)
	}
	if got.Final != final {
		t.Errorf("Final = %q, want %q", got.Final, final)
	}
}

func TestExtractJSONObject_OuterFence(t *testing.T) {
	final := `{"calls":[]}`
	raw := "```json\n" + final + "\n```"
	got, ok := ExtractJSONObject(raw, hasCallsField)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Method != ExtractOuterFence {
		t.Errorf("Method = %q, want %q", got.Method, ExtractOuterFence)
	}
	if got.Final != final {
		t.Errorf("Final = %q, want %q", got.Final, final)
	}
}

func TestExtractJSONObject_EmbeddedInReasoning(t *testing.T) {
	final := `{"calls":[{"tool":"kb_topic_upsert"}]}`
	raw := "Let me think about this.\n\n" + final + "\n\nThat's my answer."
	got, ok := ExtractJSONObject(raw, hasCallsField)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Final != final {
		t.Errorf("Final = %q, want %q", got.Final, final)
	}
}

func TestExtractJSONObject_RejectsNonMatchingShape(t *testing.T) {
	raw := `{"reply_text":"hi","escalate":false}`
	if _, ok := ExtractJSONObject(raw, hasCallsField); ok {
		t.Fatal("expected extraction to fail: no top-level calls field")
	}
}

func TestExtractJSONObject_LastCompleteCandidateWins(t *testing.T) {
	partial := `{"calls":"still thinking"}` // wrong shape for calls, but the predicate here only checks presence
	final := `{"calls":[{"tool":"kb_contacts_upsert"}]}`
	raw := "Draft: " + partial + "\n\nFinal: " + final
	got, ok := ExtractJSONObject(raw, hasCallsField)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if got.Final != final {
		t.Errorf("Final = %q, want the LAST candidate %q", got.Final, final)
	}
}
