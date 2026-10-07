// Package kbstore's import.go is the structured-import pipeline's job
// queue: the kbd_materials ROW IS the queue (internal/automation/
// scheduler.go's precedent — a durable table, not an in-memory channel that
// loses work on restart). "queued" is the only processing_status value this
// file adds to the vocabulary aiprompt/types.go's Material.ProcessingStatus
// doc already publishes (uploaded | extracting | parsed | built |
// needs_human | failed); the rest of the lifecycle reuses it exactly:
//
//	POST /kb/imports → (url) insert 'queued' │ (file) stage bytes → CAS 'parsed'→'queued'
//	  queued ──claim──> extracting ──> parsed | needs_human | failed
//	     ↑                   │
//	     └──── recover ──────┘  (updated_at older than StuckAfter, attempts+1)
//	  all rows terminal → BeginImportSynthesis (CAS) → pass 2 → MarkImportBuilt → 'built'
//
// Job bookkeeping (run id, provider, attempts, the request-scoped handle,
// pass-2 synthesis state) lives in kbd_materials.extraction_metadata.import —
// a plain JSON association, not a new table (the same choice
// recordProvenance already makes for provenance, mcp_media.go). No SQL JSON
// function is involved, so the statements are byte-identical on SQLite and
// PostgreSQL: every writer reads the row, edits the typed ImportParams in Go
// and writes the whole document back (mutateImportParams), as a compare-and-
// swap on the text it read so a concurrent writer makes it retry instead of
// clobbering. The two facts the queue filters on — which run a material
// belongs to, and the primary's pass-2 status — are mirrored into the
// import_run_id / import_synthesis_status columns by that same write, so
// "every material of run X" and "primaries nobody has started pass 2 on" are
// ordinary indexed predicates, not JSON path queries.
//
// ImportParams/SynthesisState are deliberately marshaled with NO `omitempty`
// on any field: every writer does a full read-mutate-write of the complete
// struct, so clearing a field back to its zero value (e.g. LastError on a
// successful retry) is written back as exactly that.
//
// None of these methods runs inside another method's writeDraftBlobVersioned
// closure, so every one of them is free to use s.db directly (see
// identityIndex's doc comment, mcp_read.go, for the pool-exhaustion deadlock
// that constraint exists to avoid elsewhere in this package).
package kbstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ImportStatusQueued is the one new processing_status value this pipeline
// introduces. Always transiting through it — rather than going straight
// file-upload 'parsed' → 'extracting' — is what disambiguates 'parsed'
// CompleteMaterialUpload's sense ("bytes landed, ready to be queued") from
// this pipeline's own sense ("extraction finished, ready for synthesis"):
// the two never collide because a file job is never 'extracting' without
// having first been CASed through 'queued'.
const ImportStatusQueued = "queued"

// ImportStatusCancelled is CancelImportRun's own terminal processing_status
// — set on every material that was still 'queued' or 'extracting' when the
// run was cancelled, so a material chip stops reading as perpetually
// in-progress once the run itself has stopped.
const ImportStatusCancelled = "cancelled"

// Synthesis states — SynthesisState.Status. "" means pass 2 has not begun
// for this run yet.
const (
	SynthesisRunning    = "running"
	SynthesisBuilt      = "built"
	SynthesisFailed     = "failed"
	SynthesisNeedsHuman = "needs_human"
	// SynthesisCancelled is CancelImportRun's own terminal marker — set on
	// the primary material's synthesis sub-state even when pass 2 was
	// never claimed (Synthesis was nil), since ActiveImportRun's
	// one-run-per-org gate keys off synthesis reaching a terminal status,
	// never off processing_status (see its own doc comment): leaving
	// Synthesis nil after a cancel would leave the org permanently unable
	// to submit a new run.
	SynthesisCancelled = "cancelled"
)

// ErrImportRunActive means the organization already has a non-terminal
// import run — the one-active-run-per-org 409 gate.
var ErrImportRunActive = errors.New("kbstore: an import run is already active for this organization")

// ErrImportRunNotCancelable means CancelImportRun was called after pass 2
// (synthesis) had already been claimed for the run — see CancelImportRun's
// own doc comment for why that boundary is a hard one, not just a UI
// nicety.
var ErrImportRunNotCancelable = errors.New("kbstore: this run can no longer be cancelled — synthesis has already started")

// AppliedUpsert records one pass-2 call that landed in the draft —
// SynthesisState.Applied's element shape, surfaced verbatim in the run
// status response's synthesis.applied[].
type AppliedUpsert struct {
	Tool    string `json:"tool"`
	Type    string `json:"type"`
	Key     string `json:"key"`
	Created bool   `json:"created"`
}

