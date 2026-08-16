package kbimport

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// claimBatch claims a batch of queued jobs and hands each to the worker
// pool via s.claimed — a full channel only delays a job already durably
// 'extracting' in kbd_materials, never loses it (mirrors
// internal/automation/scheduler.go's own enqueue doc comment).
func (s *Service) claimBatch(ctx context.Context) {
	jobs, err := s.deps.KB.ClaimImportJobs(ctx, s.cfg.Workers*4)
	if err != nil {
		s.deps.Log.Error("kbimport: claim import jobs", "err", err)
		return
	}
	if len(jobs) > 0 {
		s.notify()
	}
	for _, j := range jobs {
		select {
		case s.claimed <- j:
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
			s.deps.Log.Warn("kbimport: worker pool full; job stays 'extracting' for the next recovery pass", "material_id", j.ID)
		}
	}
}

// recover runs the crash-recovery reconcile pass: stuck 'extracting' jobs
// (RecoverImportJobs), stuck 'running' synthesis (RecoverStuckSynthesis),
// and any run whose pass 1 finished without pass 2 ever being kicked off
// (a worker could crash between FinishImportExtraction committing and the
// in-process maybeSynthesize call running — PendingSynthesisPrimaries
// closes exactly that gap).
func (s *Service) recover(ctx context.Context) {
	if requeued, failed, err := s.deps.KB.RecoverImportJobs(ctx, time.Now().Add(-s.cfg.StuckAfter), s.cfg.MaxAttempts, 200); err != nil {
		s.deps.Log.Error("kbimport: recover import jobs", "err", err)
	} else if requeued > 0 || failed > 0 {
		s.deps.Log.Info("kbimport: recovered stuck extraction jobs", "requeued", requeued, "failed", failed)
		s.notify()
	}

	if n, err := s.deps.KB.RecoverStuckSynthesis(ctx, time.Now().Add(-s.cfg.StuckAfter)); err != nil {
		s.deps.Log.Error("kbimport: recover stuck synthesis", "err", err)
	} else if n > 0 {
		s.deps.Log.Info("kbimport: recovered stuck synthesis runs", "count", n)
		s.notify()
	}

	pending, err := s.deps.KB.PendingSynthesisPrimaries(ctx)
	if err != nil {
		s.deps.Log.Error("kbimport: find pending synthesis primaries", "err", err)
		return
	}
	for _, p := range pending {
		s.maybeSynthesize(ctx, p.OrganizationID, p.Params.RunID)
	}
}

// runWorker pulls claimed jobs and processes them until ctx is done.
func (s *Service) runWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-s.claimed:
			if !ok {
				return
			}
			s.processJob(ctx, job)
		}
	}
}

// processJob runs pass 1 for one claimed material, records the outcome,
// and — since this may be the run's last material to finish — checks
// whether the run is now ready for pass 2.
func (s *Service) processJob(ctx context.Context, job kbstore.ImportJob) {
	ectx, cancel := context.WithTimeout(ctx, s.cfg.ExtractTimeout)
	outcome, imageIDs, extractErr := s.extractOne(ectx, job)
	cancel()

	if extractErr != nil {
		terminal, err := s.deps.KB.RequeueImportJob(ctx, job.ID, extractErr.Error(), s.cfg.MaxAttempts)
		if err != nil {
			s.deps.Log.Error("kbimport: requeue failed job", "material_id", job.ID, "err", err)
		}
		s.deps.Log.Warn("kbimport: extraction failed", "material_id", job.ID, "provider", job.Params.Provider, "terminal", terminal, "err", extractErr)
	} else {
		if err := s.deps.KB.FinishImportExtraction(ctx, job.ID, outcome); err != nil {
			s.deps.Log.Error("kbimport: finish extraction", "material_id", job.ID, "err", err)
		}
	}
	_ = imageIDs
	s.notify()
	s.maybeSynthesize(ctx, job.OrganizationID, job.Params.RunID)
}

// extractOne resolves the claimed job's provider, runs the extraction, and
// — for a URL/HTML result with embedded images — downloads and stages them
// (images.go), tagged to this job's material as parent. Returns the
// ExtractionOutcome to record and the staged image ids (informational —
// FinishImportExtraction does not need them; the run status endpoint reads
// them back via ImportRunMaterials instead).
func (s *Service) extractOne(ctx context.Context, job kbstore.ImportJob) (kbstore.ExtractionOutcome, []uuid.UUID, error) {
	provider, err := s.resolveProvider(ctx, job.Params.Provider)
	if err != nil {
		return kbstore.ExtractionOutcome{}, nil, err
	}

	src := extractor.Source{Kind: extractor.SourceURL, URL: job.SourceRef}
	if job.SourceType == "file" {
		data, err := s.readMaterialBytes(ctx, job)
		if err != nil {
			return kbstore.ExtractionOutcome{}, nil, err
		}
		src = extractor.Source{Kind: extractor.SourceFile, Filename: job.Filename, MimeType: job.MimeType, Data: data}
	}
	if !provider.Supports(src) {
		return kbstore.ExtractionOutcome{}, nil, errors.New("provider no longer supports this material")
	}

	result, err := provider.Extract(ctx, src)
	if err != nil {
		return kbstore.ExtractionOutcome{}, nil, err
	}

	var imageIDs []uuid.UUID
	if len(result.Images) > 0 {
		runBudget := s.cfg.MaxRunImageBytes
		imageIDs = s.downloadEmbeddedImages(ctx, job.OrganizationID, job.Params.RunID, job.ID, result.Images, &runBudget)
	}

	status := "parsed"
	if result.Text == "" && len(imageIDs) == 0 && job.MimeType != "" && kindOfMimeIsImage(job.MimeType) {
		status = "parsed" // the material IS the artifact — an empty extracted_text is expected, not a failure
	} else if result.Text == "" && len(result.Images) == 0 {
		status = "needs_human"
	}
	return kbstore.ExtractionOutcome{Status: status, ExtractedText: truncateRunes(result.Text, s.cfg.MaxTextPerMaterial)}, imageIDs, nil
}

func kindOfMimeIsImage(mime string) bool { return kbstore.KindOfMime(mime) == "image" }

// readMaterialBytes loads a file job's stored bytes for the provider to
// read — the same blob.Store the rest of this app already resolves
// storage_key through (see handleKBMaterialContent, mcp_media.go).
func (s *Service) readMaterialBytes(ctx context.Context, job kbstore.ImportJob) ([]byte, error) {
	if job.StorageKey == "" {
		return nil, errors.New("material has no stored bytes")
	}
	data, _, err := s.deps.Blob.Get(job.StorageKey)
	if err != nil {
		return nil, err
	}
	return data, nil
}
