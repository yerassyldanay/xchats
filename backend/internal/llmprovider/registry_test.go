package llmprovider

import (
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/llm"
)

func TestBuildRegistry_SkipsProvidersWithNoKey(t *testing.T) {
	reg := BuildRegistry([]ProviderConfig{
		{Name: "openrouter", APIKey: "key-1"},
		{Name: "openai", APIKey: ""},
	}, time.Second)

	if _, err := reg.Client(llm.ModelRef{Provider: "openrouter"}); err != nil {
		t.Fatalf("expected openrouter to be registered: %v", err)
	}
	if _, err := reg.Client(llm.ModelRef{Provider: "openai"}); err == nil {
		t.Fatal("want an explicit error for a provider with no configured key, got nil")
	}
}

func TestRegistry_UnknownProviderFailsExplicitly(t *testing.T) {
	reg := NewRegistry()
	if _, err := reg.Client(llm.ModelRef{Provider: "does-not-exist"}); err == nil {
		t.Fatal("want an explicit error for an unregistered provider")
	}
}

func TestDefaultBaseURL(t *testing.T) {
	cases := map[string]string{
		"openai":       "https://api.openai.com/v1",
		"gemini":       "https://generativelanguage.googleapis.com/v1beta/openai",
		"openrouter":   "https://openrouter.ai/api/v1",
		"unrecognized": "https://openrouter.ai/api/v1",
	}
	for provider, want := range cases {
		if got := DefaultBaseURL(provider); got != want {
			t.Errorf("DefaultBaseURL(%q) = %q, want %q", provider, got, want)
		}
	}
}
