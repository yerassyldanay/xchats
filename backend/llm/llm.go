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
//
// Parts, when non-empty, OVERRIDES Content: it is how a multimodal call (an
// image attachment routed to a configured vision model — see
// response.Engine.Generate) attaches one or more images alongside the
// rendered prompt text. Every existing caller leaves Parts nil and sets
// Content alone, which a ChatClient must keep treating exactly as before —
// Parts is strictly additive to the contract.
type Message struct {
	Role    string
	Content string
	Parts   []ContentPart
}

// ContentPartKind discriminates one piece of a multimodal message.
type ContentPartKind string

const (
	PartText  ContentPartKind = "text"
	PartImage ContentPartKind = "image"
)

// ContentPart is one piece of a multimodal Message. A PartText carries Text;
// a PartImage carries ImageURL — a data URI ("data:image/jpeg;base64,...")
// in every caller today, since xchats resolves an attachment's bytes from
// its own blob storage rather than handing a provider a fetchable public
// URL (see responsestore.ConversationRepo). The OpenAI-compatible wire
// format's image_url.url field accepts either shape, so a provider adapter
// may pass ImageURL through unchanged.
type ContentPart struct {
	Kind     ContentPartKind
	Text     string
	ImageURL string
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

// StreamClient is a ChatClient that can also deliver a completion
// incrementally, token by token, as the provider produces it. Deliberately a
// SEPARATE interface rather than a method on ChatClient: every existing
// caller (the response engine, the KB import pipeline) wants the whole
// answer or nothing, and a provider adapter that cannot stream must stay a
// valid ChatClient. A caller that wants streaming type-asserts for this and
// falls back to Complete when the assertion fails.
type StreamClient interface {
	ChatClient
	// Stream performs one chat-completion call, invoking onDelta with each
	// incremental piece of text as it arrives. It returns the SAME
	// ChatResponse Complete would have returned for the request — Text is
	// the full concatenation of every delta, so a caller never has to
	// accumulate the pieces itself to persist the result.
	//
	// onDelta runs on the goroutine that called Stream and must not block;
	// cancelling ctx is the way to stop a stream early.
	Stream(ctx context.Context, req ChatRequest, onDelta func(string)) (ChatResponse, error)
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
