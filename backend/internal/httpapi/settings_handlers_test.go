package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

var errBoom = errors.New("boom")

func TestGetSettingsReturnsDefaults(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.get("/xchats/api/v1/settings")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		Version int `json:"version"`
		LLM     struct {
			DefaultProvider string `json:"default_provider"`
		} `json:"llm"`
		SetupCompleted bool `json:"setup_completed"`
	}
	mustDecode(t, env, &got)
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
	if got.LLM.DefaultProvider != "openrouter" {
		t.Errorf("LLM.DefaultProvider = %q, want %q", got.LLM.DefaultProvider, "openrouter")
	}
	if got.SetupCompleted {
		t.Error("SetupCompleted = true on a fresh install, want false")
	}
}

func TestListIntegrationsShowsAllProvidersNoneConfigured(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.get("/xchats/api/v1/settings/integrations")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings/integrations: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		CredentialStoreAvailable bool `json:"credential_store_available"`
		Providers                []struct {
			ID          string `json:"id"`
			Configured  bool   `json:"configured"`
			Validatable bool   `json:"validatable"`
		} `json:"providers"`
	}
	mustDecode(t, env, &got)
	if !got.CredentialStoreAvailable {
		t.Error("CredentialStoreAvailable = false, want true (harness forces a file-backed store)")
	}
	if len(got.Providers) != 5 {
		t.Fatalf("providers = %d, want 5 (openrouter, openai, gemini, ngrok, langfuse)", len(got.Providers))
	}
	seen := map[string]bool{}
	for _, p := range got.Providers {
		seen[p.ID] = true
		if p.Configured {
			t.Errorf("provider %q reports configured=true before anything was saved", p.ID)
		}
	}
	for _, id := range []string{"openrouter", "openai", "gemini", "ngrok", "langfuse"} {
		if !seen[id] {
			t.Errorf("provider %q missing from the list", id)
		}
	}
}

func TestSaveIntegrationCredentialNoValidatorProviderSavesImmediately(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/ngrok/credential", map[string]any{
		"values": map[string]string{"ngrok.authtoken": "ngrok-tok-123"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save ngrok credential: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		Verified bool `json:"verified"`
	}
	mustDecode(t, env, &got)
	if got.Verified {
		t.Error("Verified = true for a provider with no Validate func, want false")
	}

	_, listEnv := h.get("/xchats/api/v1/settings/integrations")
	var list struct {
		Providers []struct {
			ID         string `json:"id"`
			Configured bool   `json:"configured"`
			Source     string `json:"source"`
		} `json:"providers"`
	}
	mustDecode(t, listEnv, &list)
	found := false
	for _, p := range list.Providers {
		if p.ID == "ngrok" {
			found = true
			if !p.Configured {
				t.Error("ngrok not reported as configured after saving its credential")
			}
			if p.Source != "file" {
				t.Errorf("ngrok source = %q, want %q", p.Source, "file")
			}
		}
	}
	if !found {
		t.Fatal("ngrok missing from the providers list")
	}
}

func TestSaveIntegrationCredentialMissingFieldFails(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/langfuse/credential", map[string]any{
		"values": map[string]string{"langfuse.public_key": "pk"}, // secret_key omitted
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("save with a missing field: status=%d, want 400; body=%s", resp.StatusCode, env["message"])
	}
}

