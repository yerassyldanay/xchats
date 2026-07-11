package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// orModelID strips the promptfoo-style "openrouter:" prefix models.yaml uses
// (e.g. "openrouter:google/gemini-2.5-flash" -> "google/gemini-2.5-flash") so the
// same models.yaml feeds both promptfoo (Layer 1) and this harness's direct OpenRouter
// calls (extraction eval) without a second model list to keep in sync.
func orModelID(id string) string {
	return strings.TrimPrefix(id, "openrouter:")
}

// orClient is a minimal OpenRouter chat/completions client — just enough to send one
// system+image turn and get back a JSON-object response. It deliberately does not use
// an SDK: this is a playground harness, and the wire format is a handful of fields.
type orClient struct {
	httpc   *http.Client
	baseURL string
	apiKey  string
}

func newORClient(baseURL, apiKey string) *orClient {
	return &orClient{
		httpc:   &http.Client{Timeout: 90 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}
}

type orUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	// CompletionTokensDetails mirrors OpenRouter's (OpenAI-compatible) usage breakdown —
	// confirmed shape via OpenRouter's own docs, NOT independently verified against a
	// live call in this repo (see ReasoningConfig's doc comment in types.go). A nil
	// pointer means "not reported by this provider," never "zero" — callers must not
	// conflate the two facts.
	CompletionTokensDetails *orCompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

// orCompletionTokensDetails is OpenRouter/OpenAI's completion-token breakdown. Every
// field is a pointer for the same "absent vs. zero" reason as the struct it's nested in.
type orCompletionTokensDetails struct {
	ReasoningTokens *int `json:"reasoning_tokens,omitempty"`
}

type orContentPart struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *orImageURL `json:"image_url,omitempty"`
}

type orImageURL struct {
	URL string `json:"url"`
}

type orMessage struct {
	Role    string          `json:"role"`
	Content []orContentPart `json:"content"`
}

type orRequest struct {
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	// FrequencyPenalty discourages the token-repetition degeneration loop observed
	// live (gpt-4o-mini burning its whole completion budget on repeated "\n" on one
	// product photo, more than once across separate runs) — a real reliability bug,
	// not a max_tokens sizing issue (it recurred even at 2000 tokens). OpenRouter
	// passes this straight through to providers that support it (OpenAI-compatible
	// param); providers that don't support it just ignore an unknown field.
	FrequencyPenalty float64       `json:"frequency_penalty,omitempty"`
	Messages         []orMessage   `json:"messages"`
	ResponseFormat   *orRespFormat `json:"response_format,omitempty"`
	// Provider pins the OpenRouter upstream route (see ProviderRoute in types.go); nil
	// (the field is entirely omitted) means unpinned, OpenRouter's default routing.
	Provider *orProviderPrefs `json:"provider,omitempty"`
	// Reasoning opts into OpenRouter's reasoning API (see ReasoningConfig in types.go);
	// nil means reasoning is left at the provider's own default (today, every model this
	// harness calls).
	Reasoning *orReasoningPrefs `json:"reasoning,omitempty"`
}

type orRespFormat struct {
	Type string `json:"type"`
}

// orProviderPrefs mirrors OpenRouter's provider-routing preferences object — see
// https://openrouter.ai/docs/guides/routing/provider-selection and ProviderRoute's doc
// comment in types.go for why pinning this matters for a comparison run.
type orProviderPrefs struct {
	Order          []string `json:"order,omitempty"`
	AllowFallbacks *bool    `json:"allow_fallbacks,omitempty"`
}

