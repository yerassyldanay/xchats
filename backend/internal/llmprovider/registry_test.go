package llmprovider

import (
	"sync"
	"testing"

	"github.com/yerassyldanay/xchats/backend/llm"
)

func TestRegistry_UnknownProviderFailsExplicitly(t *testing.T) {
	reg := NewRegistry()
	if _, err := reg.Client(llm.ModelRef{Provider: "does-not-exist"}); err == nil {
		t.Fatal("want an explicit error for an unregistered provider")
	}
}

func TestRegistry_RegisterThenClient(t *testing.T) {
	reg := NewRegistry()
	client := NewOpenAICompatible("https://example.test", "key", "openrouter", 0)
	reg.Register("openrouter", client)

	got, err := reg.Client(llm.ModelRef{Provider: "openrouter"})
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if got != client {
		t.Error("Client returned a different instance than what was Registered")
	}
}

func TestRegistry_RegisterOverwritesPriorClient(t *testing.T) {
	reg := NewRegistry()
	first := NewOpenAICompatible("https://a.test", "key-a", "openrouter", 0)
	second := NewOpenAICompatible("https://b.test", "key-b", "openrouter", 0)
	reg.Register("openrouter", first)
	reg.Register("openrouter", second)

	got, err := reg.Client(llm.ModelRef{Provider: "openrouter"})
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if got != second {
		t.Error("Client returned the FIRST registered instance, want the second (most recent) one")
	}
}

func TestRegistry_Deregister(t *testing.T) {
	reg := NewRegistry()
	reg.Register("openrouter", NewOpenAICompatible("https://example.test", "key", "openrouter", 0))
	reg.Deregister("openrouter")

	if _, err := reg.Client(llm.ModelRef{Provider: "openrouter"}); err == nil {
		t.Fatal("want an explicit error after Deregister, got nil")
	}
}

func TestRegistry_DeregisterUnknownProviderIsANoOp(t *testing.T) {
	reg := NewRegistry()
	reg.Deregister("does-not-exist") // must not panic
}

func TestRegistry_ConcurrentAccessIsSafe(t *testing.T) {
	reg := NewRegistry()
	reg.Register("openrouter", NewOpenAICompatible("https://example.test", "key", "openrouter", 0))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	// Readers, hammering Client — the response engine's own hot path.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = reg.Client(llm.ModelRef{Provider: "openrouter"})
				}
			}
		}()
	}
	// Writers, hammering Register/Deregister — an LLMRefresh call.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					reg.Register("openrouter", NewOpenAICompatible("https://example.test", "key", "openrouter", 0))
					reg.Deregister("gemini") // a provider that was never registered — exercises the no-op path too
				}
			}
		}()
	}
	close(stop)
	wg.Wait()
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
