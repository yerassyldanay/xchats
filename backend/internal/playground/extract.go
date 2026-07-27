// Package playground is the Knowledge Base builder engine: it normalizes any
// dropped input (text/url/image/pdf/doc/video) into one common form (Stage 1 —
// ingest adapters), then synthesizes those materials into draft KB rows + popups
// (Stage 2 — the builder). The brain (internal/brain) reads only the published
// result; this package is the only writer, via internal/kbstore.
package playground

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
)

// VisionClient captions media bytes ("what it shows + when to send it"). It is the
// per-adapter upgrade seam: when nil, media falls back to a describe_media popup,
// so the pipeline is fully chat-driven with no vision dependency.
type VisionClient interface {
	Describe(ctx context.Context, mimetype string, data []byte) (string, error)
}

// Extractor runs Stage-1 ingest adapters. One adapter per source_type, each
// driving the input toward extracted_text or flagging needs_human (→ a popup).
type Extractor struct {
	Vision VisionClient // optional
	HTTP   *http.Client
	Log    *slog.Logger

	// AllowPrivateHosts disables the SSRF guard on the URL adapter (loopback /
	// private / link-local addresses are otherwise refused). Default false so the
	// server-side fetch can never be aimed at internal services or the cloud
	// metadata endpoint; tests/self-hosted setups that fetch localhost set it true.
	AllowPrivateHosts bool
}

// NewExtractor builds an Extractor with sane defaults. The URL adapter's HTTP
// client is SSRF-hardened: it refuses to dial non-public addresses (catching DNS
// rebinding and redirect hops alike) unless AllowPrivateHosts is set.
func NewExtractor(vision VisionClient, log *slog.Logger) *Extractor {
	if log == nil {
		log = slog.Default()
	}
	e := &Extractor{Vision: vision, Log: log}
	e.HTTP = e.safeHTTPClient(15 * time.Second)
	return e
}

// safeHTTPClient builds the URL-adapter client. A net.Dialer.Control hook vets the
// *resolved* IP of every connection (initial and each redirect), so a hostname that
// resolves to a private/loopback/link-local address — or rebinds to one mid-fetch —
// is refused before any bytes flow. Redirects are also capped and limited to http(s).
func (e *Extractor) safeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if e.AllowPrivateHosts {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("blocked non-public address %s", address)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("blocked redirect to %q scheme", req.URL.Scheme)
			}
			return nil
		},
	}
}

// isPublicIP reports whether an IP is safe to fetch from a server: not loopback,
// private (RFC1918 / ULA), link-local, unspecified or multicast.
func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast())
}

// Extract processes one staged material: it runs the right adapter, records the
// result on the material, and (when the adapter can't extract) raises a popup so a
// human supplies the text. It broadcasts progress so the UI updates live.
func (e *Extractor) Extract(ctx context.Context, kb *kbstore.Store, bs blob.Store, hub *realtime.Hub, orgID, materialID uuid.UUID) error {
	m, err := kb.GetMaterial(ctx, materialID)
	if err != nil {
		return err
	}

	var ex kbstore.MaterialExtraction
	switch m.SourceType {
	case "text":
		ex = kbstore.MaterialExtraction{Status: "ready", ExtractedText: m.ExtractedText,
			Extraction: `{"method":"passthrough"}`}
	case "url":
		ex = e.extractURL(ctx, m.SourceRef, kbstore.OperatorComment(m.Extraction))
	case "image", "pdf", "doc":
		ex = e.extractMedia(ctx, bs, m)
	default: // video, audio, anything else → operator describes it
		ex = kbstore.MaterialExtraction{Status: "needs_human",
			Extraction: `{"method":"none","reason":"no auto-extraction for this type"}`}
	}

	if err := kb.UpdateMaterialExtraction(ctx, materialID, ex); err != nil {
		return err
	}
	if ex.Status == "needs_human" {
		prompt := "Опишите, что это и когда отправлять."
		if m.SourceType == "url" {
			prompt = "Не удалось прочитать ссылку — вставьте текст или приложите скриншот."
		}
		req, rerr := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{
			MaterialID: &materialID,
			ReqType:    "describe_media",
			Prompt:     prompt,
			Context:    jsonObj(map[string]any{"source_type": m.SourceType, "source_ref": m.SourceRef}),
			Target:     jsonObj(map[string]any{"material_id": materialID.String()}),
		})
		if rerr != nil {
			e.Log.Warn("raise describe_media failed", "err", rerr)
		} else if hub != nil {
			hub.Broadcast("kb.request.created", map[string]any{"id": req.ID, "req_type": req.ReqType})
		}
	}
	if hub != nil {
		hub.Broadcast("kb.material.updated", map[string]any{"material_id": materialID, "status": ex.Status})
	}
	return nil
}

