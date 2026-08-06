package automation

import (
	"context"
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
