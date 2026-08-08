package credentials

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Field is one input the Settings UI collects for a Provider — always a
// secret (masked in the UI, never echoed back in a GET response).
type Field struct {
	Key   Key
	Label string
}

// The ngrok integration has two distinct credentials. The authtoken connects
// the embedded agent; the API key lists account resources such as the assigned
// static development domain. Both are exported because tunnel/runtime wiring
// consumes them without duplicating credential-name literals.
const (
	KeyNgrokAuthtoken Key = "ngrok.authtoken"
	KeyNgrokAPIKey    Key = "ngrok.api_key"
)

// Provider is the static metadata and validation logic for one integration
// the Settings UI can configure a credential for.
type Provider struct {
	ID            string
	DisplayName   string
	Fields        []Field
	CredentialURL string // where an operator goes to obtain one, shown as a link
	DocsURL       string
	// HasBaseURL/HasModel tell the Settings UI whether to also show a base
	// URL / default model picker for this provider — those values are not
	// secrets, so they live in internal/settings.ProviderSettings, not here.
	HasBaseURL bool
	HasModel   bool
	// Validate checks the given field values against the live provider API.
	// nil for providers with no lightweight "is this credential valid" call
	// to make.
	// values is keyed by this Provider's own Field.Key values, plus (for
	// providers with HasBaseURL) an optional "<id>.base_url" entry the
	// caller may set to validate against a self-hosted/proxied endpoint
	// instead of the public default.
	Validate func(ctx context.Context, values map[Key]string) error
}

// providers is the fixed, ordered list every exported lookup below reads
// from — declared once so Providers()/ProviderByID never construct it twice.
var providers = []Provider{
	{
		ID: "openrouter", DisplayName: "OpenRouter",
		Fields:        []Field{{Key: "openrouter.api_key", Label: "API Key"}},
		CredentialURL: "https://openrouter.ai/keys",
		DocsURL:       "https://openrouter.ai/docs",
		HasBaseURL:    true,
		HasModel:      true,
		Validate:      validateOpenRouter,
	},
	{
		ID: "openai", DisplayName: "OpenAI",
		Fields:        []Field{{Key: "openai.api_key", Label: "API Key"}},
		CredentialURL: "https://platform.openai.com/api-keys",
		DocsURL:       "https://platform.openai.com/docs",
		HasBaseURL:    true,
		HasModel:      true,
		Validate:      validateOpenAI,
	},
	{
		ID: "gemini", DisplayName: "Google Gemini",
		Fields:        []Field{{Key: "gemini.api_key", Label: "API Key"}},
		CredentialURL: "https://aistudio.google.com/apikey",
		DocsURL:       "https://ai.google.dev/gemini-api/docs",
		HasBaseURL:    true,
		HasModel:      true,
		Validate:      validateGemini,
	},
	{
		ID: "ngrok", DisplayName: "ngrok",
		Fields: []Field{
			{Key: KeyNgrokAuthtoken, Label: "Authtoken"},
			{Key: KeyNgrokAPIKey, Label: "API Key"},
		},
		CredentialURL: "https://dashboard.ngrok.com/api",
		DocsURL:       "https://ngrok.com/docs",
		HasBaseURL:    false,
		HasModel:      false,
		Validate:      validateNgrok,
	},
	{
		ID: "langfuse", DisplayName: "Langfuse",
		Fields: []Field{
			{Key: "langfuse.public_key", Label: "Public Key"},
			{Key: "langfuse.secret_key", Label: "Secret Key"},
		},
		CredentialURL: "https://cloud.langfuse.com/project/settings/api-keys",
		DocsURL:       "https://langfuse.com/docs",
		HasBaseURL:    true,
		HasModel:      false,
		Validate:      validateLangfuse,
	},
	{
		// BYO-App: this is the operator's OWN Meta Developer App (Instagram
		// Direct, Facebook Messenger, WhatsApp Cloud API all read it — see
		// internal/meta.AppCredentials), never a shared xchats-owned app.
		// HasBaseURL is false: unlike an LLM provider, Meta offers no
		// self-hosted Graph API an operator would point this at — the
		// baseURLFor override validateMeta still honors is a test-only hook.
		ID: "meta", DisplayName: "Meta (Instagram · Messenger · WhatsApp Cloud)",
		Fields: []Field{
			{Key: "meta.app_id", Label: "App ID"},
			{Key: "meta.app_secret", Label: "App Secret"},
		},
		CredentialURL: "https://developers.facebook.com/apps/",
		DocsURL:       "https://developers.facebook.com/docs/graph-api/",
		HasBaseURL:    false,
		HasModel:      false,
		Validate:      validateMeta,
	},
}

// Providers returns the static metadata for every credential-backed
// integration the Settings UI can configure, in a fixed display order. Each
// call returns a fresh copy — callers may not mutate the package's own list.
func Providers() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	return out
}

// ProviderByID looks up one provider's metadata by id ("openrouter", ...).
func ProviderByID(id string) (Provider, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// validateTimeout bounds every provider validation call — a hung or
// slow-to-respond provider must never hang a Settings UI "test connection"
// request indefinitely.
const validateTimeout = 10 * time.Second

// maxValidateBody caps how much of a validation response body this package
// ever reads — enough for the small JSON error payloads these APIs return,
// never enough for a misbehaving endpoint to exhaust memory.
const maxValidateBody = 64 * 1024

var validateHTTPClient = &http.Client{Timeout: validateTimeout}

// doGet issues a GET against url, applying setAuth (if non-nil) to the
// request before sending it — the one place every validator's HTTP call
// goes through, so the timeout and body cap apply uniformly.
func doGet(ctx context.Context, rawURL string, setAuth func(*http.Request)) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if setAuth != nil {
		setAuth(req)
	}
	return validateHTTPClient.Do(req)
}

