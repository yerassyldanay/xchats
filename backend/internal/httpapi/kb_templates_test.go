package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// --- a tiny JSON client shared by both harness flavours ----------------------

type tmplAPI struct {
	t      *testing.T
	base   string
	client *http.Client
}

func (a tmplAPI) do(method, path string, body any) (*http.Response, json.RawMessage) {
	a.t.Helper()
	var rd *bytes.Reader
	if raw, ok := body.([]byte); ok {
		rd = bytes.NewReader(raw)
	} else {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, a.base+path, rd)
	if err != nil {
		a.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		a.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp, env.Payload
}

func (a tmplAPI) templates() templatesPayload {
	a.t.Helper()
	resp, raw := a.do(http.MethodGet, "/xchats/api/v1/kb/templates", nil)
	if resp.StatusCode != http.StatusOK {
		a.t.Fatalf("GET /kb/templates = %d", resp.StatusCode)
	}
	var out templatesPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		a.t.Fatal(err)
	}
	return out
}

func (a tmplAPI) prompt() promptPayload {
	a.t.Helper()
	_, raw := a.do(http.MethodGet, "/xchats/api/v1/kb/prompt", nil)
	var out promptPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		a.t.Fatal(err)
	}
	return out
}

func (a tmplAPI) put(id, text string, activate bool) *http.Response {
	a.t.Helper()
	resp, _ := a.do(http.MethodPut, "/xchats/api/v1/kb/templates/"+id, map[string]any{"instructions": text, "activate": activate})
	return resp
}

type templatesPayload struct {
	ActiveTemplateID string `json:"active_template_id"`
	KBConfigured     bool   `json:"kb_configured"`
	Templates        []struct {
		ID            string `json:"id"`
		Instructions  string `json:"instructions"`
		IsDefaultText bool   `json:"is_default_text"`
	} `json:"templates"`
}

func (p templatesPayload) text(id string) string {
	for _, t := range p.Templates {
		if t.ID == id {
			return t.Instructions
		}
	}
	return "<missing>"
}

func apiOf(h *harness) tmplAPI             { return tmplAPI{t: h.t, base: h.srv.URL, client: h.client} }
func apiOfPrompt(h *promptHarness) tmplAPI { return tmplAPI{t: h.t, base: h.srv.URL, client: h.client} }

// A brand-new organization: all four defaults are there, General is active, the
// assistant is NOT configured — and the preview still shows the instructions.
func TestKBTemplates_FreshOrganization(t *testing.T) {
	api := apiOfPrompt(newPromptHarness(t))

	got := api.templates()
	if got.ActiveTemplateID != aiprompt.TemplateGeneral || got.KBConfigured {
		t.Fatalf("fresh org: active=%q configured=%v, want general/false", got.ActiveTemplateID, got.KBConfigured)
	}
	if len(got.Templates) != 4 {
		t.Fatalf("want 4 templates, got %d", len(got.Templates))
	}
	for i, id := range aiprompt.PromptTemplateIDs() {
		want, _ := aiprompt.DefaultTemplateInstructions(id)
		if got.Templates[i].ID != id || got.Templates[i].Instructions != want {
			t.Errorf("template %d = %q, want the shipped default of %q", i, got.Templates[i].ID, id)
		}
		if got.Templates[i].IsDefaultText {
			t.Errorf("%s: a seeded org has database rows, not in-memory defaults", id)
		}
	}

	p := api.prompt()
	if p.Status != "not_configured" || p.TemplateID != "general" {
		t.Fatalf("preview status=%q template=%q, want not_configured/general", p.Status, p.TemplateID)
	}
	general, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateGeneral)
	if !strings.Contains(p.RenderedText, strings.TrimSpace(general)) || !strings.Contains(p.RenderedText, aiprompt.OutputContractLine) {
		t.Errorf("the not-configured preview must still render the default instructions and protected structure:\n%.400s", p.RenderedText)
	}
	if !strings.Contains(p.Error, "not configured") {
		t.Errorf("error = %q, want the not-configured explanation", p.Error)
	}
}