func TestSaveIntegrationCredentialUnknownProvider404s(t *testing.T) {
	h := newSettingsHarness(t)
	resp, _ := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/does-not-exist/credential", map[string]any{
		"values": map[string]string{},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDeleteIntegrationCredential(t *testing.T) {
	h := newSettingsHarness(t)
	resp, _ := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/ngrok/credential", map[string]any{
		"values": map[string]string{"ngrok.authtoken": "ngrok-tok-123"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: status=%d", resp.StatusCode)
	}

	resp, env := h.do(http.MethodDelete, "/xchats/api/v1/settings/integrations/ngrok/credential", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete: status=%d body=%s", resp.StatusCode, env["message"])
	}

	_, listEnv := h.get("/xchats/api/v1/settings/integrations")
	var list struct {
		Providers []struct {
			ID         string `json:"id"`
			Configured bool   `json:"configured"`
		} `json:"providers"`
	}
	mustDecode(t, listEnv, &list)
	for _, p := range list.Providers {
		if p.ID == "ngrok" && p.Configured {
			t.Error("ngrok still reports configured=true after delete")
		}
	}
}

func TestDeleteIntegrationCredentialManagedByEnvRefuses(t *testing.T) {
	t.Setenv("NGROK_AUTHTOKEN", "from-the-environment")
	h := newSettingsHarness(t)

	resp, env := h.do(http.MethodDelete, "/xchats/api/v1/settings/integrations/ngrok/credential", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("delete an env-managed credential: status=%d, want 409; body=%s", resp.StatusCode, env["message"])
	}
}

func TestTestIntegrationCredentialNoValidatorProvider(t *testing.T) {
	h := newSettingsHarness(t)
	resp, _ := h.do(http.MethodPost, "/xchats/api/v1/settings/integrations/ngrok/test", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (ngrok has no credential check)", resp.StatusCode)
	}
}

func TestTestIntegrationCredentialNotConfigured(t *testing.T) {
	h := newSettingsHarness(t)
	resp, _ := h.do(http.MethodPost, "/xchats/api/v1/settings/integrations/openrouter/test", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (nothing saved yet)", resp.StatusCode)
	}
}

// withTestProviderBaseURL points a provider's Settings BaseURL at a local
// httptest.Server, so save/test's Validate call hits it instead of the real
// public endpoint (see internal/credentials' own baseURLFor override).
func withTestProviderBaseURL(t *testing.T, h *settingsHarness, providerID string, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/"+providerID, map[string]any{
		"base_url": srv.URL,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set base_url: status=%d body=%s", resp.StatusCode, env["message"])
	}
	return srv
}

func TestSaveIntegrationCredentialValidatesAgainstConfiguredBaseURL(t *testing.T) {
	h := newSettingsHarness(t)
	withTestProviderBaseURL(t, h, "openrouter", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/openrouter/credential", map[string]any{
		"values": map[string]string{"openrouter.api_key": "sk-good"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		Verified bool `json:"verified"`
	}
	mustDecode(t, env, &got)
	if !got.Verified {
		t.Error("Verified = false for a credential the test server accepted, want true")
	}

	_, listEnv := h.get("/xchats/api/v1/settings/integrations")
	var list struct {
		Providers []struct {
			ID             string `json:"id"`
			LastVerifiedAt string `json:"last_verified_at"`
		} `json:"providers"`
	}
	mustDecode(t, listEnv, &list)
	for _, p := range list.Providers {
		if p.ID == "openrouter" && p.LastVerifiedAt == "" {
			t.Error("last_verified_at not set after a successful save")
		}
	}
}

func TestSaveIntegrationCredentialRejectedNeverSaves(t *testing.T) {
	h := newSettingsHarness(t)
	withTestProviderBaseURL(t, h, "openrouter", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/openrouter/credential", map[string]any{
		"values": map[string]string{"openrouter.api_key": "sk-bad"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("save a rejected credential: status=%d, want 422; body=%s", resp.StatusCode, env["message"])
	}

	_, listEnv := h.get("/xchats/api/v1/settings/integrations")
	var list struct {
		Providers []struct {
			ID         string `json:"id"`
			Configured bool   `json:"configured"`
		} `json:"providers"`
	}
	mustDecode(t, listEnv, &list)
	for _, p := range list.Providers {
		if p.ID == "openrouter" && p.Configured {
			t.Error("a REJECTED credential became configured — invalid must never be saved")
		}
	}
}

func TestSaveIntegrationCredentialUnverifiedRequiresForce(t *testing.T) {
	h := newSettingsHarness(t)
	withTestProviderBaseURL(t, h, "openrouter", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // neither 200 nor 401/403/400 — "unavailable"
	})

	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/openrouter/credential", map[string]any{
		"values": map[string]string{"openrouter.api_key": "sk-unknown"},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("save without force when unverifiable: status=%d, want 409; body=%s", resp.StatusCode, env["message"])
	}

	resp, env = h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/openrouter/credential", map[string]any{
		"values": map[string]string{"openrouter.api_key": "sk-unknown"},
		"force":  true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save WITH force when unverifiable: status=%d, want 200; body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		Verified bool `json:"verified"`
	}
	mustDecode(t, env, &got)
	if got.Verified {
		t.Error("Verified = true for a forced, unverified save, want false")
	}
}

