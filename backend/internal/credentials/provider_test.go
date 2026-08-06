package credentials

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProvidersReturnsACopy(t *testing.T) {
	a := Providers()
	if len(a) == 0 {
		t.Fatal("Providers() returned nothing")
	}
	a[0].DisplayName = "mutated"
	b := Providers()
	if b[0].DisplayName == "mutated" {
		t.Fatal("Providers() callers can mutate the package's own provider list")
	}
}

func TestProviderByID(t *testing.T) {
	for _, id := range []string{"openrouter", "openai", "gemini", "ngrok", "langfuse"} {
		t.Run(id, func(t *testing.T) {
			p, ok := ProviderByID(id)
			if !ok {
				t.Fatalf("ProviderByID(%q) not found", id)
			}
			if p.ID != id {
				t.Errorf("p.ID = %q, want %q", p.ID, id)
			}
			if p.DisplayName == "" {
				t.Error("DisplayName is empty")
			}
			if len(p.Fields) == 0 {
				t.Error("Fields is empty")
			}
			if p.CredentialURL == "" || p.DocsURL == "" {
				t.Error("CredentialURL/DocsURL is empty")
			}
		})
	}
	if _, ok := ProviderByID("does-not-exist"); ok {
		t.Error("ProviderByID(unknown) = ok, want not found")
	}
}

func TestNgrokHasAPIKeyValidator(t *testing.T) {
	p, ok := ProviderByID("ngrok")
	if !ok {
		t.Fatal("ngrok provider not found")
	}
	if p.Validate == nil {
		t.Fatal("ngrok Provider.Validate is nil, want account API-key validation")
	}
	if len(p.Fields) != 2 || p.Fields[0].Key != KeyNgrokAuthtoken || p.Fields[1].Key != KeyNgrokAPIKey {
		t.Errorf("ngrok fields = %#v, want authtoken and API key", p.Fields)
	}
}

// validatorCases enumerates the four validated providers together with how
// to point each one at a test server via the values map (baseURLFor's
// "<id>.base_url" convention) and how to build a values map for a given
// credential value.
var validatorCases = []struct {
	id       string
	validate func(ctx context.Context, values map[Key]string) error
	values   func(v string) map[Key]string
}{
	{"openrouter", validateOpenRouter, func(v string) map[Key]string { return map[Key]string{"openrouter.api_key": v} }},
	{"openai", validateOpenAI, func(v string) map[Key]string { return map[Key]string{"openai.api_key": v} }},
	{"gemini", validateGemini, func(v string) map[Key]string { return map[Key]string{"gemini.api_key": v} }},
	{"langfuse", validateLangfuse, func(v string) map[Key]string {
		return map[Key]string{"langfuse.public_key": "pub", "langfuse.secret_key": v}
	}},
}

func TestValidatorsClassifyResponses(t *testing.T) {
	for _, tc := range validatorCases {
		t.Run(tc.id, func(t *testing.T) {
			t.Run("200 is valid", func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()
				values := tc.values("secret")
				values[Key(tc.id+".base_url")] = srv.URL
				if err := tc.validate(context.Background(), values); err != nil {
					t.Errorf("200 response: err = %v, want nil", err)
				}
			})

			t.Run("401 is invalid", func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer srv.Close()
				values := tc.values("bad-secret")
				values[Key(tc.id+".base_url")] = srv.URL
				if err := tc.validate(context.Background(), values); !errors.Is(err, ErrInvalidCredential) {
					t.Errorf("401 response: err = %v, want ErrInvalidCredential", err)
				}
			})

			t.Run("403 is invalid", func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusForbidden)
				}))
				defer srv.Close()
				values := tc.values("bad-secret")
				values[Key(tc.id+".base_url")] = srv.URL
				if err := tc.validate(context.Background(), values); !errors.Is(err, ErrInvalidCredential) {
					t.Errorf("403 response: err = %v, want ErrInvalidCredential", err)
				}
			})

			t.Run("500 is unavailable", func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				defer srv.Close()
				values := tc.values("secret")
				values[Key(tc.id+".base_url")] = srv.URL
				if err := tc.validate(context.Background(), values); !errors.Is(err, ErrValidationUnavailable) {
					t.Errorf("500 response: err = %v, want ErrValidationUnavailable", err)
				}
			})

			t.Run("unreachable server is unavailable", func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				srv.Close() // closed before use: connections to it now refuse
				values := tc.values("secret")
				values[Key(tc.id+".base_url")] = srv.URL
				if err := tc.validate(context.Background(), values); !errors.Is(err, ErrValidationUnavailable) {
					t.Errorf("unreachable server: err = %v, want ErrValidationUnavailable", err)
				}
			})

			t.Run("caller cancellation is unavailable", func(t *testing.T) {
				block := make(chan struct{})
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					<-block
				}))
				defer func() { close(block); srv.Close() }()
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()
				values := tc.values("secret")
				values[Key(tc.id+".base_url")] = srv.URL
				if err := tc.validate(ctx, values); !errors.Is(err, ErrValidationUnavailable) {
					t.Errorf("cancelled context: err = %v, want ErrValidationUnavailable", err)
				}
			})
		})
	}
}

