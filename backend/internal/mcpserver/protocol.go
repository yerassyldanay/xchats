// Package mcpserver implements the MCP (Model Context Protocol) JSON-RPC
// surface for the Xchats knowledge base (plan/mcp.md §5): the 7 typed
// draft-upsert tools, the 4 shared knowledge tools, and the widget-oriented
// kb_media_upload tool, plus the resources/* calls the KB Manager widget
// resource needs. It never decides WHO is calling — internal/mcpauth
// resolves that and hands this package a Principal — and it never accepts a
// caller-supplied organization_id: every call is scoped to Principal.
// OrganizationID.
//
// Transport (the actual HTTP/SSE plumbing, and access-token verification)
// lives in internal/httpapi/mcp.go; this package is transport-agnostic —
// Handle takes one already-parsed JSON-RPC request and returns one response,
// so it is trivially unit-testable without an HTTP server.
package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

// protocolVersion is the MCP protocol date-version this server implements
// (the "2025-11-25" spec generation referenced throughout plan/mcp.md).
const protocolVersion = "2025-11-25"

// serverInstructions is returned from `initialize` and is the short
// rule-set plan/mcp.md §5 asks for: "The same short rules should also appear
// in the MCP server instructions so the model normally does not need to
// call [kb_info]."
const serverInstructions = `Xchats knowledge base connector.

Every write lands in the DRAFT only (kbd_draft) — never the live KB. A
human reviews and publishes the draft afterwards in Xchats; there is no
per-change approval here, and no tool publishes.

Before creating a new record, always check for an existing one:
  kb_summary → possible match → kb_read (exact key) → kb_*_upsert.
Exact key match updates that record. An exact-normalized-name match under a
DIFFERENT key is a conflict — call kb_read and update by that key instead.
A similar-but-not-exact name returns candidates — ask the user which record
they mean rather than creating a possible duplicate.

Natural keys: ref for products/tariffs/delivery zones, slug for topics,
"main" for the assistant/contacts/policies singletons. Internal database
UUIDs are never used as identity — only as media references returned by the
widget's upload flow.

Media fields take a material_id (a UUID string) returned by the KB Manager
widget's upload — never raw bytes. kb_media_upload is app-only: only the
widget calls it, not the model.

Every upsert accepts expected_draft_version (optimistic concurrency) and
provenance ({source_url?, material_ids?}). Omitted fields in "changes" stay
unchanged; an explicit null clears a clearable field.`

// Deps are the resources every tool handler needs.
type Deps struct {
	KB   *kbstore.Store
	Blob blob.Store
	Log  *slog.Logger
	// UploadBaseURL is the public base URL kb_media_upload's signed PUT
	// target is built against (e.g. https://xchats.kz).
	UploadBaseURL string
	// SignUpload mints a short-lived signed upload token for one material
	// (internal/httpapi owns the actual secret; mcpserver only calls this
	// seam so it never needs to import httpapi or hold the signing key
	// itself).
	SignUpload func(materialID string, expiresAt int64) string
	// UploadTTLSeconds bounds how long a signed upload URL stays valid.
	UploadTTLSeconds int
}

// Server dispatches JSON-RPC requests against Deps, scoped to a Principal
// resolved by internal/mcpauth for the current request.
type Server struct {
	Deps Deps
}

// New builds a Server.
func New(deps Deps) *Server { return &Server{Deps: deps} }

// Request is one parsed JSON-RPC 2.0 request/notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification is true for a request with no id — MCP/JSON-RPC
// notifications (e.g. notifications/initialized) get no response at all.
func (r Request) IsNotification() bool { return len(r.ID) == 0 }

// Response is one JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object. Codes follow the standard
// reserved ranges (-32700..-32600 parse/protocol errors); a tool that fails
// for a KB-domain reason (validation, conflict, ambiguity) is NOT an RPC
// error — it is a successful tools/call whose result carries isError:true
// (see handlers.go), so the model sees the reason as ordinary tool output
// instead of a transport-level failure it cannot recover from in-context.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

func errorResponse(id json.RawMessage, code int, message string) Response {
	return Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: message}}
}

func resultResponse(id json.RawMessage, result any) Response {
	return Response{JSONRPC: "2.0", ID: id, Result: result}
}

// Handle dispatches one JSON-RPC request. principal is nil only for the
// methods that must work before authentication in the MCP handshake sense —
// today there are none (the transport layer requires a verified Bearer
// token before Handle is ever called, per plan/mcp.md §3's 401 discovery
// flow), but the parameter stays explicit rather than reading a package
// global, so this package remains trivially testable.
func (s *Server) Handle(ctx context.Context, principal mcpauth.Principal, req Request) Response {
	switch req.Method {
	case "initialize":
		return resultResponse(req.ID, s.handleInitialize())
	case "notifications/initialized", "notifications/cancelled":
		return Response{} // caller must treat IsNotification as "send nothing"
	case "ping":
		return resultResponse(req.ID, map[string]any{})
	case "tools/list":
		return resultResponse(req.ID, s.handleToolsList())
	case "tools/call":
		return s.dispatchToolsCall(ctx, principal, req)
	case "resources/list":
		return resultResponse(req.ID, s.handleResourcesList())
	case "resources/templates/list":
		return resultResponse(req.ID, map[string]any{"resourceTemplates": []any{}})
	case "resources/read":
		return s.dispatchResourcesRead(req)
	default:
		return errorResponse(req.ID, codeMethodNotFound, "method not found: "+req.Method)
	}
}

func (s *Server) handleToolsList() map[string]any {
	return map[string]any{"tools": Tools()}
}

func (s *Server) handleInitialize() map[string]any {
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "xchats-kb",
			"version": "1.0.0",
		},
		"instructions": serverInstructions,
	}
}
