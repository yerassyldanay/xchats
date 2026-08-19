package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// Channel setup entry keys. public_access/meta_app are one-time,
// install-wide prerequisites; instagram/messenger/whatsapp_cloud are the
// three official Meta channels that depend on them. A caller (see
// nextRequiredSetup on the frontend) walks this exact order.
const (
	SetupKeyPublicAccess  = "public_access"
	SetupKeyMetaApp       = "meta_app"
	SetupKeyInstagram     = "instagram"
	SetupKeyMessenger     = "messenger"
	SetupKeyWhatsAppCloud = "whatsapp_cloud"
)

// Entry statuses. "connected" is deliberately never one of these — that
// stays reserved for an individual ACCOUNT's own connection state
// (connStatus in frontend/src/lib/format.ts); these five entries describe
// INSTALLATION prerequisites, a different axis entirely.
const (
	setupStatusReady         = "ready"
	setupStatusNotConfigured = "not_configured"
	setupStatusSetupRequired = "setup_required"
)

// MetaDashboardField names the literal Dashboard box one copy-paste value
// belongs in (App Settings → Basic → App Domains, and so on) — otherwise
// that mapping lives nowhere but a runbook.
type MetaDashboardField struct {
	Field string `json:"field"`
	Where string `json:"where"`
	Value string `json:"value"`
}

// ChannelSetupEntry is one prerequisite's status. Key/Status go to every
// caller. WebhookCallback..DashboardFields are filled in ONLY when the
// response is being built for an admin (see buildChannelSetupInfo), and
// even then only on the three Meta CHANNEL entries — public_access/meta_app
// own no Dashboard checklist of their own. omitempty is safe here
// specifically because none of these is ever legitimately empty for an
// admin's channel entry except RedirectURI/Scopes (whatsapp_cloud's real,
// no-OAuth-redirect value), so "field absent" and "not an admin's channel
// entry" coincide exactly — a member response never carries any of them.
type ChannelSetupEntry struct {
	Key    string `json:"key"`
	Status string `json:"status"`

	WebhookCallback string               `json:"webhook_callback,omitempty"`
	RedirectURI     string               `json:"redirect_uri,omitempty"`
	Scopes          []string             `json:"scopes,omitempty"`
	SubscribeFields string               `json:"subscribe_fields,omitempty"`
	DashboardPath   string               `json:"dashboard_path,omitempty"`
	DashboardFields []MetaDashboardField `json:"dashboard_fields,omitempty"`
}

// MetaStaleAccount is one connected Meta account whose registered webhook
// origin no longer matches the current public base URL — Instagram/Messenger
// only (WhatsApp Cloud self-heals via SetOverride and never sits in this
// state, see worker.RepairStaleMetaOrigins).
type MetaStaleAccount struct {
	AccountID   string `json:"account_id"`
	Channel     string `json:"channel"`
	DisplayName string `json:"display_name"`
	Detail      string `json:"detail"`
}

// ChannelSetupInfo is GET /channel-setup's response, and both credential-
// saving endpoints' response after a save. CanConfigure/PublicBaseURL/
// Entries[].Key/Status go to every authenticated caller — PublicBaseURL is
// by definition a public address, never a secret, and knowing WHAT is
// missing is what lets a member clicking "Add channel" understand an
// otherwise opaque failure. Everything from VerifyToken down is admin-only:
// the copy-paste payload an operator pastes into Meta's own Dashboard.
// StaleAccounts is a pointer so an admin with zero stale accounts still
// gets a real `[]` (never `null` — see metaStaleAccounts' own doc comment),
// while a member never gets the key at all; a plain slice's `omitempty`
// cannot make that distinction, since it hides an empty slice regardless of
// who is asking.
type ChannelSetupInfo struct {
	CanConfigure  bool                `json:"can_configure"`
	PublicBaseURL string              `json:"public_base_url"`
	Entries       []ChannelSetupEntry `json:"entries"`

	VerifyToken     string              `json:"verify_token,omitempty"`
	GraphAPIVersion string              `json:"graph_api_version,omitempty"`
	DashboardURL    string              `json:"dashboard_url,omitempty"`
	StaleAccounts   *[]MetaStaleAccount `json:"stale_accounts,omitempty"`
}

// callerIsAdmin reports whether the current request's caller holds the
// admin role in their active organization. Unlike RequireAdmin this is
// never a gate — a resolution failure just downgrades to the member
// (statuses-only) payload — since GET /channel-setup is deliberately open
// to any authenticated member.
func (s *Server) callerIsAdmin(c *gin.Context) bool {
	org, err := s.resolveOrg(c, currentUser(c).ID, currentSessionID(c))
	if err != nil {
		return false
	}
	role, err := s.store.MembershipRole(ctx(c), org.ID, currentUser(c).ID)
	return err == nil && role == "admin"
}

// handleChannelSetup answers GET /channel-setup — see ChannelSetupInfo's own
// doc comment for the member/admin payload split. No RequireAdmin: this is
// precisely how a member picking a channel in "Add channel" learns WHAT
// prerequisite is missing, even though only an admin can act on it.
func (s *Server) handleChannelSetup(c *gin.Context) {
	ok(c, s.buildChannelSetupInfo(c, s.callerIsAdmin(c)))
}