func TestValidateGeminiTreats400APIKeyInvalidAsInvalidCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"API key not valid.","status":"API_KEY_INVALID"}}`))
	}))
	defer srv.Close()
	values := map[Key]string{"gemini.api_key": "bad", "gemini.base_url": srv.URL}
	if err := validateGemini(context.Background(), values); !errors.Is(err, ErrInvalidCredential) {
		t.Errorf("gemini 400 API_KEY_INVALID: err = %v, want ErrInvalidCredential", err)
	}
}

func TestValidateGeminiTreatsOtherwiseShaped400AsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"some other problem","status":"INVALID_ARGUMENT"}}`))
	}))
	defer srv.Close()
	values := map[Key]string{"gemini.api_key": "x", "gemini.base_url": srv.URL}
	if err := validateGemini(context.Background(), values); !errors.Is(err, ErrValidationUnavailable) {
		t.Errorf("gemini 400 non-key-shaped: err = %v, want ErrValidationUnavailable", err)
	}
}

func TestValidateOpenRouterSendsBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/key" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/key")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := validateOpenRouter(context.Background(), map[Key]string{
		"openrouter.api_key": "sk-or-test", "openrouter.base_url": srv.URL,
	})
	if err != nil {
		t.Fatalf("validateOpenRouter: %v", err)
	}
	if gotAuth != "Bearer sk-or-test" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer sk-or-test")
	}
}

func TestValidateGeminiSendsKeyAsQueryParam(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("key")
		if r.URL.Path != "/v1beta/models" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1beta/models")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := validateGemini(context.Background(), map[Key]string{
		"gemini.api_key": "AIza-test", "gemini.base_url": srv.URL,
	})
	if err != nil {
		t.Fatalf("validateGemini: %v", err)
	}
	if gotQuery != "AIza-test" {
		t.Errorf("key query param = %q, want %q", gotQuery, "AIza-test")
	}
}

func TestValidateNgrokSendsAPIKeyAsBearer(t *testing.T) {
	var gotAuth, gotVersion, gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("ngrok-version")
		gotLimit = r.URL.Query().Get("limit")
		if r.URL.Path != "/reserved_domains" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/reserved_domains")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := validateNgrok(context.Background(), map[Key]string{
		KeyNgrokAPIKey: "ngrok-api-test", "ngrok.base_url": srv.URL,
	})
	if err != nil {
		t.Fatalf("validateNgrok: %v", err)
	}
	if gotAuth != "Bearer ngrok-api-test" || gotVersion != "2" || gotLimit != "1" {
		t.Errorf("request auth/version/limit = (%q, %q, %q)", gotAuth, gotVersion, gotLimit)
	}
}

func TestValidateLangfuseSendsBasicAuthAndDefaultsHost(t *testing.T) {
	var gotUser, gotPass string
	var ok bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok = r.BasicAuth()
		if r.URL.Path != "/api/public/projects" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/api/public/projects")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := validateLangfuse(context.Background(), map[Key]string{
		"langfuse.public_key": "pk-test", "langfuse.secret_key": "sk-test", "langfuse.base_url": srv.URL,
	})
	if err != nil {
		t.Fatalf("validateLangfuse: %v", err)
	}
	if !ok || gotUser != "pk-test" || gotPass != "sk-test" {
		t.Errorf("basic auth = (%q, %q, %v), want (%q, %q, true)", gotUser, gotPass, ok, "pk-test", "sk-test")
	}
}

func TestBaseURLFor(t *testing.T) {
	cases := []struct {
		name   string
		values map[Key]string
		def    string
		want   string
	}{
		{"override wins", map[Key]string{"langfuse.base_url": "https://self-hosted.example/"}, "https://cloud.langfuse.com", "https://self-hosted.example"},
		{"no override falls back to default", map[Key]string{}, "https://cloud.langfuse.com", "https://cloud.langfuse.com"},
		{"empty override falls back to default", map[Key]string{"langfuse.base_url": ""}, "https://cloud.langfuse.com", "https://cloud.langfuse.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := baseURLFor(tc.values, "langfuse", tc.def); got != tc.want {
				t.Errorf("baseURLFor = %q, want %q", got, tc.want)
			}
		})
	}
}
