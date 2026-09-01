package kbstore_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// --- EnqueueImport -----------------------------------------------------------

func TestEnqueueImport_URL_StartsQueued(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
		RunID: runID, Provider: "firecrawl", TargetType: "auto", Primary: true, Handle: "evidence.1",
		URL: "https://vendor.example/product",
	})
	if err != nil {
		t.Fatalf("EnqueueImport: %v", err)
	}

	jobs, err := kb.ClaimImportJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimImportJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != id {
		t.Fatalf("expected exactly the enqueued job to be claimable, got %+v", jobs)
	}
	if jobs[0].SourceType != "url" || jobs[0].SourceRef != "https://vendor.example/product" {
		t.Errorf("job = %+v, want the url/source_ref set", jobs[0])
	}
	if jobs[0].Params.RunID != runID || jobs[0].Params.Provider != "firecrawl" || !jobs[0].Params.Primary {
		t.Errorf("job.Params = %+v, want the enqueued params round-tripped", jobs[0].Params)
	}
}

func TestEnqueueImport_File_CASFromParsedToQueued(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	matID, err := kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: "price-list.pdf", MimeType: "application/pdf", SizeBytes: 100,
	})
	if err != nil {
		t.Fatalf("CreateUploadMaterial: %v", err)
	}

	// Not yet 'parsed' (bytes haven't landed) — CAS must refuse.
	if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "llamaparse", TargetType: "products", MaterialID: matID}); err != kbstore.ErrUnknownKind {
		t.Fatalf("EnqueueImport before upload completed: err = %v, want ErrUnknownKind", err)
	}

	if err := kb.CompleteMaterialUpload(ctx, matID, "disk", "org/x/"+matID.String(), 100, ""); err != nil {
		t.Fatalf("CompleteMaterialUpload: %v", err)
	}

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
		Provider: "llamaparse", TargetType: "products", Primary: true, Handle: "upload.1", MaterialID: matID,
	})
	if err != nil {
		t.Fatalf("EnqueueImport: %v", err)
	}
	if id != matID {
		t.Fatalf("EnqueueImport returned %s, want the same material id %s", id, matID)
	}

	jobs, err := kb.ClaimImportJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimImportJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].SourceType != "file" || jobs[0].MimeType != "application/pdf" {
		t.Fatalf("jobs = %+v, want the file job claimable with its mime type", jobs)
	}

	// Re-enqueueing an already-queued/extracting material must fail closed.
	if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "llamaparse", TargetType: "products", MaterialID: matID}); err != kbstore.ErrUnknownKind {
		t.Fatalf("re-EnqueueImport: err = %v, want ErrUnknownKind", err)
	}
}

func TestEnqueueImport_File_WrongOrg(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	ctx := context.Background()
	otherOrg, err := st.SeedOrganization(ctx, "other-org")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}

	matID, err := kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{Filename: "f.txt", MimeType: "text/plain", SizeBytes: 3})
	if err != nil {
		t.Fatal(err)
	}
	if err := kb.CompleteMaterialUpload(ctx, matID, "disk", "k", 3, ""); err != nil {
		t.Fatal(err)
	}

	if _, err := kb.EnqueueImport(ctx, otherOrg.ID, kbstore.ImportInput{Provider: "native", TargetType: "auto", MaterialID: matID}); err != kbstore.ErrUnknownKind {
		t.Fatalf("cross-org EnqueueImport: err = %v, want ErrUnknownKind", err)
	}
}

// --- TagImportMaterial (embedded images) --------------------------------------

