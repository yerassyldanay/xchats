// Package llmmock is an in-memory llm.Registry/llm.ChatClient/
// llm.StreamClient for cmd/xchats' mock-externals mode. It never makes an
// HTTP call: Complete sniffs the rendered prompt text to tell apart the
// three purpose-specific contracts this codebase's callers actually expect
// (response.Engine's aiprompt v7 customer-response JSON, internal/kbimport's
// synthesis tool-call JSON, and everything else falls back to plain prose),
// and Stream — used only by internal/chat's KB-chat assistant, the one
// caller that type-asserts for llm.StreamClient — always answers with
// streamed prose, chunked so incremental-rendering callers see more than
// one delta.
package llmmock

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// maxRecordedCalls bounds Client.Calls() — see metamock's identical
// reasoning (a long profiling run must not grow the fake's own memory).
const maxRecordedCalls = 200

// Call records one Complete/Stream request for test/introspection use.
type Call struct {
	Streamed bool
	Request  llm.ChatRequest
}

// Registry always resolves to the same shared Client regardless of
// ref.Provider/ref.Model — mock mode has no per-provider credentials to
// pick between, and every caller (response.Engine, internal/chat,
// internal/kbimport) just needs *a* working llm.ChatClient.
type Registry struct {
	client *Client
}

// NewRegistry returns a Registry backed by a fresh Client.
func NewRegistry() *Registry {
	return &Registry{client: NewClient()}
}

func (r *Registry) Client(ref llm.ModelRef) (llm.ChatClient, error) {
	return r.client, nil
}

// Underlying returns the shared mock Client, for tests that want to inspect
// recorded calls.
func (r *Registry) Underlying() *Client { return r.client }

var _ llm.Registry = (*Registry)(nil)

// Client is an in-memory llm.ChatClient + llm.StreamClient. Zero value is
// ready to use; NewClient is equivalent, kept for symmetry with metamock.
type Client struct {
	mu    sync.Mutex
	calls []Call
}

// NewClient returns a ready-to-use mock Client.
func NewClient() *Client { return &Client{} }

var (
	_ llm.ChatClient   = (*Client)(nil)
	_ llm.StreamClient = (*Client)(nil)
)

func (c *Client) record(req llm.ChatRequest, streamed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, Call{Streamed: streamed, Request: req})
	if over := len(c.calls) - maxRecordedCalls; over > 0 {
		c.calls = c.calls[over:]
	}
}

// Calls returns a snapshot of every recorded call, oldest first.
func (c *Client) Calls() []Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Call, len(c.calls))
	copy(out, c.calls)
	return out
}

// promptText concatenates every message's own Content plus any text content
// parts — enough to sniff which purpose-specific contract the caller wants;
// an image part's data URI is deliberately excluded (irrelevant to contract
// detection, and needlessly bloats the string on a vision request).
func promptText(req llm.ChatRequest) string {
	var b strings.Builder
	for _, m := range req.Messages {
		b.WriteString(m.Content)
		b.WriteByte('\n')
		for _, p := range m.Parts {
			if p.Kind == llm.PartText {
				b.WriteString(p.Text)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// Complete implements llm.ChatClient. response.Engine and internal/kbimport
// both call only Complete, never Stream (see their own doc comments) — this
// is the one method that must tell their two distinct JSON contracts apart.
func (c *Client) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	c.record(req, false)
	text := promptText(req)
	switch {
	// "reply_text" is aiprompt v7's own response-contract field name,
	// spelled out literally in every shop-kb-v7*.txt frame's instructions
	// (see aiprompt/frames) — a reliable, distinctive marker that a kbimport
	// synthesis prompt never contains.
	case strings.Contains(text, "reply_text"):
		return customerResponse(text), nil
	// `"unmapped"` is kbimport's own output contract's third top-level key
	// (internal/kbimport/prompt.go's outputContractDescription) — equally
	// distinctive against the v7 contract above.
	case strings.Contains(text, `"unmapped"`):
		return kbImportResponse(), nil
	default:
		return proseResponse(text), nil
	}
}

// Stream implements llm.StreamClient. Only internal/chat's KB-chat assistant
// ever calls this (it type-asserts for the interface and falls back to
// Complete otherwise) — so this always answers with the same prose Complete
// falls back to, chunked word-by-word so a caller exercising incremental
// rendering (SSE deltas) sees more than one delta, and stops early on
// context cancellation like a real streaming provider would.
func (c *Client) Stream(ctx context.Context, req llm.ChatRequest, onDelta func(string)) (llm.ChatResponse, error) {
	c.record(req, true)
	resp := proseResponse(promptText(req))
	var sent strings.Builder
	for _, chunk := range strings.SplitAfter(resp.Text, " ") {
		if chunk == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return llm.ChatResponse{Text: sent.String(), FinishReason: "cancelled"}, ctx.Err()
		default:
		}
		if onDelta != nil {
			onDelta(chunk)
		}
		sent.WriteString(chunk)
	}
	return resp, nil
}

// customerResponse produces a valid aiprompt v7 Response body (see
// aiprompt.Response / ValidateResponseV7): reply_language "ru", no
// escalation, and — deliberately — no {{table.ref.column}} catalog
// placeholders and an empty media_files_to_send, since a generic mock has
// no real catalog/media tokens to reference correctly and either would fail
// ValidateResponseV7's own catalog cross-check.
func customerResponse(prompt string) llm.ChatResponse {
	raw, _ := json.Marshal(map[string]any{
		"reply_text":          "Спасибо за обращение! Уточните, пожалуйста, детали вашего вопроса, и мы поможем.",
		"reply_language":      "ru",
		"media_files_to_send": []string{},
		"escalate":            false,
		"escalation_reason":   "",
		"confidence":          0.9,
	})
	return llm.ChatResponse{
		Text: string(raw), FinishReason: "stop",
		PromptTokens: estimateTokens(prompt), CompletionTokens: estimateTokens(string(raw)),
	}
}

// kbImportResponse produces a syntactically valid kbimport modelOutput body
// (internal/kbimport/synth.go's modelOutput: Calls/Notes/Unmapped) — zero
// calls is a legitimate, terminal answer kbimport's own retry logic already
// handles (see internal/kbimport's own doc comment on the corrective re-ask
// loop), so this never open-loops the mock.
func kbImportResponse() llm.ChatResponse {
	raw, _ := json.Marshal(map[string]any{
		"calls":    []any{},
		"notes":    "mock-externals: no changes synthesized",
		"unmapped": []string{},
	})
	return llm.ChatResponse{Text: string(raw), FinishReason: "stop", CompletionTokens: estimateTokens(string(raw))}
}

// proseResponse is the KB-chat assistant's answer shape: plain prose, no
// JSON contract expected.
func proseResponse(prompt string) llm.ChatResponse {
	const text = "Это тестовый ответ ассистента базы знаний в режиме mock-externals."
	return llm.ChatResponse{Text: text, FinishReason: "stop", PromptTokens: estimateTokens(prompt), CompletionTokens: estimateTokens(text)}
}

// estimateTokens is a rough, deterministic stand-in for a real tokenizer —
// good enough for the load harness's own token-count reporting, which only
// needs plausible non-zero numbers, never exact ones.
func estimateTokens(s string) int {
	if n := len(s) / 4; n > 0 {
		return n
	}
	return 1
}
