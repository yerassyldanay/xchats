// campaign_templates.go is the HTTP edge for CAM-14's reusable message
// template library — organization-wide, applied to a campaign from the
// wizard rather than retyped or copy-pasted from an old campaign. Mirrors
// campaigns.go's own CRUD shape (orgCampaign -> orgCampaignTemplate,
// campaignDTO -> a plain dto.MapCampaignTemplate call, since a template has
// no windows or recipient counts to assemble alongside it).
package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// orgCampaignTemplate resolves a template id and enforces org ownership —
// mirrors orgCampaign.
func (s *Server) orgCampaignTemplate(c *gin.Context, id uuid.UUID) (store.CampaignTemplate, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.CampaignTemplate{}, false
	}
	tmpl, err := s.store.CampaignTemplateByIDForOrg(ctx(c), id, org.ID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "template not found")
		return store.CampaignTemplate{}, false
	}
	return tmpl, true
}

// handleListCampaignTemplates is GET /campaign-templates: one page of the
// org's templates on one side of the archived split (?archived=true|false,
// default false — the Active library), optionally filtered by ?q=
// substring-matching the name.
func (s *Server) handleListCampaignTemplates(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	archived, _ := strconv.ParseBool(c.DefaultQuery("archived", "false"))
	q := strings.TrimSpace(c.Query("q"))
	limit, offset, pageNum, pageSize := s.pageParams(c)
	items, total, err := s.store.ListCampaignTemplatesForOrg(ctx(c), org.ID, archived, q, limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	out := make([]dto.CampaignTemplate, 0, len(items))
	for _, it := range items {
		out = append(out, dto.MapCampaignTemplate(it))
	}
	ok(c, page{Items: out, Page: pageNum, PageSize: pageSize, Total: total})
}

type campaignTemplateReq struct {
	Name        string `json:"name"`
	MessageBody string `json:"message_body"`
}

func (s *Server) handleCreateCampaignTemplate(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	var req campaignTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if err := purecampaign.ValidateName(name); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	if err := purecampaign.ValidateMessageBody(req.MessageBody); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	tmpl, err := s.store.CreateCampaignTemplate(ctx(c), store.CampaignTemplate{
		OrganizationID: org.ID, Name: name, MessageBody: req.MessageBody, CreatedBy: currentUser(c).ID,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	created(c, dto.MapCampaignTemplate(tmpl))
}

func (s *Server) handleGetCampaignTemplate(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	tmpl, okT := s.orgCampaignTemplate(c, id)
	if !okT {
		return
	}
	ok(c, dto.MapCampaignTemplate(tmpl))
}

func (s *Server) handleUpdateCampaignTemplate(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if _, okT := s.orgCampaignTemplate(c, id); !okT {
		return
	}

	var req struct {
		Name        *string `json:"name"`
		MessageBody *string `json:"message_body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}

	patch := store.CampaignTemplatePatch{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if err := purecampaign.ValidateName(name); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
		patch.Name = &name
	}
	if req.MessageBody != nil {
		if err := purecampaign.ValidateMessageBody(*req.MessageBody); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
		patch.MessageBody = req.MessageBody
	}

	updated, err := s.store.UpdateCampaignTemplate(ctx(c), id, patch)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, dto.MapCampaignTemplate(updated))
}

func (s *Server) setCampaignTemplateArchived(c *gin.Context, archived bool) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if _, okT := s.orgCampaignTemplate(c, id); !okT {
		return
	}
	updated, err := s.store.SetCampaignTemplateArchived(ctx(c), id, archived)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, dto.MapCampaignTemplate(updated))
}

func (s *Server) handleArchiveCampaignTemplate(c *gin.Context) { s.setCampaignTemplateArchived(c, true) }
func (s *Server) handleRestoreCampaignTemplate(c *gin.Context) { s.setCampaignTemplateArchived(c, false) }