// orReasoningPrefs mirrors OpenRouter's reasoning request object — see ReasoningConfig's
// doc comment in types.go. Effort and MaxTokens are mutually exclusive per OpenRouter's
// own API; this harness forwards whichever is set without validating the exclusivity
// itself (see buildReasoningPrefs).
type orReasoningPrefs struct {
	Enabled   bool   `json:"enabled,omitempty"`
	Effort    string `json:"effort,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Exclude   bool   `json:"exclude,omitempty"`
}

// buildProviderPrefs and buildReasoningPrefs adapt a ModelProvider's optional config
// (types.go) into the OpenRouter wire shape for a DIRECT call (extraction eval). Mirrors
// render.go's buildPassthrough, which does the analogous job for promptfoo-mediated
// calls — kept as two separate functions (not shared) since the two paths' wire shapes
// differ (promptfoo's nested "passthrough" config key vs. these top-level request
// fields), not because the underlying config differs.
func buildProviderPrefs(p ModelProvider) *orProviderPrefs {
	if p.Provider == nil {
		return nil
	}
	return &orProviderPrefs{Order: p.Provider.Order, AllowFallbacks: p.Provider.AllowFallbacks}
}

func buildReasoningPrefs(p ModelProvider) *orReasoningPrefs {
	if p.Reasoning == nil {
		return nil
	}
	return &orReasoningPrefs{
		Enabled:   p.Reasoning.Enabled,
		Effort:    p.Reasoning.Effort,
		MaxTokens: p.Reasoning.MaxTokens,
		Exclude:   p.Reasoning.Exclude,
	}
}

type orResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			// Reasoning is OpenRouter's own reasoning-content response field —
			// confirmed via OpenRouter's docs, captured SEPARATELY from Content and
			// returned via describeImage's own `reasoning` return value, never
			// concatenated into `raw`. See judge.go's ReasoningLeak / this harness's
			// ReasoningMarkerRE for the (different) concern of a model inlining
			// reasoning INSIDE Content instead of using this field.
			Reasoning string `json:"reasoning,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage *orUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// describeImage sends one system-prompt + image turn and asks for a JSON-object reply.
// It returns the raw (still-unparsed) model text so the caller can record it verbatim
// before attempting to parse it — a parse failure is itself a result worth reporting,
// not a Go error to bubble up. reasoning is OpenRouter's separate message.reasoning
// field, if the provider returned one — returned as its OWN value specifically so no
// caller can accidentally concatenate it into raw (the customer-facing/graded text).
func (c *orClient) describeImage(ctx context.Context, model ModelProvider, systemPrompt string, imgData []byte, mimetype string) (raw string, reasoning string, usage orUsage, err error) {
	dataURL := "data:" + mimetype + ";base64," + base64.StdEncoding.EncodeToString(imgData)
	req := orRequest{
		Model:            orModelID(model.ID),
		Temperature:      model.Temperature,
		MaxTokens:        model.MaxTokens,
		FrequencyPenalty: 0.3,
		ResponseFormat:   &orRespFormat{Type: "json_object"},
		Messages: []orMessage{{
			Role: "user",
			Content: []orContentPart{
				{Type: "text", Text: systemPrompt},
				{Type: "image_url", ImageURL: &orImageURL{URL: dataURL}},
			},
		}},
		Provider:  buildProviderPrefs(model),
		Reasoning: buildReasoningPrefs(model),
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", "", orUsage{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", orUsage{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpc.Do(httpReq)
	if err != nil {
		return "", "", orUsage{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", orUsage{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", orUsage{}, fmt.Errorf("openrouter: http %d: %s", resp.StatusCode, string(respBody))
	}
	var out orResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", "", orUsage{}, fmt.Errorf("openrouter: unparseable response: %w (%s)", err, string(respBody))
	}
	if out.Error != nil {
		return "", "", orUsage{}, fmt.Errorf("openrouter: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", "", orUsage{}, fmt.Errorf("openrouter: empty choices")
	}
	if out.Usage != nil {
		usage = *out.Usage
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), strings.TrimSpace(out.Choices[0].Message.Reasoning), usage, nil
}

// estimateCost mirrors the harness's existing cost-basis discipline (judge.go
// CostBasis*): a model with no verified price in models.yaml reports "unknown_pricing"
// rather than a guessed number.
func estimateCost(model ModelProvider, usage orUsage) (float64, string) {
	if model.InputPerMTok == nil || model.OutputPerMTok == nil {
		return 0, CostBasisUnknownPricing
	}
	cost := float64(usage.PromptTokens)/1e6*(*model.InputPerMTok) +
		float64(usage.CompletionTokens)/1e6*(*model.OutputPerMTok)
	return cost, CostBasisMeasured
}

// resolveVisionModels picks which models.yaml providers to run: an explicit -models
// flag wins, else EVAL_VISION_MODELS from the environment, else every provider in
// models.yaml. Model ids are matched after stripping the "openrouter:" prefix, so both
// "google/gemini-2.5-flash" and "openrouter:google/gemini-2.5-flash" select the same entry.
func resolveVisionModels(flagValue, modelsPath string) ([]ModelProvider, error) {
	mf, err := loadModels(modelsPath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", modelsPath, err)
	}
	// Normalize every provider's ID once, up front — every path below (default,
	// matched, fallback) then returns de-prefixed IDs uniformly, so no caller needs to
	// re-strip defensively and no divergence can creep back in between the paths.
	byID := map[string]ModelProvider{}
	normalized := make([]ModelProvider, len(mf.Providers))
	for i, p := range mf.Providers {
		p.ID = orModelID(p.ID)
		normalized[i] = p
		byID[p.ID] = p
	}

	selected := flagValue
	if selected == "" {
		selected = envOrDefault("EVAL_VISION_MODELS", "")
	}
	if selected == "" {
		return normalized, nil
	}
	var out []ModelProvider
	for _, id := range splitCSV(selected) {
		id = orModelID(id)
		if p, ok := byID[id]; ok {
			out = append(out, p)
			continue
		}
		// Not in models.yaml (e.g. a one-off comparison) — run it with harness defaults;
		// it will report unknown_pricing since there is no input_per_mtok/output_per_mtok for it.
		out = append(out, ModelProvider{ID: id, Temperature: 0.3, MaxTokens: 700})
	}
	return out, nil
}
