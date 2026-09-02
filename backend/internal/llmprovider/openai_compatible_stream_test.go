package llmprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/llm"
)

func TestReadStreamAccumulatesDeltas(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":", "}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"world"},"finish_reason":"stop"}]}`,
		``,
		`data: {"choices":[],"usage":{"prompt_tokens":11,"completion_tokens":3}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var deltas []string
	resp, err := readStream(strings.NewReader(body), func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatalf("readStream: %v", err)
	}
	if resp.Text != "Hello, world" {
		t.Errorf("Text = %q, want %q", resp.Text, "Hello, world")
	}
	if got := strings.Join(deltas, "|"); got != "Hello|, |world" {
		t.Errorf("deltas = %q, want each chunk delivered separately", got)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", resp.FinishReason)
	}
	if resp.PromptTokens != 11 || resp.CompletionTokens != 3 {
		t.Errorf("usage = %d/%d, want 11/3", resp.PromptTokens, resp.CompletionTokens)
	}
}

// SSE frames may carry comments, event: lines, CRLF endings, and empty
// deltas (the role-only opening chunk every OpenAI-compatible provider
// sends). None of them is content.
func TestReadStreamIgnoresNonContentFrames(t *testing.T) {
	body := ": keep-alive\r\n" +
		"\r\n" +
		"event: ping\r\n" +
		"data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\r\n" +
		"\r\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"only this\"}}]}\r\n" +
		"\r\n" +
		"data: [DONE]\r\n\r\n"

	var deltas []string
	resp, err := readStream(strings.NewReader(body), func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatalf("readStream: %v", err)
	}
	if resp.Text != "only this" {
		t.Errorf("Text = %q, want %q", resp.Text, "only this")
	}
	if len(deltas) != 1 {
		t.Errorf("deltas = %v, want exactly one — an empty delta is not content", deltas)
	}
}

// A provider can answer 200 and then fail mid-stream. That must surface as an
// error, not as a silently truncated answer the caller would persist as
// complete.
func TestReadStreamSurfacesAMidStreamError(t *testing.T) {
	body := `data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n\n" +
		`data: {"error":{"message":"upstream capacity"}}` + "\n\n"

	_, err := readStream(strings.NewReader(body), nil)
	if err == nil {
		t.Fatal("readStream succeeded, want the mid-stream error surfaced")
	}
	if !strings.Contains(err.Error(), "upstream capacity") {
		t.Errorf("err = %v, want it to carry the provider's own message", err)
	}
}

func TestReadStreamRejectsAnUnparseableChunk(t *testing.T) {
	if _, err := readStream(strings.NewReader("data: {not json}\n\n"), nil); err == nil {
		t.Fatal("readStream accepted a malformed chunk")
	}
}

func TestStreamSendsStreamTrueAndDeliversTheAnswer(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, chunk := range []string{"Витамин D ", "стоит 12 000 ₸"} {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + strconv.Quote(chunk) + `}}]}` + "\n\n"))
			w.(http.Flusher).Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	client := NewOpenAICompatible(srv.URL, "test-key", "openrouter", 5*time.Second)
	var deltas []string
	resp, err := client.Stream(context.Background(), llm.ChatRequest{
		Model:    "google/gemini-2.5-flash",
		Messages: []llm.Message{{Role: "user", Content: "цена?"}},
	}, func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if resp.Text != "Витамин D стоит 12 000 ₸" {
		t.Errorf("Text = %q", resp.Text)
	}
	if len(deltas) != 2 {
		t.Errorf("deltas = %v, want two", deltas)
	}
	if gotBody["stream"] != true {
		t.Errorf("request body stream = %v, want true", gotBody["stream"])
	}
	if gotBody["model"] != "google/gemini-2.5-flash" {
		t.Errorf("request body model = %v", gotBody["model"])
	}
}

// A rejected key must stay distinguishable from a transient failure on the
// streaming path too — it is the difference between "add a key in Settings"
// and "try again".
func TestStreamReportsAuthFailuresAsProviderAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	client := NewOpenAICompatible(srv.URL, "bad-key", "openrouter", 5*time.Second)
	_, err := client.Stream(context.Background(), llm.ChatRequest{Model: "m"}, nil)
	if !errors.Is(err, llm.ErrProviderAuth) {
		t.Errorf("err = %v, want it to wrap llm.ErrProviderAuth", err)
	}
}

// OpenAICompatible must satisfy the streaming interface the chat service
// type-asserts for; if it stops doing so, chat silently falls back to
// non-streamed answers.
func TestOpenAICompatibleIsAStreamClient(t *testing.T) {
	var _ llm.StreamClient = (*OpenAICompatible)(nil)
}
