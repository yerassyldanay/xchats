package tgingest_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
)

const testEncKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32 bytes hex

// --- fakes -------------------------------------------------------------

// fakeQueue records every publish and can be scripted to fail the NEXT
// publish of a given kind exactly once.
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

// fakeHub records every broadcast.
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
// HasDraftForTrigger (overridden when failHasDraft is set) — used to
// exercise the "lookup error tolerated" branch, which a real SQLite query
// essentially never fails on its own, without giving up the real store for
// everything else Process does.
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

// --- fixtures ------------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newAccount(t *testing.T, st *store.Store, botID int64) store.TelegramAccount {
	t.Helper()
	ctx := context.Background()
	box, err := secretbox.FromEnvValue(testEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)
	org, err := st.SeedOrganization(ctx, "tgingest-test-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	acct, err := st.ClaimTelegramAccount(ctx, store.TelegramClaim{
		ID:             config.ChannelAccountID(config.TelegramOwnerRef(botID)),
		OrganizationID: org.ID,
		BotID:          botID,
		BotUsername:    "tgingest_bot",
		BotToken:       "test-token",
	})
	if err != nil {
		t.Fatalf("claim telegram account: %v", err)
	}
	return acct
}

func newProcessor(st tgingest.Ingestor, q tgingest.Publisher, hub tgingest.Broadcaster) *tgingest.Processor {
	return tgingest.New(tgingest.Deps{Store: st, Queue: q, Hub: hub, Log: testLogger()})
}

func textUpdate(updateID, userID, msgID int64, text string) *telegram.Update {
	return &telegram.Update{
		UpdateID: updateID,
		Message: &telegram.Message{
			MessageID: msgID,
			Date:      time.Now().Unix(),
			From:      &telegram.User{ID: userID, FirstName: "Иван"},
			Chat:      &telegram.Chat{ID: userID, Type: "private"},
			Text:      text,
		},
	}
}

// --- tests -----------------------------------------------------------------

func TestProcessIgnorableHasNoSideEffects(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, 1001)
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	upd := &telegram.Update{UpdateID: 1, Message: &telegram.Message{
		MessageID: 1, From: &telegram.User{ID: 1}, Chat: &telegram.Chat{ID: -100, Type: "group"}, Text: "hi",
	}}
	outcome, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != "ignored" || outcome.Reason != "non_private_chat" {
		t.Fatalf("outcome = %+v, want {ignored non_private_chat}", outcome)
	}
	if n := len(q.Published()); n != 0 {
		t.Fatalf("published %d messages for an ignorable update, want 0", n)
	}
	if n := len(hub.Calls()); n != 0 {
		t.Fatalf("broadcast %d events for an ignorable update, want 0", n)
	}
}

func TestProcessNewMessageFullMappingBroadcastsAndPublishes(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, 1002)
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	upd := textUpdate(7, 42, 100, "сколько стоит?")
	outcome, err := p.Process(context.Background(), acct, upd, []byte(`{"update_id":7}`))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != "stored" {
		t.Fatalf("status = %q, want stored", outcome.Status)
	}
	if !outcome.Result.MessageInserted || !outcome.Result.ChatCreated {
		t.Fatalf("result = %+v, want a freshly inserted message in a freshly created chat", outcome.Result)
	}

	// The row must actually be readable back through the store.
	msg, err := st.MessageByID(context.Background(), outcome.Result.MessageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.Body != "сколько стоит?" || msg.Channel != "telegram" || msg.Direction != "in" {
		t.Fatalf("stored message = %+v", msg)
	}

	// Broadcasts: message.created and chat.created (order: message first,
	// then chat — see Process's own body).
	names := hub.names()
	if len(names) != 2 || names[0] != "message.created" || names[1] != "chat.created" {
		t.Fatalf("broadcasts = %v, want [message.created chat.created]", names)
	}

	// The AI draft task is published for the chat that was just created.
	pub := q.Published()
	if len(pub) != 1 || pub[0].Kind != queue.KindAIDraft {
		t.Fatalf("published = %+v, want exactly one ai_draft task", pub)
	}

	// A second, different chat on a redelivery-free path gets chat.updated,
	// not chat.created.
	upd2 := textUpdate(8, 42, 101, "ещё вопрос")
	outcome2, err := p.Process(context.Background(), acct, upd2, []byte(`{}`))
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
	acct := newAccount(t, st, 1003)
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	upd := &telegram.Update{UpdateID: 1, Message: &telegram.Message{
		MessageID: 1, Date: time.Now().Unix(),
		From: &telegram.User{ID: 1}, Chat: &telegram.Chat{ID: 1, Type: "private"},
		Caption: "вот фото",
		Photo:   []telegram.PhotoSize{{FileID: "f1", FileUniqueID: "u1", Width: 100, Height: 100}},
	}}
	outcome, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != "stored" {
		t.Fatalf("status = %q, want stored", outcome.Status)
	}
	if n := countKind(q.Published(), queue.KindMediaDownload); n != 1 {
		t.Fatalf("media_download publishes = %d, want 1", n)
	}
	if n := countKind(q.Published(), queue.KindAIDraft); n != 1 {
		t.Fatalf("ai_draft publishes = %d, want 1", n)
	}
}