// extractURL does a best-effort server-side fetch → readable text. On any failure
// (bad scheme, non-200, empty, blocked address) it falls back to the operator's
// comment if one was staged with the material (ready, method "operator_fallback");
// with no comment it flags needs_human → the paste/screenshot popup. The SSRF
// guard lives in the client's dialer (see safeHTTPClient); the scheme is rejected
// here, before any request is built.
func (e *Extractor) extractURL(ctx context.Context, rawURL, comment string) kbstore.MaterialExtraction {
	fallback := func(reason string) kbstore.MaterialExtraction {
		if strings.TrimSpace(comment) != "" {
			return kbstore.MaterialExtraction{Status: "ready", ExtractedText: comment,
				Extraction: jsonObj(map[string]any{"method": "operator_fallback", "error": reason})}
		}
		return needsHuman(reason)
	}
	if u, perr := url.Parse(rawURL); perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fallback("unsupported url (only http/https)")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fallback("bad url")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; xchats-kb/1.0)")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return fallback(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fallback(fmt.Sprintf("http %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return fallback(err.Error())
	}
	text := htmlToText(string(body))
	if strings.TrimSpace(text) == "" {
		return fallback("empty/JS-only page")
	}
	return kbstore.MaterialExtraction{Status: "ready", ExtractedText: text,
		Extraction: `{"method":"http_fetch"}`}
}

// extractMedia captions media via the vision client; with no client it defers to
// the operator (describe_media), keeping the pipeline chat-driven.
func (e *Extractor) extractMedia(ctx context.Context, bs blob.Store, m kbstore.Material) kbstore.MaterialExtraction {
	if e.Vision == nil || m.BlobID == "" {
		return needsHuman("no vision extractor")
	}
	data, meta, err := bs.Get(m.BlobID)
	if err != nil {
		return needsHuman("blob missing")
	}
	caption, err := e.Vision.Describe(ctx, meta.Mimetype, data)
	if err != nil || strings.TrimSpace(caption) == "" {
		return needsHuman("vision failed")
	}
	return kbstore.MaterialExtraction{Status: "ready", ExtractedText: caption,
		MediaKind: m.MediaKind, Extraction: `{"method":"vision"}`}
}

func needsHuman(reason string) kbstore.MaterialExtraction {
	return kbstore.MaterialExtraction{Status: "needs_human",
		Extraction: jsonObj(map[string]any{"method": "none", "error": reason})}
}

var (
	tagRE   = regexp.MustCompile(`(?s)<(script|style)[^>]*>.*?</(script|style)>`)
	anyTag  = regexp.MustCompile(`<[^>]+>`)
	wsRE    = regexp.MustCompile(`[ \t]*\n[ \t\n]*`)
	spaceRE = regexp.MustCompile(`[ \t]{2,}`)
)

// htmlToText strips scripts/styles/tags and collapses whitespace — a best-effort
// readable-text extraction (a headless renderer is a later per-adapter upgrade).
func htmlToText(html string) string {
	s := tagRE.ReplaceAllString(html, " ")
	s = anyTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = spaceRE.ReplaceAllString(s, " ")
	s = wsRE.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}
