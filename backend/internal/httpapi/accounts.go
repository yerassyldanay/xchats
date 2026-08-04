package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// --- accounts (list · pair via QR · logout · clean) -----------------------

// handleListWhatsAppAccounts returns the org's connected numbers, each with
// its live status badge. connection_state is kept current in real time by
// internal/whatsmeow's own event handling (Connected/Disconnected/LoggedOut
// all write straight to the account row), so this is a plain read — no
// external probe to fall back from.
func (s *Server) handleListWhatsAppAccounts(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	accts, err := s.store.ListWaAccountsForOrg(ctx(c), org.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	items := make([]dto.WhatsAppAccount, 0, len(accts))
	for _, a := range accts {
		items = append(items, dto.MapAccount(a, ""))
	}
	ok(c, gin.H{"items": items})
}

// handleListAccounts is the channel-neutral account listing: every connected
// account the org owns, on every channel, in one shape. It is what the UI's
// «Каналы» page renders. /whatsapp-accounts stays alongside it, WhatsApp-only,
// because the pairing lifecycle it drives has no meaning for any other channel.
func (s *Server) handleListAccounts(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	accts, err := s.store.ListAccountsForOrg(ctx(c), org.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	items := make([]dto.Account, 0, len(accts))
	for _, a := range accts {
		items = append(items, dto.MapNeutralAccount(a, ""))
	}
	ok(c, gin.H{"items": items})
}

// handlePairWhatsAppAccount starts a new QR pairing flow for the caller's
// organization and returns a session id to poll for the QR code (and the
// eventual paired/timeout/error outcome) via handlePairStatus.
func (s *Server) handlePairWhatsAppAccount(c *gin.Context) {
	if s.wa == nil {
		fail(c, http.StatusBadGateway, ErrWhatsApp, "whatsapp is not configured")
		return
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	sessionID, err := s.wa.Pair(ctx(c), org.ID.String())
	if err != nil {
		fail(c, http.StatusBadGateway, ErrWhatsApp, err.Error())
		return
	}
	created(c, gin.H{"session_id": sessionID, "status": "qr_required"})
}

// handlePairStatus is the poll endpoint a pairing session's caller hits
// repeatedly until it reports "connected", "timeout", or "error".
func (s *Server) handlePairStatus(c *gin.Context) {
	if s.wa == nil {
		fail(c, http.StatusBadGateway, ErrWhatsApp, "whatsapp is not configured")
		return
	}
	sessionID := c.Param("session_id")
	update, found := s.wa.PairStatus(sessionID)
	if !found {
		fail(c, http.StatusNotFound, ErrNotFound, "pairing session not found or expired")
		return
	}
	ok(c, gin.H{
		"status":     update.Status,
		"qr_code":    update.QRCode,
		"qr_base64":  update.QRBase64,
		"account_id": update.AccountID,
		"message":    update.Message,
	})
}

// handleLogoutWhatsAppAccount disconnects the account, deletes its whatsmeow
// device session, and marks it logged out — the row stays visible (unlike
// handleDeleteWhatsAppAccount's "clean") so the same number can be re-paired
// without disappearing from the list first.
func (s *Server) handleLogoutWhatsAppAccount(c *gin.Context) {
	if s.wa == nil {
		fail(c, http.StatusBadGateway, ErrWhatsApp, "whatsapp is not configured")
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAccount(c, id)
	if !okAcct {
		return
	}
	if err := s.wa.Logout(ctx(c), acct.ID.String()); err != nil {
		fail(c, http.StatusBadGateway, ErrWhatsApp, err.Error())
		return
	}
	ok(c, gin.H{"id": acct.ID.String(), "logged_out": true})
}

// handleWhatsAppAccountStatus returns an account's current connection status.
func (s *Server) handleWhatsAppAccountStatus(c *gin.Context) {
	if s.wa == nil {
		fail(c, http.StatusBadGateway, ErrWhatsApp, "whatsapp is not configured")
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAccount(c, id)
	if !okAcct {
		return
	}
	status, err := s.wa.Status(ctx(c), acct.ID.String())
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"account_id": status.AccountID, "connection_state": status.State})
}

// handleDeleteWhatsAppAccount is a "clean": best-effort logout (disconnect +
// delete the whatsmeow device session) + soft-delete our row. History is
// kept (chats hidden); re-adding the number via a fresh pairing revives the
// row with its history intact (same derived id — see config.AccountID).
func (s *Server) handleDeleteWhatsAppAccount(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAccount(c, id)
	if !okAcct {
		return
	}
	if s.wa != nil {
		if err := s.wa.Logout(ctx(c), acct.ID.String()); err != nil {
			s.log.Warn("logout on delete failed", "account_id", acct.ID, "err", err)
		}
	}
	if err := s.store.SoftDeleteAccount(ctx(c), acct.ID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"id": acct.ID.String(), "deleted": true})
}

// --- debug (SIMULATOR_ENABLED only) ----------------------------------------

// debugWaEventReq is POST /debug/wa-event's request body — see
// whatsapp.DebugEvent's doc comment for the full contract this mirrors.
type debugWaEventReq struct {
	EventType   string `json:"event_type"`
	AccountID   string `json:"account_id"`
	SenderJID   string `json:"sender_jid"`
	Text        string `json:"text"`
	IsFromMe    bool   `json:"is_from_me"`
	ExternalID  string `json:"external_id"`
	ReceiptType string `json:"receipt_type"`
}

// handleDebugWaEvent injects a synthetic whatsmeow-shaped event into the
// exact same ingest path a real WhatsApp connection uses — see
// whatsapp.Manager.InjectDebugEvent's doc comment. It is gated by
// SIMULATOR_ENABLED at the router level, same as /simulator/messages.
//
// account_id is optional: when omitted, the caller's organization's
// simulator account is resolved (creating it if needed), so this endpoint
// works end to end without a real paired WhatsApp number.
func (s *Server) handleDebugWaEvent(c *gin.Context) {
	if s.wa == nil {
		fail(c, http.StatusBadGateway, ErrWhatsApp, "whatsapp is not configured")
		return
	}
	var req debugWaEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}
	if req.EventType == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "event_type is required")
		return
	}

	accountID := req.AccountID
	if accountID == "" {
		org, okOrg := s.orgOf(c)
		if !okOrg {
			return
		}
		acct, err := s.store.GetOrCreateSimulatorAccount(ctx(c), org.ID)
		if err != nil {
			fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
			return
		}
		accountID = acct.ID.String()
	}

	res, err := s.wa.InjectDebugEvent(ctx(c), whatsapp.DebugEvent{
		EventType: req.EventType, AccountID: accountID, SenderJID: req.SenderJID,
		Text: req.Text, IsFromMe: req.IsFromMe, ExternalID: req.ExternalID, ReceiptType: req.ReceiptType,
	})
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	resp := gin.H{"ok": true}
	if res.MessageID != "" {
		resp["message_id"] = res.MessageID
		resp["chat_id"] = res.ChatID
	}
	ok(c, resp)
}

// --- helpers --------------------------------------------------------------

// orgAccount resolves an account id and enforces org ownership (404 otherwise).
// AccountByID reads the cross-channel view, so this also refuses accounts
// that do not live on the wa_* gateway: /whatsapp-accounts/:id drives pairing,
// logout and status, none of which a Telegram bot has. Its own lifecycle is
// /telegram-accounts/:id.
func (s *Server) orgAccount(c *gin.Context, id uuid.UUID) (store.Account, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.Account{}, false
	}
	acct, err := s.store.AccountByID(ctx(c), id)
	if err != nil || !acct.OrganizationID.Valid || acct.OrganizationID.UUID != org.ID ||
		acct.Channel == string(messaging.ChannelTelegram) {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return store.Account{}, false
	}
	return acct, true
}