func TestProcessMediaPublishFailureIsNonFatal(t *testing.T) {
	st := dbtest.New(t)
	acct := newAccount(t, st, 1004)
	q, hub := newFakeQueue(), &fakeHub{}
	q.FailNext(queue.KindMediaDownload, errors.New("queue full"))
	p := newProcessor(st, q, hub)

	upd := &telegram.Update{UpdateID: 1, Message: &telegram.Message{
		MessageID: 1, Date: time.Now().Unix(),
		From: &telegram.User{ID: 1}, Chat: &telegram.Chat{ID: 1, Type: "private"},
		Photo: []telegram.PhotoSize{{FileID: "f1", FileUniqueID: "u1", Width: 100, Height: 100}},
	}}
	outcome, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
	if err != nil {
		t.Fatalf("Process returned an error for a media-publish failure, want it treated as non-fatal: %v", err)
	}
	if outcome.Status != "stored" {
		t.Fatalf("status = %q, want stored", outcome.Status)
	}
	// The draft task — the durable-unit publish — must still have gone out.
	if n := countKind(q.Published(), queue.KindAIDraft); n != 1 {
		t.Fatalf("ai_draft publishes = %d, want 1 even though media_download failed", n)
	}
}

func TestProcessErrIngestOnIngestFailureEmitsNothing(t *testing.T) {
	st, db := dbtest.Open(t)
	acct := newAccount(t, st, 1005)
	q, hub := newFakeQueue(), &fakeHub{}
	p := newProcessor(st, q, hub)

	// A BEFORE INSERT trigger that RAISEs — fails inserts into tg_messages
	// specifically, mirroring internal/httpapi's own
	// TestTelegramWebhookAnswers500WhenIngestFails fixture.
	if _, err := db.Exec(context.Background(),
		`CREATE TRIGGER tg_messages_force_fail BEFORE INSERT ON tg_messages
		 BEGIN SELECT RAISE(ABORT, 'forced ingest failure'); END`); err != nil {
		t.Fatalf("install failing trigger: %v", err)
	}

	outcome, err := p.Process(context.Background(), acct, textUpdate(1, 1, 1, "hi"), []byte(`{}`))
	if !errors.Is(err, tgingest.ErrIngest) {
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
	acct := newAccount(t, st, 1006)
	q, hub := newFakeQueue(), &fakeHub{}
	q.FailNext(queue.KindAIDraft, errors.New("queue full"))
	p := newProcessor(st, q, hub)

	outcome, err := p.Process(context.Background(), acct, textUpdate(1, 1, 1, "hi"), []byte(`{}`))
	if !errors.Is(err, tgingest.ErrEnqueue) {
		t.Fatalf("err = %v, want ErrEnqueue", err)
	}
	// The message itself IS durably stored — only the follow-up enqueue failed.
	if outcome.Status != "stored" || !outcome.Result.MessageInserted {
		t.Fatalf("outcome = %+v, want Status=stored with the message row already written", outcome)
	}
}

func TestProcessDuplicateBranches(t *testing.T) {
	t.Run("no draft yet re-enqueues", func(t *testing.T) {
		st := dbtest.New(t)
		acct := newAccount(t, st, 2001)
		q, hub := newFakeQueue(), &fakeHub{}
		p := newProcessor(st, q, hub)
		upd := textUpdate(1, 1, 1, "hi")

		if _, err := p.Process(context.Background(), acct, upd, []byte(`{}`)); err != nil {
			t.Fatalf("first Process: %v", err)
		}
		if n := countKind(q.Published(), queue.KindAIDraft); n != 1 {
			t.Fatalf("ai_draft publishes after the first delivery = %d, want 1", n)
		}

		outcome, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
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
		acct := newAccount(t, st, 2002)
		q, hub := newFakeQueue(), &fakeHub{}
		p := newProcessor(st, q, hub)
		upd := textUpdate(1, 1, 1, "hi")

		outcome1, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
		if err != nil {
			t.Fatalf("first Process: %v", err)
		}
		// Simulate the worker having already generated a draft for this trigger.
		if _, err := st.WriteDraftSet(context.Background(), "telegram", outcome1.Result.ChatID,
			uuid.NullUUID{UUID: outcome1.Result.MessageID, Valid: true},
			[]store.DraftOption{{Ordinal: 1, Text: "черновик"}}); err != nil {
			t.Fatalf("WriteDraftSet: %v", err)
		}

		outcome2, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
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
		acct := newAccount(t, st, 2003)
		q, hub := newFakeQueue(), &fakeHub{}
		flaky := &flakyIngestor{Store: st}
		p := newProcessor(flaky, q, hub)
		upd := textUpdate(1, 1, 1, "hi")

		if _, err := p.Process(context.Background(), acct, upd, []byte(`{}`)); err != nil {
			t.Fatalf("first Process: %v", err)
		}
		flaky.failHasDraft = errors.New("boom: draft lookup unavailable")

		outcome, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
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
		acct := newAccount(t, st, 2004)
		q, hub := newFakeQueue(), &fakeHub{}
		p := newProcessor(st, q, hub)
		upd := textUpdate(1, 1, 1, "hi")

		if _, err := p.Process(context.Background(), acct, upd, []byte(`{}`)); err != nil {
			t.Fatalf("first Process: %v", err)
		}
		q.FailNext(queue.KindAIDraft, errors.New("queue full"))

		outcome, err := p.Process(context.Background(), acct, upd, []byte(`{}`))
		if !errors.Is(err, tgingest.ErrEnqueue) {
			t.Fatalf("err = %v, want ErrEnqueue", err)
		}
		if outcome.Status != "duplicate" {
			t.Fatalf("status = %q, want duplicate (the message itself is already durably stored)", outcome.Status)
		}
	})
}

func TestMessageKindAndPreviewParity(t *testing.T) {
	cases := []struct {
		name         string
		msg          *telegram.Message
		wantKind     string
		wantContains string
	}{
		{"plain text", &telegram.Message{Text: "привет"}, "conversation", "привет"},
		{"photo with caption", &telegram.Message{Caption: "подпись",
			Photo: []telegram.PhotoSize{{FileID: "f", Width: 10, Height: 10}}}, "imageMessage", "подпись"},
		{"photo without caption", &telegram.Message{
			Photo: []telegram.PhotoSize{{FileID: "f", Width: 10, Height: 10}}}, "imageMessage", "Фото"},
		{"sticker", &telegram.Message{Sticker: &telegram.Sticker{FileID: "s"}}, "stickerMessage", "Стикер"},
		{"video", &telegram.Message{Video: &telegram.Doc{FileID: "v"}}, "videoMessage", "Видео"},
		{"voice note (audio kind)", &telegram.Message{Voice: &telegram.Doc{FileID: "a"}}, "audioMessage", "Аудио"},
		{"audio file", &telegram.Message{Audio: &telegram.Doc{FileID: "a"}}, "audioMessage", "Аудио"},
		{"document", &telegram.Message{Document: &telegram.Doc{FileID: "d"}}, "documentMessage", "Документ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tgingest.MessageKind(tc.msg); got != tc.wantKind {
				t.Errorf("MessageKind = %q, want %q", got, tc.wantKind)
			}
			if got := tgingest.Preview(tc.msg); !strings.Contains(got, tc.wantContains) {
				t.Errorf("Preview = %q, want it to contain %q", got, tc.wantContains)
			}
		})
	}
}
