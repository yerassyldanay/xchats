package aiprompt

import (
	"strings"
	"testing"
)

func retryTestCatalog(t *testing.T) (*KB, *Catalog) {
	t.Helper()
	kb := baseKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	return kb, cat
}

func TestClassifyRetry_ValidResponseNeedsNoRetry(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Стоимость: {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":[],"escalate":false}`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonNone {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonNone)
	}
	if fb := RetryFeedback(raw, kb, cat); fb != "" {
		t.Fatalf("RetryFeedback() = %q, want empty", fb)
	}
}

func TestClassifyRetry_UnparseableJSONIsContractShape(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `not json at all`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonContractShape)
	}
	if fb := RetryFeedback(raw, kb, cat); fb != "" {
		t.Fatalf("RetryFeedback() = %q, want empty for an unparseable response (identical re-roll)", fb)
	}
}

func TestClassifyRetry_MissingPropertyIsContractShape(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Хорошо.","reply_language":"ru","media_files_to_send":[]}` // no "escalate"
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonContractShape)
	}
	if fb := RetryFeedback(raw, kb, cat); fb != "" {
		t.Fatalf("RetryFeedback() = %q, want empty for contract_shape", fb)
	}
}

func TestClassifyRetry_UnknownPropertyIsContractShape(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Хорошо.","reply_language":"ru","media_files_to_send":[],"escalate":false,"extra_field":"x"}`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonContractShape)
	}
}

func TestClassifyRetry_BadLanguageIsContractShape(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Хорошо.","reply_language":"en","media_files_to_send":[],"escalate":false}`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonContractShape)
	}
}

func TestClassifyRetry_WrongTypeIsContractShape(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Хорошо.","reply_language":"ru","media_files_to_send":[],"escalate":"false"}`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonContractShape)
	}
}

func TestClassifyRetry_UnknownMediaTokenIsMediaNotFound(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Хорошо.","reply_language":"ru","media_files_to_send":["products.nope.featured_image"],"escalate":false}`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonMediaNotFound {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonMediaNotFound)
	}
	fb := RetryFeedback(raw, kb, cat)
	if fb == "" {
		t.Fatal("want non-empty corrective feedback naming the bad token")
	}
	if !strings.Contains(fb, "products.nope.featured_image") {
		t.Fatalf("feedback %q does not name the bad token", fb)
	}
	if !strings.Contains(fb, "media_files_to_send") {
		t.Fatalf("feedback %q does not name the field", fb)
	}
}

func TestClassifyRetry_StaleMediaTokenIsMediaNotFound(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	// The model saw a catalog where products.coffee-machine.featured_image existed;
	// the KB has since gone stale (the image was removed) — cat still lists the
	// token (it reflects what the model was shown), but ResolveSend against the
	// CURRENT kb must now fail.
	for i := range kb.Products {
		if kb.Products[i].Ref == "coffee-machine" {
			kb.Products[i].FeaturedImage = ""
		}
	}
	raw := `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["products.coffee-machine.featured_image"],"escalate":false}`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonMediaNotFound {
		t.Fatalf("ClassifyRetry() = %q, want %q", got, RetryReasonMediaNotFound)
	}
	fb := RetryFeedback(raw, kb, cat)
	if !strings.Contains(fb, "products.coffee-machine.featured_image") {
		t.Fatalf("feedback %q does not name the now-stale token", fb)
	}
}

func TestClassifyRetry_ContractShapeWinsOverMediaNotFound(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	// Both a bad_language contract issue AND an unknown media token are present —
	// contract_shape must win (evals/harness/retry.go:97-101).
	raw := `{"reply_text":"Хорошо.","reply_language":"en","media_files_to_send":["products.nope.featured_image"],"escalate":false}`
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetry() = %q, want %q (contract_shape must win)", got, RetryReasonContractShape)
	}
	if fb := RetryFeedback(raw, kb, cat); fb != "" {
		t.Fatalf("RetryFeedback() = %q, want empty — contract_shape retries are an identical re-roll", fb)
	}
}
