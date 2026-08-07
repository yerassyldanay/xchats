package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

// mcpConnectionInfo is the session-level (non-admin) view of how to connect
// an MCP client (ChatGPT, Claude) to this deployment. It intentionally
// exposes less than GET /settings/tunnel (no last_error, no started_at) —
// every logged-in user can see enough to paste a URL into a connector, but
// diagnosing a broken tunnel stays an admin-only concern on /settings.
type mcpConnectionInfo struct {
	MCPURL          string   `json:"mcp_url"`
	AuthEnabled     bool     `json:"auth_enabled"`
	TunnelAvailable bool     `json:"tunnel_available"`
	TunnelRunning   bool     `json:"tunnel_running"`
	PublicURL       string   `json:"public_url,omitempty"`
	Scopes          []string `json:"scopes"`
}

// handleMCPConnectionInfo answers "what URL do I paste into ChatGPT/Claude"
// for any authenticated user, admin or member — configuring the tunnel
// itself still requires admin (see settings.go's tunnel handlers), but
// seeing the resulting URL and connector state does not.
func (s *Server) handleMCPConnectionInfo(c *gin.Context) {
	info := mcpConnectionInfo{
		MCPURL:      s.cfg.MCPResourceURL(),
		AuthEnabled: s.mcpAuthEnabled(),
		Scopes:      mcpauth.AllScopes,
	}
	if s.tunnel != nil {
		info.TunnelAvailable = true
		status := s.tunnel.Status()
		info.TunnelRunning = status.Running
		if status.Running && status.PublicURL != "" {
			info.PublicURL = status.PublicURL
			info.MCPURL = strings.TrimRight(status.PublicURL, "/") + "/mcp"
		}
	}
	ok(c, info)
}
