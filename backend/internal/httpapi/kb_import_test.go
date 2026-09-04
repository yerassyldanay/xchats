package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// kbImportRequest builds a POST /xchats/api/v1/kb/imports multipart request
// — url/file may each repeat.
func (h *harness) kbImportRequest(t *testing.T, provider, targetType, guidance string, urls []string, files map[string][]byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	writeField := func(name, value string) {
		if err := mw.WriteField(name, value); err != nil {
			t.Fatalf("write field %s: %v", name, err)
		}
	}
	writeField("provider", provider)
	writeField("target_type", targetType)
	if guidance != "" {
		writeField("guidance", guidance)
	}
	for _, u := range urls {
		writeField("url", u)
	}
	for filename, data := range files {
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file %s: %v", filename, err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatalf("write form file %s: %v", filename, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/xchats/api/v1/kb/imports", &buf)
	if err != nil {
		t.Fatalf("build kb import request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func (h *harness) postImport(t *testing.T, provider, targetType, guidance string, urls []string, files map[string][]byte) (*http.Response, map[string]json.RawMessage) {
	t.Helper()
	resp, err := h.client.Do(h.kbImportRequest(t, provider, targetType, guidance, urls, files))
	if err != nil {
		t.Fatalf("POST /kb/imports: %v", err)
	}
	defer resp.Body.Close()
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp, env
}

func TestKBImports_NoInputIs400(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postImport(t, "native", "auto", "", nil, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (body=%s)", resp.StatusCode, env["message"])
	}
}

func TestKBImports_UnknownProviderIs400(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postImport(t, "not-a-real-provider", "auto", "", []string{"https://example.com/x"}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (body=%s)", resp.StatusCode, env["message"])
	}
}

func TestKBImports_PDFWithFirecrawlIs400(t *testing.T) {
	h := newHarness(t)
	if err := h.kbImportCreds.Set(context.Background(), "firecrawl.api_key", "fc-test-key"); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	resp, env := h.postImport(t, "firecrawl", "auto", "", nil, map[string][]byte{"price-list.pdf": []byte("%PDF-1.4")})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (body=%s)", resp.StatusCode, env["message"])
	}
}

func TestKBImports_MissingCredentialIs422(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postImport(t, "firecrawl", "auto", "", []string{"https://vendor.example/x"}, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422 (body=%s)", resp.StatusCode, env["message"])
	}
}

func TestKBImports_SecondConcurrentRunIs409(t *testing.T) {
	h := newHarness(t)
	resp1, env1 := h.postImport(t, "native", "auto", "", []string{"https://example.com/first"}, nil)
	if resp1.StatusCode != http.StatusAccepted {
		t.Fatalf("first import status=%d, want 202 (body=%s)", resp1.StatusCode, env1["message"])
	}
	resp2, env2 := h.postImport(t, "native", "auto", "", []string{"https://example.com/second"}, nil)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second import status=%d, want 409 (body=%s)", resp2.StatusCode, env2["message"])
	}
}

func TestKBImports_202Shape(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postImport(t, "native", "products", "Возьми название и цену", []string{"https://example.com/a", "https://example.com/b"}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status=%d, want 202 (body=%s)", resp.StatusCode, env["message"])
	}
	var payload struct {
		RunID     string `json:"run_id"`
		Status    string `json:"status"`
		Materials []struct {
			ProcessingStatus string `json:"processing_status"`
		} `json:"materials"`
	}
	mustPayload(t, env, &payload)
	if payload.RunID == "" {
		t.Error("expected a non-empty run_id")
	}
	if len(payload.Materials) != 2 {
		t.Fatalf("materials = %+v, want 2", payload.Materials)
	}
}

func TestKBImports_GetRun_CrossOrgIs404(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postImport(t, "native", "auto", "", []string{"https://example.com/a"}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status=%d (body=%s)", resp.StatusCode, env["message"])
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	mustPayload(t, env, &created)

	// A second, unrelated org's session must never see the first org's run.
	h2 := newHarness(t)
	getResp, getEnv := h2.getRaw("/xchats/api/v1/kb/imports/" + created.RunID)
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org GET status=%d, want 404 (body=%s)", getResp.StatusCode, getEnv["message"])
	}
}

