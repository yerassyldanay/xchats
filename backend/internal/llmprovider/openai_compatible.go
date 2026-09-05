// Package llmprovider hosts provider adapters implementing llm.ChatClient and
// the Registry/factory that resolves configured clients at the composition
// root. openai_compatible is the one implementation this package needs:
// OpenRouter, OpenAI, and any Gemini-compatible endpoint all speak the same
// OpenAI-style chat/completions wire format, differing only in base URL, key,
// and model string — all resolved from configuration, never hardcoded here.
package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// OpenAICompatible calls one OpenAI-wire-compatible chat/completions endpoint.
// The request shape matches exactly what the evaluated schema_kb_v1 pipeline
// sent (evals/harness/openrouter.go's chatText): one user message whose
// content is an array of parts ([{"type":"text","text":...}], never a plain
// string), and no tools, tool_choice, or response_format field at all.
type OpenAICompatible struct {
	httpc    *http.Client
	baseURL  string
	apiKey   string
	provider string // gen_ai.system label for tracing
	timeout  time.Duration
}

// NewOpenAICompatible builds a client for one provider's OpenAI-compatible
// endpoint. timeout bounds each individual Complete call (one call = one
// attempt; the engine's own retry makes a second, separately-timed call).
func NewOpenAICompatible(baseURL, apiKey, provider string, timeout time.Duration) *OpenAICompatible {
	return &OpenAICompatible{
		httpc:    &http.Client{Timeout: timeout},
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		provider: provider,
		timeout:  timeout,
	}
}

type wireContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *wireImageURL `json:"image_url,omitempty"`
}

type wireImageURL struct {
	URL string `json:"url"`
}

type wireMessage struct {
	Role    string            `json:"role"`
	Content []wireContentPart `json:"content"`
}

type wireRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Messages    []wireMessage `json:"messages"`
}

type wireResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete implements llm.ChatClient. It applies its own per-attempt timeout
// (independent of any deadline already on ctx) so one slow attempt can never
// silently run longer than configured.
func (c *OpenAICompatible) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	wreq := wireRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Messages:    toWireMessages(req.Messages),
	}

	ctx, gen := startGeneration(ctx, genParams{
		name: "llm.complete", provider: c.provider, model: req.Model,
		temperature: req.Temperature, maxTokens: req.MaxTokens,
		input: lastContent(req.Messages),
	})
	resp, err := c.complete(ctx, wreq)
	gen.setUsage(resp.PromptTokens, resp.CompletionTokens)
	gen.end(resp.Text, err)
	return resp, err
}

func toWireMessages(msgs []llm.Message) []wireMessage {
	out := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		out[i] = wireMessage{Role: m.Role, Content: toWireContentParts(m)}
	}
	return out
}

// toWireContentParts serializes one Message's content. m.Parts, when set,
// takes priority over m.Content (see llm.Message's own doc comment) — a
// PartText becomes a {"type":"text",...} part and a PartImage becomes
// {"type":"image_url","image_url":{"url":...}}, the OpenAI-compatible shape
// every provider this client calls (OpenAI, OpenRouter, Gemini's
// OpenAI-compatible endpoint) accepts for a vision-capable model. Falling
// back to a single text part for m.Content keeps every existing (non-Parts)
// caller's wire shape byte-identical to before Parts existed.
func toWireContentParts(m llm.Message) []wireContentPart {
	if len(m.Parts) == 0 {
		return []wireContentPart{{Type: "text", Text: m.Content}}
	}
	parts := make([]wireContentPart, len(m.Parts))
	for i, p := range m.Parts {
		switch p.Kind {
		case llm.PartImage:
			parts[i] = wireContentPart{Type: "image_url", ImageURL: &wireImageURL{URL: p.ImageURL}}
		default:
			parts[i] = wireContentPart{Type: "text", Text: p.Text}
		}
	}
	return parts
}

// lastContent returns the last message's text, for tracing's "input" field.
// A Parts-carrying message (a vision call) has no top-level Content — its
// text lives in a PartText part instead, so this falls back to that rather
// than tracing an empty input for every multimodal generation.
func lastContent(msgs []llm.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	m := msgs[len(msgs)-1]
	if m.Content != "" || len(m.Parts) == 0 {
		return m.Content
	}
	for _, p := range m.Parts {
		if p.Kind == llm.PartText {
			return p.Text
		}
	}
	return ""
}

func (c *OpenAICompatible) complete(ctx context.Context, wreq wireRequest) (llm.ChatResponse, error) {
	body, err := json.Marshal(wreq)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return llm.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpc.Do(httpReq)
	if err != nil {
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return llm.ChatResponse{}, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: http %d: %w: %s", resp.StatusCode, llm.ErrProviderAuth, string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: http %d: %s", resp.StatusCode, string(respBody))
	}

	var out wireResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: unparseable response: %w (%s)", err, string(respBody))
	}
	if out.Error != nil {
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: empty choices")
	}
	result := llm.ChatResponse{
		Text:         out.Choices[0].Message.Content,
		FinishReason: out.Choices[0].FinishReason,
	}
	if out.Usage != nil {
		result.PromptTokens = out.Usage.PromptTokens
		result.CompletionTokens = out.Usage.CompletionTokens
	}
	return result, nil
}