// DroppedUpsert records one pass-2 call that failed validation and was
// dropped — plan/DECISIONS.md's "keep valid entries, drop broken ones,
// report what was dropped."
type DroppedUpsert struct {
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

// TokenUsage is pass 2's one model call's reported token spend, recorded
// per run so cost is visible rather than inferred from a bill.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// SynthesisState is pass 2's state machine — recorded on the run's PRIMARY
// material row only (ImportParams.Primary).
type SynthesisState struct {
	// Status: "" (not started) | running | built | failed | needs_human.
	Status    string          `json:"status"`
	Token     uuid.UUID       `json:"token"`
	StartedAt time.Time       `json:"started_at"`
	Notes     string          `json:"notes"`
	Applied   []AppliedUpsert `json:"applied"`
	Dropped   []DroppedUpsert `json:"dropped"`
	Usage     TokenUsage      `json:"usage"`
}

// IsTerminal reports whether st represents a finished run — built (pass 2
// landed something), failed (pass 2 gave up after a re-ask), or
// needs_human (recovery gave up on a stuck run). A nil state, or an empty
// Status, is NOT terminal — pass 2 has not even begun. Exported so
// kbimport.RunStatus (a different package) can derive RunSummary.FinishedAt
// from the same single source of truth this file's own callers use, rather
// than re-deriving "terminal" from a second, string-literal copy of the
// same status set.
func (st *SynthesisState) IsTerminal() bool {
	if st == nil {
		return false
	}
	switch st.Status {
	case SynthesisBuilt, SynthesisFailed, SynthesisNeedsHuman, SynthesisCancelled:
		return true
	}
	return false
}

// ImportParams is the kbd_materials.extraction_metadata.import payload
// every import job material carries.
type ImportParams struct {
	RunID      uuid.UUID `json:"run_id"`
	UserID     uuid.UUID `json:"user_id"`
	Provider   string    `json:"provider"`
	TargetType string    `json:"target_type"`
	Guidance   string    `json:"guidance"`
	// Primary marks the one material that anchors the run: its Synthesis
	// field is the run's single source of truth for pass-2 state.
	// BeginImportSynthesis/FinishImportSynthesis/RecoverStuckSynthesis/
	// ActiveImportRun all operate on the primary row only.
	Primary   bool   `json:"primary"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"last_error"`
	// ParentID is set on a downloaded embedded image: the material it was
	// found on (nil for every operator-submitted URL/file).
	ParentID *uuid.UUID `json:"parent_id"`
	// Handle is the request-scoped handle (upload.N / evidence.N) pass 2's
	// prompt manifest names this material by — assigned once, at staging
	// time, so the manifest survives a restart.
	Handle string `json:"handle"`
	// Synthesis is set only on the primary row, and only once pass 2 has
	// been claimed for the run (BeginImportSynthesis) — nil until then.
	Synthesis *SynthesisState `json:"synthesis"`
}

// ImportJob is one claimed row, ready for pass-1 extraction
// (internal/kbimport/pass1.go).
type ImportJob struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	SourceType     string // 'url' | 'file'
	SourceRef      string // the URL, for a url job
	Filename       string
	MimeType       string
	StorageKey     string
	Params         ImportParams
}

// ImportMaterial is one run material's full current state —
// ImportRunMaterials' element shape, used both for run-status reporting
// (GET /kb/imports/:id) and as pass 2's manifest source
// (internal/kbimport/prompt.go).
type ImportMaterial struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	SourceType         string
	SourceRef          string
	Filename           string
	MimeType           string
	StorageKey         string
	ExtractedText      string
	CustomerVisibility string
	ProcessingStatus   string
	CreatedAt          time.Time
	// UpdatedAt is the row's last write — for the primary material of a
	// terminal run, that write IS the transition into that terminal state
	// (FinishImportSynthesis/CancelImportRun both set updated_at in the
	// same statement as the state change), so kbimport.RunStatus reads this
	// as the run's finish time.
	UpdatedAt time.Time
	Params    ImportParams
}

// ExtractionOutcome is what pass 1 reports after running a Provider against
// one claimed material.
type ExtractionOutcome struct {
	// Status: 'parsed' | 'needs_human' | 'failed'.
	Status        string
	ExtractedText string
	VisualSummary string
}

// ImportInput is EnqueueImport's per-material input. Exactly one of URL or
// MaterialID is meaningful.
type ImportInput struct {
	RunID      uuid.UUID
	UserID     uuid.UUID
	Provider   string
	TargetType string
	Guidance   string
	Primary    bool
	ParentID   *uuid.UUID
	Handle     string

	// URL creates a fresh source_type='url' material starting directly at
	// 'queued' — there is nothing to stage first.
	URL string
	// MaterialID names an ALREADY-STAGED source_type='file' material
	// (landed via the ordinary CreateUploadMaterial -> blob.Put ->
	// CompleteMaterialUpload sequence, exactly like handleKBUploadMaterial)
	// and is CASed from 'parsed' to 'queued'. EnqueueImport never stages
	// file bytes itself.
	MaterialID uuid.UUID
}