func TestKBImports_GetRun_UnknownIdIs404(t *testing.T) {
	h := newHarness(t)
	resp, env := h.getRaw("/xchats/api/v1/kb/imports/00000000-0000-0000-0000-000000000099")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 (body=%s)", resp.StatusCode, env["message"])
	}
}

// TestKBImports_Cancel_QueuedRun cancels immediately after Submit — before
// this harness's own running worker pool's ClaimEvery tick (3s by default)
// has any real chance to claim the material — the same "check right after
// POST, no sleep" timing TestKBImports_202Shape above already relies on.
func TestKBImports_Cancel_QueuedRun(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postImport(t, "native", "auto", "", []string{"https://example.com/a"}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status=%d (body=%s)", resp.StatusCode, env["message"])
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	mustPayload(t, env, &created)

	cancelResp, cancelEnv := h.postJSON("/xchats/api/v1/kb/imports/"+created.RunID+"/cancel", nil)
	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status=%d, want 200 (body=%s)", cancelResp.StatusCode, cancelEnv["message"])
	}
	var run struct {
		Status     string `json:"status"`
		Cancelable bool   `json:"cancelable"`
	}
	mustPayload(t, cancelEnv, &run)
	if run.Status != "cancelled" || run.Cancelable {
		t.Fatalf("cancel response = %+v, want status=cancelled, cancelable=false", run)
	}

	getResp, getEnv := h.getRaw("/xchats/api/v1/kb/imports/" + created.RunID)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET after cancel status=%d (body=%s)", getResp.StatusCode, getEnv["message"])
	}
	var got struct {
		Status string `json:"status"`
	}
	mustPayload(t, getEnv, &got)
	if got.Status != "cancelled" {
		t.Fatalf("GET run status = %q, want cancelled", got.Status)
	}
}

func TestKBImports_Cancel_CrossOrgIs404(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postImport(t, "native", "auto", "", []string{"https://example.com/a"}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status=%d (body=%s)", resp.StatusCode, env["message"])
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	mustPayload(t, env, &created)

	h2 := newHarness(t)
	cancelResp, cancelEnv := h2.postJSON("/xchats/api/v1/kb/imports/"+created.RunID+"/cancel", nil)
	if cancelResp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org cancel status=%d, want 404 (body=%s)", cancelResp.StatusCode, cancelEnv["message"])
	}
}

func TestKBImports_Cancel_UnknownIdIs404(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/kb/imports/00000000-0000-0000-0000-000000000099/cancel", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 (body=%s)", resp.StatusCode, env["message"])
	}
}

// TestKBImports_ListImports_PaginatesAndReportsTotal covers KB-14's history
// list: ?limit=&offset= slices the org's runs and the response's "total"
// reflects the full count, not just what came back on this page. Each run
// is cancelled right after submit (same "before ClaimEvery's first tick"
// timing TestKBImports_Cancel_QueuedRun above relies on) purely to free the
// one-active-run-per-org gate for the next submission — the runs' terminal
// status is incidental to what this test checks.
func TestKBImports_ListImports_PaginatesAndReportsTotal(t *testing.T) {
	h := newHarness(t)
	runIDs := make(map[string]bool, 3)
	for i := 0; i < 3; i++ {
		resp, env := h.postImport(t, "native", "auto", "", []string{fmt.Sprintf("https://example.com/%d", i)}, nil)
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("submit #%d status=%d (body=%s)", i, resp.StatusCode, env["message"])
		}
		var created struct {
			RunID string `json:"run_id"`
		}
		mustPayload(t, env, &created)
		runIDs[created.RunID] = true

		cancelResp, cancelEnv := h.postJSON("/xchats/api/v1/kb/imports/"+created.RunID+"/cancel", nil)
		if cancelResp.StatusCode != http.StatusOK {
			t.Fatalf("cancel #%d status=%d (body=%s)", i, cancelResp.StatusCode, cancelEnv["message"])
		}
	}

	type listPayload struct {
		Runs []struct {
			RunID string `json:"run_id"`
		} `json:"runs"`
		Total int `json:"total"`
	}

	resp1, env1 := h.getRaw("/xchats/api/v1/kb/imports?limit=2&offset=0")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("page1 status=%d (body=%s)", resp1.StatusCode, env1["message"])
	}
	var page1 listPayload
	mustPayload(t, env1, &page1)
	if page1.Total != 3 || len(page1.Runs) != 2 {
		t.Fatalf("page1 = %+v, want 2 runs (total 3)", page1)
	}

	resp2, env2 := h.getRaw("/xchats/api/v1/kb/imports?limit=2&offset=2")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("page2 status=%d (body=%s)", resp2.StatusCode, env2["message"])
	}
	var page2 listPayload
	mustPayload(t, env2, &page2)
	if page2.Total != 3 || len(page2.Runs) != 1 {
		t.Fatalf("page2 = %+v, want 1 run (total 3)", page2)
	}

	seen := map[string]bool{}
	for _, r := range append(page1.Runs, page2.Runs...) {
		if seen[r.RunID] {
			t.Fatalf("run %s appeared on more than one page", r.RunID)
		}
		seen[r.RunID] = true
		if !runIDs[r.RunID] {
			t.Fatalf("unexpected run id %s", r.RunID)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("pages together covered %d runs, want all 3", len(seen))
	}
}