func TestTagImportMaterial(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()
	parent := uuid.New()

	matID, err := kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: "photo.jpg", MimeType: "image/jpeg", SizeBytes: 10, CustomerVisibility: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := kb.CompleteMaterialUpload(ctx, matID, "disk", "k", 10, ""); err != nil {
		t.Fatal(err)
	}
	if err := kb.TagImportMaterial(ctx, matID, kbstore.ImportParams{RunID: runID, ParentID: &parent, Handle: "upload.2"}); err != nil {
		t.Fatalf("TagImportMaterial: %v", err)
	}

	materials, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatalf("ImportRunMaterials: %v", err)
	}
	if len(materials) != 1 || materials[0].ID != matID {
		t.Fatalf("materials = %+v, want the tagged image", materials)
	}
	if materials[0].ProcessingStatus != "parsed" {
		t.Errorf("ProcessingStatus = %q, want parsed (never queued — nothing to extract)", materials[0].ProcessingStatus)
	}
	if materials[0].Params.Handle != "upload.2" || materials[0].Params.ParentID == nil || *materials[0].Params.ParentID != parent {
		t.Errorf("Params = %+v, want handle/parent_id round-tripped", materials[0].Params)
	}
}

func TestTagImportMaterial_UnknownID(t *testing.T) {
	kb, _, _, _ := newTestKB(t)
	if err := kb.TagImportMaterial(context.Background(), uuid.New(), kbstore.ImportParams{}); err != kbstore.ErrUnknownKind {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
}

// --- ClaimImportJobs concurrency ----------------------------------------------

// TestClaimImportJobs_ConcurrentCallersReturnDisjointSets is the pipeline's
// core durability invariant: the kbd_materials row IS the queue, and no two
// workers may ever believe they both own the same job.
func TestClaimImportJobs_ConcurrentCallersReturnDisjointSets(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	const total = 30
	want := map[uuid.UUID]bool{}
	for i := 0; i < total; i++ {
		id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
			Provider: "native", TargetType: "auto", URL: fmt.Sprintf("https://example.com/page-%d", i),
		})
		if err != nil {
			t.Fatalf("EnqueueImport #%d: %v", i, err)
		}
		want[id] = true
	}

	const workers = 6
	var wg sync.WaitGroup
	results := make([][]kbstore.ImportJob, workers)
	errs := make([]error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			results[w], errs[w] = kb.ClaimImportJobs(ctx, 10)
		}(w)
	}
	wg.Wait()

	seen := map[uuid.UUID]int{}
	claimedTotal := 0
	for w, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: ClaimImportJobs: %v", w, err)
		}
		for _, j := range results[w] {
			seen[j.ID]++
			claimedTotal++
		}
	}
	if claimedTotal != total {
		t.Fatalf("claimed %d jobs across all workers, want exactly %d (every queued row, once)", claimedTotal, total)
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("job %s was claimed %d times, want exactly 1", id, count)
		}
		if !want[id] {
			t.Errorf("job %s was not one of the enqueued ids", id)
		}
	}
	if len(seen) != total {
		t.Fatalf("got %d distinct claimed ids, want %d", len(seen), total)
	}

	// A follow-up claim finds nothing left — every row moved to 'extracting'.
	rest, err := kb.ClaimImportJobs(ctx, 100)
	if err != nil {
		t.Fatalf("ClaimImportJobs (drained): %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no jobs left to claim, got %d", len(rest))
	}
}

// --- RecoverImportJobs ---------------------------------------------------------

func TestRecoverImportJobs_RequeuesStaleRowAndBumpsAttempts(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil { // -> 'extracting'
		t.Fatal(err)
	}

	requeued, failed, err := kb.RecoverImportJobs(ctx, time.Now().Add(time.Minute), 3, 100)
	if err != nil {
		t.Fatalf("RecoverImportJobs: %v", err)
	}
	if requeued != 1 || failed != 0 {
		t.Fatalf("requeued=%d failed=%d, want 1/0", requeued, failed)
	}

	jobs, err := kb.ClaimImportJobs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ID != id {
		t.Fatalf("expected the recovered row to be claimable again, got %+v", jobs)
	}
	if jobs[0].Params.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1 after exactly one recovery cycle", jobs[0].Params.Attempts)
	}
}

