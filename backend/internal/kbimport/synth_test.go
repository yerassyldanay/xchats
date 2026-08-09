package kbimport

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// scriptedClient is a fake llm.ChatClient returning successive canned
// responses — one per call, in order — recording every request it saw.
type scriptedClient struct {
	responses []string
	calls     []llm.ChatRequest
}

func (c *scriptedClient) Complete(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	c.calls = append(c.calls, req)
	if len(c.calls) > len(c.responses) {
		return llm.ChatResponse{}, fmt.Errorf("scriptedClient: no more scripted responses (call %d)", len(c.calls))
	}
	return llm.ChatResponse{Text: c.responses[len(c.calls)-1], PromptTokens: 111, CompletionTokens: 22}, nil
}

// fakeRegistry always resolves to the same client regardless of ModelRef.
type fakeRegistry struct{ client llm.ChatClient }

func (r fakeRegistry) Client(llm.ModelRef) (llm.ChatClient, error) { return r.client, nil }

// newTestService builds a Service wired to a fresh dbtest database, a temp
// disk blob store, a temp settings store with a default model configured,
// and client as the synthesis model's fake ChatClient.
func newTestService(t *testing.T, client llm.ChatClient) (*Service, *kbstore.Store, uuid.UUID, uuid.UUID) {
	t.Helper()
	kb, st, _ := dbtest.NewKB(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, "op@example.com", "hash", "Op")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	diskStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob.NewDisk: %v", err)
	}

	settingsStore := settings.NewStore(t.TempDir())
	if _, err := settingsStore.Update(func(s *settings.Settings) {
		s.LLM.DefaultProvider = "fake"
		s.LLM.DefaultModel = "fake-model"
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	cfg := DefaultConfig()
	cfg.SynthTimeout = 10 * time.Second
	cfg.MaxCallsPerRun = 25

	svc := New(Deps{
		KB: kb, Blob: diskStore, Settings: settingsStore, LLM: fakeRegistry{client: client},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), AllowPrivateFetch: true,
	}, cfg)
	return svc, kb, org.ID, user.ID
}

// seedMaterial is one material to land as 'parsed' evidence before
// synthesis runs.
type seedMaterial struct {
	handle string
	text   string
	// file, when true, stages an actual uploaded material (so its handle is
	// a real "upload.N" reference resolveHandlesInArgs can resolve) instead
	// of a plain URL/evidence-only material.
	file     bool
	mimeType string
}

// seedRun enqueues every material under one fresh run (materials[0] is
// primary), fast-forwards each straight to 'parsed' (bypassing pass 1
// entirely — this file tests pass 2, not extraction), and returns the run
// id plus the primary material's id.
func seedRun(t *testing.T, kb *kbstore.Store, orgID, userID uuid.UUID, targetType string, materials []seedMaterial) (runID, primaryID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	runID = uuid.New()

	for i, m := range materials {
		var matID uuid.UUID
		if m.file {
			var err error
			matID, err = kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
				Filename: fmt.Sprintf("file-%d.bin", i), MimeType: orDefaultMime(m.mimeType), SizeBytes: 10, CustomerVisibility: "visible",
			})
			if err != nil {
				t.Fatalf("CreateUploadMaterial: %v", err)
			}
			if err := kb.CompleteMaterialUpload(ctx, matID, "disk", "k-"+matID.String(), 10, ""); err != nil {
				t.Fatalf("CompleteMaterialUpload: %v", err)
			}
			gotID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
				RunID: runID, UserID: userID, Provider: "native", TargetType: targetType,
				Primary: i == 0, Handle: m.handle, MaterialID: matID,
			})
			if err != nil {
				t.Fatalf("EnqueueImport(file): %v", err)
			}
			matID = gotID
		} else {
			var err error
			matID, err = kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
				RunID: runID, UserID: userID, Provider: "native", TargetType: targetType,
				Primary: i == 0, Handle: m.handle, URL: fmt.Sprintf("https://example.com/%d", i),
			})
			if err != nil {
				t.Fatalf("EnqueueImport(url): %v", err)
			}
		}
		if i == 0 {
			primaryID = matID
		}
	}

	jobs, err := kb.ClaimImportJobs(ctx, 100)
	if err != nil {
		t.Fatalf("ClaimImportJobs: %v", err)
	}
	if len(jobs) != len(materials) {
		t.Fatalf("claimed %d jobs, want %d", len(jobs), len(materials))
	}
	byHandle := map[string]string{}
	for _, m := range materials {
		byHandle[m.handle] = m.text
	}
	// Recover each job's handle from its own params (round-tripped through
	// ClaimImportJobs) rather than assuming claim order matches submission
	// order.
	for _, j := range jobs {
		text := byHandle[j.Params.Handle]
		if err := kb.FinishImportExtraction(ctx, j.ID, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: text}); err != nil {
			t.Fatalf("FinishImportExtraction: %v", err)
		}
	}
	return runID, primaryID
}

