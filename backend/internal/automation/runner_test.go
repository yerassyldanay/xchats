package automation

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// --- test doubles -----------------------------------------------------

type fakeChatClient struct {
	replyText string
}

func (f *fakeChatClient) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	b, _ := json.Marshal(map[string]any{
		"reply_text": f.replyText, "reply_language": "ru", "media_files_to_send": []string{},
		"escalate": false, "escalation_reason": "",
	})
	return llm.ChatResponse{Text: string(b)}, nil
}

type fakeRegistry struct{ client llm.ChatClient }

func (r *fakeRegistry) Client(ref llm.ModelRef) (llm.ChatClient, error) { return r.client, nil }

type fakeKBRepo struct{}

func (fakeKBRepo) Load(ctx context.Context, organizationID string) (*aiprompt.KB, error) {
	return &aiprompt.KB{
		OrganizationID: organizationID,
		Assistant:      &aiprompt.Assistant{Persona: "Тестовый ассистент.", ReplyMaxWords: 120},
		Contacts:       &aiprompt.Contacts{Phone: "+7 700 000 00 00"},
	}, nil
}

type fakeHub struct {
	mu    sync.Mutex
	calls []string
}

func (h *fakeHub) Broadcast(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, name)
}

func (h *fakeHub) count(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c == name {
			n++
		}
	}
	return n
}

// fakeQueue is a minimal queue.Queue that just records what was published —
// no Start/consumer, since these tests only assert on WHAT would be sent,
// not on the send actually completing through internal/worker.
type fakeQueue struct {
	mu        sync.Mutex
	published []queue.Message
}

func (q *fakeQueue) Publish(ctx context.Context, m queue.Message) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.published = append(q.published, m)
	return nil
}
func (q *fakeQueue) Start(ctx context.Context, h queue.Handler) {}
func (q *fakeQueue) Wait()                                      {}
func (q *fakeQueue) Close()                                     {}

func (q *fakeQueue) outboundSends() []queue.Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []queue.Message
	for _, m := range q.published {
		if m.Kind == queue.KindOutboundSend {
			out = append(out, m)
		}
	}
	return out
}

// testRunner builds a Runner over a real dbtest store, a real
// responsestore.ConversationRepo (so it reads the seeded chat/messages),
// a fake KB, and a scripted LLM — everything needed to exercise generation,
// version-gating, and the auto-send decision without a network call.
func testRunner(st *store.Store, replyText string, hub *fakeHub, q *fakeQueue) *Runner {
	svc := &response.Service{
		Conversations: &responsestore.ConversationRepo{Store: st},
		KnowledgeBase: fakeKBRepo{},
		Drafts:        nil, // Runner always swaps in its own versionGatedDrafts
		Engine: &response.Engine{
			LLMs: &fakeRegistry{client: &fakeChatClient{replyText: replyText}},
			Params: func() response.LLMParams {
				return response.LLMParams{DefaultModel: llm.ModelRef{Provider: "openrouter", Model: "test-model"}, MaxTokens: 200, Temperature: 0.3}
			},
		},
	}
	return &Runner{Store: st, Response: svc, Queue: q, Hub: hub, Log: testLogger()}
}

// --- fixtures -----------------------------------------------------------

func seedRunnerChat(t *testing.T, st *store.Store) (accountID, chatID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, t.Name())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	ownerJID := t.Name() + "@s.whatsapp.net"
	accountID = config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: ownerJID,
		ExternalHandle: "77000000000", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77011111111@s.whatsapp.net",
		RemoteJID: "77011111111@s.whatsapp.net", PhoneNumber: "77011111111",
		Direction: "in", SenderKind: "contact", ExternalMessageID: uuid.NewString(),
		MessageKind: "conversation", Body: "Сколько стоит доставка?",
		Preview: "Сколько стоит доставка?", Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	return accountID, res.ChatID
}

func alwaysWindows() []store.AutomationWindowInput {
	out := make([]store.AutomationWindowInput, 0, 7)
	for wd := 0; wd < 7; wd++ {
		out = append(out, store.AutomationWindowInput{Weekday: wd, StartMinute: 0, EndMinute: 1440})
	}
	return out
}

// --- tests ----------------------------------------------------------------

