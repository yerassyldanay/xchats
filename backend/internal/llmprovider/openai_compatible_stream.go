package llmprovider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// The streamed half of the OpenAI chat/completions wire format: the same
// endpoint and request body as complete (openai_compatible.go), plus
// "stream": true, answered as an SSE stream of `data: {...}` lines carrying
// choices[0].delta.content instead of one JSON document carrying
// choices[0].message.content. OpenRouter, OpenAI, and Gemini's
// OpenAI-compatible endpoint all speak this identically.

// maxStreamLine bounds one SSE line. A delta is a few tokens; anything near
// this is a malformed or hostile response, not a real chunk.
const maxStreamLine = 1 << 20 // 1 MiB

type wireStreamRequest struct {
	wireRequest
	Stream bool `json:"stream"`
}

type wireStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
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

// Stream implements llm.StreamClient. Like Complete it applies its own
// timeout (c.timeout) to the whole call — which for a stream bounds the
// entire generation, not just time-to-first-token, so a deployment that
// wants long answers raises the same LLM_DRAFT_TIMEOUT_SECONDS knob every
// other call already uses.
func (c *OpenAICompatible) Stream(ctx context.Context, req llm.ChatRequest, onDelta func(string)) (llm.ChatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	wreq := wireStreamRequest{
		wireRequest: wireRequest{
			Model:       req.Model,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			Messages:    toWireMessages(req.Messages),
		},
		Stream: true,
	}

	ctx, gen := startGeneration(ctx, genParams{
		name: "llm.stream", provider: c.provider, model: req.Model,
		temperature: req.Temperature, maxTokens: req.MaxTokens,
		input: lastContent(req.Messages),
	})
	resp, err := c.stream(ctx, wreq, onDelta)
	gen.setUsage(resp.PromptTokens, resp.CompletionTokens)
	gen.end(resp.Text, err)
	return resp, err
}

func (c *OpenAICompatible) stream(ctx context.Context, wreq wireStreamRequest, onDelta func(string)) (llm.ChatResponse, error) {
	body, err := json.Marshal(wreq)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return llm.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpc.Do(httpReq)
	if err != nil {
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// An error response is a normal JSON document, not a stream — read
		// it whole so the caller sees the provider's own message rather
		// than "unexpected end of stream".
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return llm.ChatResponse{}, fmt.Errorf("llmprovider: http %d: %w: %s", resp.StatusCode, llm.ErrProviderAuth, string(errBody))
		}
		return llm.ChatResponse{}, fmt.Errorf("llmprovider: http %d: %s", resp.StatusCode, string(errBody))
	}
	return readStream(resp.Body, onDelta)
}

// readStream consumes an OpenAI-style SSE body, calling onDelta for every
// non-empty content delta and accumulating the same ChatResponse a
// non-streamed call would have produced. Split out from stream so the wire
// parsing is testable without an HTTP server.
func readStream(r io.Reader, onDelta func(string)) (llm.ChatResponse, error) {
	var out llm.ChatResponse
	var text strings.Builder

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 8<<10), maxStreamLine)
	for sc.Scan() {
		// SSE frames are separated by blank lines and may carry fields
		// other than data (comments, event:, id:) — everything but a data
		// payload is noise to this parser.
		payload, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			continue
		}
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk wireStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return out, fmt.Errorf("llmprovider: unparseable stream chunk: %w (%s)", err, payload)
		}
		if chunk.Error != nil {
			// A provider can answer 200 and then fail mid-stream — surfaced
			// as an error rather than a silently truncated answer.
			return out, fmt.Errorf("llmprovider: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			// Usage arrives on its own final chunk on the providers that
			// report it at all; zero stays "not reported" (see
			// llm.ChatResponse's own doc comment).
			out.PromptTokens = chunk.Usage.PromptTokens
			out.CompletionTokens = chunk.Usage.CompletionTokens
		}
		for _, ch := range chunk.Choices {
			if ch.FinishReason != "" {
				out.FinishReason = ch.FinishReason
			}
			if ch.Delta.Content == "" {
				continue
			}
			text.WriteString(ch.Delta.Content)
			if onDelta != nil {
				onDelta(ch.Delta.Content)
			}
		}
	}
	if err := sc.Err(); err != nil {
		// Whatever arrived before the failure is still returned alongside
		// it: the caller has already handed those deltas to the user, so a
		// partial answer it can persist beats discarding them.
		out.Text = text.String()
		if errors.Is(err, bufio.ErrTooLong) {
			return out, fmt.Errorf("llmprovider: stream line exceeds %d bytes", maxStreamLine)
		}
		return out, fmt.Errorf("llmprovider: read stream: %w", err)
	}
	out.Text = text.String()
	return out, nil
}
