package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/autoresponse"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// --- fixtures + helpers -----------------------------------------------------

// craftInboundAt is craftInbound (accounts_test.go) with an explicit
// messageTimestamp. craftInbound's own timestamp is a fixed historical
// constant (2026-06-14) — fine for tests that don't care, but auto-response
// arming fails closed on any trigger older than 15 minutes, so every test
// here that needs a message to actually arm a job must control its own,
// fresh timestamp.
func craftInboundAt(owner, customer, keyID, text string, at time.Time) []byte {
	ev := map[string]any{
		"event": "messages.upsert", "instance": "any", "sender": owner,
		"data": map[string]any{
			"key":              map[string]any{"remoteJid": customer, "remoteJidAlt": customer, "fromMe": false, "id": keyID},
			"pushName":         "Клиент",
			"message":          map[string]any{"conversation": text},
			"messageType":      "conversation",
			"messageTimestamp": at.Unix(),
		},
	}
	b, _ := json.Marshal(ev)
	return b
}

// allDayEveryDay covers every hour of every weekday — used whenever a test
// needs arming to be unconditional on whatever real instant it happens to
// run at.
func allDayEveryDay() []autoresponse.Window {
	ws := make([]autoresponse.Window, 0, 7)
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		ws = append(ws, autoresponse.Window{Weekday: wd, StartMin: 0, EndMin: 1440})
	}
	return ws
}

// setAutoResponse writes accountID's policy directly through the store,
// bypassing HTTP validation — some tests need a shape the HTTP layer itself
// would reject (e.g. enabled with zero windows) to isolate a single guard.
func setAutoResponse(t *testing.T, h *harness, accountID, orgID uuid.UUID, p autoresponse.Policy) {
	t.Helper()
	if err := h.store.UpsertAutoResponse(context.Background(), accountID, orgID, uuid.Nil, p); err != nil {
		t.Fatalf("UpsertAutoResponse: %v", err)
	}
}

func chatIDFor(t *testing.T, h *harness, channel string) string {
	t.Helper()
	for _, c := range h.listChats() {
		if channel == "" || c["channel"] == channel {
			return c["id"].(string)
		}
	}
	t.Fatalf("no chat found (channel=%q)", channel)
	return ""
}

// waitForArmedJob polls until chatID has exactly one pending auto-response
// job. Needed anywhere a test forces a job due right after seeing its draft:
// handleAIDraft commits the draft (making it visible via GET /ai-drafts)
// BEFORE it arms the job — two separate moments within the same handler run
// — so polling on the draft alone can observe a window where the draft
// exists but the job has not landed yet.
func waitForArmedJob(t *testing.T, h *harness, chatID string) {
	t.Helper()
	h.waitFor("a job to arm for chat "+chatID, func() bool {
		return h.countRows(`SELECT count(*) FROM xchats.auto_response_jobs WHERE chat_id = $1 AND state = 'pending'`, chatID) == 1
	})
}

func forceJobDue(t *testing.T, h *harness, chatID string) {
	t.Helper()
	if _, err := h.store.Pool().Exec(context.Background(),
		`UPDATE xchats.auto_response_jobs SET due_at = now() - interval '1 second' WHERE chat_id = $1 AND state = 'pending'`, chatID); err != nil {
		t.Fatalf("force job due: %v", err)
	}
}

func jobState(t *testing.T, h *harness, chatID string) (state, cancelReason string) {
	t.Helper()
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT state, cancel_reason FROM xchats.auto_response_jobs WHERE chat_id = $1 ORDER BY created_at DESC LIMIT 1`, chatID).
		Scan(&state, &cancelReason); err != nil {
		t.Fatalf("job state: %v", err)
	}
	return state, cancelReason
}

func unreadCountOf(t *testing.T, h *harness, chatID string) int {
	t.Helper()
	var n int
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT unread_count FROM xchats.wa_chats WHERE id = $1`, chatID).Scan(&n); err != nil {
		t.Fatalf("unread_count: %v", err)
	}
	return n
}

// scriptedLLMClient replies with a fixed, valid JSON contract and counts how
// many times Complete was invoked — the debounce test's proof that a burst
// of messages produced exactly one generation.
type scriptedLLMClient struct {
	calls   atomic.Int64
	reply   string
	failure error
}

