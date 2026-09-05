package llmprovider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// TestComplete_WireShapeMatchesEvaluatedPipeline pins the request shape to
// exactly what the evaluated schema_kb_v1 pipeline sent (evals/harness/openrouter.go's
// chatText): one user message, content is an array of parts (never a plain
// string), and no tools/tool_choice/response_format field at all.
func TestComplete_WireShapeMatchesEvaluatedPipeline(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	}))
	defer srv.Close()

	c := NewOpenAICompatible(srv.URL, "test-key", "openrouter", 5*time.Second)
	resp, err := c.Complete(context.Background(), llm.ChatRequest{
		Model:       "google/gemini-2.5-flash",
		Temperature: 0.3,
		MaxTokens:   500,
		Messages:    []llm.Message{{Role: "user", Content: "Сколько стоит кофемашина?"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "ok" || resp.FinishReason != "stop" || resp.PromptTokens != 10 || resp.CompletionTokens != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if _, ok := gotBody["tools"]; ok {
		t.Error("request must not include a tools field")
	}
	if _, ok := gotBody["tool_choice"]; ok {
		t.Error("request must not include a tool_choice field")
	}
	if _, ok := gotBody["response_format"]; ok {
		t.Error("request must not include a response_format field")
	}
	if gotBody["model"] != "google/gemini-2.5-flash" {
		t.Errorf("model = %v, want google/gemini-2.5-flash", gotBody["model"])
	}
	if gotBody["temperature"] != 0.3 {
		t.Errorf("temperature = %v, want 0.3", gotBody["temperature"])
	}
	if gotBody["max_tokens"] != float64(500) {
		t.Errorf("max_tokens = %v, want 500", gotBody["max_tokens"])
	}

	messages, ok := gotBody["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %v, want exactly one message", gotBody["messages"])
	}
	msg, ok := messages[0].(map[string]any)
	if !ok || msg["role"] != "user" {
		t.Fatalf("unexpected message shape: %v", messages[0])
	}
	content, ok := msg["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content = %v (%T), want an array of exactly one part — not a plain string", msg["content"], msg["content"])
	}
	part, ok := content[0].(map[string]any)
	if !ok || part["type"] != "text" || part["text"] != "Сколько стоит кофемашина?" {
		t.Fatalf("unexpected content part: %v", content[0])
	}
}

// TestComplete_SerializesImageParts covers a vision call: Message.Parts
// (text + one image) must serialize as an OpenAI-compatible multi-part
// content array, with the image as {"type":"image_url","image_url":{"url":...}}.
func TestComplete_SerializesImageParts(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"это смартфон"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	const dataURI = "data:image/jpeg;base64,/9j/4AAQ=="
	c := NewOpenAICompatible(srv.URL, "test-key", "openai", 5*time.Second)
	resp, err := c.Complete(context.Background(), llm.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{{
			Role: "user",
			Parts: []llm.ContentPart{
				{Kind: llm.PartText, Text: "Что на этом фото?"},
				{Kind: llm.PartImage, ImageURL: dataURI},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "это смартфон" {
		t.Fatalf("resp.Text = %q", resp.Text)
	}

	messages, ok := gotBody["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %v", gotBody["messages"])
	}
	msg := messages[0].(map[string]any)
	content, ok := msg["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("content = %v, want exactly 2 parts (text + image)", msg["content"])
	}
	textPart := content[0].(map[string]any)
	if textPart["type"] != "text" || textPart["text"] != "Что на этом фото?" {
		t.Fatalf("unexpected text part: %v", textPart)
	}
	imagePart := content[1].(map[string]any)
	if imagePart["type"] != "image_url" {
		t.Fatalf("unexpected image part type: %v", imagePart)
	}
	imageURL, ok := imagePart["image_url"].(map[string]any)
	if !ok || imageURL["url"] != dataURI {
		t.Fatalf("image_url = %v, want url %q", imagePart["image_url"], dataURI)
	}
	if _, hasText := imagePart["text"]; hasText {
		t.Error("image part must not also carry a text field")
	}
}

func TestComplete_PropagatesProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	c := NewOpenAICompatible(srv.URL, "k", "openrouter", 5*time.Second)
	_, err := c.Complete(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	if err == nil {
		t.Fatal("want an error for a provider error field")
	}
}

func TestComplete_PropagatesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "boom")
	}))
	defer srv.Close()

	c := NewOpenAICompatible(srv.URL, "k", "openrouter", 5*time.Second)
	_, err := c.Complete(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	if err == nil {
		t.Fatal("want an error for a non-200 response")
	}
}

func TestComplete_ClassifiesUnauthorizedAsErrProviderAuth(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
			}))
			defer srv.Close()

			c := NewOpenAICompatible(srv.URL, "k", "openrouter", 5*time.Second)
			_, err := c.Complete(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
			if !errors.Is(err, llm.ErrProviderAuth) {
				t.Fatalf("err = %v, want errors.Is(err, llm.ErrProviderAuth)", err)
			}
		})
	}
}

func TestComplete_OtherHTTPErrorsAreNotErrProviderAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, "rate limited")
	}))
	defer srv.Close()

	c := NewOpenAICompatible(srv.URL, "k", "openrouter", 5*time.Second)
	_, err := c.Complete(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	if errors.Is(err, llm.ErrProviderAuth) {
		t.Fatalf("a 429 must never classify as ErrProviderAuth: %v", err)
	}
}

// TestComplete_TimeoutHonored asserts each Complete call applies its own
// per-attempt timeout regardless of the caller's context deadline.
func TestComplete_TimeoutHonored(t *testing.T) {
	// block is closed BEFORE srv.Close() runs (defers are LIFO): httptest.Server.Close
	// waits for in-flight handlers to return, so closing the server first while the
	// handler is still blocked on <-block would deadlock the test itself.
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	c := NewOpenAICompatible(srv.URL, "k", "openrouter", 50*time.Millisecond)
	start := time.Now()
	_, err := c.Complete(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("want a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Complete took %s, want it bounded by the configured timeout", elapsed)
	}
}
