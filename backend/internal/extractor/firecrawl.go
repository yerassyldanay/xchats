package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// firecrawlMaxResponseBytes bounds how much of a vendor response this
// package ever reads into memory — generous for a scraped page's markdown,
// never unbounded.
const firecrawlMaxResponseBytes = 10 << 20 // 10 MiB

// firecrawlProvider calls Firecrawl's /v2/scrape endpoint
// (https://docs.firecrawl.dev) — plain net/http, no vendored SDK (see
// package doc).
type firecrawlProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewFirecrawl builds the Firecrawl provider. baseURL "" defaults to the
// public API — a caller may point this at a self-hosted Firecrawl instance
// instead (internal/credentials' HasBaseURL: true for this provider).
func NewFirecrawl(apiKey, baseURL string, client *http.Client) Provider {
	if baseURL == "" {
		baseURL = "https://api.firecrawl.dev"
	}
	return &firecrawlProvider{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

func (p *firecrawlProvider) Name() string { return "firecrawl" }

// Supports implements the firecrawl row of the capability matrix: URL only.
func (p *firecrawlProvider) Supports(src Source) bool { return src.Kind == SourceURL }

func (p *firecrawlProvider) Extract(ctx context.Context, src Source) (Result, error) {
	if !p.Supports(src) {
		return Result{}, ErrUnsupported
	}
	reqBody, err := json.Marshal(map[string]any{
		"url":     src.URL,
		"formats": []string{"markdown"},
	})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v2/scrape", bytes.NewReader(reqBody))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("extractor: firecrawl request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, firecrawlMaxResponseBytes))
	if err != nil {
		return Result{}, fmt.Errorf("extractor: read firecrawl response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Result{}, fmt.Errorf("%w: firecrawl returned status %d", ErrProviderAuth, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("extractor: firecrawl returned status %d: %s", resp.StatusCode, truncate(string(data), 500))
	}

	var out struct {
		Success bool `json:"success"`
		Data    struct {
			Markdown string `json:"markdown"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return Result{}, fmt.Errorf("extractor: parse firecrawl response: %w", err)
	}
	if !out.Success {
		reason := out.Error
		if reason == "" {
			reason = "no markdown returned"
		}
		return Result{}, fmt.Errorf("extractor: firecrawl scrape failed: %s", reason)
	}
	return Result{Text: out.Data.Markdown, Images: imagesFromMarkdown(out.Data.Markdown), Provider: p.Name()}, nil
}