func (c *scriptedLLMClient) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	c.calls.Add(1)
	if c.failure != nil {
		return llm.ChatResponse{}, c.failure
	}
	return llm.ChatResponse{Text: c.reply}, nil
}

const okReply = `{"reply_text":"Секунду.","reply_language":"ru","media_files_to_send":[],"escalate":false}`
const escalatingReply = `{"reply_text":"Нужен оператор.","reply_language":"ru","media_files_to_send":[],"escalate":true,"escalation_reason":"нестандартный вопрос"}`

// --- debounce ----------------------------------------------------------------

func TestDebounce_CoalescesBurstIntoOneGeneration(t *testing.T) {
	client := &scriptedLLMClient{reply: okReply}
	h := newHarnessWithLLM(t, client)
	h.worker.DebounceQuiet = 150 * time.Millisecond
	h.worker.DebounceCap = 3 * time.Second

	h.webhook(craftInbound(ownerJID, customerJID, "deb-1", "первое"))
	time.Sleep(50 * time.Millisecond)
	h.webhook(craftInbound(ownerJID, customerJID, "deb-2", "второе"))
	time.Sleep(50 * time.Millisecond)
	h.webhook(craftInbound(ownerJID, customerJID, "deb-3", "третье"))

	chatID := chatIDFor(t, h, "whatsapp")
	h.waitForDraftCount(chatID, 1)

	if n := client.calls.Load(); n != 1 {
		t.Fatalf("LLM Complete called %d times for a 3-message burst, want exactly 1", n)
	}
	msgs := h.listMessages(chatID)
	inCount := 0
	for _, m := range msgs {
		if m["direction"] == "in" {
			inCount++
		}
	}
	if inCount != 3 {
		t.Fatalf("stored inbound messages = %d, want 3 (debounce must not drop any)", inCount)
	}
}

