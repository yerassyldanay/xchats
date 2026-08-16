package kbimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/extractor"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
)

// TargetTypes is the closed, operator-facing vocabulary POST
// /kb/imports' target_type accepts — "auto" plus one entry per content
// KB type pass 2 may synthesize into. assistant is deliberately absent: a
// persona is never inferred from imported content.
var TargetTypes = []string{"auto", "topics", "products", "tariffs", "contacts", "policies", "delivery_zones"}

func isValidTargetType(t string) bool {
	for _, v := range TargetTypes {
		if v == t {
			return true
		}
	}
	return false
}

// Providers is the closed, operator-facing provider vocabulary — the
// dropdown's own options (plan spec's "operator-facing dropdown").
var Providers = []string{"native", "firecrawl", "llamaparse"}

func isValidProvider(p string) bool {
	for _, v := range Providers {
		if v == p {
			return true
		}
	}
	return false
}

// SubmitFile is one already-read file's bytes — httpapi/kb_import.go reads
// each multipart file part fully before calling Submit (mirrors
// handleKBUploadMaterial's own ordering: bytes are read once, staged once).
type SubmitFile struct {
	Filename string
	MimeType string
	Data     []byte
}

// SubmitInput is POST /kb/imports' parsed body.
type SubmitInput struct {
	URLs       []string
	Files      []SubmitFile
	Provider   string
	TargetType string
	Guidance   string
}

const maxGuidanceRunes = 2000

// Submit validates in, stages every file, enqueues every material as an
// import job, and returns the freshly created run's status. This is the
// WHOLE of POST /kb/imports' server-side logic — httpapi/kb_import.go is a
// thin multipart-parsing adapter in front of it, which is what makes this
// method unit-testable without an HTTP harness (see enqueue_test.go /
// synth_test.go).
//
// Handler order matches plan spec exactly: validate vocabulary -> resolve
// + precheck the provider (credential missing -> ErrProviderNotConfigured,
// 422) -> safefetch.CheckURL every URL -> provider.Supports() precheck
// every URL/file (so an unsupported pairing like PDF+Firecrawl fails here,
// synchronously, never as an async job failure) -> ActiveImportRun gate
// (ErrRunActive, 409) -> stage every file's bytes -> EnqueueImport each
// material.
func (s *Service) Submit(ctx context.Context, orgID, userID uuid.UUID, in SubmitInput) (RunSummary, error) {
	if !isValidTargetType(in.TargetType) {
		return RunSummary{}, fmt.Errorf("%w: unknown target_type %q", ErrValidation, in.TargetType)
	}
	if !isValidProvider(in.Provider) {
		return RunSummary{}, fmt.Errorf("%w: unknown provider %q", ErrValidation, in.Provider)
	}
	if len(in.Guidance) > maxGuidanceRunes {
		return RunSummary{}, fmt.Errorf("%w: guidance exceeds %d characters", ErrValidation, maxGuidanceRunes)
	}
	if len(in.URLs) == 0 && len(in.Files) == 0 {
		return RunSummary{}, fmt.Errorf("%w: at least one URL or file is required", ErrValidation)
	}
	if len(in.URLs) > s.cfg.MaxURLsPerRun {
		return RunSummary{}, fmt.Errorf("%w: at most %d URLs per import", ErrValidation, s.cfg.MaxURLsPerRun)
	}
	if len(in.Files) > s.cfg.MaxFilesPerRun {
		return RunSummary{}, fmt.Errorf("%w: at most %d files per import", ErrValidation, s.cfg.MaxFilesPerRun)
	}

	provider, err := s.resolveProvider(ctx, in.Provider)
	if err != nil {
		return RunSummary{}, err
	}

	for _, u := range in.URLs {
		if err := safefetch.CheckURL(ctx, u, s.deps.AllowPrivateFetch); err != nil {
			return RunSummary{}, fmt.Errorf("%w: %s: %v", ErrValidation, u, err)
		}
		if !provider.Supports(extractor.Source{Kind: extractor.SourceURL, URL: u}) {
			return RunSummary{}, fmt.Errorf("%w: provider %q cannot read a URL", ErrValidation, in.Provider)
		}
	}
	for _, f := range in.Files {
		if provider.Supports(extractor.Source{Kind: extractor.SourceFile, MimeType: f.MimeType}) {
			continue
		}
		if f.MimeType == extractor.MimePDF && in.Provider != "llamaparse" {
			return RunSummary{}, fmt.Errorf("%w: %s is a PDF — PDF extraction requires the LlamaParse provider", ErrValidation, f.Filename)
		}
		if extractor.IsImage(f.MimeType) && in.Provider != "native" {
			return RunSummary{}, fmt.Errorf("%w: %s is an image — image extraction requires the native provider", ErrValidation, f.Filename)
		}
		return RunSummary{}, fmt.Errorf("%w: provider %q cannot read %s (%s)", ErrValidation, in.Provider, f.Filename, f.MimeType)
	}

	if activeRun, active, err := s.deps.KB.ActiveImportRun(ctx, orgID); err != nil {
		return RunSummary{}, err
	} else if active {
		return RunSummary{}, fmt.Errorf("%w: run %s", ErrRunActive, activeRun)
	}

	runID := uuid.New()
	primaryAssigned := false
	nextPrimary := func() bool {
		if primaryAssigned {
			return false
		}
		primaryAssigned = true
		return true
	}

	for i, u := range in.URLs {
		if _, err := s.deps.KB.EnqueueImport(ctx, orgID, kbstore.ImportInput{
			RunID: runID, UserID: userID, Provider: in.Provider, TargetType: in.TargetType, Guidance: in.Guidance,
			Primary: nextPrimary(), Handle: fmt.Sprintf("evidence.%d", i+1), URL: u,
		}); err != nil {
			return RunSummary{}, err
		}
	}
	for i, f := range in.Files {
		matID, err := s.stageFile(ctx, orgID, f)
		if err != nil {
			return RunSummary{}, err
		}
		if _, err := s.deps.KB.EnqueueImport(ctx, orgID, kbstore.ImportInput{
			RunID: runID, UserID: userID, Provider: in.Provider, TargetType: in.TargetType, Guidance: in.Guidance,
			Primary: nextPrimary(), Handle: fmt.Sprintf("upload.%d", i+1), MaterialID: matID,
		}); err != nil {
			return RunSummary{}, err
		}
	}

	s.notify()
	return s.RunStatus(ctx, orgID, runID)
}

