package tunnel

import (
	"context"
	"errors"
	"net"
	"strings"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

// connectOptions is everything realConnect (or a test double) needs to
// open one tunnel.
type connectOptions struct {
	Authtoken string
	Region    string // "" lets the ngrok SDK pick the fastest region
	Domain    string // "" means an ngrok-assigned random subdomain
}

// ngrokTunnel is the exact slice of the real ngrok SDK's Tunnel interface
// this package needs — a net.Listener plus its public URL — so a test can
// fake one without standing up the SDK's own Session/Tunnel machinery. A
// real ngrok.Tunnel satisfies this structurally, with zero adapter code.
type ngrokTunnel interface {
	net.Listener
	URL() string
}

// connectFn opens one ngrok session and tunnel — realConnect in
// production, a test double in lifecycle_test.go.
type connectFn func(ctx context.Context, opts connectOptions) (ngrokTunnel, error)

// realConnect authenticates an ngrok session with opts.Authtoken and opens
// an HTTP tunnel on it, reserving opts.Domain when one is set. The session
// itself is intentionally not exposed beyond this call: Manager only ever
// needs the resulting Tunnel (a net.Listener) to serve HTTP over, and
// closing that Tunnel is sufficient to tear down the session's one leg —
// nothing here ever creates a second tunnel on the same session for Stop
// to have to track separately.
func realConnect(ctx context.Context, opts connectOptions) (ngrokTunnel, error) {
	connectOpts := []ngrok.ConnectOption{ngrok.WithAuthtoken(opts.Authtoken)}
	if opts.Region != "" {
		connectOpts = append(connectOpts, ngrok.WithRegion(opts.Region))
	}
	sess, err := ngrok.Connect(ctx, connectOpts...)
	if err != nil {
		return nil, err
	}

	var endpointOpts []config.HTTPEndpointOption
	if opts.Domain != "" {
		endpointOpts = append(endpointOpts, config.WithDomain(opts.Domain))
	}
	tun, err := sess.Listen(ctx, config.HTTPEndpoint(endpointOpts...))
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	return tun, nil
}

// classifyConnectError distinguishes an ngrok-service-rejected connect (the
// service returned a structured, coded error — overwhelmingly an invalid,
// expired, or revoked authtoken, since Connect's entire job is session
// auth) from every other failure (DNS, network unreachable, a timeout,
// ...), which says nothing about whether the token itself is any good.
// message is always safe to store/display as-is — see sanitizeMessage for
// the additional defense-in-depth pass callers apply on top of it.
func classifyConnectError(err error) (reason, message string) {
	var ngrokErr ngrok.Error
	if errors.As(err, &ngrokErr) {
		return "auth", ngrokErr.Msg()
	}
	return "unreachable", err.Error()
}

// sanitizeMessage strips any literal occurrence of token out of msg before
// it is ever stored in Status.LastError or logged — defense in depth on
// top of classifyConnectError already preferring ngrokErr.Msg() (the
// service's own message, which never echoes back what we sent it) over the
// raw error for the one case that matters most. Mirrors
// internal/tgpoller's sanitizeForStorage.
func sanitizeMessage(msg, token string) string {
	if token == "" {
		return msg
	}
	return strings.ReplaceAll(msg, token, "[REDACTED]")
}