// importDoc is the extraction_metadata document of a freshly enqueued URL material:
// nothing but the import params.
type importDoc struct {
	Import ImportParams `json:"import"`
}

func marshalImportDoc(p ImportParams) (string, error) {
	b, err := json.Marshal(importDoc{Import: p})
	if err != nil {
		return "", fmt.Errorf("kbstore: marshal import params: %w", err)
	}
	return string(b), nil
}

// importRow is one material's import state as read for a read-modify-write: the
// processing status, the typed import params, and the whole extraction_metadata
// document decoded to raw top-level values, so keys this file does not own (the
// mcp_target provenance tag, say) survive a write byte-for-byte.
type importRow struct {
	OrgID  uuid.UUID
	Status string
	Params ImportParams

	doc map[string]json.RawMessage
	raw string // extraction_metadata exactly as read: the compare-and-swap token
}

// importCASAttempts bounds how often a write that lost its compare-and-swap to a
// concurrent writer re-reads and tries again.
const importCASAttempts = 8

var errImportConflict = errors.New("kbstore: import state changed concurrently too many times")

func loadImportRow(ctx context.Context, q dbx.DBTX, id uuid.UUID) (importRow, error) {
	var r importRow
	if err := q.QueryRow(ctx, `SELECT organization_id, processing_status, extraction_metadata FROM kbd_materials WHERE id = $1`, id).
		Scan(&r.OrgID, &r.Status, &r.raw); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return r, ErrUnknownKind
		}
		return r, err
	}
	r.doc = map[string]json.RawMessage{}
	if strings.TrimSpace(r.raw) != "" {
		if err := json.Unmarshal([]byte(r.raw), &r.doc); err != nil {
			return r, fmt.Errorf("kbstore: parse extraction_metadata: %w", err)
		}
		if r.doc == nil { // the document was the JSON literal null
			r.doc = map[string]json.RawMessage{}
		}
	}
	params, err := parseImportParams(r.raw)
	if err != nil {
		return r, err
	}
	r.Params = params
	return r, nil
}

// encode is the document to store: the original keys, with "import" replaced by the
// current params.
func (r *importRow) encode() (string, error) {
	imp, err := json.Marshal(r.Params)
	if err != nil {
		return "", fmt.Errorf("kbstore: marshal import params: %w", err)
	}
	r.doc["import"] = imp
	out, err := json.Marshal(r.doc)
	if err != nil {
		return "", fmt.Errorf("kbstore: marshal extraction_metadata: %w", err)
	}
	return string(out), nil
}

// mutateImportParams is the one write path for a material's import state. It reads the
// row, lets mutate edit the typed params (and, for the transitions that change it
// together with them, the processing status), and writes the whole document back along
// with the mirrored import_run_id / import_synthesis_status columns and updated_at.
//
// The write is a compare-and-swap on the exact metadata text and status that were read:
// a concurrent writer makes it match zero rows, and it re-reads and decides again (up to
// importCASAttempts times). That is atomic on both engines with no row lock and no JSON
// function. mutate returning false means "leave the row alone" — a guard such as "pass 2
// not yet claimed" failed — and applied is then false. ErrUnknownKind means no such row.
func mutateImportParams(ctx context.Context, q dbx.DBTX, id uuid.UUID, mutate func(*importRow) bool) (applied bool, err error) {
	for attempt := 0; attempt < importCASAttempts; attempt++ {
		row, err := loadImportRow(ctx, q, id)
		if err != nil {
			return false, err
		}
		beforeStatus := row.Status
		if !mutate(&row) {
			return false, nil
		}
		doc, err := row.encode()
		if err != nil {
			return false, err
		}
		var synthesis any // NULL until pass 2 has been claimed
		if row.Params.Synthesis != nil {
			synthesis = row.Params.Synthesis.Status
		}
		tag, err := q.Exec(ctx, `UPDATE kbd_materials
			SET processing_status = $2, extraction_metadata = $3, import_run_id = $4, import_synthesis_status = $5, updated_at = $6
			WHERE id = $1 AND extraction_metadata = $7 AND processing_status = $8`,
			id, row.Status, doc, row.Params.RunID.String(), synthesis, time.Now(), row.raw, beforeStatus)
		if err != nil {
			return false, err
		}
		if tag.RowsAffected() == 1 {
			return true, nil
		}
	}
	return false, errImportConflict
}

