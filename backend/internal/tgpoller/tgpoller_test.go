package tgpoller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
)

// --- fakes -------------------------------------------------------------

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// noopSleep never actually waits — every backoff/retry delay in these tests
// is instant. It is still safe for cancellation: Manager.sleep re-checks
// ctx.Err() immediately after calling it, and that reflects real,
// concurrently-set cancellation regardless of how fast Sleep itself returns.
func noopSleep(context.Context, time.Duration) {}

type fakeTokens struct {
	mu     sync.Mutex
	tokens map[uuid.UUID]string
}

func newFakeTokens() *fakeTokens { return &fakeTokens{tokens: map[uuid.UUID]string{}} }

func (f *fakeTokens) set(id uuid.UUID, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[id] = token
}

func (f *fakeTokens) TelegramBotToken(_ context.Context, id uuid.UUID) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tokens[id]; ok {
		return t, nil
	}
	return "", store.ErrNotFound
}

type advanceCall struct {
	AccountID uuid.UUID
	UpdateID  int64
}

type fakeOffsets struct {
	mu       sync.Mutex
	vals     map[uuid.UUID]int64
	has      map[uuid.UUID]bool
	advances []advanceCall
}

func newFakeOffsets() *fakeOffsets {
	return &fakeOffsets{vals: map[uuid.UUID]int64{}, has: map[uuid.UUID]bool{}}
}

func (f *fakeOffsets) TelegramPollOffset(_ context.Context, id uuid.UUID) (int64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.vals[id], f.has[id], nil
}

func (f *fakeOffsets) AdvanceTelegramPollOffset(_ context.Context, id uuid.UUID, updateID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advances = append(f.advances, advanceCall{id, updateID})
	if !f.has[id] || updateID > f.vals[id] {
		f.vals[id] = updateID
		f.has[id] = true
	}
	return nil
}

func (f *fakeOffsets) advancesFor(id uuid.UUID) []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []int64
	for _, a := range f.advances {
		if a.AccountID == id {
			out = append(out, a.UpdateID)
		}
	}
	return out
}

type fakeProcessor struct {
	mu        sync.Mutex
	calls     []int64
	failTimes map[int64]int
}

func newFakeProcessor() *fakeProcessor { return &fakeProcessor{failTimes: map[int64]int{}} }

func (f *fakeProcessor) Process(_ context.Context, _ store.TelegramAccount, upd *telegram.Update, _ []byte) (tgingest.Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, upd.UpdateID)
	if n := f.failTimes[upd.UpdateID]; n > 0 {
		f.failTimes[upd.UpdateID] = n - 1
		return tgingest.Outcome{}, errors.New("fake processor: forced failure")
	}
	return tgingest.Outcome{Status: "stored", Result: store.TgInboundResult{MessageInserted: true}}, nil
}

type stateCall struct {
	AccountID uuid.UUID
	State     store.TelegramWebhookState
}

type fakeState struct {
	mu    sync.Mutex
	calls []stateCall
}

func (f *fakeState) SetTelegramWebhookState(_ context.Context, id uuid.UUID, st store.TelegramWebhookState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, stateCall{id, st})
	return nil
}

func (f *fakeState) statesFor(id uuid.UUID) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if c.AccountID == id {
			out = append(out, c.State.State)
		}
	}
	return out
}

type fakeBots struct {
	mu   sync.Mutex
	bots []store.TelegramPollBot
}

func (f *fakeBots) ListTelegramPollBots(context.Context) ([]store.TelegramPollBot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.TelegramPollBot(nil), f.bots...), nil
}

// --- fixtures ------------------------------------------------------------

func testBotSpec(accountID uuid.UUID, botID int64, tokenVersion string) store.TelegramPollBot {
	return store.TelegramPollBot{
		Account:      store.TelegramAccount{ID: accountID, BotID: botID, BotUsername: fmt.Sprintf("bot_%d", botID), ConnectionState: "connecting"},
		TokenVersion: tokenVersion,
	}
}

