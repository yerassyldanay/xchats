package llmprovider

import (
	"fmt"
	"strings"
	"sync"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// Registry resolves a configured llm.ChatClient by provider name. It
// implements llm.Registry. Safe for concurrent use: Client (the response
// engine's hot path, called on every Generate) can run alongside
// Register/Deregister (an httpapi settings.go LLMRefresh call, triggered by
// a credential save/delete on a completely different goroutine) without
// external synchronization.
type Registry struct {
	mu      sync.RWMutex
	clients map[string]llm.ChatClient
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]llm.ChatClient)}
}

// Register associates a client with a provider name, overwriting any prior one.
func (r *Registry) Register(provider string, client llm.ChatClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[provider] = client
}

// Deregister removes a provider's client, if any — a no-op if none was
// registered. Called when a credential is deleted (or found to no longer
// resolve to a key), so Client fails explicitly for that provider instead
// of continuing to serve requests through a client built from a key that
// may no longer be valid.
func (r *Registry) Deregister(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, provider)
}

// Client implements llm.Registry: it resolves by ref.Provider only — the model
// string travels with each request, not with the client.
func (r *Registry) Client(ref llm.ModelRef) (llm.ChatClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clients[ref.Provider]
	if !ok {
		return nil, fmt.Errorf("llmprovider: no client configured for provider %q", ref.Provider)
	}
	return c, nil
}

// DefaultBaseURL returns the standard OpenAI-compatible base URL for a named
// provider (openrouter is the default for any unrecognized name).
func DefaultBaseURL(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "https://api.openai.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		return "https://openrouter.ai/api/v1"
	}
}
