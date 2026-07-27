package responsestore_test

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/response"
)

func TestDraftRepo_ReplaceSuggested_PersistsAndSupersedes(t *testing.T) {
	st, _, accountID := newTestStore(t)
	ctx := context.Background()

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000000000@s.whatsapp.net", RemoteJID: "77000000000@s.whatsapp.net",
		PhoneNumber: "77000000000", Direction: "in", SenderKind: "contact",
		EvolutionMessageID: "TRIG1", MessageKind: "conversation", Body: "Привет", Source: "live_webhook",
	})
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}

	confidence := 0.9
	repo := &responsestore.DraftRepo{Store: st}
	first, err := repo.ReplaceSuggested(ctx, response.DraftToPersist{
		ConversationID: res.ChatID.String(), TriggerMessageID: res.MessageID.String(),
		Text: "Первый черновик", ReplyLanguage: "ru", Confidence: &confidence,
	})
	if err != nil {
		t.Fatalf("ReplaceSuggested (first): %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("want exactly one persisted draft, got %d", len(first))
	}
	if first[0].ReplyLanguage != "ru" || first[0].Text != "Первый черновик" {
		t.Fatalf("unexpected first draft: %+v", first[0])
	}
	if first[0].TriggerMessageID != res.MessageID.String() {
		t.Fatalf("TriggerMessageID = %q, want %q", first[0].TriggerMessageID, res.MessageID.String())
	}
	if first[0].ConversationID != res.ChatID.String() {
		t.Fatalf("ConversationID = %q, want %q", first[0].ConversationID, res.ChatID.String())
	}

	second, err := repo.ReplaceSuggested(ctx, response.DraftToPersist{
		ConversationID: res.ChatID.String(), TriggerMessageID: res.MessageID.String(),
		Text: "Второй черновик", ReplyLanguage: "kk", Escalate: true, EscalationReason: "нет данных",
	})
	if err != nil {
		t.Fatalf("ReplaceSuggested (second): %v", err)
	}
	if len(second) != 1 || second[0].Text != "Второй черновик" || second[0].ReplyLanguage != "kk" || !second[0].Escalate {
		t.Fatalf("unexpected second draft: %+v", second[0])
	}
	if second[0].EscalationReason != "нет данных" {
		t.Fatalf("EscalationReason = %q", second[0].EscalationReason)
	}

	pending, err := st.PendingDrafts(ctx, res.ChatID)
	if err != nil {
		t.Fatalf("PendingDrafts: %v", err)
	}
	if len(pending) != 1 || pending[0].ID.String() != second[0].ID {
		t.Fatalf("want exactly the second draft still suggested, got %+v", pending)
	}

	firstFromDB, err := st.DraftByID(ctx, mustParseUUID(t, first[0].ID))
	if err != nil {
		t.Fatalf("DraftByID: %v", err)
	}
	if firstFromDB.DraftState != "superseded" {
		t.Fatalf("first draft state = %q, want superseded", firstFromDB.DraftState)
	}
}

func TestDraftRepo_RejectsInvalidConversationID(t *testing.T) {
	st, _, _ := newTestStore(t)
	repo := &responsestore.DraftRepo{Store: st}
	_, err := repo.ReplaceSuggested(context.Background(), response.DraftToPersist{ConversationID: "not-a-uuid", Text: "x"})
	if err == nil {
		t.Fatal("want an error for an invalid conversation id")
	}
}