func testConfig() PollConfig {
	return PollConfig{
		TimeoutSeconds: 1, ConflictBackoff: time.Millisecond,
		MinBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond,
		MaxConsecutiveFailures: 3,
		DefaultRetryAfter:      time.Millisecond, MinRetryAfter: time.Millisecond, MaxRetryAfter: 10 * time.Millisecond,
	}
}

func rawUpdateFor(updateID, userID, msgID int64, text string) telegram.RawUpdate {
	u := telegram.Update{UpdateID: updateID, Message: &telegram.Message{
		MessageID: msgID, From: &telegram.User{ID: userID}, Chat: &telegram.Chat{ID: userID, Type: "private"},
		Text: text, Date: time.Now().Unix(),
	}}
	return telegram.RawFrom(u)
}

// waitFor polls cond until it's true or the deadline expires, failing the
// test on timeout — the standard shape for asserting on a background
// goroutine's eventual, non-deterministically-timed effect.
func waitFor(t *testing.T, deadline time.Duration, cond func() bool) {
	t.Helper()
	stop := time.Now().Add(deadline)
	for time.Now().Before(stop) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition not met within %s", deadline)
	}
}

// --- tests -----------------------------------------------------------------

func TestDeleteWebhookBeforeFirstPoll(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	tg := telegram.NewFake(1, "bot")
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}
	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})

	m.Upsert(testBotSpec(accountID, 1, "v1"))
	waitFor(t, time.Second, func() bool { return len(tg.CallsFor("deleteWebhook")) > 0 })
	m.Close()

	calls := tg.CallsFor("deleteWebhook")
	if len(calls) == 0 {
		t.Fatal("deleteWebhook was never called")
	}
	if calls[0].DropPending {
		t.Fatal("deleteWebhook called with drop_pending_updates=true, want false")
	}
	// deleteWebhook must precede getUpdates.
	getUpdatesCalls := tg.CallsFor("getUpdates")
	if len(getUpdatesCalls) == 0 {
		t.Fatal("getUpdates was never called")
	}
}

func TestInOrderAdvancement(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	tg := telegram.NewFake(1, "bot")
	var call int32
	tg.GetUpdatesFn = func(ctx context.Context, _ string, req telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		n := atomic.AddInt32(&call, 1)
		if n == 1 {
			if req.Offset != 0 {
				t.Errorf("first call offset = %d, want 0 (never polled before)", req.Offset)
			}
			return []telegram.RawUpdate{rawUpdateFor(10, 1, 1, "a"), rawUpdateFor(11, 1, 2, "b"), rawUpdateFor(12, 1, 3, "c")}, nil
		}
		if n == 2 && req.Offset != 13 {
			t.Errorf("second call offset = %d, want 13 (12+1)", req.Offset)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}

	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})
	m.Upsert(testBotSpec(accountID, 1, "v1"))
	waitFor(t, time.Second, func() bool { return len(offsets.advancesFor(accountID)) >= 3 })
	m.Close()

	got := offsets.advancesFor(accountID)
	want := []int64{10, 11, 12}
	if len(got) < 3 {
		t.Fatalf("advances = %v, want at least %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("advances = %v, want ascending %v", got, want)
		}
	}
}

func TestProcessorFailureRefetchAdvancesExactlyOnce(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, state := newFakeOffsets(), &fakeState{}
	proc := newFakeProcessor()
	proc.failTimes[11] = 1 // update 11 fails once, then succeeds on refetch

	tg := telegram.NewFake(1, "bot")
	var call int32
	tg.GetUpdatesFn = func(ctx context.Context, _ string, req telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		n := atomic.AddInt32(&call, 1)
		switch n {
		case 1:
			return []telegram.RawUpdate{rawUpdateFor(10, 1, 1, "a"), rawUpdateFor(11, 1, 2, "b")}, nil
		case 2:
			// The batch broke at 11 (10 already advanced) — refetch must
			// start from 11 again, not 12.
			if req.Offset != 11 {
				t.Errorf("refetch offset = %d, want 11 (the failed update, not skipped)", req.Offset)
			}
			return []telegram.RawUpdate{rawUpdateFor(11, 1, 2, "b")}, nil
		default:
			<-ctx.Done()
			return nil, ctx.Err()
		}
	}

	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})
	m.Upsert(testBotSpec(accountID, 1, "v1"))
	waitFor(t, time.Second, func() bool {
		advances := offsets.advancesFor(accountID)
		return len(advances) >= 2 && advances[len(advances)-1] == 11
	})
	m.Close()

	advances := offsets.advancesFor(accountID)
	n11 := 0
	for _, a := range advances {
		if a == 11 {
			n11++
		}
	}
	if n11 != 1 {
		t.Fatalf("update 11 advanced %d times, want exactly 1 (the failed attempt must not advance)", n11)
	}
}