// Choosing a profile must never make an unconfigured assistant look set up.
func TestKBTemplates_ProfileSwitchBeforeConfiguration(t *testing.T) {
	api := apiOfPrompt(newPromptHarness(t))

	shop, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateOnlineShop)
	if resp := api.put("online-shop", shop, true); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT online-shop = %d", resp.StatusCode)
	}
	got := api.templates()
	if got.ActiveTemplateID != "online-shop" || got.KBConfigured {
		t.Fatalf("after switching: active=%q configured=%v, want online-shop/false", got.ActiveTemplateID, got.KBConfigured)
	}
	p := api.prompt()
	if p.Status != "not_configured" || p.TemplateID != "online-shop" || !strings.Contains(p.RenderedText, "ПРОФИЛЬ: интернет-магазин") {
		t.Fatalf("preview after switch: status=%q template=%q", p.Status, p.TemplateID)
	}

	// The first real settings save configures the assistant and keeps the profile.
	resp, _ := api.do(http.MethodPatch, "/xchats/api/v1/kb/config", map[string]any{"persona": "Ты — тестовый ассистент."})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH /kb/config = %d", resp.StatusCode)
	}
	got = api.templates()
	if got.ActiveTemplateID != "online-shop" || !got.KBConfigured {
		t.Fatalf("after configuring: active=%q configured=%v, want online-shop/true", got.ActiveTemplateID, got.KBConfigured)
	}
	if p := api.prompt(); p.Status != "ok" || p.TemplateID != "online-shop" {
		t.Fatalf("preview after configuring: status=%q template=%q", p.Status, p.TemplateID)
	}
}

func TestKBTemplates_SwitchingProfilesPreservesEdits(t *testing.T) {
	h := newHarness(t)
	api := apiOf(h)

	if resp := api.put("general", "ПРАВИЛА-GENERAL-1", true); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT general = %d", resp.StatusCode)
	}
	if resp := api.put("online-shop", "ПРАВИЛА-SHOP-1", true); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT online-shop = %d", resp.StatusCode)
	}
	got := api.templates()
	if got.ActiveTemplateID != "online-shop" || !got.KBConfigured {
		t.Fatalf("active=%q configured=%v", got.ActiveTemplateID, got.KBConfigured)
	}
	// Switch back without touching text: General's edit is still there, and so
	// is every untouched default.
	if resp := api.put("general", "ПРАВИЛА-GENERAL-1", true); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT general = %d", resp.StatusCode)
	}
	got = api.templates()
	if got.ActiveTemplateID != "general" || got.text("general") != "ПРАВИЛА-GENERAL-1" || got.text("online-shop") != "ПРАВИЛА-SHOP-1" {
		t.Fatalf("edits not preserved across profile switches: active=%q", got.ActiveTemplateID)
	}
	want, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateServiceBusiness)
	if got.text("service-business") != want {
		t.Error("an untouched profile must keep its shipped default")
	}
	// Saving without activate edits the row but leaves the active profile alone.
	api.put("online-service", "ПРАВИЛА-SERVICE-1", false)
	if got = api.templates(); got.ActiveTemplateID != "general" || got.text("online-service") != "ПРАВИЛА-SERVICE-1" {
		t.Errorf("activate=false must not switch profiles: active=%q", got.ActiveTemplateID)
	}
}

func TestKBTemplates_ValidationAndErrors(t *testing.T) {
	h := newHarness(t)
	api := apiOf(h)

	for name, text := range map[string]string{
		"empty":       "   ",
		"slot marker": "вставь %%FACTS%% сюда",
		"placeholder": "цена {{product.x.price}}",
		"uuid":        "id 123e4567-e89b-12d3-a456-426614174000",
		"file name":   "смотри price.pdf",
		"link":        "наш сайт https://example.com",
		"over limit":  strings.Repeat("я", aiprompt.MaxTemplateRunes+1),
	} {
		if resp := api.put("general", text, true); resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", name, resp.StatusCode)
		}
	}
	if resp := api.put("no-such-profile", "текст", true); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown ID: status %d, want 404", resp.StatusCode)
	}
	if resp, _ := api.do(http.MethodPut, "/xchats/api/v1/kb/templates/general", []byte("{not json")); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed JSON: status %d, want 400", resp.StatusCode)
	}
	// Nothing above may have changed a thing.
	want, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateGeneral)
	if got := api.templates(); got.text("general") != want {
		t.Error("a rejected save must not modify the stored template")
	}
}

