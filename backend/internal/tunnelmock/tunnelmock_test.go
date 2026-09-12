package tunnelmock

import (
	"context"
	"sync"
	"testing"
)

func TestStartStopStatusTransitions(t *testing.T) {
	tun := New()
	if got := tun.Status(); got.Running {
		t.Fatalf("zero-value Tunnel should start stopped, got %+v", got)
	}
	if err := tun.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	status := tun.Status()
	if !status.Running || status.PublicURL == "" || status.StartedAt == nil {
		t.Fatalf("after Start, want Running=true with a public URL and StartedAt, got %+v", status)
	}
	if err := tun.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	status = tun.Status()
	if status.Running {
		t.Fatalf("after Stop, want Running=false, got %+v", status)
	}
}

func TestStartRejectsCanceledContext(t *testing.T) {
	tun := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := tun.Start(ctx); err == nil {
		t.Fatal("expected an error starting with an already-canceled context")
	}
}

func TestConcurrentStartStopIsRaceFree(t *testing.T) {
	tun := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_ = tun.Start(context.Background())
			} else {
				_ = tun.Stop(context.Background())
			}
			_ = tun.Status()
		}(i)
	}
	wg.Wait()
}