// stageFile lands one file's bytes byte-for-byte the same way
// handleKBUploadMaterial does (internal/httpapi/kb_media.go): a MIME
// sanity check against the sniffed bytes, CreateUploadMaterial ->
// blob.Put(id+"-"+sha[:16]) -> CompleteMaterialUpload, customer_visibility
// "visible" (an operator explicitly choosing to submit this exact file is
// the same intent-signal handleKBUploadMaterial's own dedicated upload
// button already carries — see that handler's own doc comment).
func (s *Service) stageFile(ctx context.Context, orgID uuid.UUID, f SubmitFile) (uuid.UUID, error) {
	if reason := blob.MimeSanityCheck(f.MimeType, f.Data); reason != "" {
		return uuid.Nil, fmt.Errorf("%w: %s: %s", ErrValidation, f.Filename, reason)
	}
	if kbstore.KindOfMime(f.MimeType) == "" {
		return uuid.Nil, fmt.Errorf("%w: %s: unsupported file type %s — must be an image, video, audio, or document", ErrValidation, f.Filename, f.MimeType)
	}

	matID, err := s.deps.KB.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: f.Filename, MimeType: f.MimeType, SizeBytes: int64(len(f.Data)), CustomerVisibility: "visible",
	})
	if err != nil {
		return uuid.Nil, err
	}
	sum := sha256.Sum256(f.Data)
	sumHex := hex.EncodeToString(sum[:])
	storageKey, err := s.deps.Blob.Put(matID.String()+"-"+sumHex[:16], f.Data, blob.Meta{
		Mimetype: f.MimeType, FileName: f.Filename, FileSize: int64(len(f.Data)),
	})
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.deps.KB.CompleteMaterialUpload(ctx, matID, "disk", storageKey, int64(len(f.Data)), sumHex); err != nil {
		return uuid.Nil, err
	}
	return matID, nil
}