func TestRecoverImportJobs_FailsPastMaxAttempts(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}

	const maxAttempts = 3
	// Simulate maxAttempts-1 prior stuck cycles by recovering repeatedly,
	// re-claiming between each so the row is 'extracting' (stuck) again.
	for i := 0; i < maxAttempts-1; i++ {
		if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
			t.Fatalf("claim #%d: %v", i, err)
		}
		requeued, failed, err := kb.RecoverImportJobs(ctx, time.Now().Add(time.Minute), maxAttempts, 100)
		if err != nil {
			t.Fatalf("recover #%d: %v", i, err)
		}
		if requeued != 1 || failed != 0 {
			t.Fatalf("recover #%d: requeued=%d failed=%d, want 1/0 (attempt %d of %d)", i, requeued, failed, i+1, maxAttempts)
		}
	}

	// One more stuck cycle exhausts the budget.
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}
	requeued, failed, err := kb.RecoverImportJobs(ctx, time.Now().Add(time.Minute), maxAttempts, 100)
	if err != nil {
		t.Fatalf("final recover: %v", err)
	}
	if requeued != 0 || failed != 1 {
		t.Fatalf("final recover: requeued=%d failed=%d, want 0/1 (budget exhausted)", requeued, failed)
	}

	jobs, err := kb.ClaimImportJobs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("a failed row must never be claimable again, got %+v", jobs)
	}

	mats, err := kb.ListLiveMaterials(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[uuid.UUID]kbstore.Material{}
	for _, m := range mats {
		byID[m.ID] = m
	}
	if byID[id].ProcessingStatus != "failed" {
		t.Fatalf("ProcessingStatus = %q, want failed", byID[id].ProcessingStatus)
	}
}

func TestRecoverImportJobs_IgnoresFreshRows(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", URL: "https://example.com/x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil { // now 'extracting', updated_at = now
		t.Fatal(err)
	}

	// stuckBefore in the PAST — this row's updated_at is not older than it.
	requeued, failed, err := kb.RecoverImportJobs(ctx, time.Now().Add(-time.Hour), 3, 100)
	if err != nil {
		t.Fatal(err)
	}
	if requeued != 0 || failed != 0 {
		t.Fatalf("requeued=%d failed=%d, want 0/0 for a row that is not actually stuck", requeued, failed)
	}
}

// --- BeginImportSynthesis / FinishImportSynthesis / RecoverStuckSynthesis ----

func TestBeginImportSynthesis_SecondCallNotClaimed(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}

	token1, claimed1, err := kb.BeginImportSynthesis(ctx, primaryID)
	if err != nil {
		t.Fatalf("first BeginImportSynthesis: %v", err)
	}
	if !claimed1 {
		t.Fatal("expected the first call to claim the run")
	}
	if token1 == uuid.Nil {
		t.Fatal("expected a non-nil token")
	}

	token2, claimed2, err := kb.BeginImportSynthesis(ctx, primaryID)
	if err != nil {
		t.Fatalf("second BeginImportSynthesis: %v", err)
	}
	if claimed2 {
		t.Fatal("expected the second call to see claimed=false")
	}
	if token2 == token1 {
		t.Error("a rejected claim should not echo back the winning token (it must not be trusted as if it had won)")
	}
}

func TestBeginImportSynthesis_ConcurrentCallersExactlyOneWins(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 8
	var wg sync.WaitGroup
	claimed := make([]bool, attempts)
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, claimed[i], errs[i] = kb.BeginImportSynthesis(ctx, primaryID)
		}(i)
	}
	wg.Wait()

	wins := 0
	for i, err := range errs {
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if claimed[i] {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("exactly one concurrent BeginImportSynthesis call must win, got %d", wins)
	}
}

