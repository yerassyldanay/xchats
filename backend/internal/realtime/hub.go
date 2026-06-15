// Package realtime is the SSE fan-out. Build 0 broadcasts every event to every
// connected client (single org/user); per-user routing is a later concern.
package realtime

import "sync"

// Event is a named SSE event carrying an entity payload.
type Event struct {
	Name string
	Data any
}

// Hub fans events out to subscribed SSE connections.
type Hub struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan Event]struct{})}
}

// Subscribe registers a connection and returns its channel + an unsubscribe func.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
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

// Broadcast delivers an event to all subscribers, dropping for any slow consumer
// rather than blocking the producer.
func (h *Hub) Broadcast(name string, data any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- Event{Name: name, Data: data}:
		default:
		}
	}
}
