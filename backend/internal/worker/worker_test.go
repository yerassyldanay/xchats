package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

type recordingSender struct {
	got messaging.OutboundMessage
}

func (s *recordingSender) Send(_ context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	s.got = out
	return messaging.SendResult{ExternalID: "provider-message-1", Delivered: true}, nil
}

// A WhatsApp conversation ref is an opaque provider address. In particular,
// a LID must reach the whatsmeow adapter unchanged: removing its suffix turns
// it into a different phone JID and makes whatsmeow fail its LID lookup.
func TestHandleOutboundSendPreservesWhatsAppLID(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "worker-test")
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	accountID := uuid.New()
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		ExternalAccountRef: "77011111111@s.whatsapp.net", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	chatID, _, err := st.FindOrCreateChat(ctx, accountID, "5231387607239@lid", "")
	if err != nil {
		t.Fatalf("create chat: %v", err)
	}
	messageID, err := st.InsertOutbound(ctx, "whatsapp", chatID, accountID, "user", uuid.NullUUID{}, "conversation", "hello", "hello")
	if err != nil {
		t.Fatalf("insert outbound: %v", err)
	}

	recorder := &recordingSender{}
	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, recorder)
	w := &Worker{
		Store: st, Senders: senders, Hub: realtime.NewHub(),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	err = w.handleOutboundSend(ctx, OutboundTask{
		MessageID: messageID, AccountID: accountID, Channel: messaging.ChannelWhatsApp,
		Destination: "5231387607239@lid", Text: "hello",
	})
	if err != nil {
		t.Fatalf("handle outbound send: %v", err)
	}
	if recorder.got.To != "5231387607239@lid" {
		t.Fatalf("sender destination = %q, want the original LID", recorder.got.To)
	}
}