func Test401StopsAndOnlyRevivesOnNewTokenVersion(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "bad-token")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	tg := telegram.NewFake(1, "bot")
	tg.BadTokens["bad-token"] = true // deleteWebhook AND getUpdates both 401

	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})
	m.Upsert(testBotSpec(accountID, 1, "v1"))

	waitFor(t, time.Second, func() bool {
		_, stopped := m.Status(accountID)
		return stopped
	})
	registered, stopped := m.Status(accountID)
	if !registered || !stopped {
		t.Fatalf("Status = registered=%v stopped=%v, want true true", registered, stopped)
	}
	if states := state.statesFor(accountID); len(states) == 0 || states[len(states)-1] != "token_error" {
		t.Fatalf("recorded states = %v, want the last one to be token_error", states)
	}

	// Same version: must NOT restart (the bad token is still bad).
	before, _ := m.Status(accountID)
	m.Upsert(testBotSpec(accountID, 1, "v1"))
	time.Sleep(20 * time.Millisecond)
	after, afterStopped := m.Status(accountID)
	if !before || !after || !afterStopped {
		t.Fatalf("a same-version Upsert over a stopped runner changed state: before=%v after=%v stopped=%v", before, after, afterStopped)
	}

	// New version: must revive with a fresh runner.
	tokens.set(accountID, "good-token")
	m.Upsert(testBotSpec(accountID, 1, "v2"))
	waitFor(t, time.Second, func() bool {
		_, nowStopped := m.Status(accountID)
		return !nowStopped
	})
	m.Close()
}

func Test409TriggersReDeleteAndBackoff(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	tg := telegram.NewFake(1, "bot")
	var getUpdatesCalls int32
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		n := atomic.AddInt32(&getUpdatesCalls, 1)
		if n == 1 {
			return nil, &telegram.APIError{Code: 409, Description: "Conflict: terminated by other getUpdates request"}
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}

	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})
	m.Upsert(testBotSpec(accountID, 1, "v1"))
	waitFor(t, time.Second, func() bool {
		states := state.statesFor(accountID)
		return len(states) > 0 && states[len(states)-1] == "webhook_error"
	})

	// Assert liveness BEFORE Close — Close unregisters everything by
	// design, so checking Status afterward would always read registered=false.
	registered, stopped := m.Status(accountID)
	if !registered || stopped {
		t.Fatalf("a 409 must not permanently stop the runner: registered=%v stopped=%v", registered, stopped)
	}
	m.Close()

	// deleteWebhook must have been called again after the 409 (at least
	// twice total: once before the first poll, once more to recover).
	if n := len(tg.CallsFor("deleteWebhook")); n < 2 {
		t.Fatalf("deleteWebhook called %d times, want >= 2 (once at start, once after the 409)", n)
	}
}

func Test429HonorsRetryAfter(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	var sleptFor []time.Duration
	var mu sync.Mutex
	captureSleep := func(_ context.Context, d time.Duration) {
		mu.Lock()
		sleptFor = append(sleptFor, d)
		mu.Unlock()
	}

	tg := telegram.NewFake(1, "bot")
	var n int32
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		if atomic.AddInt32(&n, 1) == 1 {
			return nil, &telegram.APIError{Code: 429, Description: "Too Many Requests", RetryAfter: 2}
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}

	// testConfig's MaxRetryAfter (10ms) exists to keep OTHER tests' backoffs
	// fast; this test asserts the exact pass-through value, so it needs a
	// ceiling well above the 2s retry_after it's proving isn't clamped away.
	cfg := testConfig()
	cfg.MaxRetryAfter = 5 * time.Second
	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: captureSleep, Poll: cfg})
	m.Upsert(testBotSpec(accountID, 1, "v1"))
	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(sleptFor) > 0
	})
	m.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(sleptFor) == 0 {
		t.Fatal("no sleep observed after a 429")
	}
	if sleptFor[0] != 2*time.Second {
		t.Fatalf("slept %s after a 429 with retry_after=2, want exactly 2s (Telegram's own value passed through)", sleptFor[0])
	}
}

