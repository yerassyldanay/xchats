package worker

import (
	"context"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// RefreshExpiringInstagramTokens extends every Instagram account's
// long-lived token that expires within `before` and was last refreshed at
// least `minAge` ago (Meta requires a token be >=24h old before it can be
// refreshed again — ChannelCredentialsExpiringSoon's own doc comment).
// Unlike WhatsApp Cloud's business token (typically a non-expiring System
// User token) and unlike Telegram's bot token (never expires), Instagram
// Login's long-lived user token is the one Meta-channel credential in this
// codebase that silently stops working if nothing ever renews it — this is
// that renewal.
//
// A single account's refresh failure (a revoked token, a transient Graph
// API error) is recorded via SetChannelCredentialsRefreshError and does not
// block the rest of the batch — the stored token stays usable until it
// actually expires, so one bad account never holds back every other one's
// renewal.
func (w *Worker) RefreshExpiringInstagramTokens(ctx context.Context, before time.Duration, minAge time.Duration) (refreshed int, err error) {
	if w.MetaClient == nil {
		return 0, nil
	}
	metas, err := w.Store.ChannelCredentialsExpiringSoon(ctx, "instagram", time.Now().Add(before), minAge)
	if err != nil {
		return 0, err
	}
	for _, m := range metas {
		if refreshErr := w.refreshOneInstagramToken(ctx, m); refreshErr != nil {
			w.Log.Warn("instagram token refresh failed", "account_id", m.AccountID, "err", refreshErr)
			_ = w.Store.SetChannelCredentialsRefreshError(ctx, m.AccountID, refreshErr.Error())
			// A due-and-failing refresh is the one case connection_state=
			// token_expiring is actually meant to surface — the batch is
			// only re-attempted every 6h, so an operator needs to see this
			// rather than the account silently going dark once the token
			// finally does expire.
			_ = w.Store.SetChannelAccountState(ctx, m.AccountID, "token_expiring")
			continue
		}
		// A refresh that just succeeded resolves any earlier token_expiring
		// warning — harmless (and correctly a no-op) for an account that was
		// never marked at all.
		_ = w.Store.SetChannelAccountState(ctx, m.AccountID, "connected")
		refreshed++
	}
	if refreshed > 0 {
		w.Log.Info("instagram token refresh", "refreshed", refreshed, "candidates", len(metas))
	}
	return refreshed, nil
}

func (w *Worker) refreshOneInstagramToken(ctx context.Context, m store.ChannelCredentialsMeta) error {
	token, err := w.Store.ChannelCredentialsSecret(ctx, m.AccountID)
	if err != nil {
		return err
	}
	res, err := w.MetaClient.RefreshInstagramLongLived(ctx, token)
	if err != nil {
		return err
	}
	var expiresAt *time.Time
	if res.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(res.ExpiresIn) * time.Second)
		expiresAt = &t
	}
	return w.Store.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
		AccountID: m.AccountID, Secret: res.AccessToken, TokenKind: m.TokenKind, ExpiresAt: expiresAt,
	})
}

// StartInstagramTokenRefresher runs RefreshExpiringInstagramTokens once at
// startup (catching anything that fell due while the process was down) and
// then on a ticker until ctx is done — mirrors StartMediaSweeper's
// identical shape.
func (w *Worker) StartInstagramTokenRefresher(ctx context.Context, every, before, minAge time.Duration) {
	go func() {
		if _, err := w.RefreshExpiringInstagramTokens(ctx, before, minAge); err != nil {
			w.Log.Error("instagram token refresh (startup)", "err", err)
		}
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := w.RefreshExpiringInstagramTokens(ctx, before, minAge); err != nil {
					w.Log.Error("instagram token refresh", "err", err)
				}
			}
		}
	}()
}
