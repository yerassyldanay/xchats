package httpapi_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/llm"
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
	if len(got.Providers) != 7 {
		t.Fatalf("providers = %d, want 7 (openrouter, openai, gemini, ngrok, langfuse, firecrawl, llamaparse)", len(got.Providers))
	}
	seen := map[string]bool{}
	for _, p := range got.Providers {
		seen[p.ID] = true
		if p.Configured {
			t.Errorf("provider %q reports configured=true before anything was saved", p.ID)
		}
	}
	for _, id := range []string{"openrouter", "openai", "gemini", "ngrok", "langfuse", "firecrawl", "llamaparse"} {
		if !seen[id] {
			t.Errorf("provider %q missing from the list", id)
		}
	}
}

func TestSaveNgrokCredentialsValidatesAndSaves(t *testing.T) {
	h := newSettingsHarness(t)
	withTestProviderBaseURL(t, h, "ngrok", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ngrok-api-123" || r.Header.Get("ngrok-version") != "2" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/ngrok/credential", map[string]any{
		"values": map[string]string{"ngrok.authtoken": "ngrok-tok-123", "ngrok.api_key": "ngrok-api-123"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save ngrok credential: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		Verified bool `json:"verified"`
	}
	mustDecode(t, env, &got)
	if !got.Verified {
		t.Error("Verified = false for a valid ngrok API key, want true")
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
	withTestProviderBaseURL(t, h, "ngrok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	resp, _ := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/ngrok/credential", map[string]any{
		"values": map[string]string{"ngrok.authtoken": "ngrok-tok-123", "ngrok.api_key": "ngrok-api-123"},
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

func TestTestNgrokIntegrationRequiresConfiguredCredentials(t *testing.T) {
	h := newSettingsHarness(t)
	resp, _ := h.do(http.MethodPost, "/xchats/api/v1/settings/integrations/ngrok/test", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (ngrok credentials are not configured)", resp.StatusCode)
	}
}

func TestSaveNgrokAPIKeyPreservesStoredAuthtoken(t *testing.T) {
	h := newSettingsHarness(t)
	withTestProviderBaseURL(t, h, "ngrok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	if err := h.creds.Set(context.Background(), credentials.KeyNgrokAuthtoken, "existing-authtoken"); err != nil {
		t.Fatalf("seed authtoken: %v", err)
	}

	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/ngrok/credential", map[string]any{
		"values": map[string]string{"ngrok.api_key": "new-api-key"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save API key: status=%d body=%s", resp.StatusCode, env["message"])
	}
	got, err := h.creds.Get(context.Background(), credentials.KeyNgrokAuthtoken)
	if err != nil || got != "existing-authtoken" {
		t.Fatalf("authtoken after partial save = %q, %v", got, err)
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

func TestUpdateNgrokSettingsRoundTrips(t *testing.T) {
	h := newSettingsHarness(t)
	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/ngrok", map[string]any{
		"region": "eu", "domain": "chat.example.com",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got struct {
		Region string `json:"region"`
		Domain string `json:"domain"`
	}
	mustDecode(t, env, &got)
	if got.Region != "eu" || got.Domain != "chat.example.com" {
		t.Errorf("got = %+v, want Region=eu Domain=chat.example.com", got)
	}

	_, settingsEnv := h.get("/xchats/api/v1/settings")
	var full struct {
		Ngrok struct {
			Region string `json:"region"`
			Domain string `json:"domain"`
		} `json:"ngrok"`
	}
	mustDecode(t, settingsEnv, &full)
	if full.Ngrok.Region != "eu" || full.Ngrok.Domain != "chat.example.com" {
		t.Errorf("GET /settings after PUT /settings/ngrok = %+v, want Region=eu Domain=chat.example.com", full.Ngrok)
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

func TestDownloadBackup(t *testing.T) {
	h := newSettingsHarness(t)

	resp, err := h.client.Get(h.srv.URL + "/xchats/api/v1/settings/backup/download")
	if err != nil {
		t.Fatalf("GET backup/download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".zip") {
		t.Errorf("Content-Disposition = %q, want an attachment ending in .zip", cd)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("body is not a valid zip: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["manifest.json"] {
		t.Error("zip missing manifest.json")
	}
	if !names["xchats.db"] {
		t.Error("zip missing xchats.db")
	}
}

func TestProviderHealth_EmptyWhenNothingReportedYet(t *testing.T) {
	h := newSettingsHarness(t)
	_, env := h.get("/xchats/api/v1/settings/provider-health")
	var got struct {
		Providers []struct {
			Provider string `json:"provider"`
			Healthy  bool   `json:"healthy"`
		} `json:"providers"`
	}
	mustDecode(t, env, &got)
	if len(got.Providers) != 0 {
		t.Errorf("providers = %+v, want none before anything is reported", got.Providers)
	}
}

func TestProviderHealth_ReflectsTrackerState(t *testing.T) {
	h := newSettingsHarness(t)
	h.health.Report("openrouter", nil)
	h.health.Report("openai", fmt.Errorf("wrapped: %w", llm.ErrProviderAuth))

	_, env := h.get("/xchats/api/v1/settings/provider-health")
	var got struct {
		Providers []struct {
			Provider string `json:"provider"`
			Healthy  bool   `json:"healthy"`
		} `json:"providers"`
	}
	mustDecode(t, env, &got)
	byProvider := map[string]bool{}
	for _, p := range got.Providers {
		byProvider[p.Provider] = p.Healthy
	}
	if len(got.Providers) != 2 {
		t.Fatalf("providers = %+v, want 2 entries", got.Providers)
	}
	if !byProvider["openrouter"] {
		t.Error("openrouter should report healthy")
	}
	if byProvider["openai"] {
		t.Error("openai should report unhealthy")
	}
}

func TestUpdateCheck_NilCheckerFallsBackGracefully(t *testing.T) {
	h := newSettingsHarness(t)
	_, env := h.get("/xchats/api/v1/settings/update-check")
	var got struct {
		CurrentVersion  string `json:"current_version"`
		UpdateAvailable bool   `json:"update_available"`
	}
	mustDecode(t, env, &got)
	if got.CurrentVersion == "" {
		t.Error("current_version is empty")
	}
	if got.UpdateAvailable {
		t.Error("update_available = true, want false with no checker wired")
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

// mcpConnectionInfoPayload mirrors httpapi's unexported mcpConnectionInfo —
// GET /mcp-connection lives outside the /settings/* admin group on purpose
// (see mcp_info.go), so these tests share this harness for its fake tunnel
// rather than living in mcp_integration_test.go's heavier MCP-auth harness.
type mcpConnectionInfoPayload struct {
	MCPURL          string   `json:"mcp_url"`
	AuthEnabled     bool     `json:"auth_enabled"`
	TunnelAvailable bool     `json:"tunnel_available"`
	TunnelRunning   bool     `json:"tunnel_running"`
	PublicURL       string   `json:"public_url"`
	Scopes          []string `json:"scopes"`
}

func TestMCPConnectionInfoAvailableToMemberBeforeTunnelStarts(t *testing.T) {
	h := newSettingsHarness(t)

	resp, env := h.get("/xchats/api/v1/mcp-connection")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin GET: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got mcpConnectionInfoPayload
	mustDecode(t, env, &got)
	if !got.TunnelAvailable {
		t.Error("TunnelAvailable = false, want true (fake tunnel is wired)")
	}
	if got.TunnelRunning {
		t.Error("TunnelRunning = true before Start, want false")
	}
	if !strings.HasSuffix(got.MCPURL, "/mcp") {
		t.Errorf("MCPURL = %q, want a /mcp suffix even before the tunnel starts", got.MCPURL)
	}
	if got.PublicURL != "" {
		t.Errorf("PublicURL = %q, want empty before Start", got.PublicURL)
	}

	// The whole point of this route living outside the admin-only /settings
	// group: a member (not just an admin) can read it too.
	member := h.createMemberClient("member@xchats.test", "password123")
	resp, err := member.Get(h.srv.URL + "/xchats/api/v1/mcp-connection")
	if err != nil {
		t.Fatalf("member GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("member GET: status=%d, want 200", resp.StatusCode)
	}
}

func TestMCPConnectionInfoReflectsRunningTunnel(t *testing.T) {
	h := newSettingsHarness(t)

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/settings/tunnel/start", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start tunnel: status=%d body=%s", resp.StatusCode, env["message"])
	}

	resp, env = h.get("/xchats/api/v1/mcp-connection")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got mcpConnectionInfoPayload
	mustDecode(t, env, &got)
	if !got.TunnelRunning {
		t.Error("TunnelRunning = false after Start, want true")
	}
	want := "https://fake.ngrok.app/mcp"
	if got.MCPURL != want {
		t.Errorf("MCPURL = %q, want %q", got.MCPURL, want)
	}
	if got.PublicURL != "https://fake.ngrok.app" {
		t.Errorf("PublicURL = %q, want the fake tunnel's public URL", got.PublicURL)
	}
}

func TestMCPConnectionInfoWithoutTunnelFeature(t *testing.T) {
	h := newNilDepsSettingsHarness(t)

	resp, env := h.get("/xchats/api/v1/mcp-connection")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	var got mcpConnectionInfoPayload
	mustDecode(t, env, &got)
	if got.TunnelAvailable {
		t.Error("TunnelAvailable = true, want false (no tunnel wired in this harness)")
	}
	if got.MCPURL == "" {
		t.Error("MCPURL is empty, want the localhost-derived fallback even with no tunnel")
	}
}

func TestLLMRefreshCalledOnCredentialSaveDeleteAndSettingsChanges(t *testing.T) {
	h := newSettingsHarness(t)
	withTestProviderBaseURL(t, h, "ngrok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	*h.llmRefreshCalls = 0 // exclude the test-only base URL setup above

	resp, env := h.do(http.MethodPut, "/xchats/api/v1/settings/integrations/ngrok/credential", map[string]any{
		"values": map[string]string{"ngrok.authtoken": "tok", "ngrok.api_key": "api-key"},
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
		{http.MethodPut, "/xchats/api/v1/settings/ngrok"},
		{http.MethodGet, "/xchats/api/v1/settings/backup/download"},
		{http.MethodGet, "/xchats/api/v1/settings/provider-health"},
		{http.MethodGet, "/xchats/api/v1/settings/update-check"},
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

// TestSettingsRoutesDegradeGracefullyWithNoOptionalDepsWired exercises every
// settings route against newNilDepsSettingsHarness, where Credentials/
// Settings/Tunnel are all unset — the deployment shape a minimal build (or a
// misconfigured one) produces. Every handler's own doc comment promises a
// clear 503 rather than a panic when its dependency is missing; this is what
// actually proves that over real HTTP instead of trusting the comment.
// "openrouter" stands in for :provider throughout because it has a
// non-nil Validate — the one case (test credential) where a nil Validate
// would short-circuit to 400 before ever reaching the nil-dependency check
// this test is trying to exercise. backup/download is deliberately absent:
// s.store is never nil in this harness, since the store IS how login
// authenticates, so that particular nil branch can't be reached this way.
func TestSettingsRoutesDegradeGracefullyWithNoOptionalDepsWired(t *testing.T) {
	h := newNilDepsSettingsHarness(t)

	cases := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{"get settings", http.MethodGet, "/xchats/api/v1/settings", nil, http.StatusServiceUnavailable},
		{"list integrations", http.MethodGet, "/xchats/api/v1/settings/integrations", nil, http.StatusOK},
		{"update integration settings", http.MethodPut, "/xchats/api/v1/settings/integrations/openrouter", map[string]any{"base_url": "x"}, http.StatusServiceUnavailable},
		{"save credential", http.MethodPut, "/xchats/api/v1/settings/integrations/openrouter/credential", map[string]any{"values": map[string]string{"openrouter.api_key": "x"}}, http.StatusServiceUnavailable},
		{"delete credential", http.MethodDelete, "/xchats/api/v1/settings/integrations/openrouter/credential", nil, http.StatusServiceUnavailable},
		{"test credential", http.MethodPost, "/xchats/api/v1/settings/integrations/openrouter/test", nil, http.StatusServiceUnavailable},
		{"update llm settings", http.MethodPut, "/xchats/api/v1/settings/llm", map[string]any{"default_provider": "openrouter", "default_model": "m", "max_tokens": 500, "temperature": 0.3, "timeout_seconds": 60}, http.StatusServiceUnavailable},
		{"update credential storage", http.MethodPut, "/xchats/api/v1/settings/credential-storage", map[string]any{"credential_file_fallback_accepted": true}, http.StatusServiceUnavailable},
		{"setup complete", http.MethodPost, "/xchats/api/v1/settings/setup-complete", nil, http.StatusServiceUnavailable},
		{"update ngrok settings", http.MethodPut, "/xchats/api/v1/settings/ngrok", map[string]any{"region": "eu"}, http.StatusServiceUnavailable},
		{"provider health", http.MethodGet, "/xchats/api/v1/settings/provider-health", nil, http.StatusOK},
		{"update check", http.MethodGet, "/xchats/api/v1/settings/update-check", nil, http.StatusOK},
		{"tunnel status", http.MethodGet, "/xchats/api/v1/settings/tunnel", nil, http.StatusServiceUnavailable},
		{"tunnel start", http.MethodPost, "/xchats/api/v1/settings/tunnel/start", nil, http.StatusServiceUnavailable},
		{"tunnel stop", http.MethodPost, "/xchats/api/v1/settings/tunnel/stop", nil, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, env := h.do(tc.method, tc.path, tc.body)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d; body=%s", resp.StatusCode, tc.wantStatus, env["message"])
			}
		})
	}
}
