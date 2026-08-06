package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/ngrokapi"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
)

type ngrokCredentialReader interface {
	Get(ctx context.Context, key credentials.Key) (string, error)
}

type ngrokSettingsStore interface {
	Load() (settings.Settings, error)
	Update(func(*settings.Settings)) (settings.Settings, error)
}

type ngrokDomainLister interface {
	ListReservedDomains(ctx context.Context, apiKey string) ([]ngrokapi.ReservedDomain, error)
}

// applyNgrokPublicOrigin makes the embedded tunnel's static HTTPS origin the
// single source of truth for MCP. When no domain has been saved yet, the ngrok
// account API key discovers the account's assigned domain and persists it.
// The returned boolean is also the boot-time auto-start decision.
func applyNgrokPublicOrigin(ctx context.Context, cfg *config.Config, creds ngrokCredentialReader, settingsStore ngrokSettingsStore, domains ngrokDomainLister) (bool, error) {
	if creds == nil || settingsStore == nil {
		return false, nil
	}
	token, err := creds.Get(ctx, credentials.KeyNgrokAuthtoken)
	if errors.Is(err, credentials.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read ngrok authtoken: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return false, nil
	}

	st, err := settingsStore.Load()
	if err != nil {
		return false, fmt.Errorf("load ngrok settings: %w", err)
	}
	domain := strings.TrimSpace(st.Ngrok.Domain)
	if domain == "" {
		apiKey, err := creds.Get(ctx, credentials.KeyNgrokAPIKey)
		if errors.Is(err, credentials.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("read ngrok API key: %w", err)
		}
		if strings.TrimSpace(apiKey) == "" {
			return false, nil
		}
		if domains == nil {
			return false, errors.New("ngrok domain discovery is unavailable")
		}
		reserved, err := domains.ListReservedDomains(ctx, apiKey)
		if err != nil {
			return false, fmt.Errorf("list ngrok reserved domains: %w", err)
		}
		discovered, err := ngrokapi.SelectDefaultDomain(reserved)
		if err != nil {
			return false, err
		}
		updated, err := settingsStore.Update(func(current *settings.Settings) {
			// Do not overwrite a domain an administrator saved while the API
			// request was in flight.
			if strings.TrimSpace(current.Ngrok.Domain) == "" {
				current.Ngrok.Domain = discovered
			}
		})
		if err != nil {
			return false, fmt.Errorf("persist discovered ngrok domain: %w", err)
		}
		domain = strings.TrimSpace(updated.Ngrok.Domain)
	}
	if strings.Contains(domain, "://") {
		return false, fmt.Errorf("ngrok domain must be a hostname, not a URL")
	}
	publicURL, err := url.Parse("https://" + domain)
	if err != nil || publicURL.Hostname() == "" || publicURL.Port() != "" || publicURL.User != nil || publicURL.RawQuery != "" || publicURL.Fragment != "" || (publicURL.Path != "" && publicURL.Path != "/") {
		return false, fmt.Errorf("invalid ngrok domain %q", domain)
	}

	cfg.Server.APIBaseURL = "https://" + publicURL.Host
	return true, nil
}