func TestDebounce_SurvivesRestartViaSweep(t *testing.T) {
	h := newHarness(t)
	// Long enough that it will not fire on its own during this test — the
	// row is what has to carry it, not the in-process timer.
	h.worker.DebounceQuiet = 10 * time.Second
	h.worker.DebounceCap = 30 * time.Second

	h.webhook(craftInbound(ownerJID, customerJID, "restart-1", "привет"))
	chatID := chatIDFor(t, h, "whatsapp")

	if n := h.countRows(`SELECT count(*) FROM xchats.chat_draft_debounce WHERE chat_id = $1`, chatID); n != 1 {
		t.Fatalf("debounce row = %d, want 1 (still pending, quiet period not elapsed)", n)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.ai_drafts WHERE chat_id = $1`, chatID); n != 0 {
		t.Fatalf("a draft exists before the quiet period elapsed: %d", n)
	}

	// Force the row due — as if a restart happened well after the message
	// arrived — and drive recovery through the sweep, not the timer.
	if _, err := h.store.Pool().Exec(context.Background(),
		`UPDATE xchats.chat_draft_debounce SET generate_after = now() - interval '1 second' WHERE chat_id = $1`, chatID); err != nil {
		t.Fatalf("force debounce due: %v", err)
	}
	n, err := h.worker.RunDebounceSweep(context.Background(), 10)
	if err != nil {
		t.Fatalf("RunDebounceSweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("RunDebounceSweep fired %d rows, want 1", n)
	}
	h.waitForDraftCount(chatID, 1)
	if n := h.countRows(`SELECT count(*) FROM xchats.chat_draft_debounce WHERE chat_id = $1`, chatID); n != 0 {
		t.Fatalf("debounce row still present after it fired: %d", n)
	}
}

func TestHandleSuggest_NeverArmsAutoResponse(t *testing.T) {
	h := newHarness(t)
	h.webhook(craftInbound(ownerJID, customerJID, "sug-1", "привет"))
	chatID := chatIDFor(t, h, "whatsapp")
	h.waitForDraftCount(chatID, 1)

	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, Windows: allDayEveryDay(),
	})

	// Manual regenerate publishes KindAIDraft directly (FromDebounce=false);
	// h.postJSON's queue.Wait() fully drains that synchronous decision, so no
	// poll is needed to know arming did or didn't happen.
	h.postJSON("/xchats/api/v1/chats/"+chatID+"/ai-drafts?force=true", map[string]any{})
	if n := h.countRows(`SELECT count(*) FROM xchats.auto_response_jobs WHERE chat_id = $1`, chatID); n != 0 {
		t.Fatalf("handleSuggest armed an auto-response job: %d rows", n)
	}
}

// --- arming --------------------------------------------------------------

func TestAutoResponse_ArmsOnlyWhenScheduleCovers(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UTC()

	// A schedule covering tomorrow's weekday, never today's, is guaranteed to
	// exclude "now" regardless of when this test actually runs.
	excluded := []autoresponse.Window{{Weekday: (now.Weekday() + 1) % 7, StartMin: 0, EndMin: 1440}}
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, Windows: excluded,
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "arm-out-1", "вне графика", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	h.waitForDraftCount(chatID, 1)
	if n := h.countRows(`SELECT count(*) FROM xchats.auto_response_jobs WHERE chat_id = $1`, chatID); n != 0 {
		t.Fatalf("job armed outside the configured schedule: %d rows", n)
	}

	covered := []autoresponse.Window{{Weekday: now.Weekday(), StartMin: 0, EndMin: 1440}}
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, Windows: covered,
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "arm-in-1", "в графике", time.Now()))
	h.waitForDraftCount(chatID, 1)
	h.waitFor("a job to arm inside the configured schedule", func() bool {
		return h.countRows(`SELECT count(*) FROM xchats.auto_response_jobs WHERE chat_id = $1 AND state = 'pending'`, chatID) == 1
	})
}

// --- sending ---------------------------------------------------------------

func TestAutoResponse_SendsAfterDelayFlipsDraftAndPreservesUnread(t *testing.T) {
	h := newHarness(t)
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, Windows: allDayEveryDay(),
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "send-1", "когда доставка?", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	drafts := h.waitForDraftCount(chatID, 1)
	draftID := drafts[0]["id"].(string)
	waitForArmedJob(t, h, chatID)

	if got := unreadCountOf(t, h, chatID); got == 0 {
		t.Fatalf("unread_count = %d right after the inbound message, want > 0", got)
	}
	unreadBefore := unreadCountOf(t, h, chatID)

	forceJobDue(t, h, chatID)
	n, err := h.worker.ProcessDueAutoResponseJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("ProcessDueAutoResponseJobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("ProcessDueAutoResponseJobs claimed %d jobs, want 1", n)
	}
	h.queue.Wait() // drain the resulting KindOutboundSend

	state, _ := jobState(t, h, chatID)
	if state != "sent" {
		t.Fatalf("job state = %q, want sent", state)
	}
	var draftState string
	var sentMsgID *string
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT draft_state, sent_message_id FROM xchats.ai_drafts WHERE id = $1`, draftID).
		Scan(&draftState, &sentMsgID); err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if draftState != "sent" || sentMsgID == nil {
		t.Fatalf("draft_state=%q sent_message_id=%v, want sent + non-nil", draftState, sentMsgID)
	}

	calls := h.fake.CallsFor("sendText")
	if len(calls) != 1 {
		t.Fatalf("fake sendText calls = %d, want 1", len(calls))
	}
	if calls[0].Number != customerNum {
		t.Fatalf("sent to %q, want %q", calls[0].Number, customerNum)
	}
	if !aiMessageExists(t, h, chatID) {
		t.Fatal("auto-sent message was not recorded as sender_kind='ai'")
	}

	if got := unreadCountOf(t, h, chatID); got != unreadBefore {
		t.Fatalf("unread_count changed from %d to %d — an unattended auto-reply must not clear it", unreadBefore, got)
	}
}

