package kbimport_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/kbimport"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// scriptedClient mirrors synth_test.go's internal one but lives here since
// this file is package kbimport_test (external, exercising the public
// Start/Stop/Submit/RunStatus surface end to end) and cannot reach the
// internal one.
type scriptedClient struct{ response string }

func (c *scriptedClient) Complete(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Text: c.response, PromptTokens: 10, CompletionTokens: 5}, nil
}

type staticRegistry struct{ client llm.ChatClient }

func (r staticRegistry) Client(llm.ModelRef) (llm.ChatClient, error) { return r.client, nil }

// TestEndToEnd_StartClaimsExtractsAndSynthesizes drives the WHOLE pipeline
// through its real Start/Stop worker pool and tickers — Submit against a
// live httptest URL, then wait for the background workers to claim,
// extract, and synthesize it into the draft, with no direct calls into any
// unexported method. This is the one test proving the pieces this package
// tests in isolation elsewhere actually cooperate correctly when wired
// together via Start.
func TestEndToEnd_StartClaimsExtractsAndSynthesizes(t *testing.T) {
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>Магнитный сверлильный станок ZT-40H</h1><p>Цена: 180 000 ₸</p></body></html>`))
	}))
	defer pageSrv.Close()

	modelResp, err := json.Marshal(map[string]any{
		"calls": []map[string]any{{
			"tool": "kb_product_upsert",
			"args": map[string]any{"ref": "drill-zt40h", "changes": map[string]any{"name": "Станок ZT-40H", "price": "180 000 ₸", "in_stock": true}},
		}},
		"notes": "готово", "unmapped": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}

	kb, st, _ := dbtest.NewKB(t)
	org, err := st.SeedOrganization(context.Background(), "xchats")
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.SeedUser(context.Background(), org.ID, "op@example.com", "hash", "Op")
	if err != nil {
		t.Fatal(err)
	}
	diskStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	settingsStore := settings.NewStore(t.TempDir())
	if _, err := settingsStore.Update(func(s *settings.Settings) {
		s.LLM.DefaultProvider, s.LLM.DefaultModel = "fake", "fake-model"
	}); err != nil {
		t.Fatal(err)
	}

	cfg := kbimport.DefaultConfig()
	cfg.ClaimEvery = 20 * time.Millisecond
	cfg.RecoverEvery = time.Hour // not under test here
	svc := kbimport.New(kbimport.Deps{
		KB: kb, Blob: diskStore, Settings: settingsStore,
		LLM: staticRegistry{client: &scriptedClient{response: string(modelResp)}},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), AllowPrivateFetch: true,
	}, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)
	defer func() { cancel(); svc.Stop() }()

	run, err := svc.Submit(context.Background(), org.ID, user.ID, kbimport.SubmitInput{
		Provider: "native", TargetType: "auto", URLs: []string{pageSrv.URL},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var final kbimport.RunSummary
	for time.Now().Before(deadline) {
		final, err = svc.RunStatus(context.Background(), org.ID, run.RunID)
		if err != nil {
			t.Fatalf("RunStatus: %v", err)
		}
		if final.Status == "built" || final.Status == "failed" || final.Status == "needs_human" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if final.Status != "built" {
		t.Fatalf("final run status = %q (materials=%+v, synthesis=%+v), want built", final.Status, final.Materials, final.Synthesis)
	}
	if len(final.Synthesis.Applied) != 1 || final.Synthesis.Applied[0].Key != "drill-zt40h" {
		t.Fatalf("Synthesis.Applied = %+v, want the drill product", final.Synthesis.Applied)
	}

	v, err := kb.DraftOnly(context.Background(), org.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Products) != 1 || v.Products[0].Ref != "drill-zt40h" || v.Products[0].Price != "180 000 ₸" {
		t.Fatalf("draft products = %+v", v.Products)
	}
}

// TestCancel_DuringExtraction_StopsTheRunAndSurvivesTheLateResult drives
// Cancel against a run whose one material is genuinely mid pass-1 — the
// page fetch blocks until this test releases it — proving the cancel takes
// effect immediately (RunStatus flips to "cancelled" right away, without
// waiting for the blocked extraction to return) and that the extraction's
// eventual late result cannot resurrect it (kbstore.
// TestCancelImportRun_ExtractingMaterial_LateFinishIsDiscardedNotResurrected
// covers the same CAS mechanism directly; this is the one test proving
// Cancel's own HTTP-adjacent Service method actually benefits from it when
// wired into the real worker pool).
func TestCancel_DuringExtraction_StopsTheRunAndSurvivesTheLateResult(t *testing.T) {
	claimed := make(chan struct{})
	release := make(chan struct{})
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(claimed)
		<-release
		_, _ = w.Write([]byte(`<html><body><h1>Late page</h1></body></html>`))
	}))
	defer pageSrv.Close()

	kb, st, _ := dbtest.NewKB(t)
	org, err := st.SeedOrganization(context.Background(), "xchats")
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.SeedUser(context.Background(), org.ID, "op@example.com", "hash", "Op")
	if err != nil {
		t.Fatal(err)
	}
	diskStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cfg := kbimport.DefaultConfig()
	cfg.ClaimEvery = 20 * time.Millisecond
	cfg.RecoverEvery = time.Hour
	svc := kbimport.New(kbimport.Deps{
		KB: kb, Blob: diskStore,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), AllowPrivateFetch: true,
	}, cfg)

	ctx, cancelCtx := context.WithCancel(context.Background())
	svc.Start(ctx)
	defer func() { cancelCtx(); svc.Stop() }()

	run, err := svc.Submit(context.Background(), org.ID, user.ID, kbimport.SubmitInput{
		Provider: "native", TargetType: "auto", URLs: []string{pageSrv.URL},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case <-claimed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the worker to claim and start extracting")
	}

	status, err := svc.RunStatus(context.Background(), org.ID, run.RunID)
	if err != nil {
		t.Fatalf("RunStatus before cancel: %v", err)
	}
	if status.StartedBy != user.ID.String() {
		t.Errorf("StartedBy = %q, want %q", status.StartedBy, user.ID.String())
	}
	// The DB's own clock and its sub-second truncation (dbx.FormatTime)
	// mean StartedAt can legitimately land a few milliseconds either side
	// of a Go-side time.Now() taken around the same Submit call — assert
	// it round-tripped as a real, recent timestamp, not an exact window.
	if status.StartedAt.IsZero() || time.Since(status.StartedAt) > time.Minute {
		t.Errorf("StartedAt = %v, want a non-zero timestamp from the last minute", status.StartedAt)
	}
	if !status.Cancelable {
		t.Errorf("Cancelable = false while still extracting, want true")
	}

	if err := svc.Cancel(context.Background(), org.ID, run.RunID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	final, err := svc.RunStatus(context.Background(), org.ID, run.RunID)
	if err != nil {
		t.Fatalf("RunStatus after cancel: %v", err)
	}
	if final.Status != "cancelled" || final.Cancelable {
		t.Fatalf("RunStatus after cancel = %+v, want status=cancelled, cancelable=false", final)
	}

	// Let the blocked extraction finish and confirm it never resurrects the
	// cancelled run.
	close(release)
	time.Sleep(100 * time.Millisecond)
	stillFinal, err := svc.RunStatus(context.Background(), org.ID, run.RunID)
	if err != nil {
		t.Fatalf("RunStatus after the late extraction result: %v", err)
	}
	if stillFinal.Status != "cancelled" {
		t.Fatalf("status after the late extraction result = %q, want it to stay cancelled", stillFinal.Status)
	}
}

func TestCancel_UnknownRun_ReturnsErrNotFound(t *testing.T) {
	kb, st, _ := dbtest.NewKB(t)
	org, err := st.SeedOrganization(context.Background(), "xchats")
	if err != nil {
		t.Fatal(err)
	}
	svc := kbimport.New(kbimport.Deps{KB: kb, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, kbimport.DefaultConfig())

	if err := svc.Cancel(context.Background(), org.ID, uuid.New()); !errors.Is(err, kbimport.ErrNotFound) {
		t.Fatalf("Cancel for an unknown run: err = %v, want ErrNotFound", err)
	}
}
