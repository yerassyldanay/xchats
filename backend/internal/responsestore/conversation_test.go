package responsestore_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

func sendKindFor(direction string) string {
	if direction == "out" {
		return "external_account"
	}
	return "contact"
}

func TestConversationRepo_LoadForResponse(t *testing.T) {
	st, orgID, accountID := newTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	turns := []struct {
		direction string
		text      string
	}{
		{"in", "Привет"},
		{"out", "Здравствуйте! Чем помочь?"},
		{"in", "Сколько стоит виджет?"},
	}
	var chatID uuid.UUID
	var lastMsgID uuid.UUID
	for i, turn := range turns {
		res, err := st.UpsertInbound(ctx, store.InboundUpsert{
			AccountID: accountID, PhoneJID: "77000000000@s.whatsapp.net", RemoteJID: "77000000000@s.whatsapp.net",
			PhoneNumber:        "77000000000",
			Direction:          turn.direction,
			SenderKind:         sendKindFor(turn.direction),
			EvolutionMessageID: fmt.Sprintf("MSG%d", i),
			MessageKind:        "conversation",
			Body:               turn.text,
			Source:             "live_webhook",
			MessageTS:          base.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("seed message %d: %v", i, err)
		}
		chatID = res.ChatID
		lastMsgID = res.MessageID
	}

	repo := &responsestore.ConversationRepo{Store: st}
	got, err := repo.LoadForResponse(ctx, chatID.String())
	if err != nil {
		t.Fatalf("LoadForResponse: %v", err)
	}
	if got.OrganizationID != orgID.String() {
		t.Fatalf("OrganizationID = %q, want %q", got.OrganizationID, orgID.String())
	}
	if got.AccountID != accountID.String() {
		t.Fatalf("AccountID = %q, want %q", got.AccountID, accountID.String())
	}
	if got.Channel != messaging.ChannelWhatsApp {
		t.Fatalf("Channel = %q, want %q", got.Channel, messaging.ChannelWhatsApp)
	}
	if got.CurrentMessage != "Сколько стоит виджет?" {
		t.Fatalf("CurrentMessage = %q", got.CurrentMessage)
	}
	if len(got.History) != 2 {
		t.Fatalf("History = %+v, want exactly 2 turns (latest inbound excluded)", got.History)
	}
	if got.History[0].Role != "client" || got.History[0].Text != "Привет" {
		t.Fatalf("History[0] = %+v", got.History[0])
	}
	if got.History[1].Role != "assistant" || got.History[1].Text != "Здравствуйте! Чем помочь?" {
		t.Fatalf("History[1] = %+v", got.History[1])
	}
	if got.TriggerMessageID != lastMsgID.String() {
		t.Fatalf("TriggerMessageID = %q, want %q", got.TriggerMessageID, lastMsgID.String())
	}
}

func TestConversationRepo_RejectsUnknownConversation(t *testing.T) {
	st, _, _ := newTestStore(t)
	repo := &responsestore.ConversationRepo{Store: st}
	if _, err := repo.LoadForResponse(context.Background(), uuid.NewString()); err == nil {
		t.Fatal("want an error for an unknown conversation id")
	}
}
