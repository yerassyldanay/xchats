package httpapi_test

// cors_test.go guards a real bug a user hit: approving a Playground draft
// from a browser where the frontend and backend are on different origins
// failed with a CORS error, because If-Match (the draft store's
// optimistic-concurrency header — see stores/playground.ts's ifMatch())
// was never added to Access-Control-Allow-Headers when it was introduced.
// Go's httptest never exercises real browser CORS preflight enforcement, so
// nothing else in this suite would have caught it.

import (
	"net/http"
	"strings"
	"testing"
)

func TestCORS_PreflightAllowsIfMatchHeader(t *testing.T) {
	h := newHarness(t)

	req, err := http.NewRequest(http.MethodOptions, h.srv.URL+"/xchats/api/v1/playground/draft/approve", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "*")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "If-Match")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()

	allowed := resp.Header.Get("Access-Control-Allow-Headers")
	found := false
	for _, h := range strings.Split(allowed, ",") {
		if strings.EqualFold(strings.TrimSpace(h), "If-Match") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Access-Control-Allow-Headers=%q must include If-Match, or a cross-origin approve is blocked by the browser before it's ever sent", allowed)
	}
}