func TestFinishImportSynthesis_RoundTrips(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := kb.BeginImportSynthesis(ctx, primaryID); err != nil || !claimed {
		t.Fatalf("BeginImportSynthesis: claimed=%v err=%v", claimed, err)
	}

	st := kbstore.SynthesisState{
		Status:  kbstore.SynthesisBuilt,
		Notes:   "1 из 2 материалов использованы",
		Applied: []kbstore.AppliedUpsert{{Tool: "kb_product_upsert", Type: "product", Key: "widget-1", Created: true}},
		Dropped: []kbstore.DroppedUpsert{{Tool: "kb_tariff_upsert", Reason: "pricing_type is required"}},
		Usage:   kbstore.TokenUsage{PromptTokens: 1200, CompletionTokens: 300},
	}
	if err := kb.FinishImportSynthesis(ctx, primaryID, st); err != nil {
		t.Fatalf("FinishImportSynthesis: %v", err)
	}

	materials, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(materials) != 1 {
		t.Fatalf("materials = %+v", materials)
	}
	got := materials[0].Params.Synthesis
	if got == nil || got.Status != kbstore.SynthesisBuilt || got.Notes != st.Notes {
		t.Fatalf("Synthesis = %+v, want %+v round-tripped", got, st)
	}
	if len(got.Applied) != 1 || got.Applied[0].Key != "widget-1" {
		t.Fatalf("Applied = %+v", got.Applied)
	}
	if len(got.Dropped) != 1 || got.Dropped[0].Reason != "pricing_type is required" {
		t.Fatalf("Dropped = %+v", got.Dropped)
	}

	// The run is now terminal — ActiveImportRun must no longer report it.
	_, active, err := kb.ActiveImportRun(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("expected the run to no longer be active after FinishImportSynthesis(built)")
	}
}

func TestRecoverStuckSynthesis_MarksNeedsHuman(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := kb.BeginImportSynthesis(ctx, primaryID); err != nil || !claimed {
		t.Fatalf("BeginImportSynthesis: claimed=%v err=%v", claimed, err)
	}

	n, err := kb.RecoverStuckSynthesis(ctx, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("RecoverStuckSynthesis: %v", err)
	}
	if n != 1 {
		t.Fatalf("recovered %d stuck runs, want 1", n)
	}

	materials, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if materials[0].Params.Synthesis.Status != kbstore.SynthesisNeedsHuman {
		t.Fatalf("Status = %q, want needs_human", materials[0].Params.Synthesis.Status)
	}

	// A second recovery pass finds nothing left to recover (already terminal).
	n2, err := kb.RecoverStuckSynthesis(ctx, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second recovery pass recovered %d, want 0", n2)
	}
}

func TestRecoverStuckSynthesis_IgnoresRecentRuns(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := kb.BeginImportSynthesis(ctx, primaryID); err != nil || !claimed {
		t.Fatalf("BeginImportSynthesis: claimed=%v err=%v", claimed, err)
	}

	n, err := kb.RecoverStuckSynthesis(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("recovered %d runs that just started, want 0", n)
	}
}

// --- RequeueImportJob ----------------------------------------------------------

func TestRequeueImportJob_RetriesThenGoesNeedsHuman(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "firecrawl", TargetType: "auto", URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}

	const maxAttempts = 2
	terminal, err := kb.RequeueImportJob(ctx, id, "vendor timeout", maxAttempts)
	if err != nil {
		t.Fatalf("RequeueImportJob #1: %v", err)
	}
	if terminal {
		t.Fatal("expected the first failure to requeue, not terminate")
	}
	jobs, err := kb.ClaimImportJobs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Params.Attempts != 1 || jobs[0].Params.LastError != "vendor timeout" {
		t.Fatalf("jobs = %+v, want attempts=1 and the recorded cause", jobs)
	}

	terminal2, err := kb.RequeueImportJob(ctx, id, "vendor timeout again", maxAttempts)
	if err != nil {
		t.Fatalf("RequeueImportJob #2: %v", err)
	}
	if !terminal2 {
		t.Fatal("expected the second failure to exhaust maxAttempts=2 and terminate")
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	} // must find nothing
	remaining, err := kb.ClaimImportJobs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("a needs_human row must never be claimable again, got %+v", remaining)
	}
}

