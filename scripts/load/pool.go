package main

import (
	"fmt"
	"math/rand"
	"sync"
)

// reservoirCap bounds every reservoir this tool maintains — a run must not
// grow its own discovered-ID bookkeeping without bound as it runs longer.
const reservoirCap = 500

// reservoir is a fixed-capacity, thread-safe uniform sample of a stream of
// discovered strings (classic reservoir sampling, Algorithm R) — used for
// "some chat id we've seen," never "every chat id we've ever seen."
type reservoir struct {
	mu    sync.Mutex
	rng   *rand.Rand
	cap   int
	items []string
	seen  int64
}

func newReservoir(seed int64, cap int) *reservoir {
	return &reservoir{rng: rand.New(rand.NewSource(seed)), cap: cap}
}

// Add offers one more discovered item to the sample.
func (r *reservoir) Add(item string) {
	if item == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen++
	if len(r.items) < r.cap {
		r.items = append(r.items, item)
		return
	}
	if j := r.rng.Int63n(r.seen); j < int64(r.cap) {
		r.items[j] = item
	}
}

// Pick returns a uniformly random item from the current sample, using rng
// (the CALLER's own per-worker RNG, for run-level reproducibility — the
// reservoir's internal rng only ever decides SAMPLING admission, never
// which sampled item a given request uses). ok is false on an empty sample.
func (r *reservoir) Pick(rng *rand.Rand) (item string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) == 0 {
		return "", false
	}
	return r.items[rng.Intn(len(r.items))], true
}

// Len reports the current sample size (never more than cap).
func (r *reservoir) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

// channelReservoirs is a per-channel set of reservoirs — the "rotate
// through every seeded channel" outbound-send bucket's own discovered-chat
// pool, populated only from GET /chats responses (the one route whose
// payload actually carries each chat's channel).
type channelReservoirs struct {
	mu    sync.Mutex
	seed  int64
	byCh  map[string]*reservoir
	order []string // discovery order, for deterministic round-robin
	rr    uint64   // round-robin cursor, advanced under mu
}

func newChannelReservoirs(seed int64) *channelReservoirs {
	return &channelReservoirs{seed: seed, byCh: map[string]*reservoir{}}
}

func (c *channelReservoirs) Add(channel, chatID string) {
	if channel == "" || chatID == "" {
		return
	}
	c.mu.Lock()
	r, ok := c.byCh[channel]
	if !ok {
		r = newReservoir(c.seed^int64(len(c.order)+1), reservoirCap)
		c.byCh[channel] = r
		c.order = append(c.order, channel)
	}
	c.mu.Unlock()
	r.Add(chatID)
}

// NextChannel round-robins across every channel discovered SO FAR — "so
// far" is deliberate: an outbound-send iteration early in the run, before
// GET /chats has discovered every seeded channel, simply rotates across
// however many are known yet, and naturally includes new ones as they
// appear. Returns ok=false only when nothing has been discovered at all.
func (c *channelReservoirs) NextChannel() (channel string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.order) == 0 {
		return "", false
	}
	idx := c.rr % uint64(len(c.order))
	c.rr++
	return c.order[idx], true
}

func (c *channelReservoirs) Pick(rng *rand.Rand, channel string) (chatID string, ok bool) {
	c.mu.Lock()
	r := c.byCh[channel]
	c.mu.Unlock()
	if r == nil {
		return "", false
	}
	return r.Pick(rng)
}

// Channels returns every channel discovered so far, for reporting.
func (c *channelReservoirs) Channels() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.order))
	copy(out, c.order)
	return out
}

// contactPool is a fixed, precomputed set of synthetic contact identities —
// reused across requests (never one brand-new contact per call) so a
// sustained run's dataset size stays bounded regardless of duration, and so
// repeated messages from "the same customer" exercise the existing-
// conversation path, not just first-contact creation.
type contactPool struct {
	refs []string
}

func newContactPool(size int) *contactPool {
	refs := make([]string, size)
	for i := range refs {
		refs[i] = fmt.Sprintf("load-contact-%04d", i)
	}
	return &contactPool{refs: refs}
}

func (p *contactPool) Pick(rng *rand.Rand) string {
	return p.refs[rng.Intn(len(p.refs))]
}
