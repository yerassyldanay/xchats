// Package updatecheck asks GitHub's public releases API whether a newer
// xchats release exists than the one currently running — a self-hosted
// deployment's only "is there an update" signal, since it isn't behind any
// centralized update server of its own. Unauthenticated GitHub API calls
// are rate-limited to 60/hour per source IP, so results are cached.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Result is what one check produces.
type Result struct {
	CurrentVersion string `json:"current_version"`
	// LatestVersion/ReleaseURL are "" when GitHub has no releases yet, or
	// the check itself couldn't complete — never guessed.
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
	// CheckedAt is when this result was actually fetched from GitHub — the
	// same cached Result can be returned for up to CacheFor, so a caller
	// can tell how fresh it is.
	CheckedAt time.Time `json:"checked_at"`
}

// DefaultCacheFor bounds how often Check calls GitHub when Checker.CacheFor
// is unset.
const DefaultCacheFor = time.Hour

// Checker asks one GitHub repo's "latest release" endpoint and caches the
// answer for CacheFor. Safe for concurrent use.
type Checker struct {
	// Repo is "owner/name" (e.g. "yerassyldanay/xchats").
	Repo string
	// CurrentVersion is this build's own version — internal/version.Version
	// in production; a field here (not a package-level read) purely so
	// tests can set it directly without a global.
	CurrentVersion string
	// CacheFor bounds how often Check actually calls GitHub. Zero means
	// DefaultCacheFor.
	CacheFor time.Duration

	// httpClient/baseURL are test seams; production leaves both zero
	// (http.DefaultClient, https://api.github.com).
	httpClient *http.Client
	baseURL    string

	mu       sync.Mutex
	cached   *Result
	cachedAt time.Time
}

// NewChecker builds a Checker for repo (e.g. "yerassyldanay/xchats"),
// reporting updates relative to currentVersion.
func NewChecker(repo, currentVersion string) *Checker {
	return &Checker{Repo: repo, CurrentVersion: currentVersion}
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check returns the cached Result if still fresh, otherwise calls GitHub,
// caches the answer, and returns it. A GitHub call failure (network error,
// rate limit, or — entirely normal for a repo with no releases yet — a 404)
// is never fatal: Check falls back to "no update information available"
// (UpdateAvailable: false, LatestVersion: "") rather than returning an
// error, since "we couldn't check" must never be confused with "you are up
// to date."
func (c *Checker) Check(ctx context.Context) Result {
	cacheFor := c.CacheFor
	if cacheFor <= 0 {
		cacheFor = DefaultCacheFor
	}

	c.mu.Lock()
	if c.cached != nil && time.Since(c.cachedAt) < cacheFor {
		result := *c.cached
		c.mu.Unlock()
		return result
	}
	c.mu.Unlock()

	result := c.fetch(ctx)

	c.mu.Lock()
	c.cached = &result
	c.cachedAt = time.Now()
	c.mu.Unlock()
	return result
}

func (c *Checker) fetch(ctx context.Context) Result {
	result := Result{CurrentVersion: c.CurrentVersion, CheckedAt: time.Now().UTC()}

	base := c.baseURL
	if base == "" {
		base = "https://api.github.com"
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases/latest", base, c.Repo), nil)
	if err != nil {
		return result
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return result
	}
	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return result
	}

	result.LatestVersion = strings.TrimPrefix(rel.TagName, "v")
	result.ReleaseURL = rel.HTMLURL
	result.UpdateAvailable = result.LatestVersion != "" && result.LatestVersion != strings.TrimPrefix(c.CurrentVersion, "v")
	return result
}