func orDefaultMime(m string) string {
	if m == "" {
		return "image/jpeg"
	}
	return m
}

func draftProducts(t *testing.T, kb *kbstore.Store, orgID uuid.UUID) []kbstore.ProductRow {
	t.Helper()
	v, err := kb.DraftOnly(context.Background(), orgID)
	if err != nil {
		t.Fatalf("DraftOnly: %v", err)
	}
	return v.Products
}

func synthesisState(t *testing.T, kb *kbstore.Store, orgID, runID uuid.UUID) *kbstore.SynthesisState {
	t.Helper()
	materials, err := kb.ImportRunMaterials(context.Background(), orgID, runID)
	if err != nil {
		t.Fatalf("ImportRunMaterials: %v", err)
	}
	for _, m := range materials {
		if m.Params.Primary {
			return m.Params.Synthesis
		}
	}
	t.Fatal("no primary material found")
	return nil
}

// --- happy path ----------------------------------------------------------------

func TestRunSynthesis_ValidMultiCallLands(t *testing.T) {
	resp := `{"calls":[
		{"tool":"kb_product_upsert","args":{"ref":"drill-zt40h","changes":{"name":"Станок ZT-40H","price":"180 000 ₸","in_stock":true}}},
		{"tool":"kb_topic_upsert","args":{"slug":"warranty","changes":{"title":"Гарантия","body_md":"Гарантия 12 месяцев."}}}
	],"notes":"Добавлены товар и тема.","unmapped":[]}`
	client := &scriptedClient{responses: []string{resp}}
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{
		{handle: "evidence.1", text: "ZT-40H, цена 180 000 тенге, гарантия 12 месяцев"},
	})

	svc.maybeSynthesize(context.Background(), orgID, runID)

	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisBuilt {
		t.Fatalf("Synthesis = %+v, want built", st)
	}
	if len(st.Applied) != 2 || len(st.Dropped) != 0 {
		t.Fatalf("Applied=%d Dropped=%d, want 2/0 (%+v / %+v)", len(st.Applied), len(st.Dropped), st.Applied, st.Dropped)
	}

	products := draftProducts(t, kb, orgID)
	if len(products) != 1 || products[0].Ref != "drill-zt40h" || products[0].Price != "180 000 ₸" {
		t.Fatalf("draft products = %+v", products)
	}
	if len(client.calls) != 1 {
		t.Fatalf("model called %d times, want exactly 1 for a clean success", len(client.calls))
	}
}

func TestRunSynthesis_BadEnumDropsOneKeepsOther(t *testing.T) {
	resp := `{"calls":[
		{"tool":"kb_product_upsert","args":{"ref":"good-1","changes":{"name":"Хороший товар","in_stock":true,"sales_status":"not-a-real-status"}}},
		{"tool":"kb_product_upsert","args":{"ref":"good-2","changes":{"name":"Другой товар","in_stock":true}}}
	],"notes":"","unmapped":[]}`
	client := &scriptedClient{responses: []string{resp}}
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{
		{handle: "evidence.1", text: "два товара"},
	})
	svc.maybeSynthesize(context.Background(), orgID, runID)

	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisBuilt {
		t.Fatalf("Synthesis = %+v, want built (one survivor is enough)", st)
	}
	if len(st.Applied) != 1 || len(st.Dropped) != 1 {
		t.Fatalf("Applied=%d Dropped=%d, want 1/1 (%+v / %+v)", len(st.Applied), len(st.Dropped), st.Applied, st.Dropped)
	}
	products := draftProducts(t, kb, orgID)
	if len(products) != 1 || products[0].Ref != "good-2" {
		t.Fatalf("draft products = %+v, want only good-2", products)
	}
}

// --- media handle resolution -----------------------------------------------------