func TestFinishImportExtraction_ClearsLastErrorOnSuccess(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.RequeueImportJob(ctx, id, "transient network error", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}

	if err := kb.FinishImportExtraction(ctx, id, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "hello"}); err != nil {
		t.Fatalf("FinishImportExtraction: %v", err)
	}

	materials, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if materials[0].ProcessingStatus != "parsed" || materials[0].ExtractedText != "hello" {
		t.Fatalf("materials[0] = %+v", materials[0])
	}
	if materials[0].Params.LastError != "" {
		t.Errorf("LastError = %q, want cleared after a successful retry", materials[0].Params.LastError)
	}
	if materials[0].Params.Attempts != 1 {
		t.Errorf("Attempts = %d, want the earlier RequeueImportJob's bump preserved", materials[0].Params.Attempts)
	}
}

// --- MarkImportBuilt -----------------------------------------------------------

func TestMarkImportBuilt_OnlyAffectsParsedRows(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	parsedID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", URL: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := kb.FinishImportExtraction(ctx, parsedID, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "x"}); err != nil {
		t.Fatal(err)
	}

	failedID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{Provider: "native", TargetType: "auto", URL: "https://example.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := kb.FinishImportExtraction(ctx, failedID, kbstore.ExtractionOutcome{Status: "needs_human"}); err != nil {
		t.Fatal(err)
	}

	if err := kb.MarkImportBuilt(ctx, []uuid.UUID{parsedID, failedID}); err != nil {
		t.Fatalf("MarkImportBuilt: %v", err)
	}

	mats, err := kb.ListLiveMaterials(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[uuid.UUID]kbstore.Material{}
	for _, m := range mats {
		byID[m.ID] = m
	}
	if byID[parsedID].ProcessingStatus != "built" {
		t.Errorf("parsed material's status = %q, want built", byID[parsedID].ProcessingStatus)
	}
	if byID[failedID].ProcessingStatus != "needs_human" {
		t.Errorf("needs_human material's status = %q, want left untouched", byID[failedID].ProcessingStatus)
	}
}

// --- ActiveImportRun -------------------------------------------------------------

func TestActiveImportRun_GateLifecycle(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if _, active, err := kb.ActiveImportRun(ctx, orgID); err != nil || active {
		t.Fatalf("expected no active run initially: active=%v err=%v", active, err)
	}

	runID := uuid.New()
	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/x"})
	if err != nil {
		t.Fatal(err)
	}

	gotRun, active, err := kb.ActiveImportRun(ctx, orgID)
	if err != nil || !active || gotRun != runID {
		t.Fatalf("after enqueue: run=%s active=%v err=%v, want %s/true/nil", gotRun, active, err, runID)
	}

	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := kb.FinishImportExtraction(ctx, primaryID, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, active, err := kb.ActiveImportRun(ctx, orgID); err != nil || !active {
		t.Fatalf("mid-synthesis-not-yet-started: expected still active, active=%v err=%v", active, err)
	}

	if _, claimed, err := kb.BeginImportSynthesis(ctx, primaryID); err != nil || !claimed {
		t.Fatalf("BeginImportSynthesis: claimed=%v err=%v", claimed, err)
	}
	if _, active, err := kb.ActiveImportRun(ctx, orgID); err != nil || !active {
		t.Fatalf("mid-synthesis-running: expected still active, active=%v err=%v", active, err)
	}

	if err := kb.FinishImportSynthesis(ctx, primaryID, kbstore.SynthesisState{Status: kbstore.SynthesisBuilt}); err != nil {
		t.Fatal(err)
	}
	if _, active, err := kb.ActiveImportRun(ctx, orgID); err != nil || active {
		t.Fatalf("after finish: expected no longer active, active=%v err=%v", active, err)
	}

	// A new run may now start.
	if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: uuid.New(), Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/y"}); err != nil {
		t.Fatalf("expected a second run to be enqueueable once the first is terminal: %v", err)
	}
}

// --- CancelImportRun ---------------------------------------------------------

