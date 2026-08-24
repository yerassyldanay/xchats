package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/realtime"
)

func TestPumpRealtimeForwardsHubEventsUnchanged(t *testing.T) {
	hub := realtime.NewHub()
	type emitted struct {
		name string
		data any
	}
	got := make(chan emitted, 4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		PumpRealtime(ctx, hub, func(name string, data any) { got <- emitted{name, data} })
	}()

	// Broadcast drops for subscribers that are not attached yet, so wait for
	// the pump's Subscribe to land before producing.
	deadline := time.After(2 * time.Second)
	for {
		hub.Broadcast("message.created", map[string]any{"id": "m1"})
		select {
		case ev := <-got:
			if ev.name != "message.created" {
				t.Fatalf("event name = %q, want the hub's own name unchanged", ev.name)
			}
			m, ok := ev.data.(map[string]any)
			if !ok || m["id"] != "m1" {
				t.Fatalf("event data = %#v, want the hub payload unchanged", ev.data)
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("PumpRealtime did not return after ctx was cancelled")
			}
			return
		case <-deadline:
			t.Fatal("no event reached the emitter within 2s")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestPumpRealtimeIgnoresMissingDependencies(t *testing.T) {
	// A shell built without a hub (or without an emitter, before Wails has
	// started) must return rather than panic.
	PumpRealtime(context.Background(), nil, func(string, any) {})
	PumpRealtime(context.Background(), realtime.NewHub(), nil)
}
