package queue

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
)

func TestInMemDrainsAndSurvivesPanic(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := NewInMem(64, 2, log)
	var processed atomic.Int64
	q.Start(context.Background(), func(ctx context.Context, m Message) error {
		if m.Payload == "boom" {
			panic("bad payload") // a panicking handler must not kill the pool
		}
		processed.Add(1)
		return nil
	})
	for i := 0; i < 10; i++ {
		_ = q.Publish(Message{Kind: KindWaEvent, Payload: i})
	}
	_ = q.Publish(Message{Kind: KindWaEvent, Payload: "boom"})
	for i := 0; i < 10; i++ {
		_ = q.Publish(Message{Kind: KindWaEvent, Payload: i})
	}
	q.Wait() // drain-and-assert
	if got := processed.Load(); got != 20 {
		t.Fatalf("processed %d, want 20 (the pool stalled or dropped good work)", got)
	}
	q.Close()
}