func TestCancelImportRun_QueuedMaterialsAndFreesTheOrgGate(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	secondaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", URL: "https://example.com/b"})
	if err != nil {
		t.Fatal(err)
	}

	found, err := kb.CancelImportRun(ctx, orgID, runID)
	if err != nil || !found {
		t.Fatalf("CancelImportRun: found=%v err=%v, want true/nil", found, err)
	}

	mats, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mats {
		if m.ProcessingStatus != kbstore.ImportStatusCancelled {
			t.Errorf("material %s status = %q, want cancelled", m.ID, m.ProcessingStatus)
		}
		if m.ID == primaryID && (m.Params.Synthesis == nil || m.Params.Synthesis.Status != kbstore.SynthesisCancelled) {
			t.Errorf("primary synthesis = %+v, want a cancelled SynthesisState", m.Params.Synthesis)
		}
	}
	_ = secondaryID

	// ActiveImportRun's one-run-per-org gate must release: a cancelled run
	// with pass 2 never claimed would otherwise leave Synthesis nil forever
	// (isTerminal() false), permanently blocking a fresh Submit.
	if _, active, err := kb.ActiveImportRun(ctx, orgID); err != nil || active {
		t.Fatalf("after cancel: active=%v err=%v, want false/nil", active, err)
	}
	if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: uuid.New(), Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/c"}); err != nil {
		t.Fatalf("expected a new run to be enqueueable after cancel: %v", err)
	}
}

func TestCancelImportRun_ExtractingMaterial_LateFinishIsDiscardedNotResurrected(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}

	if found, err := kb.CancelImportRun(ctx, orgID, runID); err != nil || !found {
		t.Fatalf("CancelImportRun: found=%v err=%v, want true/nil", found, err)
	}

	// The worker that claimed primaryID before the cancel is still running
	// pass 1 (in this test, synchronously) and now reports its outcome —
	// FinishImportExtraction's own CAS (WHERE processing_status =
	// 'extracting') must find the row already 'cancelled' and refuse the
	// late write rather than resurrecting it as 'parsed'.
	if err := kb.FinishImportExtraction(ctx, primaryID, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "late result"}); !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Fatalf("late FinishImportExtraction err = %v, want ErrUnknownKind", err)
	}

	mats, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mats) != 1 || mats[0].ProcessingStatus != kbstore.ImportStatusCancelled || mats[0].ExtractedText != "" {
		t.Fatalf("material after late finish = %+v, want cancelled with no extracted text", mats[0])
	}
}

