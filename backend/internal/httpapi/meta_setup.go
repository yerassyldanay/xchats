package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// MetaChannelSetup is one channel's own slice of the setup checklist — every
// string an operator needs to paste into their Meta Developer App's
// Dashboard, built from the SAME config.MetaResolvedPublicBaseURL() the
// runtime itself registers against so the two can never drift apart. Redirect
// is empty for whatsapp_cloud: that channel connects via a manual WABA id +
// business token (whatsapp_cloud_accounts.go), never an OAuth redirect.
type MetaChannelSetup struct {
	Channel           string   `json:"channel"`
	WebhookCallback   string   `json:"webhook_callback"`
	RedirectURI       string   `json:"redirect_uri,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	SubscribeFields   string   `json:"subscribe_fields"`
	DashboardPath     string   `json:"dashboard_path"`
	DevelopmentNotice bool     `json:"development_notice"`
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

// MetaSetupInfo is GET /settings/meta-setup's full response.
type MetaSetupInfo struct {
	PublicBaseURL      string             `json:"public_base_url"`
	ProductionRequired bool               `json:"production_required"`
	VerifyToken        string             `json:"verify_token"`
	GraphAPIVersion    string             `json:"graph_api_version"`
	Channels           []MetaChannelSetup `json:"channels"`
	StaleAccounts      []MetaStaleAccount `json:"stale_accounts"`
}

// handleMetaSetup is the admin copy-paste checklist (§11's highest-leverage
// UI): every value is computable from config alone, independent of whether
// an operator has saved a Meta App ID/Secret yet — that is exactly the
// chicken-and-egg moment this exists to unblock, so unlike every other Meta
// handler this one never checks metaAppCredentials.
func (s *Server) handleMetaSetup(c *gin.Context) {
	base := s.cfg.MetaResolvedPublicBaseURL()
	info := MetaSetupInfo{
		PublicBaseURL:      base,
		ProductionRequired: s.cfg.IsProduction(),
		VerifyToken:        meta.DeriveVerifyToken(s.cfg.SessionSecret),
		GraphAPIVersion:    s.cfg.MetaResolvedGraphAPIVersion(),
		Channels: []MetaChannelSetup{
			{
				Channel:           string(messaging.ChannelInstagram),
				WebhookCallback:   base + "/meta/api/v1/webhook/instagram",
				RedirectURI:       base + instagramCallbackPath,
				Scopes:            []string{"instagram_business_basic", "instagram_business_manage_messages"},
				SubscribeFields:   "messages",
				DashboardPath:     "Meta App → Instagram → API setup with Instagram login → Business login settings + Webhooks",
				DevelopmentNotice: true,
			},
			{
				Channel:           string(messaging.ChannelMessenger),
				WebhookCallback:   base + "/meta/api/v1/webhook/messenger",
				RedirectURI:       base + messengerCallbackPath,
				Scopes:            messengerScopes,
				SubscribeFields:   "messages",
				DashboardPath:     "Meta App → Messenger → Webhooks + Page Subscriptions",
				DevelopmentNotice: true,
			},
			{
				Channel:         string(messaging.ChannelWhatsAppCloud),
				WebhookCallback: base + metaWhatsAppWebhookPath,
				Scopes:          []string{"whatsapp_business_messaging", "whatsapp_business_management"},
				SubscribeFields: "messages",
				DashboardPath:   "Meta App → WhatsApp → Configuration → Webhook",
			},
		},
		StaleAccounts: s.metaStaleAccounts(c),
	}
	ok(c, info)
}

// metaStaleAccounts collects every Instagram/Messenger account currently
// marked webhook_stale (see worker.RepairStaleMetaOrigins) — best-effort: a
// read failure here degrades the checklist to "no known stale accounts"
// rather than failing the whole page.
func (s *Server) metaStaleAccounts(c *gin.Context) []MetaStaleAccount {
	// Never nil: encoding/json renders a nil slice as `null`, and the
	// frontend checklist reads stale_accounts.length unconditionally.
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
