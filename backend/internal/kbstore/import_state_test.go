package kbstore_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// The import queue stores its state as plain JSON text plus two mirrored columns, with no
// SQL JSON function anywhere, so the same statements run on SQLite and PostgreSQL. These
// tests pin what that storage promises.

func importColumns(t *testing.T, db *dbx.DB, id uuid.UUID) (runID, synthesis *string) {
	t.Helper()
	if err := db.QueryRow(context.Background(),
		`SELECT import_run_id, import_synthesis_status FROM kbd_materials WHERE id = $1`, id).Scan(&runID, &synthesis); err != nil {
		t.Fatalf("read import columns: %v", err)
	}
	return runID, synthesis
}

func str(p *string) string {
	if p == nil {
		return "<NULL>"
	}
	return *p
}

// import_run_id / import_synthesis_status follow the metadata through every transition the
// queue filters on: enqueued (run set, pass 2 not started), claimed, finished, cancelled.
func TestImportQueueColumnsMirrorTheMetadata(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()
	runID := uuid.New()

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runID, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/p"})
	if err != nil {
		t.Fatal(err)
	}
	if run, syn := importColumns(t, db, id); str(run) != runID.String() || syn != nil {
		t.Fatalf("after enqueue: run=%s synthesis=%s, want %s / <NULL>", str(run), str(syn), runID)
	}
	pending, err := kb.PendingSynthesisPrimaries(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != id {
		t.Fatalf("PendingSynthesisPrimaries = %v (%v), want just the new primary", pending, err)
	}

	token, claimed, err := kb.BeginImportSynthesis(ctx, id)
	if err != nil || !claimed {
		t.Fatalf("BeginImportSynthesis claimed=%v err=%v", claimed, err)
	}
	if _, syn := importColumns(t, db, id); str(syn) != kbstore.SynthesisRunning {
		t.Fatalf("after claim: synthesis=%s, want %s", str(syn), kbstore.SynthesisRunning)
	}
	if pending, _ := kb.PendingSynthesisPrimaries(ctx); len(pending) != 0 {
		t.Fatalf("a claimed primary is still listed as pending: %v", pending)
	}

	if err := kb.FinishImportSynthesis(ctx, id, kbstore.SynthesisState{Status: kbstore.SynthesisBuilt, Token: token}); err != nil {
		t.Fatal(err)
	}
	if _, syn := importColumns(t, db, id); str(syn) != kbstore.SynthesisBuilt {
		t.Fatalf("after finish: synthesis=%s, want %s", str(syn), kbstore.SynthesisBuilt)
	}

	// The run's materials are found by the column, scoped to the organization.
	mats, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil || len(mats) != 1 || mats[0].ID != id {
		t.Fatalf("ImportRunMaterials = %v (%v), want the one material", mats, err)
	}
	if other, err := kb.ImportRunMaterials(ctx, uuid.New(), runID); err != nil || len(other) != 0 {
		t.Fatalf("another organization sees the run's materials: %v (%v)", other, err)
	}

	// A cancelled, never-claimed run mirrors 'cancelled'.
	cancelRun := uuid.New()
	cid, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: cancelRun, Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/c"})
	if err != nil {
		t.Fatal(err)
	}
	if found, err := kb.CancelImportRun(ctx, orgID, cancelRun); err != nil || !found {
		t.Fatalf("CancelImportRun found=%v err=%v", found, err)
	}
	if _, syn := importColumns(t, db, cid); str(syn) != kbstore.SynthesisCancelled {
		t.Fatalf("after cancel: synthesis=%s, want %s", str(syn), kbstore.SynthesisCancelled)
	}
}

// A material that is not part of any import carries neither column.
func TestNonImportMaterialsHaveNoImportColumns(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()
	matID, err := kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{Filename: "a.pdf", MimeType: "application/pdf", SizeBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if run, syn := importColumns(t, db, matID); run != nil || syn != nil {
		t.Fatalf("plain upload has import_run_id=%s import_synthesis_status=%s, want both NULL", str(run), str(syn))
	}
}

// Every import write is a read-modify-write of the whole document in Go. It must leave the
// keys it does not own — here the provenance tag recordProvenance adds — exactly as they were.
func TestImportWritesPreserveForeignMetadataKeys(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	id, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: uuid.New(), Provider: "native", TargetType: "auto", Primary: true, URL: "https://example.com/p"})
	if err != nil {
		t.Fatal(err)
	}
	const tag = `{"type":"product","key":"p-1"}`
	var doc map[string]json.RawMessage
	var raw string
	if err := db.QueryRow(ctx, `SELECT extraction_metadata FROM kbd_materials WHERE id = $1`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	doc["mcp_target"] = json.RawMessage(tag)
	withTag, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE kbd_materials SET extraction_metadata = $2 WHERE id = $1`, id, string(withTag)); err != nil {
		t.Fatal(err)
	}

	assertTagKept := func(step string) {
		t.Helper()
		var now string
		if err := db.QueryRow(ctx, `SELECT extraction_metadata FROM kbd_materials WHERE id = $1`, id).Scan(&now); err != nil {
			t.Fatal(err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal([]byte(now), &got); err != nil {
			t.Fatalf("%s: stored metadata is not JSON: %v", step, err)
		}
		if string(got["mcp_target"]) != tag {
			t.Fatalf("%s: mcp_target = %s, want %s kept untouched", step, got["mcp_target"], tag)
		}
		if _, ok := got["import"]; !ok {
			t.Fatalf("%s: the import object is missing", step)
		}
	}

	if _, err := kb.ClaimImportJobs(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := kb.FinishImportExtraction(ctx, id, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "text"}); err != nil {
		t.Fatal(err)
	}
	assertTagKept("FinishImportExtraction")

	if _, err := kb.RequeueImportJob(ctx, id, "boom", 5); err != nil {
		t.Fatal(err)
	}
	assertTagKept("RequeueImportJob")

	if err := kb.TagImportMaterial(ctx, id, kbstore.ImportParams{RunID: uuid.New(), Provider: "native", Primary: true, Handle: "evidence.9"}); err != nil {
		t.Fatal(err)
	}
	assertTagKept("TagImportMaterial")

	token, claimed, err := kb.BeginImportSynthesis(ctx, id)
	if err != nil || !claimed {
		t.Fatalf("BeginImportSynthesis claimed=%v err=%v", claimed, err)
	}
	assertTagKept("BeginImportSynthesis")

	if err := kb.FinishImportSynthesis(ctx, id, kbstore.SynthesisState{Status: kbstore.SynthesisBuilt, Token: token}); err != nil {
		t.Fatal(err)
	}
	assertTagKept("FinishImportSynthesis")
}

// Of any number of concurrent enqueues of one staged file, exactly one wins the
// 'parsed' -> 'queued' swap, and the stored params are that winner's, whole.
func TestEnqueueImport_File_ConcurrentEnqueueExactlyOneWins(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	matID, err := kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{Filename: "list.pdf", MimeType: "application/pdf", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := kb.CompleteMaterialUpload(ctx, matID, "disk", "org/x/"+matID.String(), 10, ""); err != nil {
		t.Fatal(err)
	}

	const attempts = 8
	runs := make([]uuid.UUID, attempts)
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := range runs {
		runs[i] = uuid.New()
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{RunID: runs[i], Provider: "llamaparse", TargetType: "products", MaterialID: matID})
		}(i)
	}
	wg.Wait()

	winner := -1
	for i, err := range errs {
		switch err {
		case nil:
			if winner >= 0 {
				t.Fatalf("attempts %d and %d both enqueued the same material", winner, i)
			}
			winner = i
		case kbstore.ErrUnknownKind:
		default:
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if winner < 0 {
		t.Fatal("no enqueue won")
	}
	if run, _ := importColumns(t, db, matID); str(run) != runs[winner].String() {
		t.Fatalf("stored import_run_id = %s, want the winner's %s", str(run), runs[winner])
	}
}