func TestCancelImportRun_RefusedOnceSynthesisClaimed(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := kb.FinishImportExtraction(ctx, primaryID, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := kb.BeginImportSynthesis(ctx, primaryID); err != nil || !claimed {
		t.Fatalf("BeginImportSynthesis: claimed=%v err=%v", claimed, err)
	}

	if found, err := kb.CancelImportRun(ctx, orgID, runID); !errors.Is(err, kbstore.ErrImportRunNotCancelable) || found {
		t.Fatalf("CancelImportRun once synthesis is running: found=%v err=%v, want false/ErrImportRunNotCancelable", found, err)
	}

	// Refusing must leave the run entirely untouched — still able to finish
	// synthesis normally afterward.
	if err := kb.FinishImportSynthesis(ctx, primaryID, kbstore.SynthesisState{Status: kbstore.SynthesisBuilt}); err != nil {
		t.Fatalf("FinishImportSynthesis after a refused cancel: %v", err)
	}
}

func TestCancelImportRun_AlreadyTerminalIsANoOp(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := kb.FinishImportExtraction(ctx, primaryID, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := kb.BeginImportSynthesis(ctx, primaryID); err != nil {
		t.Fatal(err)
	}
	if err := kb.FinishImportSynthesis(ctx, primaryID, kbstore.SynthesisState{Status: kbstore.SynthesisBuilt}); err != nil {
		t.Fatal(err)
	}

	found, err := kb.CancelImportRun(ctx, orgID, runID)
	if err != nil || found {
		t.Fatalf("CancelImportRun on an already-built run: found=%v err=%v, want false/nil", found, err)
	}

	mats, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if mats[0].Params.Synthesis.Status != kbstore.SynthesisBuilt {
		t.Fatalf("a no-op cancel must never touch the run's own terminal status, got %+v", mats[0].Params.Synthesis)
	}
}

func TestCancelImportRun_UnknownRunID(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	if _, err := kb.CancelImportRun(context.Background(), orgID, uuid.New()); !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Fatalf("CancelImportRun for an unknown run: err = %v, want ErrUnknownKind", err)
	}
}

// --- ImportRunMaterials is org-scoped -------------------------------------------

func TestImportRunMaterials_OrgScoped(t *testing.T) {
	kb, orgA, st, _ := newTestKB(t)
	ctx := context.Background()
	orgB, err := st.SeedOrganization(ctx, "org-b")
	if err != nil {
		t.Fatal(err)
	}

	// Both orgs happen to pick the SAME run id — a cross-org collision must
	// still never leak the other org's material into this org's manifest.
	runID := uuid.New()
	idA, err := kb.EnqueueImport(ctx, orgA, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", URL: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	idB, err := kb.EnqueueImport(ctx, orgB.ID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", URL: "https://example.com/b"})
	if err != nil {
		t.Fatal(err)
	}

	matsA, err := kb.ImportRunMaterials(ctx, orgA, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(matsA) != 1 || matsA[0].ID != idA {
		t.Fatalf("org A materials = %+v, want only idA", matsA)
	}

	matsB, err := kb.ImportRunMaterials(ctx, orgB.ID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(matsB) != 1 || matsB[0].ID != idB {
		t.Fatalf("org B materials = %+v, want only idB", matsB)
	}
}

// TestImportRunMaterials_PopulatesUpdatedAt covers KB-14's FinishedAt: a
// freshly enqueued material's updated_at must round-trip non-zero (it has
// the same DEFAULT as created_at — see the migration), the same as
// CreatedAt already did before this field existed.
func TestImportRunMaterials_PopulatesUpdatedAt(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()
	if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/x"}); err != nil {
		t.Fatal(err)
	}

	mats, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mats) != 1 || mats[0].UpdatedAt.IsZero() {
		t.Fatalf("materials = %+v, want exactly one row with a non-zero UpdatedAt", mats)
	}
}

// --- RecentImportRuns is paginated (KB-14) ----------------------------------

func TestRecentImportRuns_PaginatesAndReportsTotal(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	want := make(map[uuid.UUID]bool, 3)
	for i := 0; i < 3; i++ {
		runID := uuid.New()
		want[runID] = true
		if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
			RunID: runID, Provider: "native", TargetType: "auto", Primary: true,
			URL: fmt.Sprintf("https://example.com/%d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	// created_at is millisecond-precision and these three inserts can land
	// in the same millisecond, so DESC order across them is not something a
	// test may assume — only totality (every run appears exactly once
	// across the pages) and the page sizes themselves are guaranteed.
	page1, total1, err := kb.RecentImportRuns(ctx, orgID, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total1 != 3 || len(page1) != 2 {
		t.Fatalf("page1 = %v (total %d), want 2 ids (total 3)", page1, total1)
	}

	page2, total2, err := kb.RecentImportRuns(ctx, orgID, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total2 != 3 || len(page2) != 1 {
		t.Fatalf("page2 = %v (total %d), want 1 id (total 3)", page2, total2)
	}

	seen := make(map[uuid.UUID]bool, 3)
	for _, id := range append(append([]uuid.UUID{}, page1...), page2...) {
		if seen[id] {
			t.Fatalf("run %s appeared on more than one page", id)
		}
		seen[id] = true
		if !want[id] {
			t.Fatalf("run %s was never enqueued", id)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("pages together covered %d runs, want all 3", len(seen))
	}

	// Past the end: empty page, total still reported.
	page3, total3, err := kb.RecentImportRuns(ctx, orgID, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total3 != 3 || len(page3) != 0 {
		t.Fatalf("page3 = %v (total %d), want 0 ids (total 3)", page3, total3)
	}
}
