package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// staleAfter is how long an unconnected Evolution instance may linger in the
// maintenance view before it's flagged stale (and offered for cleanup).
const staleAfter = 7 * 24 * time.Hour

// --- accounts (add via QR · reconnect · clean) ----------------------------

// handleListWhatsAppAccounts returns the org's connected numbers, each with a
// live status badge. The status is refreshed from a single FetchInstances; if
// Evolution is unreachable we fall back to the stored state (the page still loads).
func (s *Server) handleListWhatsAppAccounts(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	accts, err := s.store.ListAccountsForOrg(ctx(c), org.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	live := s.liveStatusByInstance(c)
	items := make([]dto.WhatsAppAccount, 0, len(accts))
	for _, a := range accts {
		items = append(items, dto.MapAccount(a, live[a.InstanceName]))
	}
	ok(c, gin.H{"items": items})
}

type createAccountReq struct {
	DisplayName  string `json:"display_name"`
	InstanceName string `json:"instance_name"`
}

// handleCreateWhatsAppAccount provisions a new Evolution instance and points its
// webhook at our edge, then reports qr_required. No wa_accounts row is written
// yet — the row's id is uuidv5(owner_jid), known only once the phone scans (the
// QR poll finalizes it). The pre-connect handle is therefore the instance name.
func (s *Server) handleCreateWhatsAppAccount(c *gin.Context) {
	if s.evo == nil {
		fail(c, http.StatusBadGateway, ErrEvolution, "evolution not configured")
		return
	}
	var req createAccountReq
	_ = c.ShouldBindJSON(&req)
	name := strings.TrimSpace(req.InstanceName)
	if name == "" || strings.ContainsAny(name, " /?#") {
		fail(c, http.StatusBadRequest, ErrValidation, "instance_name required (letters, digits, '-', '_')")
		return
	}
	if err := s.evo.CreateInstance(ctx(c), name, req.DisplayName); err != nil {
		if isAlreadyExists(err) {
			fail(c, http.StatusConflict, ErrConflict, "instance name already exists")
			return
		}
		fail(c, http.StatusBadGateway, ErrEvolution, err.Error())
		return
	}
	// Register our webhook on the new instance (UPPER_SNAKE events for v2.3.7).
	if err := s.evo.SetWebhook(ctx(c), name, s.webhookURLFor(name), webhookTokenHeader, s.cfg.WebhookToken, evolution.WebhookEvents); err != nil {
		s.log.Warn("set webhook on new instance failed", "instance", name, "err", err)
	}
	s.setPendingName(name, strings.TrimSpace(req.DisplayName))
	created(c, gin.H{
		"instance_name":     name,
		"display_name":      req.DisplayName,
		"connection_status": "qr_required",
	})
}

// handleWhatsAppAccountQR is the poll endpoint. While not connected it returns the
// newest QR straight from Evolution (never stored). On "open" it learns owner_jid
// via FetchInstances and writes/revives the wa_accounts row (id = uuidv5(owner_jid)),
// joining it to the caller's org → connected.
func (s *Server) handleWhatsAppAccountQR(c *gin.Context) {
	if s.evo == nil {
		fail(c, http.StatusBadGateway, ErrEvolution, "evolution not configured")
		return
	}
	instance := strings.TrimSpace(c.Query("instance"))
	if instance == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "instance query param required")
		return
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	state, err := s.evo.ConnectionState(ctx(c), instance)
	if err != nil {
		fail(c, http.StatusBadGateway, ErrEvolution, err.Error())
		return
	}
	if state == "open" {
		acct, ferr := s.finalizeConnect(c, instance, org)
		if ferr != nil {
			fail(c, http.StatusBadGateway, ErrEvolution, ferr.Error())
			return
		}
		if acct == nil { // open but owner_jid not yet reported — keep polling
			ok(c, gin.H{"status": "connecting", "qr_code": nil, "pairing_code": nil})
			return
		}
		ok(c, gin.H{"status": "connected", "wa_account_id": acct.ID.String()})
		return
	}
	qr, err := s.evo.ConnectQR(ctx(c), instance)
	if err != nil {
		fail(c, http.StatusBadGateway, ErrEvolution, err.Error())
		return
	}
	// Not open yet (connecting/close/unknown) → show the freshly fetched QR.
	ok(c, gin.H{
		"status":       "qr_required",
		"qr_code":      qr.Code,
		"qr_base64":    qr.Base64,
		"pairing_code": qr.PairingCode,
	})
}