func TestUpdateIntegrationSettingsRoundTrips(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/openai", map[string]any{
		"base_url": "https://proxy.example/v1", "default_model": "gpt-4o", "disabled": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: status=%d body=%s", resp.StatusCode, env["message"])
	}

	_, listEnv := h.get("/xchats/api/v1/settings/integrations")
	var list struct {
		Providers []struct {
			ID           string `json:"id"`
			BaseURL      string `json:"base_url"`
			DefaultModel string `json:"default_model"`
			Disabled     bool   `json:"disabled"`
		} `json:"providers"`
	}
	mustDecode(t, listEnv, &list)
	found := false
	for _, p := range list.Providers {
		if p.ID == "openai" {
			found = true
			if p.BaseURL != "https://proxy.example/v1" || p.DefaultModel != "gpt-4o" || !p.Disabled {
				t.Errorf("openai settings = %+v, want the values just PUT", p)
			}
		}
	}
	if !found {
		t.Fatal("openai missing from the providers list")
	}
}

func TestUpdateLLMSettingsValidation(t *testing.T) {
	h := newSettingsHarness(t)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing default_provider", map[string]any{"default_model": "m", "max_tokens": 500, "temperature": 0.3, "timeout_seconds": 60}},
		{"unknown provider", map[string]any{"default_provider": "not-a-provider", "default_model": "m", "max_tokens": 500, "temperature": 0.3, "timeout_seconds": 60}},
		{"non-positive max_tokens", map[string]any{"default_provider": "openrouter", "default_model": "m", "max_tokens": 0, "temperature": 0.3, "timeout_seconds": 60}},
		{"temperature out of range", map[string]any{"default_provider": "openrouter", "default_model": "m", "max_tokens": 500, "temperature": 3.0, "timeout_seconds": 60}},
		{"non-positive timeout", map[string]any{"default_provider": "openrouter", "default_model": "m", "max_tokens": 500, "temperature": 0.3, "timeout_seconds": 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/llm", tc.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body=%s", resp.StatusCode, env["message"])
			}
		})
	}

	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/llm", map[string]any{
		"default_provider": "openai", "default_model": "gpt-4o", "vision_model": "gpt-4o",
		"max_tokens": 800, "temperature": 0.5, "timeout_seconds": 45, "retry": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid update: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		DefaultProvider string `json:"default_provider"`
		MaxTokens       int    `json:"max_tokens"`
	}
	mustDecode(t, env, &got)
	if got.DefaultProvider != "openai" || got.MaxTokens != 800 {
		t.Errorf("got = %+v, want DefaultProvider=openai MaxTokens=800", got)
	}

	_, settingsEnv := h.get("/xchats/api/v1/settings")
	var full struct {
		LLM struct {
			DefaultProvider string `json:"default_provider"`
		} `json:"llm"`
	}
	mustDecode(t, settingsEnv, &full)
	if full.LLM.DefaultProvider != "openai" {
		t.Errorf("GET /settings after PUT /settings/llm = %q, want %q", full.LLM.DefaultProvider, "openai")
	}
}

func TestUpdateCredentialStorage(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/credential-storage", map[string]any{
		"credential_file_fallback_accepted": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	_, settingsEnv := h.get("/xchats/api/v1/settings")
	var full struct {
		CredentialFileFallbackAccepted bool `json:"credential_file_fallback_accepted"`
	}
	mustDecode(t, settingsEnv, &full)
	if !full.CredentialFileFallbackAccepted {
		t.Error("CredentialFileFallbackAccepted = false after PUT true")
	}
}

