// Widget resource wiring (plan/mcp.md §6). The widget's actual UI logic
// lives in widget/kb-manager.html (a single self-contained HTML+JS document,
// embedded at build time); this file only exposes it over resources/list
// and resources/read.
//
// Wire-format caveat: exactly which _meta key (and which resource mimeType)
// each MCP host actually honors for widget association is the least stable
// part of this ecosystem as of this writing — ChatGPT's Apps SDK convention
// (_meta["openai/outputTemplate"]) and the general MCP Apps/UI extension are
// still converging. tools.go's widgetMeta() sets both a
// "openai/outputTemplate" and a generic "ui/resourceUri" key defensively.
// Hosts without any widget support still get every tool (plan/mcp.md §6:
// "Hosts without widget or file APIs still retain all model-facing tools");
// the fallback is the Xchats review page kb_info/every mutation result
// already links to. Treat this file as the seam to adjust once plan/mcp.md
// §9's MCP Inspector/ChatGPT/Claude interop pass runs against real hosts.
package mcpserver

import (
	_ "embed"
	"net/url"
	"strings"
)

// widgetResourceURI is the one reusable fullscreen widget resource
// (plan/mcp.md §6: "Use one reusable fullscreen resource").
const widgetResourceURI = "ui://xchats/kb-manager.html"

// widgetMimeType marks the resource as an MCP App per the standard
// "profile=mcp-app" parameter (plan Task 9), not a bare text/html a host
// might render as inert markup instead of an interactive widget.
const widgetMimeType = "text/html;profile=mcp-app"

//go:embed widget/kb-manager.html
var widgetHTML string

// originOf reduces a configured base URL to a bare scheme://host origin —
// the only shape a CSP domain list accepts. Returns "" for an empty or
// unparseable value (a test/dev server constructed without an upload base),
// which callers turn into an empty domain list rather than a bogus entry.
func originOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// widgetResourceMeta is the CSP metadata a host applies to the widget's
// iframe (plan Task 9).
//
// This USED to declare connect-src/frame-src 'none', on the stated premise
// that the widget "only ever talks to its host via postMessage/window.openai
// (no network calls of its own)". That premise was false from the moment the
// media upload landed: kb-manager.html PUTs file bytes straight to the signed
// target kb_media_upload returns (Deps.UploadBaseURL's origin), a real
// cross-origin request, and a host enforcing the declared policy blocked
// every upload before it left the iframe. Commit 0afda9b fixed the CORS half
// of the same failure; this is the CSP half.
//
// So the domain lists are DERIVED from the configured upload origin rather
// than hard-coded, and use the domain-list field names the component-resource
// contract actually reads — connectDomains/resourceDomains under the generic
// "ui" key, connect_domains/resource_domains under the legacy ChatGPT Apps
// SDK key — not raw CSP directive names, which nothing consumed. Same
// defensive-dual-key caveat as tools.go's widgetMeta.
//
// Only the upload origin is listed. The "Review and publish in Xchats" link
// is an <a href> navigation, which neither field governs, so FrontendBaseURL
// does not belong here.
func (s *Server) widgetResourceMeta() map[string]any {
	// Non-nil so an unconfigured server serializes [] rather than null — a
	// host reading null as "no policy object" would be a different bug.
	domains := []string{}
	if origin := originOf(s.publicBaseURL()); origin != "" {
		domains = append(domains, origin)
	}
	return map[string]any{
		"ui": map[string]any{"csp": map[string]any{
			"connectDomains":  domains,
			"resourceDomains": domains,
		}},
		"openai/widgetCSP": map[string]any{
			"connect_domains":  domains,
			"resource_domains": domains,
		},
	}
}

func (s *Server) handleResourcesList() map[string]any {
	return map[string]any{
		"resources": []map[string]any{{
			"uri":         widgetResourceURI,
			"name":        "KB Manager",
			"description": "All / Live / Draft / Record views over the Xchats knowledge base, with per-record media preview and upload.",
			"mimeType":    widgetMimeType,
			"_meta":       s.widgetResourceMeta(),
		}},
	}
}

func (s *Server) dispatchResourcesRead(req Request) Response {
	var params struct {
		URI string `json:"uri"`
	}
	if err := unmarshalParams(req.Params, &params); err != nil {
		return errorResponse(req.ID, codeInvalidParams, err.Error())
	}
	if params.URI != widgetResourceURI {
		return errorResponse(req.ID, codeInvalidParams, "unknown resource: "+params.URI)
	}
	return resultResponse(req.ID, map[string]any{
		"contents": []map[string]any{{
			"uri":      widgetResourceURI,
			"mimeType": widgetMimeType,
			"text":     widgetHTML,
			"_meta":    s.widgetResourceMeta(),
		}},
	})
}
