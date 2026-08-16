package kbimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/extractor"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
)

// downloadEmbeddedImages fetches and stages up to Config.MaxImagesPerJob
// distinct images an extraction Result referenced for ONE parent material,
// deduped by absolute URL, each capped at Config.MaxImageBytes and drawing
// down runBudget (the WHOLE RUN's remaining Config.MaxRunImageBytes,
// owned and mutated by the caller across every material in the run — this
// is called once per material, not once per run). A byproduct worth
// stating plainly: every staged image is an ordinary org material and
// shows up in the Файлы tab even if pass 2 never references it.
func (s *Service) downloadEmbeddedImages(ctx context.Context, orgID, runID, parentID uuid.UUID, images []extractor.Image, runBudget *int64) []uuid.UUID {
	seen := map[string]bool{}
	var staged []uuid.UUID
	for _, img := range images {
		if len(staged) >= s.cfg.MaxImagesPerJob {
			break
		}
		if *runBudget <= 0 {
			break
		}
		u := strings.TrimSpace(img.URL)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true

		id, size, err := s.downloadOneImage(ctx, orgID, u, img.Alt, *runBudget)
		if err != nil {
			s.deps.Log.Warn("kbimport: skip embedded image", "url", u, "err", err)
			continue
		}
		handle := "upload.img-" + id.String()[:8]
		if err := s.deps.KB.TagImportMaterial(ctx, id, kbstore.ImportParams{RunID: runID, ParentID: &parentID, Handle: handle}); err != nil {
			s.deps.Log.Warn("kbimport: tag embedded image", "id", id, "err", err)
			continue
		}
		*runBudget -= size
		staged = append(staged, id)
	}
	return staged
}

// downloadOneImage fetches, validates, and stages one image byte-for-byte
// the same way handleKBUploadMaterial stages any file (see
// enqueue.go/stageFile's own doc comment) — CreateUploadMaterial ->
// blob.Put(id+"-"+sha[:16]) -> CompleteMaterialUpload — with
// customer_visibility "auto" rather than "visible": a scraped image's
// intent is uncertain, which is exactly what kb_media_upload's own "auto"
// default is for (validateMediaRef only rejects "invisible").
func (s *Service) downloadOneImage(ctx context.Context, orgID uuid.UUID, rawURL, alt string, runBudget int64) (uuid.UUID, int64, error) {
	if err := safefetch.CheckURL(ctx, rawURL, s.deps.AllowPrivateFetch); err != nil {
		return uuid.Nil, 0, err
	}
	maxBytes := s.cfg.MaxImageBytes
	if runBudget < maxBytes {
		maxBytes = runBudget
	}
	if maxBytes <= 0 {
		return uuid.Nil, 0, fmt.Errorf("run image budget exhausted")
	}

	client := safefetch.Client(s.deps.AllowPrivateFetch, s.cfg.ExtractTimeout)
	data, resp, err := safefetch.Get(ctx, client, rawURL, maxBytes)
	if err != nil {
		return uuid.Nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return uuid.Nil, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	mimeType := firstMediaType(resp.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if reason := blob.MimeSanityCheck(mimeType, data); reason != "" {
		return uuid.Nil, 0, fmt.Errorf("%s", reason)
	}
	if kbstore.KindOfMime(mimeType) != "image" {
		return uuid.Nil, 0, fmt.Errorf("not an image mime type: %s", mimeType)
	}

	filename := filenameFromURL(rawURL, alt)
	matID, err := s.deps.KB.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: filename, MimeType: mimeType, SizeBytes: int64(len(data)), CustomerVisibility: "auto",
	})
	if err != nil {
		return uuid.Nil, 0, err
	}
	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	storageKey, err := s.deps.Blob.Put(matID.String()+"-"+sumHex[:16], data, blob.Meta{
		Mimetype: mimeType, FileName: filename, FileSize: int64(len(data)),
	})
	if err != nil {
		return uuid.Nil, 0, err
	}
	if err := s.deps.KB.CompleteMaterialUpload(ctx, matID, "disk", storageKey, int64(len(data)), sumHex); err != nil {
		return uuid.Nil, 0, err
	}
	return matID, int64(len(data)), nil
}

// firstMediaType strips any ";charset=..." parameter suffix from a
// Content-Type header value.
func firstMediaType(contentType string) string {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	return strings.TrimSpace(contentType)
}

// filenameFromURL derives a display filename for a staged image: the URL's
// own path basename when it looks like one, else a short fallback built
// from the alt text.
func filenameFromURL(rawURL, alt string) string {
	if u, err := url.Parse(rawURL); err == nil {
		if base := path.Base(u.Path); base != "" && base != "." && base != "/" {
			return base
		}
	}
	alt = strings.TrimSpace(alt)
	if alt == "" {
		return "image"
	}
	if len(alt) > 60 {
		alt = alt[:60]
	}
	return alt
}
