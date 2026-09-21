// Package realtime is the SSE fan-out. Broadcast (unscoped) reaches every
// connected client on every organization — the original Build 0 design,
// still correct for content with nothing tenant-sensitive in it.
// BroadcastScoped/Subscribe add org-scoped delivery for content that IS
// tenant-sensitive (chat/message/campaign/account events carry ids and
// counters, but a wrong-tenant client receiving them at all is still a
// cross-tenant leak the frontend's own filtering cannot be trusted to catch
// — see this feature's own authorization requirements). A subscriber
// receives every unscoped Broadcast PLUS every BroadcastScoped call whose
// orgID matches its own; it never receives a BroadcastScoped call for a
// different org.
package realtime

import (
	"sync"

	"github.com/google/uuid"
)

// Event is a named SSE event carrying an entity payload. OrgID is the zero
// UUID for an unscoped (Broadcast) event — delivered to every subscriber
// regardless of its own org — or a specific organization for a
// BroadcastScoped event, delivered only to subscribers registered for that
// same org.
type Event struct {
	Name  string
	Data  any
	OrgID uuid.UUID
}

type subscription struct {
	orgID uuid.UUID
	all   bool // true for SubscribeAll — receives every event regardless of OrgID
}

// Hub fans events out to subscribed SSE connections.
type Hub struct {
	mu   sync.RWMutex
	subs map[chan Event]subscription
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan Event]subscription)}
}

// Subscribe registers a connection scoped to orgID and returns its channel +
// an unsubscribe func: it receives every unscoped Broadcast plus every
// BroadcastScoped(orgID, ...) call for that SAME org, never another org's.
// This is what every multi-tenant, server-side consumer (the browser SSE
// endpoint) must use.
func (h *Hub) Subscribe(orgID uuid.UUID) (<-chan Event, func()) {
	return h.subscribe(subscription{orgID: orgID})
}

// SubscribeAll registers a connection that receives every event regardless
// of which org (if any) it was scoped to — correct ONLY for a genuinely
// single-tenant consumer with no per-connection org of its own, such as the
// desktop app's single local realtime pump (one install, one active session
// at a time). A multi-tenant server-side consumer must use Subscribe(orgID)
// instead, or it would defeat BroadcastScoped's whole purpose.
func (h *Hub) SubscribeAll() (<-chan Event, func()) {
	return h.subscribe(subscription{all: true})
}

func (h *Hub) subscribe(sub subscription) (<-chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subs[ch] = sub
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

// Broadcast delivers an UNSCOPED event to every subscriber regardless of
// its own org, dropping for any slow consumer rather than blocking the
// producer. Reserved for content with no tenant-sensitive fields at all —
// prefer BroadcastScoped for anything naming a chat, message, campaign,
// recipient, or account.
func (h *Hub) Broadcast(name string, data any) {
	h.broadcast(Event{Name: name, Data: data})
}

// BroadcastScoped delivers an event only to subscribers registered for
// orgID — the enforcement point this feature's own authorization
// requirement calls for ("frontend filtering is insufficient"): a client
// authenticated for a different organization never receives the event at
// all, not merely one told to discard it.
func (h *Hub) BroadcastScoped(orgID uuid.UUID, name string, data any) {
	h.broadcast(Event{Name: name, Data: data, OrgID: orgID})
}

func (h *Hub) broadcast(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch, sub := range h.subs {
		if !sub.all && ev.OrgID != uuid.Nil && ev.OrgID != sub.orgID {
			continue
		}
		select {
		case ch <- ev:
		default:
		}
	}
}
