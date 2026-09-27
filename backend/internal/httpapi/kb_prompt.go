package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// promptSectionCounts mirrors the "Сформирован из разделов" sidebar list on
// the Промпт tab (plan/ui/ui_knowledge_base_001.png) — how many rows of each
// kind fed the render.
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

// promptView is the GET /kb/prompt payload: everything the Промпт tab needs
// to show the exact prompt the response engine would build right now, plus
// enough metadata (size/tokens/status) to render its "О промпте" sidebar
// without a second round trip.
type promptView struct {
	PromptRef     string              `json:"prompt_ref"`
	RenderedText  string              `json:"rendered_text"`
	FrameText     string              `json:"frame_text"`
	CharCount     int                 `json:"char_count"`
	ApproxTokens  int                 `json:"approx_tokens"`
	BuiltAt       time.Time           `json:"built_at"`
	Status        string              `json:"status"` // ok | error
	Error         string              `json:"error,omitempty"`
	SectionCounts promptSectionCounts `json:"section_counts"`
}

// handleKBPrompt renders the exact prompt the response engine would send for
// this org right now, reading through s.kbRepo — the SAME CachedKBRepo the
// production reply path reads (server.go's doc comment on that field), so
// this can never be a second, possibly-divergent rendering path. A KB-load or
// BuildCatalog failure (KB not configured yet, or an invariant broken by a
// direct SQL edit bypassing every write-time check) is reported as
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

	// Non-Telegram channel, matching response.FrameFor's own default branch —
	// this tab shows what the engine would send right now on the WhatsApp/
	// simulator path. Before the KB loads, fall back to the shop-kb@v7
	// default (response.FrameFor(nil, ...)) purely to have SOMETHING to show
	// on a load error; the real, KB-driven selection happens right below.
	view := promptView{
		PromptRef: response.PromptRefFor(nil, messaging.ChannelWhatsApp),
		FrameText: response.FrameFor(nil, messaging.ChannelWhatsApp),
		BuiltAt:   time.Now(),
	}

	kb, err := s.kbRepo.Load(ctx(c), orgID.String())
	if err != nil {
		view.Status, view.Error = "error", err.Error()
		ok(c, view)
		return
	}
	frame := response.FrameFor(kb, messaging.ChannelWhatsApp)
	view.PromptRef = response.PromptRefFor(kb, messaging.ChannelWhatsApp)
	view.FrameText = frame
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

	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		view.Status, view.Error = "error", err.Error()
		ok(c, view)
		return
	}
	rendered, err := aiprompt.RenderPromptV7(frame, kb.PromptInput(), cat)
	if err != nil {
		view.Status, view.Error = "error", err.Error()
		ok(c, view)
		return
	}

	view.Status = "ok"
	view.RenderedText = rendered
	view.CharCount = len(rendered)
	view.ApproxTokens = len(rendered) / 4
	ok(c, view)
}
