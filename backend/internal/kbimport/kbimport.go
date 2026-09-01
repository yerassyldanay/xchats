// Package kbimport is the structured Knowledge Base import pipeline: submit
// a URL or a document, extract it with a chosen provider (pass 1), then
// synthesize the accumulated evidence into typed KB draft records (pass 2)
// through the exact same MCP typed-upsert seam a connected model would use
// (internal/mcpserver's ParseUpsertCall/UpsertTools). Nothing reaches live
// ai_* — every write lands in kbd_draft, same as every other authoring path
// (plan/mcp.md §1's safety boundary).
//
// The kbd_materials row IS the job queue (internal/kbstore/import.go),
// following internal/automation/scheduler.go's precedent rather than
// internal/queue: a durable table survives a restart, an in-memory channel
// does not. Service.Start recovers stuck jobs and stuck synthesis runs on
// boot and on a periodic tick, then runs a small worker pool claiming from
// the same table.
package kbimport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/extractor"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/safefetch"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// Errors Submit/RunStatus return — httpapi/kb_import.go maps each to its
// own HTTP status via errors.Is, so the mapping lives in exactly one place.
var (
	// ErrValidation is a submit-time input problem (bad vocabulary, an
	// unsupported provider/source pairing, an SSRF-blocked URL, too many
	// inputs) → 400.
	ErrValidation = errors.New("kbimport: validation failed")
	// ErrProviderNotConfigured means the chosen provider needs a credential
	// that is not currently resolvable → 422.
	ErrProviderNotConfigured = errors.New("kbimport: provider is not configured")
	// ErrRunActive means the organization already has a non-terminal import
	// run → 409 (one active run per org).
	ErrRunActive = errors.New("kbimport: an import run is already active for this organization")
	// ErrNotFound means the run id does not resolve for this organization.
	ErrNotFound = errors.New("kbimport: import run not found")
	// ErrCancelNotAllowed means Cancel was called after pass 2 (synthesis)
	// had already been claimed for the run → 409 (see
	// kbstore.CancelImportRun's own doc comment for why that boundary is a
	// hard one, not just a UI nicety).
	ErrCancelNotAllowed = errors.New("kbimport: this run can no longer be cancelled — synthesis has already started")
)

// Deps are the Service's external collaborators. Every write path this
// package uses (KB, Blob) is the exact same one the equivalent hand-authored
// flow (handleKBUploadMaterial, an MCP-connected model) already goes
// through — this package adds no second write path, only a second CALLER
// of the existing ones.
type Deps struct {
	KB          *kbstore.Store
	Blob        blob.Store
	Credentials *credentials.Chain // nil-tolerant: firecrawl/llamaparse then always report ErrProviderNotConfigured
	Settings    *settings.Store    // nil-tolerant: provider base-URL overrides and the synthesis model default are then always the built-in default
	LLM         llm.Registry
	Hub         *realtime.Hub // broadcasts kb.row.changed so the frontend's SSE-gated store refreshes
	Log         *slog.Logger
	// AllowPrivateFetch mirrors config.KBAllowPrivateFetch — threaded to
	// every safefetch call this package makes against an OPERATOR-SUBMITTED
	// URL (the native provider's own page fetch, and every embedded-image
	// download). It does NOT apply to the outbound calls this package makes
	// to a configured Firecrawl/LlamaParse endpoint itself — those always
	// allow a private host (self-hosting either vendor on the operator's
	// own network is a real, intentional use case; see resolveProvider).
	AllowPrivateFetch bool
}

// Config is the pipeline's tunables — see DefaultConfig for the shipped
// values.
type Config struct {
	Workers      int
	MaxAttempts  int
	StuckAfter   time.Duration
	ClaimEvery   time.Duration
	RecoverEvery time.Duration

	ExtractTimeout time.Duration
	SynthTimeout   time.Duration

	MaxURLsPerRun  int
	MaxFilesPerRun int

	MaxImagesPerJob  int
	MaxImageBytes    int64
	MaxRunImageBytes int64

	// MaxPageBytes caps the native provider's own page fetch — not named in
	// the original cap list, which enumerates image/text/call budgets but
	// not this; a generous but real ceiling is still needed so a
	// pathological page cannot be read into memory unbounded.
	MaxPageBytes int64

	MaxTextPerMaterial int // runes
	MaxTextPerRun      int // runes
	MaxCallsPerRun     int
}