func parseImportParams(extractionMetadata string) (ImportParams, error) {
	if extractionMetadata == "" {
		return ImportParams{}, nil
	}
	var doc struct {
		Import ImportParams `json:"import"`
	}
	if err := json.Unmarshal([]byte(extractionMetadata), &doc); err != nil {
		return ImportParams{}, fmt.Errorf("kbstore: parse extraction_metadata.import: %w", err)
	}
	return doc.Import, nil
}

// EnqueueImport stages one import job material: either a fresh
// source_type='url' row (starting at 'queued') or an already-uploaded file
// material (CASed from 'parsed' to 'queued', tagged with import params in
// the same statement). Returns the material id either way. ErrUnknownKind
// means MaterialID does not exist, belongs to a different org, or is not
// currently 'parsed' (already enqueued, or never finished uploading).
func (s *Store) EnqueueImport(ctx context.Context, orgID uuid.UUID, in ImportInput) (uuid.UUID, error) {
	params := ImportParams{
		RunID: in.RunID, UserID: in.UserID, Provider: in.Provider,
		TargetType: in.TargetType, Guidance: in.Guidance, Primary: in.Primary,
		ParentID: in.ParentID, Handle: in.Handle,
	}

	if in.URL != "" {
		doc, err := marshalImportDoc(params)
		if err != nil {
			return uuid.Nil, err
		}
		now := time.Now()
		id := uuid.New()
		if _, err := s.db.Exec(ctx, `INSERT INTO kbd_materials
			(id, organization_id, source_type, source_ref, extraction_metadata, processing_status, customer_visibility, import_run_id, created_at, updated_at)
			VALUES ($1, $2, 'url', $3, $4, $5, 'invisible', $6, $7, $7)`,
			id, orgID, in.URL, doc, ImportStatusQueued, params.RunID.String(), now); err != nil {
			return uuid.Nil, fmt.Errorf("kbstore: enqueue url import: %w", err)
		}
		return id, nil
	}

	if in.MaterialID == uuid.Nil {
		return uuid.Nil, errors.New("kbstore: EnqueueImport requires either a URL or a staged MaterialID")
	}
	// The compare-and-swap inside mutateImportParams carries the 'parsed' -> 'queued'
	// guard: of two concurrent enqueues of the same material exactly one applies.
	applied, err := mutateImportParams(ctx, s.db, in.MaterialID, func(r *importRow) bool {
		if r.OrgID != orgID || r.Status != "parsed" {
			return false
		}
		r.Params = params
		r.Status = ImportStatusQueued
		return true
	})
	if errors.Is(err, ErrUnknownKind) || (err == nil && !applied) {
		return uuid.Nil, ErrUnknownKind
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("kbstore: enqueue file import: %w", err)
	}
	return in.MaterialID, nil
}

// TagImportMaterial merges import params onto a material that is already
// terminal and never enters the job queue at all — a downloaded embedded
// image, staged the same byte-for-byte way handleKBUploadMaterial stages
// any file (CreateUploadMaterial -> blob.Put -> CompleteMaterialUpload,
// landing directly at 'parsed'): there is nothing to extract from an
// image (the native provider's own image handling is a no-op), so it
// never needs to be 'queued'/'extracting'. Used by
// internal/kbimport/images.go.
func (s *Store) TagImportMaterial(ctx context.Context, id uuid.UUID, params ImportParams) error {
	_, err := mutateImportParams(ctx, s.db, id, func(r *importRow) bool {
		r.Params = params
		return true
	})
	if errors.Is(err, ErrUnknownKind) {
		return ErrUnknownKind
	}
	if err != nil {
		return fmt.Errorf("kbstore: tag import material: %w", err)
	}
	return nil
}

