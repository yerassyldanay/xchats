package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// simulatorMessageReq is the simulator API's only input shape. It deliberately
// carries no API key, base URL, raw prompt, or KB override — those stay
// entirely server-side configuration.
type simulatorMessageReq struct {
	ContactRef      string `json:"contact_ref"`
	ConversationRef string `json:"conversation_ref"`
	Text            string `json:"text"`
	WaitForResponse bool   `json:"wait_for_response"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
}

// handleSimulatorMessage injects one synthetic inbound message into the same
// ingestion/response path a real WhatsApp message takes. The organization is
// always the authenticated caller's own — never taken from the request body.
// Gated by SIMULATOR_ENABLED at the router level (see Router()).
func (s *Server) handleSimulatorMessage(c *gin.Context) {
	var req simulatorMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}
	req.ContactRef = strings.TrimSpace(req.ContactRef)
	req.ConversationRef = strings.TrimSpace(req.ConversationRef)
	req.Text = strings.TrimSpace(req.Text)
	if req.ContactRef == "" || req.ConversationRef == "" || req.Text == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "contact_ref, conversation_ref, and text are required")
		return
	}
	if (req.Provider == "") != (req.Model == "") {
		fail(c, http.StatusBadRequest, ErrValidation, "provider and model must both be set, or both omitted")
		return
	}

	// Provider/model override is accepted only for an already-registered
	// provider — validated up front so a typo gets an immediate 400 instead of
	// silently degrading to a holding draft.
	var override *llm.ModelRef
	if req.Provider != "" {
		ref := llm.ModelRef{Provider: req.Provider, Model: req.Model}
		if _, err := s.response.Engine.LLMs.Client(ref); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "unregistered provider: "+req.Provider)
			return
		}
		override = &ref
	}

	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}

	acct, err := s.store.GetOrCreateSimulatorAccount(ctx(c), org.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	// Shared ingestion: the same upsert path a real WhatsApp webhook uses.
	// contact_ref and conversation_ref are independent identities (PhoneJID vs.
	// RemoteJID) so one contact can hold multiple simulator conversations.
	res, err := s.store.UpsertInbound(ctx(c), store.InboundUpsert{
		AccountID:          acct.ID,
		PhoneJID:           req.ContactRef,
		RemoteJID:          req.ConversationRef,
		PhoneNumber:        req.ContactRef,
		PushName:           req.ContactRef,
		Direction:          "in",
		SenderKind:         "contact",
		EvolutionMessageID: uuid.NewString(),
		MessageKind:        "conversation",
		Body:               req.Text,
		Preview:            preview(req.Text),
		Source:             "simulator",
		MessageTS:          time.Now(),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.emitInbound(c, res)

	if !req.WaitForResponse {
		_ = s.queue.Publish(queue.Message{Kind: queue.KindAIDraft, Payload: worker.AIDraftTask{ChatID: res.ChatID}})
		accepted(c, gin.H{"conversation_id": res.ChatID.String(), "message_id": res.MessageID.String()})
		return
	}

	persisted, err := s.response.Respond(ctx(c), messaging.ChannelSimulator, res.ChatID.String(), response.RespondOptions{ModelOverride: override})
	if err != nil {
		fail(c, http.StatusBadGateway, ErrAIUnavailable, err.Error())
		return
	}
	s.emitDrafts(c, persisted)
	if len(persisted) == 0 {
		fail(c, http.StatusInternalServerError, ErrInternal, "no draft produced")
		return
	}
	d := persisted[0]
	ok(c, gin.H{
		"conversation_id": res.ChatID.String(),
		"message_id":      res.MessageID.String(),
		"draft": gin.H{
			"id": d.ID, "text": d.Text, "reply_language": d.ReplyLanguage, "escalate": d.Escalate,
		},
	})
}

// emitInbound broadcasts the same SSE events handleWaEvent emits for a real
// inbound message, so a simulator conversation shows up live in the inbox too.
func (s *Server) emitInbound(c *gin.Context, res store.InboundResult) {
	if !res.MessageInserted {
		return
	}
	if msg, err := s.store.MessageByID(ctx(c), res.MessageID); err == nil {
		s.hub.Broadcast("message.created", dto.MapMessage(msg))
	}
	if chat, err := s.store.ChatByID(ctx(c), res.ChatID); err == nil {
		name := "chat.updated"
		if res.ChatCreated {
			name = "chat.created"
		}
		s.hub.Broadcast(name, dto.MapChat(chat))
	}
}

// emitDrafts broadcasts ai_draft.created for each persisted draft, mirroring
// worker.handleAIDraft's own broadcast for the async path.
func (s *Server) emitDrafts(c *gin.Context, persisted []response.PersistedDraft) {
	for _, p := range persisted {
		id, err := uuid.Parse(p.ID)
		if err != nil {
			continue
		}
		d, err := s.store.DraftByID(ctx(c), id)
		if err != nil {
			continue
		}
		s.hub.Broadcast("ai_draft.created", dto.MapDraft(d))
	}
}