func Test429WithoutRetryAfterUsesDefaultClamped(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	var sleptFor []time.Duration
	var mu sync.Mutex
	captureSleep := func(_ context.Context, d time.Duration) {
		mu.Lock()
		sleptFor = append(sleptFor, d)
		mu.Unlock()
	}
	tg := telegram.NewFake(1, "bot")
	var n int32
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		if atomic.AddInt32(&n, 1) == 1 {
			return nil, &telegram.APIError{Code: 429, Description: "Too Many Requests"} // no retry_after
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	cfg := testConfig()
	cfg.DefaultRetryAfter = 3 * time.Second
	cfg.MaxRetryAfter = 10 * time.Second
	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: captureSleep, Poll: cfg})
	m.Upsert(testBotSpec(accountID, 1, "v1"))
	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(sleptFor) > 0
	})
	m.Close()

	mu.Lock()
	defer mu.Unlock()
	if sleptFor[0] != 3*time.Second {
		t.Fatalf("slept %s with no retry_after present, want the configured default 3s", sleptFor[0])
	}
}

func TestBoundedNetworkBackoff(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	tg := telegram.NewFake(1, "bot")
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		return nil, errors.New("connection reset by peer") // an unclassified, persistent failure
	}

	var sleptFor []time.Duration
	var mu sync.Mutex
	captureSleep := func(_ context.Context, d time.Duration) {
		mu.Lock()
		sleptFor = append(sleptFor, d)
		mu.Unlock()
	}
	cfg := testConfig()
	cfg.MinBackoff, cfg.MaxBackoff = time.Millisecond, 8*time.Millisecond
	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: captureSleep, Poll: cfg})
	m.Upsert(testBotSpec(accountID, 1, "v1"))
	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(sleptFor) >= 6
	})
	m.Close()

	mu.Lock()
	defer mu.Unlock()
	for i, d := range sleptFor {
		if d < cfg.MinBackoff || d > cfg.MaxBackoff {
			t.Fatalf("sleep[%d] = %s, want within [%s, %s]", i, d, cfg.MinBackoff, cfg.MaxBackoff)
		}
	}
	// After MaxConsecutiveFailures (3), webhook_error must be recorded.
	waitFor(t, time.Second, func() bool {
		states := state.statesFor(accountID)
		return len(states) > 0 && states[len(states)-1] == "webhook_error"
	})
}

func TestCancellationDuringBlockedLongPoll(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	entered := make(chan struct{})
	tg := telegram.NewFake(1, "bot")
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		safeClose(entered)
		<-ctx.Done() // simulates a genuinely blocked long-poll HTTP call
		return nil, ctx.Err()
	}

	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})
	m.Upsert(testBotSpec(accountID, 1, "v1"))

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("GetUpdates was never entered")
	}

	done := make(chan struct{})
	go func() { m.Remove(accountID); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Remove did not return promptly after cancellation of a blocked long poll")
	}
	if registered, _ := m.Status(accountID); registered {
		t.Fatal("account still registered after Remove")
	}
}

// safeClose avoids a double-close panic if a slow-scheduled goroutine somehow
// re-enters GetUpdatesFn after Close (defensive; should not happen given
// the single-poller invariant, but a flaky double-close must never panic
// the whole test binary).
func safeClose(ch chan struct{}) {
	defer func() { recover() }()
	close(ch)
}