// ClaimImportJobs atomically claims up to limit 'queued' materials (oldest
// first), transitioning each to extracting. The claim runs in a dbx write
// transaction, which serializes concurrent claimers on both engines (SQLite:
// BEGIN IMMEDIATE; PostgreSQL: the transaction-scoped advisory lock), so each
// claimer's candidate scan already sees the previous claimer's commit and the
// returned sets are disjoint — no row-level locking clause is needed. The
// processing_status guard on the outer UPDATE is defense in depth: a row that
// moved on since the scan is never claimed twice.
func (s *Store) ClaimImportJobs(ctx context.Context, limit int) ([]ImportJob, error) {
	if limit <= 0 {
		limit = 10
	}
	now := time.Now()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `UPDATE kbd_materials
		SET processing_status = 'extracting', updated_at = $3
		WHERE processing_status = $1 AND id IN (
			SELECT id FROM kbd_materials WHERE processing_status = $1 ORDER BY created_at LIMIT $2
		)
		RETURNING id, organization_id, source_type, source_ref, filename, mime_type, storage_key, extraction_metadata`,
		ImportStatusQueued, limit, now)
	if err != nil {
		return nil, fmt.Errorf("kbstore: claim import jobs: %w", err)
	}

	var out []ImportJob
	for rows.Next() {
		var j ImportJob
		var filename, mimeType, storageKey *string
		var meta string
		if err := rows.Scan(&j.ID, &j.OrganizationID, &j.SourceType, &j.SourceRef, &filename, &mimeType, &storageKey, &meta); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("kbstore: scan claimed import job: %w", err)
		}
		j.Filename, j.MimeType, j.StorageKey = strOrEmpty(filename), strOrEmpty(mimeType), strOrEmpty(storageKey)
		params, err := parseImportParams(meta)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		j.Params = params
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// FinishImportExtraction records pass 1's outcome on a claimed
// ('extracting') material. The worker that called ClaimImportJobs for this
// row is the only caller that will ever call this for it — processing_status
// already moved away from the contestable 'queued' state the moment it was
// claimed — so no further concurrency guard is needed beyond the WHERE
// clause documenting that assumption. Clears any LastError from a prior
// failed attempt on success (out.Status == "parsed").
func (s *Store) FinishImportExtraction(ctx context.Context, id uuid.UUID, out ExtractionOutcome) error {
	// The status, the extracted text and the cleared last_error are one change: a failure part way
	// must not leave a parsed material that still shows the previous attempt's error, or report an
	// error for an extraction that was stored.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	applied, err := mutateImportParams(ctx, tx, id, func(r *importRow) bool {
		if r.Status != "extracting" {
			return false
		}
		r.Status = out.Status
		r.Params.LastError = ""
		return true
	})
	if err != nil {
		return fmt.Errorf("kbstore: finish import extraction: %w", err)
	}
	if !applied {
		return ErrUnknownKind
	}
	if _, err := tx.Exec(ctx, `UPDATE kbd_materials
		SET extracted_text = $2,
		    visual_summary = CASE WHEN $3 <> '' THEN $3 ELSE visual_summary END
		WHERE id = $1`,
		id, out.ExtractedText, out.VisualSummary); err != nil {
		return fmt.Errorf("kbstore: finish import extraction: %w", err)
	}
	return tx.Commit(ctx)
}

// RequeueImportJob handles a REPORTED (not crash-recovered) extraction
// failure: bump attempts, and either requeue (processing_status back to
// 'queued' for a fresh claim — "transient extraction errors receive two or
// three retries," plan/playground.md) or, once maxAttempts is exhausted,
// set 'needs_human' (the same terminal state a permanent failure gets).
// terminal reports which branch was taken. The read-modify-write is a
// compare-and-swap (mutateImportParams), so a hypothetical concurrent call for
// the same id cannot double-count attempts — this row is normally exclusively
// owned by one worker, but the guard costs little and removes the assumption.
func (s *Store) RequeueImportJob(ctx context.Context, id uuid.UUID, cause string, maxAttempts int) (terminal bool, err error) {
	_, err = mutateImportParams(ctx, s.db, id, func(r *importRow) bool {
		r.Params.Attempts++
		r.Params.LastError = cause
		terminal = r.Params.Attempts >= maxAttempts
		r.Status = ImportStatusQueued
		if terminal {
			r.Status = "needs_human"
		}
		return true
	})
	if errors.Is(err, ErrUnknownKind) {
		return false, ErrUnknownKind
	}
	if err != nil {
		return false, err
	}
	return terminal, nil
}

