package llm

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

// Vision is the multimodal extractor for the Playground's Stage-1 image adapter.
// The Drafter's wire message carries text-only content; vision needs the OpenAI
// content-array shape ([{type:text},{type:image_url}]), so it has its own request
// type here. It satisfies playground.VisionClient. PDFs/docs are not images and
// are rejected (→ the describe_media fallback) until a doc path is added.
type Vision struct {
	httpc     *http.Client
	baseURL   string
	apiKey    string
	provider  string // gen_ai.system label for Langfuse
	model     string
	maxTokens int
}

// NewVision builds a Vision client (uses a multimodal model, e.g. gpt-4o).
// provider is a label only (the wire format is OpenAI-compatible regardless).
func NewVision(baseURL, apiKey, provider, model string, maxTokens int) *Vision {
	if maxTokens <= 0 {
		maxTokens = 512
	}
	return &Vision{
		httpc:     &http.Client{Timeout: 60 * time.Second},
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		provider:  provider,
		model:     model,
		maxTokens: maxTokens,
	}
}

// visionPrompt asks for the asset's selection cue: what it shows + when to send it.
const visionPrompt = "Опиши кратко (1–2 предложения), что показывает это изображение и " +
	"в какой ситуации его стоит отправить клиенту. Не выдумывай цены и числа."

type visionContent struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *visionImage `json:"image_url,omitempty"`
}

type visionImage struct {
	URL string `json:"url"`
}

type visionMessage struct {
	Role    string          `json:"role"`
	Content []visionContent `json:"content"`
}

type visionRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []visionMessage `json:"messages"`
}

// Describe returns a caption for image bytes. Non-image mimetypes are rejected so
// the caller falls back to a describe_media popup. The call is recorded as a
// Langfuse generation span; the raw image bytes are never sent to Langfuse (only
// a redacted placeholder), per the PII policy.
func (v *Vision) Describe(ctx context.Context, mimetype string, data []byte) (string, error) {
	if !strings.HasPrefix(mimetype, "image/") {
		return "", fmt.Errorf("vision: unsupported mimetype %q", mimetype)
	}
	// This generation is its own root trace (no parent operation), so it carries
	// the feature tag directly. Image bytes are never sent — only a placeholder.
	input := fmt.Sprintf("%s\n[image omitted: %s, %d bytes]", visionPrompt, mimetype, len(data))
	ctx, gen := startGeneration(ctx, genParams{
		name: "image-caption", provider: v.provider, model: v.model,
		maxTokens: v.maxTokens, tags: []string{"xchats", "kb-vision"}, input: input,
	})
	caption, usage, err := v.describe(ctx, mimetype, data)
	if usage != nil {
		gen.setUsage(usage.PromptTokens, usage.CompletionTokens)
	}
	gen.end(caption, err)
	return caption, err
}

func (v *Vision) describe(ctx context.Context, mimetype string, data []byte) (string, *tokenUsage, error) {
	dataURL := "data:" + mimetype + ";base64," + base64.StdEncoding.EncodeToString(data)
	reqBody := visionRequest{
		Model:     v.model,
		MaxTokens: v.maxTokens,
		Messages: []visionMessage{{
			Role: "user",
			Content: []visionContent{
				{Type: "text", Text: visionPrompt},
				{Type: "image_url", ImageURL: &visionImage{URL: dataURL}},
			},
		}},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.baseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.httpc.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("vision: http %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", nil, err
	}
	var usage *tokenUsage
	if out.Usage != nil {
		usage = &tokenUsage{out.Usage.PromptTokens, out.Usage.CompletionTokens}
	}
	if out.Error != nil {
		return "", usage, fmt.Errorf("vision: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", usage, fmt.Errorf("vision: empty response")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), usage, nil
}
