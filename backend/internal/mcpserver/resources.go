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
)

// widgetResourceURI is the one reusable fullscreen widget resource
// (plan/mcp.md §6: "Use one reusable fullscreen resource").
const widgetResourceURI = "ui://xchats/kb-manager.html"

//go:embed widget/kb-manager.html
var widgetHTML string

func (s *Server) handleResourcesList() map[string]any {
	return map[string]any{
		"resources": []map[string]any{{
			"uri":         widgetResourceURI,
			"name":        "KB Manager",
			"description": "All / Live / Draft / Record / Media / Publish views over the Xchats knowledge base.",
			"mimeType":    "text/html",
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
			"mimeType": "text/html",
			"text":     widgetHTML,
		}},
	})
}
