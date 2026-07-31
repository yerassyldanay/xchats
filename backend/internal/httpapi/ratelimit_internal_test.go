package httpapi

import (
	"testing"
	"time"
)

func TestIPRateLimiter_BurstThenBlocks(t *testing.T) {
	l := newIPRateLimiter(1, 3)
	for i := 0; i < 3; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("request %d within burst should be allowed", i+1)
		}
	}
	if l.allow("1.2.3.4") {
		t.Fatal("request beyond burst should be blocked")
	}
}

func TestIPRateLimiter_KeysAreIndependent(t *testing.T) {
	l := newIPRateLimiter(1, 1)
	if !l.allow("1.2.3.4") {
		t.Fatal("first request for 1.2.3.4 should be allowed")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("second immediate request for the SAME ip should be blocked")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("a DIFFERENT ip must have its own bucket, unaffected by 1.2.3.4's usage")
	}
}

func TestIPRateLimiter_RefillsOverTime(t *testing.T) {
	l := newIPRateLimiter(20, 1) // 20 tokens/sec -> refills in ~50ms
	if !l.allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("immediate second request should be blocked (bucket just emptied)")
	}
	time.Sleep(100 * time.Millisecond)
	if !l.allow("1.2.3.4") {
		t.Fatal("after waiting past the refill interval, a request should be allowed again")
	}
}
