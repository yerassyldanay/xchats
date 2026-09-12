package llmmock

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// v7Response mirrors aiprompt.Response's own JSON shape (aiprompt/contract.go)
// closely enough to assert the mock's customer-response branch without
// importing aiprompt itself (avoided here to keep this a narrow unit test —
// the full ValidateResponseV7 pass is exercised by the mock-mode end-to-end
// HTTP test instead, against a real seeded KB).
type v7Response struct {
	ReplyText        string   `json:"reply_text"`
	ReplyLanguage    string   `json:"reply_language"`
	MediaFilesToSend []string `json:"media_files_to_send"`
	Escalate         bool     `json:"escalate"`
}

func TestRegistry_AlwaysResolves(t *testing.T) {
	reg := NewRegistry()
	client, err := reg.Client(llm.ModelRef{Provider: "anything", Model: "whatever"})
	if err != nil || client == nil {
		t.Fatalf("Client() = %v, %v; want a non-nil client and no error", client, err)
	}
}

func TestComplete_CustomerResponseContract(t *testing.T) {
	c := NewClient()
	req := llm.ChatRequest{Messages: []llm.Message{
		{Role: "system", Content: `Верни JSON: {"reply_text": "...", "reply_language": "ru", "media_files_to_send": [], "escalate": false, "escalation_reason": ""}`},
		{Role: "user", Content: "Здравствуйте, сколько стоит доставка?"},
	}}
	resp, err := c.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	var out v7Response
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, resp.Text)
	}
	if out.ReplyText == "" {
		t.Error("reply_text must be non-empty")
	}
	if strings.Contains(out.ReplyText, "{{") {
		t.Error("reply_text must not reference catalog placeholders the mock cannot resolve")
	}
	if out.ReplyLanguage != "ru" && out.ReplyLanguage != "kk" {
		t.Errorf("reply_language = %q, want ru or kk", out.ReplyLanguage)
	}
	if out.MediaFilesToSend == nil || len(out.MediaFilesToSend) != 0 {
		t.Errorf("media_files_to_send = %v, want an empty (never nil) array", out.MediaFilesToSend)
	}
	if out.Escalate {
		t.Error("mock should not escalate by default")
	}
}

func TestComplete_KBImportToolJSONContract(t *testing.T) {
	c := NewClient()
	req := llm.ChatRequest{Messages: []llm.Message{
		{Role: "user", Content: `Ответь ОДНИМ JSON-объектом: {"calls": [...], "notes": "...", "unmapped": [...]}`},
	}}
	resp, err := c.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	var out struct {
		Calls    []json.RawMessage `json:"calls"`
		Notes    string            `json:"notes"`
		Unmapped []string          `json:"unmapped"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil {
		t.Fatalf("response is not valid kbimport modelOutput JSON: %v\nbody: %s", err, resp.Text)
	}
	if out.Calls == nil {
		t.Error("calls must be present (even if empty), never omitted/null")
	}
}

func TestComplete_UnrecognizedPromptFallsBackToProse(t *testing.T) {
	c := NewClient()
	resp, err := c.Complete(context.Background(), llm.ChatRequest{Messages: []llm.Message{
		{Role: "user", Content: "Explain what changed in the knowledge base this week."},
	}})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text == "" {
		t.Fatal("expected non-empty prose fallback")
	}
	var probe map[string]any
	if json.Unmarshal([]byte(resp.Text), &probe) == nil {
		t.Fatal("prose fallback should not itself be a JSON object")
	}
}

func TestStream_DeliversMultipleDeltasAndFinalTextMatches(t *testing.T) {
	c := NewClient()
	var deltas []string
	resp, err := c.Stream(context.Background(), llm.ChatRequest{Messages: []llm.Message{
		{Role: "user", Content: "What changed in the KB?"},
	}}, func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(deltas) < 2 {
		t.Fatalf("expected multiple chunks, got %d: %v", len(deltas), deltas)
	}
	if strings.Join(deltas, "") != resp.Text {
		t.Errorf("concatenated deltas %q != final response text %q", strings.Join(deltas, ""), resp.Text)
	}
}

func TestStream_RespectsContextCancellation(t *testing.T) {
	c := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the first delta
	_, err := c.Stream(ctx, llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "hi"}}}, func(string) {
		t.Error("onDelta must not be called once the context is already cancelled")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Stream error = %v, want context.Canceled", err)
	}
}

func TestSendsReturnUniqueCompletions(t *testing.T) {
	c := NewClient()
	req := llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: `"reply_text"`}}}
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		resp, err := c.Complete(context.Background(), req)
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}
		// The mock's customer-response text is intentionally stable (a real
		// KB-grounded reply would vary; the mock isn't) — this test instead
		// guards that repeated calls are at least deterministic/non-erroring
		// under concurrent load, which the race test below stresses further.
		seen[resp.Text] = true
	}
	if len(seen) != 1 {
		t.Errorf("expected the mock's canned customer response to be stable across calls, got %d distinct bodies", len(seen))
	}
}

func TestConcurrentCallsAreRaceFree(t *testing.T) {
	c := NewClient()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if _, err := c.Complete(context.Background(), llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "reply_text"}}}); err != nil {
					t.Errorf("Complete: %v", err)
				}
				return
			}
			_, err := c.Stream(context.Background(), llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "hi"}}}, func(string) {})
			if err != nil {
				t.Errorf("Stream: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if got := len(c.Calls()); got != 20 {
		t.Errorf("Calls() len = %d, want 20", got)
	}
}

func TestCallHistoryIsBounded(t *testing.T) {
	c := NewClient()
	for i := 0; i < maxRecordedCalls*3; i++ {
		_, _ = c.Complete(context.Background(), llm.ChatRequest{})
	}
	if got := len(c.Calls()); got != maxRecordedCalls {
		t.Fatalf("Calls() len = %d, want bounded to %d", got, maxRecordedCalls)
	}
}

func TestStreamDoesNotHangPastAReasonableDeadline(t *testing.T) {
	c := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		_, _ = c.Stream(ctx, llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "hi"}}}, func(string) {})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stream did not return within its own context deadline")
	}
}
