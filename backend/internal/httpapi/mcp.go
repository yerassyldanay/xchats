package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
)

// mcpRequestBodyLimit bounds one JSON-RPC POST body — every real MCP tool
// call here is small (KB text fields, media references as UUID strings);
// there is no legitimate reason for a multi-megabyte request.
const mcpRequestBodyLimit = 2 << 20 // 2 MiB

// handleMCP is the single POST /mcp entry point (plan/mcp.md §3): verify the
// bearer access token (never accepted as a query parameter — only the
// Authorization header), re-check the bound tenant is still live, then
// dispatch the JSON-RPC body (a single request or a batch array) to
// internal/mcpserver.
func (s *Server) handleMCP(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	if !s.validateMCPTransport(c) {
		return
	}
	if accept := c.GetHeader("Accept"); accept != "" && !acceptsJSON(accept) {
		fail(c, http.StatusNotAcceptable, ErrValidation, "Accept must include application/json")
		return
	}
	principal, ok := s.authenticateMCPRequest(c)
	if !ok {
		return // response already written (401 + WWW-Authenticate, or 403)
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, mcpRequestBodyLimit+1))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "unreadable body")
		return
	}
	if len(body) > mcpRequestBodyLimit {
		fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "request body too large")
		return
	}

	// A batch is a JSON array; a single call is a JSON object. Try array
	// first (a lone '[' is unambiguous) so a single-request body — the
	// overwhelming common case — still only needs the cheap non-batch path.
	trimmed := strings.TrimLeft(string(body), " \t\r\n")
	if strings.HasPrefix(trimmed, "[") {
		s.handleMCPBatch(c, principal, body)
		return
	}
	s.handleMCPSingle(c, principal, body)
}

func (s *Server) handleMCPSingle(c *gin.Context, principal mcpauth.Principal, body []byte) {
	var req mcpserver.Request
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, mcpserver.Response{JSONRPC: "2.0", Error: &mcpserver.RPCError{Code: -32700, Message: "parse error"}})
		return
	}
	resp := s.mcpServer.Handle(ctx(c), principal, req)
	if req.IsNotification() {
		c.Status(http.StatusAccepted)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleMCPBatch(c *gin.Context, principal mcpauth.Principal, body []byte) {
	var reqs []mcpserver.Request
	if err := json.Unmarshal(body, &reqs); err != nil {
		c.JSON(http.StatusBadRequest, mcpserver.Response{JSONRPC: "2.0", Error: &mcpserver.RPCError{Code: -32700, Message: "parse error"}})
		return
	}
	if len(reqs) == 0 {
		c.JSON(http.StatusBadRequest, mcpserver.Response{JSONRPC: "2.0", Error: &mcpserver.RPCError{Code: -32600, Message: "empty batch"}})
		return
	}
	var out []mcpserver.Response
	for _, req := range reqs {
		resp := s.mcpServer.Handle(ctx(c), principal, req)
		if !req.IsNotification() {
			out = append(out, resp)
		}
	}
	if len(out) == 0 {
		c.Status(http.StatusAccepted)
		return
	}
	c.JSON(http.StatusOK, out)
}

// handleMCPGet answers the SSE-stream half of Streamable HTTP (plan Task 10)
// with an explicit, spec-compliant 405: this server never sends
// server-initiated messages — every KB tool is plain request/response, with
// nothing an SSE stream would ever carry — and the transport spec allows
// exactly this ("or reject with 405") instead of requiring a stream with
// nothing to send through it.
func (s *Server) handleMCPGet(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	if !s.validateMCPTransport(c) {
		return
	}
	c.Header("Allow", "POST, DELETE")
	fail(c, http.StatusMethodNotAllowed, ErrValidation, "this server does not support the SSE stream (GET); use POST")
}

// handleMCPDelete acknowledges the client closing its MCP transport/session
// (plan Task 10: "shutdown is closing the transport ... not a JSON-RPC
// shutdown method" — deliberately not inventing one). This server keeps no
// session state of its own to tear down: every /mcp request is already
// fully self-contained, the bearer token IS the entire auth/scope state
// (see authenticateMCPRequest) — so this is an authenticated
// acknowledgment, not state cleanup.
func (s *Server) handleMCPDelete(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	if !s.validateMCPTransport(c) {
		return
	}
	if _, ok := s.authenticateMCPRequest(c); !ok {
		return
	}
	c.Status(http.StatusOK)
}

// mcpAllowedOrigin reports whether origin is acceptable for the MCP
// transport (plan Task 10's DNS-rebinding guard) — reusing the SAME
// CORSOrigins allowlist s.cors() already trusts, rather than a second list
// to keep in sync. A request with NO Origin header (the common case: an MCP
// host calls this endpoint server-to-server, not from browser JS) is always
// allowed — Origin exists to guard against a BROWSER-originated request,
// and a missing header means this isn't one.
func (s *Server) mcpAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	for _, o := range s.cfg.CORSOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

// validateMCPTransport runs the Streamable HTTP checks common to every
// method on /mcp (plan Task 10): a forged/unlisted Origin (DNS rebinding),
// and an MCP-Protocol-Version the client declares that this server — which
// implements exactly one protocol version — cannot satisfy. Returns false
// once it has already written a response; callers must return immediately
// in that case. On success it echoes MCP-Protocol-Version back so a host
// can confirm what it's actually talking to.
func (s *Server) validateMCPTransport(c *gin.Context) bool {
	if origin := c.GetHeader("Origin"); !s.mcpAllowedOrigin(origin) {
		fail(c, http.StatusForbidden, ErrValidation, "origin not allowed")
		return false
	}
	if v := c.GetHeader("MCP-Protocol-Version"); v != "" && v != mcpserver.ProtocolVersion {
		fail(c, http.StatusBadRequest, ErrValidation, "unsupported MCP-Protocol-Version "+v+" (this server implements "+mcpserver.ProtocolVersion+")")
		return false
	}
	c.Header("MCP-Protocol-Version", mcpserver.ProtocolVersion)
	return true
}

// acceptsJSON reports whether an Accept header includes application/json
// (or a wildcard covering it) — the only content type /mcp ever responds
// with.
func acceptsJSON(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		mt := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mt == "application/json" || mt == "application/*" || mt == "*/*" {
			return true
		}
	}
	return false
}

// authenticateMCPRequest verifies the bearer token cryptographically, then
// re-checks the bound organization membership against the live tables
// (plan/mcp.md §3) — a token that verifies but names a since-removed
// membership is rejected here, not trusted from its own claims.
func (s *Server) authenticateMCPRequest(c *gin.Context) (mcpauth.Principal, bool) {
	authz := c.GetHeader("Authorization")
	token, ok := strings.CutPrefix(authz, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		s.unauthorizedMCP(c)
		return mcpauth.Principal{}, false
	}
	principal, err := s.mcpAuth.VerifyAccessToken(ctx(c), token)
	if err != nil {
		s.unauthorizedMCP(c)
		return mcpauth.Principal{}, false
	}
	inOrg, err := s.store.UserInOrg(ctx(c), principal.UserID, principal.OrganizationID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "tenant check failed")
		return mcpauth.Principal{}, false
	}
	if !inOrg {
		s.unauthorizedMCP(c)
		return mcpauth.Principal{}, false
	}
	return principal, true
}
