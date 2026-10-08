package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// promptSectionCounts mirrors the "Сформирован из разделов" sidebar list on
// the Final Template tab — how many rows of each kind fed the render.
type promptSectionCounts struct {
	Topics      int `json:"topics"`
	Products    int `json:"products"`
	Tariffs     int `json:"tariffs"`
	Zones       int `json:"zones"`
	Contacts    int `json:"contacts"`
	Policies    int `json:"policies"`
	Specialists int `json:"specialists"`
	Services    int `json:"services"`
}

// Preview statuses of promptView.Status.
const (
	promptStatusOK            = "ok"
	promptStatusError         = "error"
	promptStatusNotConfigured = "not_configured"
)

// promptView is the GET /kb/prompt payload: everything the Final Template tab
// needs to show the exact prompt the response engine would build right now,
// plus enough metadata (size/tokens/status) to render its "О промпте" sidebar
// without a second round trip.
type promptView struct {
	PromptRef     string              `json:"prompt_ref"`
	TemplateID    string              `json:"template_id"`
	RenderedText  string              `json:"rendered_text"`
	FrameText     string              `json:"frame_text"`
	CharCount     int                 `json:"char_count"`
	ApproxTokens  int                 `json:"approx_tokens"`
	BuiltAt       time.Time           `json:"built_at"`
	Status        string              `json:"status"` // ok | not_configured | error
	Error         string              `json:"error,omitempty"`
	SectionCounts promptSectionCounts `json:"section_counts"`
}

// handleKBPrompt renders the exact prompt the response engine would send for
// this org right now. It reads through s.kbRepo — the SAME CachedKBRepo the
// production reply path reads (server.go's doc comment on that field) — and
// renders with response.RenderSystemPrompt — the SAME builder Engine.Generate
// calls — so this can never be a second, possibly-divergent rendering path.
//
// An organization whose assistant settings were never saved is not an error
// here: its active template's default instructions are rendered over an empty
// KB and the view is marked status:"not_configured", so a new user sees the
// instructions their assistant will run on (and why it will not reply yet)
// instead of a blank tab. Any other KB-load or render failure is reported as
// status:"error" with the exact message in a normal 200 response — the
// operator is looking at exactly why the AI would fail to answer right now,
// not chasing an opaque 5xx.
func (s *Server) handleKBPrompt(c *gin.Context) {
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	if s.kbRepo == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "prompt builder not available")
		return
	}

	view := promptView{BuiltAt: time.Now(), Status: promptStatusOK}
	kb, err := s.kbRepo.Load(ctx(c), orgID.String())
	if errors.Is(err, responsestore.ErrKBNotConfigured) {
		view.Status = promptStatusNotConfigured
		view.Error = err.Error()
		kb, err = s.unconfiguredPreviewKB(c, orgID)
	}
	if err != nil {
		view.Status, view.Error = promptStatusError, err.Error()
		ok(c, view)
		return
	}

	view.TemplateID, view.PromptRef = templateIDOf(kb), response.PromptRefFor(kb)
	view.FrameText = response.FrameFor(kb, messaging.ChannelWhatsApp)
	view.SectionCounts = promptSectionCounts{
		Topics: len(kb.Topics), Products: len(kb.Products), Tariffs: len(kb.Tariffs), Zones: len(kb.DeliveryZones),
		Specialists: len(kb.Specialists), Services: len(kb.Services),
	}
	if kb.Contacts != nil {
		view.SectionCounts.Contacts = 1
	}
	if kb.Policies != nil {
		view.SectionCounts.Policies = 1
	}

	sys, _, err := response.RenderSystemPrompt(kb, messaging.ChannelWhatsApp)
	if err != nil {
		view.Status, view.Error = promptStatusError, err.Error()
		ok(c, view)
		return
	}
	view.RenderedText = sys.Text
	view.CharCount = len(sys.Text)
	view.ApproxTokens = len(sys.Text) / 4
	ok(c, view)
}

func templateIDOf(kb *aiprompt.KB) string {
	if kb == nil || kb.PromptTemplate == nil {
		return ""
	}
	return kb.PromptTemplate.ID
}

// unconfiguredPreviewKB is the empty KB that carries the org's active template
// for the not-configured preview: no rows, just the instructions.
func (s *Server) unconfiguredPreviewKB(c *gin.Context, orgID uuid.UUID) (*aiprompt.KB, error) {
	tpl := aiprompt.DefaultPromptTemplate(aiprompt.DefaultTemplateID)
	if s.kb != nil {
		active, err := s.kb.ActivePromptTemplate(ctx(c), orgID)
		if err != nil {
			return nil, err
		}
		tpl = &active
	}
	return &aiprompt.KB{OrganizationID: orgID.String(), PromptTemplate: tpl}, nil
}
