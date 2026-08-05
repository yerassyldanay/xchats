package providerhealth

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/yerassyldanay/xchats/backend/llm"
)

var errBoom = errors.New("boom")
var errAuth = fmt.Errorf("wrapped: %w", llm.ErrProviderAuth)

func TestTracker_FirstReportAlwaysNotifies(t *testing.T) {
	var got []Status
	tr := NewTracker(func(s Status) { got = append(got, s) })

	tr.Report("openrouter", nil)
	if len(got) != 1 {
		t.Fatalf("onChange calls = %d, want 1 for the first-ever report", len(got))
	}
	if !got[0].Healthy || got[0].Provider != "openrouter" {
		t.Errorf("got %+v, want Provider=openrouter Healthy=true", got[0])
	}
}

func TestTracker_RepeatedSameStatusDoesNotRenotify(t *testing.T) {
	calls := 0
	tr := NewTracker(func(Status) { calls++ })

	tr.Report("openrouter", nil)
	tr.Report("openrouter", nil)
	tr.Report("openrouter", nil)
	if calls != 1 {
		t.Errorf("onChange calls = %d, want 1 (no change after the first report)", calls)
	}
}

func TestTracker_TransitionToUnhealthyNotifies(t *testing.T) {
	var got []Status
	tr := NewTracker(func(s Status) { got = append(got, s) })

	tr.Report("openrouter", nil)     // healthy
	tr.Report("openrouter", errAuth) // -> unhealthy: notify
	tr.Report("openrouter", errAuth) // still unhealthy: no notify
	tr.Report("openrouter", nil)     // -> healthy again: notify

	if len(got) != 3 {
		t.Fatalf("onChange calls = %d, want 3 (healthy, unhealthy, healthy)", len(got))
	}
	if got[1].Healthy {
		t.Errorf("2nd notification Healthy = true, want false")
	}
	if !got[2].Healthy {
		t.Errorf("3rd notification Healthy = false, want true")
	}
}

func TestTracker_NonAuthErrorsAreIgnoredEntirely(t *testing.T) {
	calls := 0
	tr := NewTracker(func(Status) { calls++ })

	tr.Report("openrouter", errBoom) // never reported healthy or unhealthy before — must stay unknown
	if calls != 0 {
		t.Fatalf("onChange calls = %d, want 0 — a non-auth error must never establish a status", calls)
	}
	if snap := tr.Snapshot(); len(snap) != 0 {
		t.Fatalf("Snapshot = %v, want empty — provider health is still unknown", snap)
	}
}

func TestTracker_NonAuthErrorDoesNotFlapAnEstablishedStatus(t *testing.T) {
	calls := 0
	tr := NewTracker(func(Status) { calls++ })

	tr.Report("openrouter", nil)     // healthy: notify (1)
	tr.Report("openrouter", errBoom) // a rate limit or network blip — must NOT flip healthy
	if calls != 1 {
		t.Fatalf("onChange calls = %d, want 1 — a non-auth error must not change an established status", calls)
	}
	snap := tr.Snapshot()
	if len(snap) != 1 || !snap[0].Healthy {
		t.Errorf("Snapshot = %+v, want exactly one healthy=true entry", snap)
	}
}

func TestTracker_SnapshotTracksEachProviderIndependently(t *testing.T) {
	tr := NewTracker(nil)
	tr.Report("openrouter", nil)
	tr.Report("openai", errAuth)

	snap := tr.Snapshot()
	byProvider := map[string]bool{}
	for _, s := range snap {
		byProvider[s.Provider] = s.Healthy
	}
	if len(snap) != 2 {
		t.Fatalf("Snapshot len = %d, want 2", len(snap))
	}
	if !byProvider["openrouter"] {
		t.Error("openrouter should be healthy")
	}
	if byProvider["openai"] {
		t.Error("openai should be unhealthy")
	}
}

func TestTracker_NilOnChangeIsSafe(t *testing.T) {
	tr := NewTracker(nil)
	tr.Report("openrouter", errAuth) // must not panic
	if snap := tr.Snapshot(); len(snap) != 1 || snap[0].Healthy {
		t.Errorf("Snapshot = %+v, want one unhealthy entry", snap)
	}
}

func TestTracker_ConcurrentReportsAreRace_Free(t *testing.T) {
	tr := NewTracker(func(Status) {})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				tr.Report("openrouter", nil)
			} else {
				tr.Report("openrouter", errAuth)
			}
		}(i)
		go func() { defer wg.Done(); tr.Snapshot() }()
	}
	wg.Wait()
}
