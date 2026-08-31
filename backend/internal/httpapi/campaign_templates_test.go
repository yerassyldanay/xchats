package httpapi_test

import (
	"net/http"
	"testing"
)

type campaignTemplateDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	MessageBody string   `json:"message_body"`
	Variables   []string `json:"variables"`
	IsArchived  bool     `json:"is_archived"`
	CreatedBy   string   `json:"created_by"`
}

type campaignTemplatePageDTO struct {
	Items []campaignTemplateDTO `json:"items"`
	Total int                   `json:"total"`
}

func (h *harness) createCampaignTemplate(t *testing.T, name, body string) campaignTemplateDTO {
	t.Helper()
	resp, env := h.postJSON("/xchats/api/v1/campaign-templates", map[string]any{"name": name, "message_body": body})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create campaign template status=%d body=%s", resp.StatusCode, env["message"])
	}
	var tmpl campaignTemplateDTO
	mustPayload(t, env, &tmpl)
	return tmpl
}

func TestCreateAndGetCampaignTemplate(t *testing.T) {
	h := newHarness(t)
	tmpl := h.createCampaignTemplate(t, "Summer promo", "Hi {{name}}, {{discount}}% off!")
	if tmpl.IsArchived {
		t.Error("a freshly created template must not be archived")
	}
	if len(tmpl.Variables) != 2 {
		t.Errorf("variables = %v, want 2 entries", tmpl.Variables)
	}

	var got campaignTemplateDTO
	h.get("/xchats/api/v1/campaign-templates/"+tmpl.ID, &got)
	if got.ID != tmpl.ID || got.Name != "Summer promo" {
		t.Errorf("get = %+v", got)
	}

	resp, _ := h.postJSON("/xchats/api/v1/campaign-templates", map[string]any{"name": "", "message_body": "hi"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty name status = %d, want 400", resp.StatusCode)
	}
	resp2, _ := h.postJSON("/xchats/api/v1/campaign-templates", map[string]any{"name": "No body"})
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("empty message_body status = %d, want 400", resp2.StatusCode)
	}
}

func TestGetCampaignTemplate_CrossOrgIs404(t *testing.T) {
	h := newHarness(t)
	tmpl := h.createCampaignTemplate(t, "Org A only", "hi")

	h2 := newHarness(t)
	resp, env := h2.getRaw("/xchats/api/v1/campaign-templates/" + tmpl.ID)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org get status=%d, want 404 (body=%s)", resp.StatusCode, env["message"])
	}
}

func TestUpdateCampaignTemplate(t *testing.T) {
	h := newHarness(t)
	tmpl := h.createCampaignTemplate(t, "Original", "Hi {{name}}")

	resp, env := h.patchJSON("/xchats/api/v1/campaign-templates/"+tmpl.ID, map[string]any{
		"name": "Renamed", "message_body": "Hello {{name}}, {{offer}} awaits",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d body=%s", resp.StatusCode, env["message"])
	}
	var updated campaignTemplateDTO
	mustPayload(t, env, &updated)
	if updated.Name != "Renamed" || len(updated.Variables) != 2 {
		t.Errorf("updated = %+v", updated)
	}

	respBad, _ := h.patchJSON("/xchats/api/v1/campaign-templates/"+tmpl.ID, map[string]any{"name": "   "})
	if respBad.StatusCode != http.StatusBadRequest {
		t.Errorf("blank-name update status = %d, want 400", respBad.StatusCode)
	}
}

func TestArchiveAndRestoreCampaignTemplate(t *testing.T) {
	h := newHarness(t)
	tmpl := h.createCampaignTemplate(t, "Seasonal", "Happy holidays {{name}}")

	resp, env := h.postJSON("/xchats/api/v1/campaign-templates/"+tmpl.ID+"/archive", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", resp.StatusCode, env["message"])
	}
	var archived campaignTemplateDTO
	mustPayload(t, env, &archived)
	if !archived.IsArchived {
		t.Error("is_archived = false after archiving, want true")
	}

	resp2, env2 := h.postJSON("/xchats/api/v1/campaign-templates/"+tmpl.ID+"/restore", nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("restore status=%d body=%s", resp2.StatusCode, env2["message"])
	}
	var restored campaignTemplateDTO
	mustPayload(t, env2, &restored)
	if restored.IsArchived {
		t.Error("is_archived = true after restoring, want false")
	}
}

func TestListCampaignTemplates_ArchivedFilterAndSearch(t *testing.T) {
	h := newHarness(t)
	active := h.createCampaignTemplate(t, "Летняя акция", "Привет {{name}}")
	toArchive := h.createCampaignTemplate(t, "Old winter sale", "Bye {{name}}")
	if resp, _ := h.postJSON("/xchats/api/v1/campaign-templates/"+toArchive.ID+"/archive", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("archive setup failed: status=%d", resp.StatusCode)
	}

	var activePage campaignTemplatePageDTO
	h.get("/xchats/api/v1/campaign-templates", &activePage)
	if activePage.Total != 1 || len(activePage.Items) != 1 || activePage.Items[0].ID != active.ID {
		t.Errorf("default (active) list = %+v, want just %q", activePage, active.Name)
	}

	var archivedPage campaignTemplatePageDTO
	h.get("/xchats/api/v1/campaign-templates?archived=true", &archivedPage)
	if archivedPage.Total != 1 || len(archivedPage.Items) != 1 || archivedPage.Items[0].ID != toArchive.ID {
		t.Errorf("archived list = %+v, want just %q", archivedPage, toArchive.Name)
	}

	var searched campaignTemplatePageDTO
	h.get("/xchats/api/v1/campaign-templates?q=%D0%BB%D0%B5%D1%82%D0%BD", &searched) // "летн"
	if searched.Total != 1 || len(searched.Items) != 1 || searched.Items[0].ID != active.ID {
		t.Errorf("search list = %+v, want just %q", searched, active.Name)
	}
}