func TestRunnerSuggestionsModeGeneratesButNeverSends(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	if err := (&Scheduler{Store: st}).OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	version, err := st.CurrentBurstVersion(ctx, chatID)
	if err != nil || version != 1 {
		t.Fatalf("burst version = %d, %v, want 1", version, err)
	}
	jobID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", version)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	hub, q := &fakeHub{}, &fakeQueue{}
	r := testRunner(st, "Доставка стоит 1000 тенге.", hub, q)
	r.HandleRun(ctx, jobID)

	pending, err := st.PendingDrafts(ctx, chatID)
	if err != nil {
		t.Fatalf("PendingDrafts: %v", err)
	}
	if len(pending) != 1 || pending[0].DraftText != "Доставка стоит 1000 тенге." {
		t.Fatalf("pending drafts = %+v", pending)
	}
	if pending[0].DraftState != "suggested" {
		t.Fatalf("draft state = %q, want suggested (suggestions mode never sends)", pending[0].DraftState)
	}
	if hub.count("ai_draft.created") != 1 {
		t.Fatalf("ai_draft.created broadcasts = %d, want 1", hub.count("ai_draft.created"))
	}
	if len(q.outboundSends()) != 0 {
		t.Fatalf("suggestions mode must never enqueue an outbound send: %+v", q.outboundSends())
	}
	if _, err := st.DispatchJobByID(ctx, jobID); err != store.ErrNotFound {
		t.Fatalf("dispatch job should be deleted once settled: %v", err)
	}
}

func TestRunnerScheduledAutoInWindowAutoSends(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, alwaysWindows()); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}

	before, err := st.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}

	if err := (&Scheduler{Store: st}).OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	version, _ := st.CurrentBurstVersion(ctx, chatID)
	jobID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", version)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	hub, q := &fakeHub{}, &fakeQueue{}
	r := testRunner(st, "Доставка бесплатно.", hub, q)
	r.HandleRun(ctx, jobID)

	pending, err := st.PendingDrafts(ctx, chatID)
	if err != nil {
		t.Fatalf("PendingDrafts: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("an auto-sent draft must not remain in the suggested list: %+v", pending)
	}
	sends := q.outboundSends()
	if len(sends) != 1 {
		t.Fatalf("outbound sends = %d, want 1", len(sends))
	}
	task, ok := sends[0].Payload.(worker.OutboundTask)
	if !ok {
		t.Fatalf("outbound send payload = %T, want worker.OutboundTask", sends[0].Payload)
	}
	if task.Text != "Доставка бесплатно." {
		t.Errorf("outbound task text = %q", task.Text)
	}
	if task.AccountID != accountID {
		t.Errorf("outbound task account = %s, want %s", task.AccountID, accountID)
	}
	if task.Channel != messaging.ChannelWhatsApp {
		t.Errorf("outbound task channel = %q, want whatsapp", task.Channel)
	}

	after, err := st.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID after send: %v", err)
	}
	if after.UnreadCount != before.UnreadCount {
		t.Fatalf("auto-send must not clear unread: before=%d after=%d", before.UnreadCount, after.UnreadCount)
	}
	if after.LastMessagePreview != "Доставка бесплатно." {
		t.Fatalf("last_message_preview = %q", after.LastMessagePreview)
	}
	if hub.count("message.created") != 1 {
		t.Fatalf("message.created broadcasts = %d, want 1", hub.count("message.created"))
	}
	if hub.count("ai_draft.updated") != 1 {
		t.Fatalf("ai_draft.updated broadcasts = %d, want 1", hub.count("ai_draft.updated"))
	}
}

func TestRunnerScheduledAutoOutOfWindowOnlySuggests(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)
	// scheduled_auto with NO windows configured: InSchedule is always false.
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}

	if err := (&Scheduler{Store: st}).OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	version, _ := st.CurrentBurstVersion(ctx, chatID)
	jobID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", version)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	hub, q := &fakeHub{}, &fakeQueue{}
	r := testRunner(st, "ответ вне расписания", hub, q)
	r.HandleRun(ctx, jobID)

	pending, err := st.PendingDrafts(ctx, chatID)
	if err != nil {
		t.Fatalf("PendingDrafts: %v", err)
	}
	if len(pending) != 1 || pending[0].DraftState != "suggested" {
		t.Fatalf("expected exactly one visible suggestion outside the schedule: %+v", pending)
	}
	if len(q.outboundSends()) != 0 {
		t.Fatalf("must not send outside the configured schedule: %+v", q.outboundSends())
	}
}

