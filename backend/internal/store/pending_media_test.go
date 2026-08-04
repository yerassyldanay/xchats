package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// TestPendingTelegramMediaFiltersByAge pins the one call site that needed a
// real logic change, not just a mechanical SQL edit: the old
// "now() - $1::interval" predicate becomes a Go-computed cutoff
// (time.Now().Add(-olderThan)) bound directly, since SQLite has no INTERVAL
// type. A row updated recently must NOT be returned; one quiet for longer
// than olderThan must be.
func TestPendingTelegramMediaFiltersByAge(t *testing.T) {
	// dbtest.Open (not New) so the backdate write below shares st's own
	// connection (see internal/dbx.Open's path-based sharing) instead of
	// talking to an unrelated database.
	st, db := dbtest.Open(t)
	ctx := context.Background()

	box, err := secretbox.FromEnvValue(viewsEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)

	org, err := st.SeedOrganization(ctx, "pending-media-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	const botID = int64(9001)
	tgAccount, err := st.ClaimTelegramAccount(ctx, store.TelegramClaim{
		ID:             config.ChannelAccountID(config.TelegramOwnerRef(botID)),
		OrganizationID: org.ID,
		BotID:          botID,
		BotUsername:    "pending_media_bot",
		BotToken:       "9001:token",
	})
	if err != nil {
		t.Fatalf("claim telegram account: %v", err)
	}

	// A fresh (just-ingested) pending attachment.
	fresh, err := st.IngestTelegramInbound(ctx, store.TgInbound{
		AccountID: tgAccount.ID, UpdateID: 1,
		TelegramChatID: 1, TelegramUserID: 1, TelegramMessageID: 1,
		MessageKind: "photo", Body: "", MessageTS: time.Now(),
		Media: &store.TgMedia{FileID: "file-fresh", MediaType: "photo"},
	})
	if err != nil {
		t.Fatalf("ingest fresh: %v", err)
	}

	// A second attachment, backdated well past the age threshold by writing
	// updated_at directly (the ingest path always stamps "now").
	stale, err := st.IngestTelegramInbound(ctx, store.TgInbound{
		AccountID: tgAccount.ID, UpdateID: 2,
		TelegramChatID: 1, TelegramUserID: 1, TelegramMessageID: 2,
		MessageKind: "photo", Body: "", MessageTS: time.Now(),
		Media: &store.TgMedia{FileID: "file-stale", MediaType: "photo"},
	})
	if err != nil {
		t.Fatalf("ingest stale: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE tg_message_media SET updated_at = $2 WHERE message_id = $1`,
		stale.MessageID, time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatalf("backdate media: %v", err)
	}

	pending, err := st.PendingTelegramMedia(ctx, time.Hour, 100)
	if err != nil {
		t.Fatalf("PendingTelegramMedia: %v", err)
	}

	var sawStale, sawFresh bool
	for _, p := range pending {
		switch p.MessageID {
		case stale.MessageID:
			sawStale = true
		case fresh.MessageID:
			sawFresh = true
		}
	}
	if !sawStale {
		t.Error("a media row quiet for 2h with a 1h threshold was not returned")
	}
	if sawFresh {
		t.Error("a media row updated moments ago with a 1h threshold was returned")
	}

	// Tightening the threshold beyond the stale row's actual age must drop it.
	pending, err = st.PendingTelegramMedia(ctx, 3*time.Hour, 100)
	if err != nil {
		t.Fatalf("PendingTelegramMedia (tighter): %v", err)
	}
	for _, p := range pending {
		if p.MessageID == stale.MessageID {
			t.Error("a media row only 2h old was returned for a 3h threshold")
		}
	}
}