func TestAutoResponse_CancelledByOperatorReply(t *testing.T) {
	h := newHarness(t)
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, Windows: allDayEveryDay(),
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "cancel-reply-1", "привет", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	h.waitForDraftCount(chatID, 1)
	waitForArmedJob(t, h, chatID)

	resp, _ := h.postJSON("/xchats/api/v1/chats/"+chatID+"/messages", map[string]any{"text": "оператор уже здесь"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manual send status = %d, want 200", resp.StatusCode)
	}

	state, reason := jobState(t, h, chatID)
	if state != "cancelled" || reason != "operator_reply" {
		t.Fatalf("job = (%q,%q), want (cancelled, operator_reply)", state, reason)
	}

	// Even if a sweep somehow still tried, a cancelled job cannot be reclaimed.
	forceJobDue(t, h, chatID) // no-op: WHERE state='pending' matches nothing
	n, err := h.worker.ProcessDueAutoResponseJobs(context.Background(), 10)
	if err != nil || n != 0 {
		t.Fatalf("ProcessDueAutoResponseJobs claimed %d jobs (err=%v), want 0 — a cancelled job must never be claimed", n, err)
	}
	if calls := h.fake.CallsFor("sendText"); len(calls) != 1 || calls[0].Text != "оператор уже здесь" {
		t.Fatalf("fake sendText calls = %+v, want exactly the operator's own message", calls)
	}
}

func TestAutoResponse_CancelledByManualApprove(t *testing.T) {
	h := newHarness(t)
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, Windows: allDayEveryDay(),
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "cancel-approve-1", "сколько стоит?", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	drafts := h.waitForDraftCount(chatID, 1)
	draftID := drafts[0]["id"].(string)
	waitForArmedJob(t, h, chatID)

	resp, _ := h.postJSON("/xchats/api/v1/ai-drafts/"+draftID+"/approve", map[string]any{"media_ids": []string{}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", resp.StatusCode)
	}

	state, reason := jobState(t, h, chatID)
	if state != "cancelled" || reason != "approved" {
		t.Fatalf("job = (%q,%q), want (cancelled, approved)", state, reason)
	}
	if calls := h.fake.CallsFor("sendText"); len(calls) != 1 {
		t.Fatalf("fake sendText calls = %d, want exactly 1 (the approved send)", len(calls))
	}
}

// --- escalation --------------------------------------------------------------

func TestAutoResponse_SkipsModelEscalationWhenConfigured(t *testing.T) {
	h := newHarnessWithLLM(t, &scriptedLLMClient{reply: escalatingReply})
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, SkipWhenEscalated: true, Windows: allDayEveryDay(),
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "esc-model-1", "сложный вопрос", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	h.waitForDraftCount(chatID, 1)
	waitForArmedJob(t, h, chatID)
	forceJobDue(t, h, chatID)

	n, err := h.worker.ProcessDueAutoResponseJobs(context.Background(), 10)
	if err != nil || n != 1 {
		t.Fatalf("ProcessDueAutoResponseJobs: n=%d err=%v", n, err)
	}
	h.queue.Wait()

	state, reason := jobState(t, h, chatID)
	if state != "cancelled" || reason != "escalated" {
		t.Fatalf("job = (%q,%q), want (cancelled, escalated)", state, reason)
	}
	if calls := h.fake.CallsFor("sendText"); len(calls) != 0 {
		t.Fatalf("fake sendText calls = %d, want 0 — an escalated draft must never auto-send", len(calls))
	}
}

func TestAutoResponse_EngineErrorEscalationNeverSendsEvenWithSkipFalse(t *testing.T) {
	h := newHarnessWithLLM(t, &scriptedLLMClient{failure: errBoom{}})
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, SkipWhenEscalated: false, Windows: allDayEveryDay(),
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "esc-engine-1", "привет", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	h.waitForDraftCount(chatID, 1)
	waitForArmedJob(t, h, chatID)
	forceJobDue(t, h, chatID)

	n, err := h.worker.ProcessDueAutoResponseJobs(context.Background(), 10)
	if err != nil || n != 1 {
		t.Fatalf("ProcessDueAutoResponseJobs: n=%d err=%v", n, err)
	}
	h.queue.Wait()

	state, reason := jobState(t, h, chatID)
	if state != "cancelled" || reason != "escalated" {
		t.Fatalf("job = (%q,%q), want (cancelled, escalated) — an engine_error holding draft must never send even with skip_when_escalated=false", state, reason)
	}
	if calls := h.fake.CallsFor("sendText"); len(calls) != 0 {
		t.Fatalf("fake sendText calls = %d, want 0", len(calls))
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom: llm unavailable" }

// --- fixed mode --------------------------------------------------------------

func TestAutoResponse_FixedModeSendsFixedTextAndSupersedesDraft(t *testing.T) {
	h := newHarness(t)
	const fixedText = "Сейчас нерабочее время, мы ответим утром."
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeFixed, FixedText: fixedText, Timezone: "UTC",
		DelaySeconds: 60, Windows: allDayEveryDay(),
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "fixed-1", "есть скидки?", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	drafts := h.waitForDraftCount(chatID, 1)
	draftID := drafts[0]["id"].(string)
	waitForArmedJob(t, h, chatID)

	forceJobDue(t, h, chatID)
	n, err := h.worker.ProcessDueAutoResponseJobs(context.Background(), 10)
	if err != nil || n != 1 {
		t.Fatalf("ProcessDueAutoResponseJobs: n=%d err=%v", n, err)
	}
	h.queue.Wait()

	var draftState string
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT draft_state FROM xchats.ai_drafts WHERE id = $1`, draftID).Scan(&draftState); err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if draftState != "superseded" {
		t.Fatalf("draft_state = %q, want superseded (never sent — the fixed text is what actually went out)", draftState)
	}

	calls := h.fake.CallsFor("sendText")
	if len(calls) != 1 || calls[0].Text != fixedText {
		t.Fatalf("fake sendText calls = %+v, want exactly one call with the fixed text %q", calls, fixedText)
	}
}

// --- reaper --------------------------------------------------------------

func TestAutoResponse_ReaperFailsStaleClaimsWithNoCustomerMessage(t *testing.T) {
	h := newHarness(t)
	setAutoResponse(t, h, h.accountID, h.orgID, autoresponse.Policy{
		Enabled: true, ReplyMode: autoresponse.ReplyModeAI, Timezone: "UTC",
		DelaySeconds: 60, Windows: allDayEveryDay(),
	})
	h.webhook(craftInboundAt(ownerJID, customerJID, "reap-1", "привет", time.Now()))
	chatID := chatIDFor(t, h, "whatsapp")
	h.waitForDraftCount(chatID, 1)
	waitForArmedJob(t, h, chatID)

	// Simulate a worker that claimed the job and then died before sending.
	if _, err := h.store.Pool().Exec(context.Background(),
		`UPDATE xchats.auto_response_jobs SET state = 'claimed', updated_at = now() - interval '10 minutes' WHERE chat_id = $1`, chatID); err != nil {
		t.Fatalf("simulate stale claim: %v", err)
	}

	n, err := h.store.ReapStaleAutoResponseClaims(context.Background(), 5*time.Minute)
	if err != nil {
		t.Fatalf("ReapStaleAutoResponseClaims: %v", err)
	}
	if n != 1 {
		t.Fatalf("reaped %d jobs, want 1", n)
	}
	state, _ := jobState(t, h, chatID)
	if state != "failed" {
		t.Fatalf("job state = %q, want failed", state)
	}
	if calls := h.fake.CallsFor("sendText"); len(calls) != 0 {
		t.Fatalf("fake sendText calls = %d, want 0 — claimed must imply nothing was ever written", len(calls))
	}
}

// --- HTTP: PUT /accounts/:id/auto-response ------------------------------------

func TestPutAutoResponse_RoundTripsIdempotentlyAndValidates(t *testing.T) {
	h := newHarness(t)

	body := map[string]any{
		"enabled": true, "reply_mode": "ai", "timezone": "Asia/Almaty",
		"delay_seconds": 120, "cooldown_seconds": 300,
		"skip_when_escalated": true, "pause_when_assigned": false,
		"windows": []map[string]any{
			{"weekday": 1, "start": "18:00", "end": "09:00"}, // Mon, outside 09:00-18:00 -> splits into 2 rows
			{"weekday": 6, "start": "00:00", "end": "24:00"}, // Sat, all day
		},
	}
	path := "/xchats/api/v1/accounts/" + h.accountID.String() + "/auto-response"

	resp, env := h.putJSON(path, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200: %v", resp.StatusCode, env)
	}
	var out struct {
		AccountID    string `json:"account_id"`
		AutoResponse struct {
			Enabled      bool   `json:"enabled"`
			ReplyMode    string `json:"reply_mode"`
			Timezone     string `json:"timezone"`
			DelaySeconds int    `json:"delay_seconds"`
			Windows      []struct {
				Weekday int    `json:"weekday"`
				Start   string `json:"start"`
				End     string `json:"end"`
			} `json:"windows"`
		} `json:"auto_response"`
	}
	mustPayload(t, env, &out)
	if out.AccountID != h.accountID.String() {
		t.Fatalf("account_id = %q, want %q", out.AccountID, h.accountID.String())
	}
	if !out.AutoResponse.Enabled || out.AutoResponse.ReplyMode != "ai" || out.AutoResponse.Timezone != "Asia/Almaty" {
		t.Fatalf("unexpected policy echoed back: %+v", out.AutoResponse)
	}
	if len(out.AutoResponse.Windows) != 3 {
		t.Fatalf("windows = %d, want 3 (Monday's wrap splits into 2 + Saturday's 1)", len(out.AutoResponse.Windows))
	}

	// Idempotent: PUT the identical body again and expect the identical shape.
	resp2, env2 := h.putJSON(path, body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second PUT status = %d, want 200", resp2.StatusCode)
	}
	var out2 struct {
		AutoResponse struct {
			Windows []struct {
				Weekday int    `json:"weekday"`
				Start   string `json:"start"`
				End     string `json:"end"`
			} `json:"windows"`
		} `json:"auto_response"`
	}
	mustPayload(t, env2, &out2)
	if len(out2.AutoResponse.Windows) != len(out.AutoResponse.Windows) {
		t.Fatalf("second PUT windows = %d, want %d (idempotent)", len(out2.AutoResponse.Windows), len(out.AutoResponse.Windows))
	}

	// GET /accounts reflects the same policy.
	var accts struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/accounts", &accts)
	found := false
	for _, a := range accts.Items {
		if a["id"] == h.accountID.String() {
			found = true
			ar, _ := a["auto_response"].(map[string]any)
			if ar == nil || ar["enabled"] != true {
				t.Fatalf("GET /accounts auto_response not reflected: %v", a["auto_response"])
			}
		}
	}
	if !found {
		t.Fatal("seeded account missing from GET /accounts")
	}

	// Bad timezone -> 400.
	bad := map[string]any{}
	for k, v := range body {
		bad[k] = v
	}
	bad["timezone"] = "Not/AZone"
	resp3, env3 := h.putJSON(path, bad)
	if resp3.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad timezone status = %d, want 400: %v", resp3.StatusCode, env3)
	}

	// enabled with zero windows -> 400.
	noWindows := map[string]any{}
	for k, v := range body {
		noWindows[k] = v
	}
	noWindows["windows"] = []map[string]any{}
	resp4, _ := h.putJSON(path, noWindows)
	if resp4.StatusCode != http.StatusBadRequest {
		t.Fatalf("enabled-without-windows status = %d, want 400", resp4.StatusCode)
	}

	// Cross-org: an account belonging to a different org 404s.
	foreignOrg, err := h.store.SeedOrganization(context.Background(), "xchats-foreign-autoresponse")
	if err != nil {
		t.Fatalf("seed foreign org: %v", err)
	}
	foreignAccountID := uuid.New()
	if _, err := h.store.SeedAccount(context.Background(), store.Account{
		ID:                 foreignAccountID,
		OrganizationID:     uuid.NullUUID{UUID: foreignOrg.ID, Valid: true},
		DisplayName:        "Foreign",
		ExternalAccountRef: "foreign-jid@s.whatsapp.net",
		ExternalHandle:     "70000000000",
		InstanceName:       "foreign-instance",
		ConnectionState:    "connected",
	}); err != nil {
		t.Fatalf("seed foreign account: %v", err)
	}
	resp5, _ := h.putJSON("/xchats/api/v1/accounts/"+foreignAccountID.String()+"/auto-response", body)
	if resp5.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org PUT status = %d, want 404", resp5.StatusCode)
	}
}
