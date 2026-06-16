// Package llm is the Drafter adapter: an OpenAI-compatible chat/completions call,
// forcing the emit_draft function for strict JSON (docs/10 · the LLM call).
// Provider-switchable (openrouter | openai | gemini) — they all speak the OpenAI
// wire format, so only base URL / key / model change (resolved in config). Config
// is provider-neutral: LLM_* only.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// Drafter calls the configured OpenAI-compatible provider. It satisfies the
// drafter contract consumed by internal/assistant.RealDrafter.
type Drafter struct {
	httpc         *http.Client
	baseURL       string
	apiKey        string
	fastModel     string // used for drafting (the Draft path)
	thinkingModel string // stronger model, available for harder calls
	maxTokens     int
	temperature   float64
}

// New builds the Drafter from LLM_* config. Drafting uses fastModel; thinkingModel
// is retained for callers that need deeper reasoning.
func New(baseURL, apiKey, fastModel, thinkingModel string, maxTokens int, temperature float64) *Drafter {
	return &Drafter{
		httpc:         &http.Client{Timeout: 60 * time.Second},
		baseURL:       strings.TrimRight(baseURL, "/"),
		apiKey:        apiKey,
		fastModel:     fastModel,
		thinkingModel: thinkingModel,
		maxTokens:     maxTokens,
		temperature:   temperature,
	}
}

// --- wire types (OpenAI-compatible) ---

type chatRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Messages    []message `json:"messages"`
	Tools       []tool    `json:"tools"`
	ToolChoice  any       `json:"tool_choice"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type tool struct {
	Type     string      `json:"type"`
	Function functionDef `json:"function"`
}

type functionDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			ToolCalls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Draft issues the chat/completions call and decodes the forced tool arguments.
func (d *Drafter) Draft(ctx context.Context, p brain.Prompt) (domain.RawDraft, error) {
	reqBody := chatRequest{
		Model:       d.fastModel,
		MaxTokens:   d.maxTokens,
		Temperature: d.temperature,
		Messages: []message{
			{Role: "system", Content: p.System},
			{Role: "user", Content: p.User},
		},
		Tools:      []tool{emitDraftTool()},
		ToolChoice: map[string]any{"type": "function", "function": map[string]string{"name": "emit_draft"}},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return domain.RawDraft{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return domain.RawDraft{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.httpc.Do(req)
	if err != nil {
		return domain.RawDraft{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return domain.RawDraft{}, fmt.Errorf("llm http %d: %s", resp.StatusCode, string(body))
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return domain.RawDraft{}, fmt.Errorf("llm decode: %w", err)
	}
	if cr.Error != nil {
		return domain.RawDraft{}, fmt.Errorf("llm error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return domain.RawDraft{}, fmt.Errorf("llm: no choices")
	}
	args := firstToolArgs(cr)
	if args == "" {
		return domain.RawDraft{}, fmt.Errorf("llm: no emit_draft tool call")
	}
	return ParseRawDraft(args)
}

func firstToolArgs(cr chatResponse) string {
	calls := cr.Choices[0].Message.ToolCalls
	if len(calls) == 0 {
		// Fallback: some providers return JSON in content when tools aren't supported.
		return strings.TrimSpace(cr.Choices[0].Message.Content)
	}
	return calls[0].Function.Arguments
}

// ParseRawDraft defensively parses the tool arguments (a JSON string), stripping
// any stray code fences before unmarshalling (docs/02 · post-processing step 1).
func ParseRawDraft(s string) (domain.RawDraft, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	var rd domain.RawDraft
	if err := json.Unmarshal([]byte(s), &rd); err != nil {
		return domain.RawDraft{}, fmt.Errorf("parse draft json: %w", err)
	}
	return rd, nil
}

// emitDraftTool is the forced function schema (docs/10).
func emitDraftTool() tool {
	return tool{
		Type: "function",
		Function: functionDef{
			Name:        "emit_draft",
			Description: "Return exactly one reply draft for the human to review.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reply_text":     map[string]any{"type": "string", "description": "Uses {{price.*}}/{{limit.*}} tokens, never digits."},
					"reply_language": map[string]any{"type": "string", "enum": []string{"ru", "kk"}},
					"asset_refs":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 3},
					"profile_patch":  map[string]any{"type": "object", "description": "Only newly-confident fields."},
					"suggested_callback": map[string]any{"type": "object", "properties": map[string]any{
						"due_at": map[string]any{"type": "string"}, "note": map[string]any{"type": "string"},
					}},
					"suggested_status": map[string]any{"type": "object", "properties": map[string]any{
						"stage": map[string]any{"type": "string"},
					}},
					"confidence":        map[string]any{"type": "number"},
					"escalate":          map[string]any{"type": "boolean"},
					"escalation_reason": map[string]any{"type": "string"},
				},
				"required": []string{"reply_text", "reply_language", "asset_refs", "confidence", "escalate"},
			},
		},
	}
}
