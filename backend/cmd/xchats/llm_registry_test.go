package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/llmprovider"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// okChatResponse is the minimal valid OpenAI-compatible chat-completion
// body llmprovider.OpenAICompatible needs to parse a Complete call
// successfully — mirrors internal/llmprovider/openai_compatible_test.go's
// own fixture.
const okChatResponse = `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`

func TestPopulateLLMRegistryUsesConfigKeyWhenNoCreds(t *testing.T) {
	reg := llmprovider.NewRegistry()
	cfg := &config.Config{OpenRouterAPIKey: "sk-from-config"}
	settingsStore := settings.NewStore(t.TempDir())

	populateLLMRegistry(context.Background(), reg, nil, settingsStore, cfg)

	if _, err := reg.Client(llm.ModelRef{Provider: "openrouter"}); err != nil {
		t.Errorf("openrouter not registered from cfg's key: %v", err)
	}
	if _, err := reg.Client(llm.ModelRef{Provider: "openai"}); err == nil {
		t.Error("openai registered despite no key anywhere")
	}
}

func TestPopulateLLMRegistryPrefersCredsOverConfig(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, okChatResponse)
	}))
	defer srv.Close()

	reg := llmprovider.NewRegistry()
	cfg := &config.Config{OpenRouterAPIKey: "sk-from-config", OpenRouterBaseURL: srv.URL}
	settingsStore := settings.NewStore(t.TempDir())
	creds, err := credentials.Open(credentials.OpenOptions{AllowFile: true, ForceFile: true, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("credentials.Open: %v", err)
	}
	if err := creds.Set(context.Background(), "openrouter.api_key", "sk-from-creds"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	populateLLMRegistry(context.Background(), reg, creds, settingsStore, cfg)
	client, err := reg.Client(llm.ModelRef{Provider: "openrouter"})
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if _, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "m", Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotAuth != "Bearer sk-from-creds" {
		t.Errorf("Authorization header = %q, want the CREDS key (sk-from-creds), not cfg's", gotAuth)
	}
}

func TestPopulateLLMRegistryDeregistersWhenCredentialDeleted(t *testing.T) {
	reg := llmprovider.NewRegistry()
	cfg := &config.Config{} // no cfg fallback key at all
	settingsStore := settings.NewStore(t.TempDir())
	creds, err := credentials.Open(credentials.OpenOptions{AllowFile: true, ForceFile: true, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("credentials.Open: %v", err)
	}
	if err := creds.Set(context.Background(), "openrouter.api_key", "sk-temp"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	populateLLMRegistry(context.Background(), reg, creds, settingsStore, cfg)
	if _, err := reg.Client(llm.ModelRef{Provider: "openrouter"}); err != nil {
		t.Fatalf("openrouter not registered before delete: %v", err)
	}

	if err := creds.Delete(context.Background(), "openrouter.api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	populateLLMRegistry(context.Background(), reg, creds, settingsStore, cfg)

	if _, err := reg.Client(llm.ModelRef{Provider: "openrouter"}); err == nil {
		t.Error("openrouter still registered after its credential was deleted and the registry repopulated")
	}
}

func TestPopulateLLMRegistryUsesSettingsBaseURLOverride(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, okChatResponse)
	}))
	defer srv.Close()

	reg := llmprovider.NewRegistry()
	// cfg's own base URL deliberately points somewhere that would fail if
	// ever actually dialed — proving Settings' override, not cfg's, wins.
	cfg := &config.Config{OpenRouterAPIKey: "sk-test", OpenRouterBaseURL: "http://127.0.0.1:1"}
	settingsStore := settings.NewStore(t.TempDir())
	if _, err := settingsStore.Update(func(st *settings.Settings) {
		st.Providers["openrouter"] = settings.ProviderSettings{BaseURL: srv.URL}
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	populateLLMRegistry(context.Background(), reg, nil, settingsStore, cfg)
	client, err := reg.Client(llm.ModelRef{Provider: "openrouter"})
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if _, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "m", Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}); err != nil {
		t.Fatalf("Complete: %v (want it to reach the Settings-overridden base URL)", err)
	}
	if !hit {
		t.Error("the Settings-overridden base URL's server was never reached")
	}
}

func TestResolveLLMParamsFromSettings(t *testing.T) {
	cfg := &config.Config{}
	settingsStore := settings.NewStore(t.TempDir())
	if _, err := settingsStore.Update(func(st *settings.Settings) {
		st.LLM = settings.LLMSettings{
			DefaultProvider: "openai", DefaultModel: "gpt-4o",
			MaxTokens: 900, Temperature: 0.6, Retry: false, TimeoutSeconds: 30,
		}
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := resolveLLMParams(settingsStore, cfg)
	want := llm.ModelRef{Provider: "openai", Model: "gpt-4o"}
	if got.DefaultModel != want {
		t.Errorf("DefaultModel = %+v, want %+v", got.DefaultModel, want)
	}
	if got.MaxTokens != 900 || got.Temperature != 0.6 || got.RetryEnabled {
		t.Errorf("got = %+v, want MaxTokens=900 Temperature=0.6 RetryEnabled=false", got)
	}
}

func TestResolveLLMParamsFallsBackToConfigOnLoadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	settingsStore := settings.NewStore(dir)
	cfg := &config.Config{
		LLMDefaultProvider: "gemini", LLMDefaultModel: "gemini-2.5-flash",
		LLMDraftMaxTokens: 400, LLMDraftTemperature: 0.1, LLMDraftRetry: true,
	}

	got := resolveLLMParams(settingsStore, cfg)
	want := llm.ModelRef{Provider: "gemini", Model: "gemini-2.5-flash"}
	if got.DefaultModel != want {
		t.Errorf("DefaultModel = %+v, want %+v (the cfg fallback)", got.DefaultModel, want)
	}
	if got.MaxTokens != 400 || got.Temperature != 0.1 || !got.RetryEnabled {
		t.Errorf("got = %+v, want the cfg fallback values", got)
	}
}

func TestResolveLLMParamsFallbackDefaultsWhenConfigAlsoEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := resolveLLMParams(settings.NewStore(dir), &config.Config{})
	if got.DefaultModel.Provider != "openrouter" || got.DefaultModel.Model != "google/gemini-2.5-flash" {
		t.Errorf("DefaultModel = %+v, want the hardcoded openrouter/gemini-2.5-flash fallback", got.DefaultModel)
	}
}