// DefaultConfig returns the shipped tunables.
func DefaultConfig() Config {
	return Config{
		Workers:      2,
		MaxAttempts:  3,
		StuckAfter:   5 * time.Minute,
		ClaimEvery:   3 * time.Second,
		RecoverEvery: 60 * time.Second,

		ExtractTimeout: 120 * time.Second,
		SynthTimeout:   120 * time.Second,

		MaxURLsPerRun:  10,
		MaxFilesPerRun: 10,

		MaxImagesPerJob:  8,
		MaxImageBytes:    5 << 20,
		MaxRunImageBytes: 20 << 20,

		MaxPageBytes: 10 << 20,

		MaxTextPerMaterial: 20_000,
		MaxTextPerRun:      80_000,
		MaxCallsPerRun:     25,
	}
}

// Service runs the import pipeline.
type Service struct {
	deps Deps
	cfg  Config

	claimed chan kbstore.ImportJob
	wg      sync.WaitGroup
}

// New builds a Service. Call Start to begin processing.
func New(deps Deps, cfg Config) *Service {
	if deps.Log == nil {
		deps.Log = slog.Default()
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	return &Service{deps: deps, cfg: cfg, claimed: make(chan kbstore.ImportJob, 64)}
}

// Start launches the claim/recover loop and the worker pool — modeled on
// internal/automation/scheduler.go's Start: a startup recovery pass, then
// ClaimEvery/RecoverEvery tickers. Every launched goroutine stops once ctx
// is done; call Stop afterward to block until they actually have.
func (s *Service) Start(ctx context.Context) {
	s.recover(ctx)

	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runWorker(ctx)
		}()
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		claim := time.NewTicker(s.cfg.ClaimEvery)
		recoverTick := time.NewTicker(s.cfg.RecoverEvery)
		defer claim.Stop()
		defer recoverTick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-claim.C:
				s.claimBatch(ctx)
			case <-recoverTick.C:
				s.recover(ctx)
			}
		}
	}()
}

// Stop blocks until every goroutine Start launched has exited. Call this
// with ctx already canceled (or about to be).
func (s *Service) Stop() { s.wg.Wait() }

// notify broadcasts kb.row.changed — the same event GET /kb's live writes
// already broadcast (kbLiveChanged, httpapi/kb_live.go). The frontend's
// import store and the existing Playground store both already bind to this
// one event name; no new SSE event type is needed for material-level
// progress (GET /kb already returns materials[].processing_status).
func (s *Service) notify() {
	if s.deps.Hub != nil {
		s.deps.Hub.Broadcast("kb.row.changed", map[string]any{})
	}
}

// resolveProvider builds a live extractor.Provider for name, re-reading
// credentials/settings fresh on every call — "keys resolve per run, not at
// boot, so a just-saved key works without a restart."
func (s *Service) resolveProvider(ctx context.Context, name string) (extractor.Provider, error) {
	switch name {
	case "native":
		return extractor.NewNative(safefetch.Client(s.deps.AllowPrivateFetch, s.cfg.ExtractTimeout), s.cfg.MaxPageBytes), nil
	case "firecrawl":
		key, err := s.requireCredential(ctx, "firecrawl.api_key")
		if err != nil {
			return nil, err
		}
		// allowPrivate: true — this client only ever talks to the
		// OPERATOR-CONFIGURED Firecrawl endpoint (public default or a
		// deliberately self-hosted base_url), never an operator-submitted
		// arbitrary URL. See Deps.AllowPrivateFetch's own doc comment.
		return extractor.NewFirecrawl(key, s.providerBaseURL("firecrawl"), safefetch.Client(true, s.cfg.ExtractTimeout)), nil
	case "llamaparse":
		key, err := s.requireCredential(ctx, "llamaparse.api_key")
		if err != nil {
			return nil, err
		}
		return extractor.NewLlamaParse(key, s.providerBaseURL("llamaparse"), safefetch.Client(true, s.cfg.ExtractTimeout)), nil
	default:
		return nil, fmt.Errorf("%w: unknown provider %q", ErrValidation, name)
	}
}

