// Package safefetch is the SSRF-hardened HTTP client shared by every fetch
// surface that accepts a remote URL: the native URL extractor, embedded-image
// downloads, and OAuth Client ID Metadata Document fetches. These are separate
// trust boundaries that share the same network-level protections.
//
// Layered defence: CheckURL is a synchronous, best-effort pre-check — reject
// an obviously-internal host at submit time (a 400 before any async job
// runs), not the actual security boundary. Client's net.Dialer.Control hook
// is the real boundary: it re-vets the RESOLVED address of every connection
// attempt, including each redirect hop, so a hostname that resolves
// differently between CheckURL's lookup and the transport's own DNS
// resolution (DNS rebinding) is still refused before any bytes flow.
package safefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// ErrBlockedHost means a URL's host — at CheckURL time, or at actual
// connect/redirect time inside a Client built by this package — resolved to
// a non-public address (loopback, private, link-local, unspecified, or
// multicast) and was refused before any bytes were exchanged with it.
var ErrBlockedHost = errors.New("safefetch: blocked non-public host")

// ErrTooLarge means a response exceeded the caller's declared maxBytes cap —
// either its Content-Length header alone, or the actual body once streamed
// (an absent or understated Content-Length, e.g. chunked transfer, cannot be
// trusted and is caught the same way).
var ErrTooLarge = errors.New("safefetch: response exceeds the maximum allowed size")

// dialTimeout bounds the TCP handshake itself, independent of the overall
// per-request Timeout a Client caller supplies.
const dialTimeout = 10 * time.Second

// maxRedirects is enough to follow a normal canonical-URL/tracking-redirect
// chain, never enough to be useful as an amplification or loop vector.
const maxRedirects = 5

// Client builds an SSRF-hardened *http.Client: a net.Dialer.Control hook
// vets the RESOLVED IP of every connection — the initial one and each
// redirect hop — refusing a private/loopback/link-local/unspecified/
// multicast address before any bytes flow, and CheckRedirect additionally
// requires every redirect target to be https and caps the chain at
// maxRedirects. allowPrivate is the single escape hatch (config's
// KBAllowPrivateFetch): true only for a trusted deployment deliberately
// fetching its own loopback tooling — false in every real deployment.
func Client(allowPrivate bool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: dialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("%w: %s", ErrBlockedHost, address)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: checkRedirectPolicy,
	}
}

// checkRedirectPolicy is Client's CheckRedirect rule, factored out so it can
// be exercised directly against a synthetic redirect chain — testing the
// >5-redirects and https-only rules through a real multi-hop TLS server
// chain would need a trusted certificate for every hop, which buys nothing
// over testing the pure decision function itself.
func checkRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("safefetch: stopped after %d redirects", maxRedirects)
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("safefetch: blocked redirect to %q scheme", req.URL.Scheme)
	}
	return nil
}

// CheckURL validates rawURL before it is handed to ANY fetch surface —
// the native extractor's own client, or a vendor API (Firecrawl/LlamaParse)
// that will fetch it from a DIFFERENT network on our behalf. It requires an
// http(s) scheme, a non-empty host, and — unless allowPrivate — that every
// address the host currently resolves to is public.
//
// This is a synchronous, out-of-band lookup: it does NOT itself protect
// against DNS rebinding (a hostname resolving differently a moment later, at
// actual connect time) — that is Client's net.Dialer.Control hook's job, run
// again on every real connection attempt. CheckURL exists so an obviously
// bad submission (a literal 127.0.0.1, a hostname that only ever resolves
// privately) fails fast and synchronously at submit time instead of only
// surfacing later as an opaque async extraction failure. It is also the
// only layer that can meaningfully warn about a VENDOR fetch (Firecrawl/
// LlamaParse never touch this process's own network, so Client's dialer
// hook does not apply to them at all) — a vendor pointed at a host that is
// private only on ITS OWN network is a residual risk this cannot see.
func CheckURL(ctx context.Context, rawURL string, allowPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("safefetch: invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("safefetch: unsupported scheme %q — only http/https URLs are allowed", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("safefetch: URL has no host")
	}
	if allowPrivate {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("%w: %s", ErrBlockedHost, host)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("safefetch: resolve host %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("safefetch: host %q did not resolve to any address", host)
	}
	for _, a := range addrs {
		if !isPublicIP(a.IP) {
			return fmt.Errorf("%w: %s resolves to %s", ErrBlockedHost, host, a.IP)
		}
	}
	return nil
}

// Get performs one GET against rawURL through client (build one with
// Client), capping the response to maxBytes: a declared Content-Length over
// the cap is rejected before any body byte is read, and the body read
// itself is bounded by an io.LimitReader one byte past the cap so an
// absent or understated Content-Length (chunked transfer, or a server that
// simply lies) cannot bypass the limit either way. The returned *http.Response
// has its Body already fully drained and closed — safe to inspect
// StatusCode/Header, never to read Body again.
func Get(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, *http.Response, error) {
	return get(ctx, client, rawURL, maxBytes, "")
}

// GetJSON is Get with an explicit JSON response preference.
func GetJSON(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, *http.Response, error) {
	return get(ctx, client, rawURL, maxBytes, "application/json")
}

func get(ctx context.Context, client *http.Client, rawURL string, maxBytes int64, accept string) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.ContentLength > 0 && resp.ContentLength > maxBytes {
		return nil, resp, fmt.Errorf("%w: content-length %d exceeds %d bytes", ErrTooLarge, resp.ContentLength, maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, resp, err
	}
	if int64(len(data)) > maxBytes {
		return nil, resp, ErrTooLarge
	}
	return data, resp, nil
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast())
}