// buildChannelSetupInfo computes every prerequisite's live status and, for
// an admin caller, the full Dashboard copy-paste payload. Shared by GET
// /channel-setup and both credential-saving endpoints below, which answer
// with this SAME refreshed shape rather than echoing what was just saved
// (the credential value itself is never echoed, masked or otherwise).
func (s *Server) buildChannelSetupInfo(c *gin.Context, admin bool) ChannelSetupInfo {
	base := s.cfg.MetaResolvedPublicBaseURL()
	publicReady := base != ""
	_, metaReady := s.metaCreds.MetaAppCredentials(ctx(c))
	_, instagramReady := s.metaCreds.InstagramAppCredentials(ctx(c))
	metaChannelsReady := publicReady && metaReady

	info := ChannelSetupInfo{
		CanConfigure:  admin,
		PublicBaseURL: base,
		Entries: []ChannelSetupEntry{
			{Key: SetupKeyPublicAccess, Status: readyOr(publicReady, setupStatusNotConfigured)},
			{Key: SetupKeyMetaApp, Status: readyOr(metaReady, setupStatusNotConfigured)},
			s.channelSetupEntry(SetupKeyInstagram, metaChannelsReady && instagramReady, admin, base),
			s.channelSetupEntry(SetupKeyMessenger, metaChannelsReady, admin, base),
			s.channelSetupEntry(SetupKeyWhatsAppCloud, metaChannelsReady, admin, base),
		},
	}
	if !admin {
		return info
	}
	info.VerifyToken = meta.DeriveVerifyToken(s.cfg.SessionSecret)
	info.GraphAPIVersion = s.cfg.MetaResolvedGraphAPIVersion()
	if p, okProvider := credentials.ProviderByID("meta"); okProvider {
		info.DashboardURL = p.CredentialURL
	}
	stale := s.metaStaleAccounts(c)
	info.StaleAccounts = &stale
	return info
}

func readyOr(ready bool, otherwise string) string {
	if ready {
		return setupStatusReady
	}
	return otherwise
}

// channelSetupEntry builds one Meta channel's status entry. base is used
// even when empty (public access not yet configured): an admin still needs
// to see the SHAPE of every value about to exist, matching how the old
// meta-setup checklist this supersedes always computed its values from
// config alone — see its own former doc comment for that reasoning.
func (s *Server) channelSetupEntry(key string, ready, admin bool, base string) ChannelSetupEntry {
	e := ChannelSetupEntry{Key: key, Status: readyOr(ready, setupStatusSetupRequired)}
	if !admin {
		return e
	}
	switch key {
	case SetupKeyInstagram:
		redirect := base + instagramCallbackPath
		e.WebhookCallback = base + "/meta/api/v1/webhook/instagram"
		e.RedirectURI = redirect
		e.Scopes = []string{"instagram_business_basic", "instagram_business_manage_messages"}
		e.SubscribeFields = "messages"
		e.DashboardPath = "Meta App → Instagram → API setup with Instagram login → Business login settings + Webhooks"
		e.DashboardFields = []MetaDashboardField{
			{Field: "OAuth redirect URIs", Where: "Instagram → API setup with Instagram login → Business login settings", Value: redirect},
		}
	case SetupKeyMessenger:
		redirect := base + messengerCallbackPath
		e.WebhookCallback = base + "/meta/api/v1/webhook/messenger"
		e.RedirectURI = redirect
		e.Scopes = messengerScopes
		e.SubscribeFields = "messages"
		e.DashboardPath = "Meta App → Messenger → Webhooks + Page Subscriptions"
		e.DashboardFields = []MetaDashboardField{
			{Field: "Valid OAuth Redirect URIs", Where: "Facebook Login → Settings", Value: redirect},
			{Field: "App Domains", Where: "App Settings → Basic", Value: metaAppDomain(base)},
			{Field: "Site URL", Where: "App Settings → Basic → Website platform", Value: joinURL(base, "/")},
		}
	case SetupKeyWhatsAppCloud:
		callback := base + metaWhatsAppWebhookPath
		e.WebhookCallback = callback
		e.SubscribeFields = "messages"
		e.DashboardPath = "Meta App → WhatsApp → Configuration → Webhook"
		e.DashboardFields = []MetaDashboardField{
			{Field: "Callback URL", Where: "WhatsApp → Configuration → Webhook", Value: callback},
		}
	}
	return e
}