// ProviderStatus reports whether name requires a credential and, if so,
// whether one currently resolves — through the EXACT SAME requireCredential
// call resolveProvider itself makes, so GET /kb/import/providers
// (internal/httpapi) can never disagree with Submit's own precheck: an
// unknown name reports (false, false), matching Submit's own rejection of
// it.
func (s *Service) ProviderStatus(ctx context.Context, name string) (requiresCredential, configured bool) {
	switch name {
	case "native":
		return false, true
	case "firecrawl":
		_, err := s.requireCredential(ctx, "firecrawl.api_key")
		return true, err == nil
	case "llamaparse":
		_, err := s.requireCredential(ctx, "llamaparse.api_key")
		return true, err == nil
	default:
		return false, false
	}
}

func (s *Service) requireCredential(ctx context.Context, key credentials.Key) (string, error) {
	if s.deps.Credentials == nil {
		return "", fmt.Errorf("%w: no credential store is configured", ErrProviderNotConfigured)
	}
	v, err := s.deps.Credentials.Get(ctx, key)
	if err != nil || v == "" {
		return "", fmt.Errorf("%w: %s is not configured", ErrProviderNotConfigured, key)
	}
	return v, nil
}

func (s *Service) providerBaseURL(id string) string {
	if s.deps.Settings == nil {
		return ""
	}
	st, err := s.deps.Settings.Load()
	if err != nil {
		return ""
	}
	return st.Providers[id].BaseURL
}

// resolveSynthModel reads the response engine's own default provider/model
// (internal/settings.LLMSettings) — pass 2 deliberately has no separate
// "which model does import synthesis use" setting; it rides the same
// default every other AI-drafting path in this app already uses.
func (s *Service) resolveSynthModel() (llm.ModelRef, error) {
	if s.deps.Settings == nil {
		return llm.ModelRef{}, fmt.Errorf("%w: no settings store is configured", ErrProviderNotConfigured)
	}
	st, err := s.deps.Settings.Load()
	if err != nil {
		return llm.ModelRef{}, err
	}
	if st.LLM.DefaultProvider == "" || st.LLM.DefaultModel == "" {
		return llm.ModelRef{}, fmt.Errorf("%w: no default model is configured", ErrProviderNotConfigured)
	}
	return llm.ModelRef{Provider: st.LLM.DefaultProvider, Model: st.LLM.DefaultModel}, nil
}

// RunSummary is GET /kb/imports/:id's payload.
type RunSummary struct {
	RunID uuid.UUID `json:"run_id"`
	Status string   `json:"status"` // extracting | synthesizing | built | failed | needs_human | cancelled
	// StartedBy/StartedAt come from the run's primary material — the
	// user id Submit recorded (ImportParams.UserID, already stored on
	// every run before this) and that row's own created_at. StartedBy is
	// a raw uuid, matching how CreatedBy is exposed elsewhere in this app
	// (dto.Campaign.CreatedBy) — resolving it to a display name is a
	// client-side lookup against the org's already-loaded user list, not
	// this API's job.
	StartedBy string    `json:"started_by"`
	StartedAt time.Time `json:"started_at"`
	// FinishedAt is set once the run reaches a terminal synthesis state
	// (built/failed/needs_human/cancelled) — the primary material's own
	// UpdatedAt at that point, since every terminal transition
	// (FinishImportSynthesis, CancelImportRun) writes it in the same
	// statement as the state change. A pointer, omitted while the run is
	// still active, matching Synthesis' own omitempty below rather than a
	// zero time.Time the client would have to special-case.
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Materials  []MaterialStatus `json:"materials"`
	// Cancelable is true only while pass 1 is still in progress — see
	// kbstore.CancelImportRun's own doc comment for why cancelling is
	// refused once synthesis has been claimed.
	Cancelable bool              `json:"cancelable"`
	Synthesis  *SynthesisSummary `json:"synthesis,omitempty"`
}

// MaterialStatus is one material's status within RunSummary.
type MaterialStatus struct {
	ID               uuid.UUID `json:"id"`
	Kind             string    `json:"kind"` // url | file
	Label            string    `json:"label"`
	Handle           string    `json:"handle"`
	ProcessingStatus string    `json:"processing_status"`
	Error            string    `json:"error,omitempty"`
}

// SynthesisSummary mirrors kbstore.SynthesisState for the run status API.
type SynthesisSummary struct {
	Status  string                  `json:"status"`
	Notes   string                  `json:"notes,omitempty"`
	Applied []kbstore.AppliedUpsert `json:"applied,omitempty"`
	Dropped []kbstore.DroppedUpsert `json:"dropped,omitempty"`
	Usage   kbstore.TokenUsage      `json:"usage"`
}