// TestKBImports_EndToEnd_FirecrawlURLLandsInDraftOnly drives the WHOLE
// pipeline through real HTTP: submit a URL against an httptest Firecrawl
// double, wait for the background pipeline (already running on this
// harness) to extract and synthesize it, then confirm GET /playground/draft
// shows the new product while GET /kb (live) is untouched — nothing
// reaches live ai_* without an explicit Publish.
func TestKBImports_EndToEnd_FirecrawlURLLandsInDraftOnly(t *testing.T) {
	modelResp, err := json.Marshal(map[string]any{
		"calls": []map[string]any{{
			"tool": "kb_product_upsert",
			"args": map[string]any{"ref": "zt40h", "changes": map[string]any{"name": "Станок ZT-40H", "price": "180 000 ₸", "availability_status": "in_stock"}},
		}},
		"notes": "готово", "unmapped": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := newHarnessWithLLM(t, scriptedImportClient{response: string(modelResp)})

	firecrawlDouble := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fc-e2e-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"markdown": "# ZT-40H\n\nЦена: 180 000 ₸\n"},
		})
	}))
	defer firecrawlDouble.Close()

	if err := h.kbImportCreds.Set(context.Background(), "firecrawl.api_key", "fc-e2e-key"); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	if _, err := h.kbImportSettings.Update(func(s *settings.Settings) {
		if s.Providers == nil {
			s.Providers = map[string]settings.ProviderSettings{}
		}
		s.Providers["firecrawl"] = settings.ProviderSettings{BaseURL: firecrawlDouble.URL}
	}); err != nil {
		t.Fatalf("point firecrawl base_url at the double: %v", err)
	}

	resp, env := h.postImport(t, "firecrawl", "auto", "", []string{"https://vendor.example/zt40h"}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status=%d, want 202 (body=%s)", resp.StatusCode, env["message"])
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	mustPayload(t, env, &created)

	deadline := time.Now().Add(5 * time.Second)
	var finalStatus string
	for time.Now().Before(deadline) {
		getResp, getEnv := h.getRaw("/xchats/api/v1/kb/imports/" + created.RunID)
		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("GET run status=%d (body=%s)", getResp.StatusCode, getEnv["message"])
		}
		var status struct {
			Status string `json:"status"`
		}
		mustPayload(t, getEnv, &status)
		finalStatus = status.Status
		if finalStatus == "built" || finalStatus == "failed" || finalStatus == "needs_human" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if finalStatus != "built" {
		t.Fatalf("final run status = %q, want built", finalStatus)
	}

	var draft struct {
		Products []struct {
			Ref   string `json:"ref"`
			Name  string `json:"name"`
			Price string `json:"price"`
		} `json:"products"`
	}
	h.get("/xchats/api/v1/playground/draft", &draft)
	found := false
	for _, p := range draft.Products {
		if p.Ref == "zt40h" {
			found = true
			if p.Price != "180 000 ₸" {
				t.Errorf("draft product price = %q, want 180 000 ₸", p.Price)
			}
		}
	}
	if !found {
		t.Fatalf("draft products = %+v, want zt40h present", draft.Products)
	}

	var live struct {
		Products []struct {
			Ref string `json:"ref"`
		} `json:"products"`
	}
	h.get("/xchats/api/v1/kb", &live)
	for _, p := range live.Products {
		if p.Ref == "zt40h" {
			t.Fatalf("live KB already contains zt40h — an import must never reach ai_* without Publish")
		}
	}
}