// metaAppDomain extracts the bare host from xchats' public base URL — the
// literal value Meta's "App Domains" Dashboard field wants. App Domains
// rejects a scheme or path; pasting the full URL there instead of the bare
// host is itself a common cause of "Домен этого URL не включен в список
// доменов приложения" (see this feature's own motivating bug report).
func metaAppDomain(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// joinURL appends suffix to base without producing a double slash — used
// only for the cosmetic "Site URL" Dashboard field value.
func joinURL(base, suffix string) string {
	return strings.TrimRight(base, "/") + suffix
}

// metaStaleAccounts collects every Instagram/Messenger account currently
// marked webhook_stale (see worker.RepairStaleMetaOrigins) — best-effort: a
// read failure here degrades the checklist to "no known stale accounts"
// rather than failing the whole page.
func (s *Server) metaStaleAccounts(c *gin.Context) []MetaStaleAccount {
	// Never nil: encoding/json renders a nil slice as `null`, and
	// ChannelSetupInfo.StaleAccounts is read unconditionally once present.
	out := []MetaStaleAccount{}
	for _, ch := range []messaging.Channel{messaging.ChannelInstagram, messaging.ChannelMessenger} {
		accts, err := s.store.ChannelAccountsWithWebhook(ctx(c), string(ch))
		if err != nil {
			continue
		}
		for _, a := range accts {
			if a.ConnectionState != "webhook_stale" {
				continue
			}
			out = append(out, MetaStaleAccount{
				AccountID: a.ID.String(), Channel: a.Channel, DisplayName: a.DisplayName, Detail: a.WebhookLastError,
			})
		}
	}
	return out
}

// --- credential-saving endpoints (admin-only) ------------------------------

// saveMetaAppReq is PUT /channel-setup/meta-app's body.
type saveMetaAppReq struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
	// Force saves an unverifiable pair anyway (the debug_token check itself
	// could not complete — network trouble, Meta down — never a confirmed
	// rejection; ErrInvalidCredential can never be forced past).
	Force bool `json:"force"`
}

// handleSaveMetaApp saves the operator's Meta Developer App id/secret,
// running the SAME validateMeta debug_token check (and the same
// valid/invalid/unverifiable three-way outcome and status codes) the
// Settings "meta" provider card already used — see
// handleSaveIntegrationCredential's own doc comment for the shared
// reasoning this mirrors. Responds with the refreshed status payload only;
// the credential value is never echoed, masked or otherwise.
func (s *Server) handleSaveMetaApp(c *gin.Context) {
	if s.credentials == nil {
		fail(c, http.StatusServiceUnavailable, ErrCredentialStore, "no credential store is available")
		return
	}
	var req saveMetaAppReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	appID, appSecret := strings.TrimSpace(req.AppID), strings.TrimSpace(req.AppSecret)
	if appID == "" || appSecret == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "app_id and app_secret обязательны")
		return
	}
	provider, okProvider := credentials.ProviderByID("meta")
	if !okProvider {
		fail(c, http.StatusInternalServerError, ErrInternal, "meta provider not registered")
		return
	}

	values := map[credentials.Key]string{"meta.app_id": appID, "meta.app_secret": appSecret}
	s.baseURLValueFor(provider, values)
	switch verifyErr := s.verifyProviderCredential(ctx(c), provider, values); {
	case verifyErr == nil:
	case errors.Is(verifyErr, credentials.ErrInvalidCredential):
		fail(c, http.StatusUnprocessableEntity, ErrCredentialInvalid, "credential was rejected by the provider")
		return
	case errors.Is(verifyErr, credentials.ErrValidationUnavailable):
		if !req.Force {
			fail(c, http.StatusConflict, ErrCredentialUnverified,
				"could not verify the credential; resubmit with force=true to save it anyway")
			return
		}
	default:
		fail(c, http.StatusInternalServerError, ErrInternal, verifyErr.Error())
		return
	}

	for _, f := range provider.Fields {
		if err := s.credentials.Set(ctx(c), f.Key, values[f.Key]); err != nil {
			fail(c, http.StatusInternalServerError, ErrCredentialStore, err.Error())
			return
		}
	}
	ok(c, s.buildChannelSetupInfo(c, true))
}

// saveInstagramAppReq is PUT /channel-setup/instagram-app's body.
type saveInstagramAppReq struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// handleSaveInstagramApp saves the operator's Instagram App id/secret — see
// credentials.KeyInstagramAppID's own doc comment for why this is a
// SEPARATE pair from meta-app above. Deliberately no `force` and no
// Validate hook: validateMeta's debug_token self-check is a
// graph.facebook.com app-access-token trick with no Instagram equivalent —
// proving an Instagram pair is real needs a real user-consented
// authorization code, which only the connect flow itself can produce.
// Responds with the refreshed status payload only.
func (s *Server) handleSaveInstagramApp(c *gin.Context) {
	if s.credentials == nil {
		fail(c, http.StatusServiceUnavailable, ErrCredentialStore, "no credential store is available")
		return
	}
	var req saveInstagramAppReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	appID, appSecret := strings.TrimSpace(req.AppID), strings.TrimSpace(req.AppSecret)
	if appID == "" || appSecret == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "app_id and app_secret обязательны")
		return
	}
	if err := s.credentials.Set(ctx(c), credentials.KeyInstagramAppID, appID); err != nil {
		fail(c, http.StatusInternalServerError, ErrCredentialStore, err.Error())
		return
	}
	if err := s.credentials.Set(ctx(c), credentials.KeyInstagramAppSecret, appSecret); err != nil {
		fail(c, http.StatusInternalServerError, ErrCredentialStore, err.Error())
		return
	}
	ok(c, s.buildChannelSetupInfo(c, true))
}
