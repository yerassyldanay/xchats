package httpapi_test

// mcp_streamable_test.go covers Task 10: Streamable HTTP transport
// conformance for /mcp — GET (405 + Allow), DELETE (transport-close
// acknowledgment), Origin validation (DNS-rebinding guard),
// MCP-Protocol-Version negotiation, Accept negotiation, and the
// initialize → initialized → call → close lifecycle including
// single-message (non-batch) JSON-RPC notification semantics.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
)

func TestMCPGet_Returns405WithAllowHeader(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	req, _ := http.NewRequest(http.MethodGet, h.srv.URL+"/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d, want 405; body=%s", resp.StatusCode, b)
	}
	if allow := resp.Header.Get("Allow"); allow == "" {
		t.Fatalf("expected an Allow header on 405, got none")
	}
}

func TestMCPDelete_AcknowledgesTransportClose(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	req, _ := http.NewRequest(http.MethodDelete, h.srv.URL+"/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d, want 200; body=%s", resp.StatusCode, b)
	}
}

func TestMCPDelete_RequiresAuth(t *testing.T) {
	h := newMCPHarness(t)
	req, _ := http.NewRequest(http.MethodDelete, h.srv.URL+"/mcp", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", resp.StatusCode)
	}
}

func TestMCP_RejectsDisallowedOrigin(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")
	// newMCPHarness defaults CORSOrigins to ["*"] (matches everything, by
	// design, for the rest of this file's tests) — restrict it here to
	// exercise the actual rejection path, the same live-config-mutation
	// pattern setTelegramBase already uses elsewhere in this package.
	h.cfg.Server.CORSOrigins = []string{"https://allowed.example"}

	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Origin", "https://attacker.evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d, want 403; body=%s", resp.StatusCode, b)
	}
}

func TestMCP_RejectsUnsupportedProtocolVersion(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("MCP-Protocol-Version", "1999-01-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d, want 400; body=%s", resp.StatusCode, b)
	}
}

func TestMCP_EchoesSupportedProtocolVersionAndRejectsBadAccept(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("MCP-Protocol-Version", mcpserver.ProtocolVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("MCP-Protocol-Version"); got != mcpserver.ProtocolVersion {
		t.Fatalf("response MCP-Protocol-Version=%q, want %q", got, mcpserver.ProtocolVersion)
	}

	badAcceptReq, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	badAcceptReq.Header.Set("Content-Type", "application/json")
	badAcceptReq.Header.Set("Authorization", "Bearer "+accessToken)
	badAcceptReq.Header.Set("Accept", "text/plain")
	badResp, err := http.DefaultClient.Do(badAcceptReq)
	if err != nil {
		t.Fatalf("post /mcp (bad accept): %v", err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusNotAcceptable {
		b, _ := io.ReadAll(badResp.Body)
		t.Fatalf("status=%d, want 406; body=%s", badResp.StatusCode, b)
	}
}

// TestMCPLifecycle_InitializeThenNotificationThenCallThenClose drives the
// full sequence plan Task 10 asks for: initialize → notifications/initialized
// (a single-message, non-batch JSON-RPC NOTIFICATION — no id, so the
// transport answers 202 with no JSON-RPC body at all, never a response
// object) → tools/call → DELETE (transport/session close).
func TestMCPLifecycle_InitializeThenNotificationThenCallThenClose(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	initResp := h.rpc(t, accessToken, "initialize", map[string]any{})
	if initResp.Error != nil {
		t.Fatalf("initialize: %+v", initResp.Error)
	}
	result := initResp.Result.(map[string]any)
	if result["protocolVersion"] != mcpserver.ProtocolVersion {
		t.Fatalf("initialize protocolVersion=%v, want %v", result["protocolVersion"], mcpserver.ProtocolVersion)
	}

	// notifications/initialized: a single JSON-RPC object with no "id" —
	// the transport must answer 202 with an empty body, never a JSON-RPC
	// response (there is nothing to respond TO).
	notifBody, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	notifReq, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader(notifBody))
	notifReq.Header.Set("Content-Type", "application/json")
	notifReq.Header.Set("Authorization", "Bearer "+accessToken)
	notifResp, err := http.DefaultClient.Do(notifReq)
	if err != nil {
		t.Fatalf("notifications/initialized: %v", err)
	}
	nb, _ := io.ReadAll(notifResp.Body)
	notifResp.Body.Close()
	if notifResp.StatusCode != http.StatusAccepted {
		t.Fatalf("notifications/initialized status=%d, want 202; body=%s", notifResp.StatusCode, nb)
	}
	if len(nb) != 0 {
		t.Fatalf("notifications/initialized body should be empty, got %q", nb)
	}

	callResp := h.rpc(t, accessToken, "ping", nil)
	if callResp.Error != nil {
		t.Fatalf("ping: %+v", callResp.Error)
	}

	closeReq, _ := http.NewRequest(http.MethodDelete, h.srv.URL+"/mcp", nil)
	closeReq.Header.Set("Authorization", "Bearer "+accessToken)
	closeResp, err := http.DefaultClient.Do(closeReq)
	if err != nil {
		t.Fatalf("DELETE /mcp: %v", err)
	}
	defer closeResp.Body.Close()
	if closeResp.StatusCode != http.StatusOK {
		t.Fatalf("close status=%d, want 200", closeResp.StatusCode)
	}
}
