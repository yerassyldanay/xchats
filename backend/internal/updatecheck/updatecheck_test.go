package updatecheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func checkerAgainst(t *testing.T, currentVersion string, handler http.HandlerFunc) (*Checker, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	c := NewChecker("yerassyldanay/xchats", currentVersion)
	c.baseURL = srv.URL
	return c, &calls
}

func releaseHandler(tag string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://github.com/yerassyldanay/xchats/releases/tag/%s"}`, tag, tag)
	}
}

func TestCheck_UpdateAvailableWhenLatestTagDiffersFromCurrent(t *testing.T) {
	c, _ := checkerAgainst(t, "0.1.0", releaseHandler("v0.2.0"))
	res := c.Check(context.Background())
	if !res.UpdateAvailable {
		t.Error("UpdateAvailable = false, want true")
	}
	if res.LatestVersion != "0.2.0" {
		t.Errorf("LatestVersion = %q, want 0.2.0 (the v prefix stripped)", res.LatestVersion)
	}
	if res.ReleaseURL == "" {
		t.Error("ReleaseURL is empty")
	}
	if res.CurrentVersion != "0.1.0" {
		t.Errorf("CurrentVersion = %q, want 0.1.0", res.CurrentVersion)
	}
}

func TestCheck_NoUpdateWhenTagsMatch(t *testing.T) {
	c, _ := checkerAgainst(t, "0.1.0", releaseHandler("v0.1.0"))
	res := c.Check(context.Background())
	if res.UpdateAvailable {
		t.Error("UpdateAvailable = true, want false when the latest release matches the running version")
	}
}

func TestCheck_NoUpdateWhenCurrentVersionHasNoVPrefixButTagDoes(t *testing.T) {
	// GitHub tags conventionally carry a "v" prefix; internal/version.Version
	// does not. Both must normalize to the same comparison.
	c, _ := checkerAgainst(t, "v0.1.0", releaseHandler("v0.1.0"))
	res := c.Check(context.Background())
	if res.UpdateAvailable {
		t.Error("UpdateAvailable = true, want false — both sides should normalize identically")
	}
}

func TestCheck_NoReleasesYetIsNotAnError(t *testing.T) {
	c, _ := checkerAgainst(t, "0.1.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	res := c.Check(context.Background())
	if res.UpdateAvailable {
		t.Error("UpdateAvailable = true, want false for a repo with no releases")
	}
	if res.LatestVersion != "" {
		t.Errorf("LatestVersion = %q, want empty", res.LatestVersion)
	}
}

func TestCheck_NetworkErrorIsNotFatal(t *testing.T) {
	c := NewChecker("yerassyldanay/xchats", "0.1.0")
	c.baseURL = "http://127.0.0.1:1" // nothing listens here
	res := c.Check(context.Background())
	if res.UpdateAvailable {
		t.Error("UpdateAvailable = true, want false when the check itself fails")
	}
	if res.CurrentVersion != "0.1.0" {
		t.Errorf("CurrentVersion = %q, want 0.1.0 even on failure", res.CurrentVersion)
	}
}

func TestCheck_CachesWithinCacheFor(t *testing.T) {
	c, calls := checkerAgainst(t, "0.1.0", releaseHandler("v0.2.0"))
	c.CacheFor = time.Hour

	c.Check(context.Background())
	c.Check(context.Background())
	c.Check(context.Background())
	if *calls != 1 {
		t.Errorf("GitHub was called %d times, want 1 (subsequent calls should hit the cache)", *calls)
	}
}

func TestCheck_RefetchesAfterCacheExpires(t *testing.T) {
	c, calls := checkerAgainst(t, "0.1.0", releaseHandler("v0.2.0"))
	c.CacheFor = time.Millisecond

	c.Check(context.Background())
	time.Sleep(10 * time.Millisecond)
	c.Check(context.Background())
	if *calls != 2 {
		t.Errorf("GitHub was called %d times, want 2 (cache should have expired)", *calls)
	}
}

func TestCheck_MalformedJSONIsNotFatal(t *testing.T) {
	c, _ := checkerAgainst(t, "0.1.0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not json`)
	})
	res := c.Check(context.Background())
	if res.UpdateAvailable {
		t.Error("UpdateAvailable = true, want false for an unparseable response")
	}
}
