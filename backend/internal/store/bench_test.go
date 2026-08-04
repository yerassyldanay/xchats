package store_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// benchAccount seeds one org + WhatsApp account for the benchmarks below —
// setup cost the benchmark loop itself must not be charged for.
func benchAccount(b *testing.B) (*store.Store, uuid.UUID) {
	b.Helper()
	ctx := context.Background()
	st := dbtest.New(b)
	org, err := st.SeedOrganization(ctx, "bench-org")
	if err != nil {
		b.Fatalf("seed org: %v", err)
	}
	ownerJID := "77099999999@s.whatsapp.net"
	accountID := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "Bench", ExternalAccountRef: config.CanonicalJID(ownerJID),
		ExternalHandle: config.PhoneFromJID(ownerJID), InstanceName: "bench", ConnectionState: "connected",
	}); err != nil {
		b.Fatalf("seed account: %v", err)
	}
	return st, accountID
}

// BenchmarkUpsertInbound_NewChat is the worst case per message: a brand-new
// contact and chat every call, exercising every ON CONFLICT/two-step-insert
// path UpsertInbound has (see internal/store/ingest.go's upsertChatTwoStep).
// This is the shape of traffic from many distinct customers each messaging
// for the first time.
func BenchmarkUpsertInbound_NewChat(b *testing.B) {
	ctx := context.Background()
	st, accountID := benchAccount(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		phone := fmt.Sprintf("7700%07d", i)
		if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
			AccountID: accountID, PhoneJID: phone + "@s.whatsapp.net", RemoteJID: phone + "@s.whatsapp.net",
			PhoneNumber:        phone,
			Direction:          "in",
			SenderKind:         "contact",
			EvolutionMessageID: fmt.Sprintf("BENCH-NEW-%d", i),
			MessageKind:        "conversation",
			Body:               "Здравствуйте, сколько стоит доставка?",
			Source:             "live_webhook",
			MessageTS:          time.Now(),
		}); err != nil {
			b.Fatalf("UpsertInbound: %v", err)
		}
	}
}

// BenchmarkUpsertInbound_ExistingChat is the steady-state case: every
// message lands in the SAME already-created chat (a real conversation's
// second, third, ... message) — no chat/contact creation cost, only the
// message insert + chat aggregate update.
func BenchmarkUpsertInbound_ExistingChat(b *testing.B) {
	ctx := context.Background()
	st, accountID := benchAccount(b)
	const phone = "77011234567"
	if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: phone + "@s.whatsapp.net", RemoteJID: phone + "@s.whatsapp.net",
		PhoneNumber: phone, Direction: "in", SenderKind: "contact",
		EvolutionMessageID: "BENCH-SEED", MessageKind: "conversation", Body: "seed", Source: "live_webhook",
		MessageTS: time.Now(),
	}); err != nil {
		b.Fatalf("seed chat: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
			AccountID: accountID, PhoneJID: phone + "@s.whatsapp.net", RemoteJID: phone + "@s.whatsapp.net",
			PhoneNumber:        phone,
			Direction:          "in",
			SenderKind:         "contact",
			EvolutionMessageID: fmt.Sprintf("BENCH-EXISTING-%d", i),
			MessageKind:        "conversation",
			Body:               "А в наличии есть?",
			Source:             "live_webhook",
			MessageTS:          time.Now(),
		}); err != nil {
			b.Fatalf("UpsertInbound: %v", err)
		}
	}
}

// BenchmarkUpsertInbound_ExistingChat_Parallel measures the throughput
// ceiling internal/dbx's single-writer-serialization design (see its
// package doc) actually sustains under concurrent load — every goroutine
// here contends for the SAME one physical connection AND the same chat's
// aggregate row, the worst case for contention, and the number this
// benchmark reports is the realistic answer to "how many inbound webhook
// deliveries per second can this process actually absorb."
func BenchmarkUpsertInbound_ExistingChat_Parallel(b *testing.B) {
	ctx := context.Background()
	st, accountID := benchAccount(b)
	const phone = "77011234567"
	if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: phone + "@s.whatsapp.net", RemoteJID: phone + "@s.whatsapp.net",
		PhoneNumber: phone, Direction: "in", SenderKind: "contact",
		EvolutionMessageID: "BENCH-SEED", MessageKind: "conversation", Body: "seed", Source: "live_webhook",
		MessageTS: time.Now(),
	}); err != nil {
		b.Fatalf("seed chat: %v", err)
	}

	var n int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := atomic.AddInt64(&n, 1)
			if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
				AccountID: accountID, PhoneJID: phone + "@s.whatsapp.net", RemoteJID: phone + "@s.whatsapp.net",
				PhoneNumber:        phone,
				Direction:          "in",
				SenderKind:         "contact",
				EvolutionMessageID: fmt.Sprintf("BENCH-PAR-%d", id),
				MessageKind:        "conversation",
				Body:               "А в наличии есть?",
				Source:             "live_webhook",
				MessageTS:          time.Now(),
			}); err != nil {
				b.Fatalf("UpsertInbound: %v", err)
			}
		}
	})
}

// BenchmarkChatByID/BenchmarkMessagesForChat cover the two read paths every
// GET /chats/:id request makes (internal/httpapi's chat detail view).
func BenchmarkChatByID(b *testing.B) {
	ctx := context.Background()
	st, accountID := benchAccount(b)
	const phone = "77011234567"
	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: phone + "@s.whatsapp.net", RemoteJID: phone + "@s.whatsapp.net",
		PhoneNumber: phone, Direction: "in", SenderKind: "contact",
		EvolutionMessageID: "BENCH-SEED", MessageKind: "conversation", Body: "seed", Source: "live_webhook",
		MessageTS: time.Now(),
	})
	if err != nil {
		b.Fatalf("seed chat: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.ChatByID(ctx, res.ChatID); err != nil {
			b.Fatalf("ChatByID: %v", err)
		}
	}
}

func BenchmarkMessagesForChat(b *testing.B) {
	ctx := context.Background()
	st, accountID := benchAccount(b)
	const phone = "77011234567"
	var chatID uuid.UUID
	for i := 0; i < 50; i++ {
		res, err := st.UpsertInbound(ctx, store.InboundUpsert{
			AccountID: accountID, PhoneJID: phone + "@s.whatsapp.net", RemoteJID: phone + "@s.whatsapp.net",
			PhoneNumber:        phone,
			Direction:          "in",
			SenderKind:         "contact",
			EvolutionMessageID: fmt.Sprintf("BENCH-HIST-%d", i),
			MessageKind:        "conversation",
			Body:               "история сообщений",
			Source:             "live_webhook",
			MessageTS:          time.Now(),
		})
		if err != nil {
			b.Fatalf("seed message %d: %v", i, err)
		}
		chatID = res.ChatID
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := st.MessagesForChat(ctx, chatID, time.Time{}, 20); err != nil {
			b.Fatalf("MessagesForChat: %v", err)
		}
	}
}