func TestRunSynthesis_UploadHandleResolvesToFeaturedImage(t *testing.T) {
	resp := `{"calls":[
		{"tool":"kb_product_upsert","args":{"ref":"drill-zt40h","changes":{"name":"Станок","in_stock":true,"featured_image":"upload.1"}}}
	],"notes":"","unmapped":[]}`
	client := &scriptedClient{responses: []string{resp}}
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{
		{handle: "upload.1", text: "", file: true, mimeType: "image/jpeg"},
		{handle: "evidence.1", text: "ZT-40H"},
	})
	svc.maybeSynthesize(context.Background(), orgID, runID)

	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisBuilt {
		t.Fatalf("Synthesis = %+v, want built", st)
	}
	products := draftProducts(t, kb, orgID)
	if len(products) != 1 || products[0].FeaturedImage == nil {
		t.Fatalf("draft products = %+v, want featured_image set", products)
	}
}

func TestRunSynthesis_EvidenceHandleInMediaFieldDropsCall(t *testing.T) {
	resp := `{"calls":[
		{"tool":"kb_product_upsert","args":{"ref":"drill-zt40h","changes":{"name":"Станок","in_stock":true,"featured_image":"evidence.1"}}},
		{"tool":"kb_topic_upsert","args":{"slug":"ok-topic","changes":{"title":"ОК","body_md":"Текст."}}}
	],"notes":"","unmapped":[]}`
	client := &scriptedClient{responses: []string{resp}}
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{
		{handle: "evidence.1", text: "источник без прав на медиа"},
	})
	svc.maybeSynthesize(context.Background(), orgID, runID)

	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisBuilt {
		t.Fatalf("Synthesis = %+v, want built (the topic call still survives)", st)
	}
	if len(st.Dropped) != 1 {
		t.Fatalf("Dropped = %+v, want exactly 1 (the evidence-handle product call)", st.Dropped)
	}
	products := draftProducts(t, kb, orgID)
	if len(products) != 0 {
		t.Fatalf("draft products = %+v, want none — the only product call used an evidence handle", products)
	}
}

func TestRunSynthesis_BareUUIDInMediaFieldDropsCall(t *testing.T) {
	fakeUUID := uuid.New().String()
	resp := fmt.Sprintf(`{"calls":[
		{"tool":"kb_product_upsert","args":{"ref":"drill-zt40h","changes":{"name":"Станок","in_stock":true,"featured_image":%q}}}
	],"notes":"","unmapped":[]}`, fakeUUID)
	client := &scriptedClient{responses: []string{resp, resp}} // the zero-survivor path re-asks once
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{
		{handle: "evidence.1", text: "текст"},
	})
	svc.maybeSynthesize(context.Background(), orgID, runID)

	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisFailed {
		t.Fatalf("Synthesis = %+v, want failed (a bare UUID must never resolve as if it were a handle)", st)
	}
	products := draftProducts(t, kb, orgID)
	if len(products) != 0 {
		t.Fatalf("draft products = %+v, want none written", products)
	}
}

// --- parse failure retry ----------------------------------------------------------

func TestRunSynthesis_UnparseableOutput_OneRetryThenFailed_NothingWritten(t *testing.T) {
	client := &scriptedClient{responses: []string{"это не json вообще", "и это тоже не json"}}
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{
		{handle: "evidence.1", text: "текст"},
	})
	svc.maybeSynthesize(context.Background(), orgID, runID)

	if len(client.calls) != 2 {
		t.Fatalf("model called %d times, want exactly 2 (one original + one corrective re-ask)", len(client.calls))
	}
	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisFailed {
		t.Fatalf("Synthesis = %+v, want failed", st)
	}
	products := draftProducts(t, kb, orgID)
	v, err := kb.DraftOnly(context.Background(), orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 0 || len(v.Topics) != 0 || len(v.Tariffs) != 0 {
		t.Fatalf("draft = %+v, want completely empty — unparseable output must write nothing", v)
	}
}

// --- explicit target ---------------------------------------------------------------

func TestRunSynthesis_ExplicitTargetDropsForeignToolCall(t *testing.T) {
	resp := `{"calls":[
		{"tool":"kb_topic_upsert","args":{"slug":"wrong-type","changes":{"title":"Не тот тип","body_md":"Текст."}}},
		{"tool":"kb_product_upsert","args":{"ref":"right-type","changes":{"name":"Товар","in_stock":true}}}
	],"notes":"","unmapped":[]}`
	client := &scriptedClient{responses: []string{resp}}
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "products", []seedMaterial{
		{handle: "evidence.1", text: "текст"},
	})
	svc.maybeSynthesize(context.Background(), orgID, runID)

	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisBuilt {
		t.Fatalf("Synthesis = %+v, want built", st)
	}
	if len(st.Applied) != 1 || st.Applied[0].Tool != "kb_product_upsert" {
		t.Fatalf("Applied = %+v, want only the product call", st.Applied)
	}
	if len(st.Dropped) != 1 || st.Dropped[0].Tool != "kb_topic_upsert" {
		t.Fatalf("Dropped = %+v, want the topic call dropped for target mismatch", st.Dropped)
	}
	v, err := kb.DraftOnly(context.Background(), orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Topics) != 0 {
		t.Fatalf("draft topics = %+v, want none — target_type=products must exclude kb_topic_upsert", v.Topics)
	}
}

