package main

import (
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// TestAipromptCrossModuleWiring proves the harness module resolves and links
// against backend/aiprompt via the go.mod require+replace pair (see
// go.mod: require github.com/yerassyldanay/xchats/backend + a local replace
// pointing at ../../backend). aiprompt is deliberately outside backend/internal
// so this cross-module import is legal — Go's internal-package rule would
// otherwise block it regardless of the replace directive. Every other
// schema_kb_v1 test exercises the real package; this one only exists to fail
// loudly, with an obvious cause, the moment the module wiring itself breaks.
func TestAipromptCrossModuleWiring(t *testing.T) {
	kb := &aiprompt.KB{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		Assistant:      &aiprompt.Assistant{Persona: "Ты — тестовый ассистент."},
		Products: []aiprompt.Product{
			{Ref: "smoke-test", Name: "Тест", Price: "1 000 ₸", InStock: true, SalesStatus: "active"},
		},
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatalf("aiprompt.BuildCatalog: %v", err)
	}
	if cat.FactByToken("{{product.smoke-test.price}}") == nil {
		t.Fatal("expected the price fact to be present")
	}

	prompt, err := aiprompt.RenderPrompt("FACTS:\n%%FACTS%%\n\nSCHEMA:\n%%RESPONSE_SCHEMA%%", kb.PromptInput(), cat)
	if err != nil {
		t.Fatalf("aiprompt.RenderPrompt: %v", err)
	}
	if !strings.Contains(prompt, "{{product.smoke-test.price}}") {
		t.Errorf("expected the rendered prompt to contain the price token, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "media_files_to_send") {
		t.Errorf("expected the rendered response schema to mention media_files_to_send, got:\n%s", prompt)
	}
}
