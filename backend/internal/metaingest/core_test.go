package metaingest_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/metaingest"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// --- fakes (mirrors internal/tgingest's own test fakes) --------------------

type fakeQueue struct {
	mu        sync.Mutex
	published []queue.Message
	failNext  map[queue.Kind]error
}

func newFakeQueue() *fakeQueue { return &fakeQueue{failNext: map[queue.Kind]error{}} }

func (f *fakeQueue) Publish(_ context.Context, m queue.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.failNext[m.Kind]; ok {
		delete(f.failNext, m.Kind)
		return err
	}
	f.published = append(f.published, m)
	return nil
}

func (f *fakeQueue) FailNext(kind queue.Kind, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext[kind] = err
}

func (f *fakeQueue) Published() []queue.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]queue.Message(nil), f.published...)
}

func countKind(msgs []queue.Message, kind queue.Kind) int {
	n := 0
	for _, m := range msgs {
		if m.Kind == kind {
			n++
		}
	}
	return n
}

type fakeHub struct {
	mu    sync.Mutex
	calls []broadcastCall
}

type broadcastCall struct {
	Name string
	Data any
}

func (h *fakeHub) Broadcast(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, broadcastCall{Name: name, Data: data})
}

func (h *fakeHub) Calls() []broadcastCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]broadcastCall(nil), h.calls...)
}

func (h *fakeHub) names() []string {
	var out []string
	for _, c := range h.Calls() {
		out = append(out, c.Name)
	}
	return out
}

// flakyIngestor wraps a real *store.Store, forwarding every method except
// HasDraftForTrigger (overridden when failHasDraft is set) — see
// tgingest_test.go's identical fixture.
type flakyIngestor struct {
	*store.Store
	failHasDraft error
}

func (f *flakyIngestor) HasDraftForTrigger(ctx context.Context, id uuid.UUID) (bool, error) {
	if f.failHasDraft != nil {
		return false, f.failHasDraft
	}
	return f.Store.HasDraftForTrigger(ctx, id)
}

// --- fixtures ----------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newAccount(t *testing.T, st *store.Store, phoneNumberID string) store.ChannelAccount {
	t.Helper()
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "metaingest-test-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	acct, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID:                config.ChannelAccountID(config.WhatsAppCloudOwnerRef(phoneNumberID)),
		OrganizationID:    org.ID,
		Channel:           "whatsapp_cloud",
		ExternalAccountID: phoneNumberID,
		DisplayName:       "metaingest test number",
	})
	if err != nil {
		t.Fatalf("claim channel account: %v", err)
	}
	return acct
}

func newProcessor(st metaingest.Ingestor, q metaingest.Publisher, hub metaingest.Broadcaster) *metaingest.Processor {
	return metaingest.New(metaingest.Deps{Store: st, Queue: q, Hub: hub, Log: testLogger()})
}

func inboundText(from, externalMessageID, body string) metaingest.NormalizedMessage {
	return metaingest.NormalizedMessage{
		ExternalContactID: from, ContactHandle: from, ContactDisplayName: "Клиент Тест",
		ExternalThreadID: from, Direction: "in", SenderKind: "contact",
		ExternalMessageID: externalMessageID, MessageKind: "conversation", Body: body, Preview: body,
	}
}

// --- Process -----------------------------------------------------------

func TestProcessNewMessageFullMappingBroadcastsAndPublishes(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, "15550001111")
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	outcome, err := p.Process(context.Background(), acct, inboundText("77011234567", "wamid.1", "сколько стоит?"))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != "stored" {
		t.Fatalf("status = %q, want stored", outcome.Status)
	}
	if !outcome.Result.MessageInserted || !outcome.Result.ChatCreated {
		t.Fatalf("result = %+v, want a freshly inserted message in a freshly created chat", outcome.Result)
	}

	msg, err := st.MessageByID(context.Background(), outcome.Result.MessageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.Body != "сколько стоит?" || msg.Channel != "whatsapp_cloud" || msg.Direction != "in" {
		t.Fatalf("stored message = %+v", msg)
	}

	names := hub.names()
	if len(names) != 2 || names[0] != "message.created" || names[1] != "chat.created" {
		t.Fatalf("broadcasts = %v, want [message.created chat.created]", names)
	}

	pub := q.Published()
	if len(pub) != 1 || pub[0].Kind != queue.KindAIDraft {
		t.Fatalf("published = %+v, want exactly one ai_draft task", pub)
	}

	// A second message from the SAME customer lands in the same chat —
	// chat.updated, not chat.created.
	outcome2, err := p.Process(context.Background(), acct, inboundText("77011234567", "wamid.2", "ещё вопрос"))
	if err != nil {
		t.Fatalf("Process (second message, same chat): %v", err)
	}
	if outcome2.Result.ChatCreated {
		t.Fatal("second message in the SAME chat reported ChatCreated=true")
	}
	names2 := hub.names()
	if got := names2[len(names2)-1]; got != "chat.updated" {
		t.Fatalf("last broadcast = %q, want chat.updated", got)
	}
}

