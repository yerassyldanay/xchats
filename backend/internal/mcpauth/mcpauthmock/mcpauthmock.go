// Package mcpauthmock is a drop-in for mcpauth.FetchCIMD (Client ID
// Metadata Document discovery) for cmd/xchats' mock-externals mode.
package mcpauthmock

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

// FetchCIMD answers any https:// client_id URL with a realistic, always-
// valid client metadata document, never dialing out — pass it to
// mcpauth.SetCIMDFetcher in the mock-externals composition root.
func FetchCIMD(ctx context.Context, clientIDURL string) (mcpauth.Client, error) {
	if err := ctx.Err(); err != nil {
		return mcpauth.Client{}, err
	}
	return mcpauth.Client{
		ClientID:     clientIDURL,
		ClientName:   "Mock MCP Client",
		RedirectURIs: []string{clientIDURL},
		Source:       "cimd",
	}, nil
}
