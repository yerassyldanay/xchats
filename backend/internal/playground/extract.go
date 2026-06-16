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
	"net/http"
	"regexp"
	"strings"
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
}

// NewExtractor builds an Extractor with sane defaults.
func NewExtractor(vision VisionClient, log *slog.Logger) *Extractor {
	if log == nil {
		log = slog.Default()
	}
	return &Extractor{Vision: vision, HTTP: &http.Client{Timeout: 15 * time.Second}, Log: log}
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
		ex = e.extractURL(ctx, m.SourceRef)
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
		if _, rerr := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{
			MaterialID: &materialID,
			ReqType:    "describe_media",
			Prompt:     prompt,
			Context:    fmt.Sprintf(`{"source_type":%q,"source_ref":%q}`, m.SourceType, m.SourceRef),
			Target:     fmt.Sprintf(`{"material_id":%q}`, materialID),
		}); rerr != nil {
			e.Log.Warn("raise describe_media failed", "err", rerr)
		}
	}
	if hub != nil {
		hub.Broadcast("kb.material.updated", map[string]any{"material_id": materialID, "status": ex.Status})
	}
	return nil
}

// extractURL does a best-effort server-side fetch → readable text. On any failure
// (non-200, empty, blocked) it flags needs_human → the paste/screenshot popup.
func (e *Extractor) extractURL(ctx context.Context, url string) kbstore.MaterialExtraction {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return needsHuman("bad url")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; xchats-kb/1.0)")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return needsHuman(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return needsHuman(fmt.Sprintf("http %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return needsHuman(err.Error())
	}
	text := htmlToText(string(body))
	if strings.TrimSpace(text) == "" {
		return needsHuman("empty/JS-only page")
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
		Extraction: fmt.Sprintf(`{"method":"none","error":%q}`, reason)}
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