func TestProcessMediaAttachmentPublishesDownloadTask(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, "15550001112")
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	msg := inboundText("77011234567", "wamid.3", "вот скриншот")
	msg.MessageKind = "imageMessage"
	msg.Attachment = &metaingest.NormalizedAttachment{MediaType: "image", Mimetype: "image/jpeg", ProviderRef: "media-abc"}

	outcome, err := p.Process(context.Background(), acct, msg)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != "stored" {
		t.Fatalf("status = %q, want stored", outcome.Status)
	}
	pub := q.Published()
	if n := countKind(pub, queue.KindMediaDownload); n != 1 {
		t.Fatalf("media_download publishes = %d, want 1", n)
	}
	if n := countKind(pub, queue.KindAIDraft); n != 1 {
		t.Fatalf("ai_draft publishes = %d, want 1", n)
	}
	var dlTask worker.MediaDownloadTask
	for _, m := range pub {
		if m.Kind == queue.KindMediaDownload {
			dlTask = m.Payload.(worker.MediaDownloadTask)
		}
	}
	if dlTask.FileID != "media-abc" || dlTask.Channel != messaging.ChannelWhatsAppCloud || dlTask.AccountID != acct.ID {
		t.Fatalf("media_download task = %+v", dlTask)
	}
	if dlTask.BlobID != worker.ChannelBlobID(outcome.Result.MessageID) {
		t.Fatalf("BlobID = %q, want %q", dlTask.BlobID, worker.ChannelBlobID(outcome.Result.MessageID))
	}

	meta, err := st.ChannelMediaMetaFor(context.Background(), outcome.Result.MessageID)
	if err != nil {
		t.Fatalf("ChannelMediaMetaFor: %v", err)
	}
	if meta.ProviderRef != "media-abc" || meta.MediaType != "image" {
		t.Fatalf("media meta = %+v", meta)
	}
}

func TestProcessMediaPublishFailureIsNonFatal(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, "15550001113")
	q, hub := newFakeQueue(), &fakeHub{}
	q.FailNext(queue.KindMediaDownload, errors.New("queue full"))
	p := newProcessor(st, q, hub)

	msg := inboundText("77011234567", "wamid.4", "")
	msg.Attachment = &metaingest.NormalizedAttachment{MediaType: "image", ProviderRef: "media-xyz"}

	outcome, err := p.Process(context.Background(), acct, msg)
	if err != nil {
		t.Fatalf("Process returned an error for a media-publish failure, want it treated as non-fatal: %v", err)
	}
	if outcome.Status != "stored" {
		t.Fatalf("status = %q, want stored", outcome.Status)
	}
	if n := countKind(q.Published(), queue.KindAIDraft); n != 1 {
		t.Fatalf("ai_draft publishes = %d, want 1 even though media_download failed", n)
	}
}

func TestProcessErrIngestOnIngestFailureEmitsNothing(t *testing.T) {
	st, db := dbtest.Open(t)
	acct := newAccount(t, st, "15550001114")
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	if _, err := db.Exec(context.Background(),
		`CREATE TRIGGER channel_messages_force_fail BEFORE INSERT ON channel_messages
		 BEGIN SELECT RAISE(ABORT, 'forced ingest failure'); END`); err != nil {
		t.Fatalf("install failing trigger: %v", err)
	}

	outcome, err := p.Process(context.Background(), acct, inboundText("77011234567", "wamid.5", "hi"))
	if !errors.Is(err, metaingest.ErrIngest) {
		t.Fatalf("err = %v, want ErrIngest", err)
	}
	if outcome.Status != "" {
		t.Fatalf("outcome = %+v, want the zero value on an ingest failure", outcome)
	}
	if n := len(q.Published()); n != 0 {
		t.Fatalf("published %d messages on an ingest failure, want 0 (nothing emitted)", n)
	}
	if n := len(hub.Calls()); n != 0 {
		t.Fatalf("broadcast %d events on an ingest failure, want 0", n)
	}
}