// RunStatus reads back one run's current state — org-scoped, so a
// cross-org run id is indistinguishable from an unknown one (ErrNotFound,
// both map to a plain 404).
func (s *Service) RunStatus(ctx context.Context, orgID, runID uuid.UUID) (RunSummary, error) {
	materials, err := s.deps.KB.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		return RunSummary{}, err
	}
	if len(materials) == 0 {
		return RunSummary{}, ErrNotFound
	}

	out := RunSummary{RunID: runID}
	var primary *kbstore.ImportMaterial
	for i := range materials {
		m := materials[i]
		if m.Params.Primary {
			primary = &materials[i]
		}
		label := m.SourceRef
		if m.SourceType == "file" {
			label = m.Filename
		}
		out.Materials = append(out.Materials, MaterialStatus{
			ID: m.ID, Kind: m.SourceType, Label: label, Handle: m.Params.Handle,
			ProcessingStatus: m.ProcessingStatus, Error: m.Params.LastError,
		})
	}
	out.Status = deriveRunStatus(materials, primary)
	if primary != nil {
		out.StartedBy = primary.Params.UserID.String()
		out.StartedAt = primary.CreatedAt
		out.Cancelable = primary.Params.Synthesis == nil
		if primary.Params.Synthesis != nil {
			st := primary.Params.Synthesis
			out.Synthesis = &SynthesisSummary{Status: st.Status, Notes: st.Notes, Applied: st.Applied, Dropped: st.Dropped, Usage: st.Usage}
			if st.IsTerminal() {
				finishedAt := primary.UpdatedAt
				out.FinishedAt = &finishedAt
			}
		}
	}
	return out, nil
}

// Cancel stops runID from progressing further — see
// kbstore.CancelImportRun's own doc comment for exactly what "stops" means
// and its safety boundary.
func (s *Service) Cancel(ctx context.Context, orgID, runID uuid.UUID) error {
	cancelled, err := s.deps.KB.CancelImportRun(ctx, orgID, runID)
	if err != nil {
		switch {
		case errors.Is(err, kbstore.ErrUnknownKind):
			return ErrNotFound
		case errors.Is(err, kbstore.ErrImportRunNotCancelable):
			return ErrCancelNotAllowed
		default:
			return err
		}
	}
	if cancelled {
		s.notify()
	}
	return nil
}

// ListRuns returns one page of the org's import runs' summaries, newest
// first — GET /kb/imports' listing source: limit=1/offset=0 for "just the
// active run" (the pre-KB-14 caller), or limit=pageSize/offset=(page-1)*
// pageSize for KB-14's history list. limit<=0 defaults to 20. total is the
// org's full run count, for the caller to compute page count.
func (s *Service) ListRuns(ctx context.Context, orgID uuid.UUID, limit, offset int) (runs []RunSummary, total int, err error) {
	runIDs, total, err := s.deps.KB.RecentImportRuns(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]RunSummary, 0, len(runIDs))
	for _, runID := range runIDs {
		summary, err := s.RunStatus(ctx, orgID, runID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue // vanished between the listing scan and this read — skip, not fatal
			}
			return nil, 0, err
		}
		out = append(out, summary)
	}
	return out, total, nil
}

// deriveRunStatus computes RunSummary.Status from the primary's synthesis
// state (once meaningful) and, before that, from whether any material is
// still mid pass-1.
func deriveRunStatus(materials []kbstore.ImportMaterial, primary *kbstore.ImportMaterial) string {
	if primary != nil && primary.Params.Synthesis != nil {
		switch primary.Params.Synthesis.Status {
		case kbstore.SynthesisRunning:
			return "synthesizing"
		case kbstore.SynthesisBuilt:
			return "built"
		case kbstore.SynthesisFailed:
			return "failed"
		case kbstore.SynthesisNeedsHuman:
			return "needs_human"
		case kbstore.SynthesisCancelled:
			return "cancelled"
		}
	}
	for _, m := range materials {
		if m.ProcessingStatus == kbstore.ImportStatusQueued || m.ProcessingStatus == "extracting" {
			return "extracting"
		}
	}
	return "extracting" // every material terminal; synthesis about to be claimed
}
