// Package ngrokmock is an in-memory ngrok account-discovery client for
// cmd/xchats' mock-externals mode — it structurally satisfies cmd/xchats'
// own unexported ngrokDomainLister interface (Go interfaces are structural,
// so this package need not import cmd/xchats at all) in place of
// ngrokapi.NewClient(), so applyNgrokPublicOrigin's discovery step never
// dials the real ngrok API, whether or not a real ngrok API key happens to
// already be sitting in the credential store from prior real use.
package ngrokmock

import (
	"context"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/ngrokapi"
)

// Domain is the fixed reserved domain every mock account "owns" — shaped
// like a real ngrok free-tier reserved domain so applyNgrokPublicOrigin's
// own validation (must be a bare hostname, no scheme/port/userinfo) and
// ngrokapi.SelectDefaultDomain's own tie-breaking both accept it exactly
// like a real one would.
const Domain = "mock-tunnel.ngrok-free.app"

// DomainLister is a ready-to-use, stateless mock — a zero value works.
type DomainLister struct{}

// New returns a ready-to-use mock domain lister.
func New() DomainLister { return DomainLister{} }

func (DomainLister) ListReservedDomains(ctx context.Context, apiKey string) ([]ngrokapi.ReservedDomain, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []ngrokapi.ReservedDomain{{
		ID: "rd_mock1", Domain: Domain, CreatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
	}}, nil
}