func TestProcessErrEnqueueOnNewMessageDraftPublishFailure(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, "15550001115")
	q, hub := newFakeQueue(), &fakeHub{}
	q.FailNext(queue.KindAIDraft, errors.New("queue full"))
	p := newProcessor(st, q, hub)

	outcome, err := p.Process(context.Background(), acct, inboundText("77011234567", "wamid.6", "hi"))
	if !errors.Is(err, metaingest.ErrEnqueue) {
		t.Fatalf("err = %v, want ErrEnqueue", err)
	}
	if outcome.Status != "stored" || !outcome.Result.MessageInserted {
		t.Fatalf("outcome = %+v, want Status=stored with the message row already written", outcome)
	}
}

func TestProcessOutboundEchoDoesNotArmAutomation(t *testing.T) {
	// WhatsApp Cloud never reaches this branch in production (§10) — but the
	// same Process is architected to serve Instagram/Messenger's is_echo
	// shape from Phase 4/5, so the branch is exercised here directly.
	st := dbtest.New(t)
	acct := newAccount(t, st, "15550001116")
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	msg := inboundText("77011234567", "wamid.7", "operator reply")
	msg.Direction = "out"
	msg.SenderKind = "external_account"

	outcome, err := p.Process(context.Background(), acct, msg)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != "stored" {
		t.Fatalf("status = %q, want stored", outcome.Status)
	}
	if n := countKind(q.Published(), queue.KindAIDraft); n != 0 {
		t.Fatalf("ai_draft publishes for an outbound echo = %d, want 0", n)
	}
}

func TestProcessDuplicateBranches(t *testing.T) {
	t.Run("no draft yet re-enqueues", func(t *testing.T) {
		st := dbtest.New(t)
		acct := newAccount(t, st, "15550002001")
		q, hub := newFakeQueue(), &fakeHub{}
		p := newProcessor(st, q, hub)
		msg := inboundText("77011234567", "wamid.d1", "hi")

		if _, err := p.Process(context.Background(), acct, msg); err != nil {
			t.Fatalf("first Process: %v", err)
		}
		if n := countKind(q.Published(), queue.KindAIDraft); n != 1 {
			t.Fatalf("ai_draft publishes after the first delivery = %d, want 1", n)
		}

		outcome, err := p.Process(context.Background(), acct, msg)
		if err != nil {
			t.Fatalf("second (redelivered) Process: %v", err)
		}
		if outcome.Status != "duplicate" {
			t.Fatalf("status = %q, want duplicate", outcome.Status)
		}
		if n := countKind(q.Published(), queue.KindAIDraft); n != 2 {
			t.Fatalf("ai_draft publishes after redelivery (no draft ever landed) = %d, want 2 (re-enqueued)", n)
		}
	})

	t.Run("draft already present skips re-enqueue", func(t *testing.T) {
		st := dbtest.New(t)
		acct := newAccount(t, st, "15550002002")
		q, hub := newFakeQueue(), &fakeHub{}
		p := newProcessor(st, q, hub)
		msg := inboundText("77011234567", "wamid.d2", "hi")

		outcome1, err := p.Process(context.Background(), acct, msg)
		if err != nil {
			t.Fatalf("first Process: %v", err)
		}
		if _, err := st.WriteDraftSet(context.Background(), "whatsapp_cloud", outcome1.Result.ChatID,
			uuid.NullUUID{UUID: outcome1.Result.MessageID, Valid: true},
			[]store.DraftOption{{Ordinal: 1, Text: "черновик"}}); err != nil {
			t.Fatalf("WriteDraftSet: %v", err)
		}

		outcome2, err := p.Process(context.Background(), acct, msg)
		if err != nil {
			t.Fatalf("second Process: %v", err)
		}
		if outcome2.Status != "duplicate" {
			t.Fatalf("status = %q, want duplicate", outcome2.Status)
		}
		if n := countKind(q.Published(), queue.KindAIDraft); n != 1 {
			t.Fatalf("ai_draft publishes = %d, want 1 (a draft already exists — no re-enqueue)", n)
		}
	})

	t.Run("draft lookup error is tolerated", func(t *testing.T) {
		st := dbtest.New(t)
		acct := newAccount(t, st, "15550002003")
		q, hub := newFakeQueue(), &fakeHub{}
		flaky := &flakyIngestor{Store: st}
		p := newProcessor(flaky, q, hub)
		msg := inboundText("77011234567", "wamid.d3", "hi")

		if _, err := p.Process(context.Background(), acct, msg); err != nil {
			t.Fatalf("first Process: %v", err)
		}
		flaky.failHasDraft = errors.New("boom: draft lookup unavailable")

		outcome, err := p.Process(context.Background(), acct, msg)
		if err != nil {
			t.Fatalf("second Process: %v, want the lookup failure tolerated (message is stored either way)", err)
		}
		if outcome.Status != "duplicate" {
			t.Fatalf("status = %q, want duplicate", outcome.Status)
		}
		if n := countKind(q.Published(), queue.KindAIDraft); n != 1 {
			t.Fatalf("ai_draft publishes = %d, want 1 (lookup failed, so no re-enqueue was attempted)", n)
		}
	})

	t.Run("re-enqueue publish failure surfaces ErrEnqueue", func(t *testing.T) {
		st := dbtest.New(t)
		acct := newAccount(t, st, "15550002004")
		q, hub := newFakeQueue(), &fakeHub{}
		p := newProcessor(st, q, hub)
		msg := inboundText("77011234567", "wamid.d4", "hi")

		if _, err := p.Process(context.Background(), acct, msg); err != nil {
			t.Fatalf("first Process: %v", err)
		}
		q.FailNext(queue.KindAIDraft, errors.New("queue full"))

		outcome, err := p.Process(context.Background(), acct, msg)
		if !errors.Is(err, metaingest.ErrEnqueue) {
			t.Fatalf("err = %v, want ErrEnqueue", err)
		}
		if outcome.Status != "duplicate" {
			t.Fatalf("status = %q, want duplicate (the message itself is already durably stored)", outcome.Status)
		}
	})
}

