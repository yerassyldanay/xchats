package desktop

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/internal/realtime"
)

// Emitter delivers one backend event to the desktop frontend. In the real
// shell this is Wails' runtime.EventsEmit; as an interface-free function type
// it keeps this file (and its test) free of any Wails import.
type Emitter func(name string, data any)

// PumpRealtime forwards every realtime.Hub event to the desktop frontend
// until ctx is cancelled or the hub closes the subscription.
//
// This is the desktop replacement for the SSE stream, and deliberately the
// only one: the event names and payloads are the hub's own, unchanged, so
// frontend/src/lib/sse.ts binds the same "message.created", "chat.updated",
// "kb.row.changed" … handlers over either transport and nothing downstream of
// connectRealtime can tell which one it got.
//
// The hub already drops events for a slow consumer rather than blocking a
// producer (realtime.Hub.Broadcast), and this subscriber does no I/O of its
// own — EventsEmit hands the payload to the WebView's own queue — so the
// pump cannot become the thing that stalls a webhook ingest.
func PumpRealtime(ctx context.Context, hub *realtime.Hub, emit Emitter) {
	if hub == nil || emit == nil {
		return
	}
	events, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, alive := <-events:
			if !alive {
				return
			}
			emit(ev.Name, ev.Data)
		}
	}
}
