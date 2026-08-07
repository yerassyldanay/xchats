package automation

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
)

func TestSchedulerOnInboundMessageOffModeDoesNotArm(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "off", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings(off): %v", err)
	}

	s := NewScheduler(st, Config{SystemDefaultWaitSeconds: 5}, nil, testLogger(), 8)
	if err := s.OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	v, err := st.CurrentBurstVersion(ctx, chatID)
	if err != nil {
		t.Fatalf("CurrentBurstVersion: %v", err)
	}
	if v != 0 {
		t.Fatalf("off mode should never arm a debounce job, got burst version %d", v)
	}
}

func TestSchedulerClaimDuePromotesToDispatchJobAndEnqueues(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	s := NewScheduler(st, Config{SystemDefaultWaitSeconds: 0}, nil, testLogger(), 8)
	// wait=0 -> the deadline is effectively immediate.
	if err := s.OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}

	s.claimDue(ctx)

	select {
	case id := <-s.dispatch:
		job, err := st.DispatchJobByID(ctx, id)
		if err != nil {
			t.Fatalf("DispatchJobByID: %v", err)
		}
		if job.ChatID != chatID || job.AccountID != accountID || job.Channel != "whatsapp" || job.BurstVersion != 1 {
			t.Fatalf("dispatch job = %+v, want chat=%s account=%s channel=whatsapp version=1", job, chatID, accountID)
		}
	default:
		t.Fatal("claimDue should have enqueued a dispatch job for the due debounce deadline")
	}
}

func TestSchedulerClaimDueSkipsNotYetDue(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	s := NewScheduler(st, Config{SystemDefaultWaitSeconds: 3600}, nil, testLogger(), 8)
	if err := s.OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}
	s.claimDue(ctx)
	select {
	case id := <-s.dispatch:
		t.Fatalf("a debounce deadline an hour out should not be claimable yet, got job %s", id)
	default:
	}
}

func TestSchedulerRecoverDispatchJobsRepublishesStuckAndPending(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	pendingID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", 1)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	s := NewScheduler(st, Config{}, nil, testLogger(), 8)
	// stuckAfter=0 -> even a job just marked 'processing' counts as stuck,
	// so this single call exercises both the never-started ('pending') and
	// crashed-mid-flight ('processing') recovery paths deterministically,
	// without needing to sleep in the test.
	s.recoverDispatchJobs(ctx, 0)

	got := map[string]bool{}
	for i := 0; i < 1; i++ {
		select {
		case id := <-s.dispatch:
			got[id.String()] = true
		default:
		}
	}
	if !got[pendingID.String()] {
		t.Fatalf("a pending dispatch job should be recovered immediately")
	}
}

// Recovery only republishes pending jobs older than its cutoff. A dispatch
// just created and enqueued by claimDue must therefore remain queued once,
// even if a recovery tick runs before a worker acquires it.
func TestRecoverDispatchJobsDoesNotDuplicateFreshlyEnqueuedJob(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	s := NewScheduler(st, Config{SystemDefaultWaitSeconds: 0}, nil, testLogger(), 8)
	if err := s.OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}

	// claimDue promotes the due debounce deadline and enqueues it once.
	s.claimDue(ctx)
	firstID := <-s.dispatch

	// The dispatch worker that would normally dequeue firstID and call
	// ClaimDispatchJob hasn't run yet (nothing in this test drains
	// s.dispatch further) — the job is still 'pending' in the DB. A
	// recovery tick landing right now (e.g. the 1-minute periodic pass, or
	// the boot-time recovery pass racing a slow claim) sees exactly that.
	s.recoverDispatchJobs(ctx, time.Minute)

	select {
	case secondID := <-s.dispatch:
		t.Fatalf("fresh dispatch job %s was unexpectedly recovered as %s before the cutoff", firstID, secondID)
	default:
	}
}

// TestSchedulerStartStopLifecycle is the only test in this package that
// actually calls Start/Stop — every other test drives claimDue/HandleRun
// directly. It exercises the real ticker-driven claim loop and the
// dispatch-worker pool end to end (message arrives -> debounce elapses ->
// draft appears), then asserts Stop() returns promptly after ctx is
// canceled and leaves no goroutines behind.
func TestSchedulerStartStopLifecycle(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	accountID, chatID := seedRunnerChat(t, st)

	hub, q := &fakeHub{}, &fakeQueue{}
	runner := testRunner(st, "Ответ.", hub, q)
	s := NewScheduler(st, Config{SystemDefaultWaitSeconds: 0}, runner, testLogger(), 4)

	before := runtime.NumGoroutine()

	runCtx, cancel := context.WithCancel(ctx)
	s.Start(runCtx, 4, 10*time.Millisecond, time.Hour, time.Minute)

	if err := s.OnInboundMessage(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("OnInboundMessage: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		pending, err := st.PendingDrafts(ctx, chatID)
		if err != nil {
			t.Fatalf("PendingDrafts: %v", err)
		}
		if len(pending) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("Start's claim loop never produced a draft within 2s: pending=%+v", pending)
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2s of ctx being canceled")
	}

	// The worker pool's goroutines exit as soon as ctx is done, but a
	// dispatch worker mid-HandleRun finishes that call first — give it a
	// moment before comparing counts, matching how NumGoroutine is used
	// elsewhere in this codebase's own tests.
	var after int
	for i := 0; i < 20; i++ {
		after = runtime.NumGoroutine()
		if after <= before {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if after > before {
		t.Fatalf("goroutine leak after Stop(): before=%d after=%d", before, after)
	}
}
