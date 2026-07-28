package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// promptSectionCounts mirrors the "Сформирован из разделов" sidebar list on
// the Промпт tab (plan/ui/ui_knowledge_base_001.png) — how many rows of each
// kind fed the render.
type promptSectionCounts struct {
	Topics   int `json:"topics"`
	Products int `json:"products"`
	Tariffs  int `json:"tariffs"`
	Zones    int `json:"zones"`
	Contacts int `json:"contacts"`
	Policies int `json:"policies"`
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

	frame := aiprompt.FrameShopKBV4RU()
	view := promptView{PromptRef: aiprompt.PromptRefShopKBV4, FrameText: frame, BuiltAt: time.Now()}

	kb, err := s.kbRepo.Load(ctx(c), orgID.String())
	if err != nil {
		view.Status, view.Error = "error", err.Error()
		ok(c, view)
		return
	}
	view.SectionCounts = promptSectionCounts{
		Topics: len(kb.Topics), Products: len(kb.Products), Tariffs: len(kb.Tariffs), Zones: len(kb.DeliveryZones),
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
	rendered, err := aiprompt.RenderPrompt(frame, kb.PromptInput(), cat)
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
