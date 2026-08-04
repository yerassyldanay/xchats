package whatsapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// Fake is an in-process Manager for component/integration tests — the
// WhatsApp-side sibling of internal/telegram.Fake.
// InjectDebugEvent performs real store writes and SSE broadcasts (so a test
// exercises the genuine ingest -> worker -> AI-draft pipeline exactly like a
// real whatsmeow event would); outbound sends are recorded rather than
// reaching a real WhatsApp connection. Pairing is entirely canned: a real QR
// scan cannot happen in a test, so CompletePair stands in for one.
type Fake struct {
	Store *store.Store
	Hub   *realtime.Hub
	Queue queue.Queue // optional; nil skips the auto-AI-draft enqueue on inbound

	mu    sync.Mutex
	seq   int
	Calls []SentCall

	pairSessionID string
	pairUpdate    PairingUpdate
	LoggedOut     []string
}

// SentCall records one outbound send for test assertions.
type SentCall struct {
	AccountID string
	To        string
	Text      string
	MediaKind string
}

// NewFake returns a Fake wired to st/hub/q — the same store, hub and queue
// the test's Server and Worker use, so InjectDebugEvent's writes (and the
// auto-AI-draft enqueue on a genuine inbound message) are visible everywhere.
func NewFake(st *store.Store, hub *realtime.Hub, q queue.Queue) *Fake {
	return &Fake{Store: st, Hub: hub, Queue: q}
}

var _ Manager = (*Fake)(nil)

func (f *Fake) Pair(ctx context.Context, orgID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pairSessionID = uuid.NewString()
	f.pairUpdate = PairingUpdate{SessionID: f.pairSessionID, Status: "qr_required", QRCode: "FAKEQR", QRBase64: "data:image/png;base64,Zg=="}
	return f.pairSessionID, nil
}

func (f *Fake) PairStatus(sessionID string) (PairingUpdate, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pairSessionID == "" || sessionID != f.pairSessionID {
		return PairingUpdate{}, false
	}
	return f.pairUpdate, true
}

// CompletePair moves the last started pairing session straight to
// "connected" — a test's stand-in for a phone scanning the QR code.
func (f *Fake) CompletePair(accountID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pairUpdate = PairingUpdate{SessionID: f.pairSessionID, Status: "connected", AccountID: accountID}
}

func (f *Fake) Logout(ctx context.Context, accountID string) error {
	f.mu.Lock()
	f.LoggedOut = append(f.LoggedOut, accountID)
	f.mu.Unlock()
	id, err := uuid.Parse(accountID)
	if err != nil {
		return err
	}
	return f.Store.SetAccountState(ctx, id, "logged_out")
}

func (f *Fake) Status(ctx context.Context, accountID string) (ConnectionChanged, error) {
	id, err := uuid.Parse(accountID)
	if err != nil {
		return ConnectionChanged{}, err
	}
	acct, err := f.Store.AccountByID(ctx, id)
	if err != nil {
		return ConnectionChanged{}, err
	}
	return ConnectionChanged{AccountID: accountID, State: acct.ConnectionState}, nil
}

// InjectDebugEvent mirrors internal/whatsmeow.Manager's own ingest core
// (store upsert -> SSE for "message"; monotonic delivery-state advance for
// "receipt") closely enough that a component test built on this Fake
// exercises the same downstream httpapi/worker/store behavior a real
// whatsmeow-backed InjectDebugEvent would.
func (f *Fake) InjectDebugEvent(ctx context.Context, evt DebugEvent) (DebugEventResult, error) {
	accountID, err := uuid.Parse(evt.AccountID)
	if err != nil {
		return DebugEventResult{}, fmt.Errorf("whatsapp.Fake: invalid account id %q: %w", evt.AccountID, err)
	}
	switch evt.EventType {
	case "message":
		return f.injectMessage(ctx, accountID, evt)
	case "receipt":
		return DebugEventResult{}, f.injectReceipt(ctx, accountID, evt)
	case "connected", "disconnected", "logged_out":
		return DebugEventResult{}, f.Store.SetAccountState(ctx, accountID, evt.EventType)
	default:
		return DebugEventResult{}, fmt.Errorf("whatsapp.Fake: unknown event_type %q", evt.EventType)
	}
}