func TestTokenReplaceNoOverlap(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}

	var gauge int32
	var maxGauge int32
	tg := telegram.NewFake(1, "bot")
	tg.GetUpdatesFn = func(ctx context.Context, token string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		n := atomic.AddInt32(&gauge, 1)
		for {
			cur := atomic.LoadInt32(&maxGauge)
			if n <= cur || atomic.CompareAndSwapInt32(&maxGauge, cur, n) {
				break
			}
		}
		defer atomic.AddInt32(&gauge, -1)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil, nil
		}
	}

	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})

	m.Upsert(testBotSpec(accountID, 1, "v1"))
	time.Sleep(15 * time.Millisecond) // let it get into a getUpdates call
	tokens.set(accountID, "tok-2")
	m.Upsert(testBotSpec(accountID, 1, "v2")) // Upsert BLOCKS until the old runner has fully exited
	time.Sleep(30 * time.Millisecond)
	m.Close()

	if got := atomic.LoadInt32(&maxGauge); got > 1 {
		t.Fatalf("max concurrent getUpdates callers = %d, want <= 1 (old and new runners overlapped)", got)
	}
}

func TestReconcileDiffAndConcurrentCalls(t *testing.T) {
	a1, a2, a3 := uuid.New(), uuid.New(), uuid.New()
	tokens := newFakeTokens()
	tokens.set(a1, "t1")
	tokens.set(a2, "t2")
	tokens.set(a3, "t3")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}
	tg := telegram.NewFake(1, "bot")
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})

	m.Reconcile([]store.TelegramPollBot{testBotSpec(a1, 1, "v1"), testBotSpec(a2, 2, "v1")})
	waitFor(t, time.Second, func() bool {
		r1, _ := m.Status(a1)
		r2, _ := m.Status(a2)
		return r1 && r2
	})

	// Concurrent Reconcile calls with overlapping/changing wants must never
	// panic or deadlock, and must converge on the final desired set.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		m.Reconcile([]store.TelegramPollBot{testBotSpec(a1, 1, "v1"), testBotSpec(a3, 3, "v1")})
	}()
	go func() {
		defer wg.Done()
		m.Reconcile([]store.TelegramPollBot{testBotSpec(a1, 1, "v1"), testBotSpec(a3, 3, "v1")})
	}()
	wg.Wait()

	waitFor(t, time.Second, func() bool {
		r1, _ := m.Status(a1)
		r2, _ := m.Status(a2)
		r3, _ := m.Status(a3)
		return r1 && !r2 && r3
	})
	m.Close()
}

func TestCloseWaits(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	tokens := newFakeTokens()
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}
	var active int32
	tg := telegram.NewFake(1, "bot")
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		atomic.AddInt32(&active, 1)
		defer atomic.AddInt32(&active, -1)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: &fakeBots{}, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})

	var specs []store.TelegramPollBot
	for i, id := range ids {
		tokens.set(id, fmt.Sprintf("tok-%d", i))
		specs = append(specs, testBotSpec(id, int64(i+1), "v1"))
	}
	m.Reconcile(specs)
	waitFor(t, time.Second, func() bool { return atomic.LoadInt32(&active) == int32(len(ids)) })

	m.Close() // must not return until every runner's goroutine has actually exited

	if got := atomic.LoadInt32(&active); got != 0 {
		t.Fatalf("%d runners still active immediately after Close returned", got)
	}
	for _, id := range ids {
		if registered, _ := m.Status(id); registered {
			t.Fatalf("account %s still registered after Close", id)
		}
	}
}

func TestStartReconcilesFromBotLister(t *testing.T) {
	accountID := uuid.New()
	tokens := newFakeTokens()
	tokens.set(accountID, "tok-1")
	offsets, proc, state := newFakeOffsets(), newFakeProcessor(), &fakeState{}
	tg := telegram.NewFake(1, "bot")
	tg.GetUpdatesFn = func(ctx context.Context, _ string, _ telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	bots := &fakeBots{bots: []store.TelegramPollBot{testBotSpec(accountID, 1, "v1")}}
	m := NewManager(Deps{TG: tg, Tokens: tokens, Offsets: offsets, Processor: proc, State: state,
		Bots: bots, Log: testLogger(), Sleep: noopSleep, Poll: testConfig()})

	m.Start(context.Background())
	waitFor(t, time.Second, func() bool {
		registered, _ := m.Status(accountID)
		return registered
	})
	m.Close()
}
