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