// RecoverImportJobs re-queues every 'extracting' material whose updated_at
// is older than stuckBefore — a crash left it claimed but never finished —
// bumping its attempts; a row that has thereby exhausted maxAttempts is
// force-failed instead of requeued again (plan/playground.md: "A parse
// stuck beyond a timeout is force-failed"; 'failed', not 'needs_human',
// distinguishes "the infrastructure lost track of this job" from an
// ordinary reported extraction failure — see RequeueImportJob). Wrapped in
// one transaction: this package's single-connection pool means a
// concurrent recovery pass would otherwise be a real race (unlike
// FinishImportExtraction's single-owner assumption, a recovery scan and an
// about-to-finish worker CAN legitimately overlap).
func (s *Store) RecoverImportJobs(ctx context.Context, stuckBefore time.Time, maxAttempts, limit int) (requeued, failed int, err error) {
	if limit <= 0 {
		limit = 200
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT id FROM kbd_materials
		WHERE processing_status = 'extracting' AND updated_at < $1 ORDER BY updated_at LIMIT $2`,
		stuckBefore, limit)
	if err != nil {
		return 0, 0, fmt.Errorf("kbstore: find stuck import jobs: %w", err)
	}
	var candidates []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, 0, err
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	_ = rows.Close()

	for _, id := range candidates {
		var terminal bool
		applied, err := mutateImportParams(ctx, tx, id, func(r *importRow) bool {
			if r.Status != "extracting" {
				return false // finished or was cancelled since the scan: not stuck any more
			}
			r.Params.Attempts++
			terminal = r.Params.Attempts >= maxAttempts
			if terminal {
				r.Status = "failed"
				r.Params.LastError = "extraction timed out and exhausted its retry budget"
			} else {
				r.Status = ImportStatusQueued
				r.Params.LastError = "extraction timed out; retrying"
			}
			return true
		})
		if err != nil {
			return requeued, failed, err
		}
		if !applied {
			continue
		}
		if terminal {
			failed++
		} else {
			requeued++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return requeued, failed, nil
}

// mergeImportParams reads id's current extraction_metadata.import, applies
// mutate to the complete struct, and writes the complete struct back through
// mutateImportParams — mutate must only ever change specific fields on the
// struct it was handed, never construct a fresh one.
func (s *Store) mergeImportParams(ctx context.Context, id uuid.UUID, mutate func(*ImportParams)) error {
	_, err := mutateImportParams(ctx, s.db, id, func(r *importRow) bool {
		mutate(&r.Params)
		return true
	})
	return err
}

// ImportRunMaterials returns every material tagged with runID, org-scoped,
// oldest first.
func (s *Store) ImportRunMaterials(ctx context.Context, orgID, runID uuid.UUID) ([]ImportMaterial, error) {
	rows, err := s.db.Query(ctx, `SELECT id, source_type, source_ref, filename, mime_type, storage_key,
		extracted_text, customer_visibility, processing_status, created_at, updated_at, extraction_metadata
		FROM kbd_materials
		WHERE organization_id = $1 AND import_run_id = $2
		ORDER BY created_at`,
		orgID, runID.String())
	if err != nil {
		return nil, fmt.Errorf("kbstore: list import run materials: %w", err)
	}
	defer rows.Close()

	var out []ImportMaterial
	for rows.Next() {
		var m ImportMaterial
		var filename, mimeType, storageKey, visibility *string
		var meta string
		if err := rows.Scan(&m.ID, &m.SourceType, &m.SourceRef, &filename, &mimeType, &storageKey,
			&m.ExtractedText, &visibility, &m.ProcessingStatus, &m.CreatedAt, &m.UpdatedAt, &meta); err != nil {
			return nil, err
		}
		m.OrganizationID = orgID
		m.Filename, m.MimeType, m.StorageKey = strOrEmpty(filename), strOrEmpty(mimeType), strOrEmpty(storageKey)
		m.CustomerVisibility = strOrEmpty(visibility)
		params, err := parseImportParams(meta)
		if err != nil {
			return nil, err
		}
		m.Params = params
		out = append(out, m)
	}
	return out, rows.Err()
}

// CancelImportRun stops runID from progressing further, provided pass 2
// has not started yet. Every 'queued' (not yet claimed) or 'extracting'
// (mid pass-1) material moves to ImportStatusCancelled; the primary's
// synthesis is marked SynthesisCancelled too — even though pass 2 was
// never claimed — for the reason SynthesisCancelled's own doc comment
// gives.
//
// A material a worker already has claimed and is mid pass-1 (an in-flight
// extractor.Extract call) is NOT interrupted — it keeps running up to
// ExtractTimeout, exactly like every other 'extracting' row this package's
// own CAS convention already tolerates a race on: its eventual
// FinishImportExtraction/RequeueImportJob call finds the row no longer
// 'extracting' and simply discards the result (see those methods' own
// WHERE-clause CAS) — RequeueImportJob is the one exception (no CAS guard
// of its own), so a cancel landing in the split second between a reported
// extraction failure and RequeueImportJob's write can rarely be undone by
// it; narrow enough, and the same class of race RecoverImportJobs' own doc
// comment already accepts elsewhere in this package, to leave as is rather
// than widening this change to add one.
//
// Refuses (ErrImportRunNotCancelable) once the primary's synthesis has
// already been claimed (Params.Synthesis != nil): FinishImportSynthesis
// writes with NO CAS guard of its own (its own doc comment: claiming
// BeginImportSynthesis makes that call the run's only remaining writer),
// so a synthesis result landing after this would silently overwrite
// 'cancelled' back to built/failed/needs_human — unlike pass 1, there is
// no race-tolerant path here to rely on.
//
// found reports whether runID resolved to a run that was not ALREADY
// terminal — false is a benign no-op (the run finished naturally in the
// same instant), not an error.
func (s *Store) CancelImportRun(ctx context.Context, orgID, runID uuid.UUID) (found bool, err error) {
	materials, err := s.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		return false, err
	}
	if len(materials) == 0 {
		return false, ErrUnknownKind
	}
	var primary *ImportMaterial
	for i := range materials {
		if materials[i].Params.Primary {
			primary = &materials[i]
		}
	}
	if primary == nil {
		return false, ErrUnknownKind
	}
	if primary.Params.Synthesis.IsTerminal() {
		return false, nil
	}
	if primary.Params.Synthesis != nil {
		return false, ErrImportRunNotCancelable
	}

	now := time.Now()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The primary's synthesis marker goes first, guarded: if pass 2 was claimed since the
	// read above, the run can no longer be cancelled and nothing here commits.
	applied, err := mutateImportParams(ctx, tx, primary.ID, func(r *importRow) bool {
		if r.Params.Synthesis != nil {
			return false
		}
		r.Params.Synthesis = &SynthesisState{Status: SynthesisCancelled}
		return true
	})
	if err != nil {
		return false, fmt.Errorf("kbstore: cancel import run synthesis: %w", err)
	}
	if !applied {
		return false, ErrImportRunNotCancelable
	}

	if _, err := tx.Exec(ctx, `UPDATE kbd_materials
		SET processing_status = $3, updated_at = $4
		WHERE organization_id = $1
		  AND import_run_id = $2
		  AND processing_status IN ('queued', 'extracting')`,
		orgID, runID.String(), ImportStatusCancelled, now); err != nil {
		return false, fmt.Errorf("kbstore: cancel import run materials: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// RecentImportRuns returns a page of the org's import run ids (ordered by
// their primary material's created_at, newest first) — GET /kb/imports'
// listing source, for both the "just the latest run" caller (limit=1,
// offset=0) and KB-14's paginated history (limit=pageSize,
// offset=(page-1)*pageSize). total is the org's full run count, letting the
// caller compute page count without a second query.
func (s *Store) RecentImportRuns(ctx context.Context, orgID uuid.UUID, limit, offset int) (ids []uuid.UUID, total int, err error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx, `SELECT extraction_metadata FROM kbd_materials
		WHERE organization_id = $1 AND import_run_id IS NOT NULL
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("kbstore: list recent import runs: %w", err)
	}
	defer rows.Close()
	var all []uuid.UUID
	for rows.Next() {
		var meta string
		if err := rows.Scan(&meta); err != nil {
			return nil, 0, err
		}
		params, err := parseImportParams(meta)
		if err != nil {
			return nil, 0, err
		}
		if !params.Primary {
			continue
		}
		all = append(all, params.RunID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	total = len(all)
	if offset >= total {
		return []uuid.UUID{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

// ActiveImportRun returns the org's current non-terminal run's id, if any —
// POST /kb/imports' one-active-run-per-org 409 gate. A run counts as active
// until its primary material's synthesis reaches a terminal status
// (built/failed/needs_human); before pass 2 even starts, Synthesis is nil,
// which is also "not terminal" — so a run in the middle of pass-1
// extraction correctly still counts as active.
func (s *Store) ActiveImportRun(ctx context.Context, orgID uuid.UUID) (uuid.UUID, bool, error) {
	rows, err := s.db.Query(ctx, `SELECT extraction_metadata FROM kbd_materials
		WHERE organization_id = $1 AND import_run_id IS NOT NULL
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("kbstore: find active import run: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var meta string
		if err := rows.Scan(&meta); err != nil {
			return uuid.Nil, false, err
		}
		params, err := parseImportParams(meta)
		if err != nil {
			return uuid.Nil, false, err
		}
		if !params.Primary {
			continue
		}
		if !params.Synthesis.IsTerminal() {
			return params.RunID, true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, false, err
	}
	return uuid.Nil, false, nil
}

// PendingSynthesisPrimaries returns every primary material, across every
// organization, whose synthesis has not yet been claimed (Synthesis is
// nil/absent) but whose run_id is set — the periodic recovery scan's
// source for "did every material in this run finish pass 1 without anyone
// kicking off pass 2?" (a worker crash between FinishImportExtraction
// committing and the in-process maybeSynthesize call actually running is
// the gap this closes; RecoverStuckSynthesis only covers a run where
// synthesis WAS started and then got stuck, a different gap). The caller
// still has to check whether each run's siblings are all terminal before
// acting — this only narrows the global material table down to "primaries
// nobody has touched for pass 2 yet," which in a healthy system is a
// small, cheap set.
func (s *Store) PendingSynthesisPrimaries(ctx context.Context) ([]ImportMaterial, error) {
	rows, err := s.db.Query(ctx, `SELECT id, organization_id, source_type, source_ref, filename, mime_type, storage_key,
		extracted_text, customer_visibility, processing_status, extraction_metadata
		FROM kbd_materials
		WHERE import_run_id IS NOT NULL AND import_synthesis_status IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("kbstore: find pending synthesis primaries: %w", err)
	}
	defer rows.Close()

	var out []ImportMaterial
	for rows.Next() {
		var m ImportMaterial
		var filename, mimeType, storageKey, visibility *string
		var meta string
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.SourceType, &m.SourceRef, &filename, &mimeType, &storageKey,
			&m.ExtractedText, &visibility, &m.ProcessingStatus, &meta); err != nil {
			return nil, err
		}
		m.Filename, m.MimeType, m.StorageKey = strOrEmpty(filename), strOrEmpty(mimeType), strOrEmpty(storageKey)
		m.CustomerVisibility = strOrEmpty(visibility)
		params, err := parseImportParams(meta)
		if err != nil {
			return nil, err
		}
		if !params.Primary {
			continue
		}
		m.Params = params
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkImportBuilt flips every given material from 'parsed' to 'built' —
// pass 2 consumed its evidence (mirrors the legacy MarkMaterialsBuilt's
// exact semantics, scoped to this pipeline's own ids). A row that is not
// currently 'parsed' (e.g. needs_human/failed, correctly excluded from the
// evidence pass 2 actually read) is left untouched.
func (s *Store) MarkImportBuilt(ctx context.Context, ids []uuid.UUID) error {
	now := time.Now()
	for chunk := range slices.Chunk(ids, dbx.MaxInList) {
		list, args := dbx.InList([]any{now}, chunk)
		if _, err := s.db.Exec(ctx, `UPDATE kbd_materials
			SET processing_status = 'built', updated_at = $1
			WHERE id IN `+list+` AND processing_status = 'parsed'`, args...); err != nil {
			return err
		}
	}
	return nil
}

// BeginImportSynthesis atomically claims the run for pass-2 synthesis: it
// succeeds (claimed=true) only if the primary material's synthesis status
// is currently empty (never started). The claim is mutateImportParams'
// compare-and-swap, exactly like CompleteMaterialUpload's own
// 'uploaded'-guarded transition (mcp_media.go): of any number of concurrent
// calls for the SAME primaryID, one write matches the row as it was read and
// every other loses the swap, re-reads, finds the claim already made and
// reports claimed=false.
func (s *Store) BeginImportSynthesis(ctx context.Context, primaryID uuid.UUID) (token uuid.UUID, claimed bool, err error) {
	token = uuid.New()
	claimed, err = mutateImportParams(ctx, s.db, primaryID, func(r *importRow) bool {
		if r.Params.Synthesis != nil && r.Params.Synthesis.Status != "" {
			return false
		}
		r.Params.Synthesis = &SynthesisState{Status: SynthesisRunning, Token: token, StartedAt: time.Now()}
		return true
	})
	if errors.Is(err, ErrUnknownKind) {
		return token, false, nil // no such material: nothing was claimed
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("kbstore: begin import synthesis: %w", err)
	}
	return token, claimed, nil
}

// FinishImportSynthesis records pass 2's final outcome on the primary
// material — called once, by the same caller that successfully claimed
// BeginImportSynthesis (by the time this runs, that claim makes this call
// the run's only remaining writer of the synthesis sub-object, so no
// further CAS guard is needed here).
func (s *Store) FinishImportSynthesis(ctx context.Context, primaryID uuid.UUID, st SynthesisState) error {
	_, err := mutateImportParams(ctx, s.db, primaryID, func(r *importRow) bool {
		final := st
		r.Params.Synthesis = &final
		return true
	})
	if errors.Is(err, ErrUnknownKind) {
		return ErrUnknownKind
	}
	if err != nil {
		return fmt.Errorf("kbstore: finish import synthesis: %w", err)
	}
	return nil
}

// RecoverStuckSynthesis marks every primary material whose synthesis is
// still 'running' with a started_at older than stuckBefore as
// 'needs_human' — the one place this pipeline deliberately chooses "stop
// and ask" over "at least once": a crash between the upserts landing and
// MarkImportBuilt recording it must never silently re-run pass 2 (natural
// keys mean a re-run would update rather than duplicate, but it would bump
// base_version and could re-append a gallery image).
func (s *Store) RecoverStuckSynthesis(ctx context.Context, stuckBefore time.Time) (int, error) {
	rows, err := s.db.Query(ctx, `SELECT id FROM kbd_materials WHERE import_synthesis_status = $1`, SynthesisRunning)
	if err != nil {
		return 0, fmt.Errorf("kbstore: find stuck synthesis: %w", err)
	}
	var candidates []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	n := 0
	for _, id := range candidates {
		applied, err := mutateImportParams(ctx, s.db, id, func(r *importRow) bool {
			syn := r.Params.Synthesis
			if syn == nil || syn.Status != SynthesisRunning {
				return false // resolved between the scan above and here — leave it alone
			}
			if !syn.StartedAt.Before(stuckBefore) {
				return false // still within budget
			}
			syn.Status = SynthesisNeedsHuman
			return true
		})
		if err != nil {
			return n, err
		}
		if applied {
			n++
		}
	}
	return n, nil
}