// TestKBImportProviders_ShapeAndFamilies checks the endpoint against
// plan/playground.md's own capability table without any credential
// configured: native is always usable (no credential needed at all — see
// handleKBImportProviders' "Configured defaults true" comment), firecrawl
// and llamaparse both require one and, unconfigured here, report
// configured=false.
func TestKBImportProviders_ShapeAndFamilies(t *testing.T) {
	h := newHarness(t)
	var body struct {
		Providers []struct {
			ID                 string   `json:"id"`
			DisplayName        string   `json:"display_name"`
			Families           []string `json:"families"`
			RequiresCredential bool     `json:"requires_credential"`
			Configured         bool     `json:"configured"`
		} `json:"providers"`
	}
	h.get("/xchats/api/v1/kb/import/providers", &body)

	byID := map[string]struct {
		Families           []string
		RequiresCredential bool
		Configured         bool
	}{}
	for _, p := range body.Providers {
		byID[p.ID] = struct {
			Families           []string
			RequiresCredential bool
			Configured         bool
		}{p.Families, p.RequiresCredential, p.Configured}
	}

	native, ok := byID["native"]
	if !ok {
		t.Fatal("providers list is missing native")
	}
	if native.RequiresCredential {
		t.Error("native.requires_credential = true, want false")
	}
	if !native.Configured {
		t.Error("native.configured = false, want true — it needs no credential at all")
	}
	if !containsStr(native.Families, "url") || !containsStr(native.Families, "docx") || containsStr(native.Families, "pdf") {
		t.Errorf("native.families = %v, want url+docx present, pdf absent", native.Families)
	}

	firecrawl, ok := byID["firecrawl"]
	if !ok {
		t.Fatal("providers list is missing firecrawl")
	}
	if !firecrawl.RequiresCredential {
		t.Error("firecrawl.requires_credential = false, want true")
	}
	if firecrawl.Configured {
		t.Error("firecrawl.configured = true, want false — no credential was saved")
	}
	if !containsStr(firecrawl.Families, "url") || len(firecrawl.Families) != 1 {
		t.Errorf("firecrawl.families = %v, want exactly [url]", firecrawl.Families)
	}

	llamaparse, ok := byID["llamaparse"]
	if !ok {
		t.Fatal("providers list is missing llamaparse")
	}
	if !containsStr(llamaparse.Families, "pdf") {
		t.Errorf("llamaparse.families = %v, want pdf present", llamaparse.Families)
	}
}

// TestKBImportProviders_ConfiguredFlipsOnceCredentialSaved is the "no
// disagreement with Submit's own precheck" guarantee: the same credential
// save Submit's resolveProvider would pick up flips this endpoint's
// configured flag too, with no restart.
func TestKBImportProviders_ConfiguredFlipsOnceCredentialSaved(t *testing.T) {
	h := newHarness(t)
	if err := h.kbImportCreds.Set(context.Background(), "firecrawl.api_key", "fc-test-key"); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	var body struct {
		Providers []struct {
			ID         string `json:"id"`
			Configured bool   `json:"configured"`
		} `json:"providers"`
	}
	h.get("/xchats/api/v1/kb/import/providers", &body)
	for _, p := range body.Providers {
		if p.ID == "firecrawl" {
			if !p.Configured {
				t.Error("firecrawl.configured = false after saving its key, want true")
			}
			return
		}
	}
	t.Fatal("providers list is missing firecrawl")
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// scriptedImportClient is a fake llm.ChatClient dedicated to this file's
// end-to-end test — kept separate from fakeLLMClient (integration_test.go)
// since that one's fixed response is the customer-response contract's
// shape, not kbimport's {"calls": [...]}.
type scriptedImportClient struct{ response string }

func (c scriptedImportClient) Complete(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Text: c.response, PromptTokens: 10, CompletionTokens: 5}, nil
}
