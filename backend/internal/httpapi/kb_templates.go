package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// --- /kb/templates — editable prompt templates (General Template tab) -------
//
// One ai_prompt_templates row per (organization, template ID); a PUT updates
// its row in place. The selected profile is stored on the assistant settings
// row. The FINAL prompt is never stored: GET /kb/prompt renders it on demand
// from the active template and the current KB.

type promptTemplateJSON struct {
	ID            string     `json:"id"`
	Instructions  string     `json:"instructions"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
	IsDefaultText bool       `json:"is_default_text"`
}

type promptTemplatesJSON struct {
	ActiveTemplateID string               `json:"active_template_id"`
	KBConfigured     bool                 `json:"kb_configured"`
	Templates        []promptTemplateJSON `json:"templates"`
}

func templatesJSON(v *kbstore.PromptTemplatesView) promptTemplatesJSON {
	out := promptTemplatesJSON{ActiveTemplateID: v.ActiveTemplateID, KBConfigured: v.KBConfigured}
	for _, t := range v.Templates {
		row := promptTemplateJSON{ID: t.ID, Instructions: t.Instructions, IsDefaultText: t.IsDefaultText}
		if !t.UpdatedAt.IsZero() {
			u := t.UpdatedAt
			row.UpdatedAt = &u
		}
		out.Templates = append(out.Templates, row)
	}
	return out
}

func (s *Server) handleKBGetTemplates(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	view, err := s.kb.ListPromptTemplates(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, templatesJSON(view))
}

type putTemplateReq struct {
	Instructions string `json:"instructions"`
	// Activate makes this profile the active one in the same write. The General
	// Template tab always sends true: saving a profile selects it.
	Activate bool `json:"activate"`
}

func (s *Server) handleKBPutTemplate(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	id := c.Param("id")
	if !aiprompt.IsPromptTemplateID(id) {
		fail(c, http.StatusNotFound, ErrNotFound, "unknown template")
		return
	}
	var req putTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad template")
		return
	}
	if err := aiprompt.ValidateTemplateInstructions(req.Instructions); err != nil {
		fail(c, http.StatusUnprocessableEntity, ErrValidation, err.Error())
		return
	}
	if msg := s.trialRenderTemplate(c, orgID, id, req.Instructions); msg != "" {
		fail(c, http.StatusUnprocessableEntity, ErrValidation, msg)
		return
	}
	if err := s.kb.SavePromptTemplate(ctx(c), orgID, currentUser(c).ID, id, req.Instructions, req.Activate); err != nil {
		switch {
		case errors.Is(err, kbstore.ErrUnknownTemplate):
			fail(c, http.StatusNotFound, ErrNotFound, "unknown template")
		case isTemplateValidationError(err):
			fail(c, http.StatusUnprocessableEntity, ErrValidation, err.Error())
		default:
			s.kbFail(c, err)
		}
		return
	}
	// Drop the cached prompt-facing KB (it carries the template) so the NEXT
	// customer reply and the next GET /kb/prompt rebuild from the new text, and
	// tell every open tab to refresh its Final Template preview.
	s.invalidateKBCache(orgID)
	s.hub.Broadcast("kb.row.changed", gin.H{})
	view, err := s.kb.ListPromptTemplates(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, templatesJSON(view))
}

func isTemplateValidationError(err error) bool {
	return errors.Is(err, aiprompt.ErrTemplateEmpty) || errors.Is(err, aiprompt.ErrTemplateTooLong) ||
		errors.Is(err, aiprompt.ErrTemplateReservedSyntax) || errors.Is(err, aiprompt.ErrTemplateLeakShapedString)
}

// trialRenderTemplate renders the candidate text against the organization's
// current KB through the SAME builder customer replies use, returning a
// non-empty user-facing message when the candidate would break the render. It
// is skipped (returns "") when there is nothing to render against — the KB is
// not configured, or the KB itself already fails to build — since a template
// edit is not the cause of either.
func (s *Server) trialRenderTemplate(c *gin.Context, orgID uuid.UUID, id, instructions string) string {
	if s.kbRepo == nil {
		return ""
	}
	kb, err := s.kbRepo.Load(ctx(c), orgID.String())
	if err != nil {
		return ""
	}
	// The cached KB is shared: render copies, never mutate it.
	current := *kb
	if _, _, err := response.RenderSystemPrompt(&current, messaging.ChannelWhatsApp); err != nil {
		return ""
	}
	candidate := *kb
	candidate.PromptTemplate = &aiprompt.PromptTemplate{ID: id, Instructions: instructions}
	if _, _, err := response.RenderSystemPrompt(&candidate, messaging.ChannelWhatsApp); err != nil {
		return "the template does not render against your current knowledge base: " + err.Error()
	}
	return ""
}