// classify turns a validation response into the package's three-way
// verdict: nil (200), ErrInvalidCredential (401/403, or a 400 whose body
// badRequestIsInvalid recognizes as a rejected-key shape), or
// ErrValidationUnavailable (anything else). Secrets are never present in
// the response body of any of these APIs on a rejection, so logging body
// contents on error would be safe — this package simply never does, since
// nothing here logs at all.
func classify(resp *http.Response, badRequestIsInvalid func([]byte) bool) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxValidateBody))
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ErrInvalidCredential
	case resp.StatusCode == http.StatusBadRequest && badRequestIsInvalid != nil && badRequestIsInvalid(body):
		return ErrInvalidCredential
	default:
		return ErrValidationUnavailable
	}
}

// baseURLFor resolves the base URL a provider's validator should hit:
// values["<id>.base_url"] if the caller set one (a self-hosted/proxied
// endpoint, or a test double), else def, the real public default. Every
// validator below goes through this rather than a hardcoded literal, so
// tests can point any of them at an httptest.Server without a live network
// call to a third party.
func baseURLFor(values map[Key]string, id, def string) string {
	if base := values[Key(id+".base_url")]; base != "" {
		return strings.TrimRight(base, "/")
	}
	return def
}

func validateOpenRouter(ctx context.Context, values map[Key]string) error {
	base := baseURLFor(values, "openrouter", "https://openrouter.ai/api/v1")
	resp, err := doGet(ctx, base+"/key", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+values["openrouter.api_key"])
	})
	if err != nil {
		return ErrValidationUnavailable
	}
	return classify(resp, nil)
}

func validateOpenAI(ctx context.Context, values map[Key]string) error {
	base := baseURLFor(values, "openai", "https://api.openai.com")
	resp, err := doGet(ctx, base+"/v1/models", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+values["openai.api_key"])
	})
	if err != nil {
		return ErrValidationUnavailable
	}
	return classify(resp, nil)
}

func validateGemini(ctx context.Context, values map[Key]string) error {
	base := baseURLFor(values, "gemini", "https://generativelanguage.googleapis.com")
	u := base + "/v1beta/models?key=" + url.QueryEscape(values["gemini.api_key"])
	resp, err := doGet(ctx, u, nil)
	if err != nil {
		return ErrValidationUnavailable
	}
	return classify(resp, isGeminiInvalidKeyBody)
}

// validateNgrok verifies the account API key by listing reserved domains.
// The agent authtoken is independently verified when a tunnel is opened.
func validateNgrok(ctx context.Context, values map[Key]string) error {
	base := baseURLFor(values, "ngrok", "https://api.ngrok.com")
	resp, err := doGet(ctx, base+"/reserved_domains?limit=1", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+values[KeyNgrokAPIKey])
		r.Header.Set("ngrok-version", "2")
	})
	if err != nil {
		return ErrValidationUnavailable
	}
	return classify(resp, nil)
}

// isGeminiInvalidKeyBody recognizes Gemini's own shape for a rejected key:
// unlike every other provider here, an invalid API key comes back as HTTP
// 400 (not 401/403) with this specific error status in the body.
func isGeminiInvalidKeyBody(body []byte) bool {
	var payload struct {
		Error struct {
			Status string `json:"status"`
		} `json:"error"`
	}
	return json.Unmarshal(body, &payload) == nil && payload.Error.Status == "API_KEY_INVALID"
}

func validateLangfuse(ctx context.Context, values map[Key]string) error {
	base := baseURLFor(values, "langfuse", "https://cloud.langfuse.com")
	resp, err := doGet(ctx, base+"/api/public/projects", func(r *http.Request) {
		r.SetBasicAuth(values["langfuse.public_key"], values["langfuse.secret_key"])
	})
	if err != nil {
		return ErrValidationUnavailable
	}
	return classify(resp, nil)
}

// validateMeta proves an App ID/Secret pair is real with a zero-side-effect
// debug_token call against the app's OWN token (<id>|<secret> — Meta's own
// app-access-token shorthand), input_token and access_token both set to it.
// A malformed/wrong pair makes that token itself invalid, and Meta rejects
// the call outright — there is no dedicated "verify these app credentials"
// endpoint, so this self-check is the cheapest real proof available.
func validateMeta(ctx context.Context, values map[Key]string) error {
	appToken := values["meta.app_id"] + "|" + values["meta.app_secret"]
	base := baseURLFor(values, "meta", "https://graph.facebook.com/v21.0")
	u := base + "/debug_token?input_token=" + url.QueryEscape(appToken) + "&access_token=" + url.QueryEscape(appToken)
	resp, err := doGet(ctx, u, nil)
	if err != nil {
		return ErrValidationUnavailable
	}
	return classify(resp, isMetaInvalidAppBody)
}

// isMetaInvalidAppBody recognizes debug_token's shape for a rejected
// request: like Gemini (isGeminiInvalidKeyBody), Graph API returns 400 —
// not 401/403 — for most OAuthException-shaped errors, including "this
// app_id/app_secret pair does not resolve to a real app". Any non-zero
// error code on THIS specific self-referential call (input_token ==
// access_token == our own claimed app token) means the pair is invalid —
// there is no other reason this always-available endpoint would reject it.
func isMetaInvalidAppBody(body []byte) bool {
	var payload struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	return json.Unmarshal(body, &payload) == nil && payload.Error.Code != 0
}
