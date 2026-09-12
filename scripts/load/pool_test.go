package main

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func TestReservoir_BoundedByCapacity(t *testing.T) {
	r := newReservoir(1, 10)
	for i := 0; i < 1000; i++ {
		r.Add(fmt.Sprintf("item-%d", i))
	}
	if got := r.Len(); got != 10 {
		t.Fatalf("Len() = %d, want 10", got)
	}
}

func TestReservoir_PickReturnsAddedItems(t *testing.T) {
	r := newReservoir(1, 10)
	r.Add("only-item")
	rng := rand.New(rand.NewSource(1))
	got, ok := r.Pick(rng)
	if !ok || got != "only-item" {
		t.Fatalf("Pick() = %q, %v; want only-item, true", got, ok)
	}
}

func TestReservoir_EmptyPickReturnsFalse(t *testing.T) {
	r := newReservoir(1, 10)
	if _, ok := r.Pick(rand.New(rand.NewSource(1))); ok {
		t.Fatal("Pick() on empty reservoir returned ok=true")
	}
}

func TestReservoir_IgnoresEmptyString(t *testing.T) {
	r := newReservoir(1, 10)
	r.Add("")
	if got := r.Len(); got != 0 {
		t.Fatalf("Len() = %d after adding an empty string, want 0", got)
	}
}

// TestReservoir_ConcurrentAddIsRaceFree exercises the reservoir under
// concurrent writers — the shape of 32 load-test workers discovering chat
// ids simultaneously.
func TestReservoir_ConcurrentAddIsRaceFree(t *testing.T) {
	r := newReservoir(1, 50)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				r.Add(fmt.Sprintf("worker-%d-item-%d", i, j))
			}
		}(i)
	}
	wg.Wait()
	if got := r.Len(); got != 50 {
		t.Fatalf("Len() = %d, want 50 (bounded)", got)
	}
}

func TestChannelReservoirs_RoundRobinsAcrossDiscoveredChannels(t *testing.T) {
	c := newChannelReservoirs(1)
	if _, ok := c.NextChannel(); ok {
		t.Fatal("NextChannel() before any discovery should return ok=false")
	}
	c.Add("whatsapp", "chat-1")
	c.Add("telegram", "chat-2")
	c.Add("instagram", "chat-3")

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		ch, ok := c.NextChannel()
		if !ok {
			t.Fatalf("NextChannel() returned ok=false at iteration %d", i)
		}
		seen[ch]++
	}
	for _, ch := range []string{"whatsapp", "telegram", "instagram"} {
		if seen[ch] != 3 {
			t.Errorf("channel %q selected %d times over 9 rotations, want 3 (even round-robin)", ch, seen[ch])
		}
	}
}

func TestChannelReservoirs_PickReturnsFromNamedChannelOnly(t *testing.T) {
	c := newChannelReservoirs(1)
	c.Add("whatsapp", "wa-chat")
	c.Add("telegram", "tg-chat")
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		got, ok := c.Pick(rng, "whatsapp")
		if !ok || got != "wa-chat" {
			t.Fatalf("Pick(whatsapp) = %q, %v; want wa-chat, true", got, ok)
		}
	}
	if _, ok := c.Pick(rng, "instagram"); ok {
		t.Fatal("Pick() on an undiscovered channel returned ok=true")
	}
}

func TestContactPool_BoundedSizeAndDeterministicPicks(t *testing.T) {
	p := newContactPool(5)
	seen := map[string]bool{}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		seen[p.Pick(rng)] = true
	}
	if len(seen) > 5 {
		t.Fatalf("saw %d distinct contacts from a pool of 5", len(seen))
	}

	// Same seed -> same sequence of picks, across two independent RNGs.
	rngA := rand.New(rand.NewSource(42))
	rngB := rand.New(rand.NewSource(42))
	for i := 0; i < 20; i++ {
		a, b := p.Pick(rngA), p.Pick(rngB)
		if a != b {
			t.Fatalf("pick %d diverged: %q vs %q for the same seed", i, a, b)
		}
	}
}
