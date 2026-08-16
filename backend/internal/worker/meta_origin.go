package worker

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
)

// RepairStaleMetaOrigins compares every connected Meta channel account's
// registered webhook origin (channel_accounts.webhook_url, stamped with
// whatever URL was actually pushed to Meta at connect time) against
// currentPublicBaseURL — config.MetaResolvedPublicBaseURL(), resolved once
// at boot after the tunnel/public origin has settled. That single boot-time
// pass is deliberately the ONLY trigger (no periodic re-check, no live
// tunnel.Deps.OnStatus hook — the latter is explicitly out of scope): this
// codebase's embedded ngrok integration always resolves a single RESERVED
// domain (see ngrok_runtime.go's own doc comment), which is stable for a
// process's entire lifetime once resolved, so "at boot and on every tunnel
// start" collapses to "once per process boot" here.
//
// WhatsApp Cloud self-heals: its per-WABA override is set BY API, so a stale
// account is silently re-pushed with the new URL — no operator action at
// all. Instagram and Messenger cannot: their webhook is app-level, requiring
// a Meta Dashboard edit, so a stale account is marked connection_state=
// webhook_stale with webhook_last_error explaining exactly what changed —
// surfaced by GET /channel-setup's stale_accounts list and by the
// account's own card (MapNeutralAccount already passes connection_state and
// webhook_last_error through verbatim; hasWebhookHealth's allowlist already
// covers all three Meta channels, fixed in Phase 0).
//
// verifyToken is meta.DeriveVerifyToken(cfg.SessionSecret) — WA Cloud's
// SetOverride call needs it, mirroring registerWhatsAppCloudWebhook's own
// shape (internal/httpapi/whatsapp_cloud_accounts.go). A blank
// currentPublicBaseURL (no public HTTPS origin configured yet) is a no-op:
// there is nothing to compare against.
func (w *Worker) RepairStaleMetaOrigins(ctx context.Context, currentPublicBaseURL, verifyToken string) (repaired, stale int, err error) {
	currentOrigin := originOf(currentPublicBaseURL)
	if currentOrigin == "" {
		return 0, 0, nil
	}

	n, waErr := w.repairWhatsAppCloudOrigins(ctx, currentPublicBaseURL, currentOrigin, verifyToken)
	if waErr != nil {
		w.Log.Error("whatsapp cloud origin repair", "err", waErr)
	}
	repaired += n

	for _, ch := range []struct{ channel, path string }{
		{"instagram", "/meta/api/v1/webhook/instagram"},
		{"messenger", "/meta/api/v1/webhook/messenger"},
	} {
		n, chErr := w.markStaleMessengerish(ctx, ch.channel, currentPublicBaseURL+ch.path, currentOrigin)
		if chErr != nil {
			w.Log.Error("meta origin staleness check", "channel", ch.channel, "err", chErr)
			continue
		}
		stale += n
	}
	return repaired, stale, nil
}

// repairWhatsAppCloudOrigins is the self-healing half — see
// RepairStaleMetaOrigins' own doc comment.
func (w *Worker) repairWhatsAppCloudOrigins(ctx context.Context, currentPublicBaseURL, currentOrigin, verifyToken string) (int, error) {
	if w.WACloud == nil {
		return 0, nil
	}
	accts, err := w.Store.ChannelAccountsWithWebhook(ctx, "whatsapp_cloud")
	if err != nil {
		return 0, err
	}
	repaired := 0
	for _, a := range accts {
		if originOf(a.WebhookURL) == currentOrigin {
			continue
		}
		wabaID := wabaIDFromProviderMeta(a.ProviderMeta)
		if wabaID == "" {
			continue
		}
		token, err := w.Store.ChannelCredentialsSecret(ctx, a.ID)
		if err != nil {
			w.Log.Warn("whatsapp cloud origin repair: no credentials", "account_id", a.ID, "err", err)
			continue
		}
		newCallback := currentPublicBaseURL + "/meta/api/v1/webhook/whatsapp"
		if err := w.WACloud.SetOverride(ctx, wabaID, whatsappcloud.SubscribedAppsOverride{
			CallbackURI: newCallback, VerifyToken: verifyToken,
		}, token); err != nil {
			w.Log.Warn("whatsapp cloud origin repair: SetOverride failed", "account_id", a.ID, "err", err)
			_ = w.Store.SetChannelWebhookState(ctx, a.ID, store.ChannelWebhookState{
				State:     "webhook_error",
				LastError: "публичный адрес изменился, автоматическое обновление webhook не удалось: " + err.Error(),
				Checked:   true,
			})
			continue
		}
		_ = w.Store.SetChannelWebhookState(ctx, a.ID, store.ChannelWebhookState{
			State: "connected", URL: newCallback, Registered: true, Checked: true,
		})
		w.Log.Info("whatsapp cloud webhook origin repaired", "account_id", a.ID)
		repaired++
	}
	return repaired, nil
}

// markStaleMessengerish is the mark-and-surface half for Instagram/Messenger
// — see RepairStaleMetaOrigins' own doc comment. It re-marks an
// already-stale account too (not just newly-transitioned ones): the count
// this returns feeds a log line and GET /channel-setup's list, both of
// which want "how many need attention right now", not "how many changed
// this boot".
func (w *Worker) markStaleMessengerish(ctx context.Context, channel, newCallbackURL, currentOrigin string) (int, error) {
	accts, err := w.Store.ChannelAccountsWithWebhook(ctx, channel)
	if err != nil {
		return 0, err
	}
	stale := 0
	for _, a := range accts {
		if originOf(a.WebhookURL) == currentOrigin {
			continue
		}
		stale++
		if a.ConnectionState == "webhook_stale" {
			continue // already marked on an earlier boot
		}
		_ = w.Store.SetChannelWebhookState(ctx, a.ID, store.ChannelWebhookState{
			State:     "webhook_stale",
			LastError: "публичный адрес изменился — обновите webhook в Meta Dashboard на " + newCallbackURL,
			Checked:   true,
		})
		w.Log.Warn("meta account webhook stale", "account_id", a.ID, "channel", channel)
	}
	return stale, nil
}

// originOf returns rawURL's scheme+host ("https://xchats.example.com"),
// or "" for anything unparseable/incomplete — deliberately fails closed
// (never a false "match") so a malformed stored URL is treated as stale
// rather than silently skipped.
func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// wabaIDFromProviderMeta reads a whatsapp_cloud account's owning WABA id
// back out of provider_meta — the worker-package twin of httpapi's
// whatsAppCloudWABAID (internal/httpapi/whatsapp_cloud_accounts.go), kept as
// its own tiny copy rather than a shared export: it is three lines decoding
// one JSON key, and worker has no other reason to import httpapi (nor could
// it — httpapi imports worker).
func wabaIDFromProviderMeta(raw []byte) string {
	var m struct {
		WABAID string `json:"waba_id"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	return m.WABAID
}
