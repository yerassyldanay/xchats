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
// pass-2 synthesis state) lives in kbd_materials.extraction_metadata.import
// — a plain JSON association merged via SQLite's json_patch (RFC 7396
// merge-patch), the same "not a new table" choice recordProvenance already
// makes for provenance (mcp_media.go), just expressed at the SQL layer
// instead of Go's jsonShallowMerge so the concurrency-critical claim/CAS
// operations below can stay single-statement atomic (see ClaimImportJobs
// and BeginImportSynthesis's own doc comments for why that matters under
// this package's single-connection pool — internal/dbx's package doc).
// ImportParams/SynthesisState are deliberately marshaled with NO
// `omitempty` on any field: every writer here does a full read-mutate-write
// of the complete struct, and RFC 7396 treats an explicit `"field":""` as
// "set it" but an OMITTED field as "leave whatever is already there" — with
// omitempty, clearing a field back to its zero value (e.g. LastError on a
// successful retry) would silently fail to take effect.
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

// isTerminal reports whether st represents a finished run — built (pass 2
// landed something), failed (pass 2 gave up after a re-ask), or
// needs_human (recovery gave up on a stuck run). A nil state, or an empty
// Status, is NOT terminal — pass 2 has not even begun.
func (st *SynthesisState) isTerminal() bool {
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
	Params             ImportParams
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

// importMetaPatch is the {"import": ...} shape every json_patch call in
// this file merges into kbd_materials.extraction_metadata.
type importMetaPatch struct {
	Import ImportParams `json:"import"`
}

func marshalImportPatch(p ImportParams) (string, error) {
	b, err := json.Marshal(importMetaPatch{Import: p})
	if err != nil {
		return "", fmt.Errorf("kbstore: marshal import params: %w", err)
	}
	return string(b), nil
}

// synthesisMetaPatch is the {"import":{"synthesis": ...}} shape
// BeginImportSynthesis/FinishImportSynthesis merge — nested one level
// deeper than importMetaPatch so it touches ONLY the synthesis sub-object,
// leaving every sibling import.* field (run_id, provider, attempts, ...)
// untouched regardless of what this process currently has in memory for
// them.
type synthesisMetaPatch struct {
	Import struct {
		Synthesis SynthesisState `json:"synthesis"`
	} `json:"import"`
}

func marshalSynthesisPatch(st SynthesisState) (string, error) {
	var p synthesisMetaPatch
	p.Import.Synthesis = st
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("kbstore: marshal synthesis state: %w", err)
	}
	return string(b), nil
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
		patch, err := marshalImportPatch(params)
		if err != nil {
			return uuid.Nil, err
		}
		var id uuid.UUID
		err = s.db.QueryRow(ctx, `INSERT INTO kbd_materials
			(organization_id, source_type, source_ref, extraction_metadata, processing_status, customer_visibility)
			VALUES ($1, 'url', $2, $3, $4, 'invisible')
			RETURNING id`,
			orgID, in.URL, patch, ImportStatusQueued).Scan(&id)
		if err != nil {
			return uuid.Nil, fmt.Errorf("kbstore: enqueue url import: %w", err)
		}
		return id, nil
	}

	if in.MaterialID == uuid.Nil {
		return uuid.Nil, errors.New("kbstore: EnqueueImport requires either a URL or a staged MaterialID")
	}
	patch, err := marshalImportPatch(params)
	if err != nil {
		return uuid.Nil, err
	}
	tag, err := s.db.Exec(ctx, `UPDATE kbd_materials
		SET processing_status = $4,
		    extraction_metadata = json_patch(extraction_metadata, $3),
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND organization_id = $2 AND processing_status = 'parsed'`,
		in.MaterialID, orgID, patch, ImportStatusQueued)
	if err != nil {
		return uuid.Nil, fmt.Errorf("kbstore: enqueue file import: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return uuid.Nil, ErrUnknownKind
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
	patch, err := marshalImportPatch(params)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `UPDATE kbd_materials
		SET extraction_metadata = json_patch(extraction_metadata, $2),
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, id, patch)
	if err != nil {
		return fmt.Errorf("kbstore: tag import material: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUnknownKind
	}
	return nil
}

// ClaimImportJobs atomically claims up to limit 'queued' materials (oldest
// first), transitioning each straight to 'extracting' — a single
// UPDATE ... WHERE id IN (subquery) ... RETURNING statement. This is atomic
// under this package's single-connection pool (internal/dbx's package doc:
// "every writer serializes through Go's own pool queueing"): the *Rows this
// returns holds that one connection until fully drained/closed, so a second
// concurrent caller's own Query() call cannot even begin executing until
// this one has completed — two concurrent callers therefore always return
// disjoint sets, never double-claiming the same row.
func (s *Store) ClaimImportJobs(ctx context.Context, limit int) ([]ImportJob, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(ctx, `UPDATE kbd_materials
		SET processing_status = 'extracting', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id IN (
			SELECT id FROM kbd_materials WHERE processing_status = $1 ORDER BY created_at LIMIT $2
		)
		RETURNING id, organization_id, source_type, source_ref, filename, mime_type, storage_key, extraction_metadata`,
		ImportStatusQueued, limit)
	if err != nil {
		return nil, fmt.Errorf("kbstore: claim import jobs: %w", err)
	}
	defer rows.Close()

	var out []ImportJob
	for rows.Next() {
		var j ImportJob
		var filename, mimeType, storageKey *string
		var meta string
		if err := rows.Scan(&j.ID, &j.OrganizationID, &j.SourceType, &j.SourceRef, &filename, &mimeType, &storageKey, &meta); err != nil {
			return nil, fmt.Errorf("kbstore: scan claimed import job: %w", err)
		}
		j.Filename, j.MimeType, j.StorageKey = strOrEmpty(filename), strOrEmpty(mimeType), strOrEmpty(storageKey)
		params, err := parseImportParams(meta)
		if err != nil {
			return nil, err
		}
		j.Params = params
		out = append(out, j)
	}
	return out, rows.Err()
}

// FinishImportExtraction records pass 1's outcome on a claimed
// ('extracting') material. The worker that called ClaimImportJobs for this
// row is the only caller that will ever call this for it — processing_status
// already moved away from the contestable 'queued' state the moment it was
// claimed — so no further concurrency guard is needed beyond the WHERE
// clause documenting that assumption. Clears any LastError from a prior
// failed attempt on success (out.Status == "parsed").
func (s *Store) FinishImportExtraction(ctx context.Context, id uuid.UUID, out ExtractionOutcome) error {
	tag, err := s.db.Exec(ctx, `UPDATE kbd_materials
		SET processing_status = $2, extracted_text = $3,
		    visual_summary = CASE WHEN $4 <> '' THEN $4 ELSE visual_summary END,
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND processing_status = 'extracting'`,
		id, out.Status, out.ExtractedText, out.VisualSummary)
	if err != nil {
		return fmt.Errorf("kbstore: finish import extraction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUnknownKind
	}
	return s.mergeImportParams(ctx, id, func(p *ImportParams) { p.LastError = "" })
}

// RequeueImportJob handles a REPORTED (not crash-recovered) extraction
// failure: bump attempts, and either requeue (processing_status back to
// 'queued' for a fresh claim — "transient extraction errors receive two or
// three retries," plan/playground.md) or, once maxAttempts is exhausted,
// set 'needs_human' (the same terminal state a permanent failure gets).
// terminal reports which branch was taken. Wrapped in a transaction so a
// hypothetical concurrent call for the same id cannot double-count attempts
// — this row is normally exclusively owned by one worker, but the guard
// costs little and removes the assumption.
func (s *Store) RequeueImportJob(ctx context.Context, id uuid.UUID, cause string, maxAttempts int) (terminal bool, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var meta string
	if err := tx.QueryRow(ctx, `SELECT extraction_metadata FROM kbd_materials WHERE id = $1`, id).Scan(&meta); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return false, ErrUnknownKind
		}
		return false, err
	}
	params, err := parseImportParams(meta)
	if err != nil {
		return false, err
	}
	params.Attempts++
	params.LastError = cause
	terminal = params.Attempts >= maxAttempts
	newStatus := ImportStatusQueued
	if terminal {
		newStatus = "needs_human"
	}
	patch, err := marshalImportPatch(params)
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE kbd_materials
		SET processing_status = $2, extraction_metadata = json_patch(extraction_metadata, $3),
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, id, newStatus, patch); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
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
	defer tx.Rollback(ctx)

	type stuckRow struct {
		id   uuid.UUID
		meta string
	}
	rows, err := tx.Query(ctx, `SELECT id, extraction_metadata FROM kbd_materials
		WHERE processing_status = 'extracting' AND updated_at < $1 ORDER BY updated_at LIMIT $2`,
		dbx.FormatTime(stuckBefore), limit)
	if err != nil {
		return 0, 0, fmt.Errorf("kbstore: find stuck import jobs: %w", err)
	}
	var candidates []stuckRow
	for rows.Next() {
		var c stuckRow
		if err := rows.Scan(&c.id, &c.meta); err != nil {
			rows.Close()
			return 0, 0, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, err
	}
	rows.Close()

	for _, c := range candidates {
		params, perr := parseImportParams(c.meta)
		if perr != nil {
			return requeued, failed, perr
		}
		params.Attempts++
		terminal := params.Attempts >= maxAttempts
		newStatus := ImportStatusQueued
		if terminal {
			newStatus = "failed"
			params.LastError = "extraction timed out and exhausted its retry budget"
		} else {
			params.LastError = "extraction timed out; retrying"
		}
		patch, perr := marshalImportPatch(params)
		if perr != nil {
			return requeued, failed, perr
		}
		if _, err := tx.Exec(ctx, `UPDATE kbd_materials
			SET processing_status = $2, extraction_metadata = json_patch(extraction_metadata, $3),
			    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE id = $1`, c.id, newStatus, patch); err != nil {
			return requeued, failed, err
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
// mutate to the complete struct, and writes the complete struct back via
// json_patch — see the package doc comment for why mutate must only ever
// change specific fields on the struct it was handed, never construct a
// fresh one.
func (s *Store) mergeImportParams(ctx context.Context, id uuid.UUID, mutate func(*ImportParams)) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var meta string
	if err := tx.QueryRow(ctx, `SELECT extraction_metadata FROM kbd_materials WHERE id = $1`, id).Scan(&meta); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return ErrUnknownKind
		}
		return err
	}
	params, err := parseImportParams(meta)
	if err != nil {
		return err
	}
	mutate(&params)
	patch, err := marshalImportPatch(params)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE kbd_materials
		SET extraction_metadata = json_patch(extraction_metadata, $2),
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, id, patch); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ImportRunMaterials returns every material tagged with runID, org-scoped,
// oldest first.
func (s *Store) ImportRunMaterials(ctx context.Context, orgID, runID uuid.UUID) ([]ImportMaterial, error) {
	rows, err := s.db.Query(ctx, `SELECT id, source_type, source_ref, filename, mime_type, storage_key,
		extracted_text, customer_visibility, processing_status, created_at, extraction_metadata
		FROM kbd_materials
		WHERE organization_id = $1 AND json_extract(extraction_metadata, '$.import.run_id') = $2
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
		var createdAt string
		var meta string
		if err := rows.Scan(&m.ID, &m.SourceType, &m.SourceRef, &filename, &mimeType, &storageKey,
			&m.ExtractedText, &visibility, &m.ProcessingStatus, &createdAt, &meta); err != nil {
			return nil, err
		}
		m.OrganizationID = orgID
		m.Filename, m.MimeType, m.StorageKey = strOrEmpty(filename), strOrEmpty(mimeType), strOrEmpty(storageKey)
		m.CustomerVisibility = strOrEmpty(visibility)
		if t, err := dbx.ParseTime(createdAt); err == nil {
			m.CreatedAt = t
		}
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
	if primary.Params.Synthesis.isTerminal() {
		return false, nil
	}
	if primary.Params.Synthesis != nil {
		return false, ErrImportRunNotCancelable
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE kbd_materials
		SET processing_status = $3, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1
		  AND json_extract(extraction_metadata, '$.import.run_id') = $2
		  AND processing_status IN ('queued', 'extracting')`,
		orgID, runID.String(), ImportStatusCancelled); err != nil {
		return false, fmt.Errorf("kbstore: cancel import run materials: %w", err)
	}

	patch, err := marshalSynthesisPatch(SynthesisState{Status: SynthesisCancelled})
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE kbd_materials
		SET extraction_metadata = json_patch(extraction_metadata, $3),
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND organization_id = $2`,
		primary.ID, orgID, patch); err != nil {
		return false, fmt.Errorf("kbstore: cancel import run synthesis: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// RecentImportRuns returns the org's most recent import run ids (ordered by
// their primary material's created_at, newest first), capped at limit —
// GET /kb/imports' listing source.
func (s *Store) RecentImportRuns(ctx context.Context, orgID uuid.UUID, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(ctx, `SELECT extraction_metadata FROM kbd_materials
		WHERE organization_id = $1 AND json_extract(extraction_metadata, '$.import.run_id') IS NOT NULL
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("kbstore: list recent import runs: %w", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var meta string
		if err := rows.Scan(&meta); err != nil {
			return nil, err
		}
		params, err := parseImportParams(meta)
		if err != nil {
			return nil, err
		}
		if !params.Primary {
			continue
		}
		out = append(out, params.RunID)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// ActiveImportRun returns the org's current non-terminal run's id, if any —
// POST /kb/imports' one-active-run-per-org 409 gate. A run counts as active
// until its primary material's synthesis reaches a terminal status
// (built/failed/needs_human); before pass 2 even starts, Synthesis is nil,
// which is also "not terminal" — so a run in the middle of pass-1
// extraction correctly still counts as active.
func (s *Store) ActiveImportRun(ctx context.Context, orgID uuid.UUID) (uuid.UUID, bool, error) {
	rows, err := s.db.Query(ctx, `SELECT extraction_metadata FROM kbd_materials
		WHERE organization_id = $1 AND json_extract(extraction_metadata, '$.import.run_id') IS NOT NULL
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
		if !params.Synthesis.isTerminal() {
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
		WHERE json_extract(extraction_metadata, '$.import.run_id') IS NOT NULL
		  AND json_extract(extraction_metadata, '$.import.synthesis') IS NULL`)
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
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `UPDATE kbd_materials
		SET processing_status = 'built', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id IN (SELECT value FROM json_each($1)) AND processing_status = 'parsed'`,
		dbx.UUIDArray(ids))
	return err
}

// BeginImportSynthesis atomically claims the run for pass-2 synthesis: it
// succeeds (claimed=true) only if the primary material's synthesis status
// is currently empty (never started) — a single UPDATE ... WHERE
// json_extract(...) guard, atomic under this package's single-connection
// pool exactly like CompleteMaterialUpload's own 'uploaded'-guarded
// transition (mcp_media.go). A second concurrent call for the SAME
// primaryID always sees claimed=false: by the time its own WHERE clause
// evaluates, the first call's write (whichever one actually ran first —
// the single-connection pool makes them strictly ordered, never
// interleaved) has already left no unclaimed state for it to match.
func (s *Store) BeginImportSynthesis(ctx context.Context, primaryID uuid.UUID) (token uuid.UUID, claimed bool, err error) {
	token = uuid.New()
	patch, err := marshalSynthesisPatch(SynthesisState{Status: SynthesisRunning, Token: token, StartedAt: time.Now()})
	if err != nil {
		return uuid.Nil, false, err
	}
	tag, err := s.db.Exec(ctx, `UPDATE kbd_materials
		SET extraction_metadata = json_patch(extraction_metadata, $2),
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1
		  AND (json_extract(extraction_metadata, '$.import.synthesis.status') IS NULL
		       OR json_extract(extraction_metadata, '$.import.synthesis.status') = '')`,
		primaryID, patch)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("kbstore: begin import synthesis: %w", err)
	}
	return token, tag.RowsAffected() == 1, nil
}

// FinishImportSynthesis records pass 2's final outcome on the primary
// material — called once, by the same caller that successfully claimed
// BeginImportSynthesis (by the time this runs, that claim makes this call
// the run's only remaining writer of the synthesis sub-object, so no
// further CAS guard is needed here).
func (s *Store) FinishImportSynthesis(ctx context.Context, primaryID uuid.UUID, st SynthesisState) error {
	patch, err := marshalSynthesisPatch(st)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `UPDATE kbd_materials
		SET extraction_metadata = json_patch(extraction_metadata, $2),
		    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, primaryID, patch)
	if err != nil {
		return fmt.Errorf("kbstore: finish import synthesis: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUnknownKind
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
	type stuckRow struct {
		id   uuid.UUID
		meta string
	}
	rows, err := s.db.Query(ctx, `SELECT id, extraction_metadata FROM kbd_materials
		WHERE json_extract(extraction_metadata, '$.import.synthesis.status') = $1`, SynthesisRunning)
	if err != nil {
		return 0, fmt.Errorf("kbstore: find stuck synthesis: %w", err)
	}
	var candidates []stuckRow
	for rows.Next() {
		var c stuckRow
		if err := rows.Scan(&c.id, &c.meta); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	n := 0
	for _, c := range candidates {
		params, err := parseImportParams(c.meta)
		if err != nil {
			return n, err
		}
		if params.Synthesis == nil || params.Synthesis.Status != SynthesisRunning {
			continue // resolved between the scan above and here — leave it alone
		}
		if !params.Synthesis.StartedAt.Before(stuckBefore) {
			continue // still within budget
		}
		params.Synthesis.Status = SynthesisNeedsHuman
		patch, err := marshalSynthesisPatch(*params.Synthesis)
		if err != nil {
			return n, err
		}
		tag, err := s.db.Exec(ctx, `UPDATE kbd_materials
			SET extraction_metadata = json_patch(extraction_metadata, $2),
			    updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE id = $1 AND json_extract(extraction_metadata, '$.import.synthesis.status') = $3`,
			c.id, patch, SynthesisRunning)
		if err != nil {
			return n, err
		}
		if tag.RowsAffected() == 1 {
			n++
		}
	}
	return n, nil
}