// handleReconnectWhatsAppAccount re-issues a QR for an existing account whose
// session broke. Identity is the number, so the same account row is reused once
// re-scanned (history intact). If the instance was deleted on Evolution we
// recreate it under the same name first.
func (s *Server) handleReconnectWhatsAppAccount(c *gin.Context) {
	if s.evo == nil {
		fail(c, http.StatusBadGateway, ErrEvolution, "evolution not configured")
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
	if st, err := s.evo.ConnectionState(ctx(c), acct.InstanceName); err == nil && st == "open" {
		ok(c, gin.H{"status": "connected", "wa_account_id": acct.ID.String(), "instance_name": acct.InstanceName})
		return
	}
	qr, err := s.evo.ConnectQR(ctx(c), acct.InstanceName)
	if err != nil {
		// The instance was likely deleted on Evolution — recreate it under the
		// same name and re-issue the QR.
		if cerr := s.evo.CreateInstance(ctx(c), acct.InstanceName, acct.DisplayName); cerr != nil {
			fail(c, http.StatusBadGateway, ErrEvolution, cerr.Error())
			return
		}
		_ = s.evo.SetWebhook(ctx(c), acct.InstanceName, s.webhookURLFor(acct.InstanceName), webhookTokenHeader, s.cfg.WebhookToken, evolution.WebhookEvents)
		qr, err = s.evo.ConnectQR(ctx(c), acct.InstanceName)
		if err != nil {
			fail(c, http.StatusBadGateway, ErrEvolution, err.Error())
			return
		}
	}
	_ = s.store.SetAccountState(ctx(c), acct.ID, "qr_required")
	ok(c, gin.H{
		"status":        "qr_required",
		"instance_name": acct.InstanceName,
		"qr_code":       qr.Code,
		"qr_base64":     qr.Base64,
		"pairing_code":  qr.PairingCode,
	})
}

// handleDeleteWhatsAppAccount is a "clean": log out + delete the Evolution
// instance and soft-delete our row. History is kept (chats hidden); re-adding the
// number revives the row with its history.
func (s *Server) handleDeleteWhatsAppAccount(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAccount(c, id)
	if !okAcct {
		return
	}
	if s.evo != nil && acct.InstanceName != "" {
		if err := s.evo.LogoutInstance(ctx(c), acct.InstanceName); err != nil {
			s.log.Warn("logout instance failed", "instance", acct.InstanceName, "err", err)
		}
		if err := s.evo.DeleteInstance(ctx(c), acct.InstanceName); err != nil {
			s.log.Warn("delete instance failed", "instance", acct.InstanceName, "err", err)
		}
	}
	if err := s.store.SoftDeleteAccount(ctx(c), acct.ID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"id": acct.ID.String(), "deleted": true})
}

// --- instances maintenance ------------------------------------------------