func (f *Fake) injectMessage(ctx context.Context, accountID uuid.UUID, evt DebugEvent) (DebugEventResult, error) {
	phoneJID := CanonicalJID(evt.SenderJID)
	externalID := evt.ExternalID
	if externalID == "" {
		externalID = uuid.NewString()
	}
	direction, senderKind := "in", "contact"
	if evt.IsFromMe {
		direction, senderKind = "out", "external_account"
	}
	res, err := f.Store.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: phoneJID, RemoteJID: phoneJID,
		PhoneNumber: PhoneFromJID(phoneJID), PushName: evt.SenderJID,
		Direction: direction, SenderKind: senderKind, ExternalMessageID: externalID,
		MessageKind: "conversation", Body: evt.Text, Preview: evt.Text,
		Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		return DebugEventResult{}, err
	}
	if !res.MessageInserted {
		f.broadcastMessage(ctx, res.MessageID, "message.updated")
		return DebugEventResult{MessageID: res.MessageID.String(), ChatID: res.ChatID.String()}, nil
	}
	f.broadcastMessage(ctx, res.MessageID, "message.created")
	name := "chat.updated"
	if res.ChatCreated {
		name = "chat.created"
	}
	if chat, err := f.Store.ChatByID(ctx, res.ChatID); err == nil {
		f.Hub.Broadcast(name, dto.MapChat(chat))
	}
	// Mirrors internal/whatsmeow.Manager's own auto-draft-on-inbound: a fresh
	// customer message queues an AI suggestion without anyone pressing
	// "Suggest" — echoes of our own sends (direction "out") never reach here.
	if f.Queue != nil && evt.EventType == "message" && !evt.IsFromMe {
		_ = f.Queue.Publish(ctx, queue.Message{Kind: queue.KindAIDraft, Payload: worker.AIDraftTask{ChatID: res.ChatID}})
	}
	return DebugEventResult{MessageID: res.MessageID.String(), ChatID: res.ChatID.String()}, nil
}

func (f *Fake) injectReceipt(ctx context.Context, accountID uuid.UUID, evt DebugEvent) error {
	state, ok := debugReceiptState(evt.ReceiptType)
	if !ok {
		return fmt.Errorf("whatsapp.Fake: unknown receipt_type %q", evt.ReceiptType)
	}
	if evt.ExternalID == "" {
		return fmt.Errorf("whatsapp.Fake: external_id is required for a receipt event")
	}
	rank := map[string]int{"delivered": 2, "read": 3}[state]
	msgID, _, err := f.Store.AdvanceDeliveryState(ctx, accountID, evt.ExternalID, state, rank)
	if err != nil {
		if err == store.ErrNotFound {
			return nil
		}
		return err
	}
	f.broadcastMessage(ctx, msgID, "message.updated")
	return nil
}

func debugReceiptState(receiptType string) (string, bool) {
	switch receiptType {
	case "":
		return "delivered", true
	case "read", "played":
		return "read", true
	default:
		return "", false
	}
}

func (f *Fake) broadcastMessage(ctx context.Context, id uuid.UUID, name string) {
	if msg, err := f.Store.MessageByID(ctx, id); err == nil {
		f.Hub.Broadcast(name, dto.MapMessage(msg))
	}
}

// ChannelSender returns a fake outbound sender that records calls instead of
// reaching a real WhatsApp connection.
func (f *Fake) ChannelSender() messaging.ChannelSender { return &fakeSender{f: f} }

type fakeSender struct{ f *Fake }

func (s *fakeSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	s.f.mu.Lock()
	s.f.seq++
	id := fmt.Sprintf("FAKEWA%04d", s.f.seq)
	kind := ""
	if out.Media != nil {
		kind = out.Media.Kind
	}
	s.f.Calls = append(s.f.Calls, SentCall{AccountID: out.AccountID, To: out.To, Text: out.Text, MediaKind: kind})
	s.f.mu.Unlock()
	return messaging.SendResult{ExternalID: id, Delivered: true}, nil
}

// TextCalls returns the recorded text-only sends (no attachment).
func (f *Fake) TextCalls() []SentCall {
	return f.callsWhere(func(c SentCall) bool { return c.MediaKind == "" })
}

// MediaCalls returns the recorded sends that carried an attachment, of any kind.
func (f *Fake) MediaCalls() []SentCall {
	return f.callsWhere(func(c SentCall) bool { return c.MediaKind != "" })
}

func (f *Fake) callsWhere(pred func(SentCall) bool) []SentCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []SentCall
	for _, c := range f.Calls {
		if pred(c) {
			out = append(out, c)
		}
	}
	return out
}
