package realtime

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func recvOrTimeout(t *testing.T, ch <-chan Event) (Event, bool) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		return ev, ok
	case <-time.After(200 * time.Millisecond):
		return Event{}, false
	}
}

// TestBroadcastReachesEverySubscriberRegardlessOfOrg pins the ORIGINAL,
// still-supported behavior: an unscoped Broadcast is process-global, exactly
// as it always was, for content with nothing tenant-sensitive in it.
func TestBroadcastReachesEverySubscriberRegardlessOfOrg(t *testing.T) {
	h := NewHub()
	orgA, orgB := uuid.New(), uuid.New()
	chA, unsubA := h.Subscribe(orgA)
	defer unsubA()
	chB, unsubB := h.Subscribe(orgB)
	defer unsubB()

	h.Broadcast("kb.row.changed", "payload")

	for name, ch := range map[string]<-chan Event{"orgA": chA, "orgB": chB} {
		ev, ok := recvOrTimeout(t, ch)
		if !ok {
			t.Fatalf("%s: did not receive the unscoped broadcast", name)
		}
		if ev.Name != "kb.row.changed" {
			t.Errorf("%s: event name = %q", name, ev.Name)
		}
	}
}

// TestBroadcastScopedReachesOnlyItsOwnOrg is the enforcement point this
// feature's own authorization requirement calls for: a subscriber
// registered for a DIFFERENT org than the one an event was scoped to must
// never receive it at all — not merely be expected to discard it.
func TestBroadcastScopedReachesOnlyItsOwnOrg(t *testing.T) {
	h := NewHub()
	orgA, orgB := uuid.New(), uuid.New()
	chA, unsubA := h.Subscribe(orgA)
	defer unsubA()
	chB, unsubB := h.Subscribe(orgB)
	defer unsubB()

	h.BroadcastScoped(orgA, "chat.updated", "org-a-chat")

	evA, ok := recvOrTimeout(t, chA)
	if !ok {
		t.Fatal("orgA subscriber: did not receive its own org's scoped event")
	}
	if evA.Data != "org-a-chat" {
		t.Errorf("orgA subscriber: data = %v", evA.Data)
	}

	if ev, ok := recvOrTimeout(t, chB); ok {
		t.Fatalf("orgB subscriber received an event scoped to a different org: %+v", ev)
	}
}

// TestSubscribeAllReceivesScopedEventsForEveryOrg confirms the desktop
// pump's exemption (SubscribeAll) still sees every org's scoped events too
// — the isolation guarantee is opt-in per the caller's own subscription
// choice, not a blanket restriction on the hub.
func TestSubscribeAllReceivesScopedEventsForEveryOrg(t *testing.T) {
	h := NewHub()
	orgA, orgB := uuid.New(), uuid.New()
	chAll, unsubAll := h.SubscribeAll()
	defer unsubAll()

	h.BroadcastScoped(orgA, "chat.updated", "a")
	h.BroadcastScoped(orgB, "chat.updated", "b")

	first, ok := recvOrTimeout(t, chAll)
	if !ok {
		t.Fatal("SubscribeAll: did not receive orgA's scoped event")
	}
	second, ok := recvOrTimeout(t, chAll)
	if !ok {
		t.Fatal("SubscribeAll: did not receive orgB's scoped event")
	}
	got := map[any]bool{first.Data: true, second.Data: true}
	if !got["a"] || !got["b"] {
		t.Errorf("SubscribeAll saw %v, want both a and b", got)
	}
}

// TestUnsubscribeStopsDelivery confirms the returned unsubscribe func
// actually removes the subscription (both from further Broadcasts and
// BroadcastScoped calls), and closes the channel.
func TestUnsubscribeStopsDelivery(t *testing.T) {
	h := NewHub()
	org := uuid.New()
	ch, unsub := h.Subscribe(org)
	unsub()

	h.Broadcast("kb.row.changed", "x")
	h.BroadcastScoped(org, "chat.updated", "y")

	_, ok := recvOrTimeout(t, ch)
	if ok {
		t.Fatal("received an event after unsubscribing")
	}
	// The channel itself is closed (recvOrTimeout would time out on a plain
	// nil channel too, so confirm closure directly).
	select {
	case _, open := <-ch:
		if open {
			t.Error("channel still open after unsubscribe")
		}
	default:
		t.Error("channel neither closed nor drained after unsubscribe")
	}
}
