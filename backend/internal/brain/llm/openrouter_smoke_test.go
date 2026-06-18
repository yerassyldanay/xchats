package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
)

// TestDraft_LiveHTTPPath drives the real Drafter HTTP client against a fake
// OpenAI-compatible server: it asserts the forced emit_draft tool_choice is sent
// and the tool-call arguments are parsed into a RawDraft (the one seam the
// fakeLLM unit tests skip).
func TestDraft_LiveHTTPPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		if req["tool_choice"] == nil {
			t.Errorf("expected forced tool_choice, got none")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"emit_draft",
			"arguments":"{\"reply_text\":\"Стандарт — {{price.standard}}.\",\"reply_language\":\"ru\",\"asset_refs\":[\"pricing_card\"],\"confidence\":0.9,\"escalate\":false}"}}]}}]}`)
	}))
	defer srv.Close()

	d := New(srv.URL, "test-key", "openai", "openai/gpt-4o-mini", "", 1024, 0.3)
	raw, err := d.Draft(context.Background(), brain.Prompt{System: "sys", User: "Сколько стоит?"})
	if err != nil {
		t.Fatalf("live draft: %v", err)
	}
	if !strings.Contains(raw.ReplyText, "{{price.standard}}") {
		t.Fatalf("token should survive the wire intact: %q", raw.ReplyText)
	}
	if raw.ReplyLanguage != "ru" || raw.Escalate || len(raw.AssetRefs) != 1 {
		t.Fatalf("unexpected parsed draft: %+v", raw)
	}
}

// TestDraft_ContentFallback covers a provider that returns JSON in message.content
// (no tool_calls) — the defensive fallback path.
func TestDraft_ContentFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"reply_text\\\":\\\"ok\\\",\\\"reply_language\\\":\\\"ru\\\",\\\"asset_refs\\\":[],\\\"confidence\\\":0.5,\\\"escalate\\\":false}\\n```\"}}]}")
	}))
	defer srv.Close()

	d := New(srv.URL, "k", "openai", "m", "", 256, 0.2)
	raw, err := d.Draft(context.Background(), brain.Prompt{})
	if err != nil {
		t.Fatalf("content fallback: %v", err)
	}
	if raw.ReplyText != "ok" {
		t.Fatalf("fallback parse failed: %+v", raw)
	}
}
