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
	// the KB has since gone stale (the image AND its gallery_images fallback
	// were both removed — featured_image alone is override-with-fallback, so
	// clearing just the explicit value would re-resolve to gallery_images[0]
	// instead of going stale, see resolvedFeaturedImage) — cat still lists
	// the token (it reflects what the model was shown), but ResolveSend
	// against the CURRENT kb must now fail.
	for i := range kb.Products {
		if kb.Products[i].Ref == "coffee-machine" {
			kb.Products[i].FeaturedImage = ""
			kb.Products[i].GalleryImages = nil
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

// TestClassifyRetryV7_ValidKBGapNeedsNoRetry is the regression guard for the
// bug fixed alongside ClassifyRetryV7's introduction: ClassifyRetry (v6)
// treats a "kb_gap" property as unknown_property (contract_shape), which
// would force a wasted retry on nearly every v7 escalation. ClassifyRetryV7
// must not.
func TestClassifyRetryV7_ValidKBGapNeedsNoRetry(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Секунду, уточню.","reply_language":"ru","media_files_to_send":[],"escalate":true,` +
		`"escalation_reason":"нет цены","kb_gap":{"reason_code":"missing_field","target_entity_type":"product",` +
		`"target_entity_ref":"coffee-machine","missing_fields":["price"]}}`

	if got := ClassifyRetryV7(raw, kb, cat); got != RetryReasonNone {
		t.Fatalf("ClassifyRetryV7() = %q, want %q — a valid kb_gap must never force a retry", got, RetryReasonNone)
	}
	if fb := RetryFeedbackV7(raw, kb, cat); fb != "" {
		t.Fatalf("RetryFeedbackV7() = %q, want empty", fb)
	}

	// The bug this guards against: the OLD (v6) function really does
	// misclassify the identical response as contract_shape.
	if got := ClassifyRetry(raw, kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetry() = %q, want %q — kb_gap must be unknown_property on the v6 path (documents why ClassifyRetryV7 exists)", got, RetryReasonContractShape)
	}
}

func TestClassifyRetryV7_UnparseableJSONIsContractShape(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	if got := ClassifyRetryV7("not json at all", kb, cat); got != RetryReasonContractShape {
		t.Fatalf("ClassifyRetryV7() = %q, want %q", got, RetryReasonContractShape)
	}
}

func TestClassifyRetryV7_MediaNotFoundStillDetected(t *testing.T) {
	kb, cat := retryTestCatalog(t)
	raw := `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["products.nope.featured_image"],"escalate":false}`
	if got := ClassifyRetryV7(raw, kb, cat); got != RetryReasonMediaNotFound {
		t.Fatalf("ClassifyRetryV7() = %q, want %q", got, RetryReasonMediaNotFound)
	}
	fb := RetryFeedbackV7(raw, kb, cat)
	if !strings.Contains(fb, "products.nope.featured_image") {
		t.Fatalf("feedback %q does not name the bad token", fb)
	}
}