func TestRunSynthesis_AssistantCallAlwaysRejected(t *testing.T) {
	resp := `{"calls":[{"tool":"kb_assistant_upsert","args":{"changes":{"persona":"Новая личность"}}}],"notes":"","unmapped":[]}`
	client := &scriptedClient{responses: []string{resp, resp}}
	svc, kb, orgID, _ := newTestService(t, client)

	runID, _ := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{
		{handle: "evidence.1", text: "текст"},
	})
	svc.maybeSynthesize(context.Background(), orgID, runID)

	st := synthesisState(t, kb, orgID, runID)
	if st == nil || st.Status != kbstore.SynthesisFailed {
		t.Fatalf("Synthesis = %+v, want failed — a persona must never be inferred from imported content", st)
	}
}

// --- BeginImportSynthesis CAS integration ------------------------------------------

func TestMaybeSynthesize_NotReadyUntilAllTerminal(t *testing.T) {
	client := &scriptedClient{responses: []string{`{"calls":[],"notes":"","unmapped":[]}`}}
	svc, kb, orgID, userID := newTestService(t, client)
	ctx := context.Background()

	runID := uuid.New()
	primaryID, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
		RunID: runID, UserID: userID, Provider: "native", TargetType: "auto", Primary: true, Handle: "evidence.1", URL: "https://example.com/a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.EnqueueImport(ctx, orgID, kbstore.ImportInput{
		RunID: runID, UserID: userID, Provider: "native", TargetType: "auto", Handle: "evidence.2", URL: "https://example.com/b",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.ClaimImportJobs(ctx, 100); err != nil {
		t.Fatal(err)
	}
	// Finish only the primary — the sibling is still 'extracting'.
	if err := kb.FinishImportExtraction(ctx, primaryID, kbstore.ExtractionOutcome{Status: "parsed", ExtractedText: "x"}); err != nil {
		t.Fatal(err)
	}

	svc.maybeSynthesize(ctx, orgID, runID)

	if len(client.calls) != 0 {
		t.Fatalf("model was called %d times before every material was terminal, want 0", len(client.calls))
	}
	materials, err := kb.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range materials {
		if m.Params.Primary && m.Params.Synthesis != nil {
			t.Fatalf("synthesis was claimed before the run was ready: %+v", m.Params.Synthesis)
		}
	}
}

func TestMaybeSynthesize_AlreadyTerminalIsANoOp(t *testing.T) {
	client := &scriptedClient{}
	svc, kb, orgID, _ := newTestService(t, client)
	runID, primaryID := seedRun(t, kb, orgID, uuid.Nil, "auto", []seedMaterial{{handle: "evidence.1", text: "x"}})
	if err := kb.FinishImportSynthesis(context.Background(), primaryID, kbstore.SynthesisState{Status: kbstore.SynthesisBuilt}); err != nil {
		t.Fatal(err)
	}

	svc.maybeSynthesize(context.Background(), orgID, runID)

	if len(client.calls) != 0 {
		t.Fatalf("model was called for an already-terminal run, want 0 calls")
	}
}

// --- parseModelOutput --------------------------------------------------------------

func TestParseModelOutput(t *testing.T) {
	if _, ok := parseModelOutput(`not json`); ok {
		t.Error("expected non-JSON to fail parsing")
	}
	if _, ok := parseModelOutput(`{"reply_text":"hi"}`); ok {
		t.Error("expected an object with no 'calls' key to fail parsing")
	}
	out, ok := parseModelOutput("```json\n" + `{"calls":[],"notes":"ok","unmapped":[]}` + "\n```")
	if !ok || out.Notes != "ok" {
		t.Errorf("parseModelOutput(fenced) = %+v, %v", out, ok)
	}
}

func TestFindPrimary(t *testing.T) {
	materials := []kbstore.ImportMaterial{{ID: uuid.New()}, {ID: uuid.New(), Params: kbstore.ImportParams{Primary: true}}}
	p := findPrimary(materials)
	if p == nil || !p.Params.Primary {
		t.Fatalf("findPrimary = %+v, want the second material", p)
	}
	if findPrimary(materials[:1]) != nil {
		t.Error("findPrimary with no primary present should return nil")
	}
}