// --- ApplyStatus ---------------------------------------------------------

func TestApplyStatusAdvancesAndBroadcasts(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, "15550003001")
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	// Seed a chat via a normal inbound message first (FindOrCreateChat is
	// wa_*-specific; the channel core has no direct equivalent), then add an
	// outbound message with a known external id — the shape a real send
	// leaves behind (InsertOutbound + StampOutboundSent).
	ctx := context.Background()
	inbound, err := p.Process(ctx, acct, inboundText("77011234567", "wamid.in", "hi"))
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	msgID, err := st.InsertOutbound(ctx, "whatsapp_cloud", inbound.Result.ChatID, acct.ID, "user", uuid.NullUUID{}, "conversation", "reply", "reply")
	if err != nil {
		t.Fatalf("InsertOutbound: %v", err)
	}
	if err := st.StampOutboundSent(ctx, "whatsapp_cloud", msgID, "wamid.out1"); err != nil {
		t.Fatalf("StampOutboundSent: %v", err)
	}

	applied, err := p.ApplyStatus(ctx, metaingest.StatusUpdate{
		AccountID: acct.ID, Channel: "whatsapp_cloud", ExternalMessageID: "wamid.out1", Status: "delivered", Rank: 2,
	})
	if err != nil {
		t.Fatalf("ApplyStatus: %v", err)
	}
	if !applied {
		t.Fatal("applied = false, want true")
	}
	got, err := st.MessageByID(ctx, msgID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if got.DeliveryState != "delivered" {
		t.Fatalf("delivery_state = %q, want delivered", got.DeliveryState)
	}
	names := hub.names()
	if len(names) == 0 || names[len(names)-1] != "message.updated" {
		t.Fatalf("broadcasts = %v, want the last one to be message.updated", names)
	}

	// A stale (lower-rank) status must not regress the state.
	applied2, err := p.ApplyStatus(ctx, metaingest.StatusUpdate{
		AccountID: acct.ID, Channel: "whatsapp_cloud", ExternalMessageID: "wamid.out1", Status: "sent", Rank: 1,
	})
	if err != nil {
		t.Fatalf("ApplyStatus (stale): %v", err)
	}
	if applied2 {
		t.Fatal("a lower-rank status must not apply (monotonic ladder)")
	}
}

func TestApplyStatusUnknownMessageIsNotAnError(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, "15550003002")
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	applied, err := p.ApplyStatus(context.Background(), metaingest.StatusUpdate{
		AccountID: acct.ID, Channel: "whatsapp_cloud", ExternalMessageID: "wamid.never-sent", Status: "delivered", Rank: 2,
	})
	if err != nil {
		t.Fatalf("ApplyStatus: %v, want no error for an id this install never sent", err)
	}
	if applied {
		t.Fatal("applied = true for an unknown external message id")
	}
	if n := len(hub.Calls()); n != 0 {
		t.Fatalf("broadcast %d events for a no-op status update, want 0", n)
	}
}
