package main

import (
	"context"
	"errors"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/ngrokapi"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
)

type fakeNgrokCredentialReader struct {
	token     string
	apiKey    string
	tokenErr  error
	apiKeyErr error
}

func (f fakeNgrokCredentialReader) Get(_ context.Context, key credentials.Key) (string, error) {
	switch key {
	case credentials.KeyNgrokAuthtoken:
		if f.tokenErr != nil {
			return "", f.tokenErr
		}
		if f.token == "" {
			return "", credentials.ErrNotFound
		}
		return f.token, nil
	case credentials.KeyNgrokAPIKey:
		if f.apiKeyErr != nil {
			return "", f.apiKeyErr
		}
		if f.apiKey == "" {
			return "", credentials.ErrNotFound
		}
		return f.apiKey, nil
	default:
		return "", credentials.ErrNotFound
	}
}

type fakeNgrokSettingsStore struct {
	domain    string
	loadErr   error
	updateErr error
	updates   int
}

func (f *fakeNgrokSettingsStore) Load() (settings.Settings, error) {
	if f.loadErr != nil {
		return settings.Settings{}, f.loadErr
	}
	return settings.Settings{Ngrok: settings.NgrokSettings{Domain: f.domain}}, nil
}

func (f *fakeNgrokSettingsStore) Update(fn func(*settings.Settings)) (settings.Settings, error) {
	if f.updateErr != nil {
		return settings.Settings{}, f.updateErr
	}
	st := settings.Settings{Ngrok: settings.NgrokSettings{Domain: f.domain}}
	fn(&st)
	f.domain = st.Ngrok.Domain
	f.updates++
	return st, nil
}

type fakeNgrokDomainLister struct {
	domains []ngrokapi.ReservedDomain
	err     error
	calls   int
}

func (f *fakeNgrokDomainLister) ListReservedDomains(context.Context, string) ([]ngrokapi.ReservedDomain, error) {
	f.calls++
	return f.domains, f.err
}

func TestApplyNgrokPublicOrigin(t *testing.T) {
	tests := []struct {
		name       string
		creds      ngrokCredentialReader
		settings   *fakeNgrokSettingsStore
		domains    *fakeNgrokDomainLister
		wantOrigin string
		wantDomain string
		wantAuto   bool
		wantErr    bool
	}{
		{
			name:       "saved token and domain override API base URL",
			creds:      fakeNgrokCredentialReader{token: "secret-token"},
			settings:   &fakeNgrokSettingsStore{domain: "stable-example.ngrok-free.app"},
			wantOrigin: "https://stable-example.ngrok-free.app",
			wantDomain: "stable-example.ngrok-free.app",
			wantAuto:   true,
		},
		{
			name:       "missing token keeps configured fallback",
			creds:      fakeNgrokCredentialReader{tokenErr: credentials.ErrNotFound},
			settings:   &fakeNgrokSettingsStore{domain: "stable-example.ngrok-free.app"},
			wantOrigin: "http://localhost:8090",
			wantDomain: "stable-example.ngrok-free.app",
		},
		{
			name:       "blank domain without API key keeps configured fallback",
			creds:      fakeNgrokCredentialReader{token: "secret-token"},
			settings:   &fakeNgrokSettingsStore{},
			wantOrigin: "http://localhost:8090",
		},
		{
			name:     "API key discovers and persists assigned domain",
			creds:    fakeNgrokCredentialReader{token: "secret-token", apiKey: "secret-api-key"},
			settings: &fakeNgrokSettingsStore{},
			domains: &fakeNgrokDomainLister{domains: []ngrokapi.ReservedDomain{
				{Domain: "assigned-example.ngrok-free.app", CreatedAt: "2024-01-01T00:00:00Z"},
			}},
			wantOrigin: "https://assigned-example.ngrok-free.app",
			wantDomain: "assigned-example.ngrok-free.app",
			wantAuto:   true,
		},
		{
			name:       "domain discovery failure is returned",
			creds:      fakeNgrokCredentialReader{token: "secret-token", apiKey: "secret-api-key"},
			settings:   &fakeNgrokSettingsStore{},
			domains:    &fakeNgrokDomainLister{err: errors.New("API down")},
			wantOrigin: "http://localhost:8090",
			wantErr:    true,
		},
		{
			name:       "account without reserved domains is rejected",
			creds:      fakeNgrokCredentialReader{token: "secret-token", apiKey: "secret-api-key"},
			settings:   &fakeNgrokSettingsStore{},
			domains:    &fakeNgrokDomainLister{},
			wantOrigin: "http://localhost:8090",
			wantErr:    true,
		},
		{
			name:       "invalid domain is rejected",
			creds:      fakeNgrokCredentialReader{token: "secret-token"},
			settings:   &fakeNgrokSettingsStore{domain: "stable-example.ngrok-free.app/path"},
			wantOrigin: "http://localhost:8090",
			wantDomain: "stable-example.ngrok-free.app/path",
			wantErr:    true,
		},
		{
			name:       "domain with port is rejected",
			creds:      fakeNgrokCredentialReader{token: "secret-token"},
			settings:   &fakeNgrokSettingsStore{domain: "stable-example.ngrok-free.app:443"},
			wantOrigin: "http://localhost:8090",
			wantDomain: "stable-example.ngrok-free.app:443",
			wantErr:    true,
		},
		{
			name:       "settings read failure is returned",
			creds:      fakeNgrokCredentialReader{token: "secret-token"},
			settings:   &fakeNgrokSettingsStore{loadErr: errors.New("read failed")},
			wantOrigin: "http://localhost:8090",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Server: config.ServerConfig{APIBaseURL: "http://localhost:8090"}}
			gotAuto, err := applyNgrokPublicOrigin(context.Background(), cfg, tt.creds, tt.settings, tt.domains)
			if (err != nil) != tt.wantErr {
				t.Fatalf("applyNgrokPublicOrigin error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotAuto != tt.wantAuto {
				t.Errorf("auto-start = %v, want %v", gotAuto, tt.wantAuto)
			}
			if cfg.Server.APIBaseURL != tt.wantOrigin {
				t.Errorf("APIBaseURL = %q, want %q", cfg.Server.APIBaseURL, tt.wantOrigin)
			}
			if tt.settings.domain != tt.wantDomain {
				t.Errorf("stored domain = %q, want %q", tt.settings.domain, tt.wantDomain)
			}
		})
	}
}