// A candidate that passes the static checks but would break the render against
// the current KB is rejected before it is saved (here: it names a stored file).
func TestKBTemplates_TrialRenderRejectsTemplatesThatBreakTheRender(t *testing.T) {
	h := newHarness(t)
	api := apiOf(h)
	ctx := context.Background()
	if _, err := h.db.Exec(ctx, `INSERT INTO kbd_materials (id, organization_id, source_type, filename, created_at, updated_at)
		VALUES ($1, $2, 'file', 'zq-private-asset-77', $3, $3)`, uuid.NewString(), h.orgID, int64(1791417600000)); err != nil {
		t.Fatal(err)
	}
	h.invalidateKB()

	resp := api.put("general", "Правила, где упомянут файл zq-private-asset-77 целиком.", true)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422 from the trial render", resp.StatusCode)
	}
	want, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateGeneral)
	if got := api.templates(); got.text("general") != want {
		t.Error("the rejected candidate must not have been stored")
	}
}

// capturingLLM records the prompt of every model call.
type capturingLLM struct {
	mu      sync.Mutex
	prompts []string
}

func (c *capturingLLM) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prompts = append(c.prompts, req.Messages[0].Content)
	return llm.ChatResponse{Text: `{"reply_text":"Секунду, уточню и вернусь.","reply_language":"ru","media_files_to_send":[],"escalate":false}`}, nil
}

func (c *capturingLLM) last(t *testing.T) string {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.prompts) == 0 {
		t.Fatal("the model was never called")
	}
	return c.prompts[len(c.prompts)-1]
}

// The whole point: an edited template and fresh KB data reach the model on the
// very next reply, and the reply prompt is exactly what the preview shows.
func TestKBTemplates_EditAndKBChangeReachRealAIResponses(t *testing.T) {
	cap := &capturingLLM{}
	h := newHarnessWithLLM(t, cap)
	api := apiOf(h)

	ask := func(text string) {
		t.Helper()
		resp, _ := h.postJSON("/xchats/api/v1/simulator/messages", map[string]any{
			"contact_ref": "tmpl-contact", "conversation_ref": "tmpl-conv", "text": text, "wait_for_response": true,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("simulator message = %d", resp.StatusCode)
		}
	}

	if resp := api.put("general", "МАРКЕР-ПРАВИЛ-ОДИН: отвечай кратко.", true); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT = %d", resp.StatusCode)
	}
	if resp, _ := h.postJSON("/xchats/api/v1/kb/products", map[string]any{
		"ref": "new-gadget", "name": "Новый гаджет", "price": "7 777 ₸", "description": "МАРКЕР-ТОВАРА",
		"category": "Техника", "availability_status": "in_stock",
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("upsert product = %d", resp.StatusCode)
	}

	ask("Здравствуйте")
	preview := api.prompt()
	if preview.Status != "ok" || !strings.Contains(preview.RenderedText, "МАРКЕР-ПРАВИЛ-ОДИН") || !strings.Contains(preview.RenderedText, "product: new-gadget") {
		t.Fatalf("preview lacks the edit or the new product (status %q)", preview.Status)
	}
	if got := cap.last(t); !strings.HasPrefix(got, preview.RenderedText) {
		t.Fatal("the model prompt must begin with exactly the Final Template preview text")
	}

	// A second edit and a KB edit are both live on the next reply.
	api.put("general", "МАРКЕР-ПРАВИЛ-ДВА: отвечай подробно.", true)
	h.postJSON("/xchats/api/v1/kb/products", map[string]any{
		"ref": "new-gadget", "name": "Новый гаджет", "price": "7 777 ₸", "description": "МАРКЕР-ТОВАРА-ИЗМЕНЁН",
		"category": "Техника", "availability_status": "in_stock",
	})
	ask("Ещё вопрос")
	got := cap.last(t)
	if !strings.Contains(got, "МАРКЕР-ПРАВИЛ-ДВА") || strings.Contains(got, "МАРКЕР-ПРАВИЛ-ОДИН") || !strings.Contains(got, "МАРКЕР-ТОВАРА-ИЗМЕНЁН") {
		t.Error("the second reply must use the edited template and the edited product")
	}
	if p := api.prompt(); !strings.HasPrefix(got, p.RenderedText) {
		t.Error("after the edits the model prompt must still equal the preview")
	}
}
