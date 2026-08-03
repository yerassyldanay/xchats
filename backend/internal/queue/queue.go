// Package queue is the async seam between the HTTP/webhook edges and the workers.
// Build 0 ships exactly one driver: an in-memory buffered channel + worker pool,
// behind the Queue port (swappable to Redis/Kafka later via QUEUE_DRIVER).
package queue

import (
	"context"
	"log/slog"
	"sync"
)

// Kind enumerates the task kinds that ride the queue.
type Kind string

const (
	KindWaEvent       Kind = "wa_event"
	KindMediaDownload Kind = "media_download"
	KindOutboundSend  Kind = "outbound_send"
	KindAIDraft       Kind = "ai_draft"
)

// Message is one unit of async work. Payload is kind-specific.
type Message struct {
	Kind    Kind
	Payload any
}

// Handler processes a single message. A returned error is logged and the message
// dropped (Build 0 accepts the loss window); panics are recovered by the worker.
type Handler func(ctx context.Context, m Message) error

// Queue is the port producers and consumers depend on. Publish takes a context
// so a full buffer becomes a retryable error at the producer instead of an
// unbounded block — the Telegram webhook, for one, turns that error into a 500
// so Telegram redelivers rather than hanging the request.
type Queue interface {
	Publish(ctx context.Context, m Message) error
	Start(ctx context.Context, h Handler)
	Wait() // block until all in-flight work is processed (drain-and-assert in tests)
	Close()
}

// InMem is the v1 in-memory driver.
type InMem struct {
	ch      chan Message
	wg      sync.WaitGroup
	workers int
	log     *slog.Logger
	closeMu sync.Once
}

// NewInMem builds an in-memory queue with the given buffer and worker count.
func NewInMem(buffer, workers int, log *slog.Logger) *InMem {
	if buffer <= 0 {
		buffer = 1024
	}
	if workers <= 0 {
		workers = 4
	}
	return &InMem{ch: make(chan Message, buffer), workers: workers, log: log}
}

// Publish enqueues a message, tracking it as in-flight so Wait() can drain.
// When the buffer is full it blocks only until ctx is done, then returns
// ctx.Err() and undoes the in-flight accounting — so backpressure surfaces as a
// retryable error at the producer instead of wedging the calling goroutine
// forever (the send is a plain `q.ch <- m` otherwise).
func (q *InMem) Publish(ctx context.Context, m Message) error {
	q.wg.Add(1)
	select {
	case q.ch <- m:
		return nil
	case <-ctx.Done():
		q.wg.Done() // never enqueued — don't leave Wait() hanging on it
		return ctx.Err()
	}
}

// Start launches the worker pool; each worker recovers from panics so one bad
// payload can't kill the pool and stall the whole inbox.
func (q *InMem) Start(ctx context.Context, h Handler) {
	for i := 0; i < q.workers; i++ {
		go func(id int) {
			for {
				select {
				case <-ctx.Done():
					return
				case m, ok := <-q.ch:
					if !ok {
						return
					}
					q.process(ctx, h, m)
				}
			}
		}(i)
	}
}

func (q *InMem) process(ctx context.Context, h Handler, m Message) {
	defer q.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			q.log.Error("queue handler panic", "kind", m.Kind, "panic", r)
		}
	}()
	if err := h(ctx, m); err != nil {
		q.log.Error("queue task failed", "kind", m.Kind, "err", err)
	}
}

// Wait blocks until every published message has been processed.
func (q *InMem) Wait() { q.wg.Wait() }

// Close stops accepting new work.
func (q *InMem) Close() {
	q.closeMu.Do(func() { close(q.ch) })
}