func TestRunnerDiscardsStaleGeneration(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	sched := &Scheduler{Store: st}
	if err := sched.OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage #1: %v", err)
	}
	staleVersion, _ := st.CurrentBurstVersion(ctx, chatID)
	jobID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", staleVersion)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	// A new customer message arrives (simulating one arriving mid-generation)
	// and bumps the chat's live burst version past what jobID was claimed at.
	if err := sched.OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage #2: %v", err)
	}

	hub, q := &fakeHub{}, &fakeQueue{}
	r := testRunner(st, "устаревший ответ", hub, q)
	r.HandleRun(ctx, jobID)

	pending, err := st.PendingDrafts(ctx, chatID)
	if err != nil {
		t.Fatalf("PendingDrafts: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("a stale generation must never become visible: %+v", pending)
	}
	if hub.count("ai_draft.created") != 0 {
		t.Fatalf("a discarded stale draft must not broadcast ai_draft.created")
	}
	if len(q.outboundSends()) != 0 {
		t.Fatalf("a discarded stale draft must never be sent")
	}
	if _, err := st.DispatchJobByID(ctx, jobID); err != store.ErrNotFound {
		t.Fatalf("the stale dispatch job should be deleted once settled: %v", err)
	}
}

func TestRunnerOffModeAtRuntimeDiscardsAlreadyProcessingJob(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	if err := (&Scheduler{Store: st}).OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	version, _ := st.CurrentBurstVersion(ctx, chatID)
	jobID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", version)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}
	// Simulate this job already being claimed by a worker (status
	// 'processing') at the exact moment an operator switches the channel
	// off — SetAutomationSettings's proactive cleanup only deletes
	// 'pending' rows, so this is the defense-in-depth path inside HandleRun
	// itself that must catch it.
	if err := st.MarkDispatchProcessing(ctx, jobID); err != nil {
		t.Fatalf("MarkDispatchProcessing: %v", err)
	}
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "off", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings(off): %v", err)
	}

	hub, q := &fakeHub{}, &fakeQueue{}
	r := testRunner(st, "не должен появиться", hub, q)
	r.HandleRun(ctx, jobID)

	pending, err := st.PendingDrafts(ctx, chatID)
	if err != nil {
		t.Fatalf("PendingDrafts: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("off mode must generate nothing, even for a job already in flight: %+v", pending)
	}
	if _, err := st.DispatchJobByID(ctx, jobID); err != store.ErrNotFound {
		t.Fatalf("the invalidated dispatch job should be deleted: %v", err)
	}
}

func TestRunnerHandleRunUnknownJobIsANoop(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	hub, q := &fakeHub{}, &fakeQueue{}
	r := testRunner(st, "n/a", hub, q)

	// Must not panic on a job id that does not exist (already handled, or
	// never existed).
	r.HandleRun(ctx, uuid.New())

	if len(hub.calls) != 0 || len(q.published) != 0 {
		t.Fatalf("an unknown job must produce no side effects: hub=%v queue=%v", hub.calls, q.published)
	}
}

// gatedChatClient blocks inside Complete until the test releases it, and
// reports each entry on arrived. Used to force two concurrent HandleRun
// calls to reach the version-gated persist / auto-send claim at exactly the
// same moment, instead of relying on incidental goroutine scheduling.
type gatedChatClient struct {
	replyText string
	arrived   chan struct{}
	release   chan struct{}
}

func (c *gatedChatClient) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	c.arrived <- struct{}{}
	<-c.release
	b, _ := json.Marshal(map[string]any{
		"reply_text": c.replyText, "reply_language": "ru", "media_files_to_send": []string{},
		"escalate": false, "escalation_reason": "",
	})
	return llm.ChatResponse{Text: string(b)}, nil
}

