package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// llamaparseMaxResponseBytes bounds how much of a vendor response this
// package ever reads into memory — generous for a parsed document's
// markdown, never unbounded.
const llamaparseMaxResponseBytes = 20 << 20 // 20 MiB

// llamaparseDefaultPollInterval is how often this polls a submitted job's
// status — LlamaParse jobs typically take single-digit seconds for a normal
// document, so sub-second polling would just be wasted requests.
const llamaparseDefaultPollInterval = 2 * time.Second

// llamaparseProvider calls LlamaParse's async upload -> poll -> markdown
// API (https://docs.cloud.llamaindex.ai) — plain net/http, no vendored SDK.
type llamaparseProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	pollInterval time.Duration
}

// NewLlamaParse builds the LlamaParse provider. baseURL "" defaults to the
// public API.
func NewLlamaParse(apiKey, baseURL string, client *http.Client) Provider {
	if baseURL == "" {
		baseURL = "https://api.cloud.llamaindex.ai"
	}
	return &llamaparseProvider{
		apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), client: client,
		pollInterval: llamaparseDefaultPollInterval,
	}
}

func (p *llamaparseProvider) Name() string { return "llamaparse" }

// Supports implements the llamaparse row of the capability matrix: PDF,
// DOCX, and txt/md/csv — never a URL (LlamaParse parses documents, not
// pages) and never an image (v1 has no vision call — see the "Vision gap"
// risk in plan/DECISIONS.md).
func (p *llamaparseProvider) Supports(src Source) bool {
	if src.Kind != SourceFile {
		return false
	}
	switch src.MimeType {
	case MimePDF, MimeDOCX:
		return true
	}
	return isTextlike(src.MimeType)
}

func (p *llamaparseProvider) Extract(ctx context.Context, src Source) (Result, error) {
	if !p.Supports(src) {
		return Result{}, ErrUnsupported
	}
	jobID, err := p.upload(ctx, src)
	if err != nil {
		return Result{}, err
	}
	md, err := p.pollUntilDone(ctx, jobID)
	if err != nil {
		return Result{}, err
	}
	return Result{Text: md, Images: imagesFromMarkdown(md), Provider: p.Name()}, nil
}

func (p *llamaparseProvider) upload(ctx context.Context, src Source) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	filename := src.Filename
	if filename == "" {
		filename = "upload"
	}
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(src.Data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/v1/parsing/upload", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	data, status, err := p.do(req)
	if err != nil {
		return "", fmt.Errorf("extractor: llamaparse upload: %w", err)
	}
	if err := authOrStatusError("llamaparse upload", status, data); err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("extractor: parse llamaparse upload response: %w", err)
	}
	if out.ID == "" {
		return "", errors.New("extractor: llamaparse upload response has no job id")
	}
	return out.ID, nil
}

// pollUntilDone polls the job's status until it reaches a terminal state or
// ctx is done. There is no separate poll-timeout knob here: the caller
// (internal/kbimport/pass1.go) wraps the whole Extract call in a context
// carrying its own ExtractTimeout, and that deadline is this loop's actual
// bound — a job that outlives it is force-failed the same way any other
// extraction timeout is (plan/playground.md: "A parse stuck beyond a
// timeout is force-failed").
func (p *llamaparseProvider) pollUntilDone(ctx context.Context, jobID string) (string, error) {
	for {
		status, err := p.jobStatus(ctx, jobID)
		if err != nil {
			return "", err
		}
		switch strings.ToUpper(status) {
		case "SUCCESS", "COMPLETED":
			return p.fetchMarkdown(ctx, jobID)
		case "ERROR", "FAILED", "CANCELED", "CANCELLED":
			return "", fmt.Errorf("extractor: llamaparse job %s did not succeed: status %s", jobID, status)
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("extractor: llamaparse job %s: %w", jobID, ctx.Err())
		case <-time.After(p.pollInterval):
		}
	}
}

func (p *llamaparseProvider) jobStatus(ctx context.Context, jobID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/v1/parsing/job/"+jobID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	data, status, err := p.do(req)
	if err != nil {
		return "", fmt.Errorf("extractor: llamaparse job status: %w", err)
	}
	if err := authOrStatusError("llamaparse job status", status, data); err != nil {
		return "", err
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("extractor: parse llamaparse job status: %w", err)
	}
	return out.Status, nil
}

func (p *llamaparseProvider) fetchMarkdown(ctx context.Context, jobID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/v1/parsing/job/"+jobID+"/result/markdown", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	data, status, err := p.do(req)
	if err != nil {
		return "", fmt.Errorf("extractor: llamaparse fetch result: %w", err)
	}
	if err := authOrStatusError("llamaparse result", status, data); err != nil {
		return "", err
	}
	var out struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("extractor: parse llamaparse result: %w", err)
	}
	return out.Markdown, nil
}

// do runs req and returns its drained body plus status code — the shared
// tail every llamaparse call above goes through.
func (p *llamaparseProvider) do(req *http.Request) ([]byte, int, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, llamaparseMaxResponseBytes))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// authOrStatusError classifies a non-2xx LlamaParse response into
// ErrProviderAuth (401/403) or a generic status error carrying a bounded
// snippet of the response body.
func authOrStatusError(what string, status int, body []byte) error {
	if status == http.StatusOK {
		return nil
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("%w: %s returned status %d", ErrProviderAuth, what, status)
	}
	return fmt.Errorf("extractor: %s returned status %d: %s", what, status, truncate(string(body), 500))
}
