package httpapi_test

// mcp_production_hardening_test.go covers the remaining piece of Task 17
// exercisable without spinning up a second process/replica: discovery
// output must be derived ONLY from the configured base URLs
// (cfg.Server.APIBaseURL / cfg.Server.FrontendBaseURL), never from a request's Host or
// X-Forwarded-* headers — those are attacker-controlled on any request that
// didn't actually arrive through a configured trusted proxy. A forged Host
// plus an untrusted X-Forwarded-Proto must leave every discovery response
// byte-identical to an unforged request.
//
// (The other two Task 17 checks — an ephemeral-key/localhost-URL startup
// failure, and persistent-key survival across a restart/multiple replicas —
// live in cmd/xchats's production_config_test.go and mcpauth's
// jwt_persistence_test.go respectively, where the relevant code actually is.)

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMCPDiscovery_ForgedHostAndForwardedHeadersDoNotAffectOutput(t *testing.T) {
	h := newMCPHarness(t)

	baseline := map[string]any{}
	h.get200Into(t, "/.well-known/oauth-authorization-server", &baseline)

	req, err := http.NewRequest(http.MethodGet, h.srv.URL+"/.well-known/oauth-authorization-server", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// A direct, unproxied caller can set any of these; none is in this
	// server's (empty-by-default) trusted-proxy list, so none must matter.
	req.Host = "attacker.evil.example"
	req.Header.Set("X-Forwarded-Host", "attacker.evil.example")
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("forged request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("forged request status=%d body=%s", resp.StatusCode, b)
	}
	var forged map[string]any
	if err := json.Unmarshal(b, &forged); err != nil {
		t.Fatalf("decode: %v body=%s", err, b)
	}

	for _, key := range []string{"issuer", "authorization_endpoint", "token_endpoint", "registration_endpoint", "jwks_uri"} {
		if forged[key] != baseline[key] {
			t.Fatalf("forged Host/X-Forwarded-* changed %s: got %v, want unchanged %v (attacker-controlled headers must never drive discovery output)", key, forged[key], baseline[key])
		}
		if got, _ := forged[key].(string); got == "" || strings.Contains(got, "attacker.evil.example") {
			t.Fatalf("%s=%q leaked the forged host", key, got)
		}
	}
}