// handleListInstances lists every raw Evolution instance with a managed flag (we
// hold a live account row for it) and a stale flag (not connected and older than
// staleAfter). This is the broom for orphaned/abandoned instances.
func (s *Server) handleListInstances(c *gin.Context) {
	if s.evo == nil {
		fail(c, http.StatusBadGateway, ErrEvolution, "evolution not configured")
		return
	}
	insts, err := s.evo.FetchInstances(ctx(c))
	if err != nil {
		fail(c, http.StatusBadGateway, ErrEvolution, err.Error())
		return
	}
	managed, err := s.store.ManagedInstanceNames(ctx(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	now := time.Now().UTC()
	items := make([]gin.H, 0, len(insts))
	for _, in := range insts {
		var created *string
		if !in.CreatedAt.IsZero() {
			t := in.CreatedAt.UTC().Format(time.RFC3339)
			created = &t
		}
		items = append(items, gin.H{
			"name":              in.Name,
			"connection_status": mapConnState(in.ConnectionStatus),
			"owner_jid":         in.OwnerJID,
			"phone_number":      in.PhoneNumber,
			"created_at":        created,
			"managed":           managed[in.Name],
			"stale":             isStale(in, now),
		})
	}
	ok(c, gin.H{"items": items})
}

// handleDeleteInstance removes a stray Evolution instance. It refuses managed
// instances (delete the account instead, which keeps its history).
func (s *Server) handleDeleteInstance(c *gin.Context) {
	if s.evo == nil {
		fail(c, http.StatusBadGateway, ErrEvolution, "evolution not configured")
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "instance name required")
		return
	}
	managed, err := s.store.ManagedInstanceNames(ctx(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if managed[name] {
		fail(c, http.StatusConflict, ErrConflict, "instance is managed — delete the account instead")
		return
	}
	_ = s.evo.LogoutInstance(ctx(c), name)
	if err := s.evo.DeleteInstance(ctx(c), name); err != nil {
		fail(c, http.StatusBadGateway, ErrEvolution, err.Error())
		return
	}
	ok(c, gin.H{"name": name, "deleted": true})
}

// --- helpers --------------------------------------------------------------

// finalizeConnect learns owner_jid for a just-opened instance and writes/revives
// the account row. Returns (nil, nil) if owner_jid isn't reported yet (keep polling).
func (s *Server) finalizeConnect(c *gin.Context, instance string, org store.Organization) (*store.Account, error) {
	insts, err := s.evo.FetchInstances(ctx(c))
	if err != nil {
		return nil, err
	}
	var found *evolution.Instance
	for i := range insts {
		if insts[i].Name == instance {
			found = &insts[i]
			break
		}
	}
	if found == nil || found.OwnerJID == "" {
		return nil, nil
	}
	ownerJID := config.CanonicalJID(found.OwnerJID)
	acct, err := s.store.UpsertConnectedAccount(ctx(c), store.Account{
		ID:              config.AccountID(ownerJID),
		OrganizationID:  uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName:     s.takePendingName(instance),
		OwnerJID:        ownerJID,
		PhoneNumber:     config.PhoneFromJID(ownerJID),
		InstanceName:    instance,
		InstanceID:      found.ID,
		ConnectionState: "connected",
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("account connected", "id", acct.ID, "owner_jid", acct.OwnerJID, "instance", instance)
	s.hub.Broadcast("wa_account.status_changed", dto.MapAccount(acct, "connected"))
	return &acct, nil
}

// liveStatusByInstance returns each instance's live status keyed by name, or an
// empty map if Evolution is unreachable (callers fall back to the stored state).
func (s *Server) liveStatusByInstance(c *gin.Context) map[string]string {
	out := map[string]string{}
	if s.evo == nil {
		return out
	}
	insts, err := s.evo.FetchInstances(ctx(c))
	if err != nil {
		s.log.Warn("fetchInstances for live status failed", "err", err)
		return out
	}
	for _, in := range insts {
		out[in.Name] = mapConnState(in.ConnectionStatus)
	}
	return out
}

// orgAccount resolves an account id and enforces org ownership (404 otherwise).
func (s *Server) orgAccount(c *gin.Context, id uuid.UUID) (store.Account, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.Account{}, false
	}
	acct, err := s.store.AccountByID(ctx(c), id)
	if err != nil || !acct.OrganizationID.Valid || acct.OrganizationID.UUID != org.ID {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return store.Account{}, false
	}
	return acct, true
}

func (s *Server) webhookURLFor(instance string) string {
	// The webhook handler resolves the account from the payload owner_jid, so the
	// path segment is opaque; the instance name keeps it human-debuggable.
	return strings.TrimRight(s.cfg.WebhookPublicBaseURL, "/") + "/evolution/api/v1/webhook/" + instance
}

func (s *Server) setPendingName(instance, displayName string) {
	s.pendingMu.Lock()
	s.pendingNames[instance] = displayName
	s.pendingMu.Unlock()
}

// takePendingName returns and clears the display name stashed at "add" time.
func (s *Server) takePendingName(instance string) string {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	name := s.pendingNames[instance]
	delete(s.pendingNames, instance)
	return name
}

// isStale flags an instance that never (or no longer) connected and has lingered
// past the cutoff. Unknown createdAt is treated as not-stale (don't sweep blindly).
func isStale(in evolution.Instance, now time.Time) bool {
	if mapConnState(in.ConnectionStatus) == "connected" {
		return false
	}
	if in.CreatedAt.IsZero() {
		return false
	}
	return in.CreatedAt.Before(now.Add(-staleAfter))
}

// mapConnState normalizes Evolution's raw state to our connection_state vocabulary.
func mapConnState(evoStatus string) string {
	switch evoStatus {
	case "open":
		return "connected"
	case "connecting":
		return "connecting"
	case "close", "closed":
		return "disconnected"
	case "":
		return ""
	default:
		return evoStatus
	}
}

func isAlreadyExists(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already") || strings.Contains(msg, "exists") || strings.Contains(msg, "in use")
}