// TestConcurrentHandleRunOnSameDispatchJobCanDoubleSend exercises the
// scenario TestRecoverDispatchJobsCanDuplicateAnAlreadyClaimedJob proves is
// reachable: the SAME dispatch job id handed to two workers at once. Both
// goroutines call Runner.HandleRun(ctx, jobID) concurrently; the gated LLM
// client holds both at the generation step until both have arrived, then
// releases them together, maximizing the chance both reach the
// version-gated persist (store.WriteDraftSetIfVersionCurrent) and the
// auto-send claim (store.ClaimDraftForAutoSend) concurrently. Both checks
// key off the SAME burst_version / trigger message — neither was designed
// to fence against a sibling draft generated from the identical burst — so
// this can produce two independently "sent" outbound messages to the same
// customer for one customer message.
//
// This test asserts the correct outcome (exactly one send). In practice it
// passes reliably (verified over 20 runs under -race): releasing both
// goroutines simultaneously tends to let the SECOND persist's supersede-
// existing-suggested-drafts step (store.WriteDraftSetIfVersionCurrent) land
// before the FIRST has reached ClaimDraftForAutoSend, which self-heals this
// particular interleaving. That does not mean the bug is fake — see
// TestSequentialDuplicateDispatchJobsBothAutoSend below, which forces the
// interleaving self-healing does NOT cover (one worker's full round trip,
// including the send, completing before the second's generation begins —
// the realistic shape of two dispatch jobs picked up a full recovery-tick
// apart) and fails deterministically.
func TestConcurrentHandleRunOnSameDispatchJobCanDoubleSend(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, alwaysWindows()); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}

	if err := (&Scheduler{Store: st}).OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	version, _ := st.CurrentBurstVersion(ctx, chatID)
	jobID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", version)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	hub, q := &fakeHub{}, &fakeQueue{}
	client := &gatedChatClient{replyText: "Доставка бесплатно.", arrived: make(chan struct{}, 2), release: make(chan struct{})}
	r := &Runner{
		Store: st,
		Response: &response.Service{
			Conversations: &responsestore.ConversationRepo{Store: st},
			KnowledgeBase: fakeKBRepo{},
			Engine: &response.Engine{
				LLMs: &fakeRegistry{client: client},
				Params: func() response.LLMParams {
					return response.LLMParams{DefaultModel: llm.ModelRef{Provider: "openrouter", Model: "test-model"}, MaxTokens: 200, Temperature: 0.3}
				},
			},
		},
		Queue: q, Hub: hub, Log: testLogger(),
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			r.HandleRun(ctx, jobID)
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-client.arrived:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for both concurrent HandleRun calls to start generating")
		}
	}
	close(client.release)
	wg.Wait()

	sends := q.outboundSends()
	if len(sends) != 1 {
		t.Fatalf("BUG (see TestRecoverDispatchJobsCanDuplicateAnAlreadyClaimedJob / Finding #1): two "+
			"concurrent HandleRun calls on the same dispatch job id produced %d outbound sends, want "+
			"exactly 1: %+v", len(sends), sends)
	}
}

// TestSequentialDuplicateDispatchJobsBothAutoSend reproduces Finding #1's
// double-send deterministically, with no goroutines, by driving the exact
// end state a duplicate dispatch enqueue produces in production: two
// dispatch job rows for the SAME chat and the SAME burst_version (nothing
// in the schema prevents this — automation_dispatch_jobs' primary key is a
// random id, not (chat_id, burst_version); see
// migrations/sqlite/0009_automation.up.sql), run one all the way to
// completion (generate, persist, auto-send, delete), then run the second.
//
// The first HandleRun's auto-send records its outbound message via
// InsertAutomationOutbound with sender_kind='ai' (see
// store.InsertAutomationOutbound / automation.go's package doc). The second
// HandleRun's own persist step (store.WriteDraftSetIfVersionCurrent) still
// matches the unchanged burst_version and so is not discarded as stale, and
// its own auto-send claim (store.ClaimDraftForAutoSend) checks for a human
// reply via `sender_kind IN ('user','external_account')` — 'ai' is not in
// that list (Finding C) — so it sees no reply, sees the same still-latest
// inbound trigger, and sends too. Two messages reach the customer for the
// one inbound message that started this burst.
func TestSequentialDuplicateDispatchJobsBothAutoSend(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, alwaysWindows()); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}

	if err := (&Scheduler{Store: st}).OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	version, _ := st.CurrentBurstVersion(ctx, chatID)

	// Two distinct dispatch job rows for the identical (chat, burst) —
	// exactly what TestRecoverDispatchJobsCanDuplicateAnAlreadyClaimedJob
	// shows recoverDispatchJobs can hand to two different workers (whether
	// as the same id delivered twice, or — equally possible, since
	// CreateDispatchJob has no dedup — as two separate rows from two racing
	// claim/recovery passes).
	jobA, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", version)
	if err != nil {
		t.Fatalf("CreateDispatchJob (A): %v", err)
	}
	jobB, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", version)
	if err != nil {
		t.Fatalf("CreateDispatchJob (B): %v", err)
	}

	hub, q := &fakeHub{}, &fakeQueue{}
	r := testRunner(st, "Доставка бесплатно.", hub, q)

	r.HandleRun(ctx, jobA)
	if got := len(q.outboundSends()); got != 1 {
		t.Fatalf("setup: worker A's run should send once, got %d", got)
	}

	r.HandleRun(ctx, jobB)

	sends := q.outboundSends()
	if len(sends) != 1 {
		t.Fatalf("BUG (Finding #1 + Finding C — store.ClaimDraftForAutoSend's human-reply check excludes "+
			"sender_kind='ai'): a second dispatch job for the same already-answered burst auto-sent AGAIN "+
			"— outbound sends = %d, want 1: %+v", len(sends), sends)
	}
}