func TestSetupComplete(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.do(http.MethodPost, "/xchats/api/v1/settings/setup-complete", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	_, settingsEnv := h.get("/xchats/api/v1/settings")
	var full struct {
		SetupCompleted bool `json:"setup_completed"`
	}
	mustDecode(t, settingsEnv, &full)
	if !full.SetupCompleted {
		t.Error("SetupCompleted = false after POST /settings/setup-complete")
	}
}

func TestTunnelStatusStartStop(t *testing.T) {
	h := newSettingsHarness(t)

	_, env := h.get("/xchats/api/v1/settings/tunnel")
	var status struct {
		Running bool `json:"running"`
	}
	mustDecode(t, env, &status)
	if status.Running {
		t.Error("Running = true before Start, want false")
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/settings/tunnel/start", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var started struct {
		Running   bool   `json:"running"`
		PublicURL string `json:"public_url"`
	}
	mustDecode(t, env, &started)
	if !started.Running || started.PublicURL == "" {
		t.Errorf("status after start = %+v, want Running=true and a PublicURL", started)
	}
	if h.tun.starts != 1 {
		t.Errorf("fake tunnel Start called %d times, want 1", h.tun.starts)
	}

	resp, env = h.do(http.MethodPost, "/xchats/api/v1/settings/tunnel/stop", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var stopped struct {
		Running bool `json:"running"`
	}
	mustDecode(t, env, &stopped)
	if stopped.Running {
		t.Error("Running = true after Stop, want false")
	}
	if h.tun.stops != 1 {
		t.Errorf("fake tunnel Stop called %d times, want 1", h.tun.stops)
	}
}

func TestTunnelStartFailureReturnsStatusWithLastError(t *testing.T) {
	h := newSettingsHarness(t)
	h.tun.startErr = errBoom

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/settings/tunnel/start", nil)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d, want 502; body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		LastError string `json:"last_error"`
	}
	mustDecode(t, env, &got)
	if got.LastError == "" {
		t.Error("LastError is empty in the failure response payload")
	}
}

func TestLLMRefreshCalledOnCredentialSaveDeleteAndSettingsChanges(t *testing.T) {
	h := newSettingsHarness(t)

	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/ngrok/credential", map[string]any{
		"values": map[string]string{"ngrok.authtoken": "tok"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := h.llmRefreshCallCount(); got != 1 {
		t.Errorf("llmRefresh calls after save = %d, want 1", got)
	}

	resp, env = h.do(http.MethodDelete, "/xchats/api/v1/settings/integrations/ngrok/credential", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := h.llmRefreshCallCount(); got != 2 {
		t.Errorf("llmRefresh calls after delete = %d, want 2", got)
	}

	resp, env = h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/openai", map[string]any{
		"base_url": "https://proxy.example",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update provider settings: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := h.llmRefreshCallCount(); got != 3 {
		t.Errorf("llmRefresh calls after provider settings update = %d, want 3", got)
	}

	resp, env = h.do(http.MethodPut, "/xchats/api/v1/settings/llm", map[string]any{
		"default_provider": "openrouter", "default_model": "m",
		"max_tokens": 500, "temperature": 0.3, "timeout_seconds": 60,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update llm settings: status=%d body=%s", resp.StatusCode, env["message"])
	}
	if got := h.llmRefreshCallCount(); got != 4 {
		t.Errorf("llmRefresh calls after LLM settings update = %d, want 4", got)
	}
}

func TestSettingsRoutesRequireAdmin(t *testing.T) {
	h := newSettingsHarness(t)
	member := h.createMemberClient("member@xchats.test", "password123")

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/xchats/api/v1/settings"},
		{http.MethodGet, "/xchats/api/v1/settings/integrations"},
		{http.MethodPut, "/xchats/api/v1/settings/llm"},
		{http.MethodGet, "/xchats/api/v1/settings/tunnel"},
		{http.MethodPost, "/xchats/api/v1/settings/tunnel/start"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, h.srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := member.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.method, tc.path, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status = %d, want 403", resp.StatusCode)
			}
		})
	}
}
