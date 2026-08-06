// Package llm defines the provider-neutral chat-completion contracts the
// response engine calls through. It has no dependency on any specific
// provider's wire format (OpenRouter, OpenAI, Gemini, ...) — provider adapters
// implementing ChatClient live in backend/internal/llmprovider, and switching
// model or provider is configuration only: it never requires a code change
// here or in the engine.
package llm

import (
	"context"
	"errors"
)

// ErrProviderAuth is the sentinel a ChatClient wraps into its returned error
// when a provider rejects a call as unauthorized (an invalid or revoked API
// key) — as opposed to a transient network error, a rate limit, or a
// malformed request, none of which say anything about whether the
// credential itself still works. Callers use errors.Is(err, ErrProviderAuth)
// to distinguish "this integration needs a new key" from "try again later."
var ErrProviderAuth = errors.New("llm: provider rejected the request as unauthorized")

// Message is one chat turn. The evaluated schema_kb_v1 pipeline sends exactly
// one "user" message containing the whole rendered prompt (no system message,
// no history as separate turns — history is rendered into the prompt text
// itself); callers outside a retry follow that same shape.
type Message struct {
	Role    string
	Content string
}

// ChatRequest is one chat-completion call. Model is a provider-specific model
// id (e.g. "google/gemini-2.5-flash") — the Registry resolves a ChatClient by
// provider only, so the model string always travels with the request.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// ChatResponse is one chat-completion result. PromptTokens/CompletionTokens
// are zero when the provider didn't report usage, not necessarily "no tokens
// used."
type ChatResponse struct {
	Text             string
	FinishReason     string
	PromptTokens     int
	CompletionTokens int
}

// ChatClient performs one chat-completion call against a configured provider.
type ChatClient interface {
	Complete(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// ModelRef names a specific model on a specific provider — the unit the
// Registry resolves a ChatClient by, and what a request or draft is recorded
// against for provenance ("prompt_ref=... provider=... model=...").
type ModelRef struct {
	Provider string
	Model    string
}

// Registry resolves a configured ChatClient by provider. Built once at the
// composition root from environment configuration; resolving an unconfigured
// provider fails explicitly rather than silently falling back to a default or
// a stub client.
type Registry interface {
	Client(ref ModelRef) (ChatClient, error)
}
