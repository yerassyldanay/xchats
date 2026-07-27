package llmprovider

import (
	"fmt"
	"strings"
	"time"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// Registry resolves a configured llm.ChatClient by provider name. It
// implements llm.Registry.
type Registry struct {
	clients map[string]llm.ChatClient
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]llm.ChatClient)}
}

// Register associates a client with a provider name, overwriting any prior one.
func (r *Registry) Register(provider string, client llm.ChatClient) {
	r.clients[provider] = client
}

// Client implements llm.Registry: it resolves by ref.Provider only — the model
// string travels with each request, not with the client.
func (r *Registry) Client(ref llm.ModelRef) (llm.ChatClient, error) {
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

// ProviderConfig is one provider's connection settings, resolved from
// environment configuration at the composition root.
type ProviderConfig struct {
	Name    string // "openrouter" | "openai" | "gemini"
	APIKey  string
	BaseURL string // empty means use DefaultBaseURL(Name)
}

// BuildRegistry constructs a Registry with one OpenAICompatible client per
// provider that has a non-blank API key — a provider with no key configured is
// simply not registered (Client then fails explicitly for it, per llm.Registry's
// contract), never given a stub fallback.
func BuildRegistry(providers []ProviderConfig, timeout time.Duration) *Registry {
	reg := NewRegistry()
	for _, p := range providers {
		if strings.TrimSpace(p.APIKey) == "" {
			continue
		}
		baseURL := p.BaseURL
		if baseURL == "" {
			baseURL = DefaultBaseURL(p.Name)
		}
		reg.Register(p.Name, NewOpenAICompatible(baseURL, p.APIKey, p.Name, timeout))
	}
	return reg
}
