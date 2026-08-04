package queue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
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
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_ = q.Publish(ctx, Message{Kind: KindAIDraft, Payload: i})
	}
	_ = q.Publish(ctx, Message{Kind: KindAIDraft, Payload: "boom"})
	for i := 0; i < 10; i++ {
		_ = q.Publish(ctx, Message{Kind: KindAIDraft, Payload: i})
	}
	q.Wait() // drain-and-assert
	if got := processed.Load(); got != 20 {
		t.Fatalf("processed %d, want 20 (the pool stalled or dropped good work)", got)
	}
	q.Close()
}

// TestPublishReturnsOnFullBufferInsteadOfBlocking pins the backpressure
// contract: with no workers started, a buffer of 1 fills on the first publish
// and the second must come back as ctx.Err() — not wedge the caller — and must
// not leave phantom in-flight work behind for Wait().
func TestPublishReturnsOnFullBufferInsteadOfBlocking(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := NewInMem(1, 1, log) // buffer of 1; Start is deliberately never called

	if err := q.Publish(context.Background(), Message{Kind: KindAIDraft, Payload: 1}); err != nil {
		t.Fatalf("first publish (buffer has room): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- q.Publish(ctx, Message{Kind: KindAIDraft, Payload: 2}) }()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("publish on a full buffer = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("publish blocked past its own deadline — backpressure is still unbounded")
	}

	// The rejected message must not count as in-flight: once the pool drains the
	// one that did make it in, Wait() has to return.
	q.Start(context.Background(), func(context.Context, Message) error { return nil })
	waited := make(chan struct{})
	go func() { q.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() still counts the rejected publish as in-flight")
	}
}
