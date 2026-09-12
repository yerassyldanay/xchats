package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// requestTimeout bounds every individual HTTP call this tool makes — a
// hung server must never hang the whole load run.
const requestTimeout = 15 * time.Second

// envelope is every xchats API response's outer shape: {"payload":...,
// "errcode":"...","message":"..."}.
type envelope struct {
	Payload json.RawMessage `json:"payload"`
	Errcode string          `json:"errcode"`
	Message string          `json:"message"`
}

// apiClient is a shared, authenticated HTTP client — *http.Client (and its
// cookiejar.Jar) is safe for concurrent use by multiple goroutines, so one
// instance is reused by every worker after Login/EnsureAuthenticated runs
// once at startup.
type apiClient struct {
	baseURL string
	http    *http.Client
}

func newAPIClient(baseURL string) (*apiClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &apiClient{
		baseURL: baseURL,
		http:    &http.Client{Jar: jar, Timeout: requestTimeout},
	}, nil
}

// do issues one HTTP request and decodes the envelope. body may be nil (no
// request body — a GET). out, if non-nil, receives the decoded payload.
// Returns the HTTP status code even on a non-2xx response, alongside an
// error, so callers can distinguish transport failures from HTTP-level
// ones per the load tool's own error-classification contract.
func (c *apiClient) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("transport: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	if out == nil {
		return resp.StatusCode, nil
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return resp.StatusCode, fmt.Errorf("decode envelope: %w: body=%s", err, truncate(string(raw), 300))
	}
	if env.Errcode != "" && env.Errcode != "OK" {
		return resp.StatusCode, fmt.Errorf("api error %s: %s", env.Errcode, env.Message)
	}
	if err := json.Unmarshal(env.Payload, out); err != nil {
		return resp.StatusCode, fmt.Errorf("decode payload: %w: body=%s", err, truncate(string(raw), 300))
	}
	return resp.StatusCode, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// mePayload mirrors httpapi's login/me response shape closely enough for
// this tool's own needs (only MustChangePassword is read).
type mePayload struct {
	User struct {
		MustChangePassword bool `json:"must_change_password"`
	} `json:"user"`
}

// EnsureAuthenticated logs in once, transparently completing the
// fresh-install password-rotation flow (POST /auth/password) when the
// account's password must be changed — after this call, c's cookie jar
// carries a valid session for every subsequent request from any worker.
//
// If login with password is rejected, it retries once with newPassword
// before giving up: a repeated invocation against the SAME already-seeded
// server (e.g. the Makefile's profile-load then profile-trace, both
// authenticating fresh against one long-lived profile-server) finds the
// account already rotated by a prior run, and the original bootstrap
// password no longer works — this is not a fresh 401, just stale local
// knowledge of which password is current.
func (c *apiClient) EnsureAuthenticated(ctx context.Context, email, password, newPassword string) error {
	me, loginErr := c.login(ctx, email, password)
	if loginErr != nil {
		var err error
		me, err = c.login(ctx, email, newPassword)
		if err != nil {
			return fmt.Errorf("login failed with both the given password and --new-password (likely already rotated by a prior run): %w", loginErr)
		}
	}
	if !me.User.MustChangePassword {
		return nil
	}
	if _, err := c.do(ctx, http.MethodPost, "/xchats/api/v1/auth/password", map[string]string{
		"current_password": password, "new_password": newPassword,
	}, &me); err != nil {
		return fmt.Errorf("password rotation: %w", err)
	}
	return nil
}

func (c *apiClient) login(ctx context.Context, email, password string) (mePayload, error) {
	var me mePayload
	if _, err := c.do(ctx, http.MethodPost, "/xchats/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, &me); err != nil {
		return mePayload{}, fmt.Errorf("login: %w", err)
	}
	return me, nil
}
