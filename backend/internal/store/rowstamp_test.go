package store

import (
	"sync"
	"testing"
	"time"
)

func TestRowStampSpacesRowsOneMillisecondApart(t *testing.T) {
	base := time.UnixMilli(1791244800000).UTC()
	if got := rowStamp(base, 0); !got.Equal(base) {
		t.Fatalf("rowStamp(base, 0) = %v, want the base itself", got)
	}
	prev := base.Add(-time.Millisecond)
	for i := 0; i < 1000; i++ {
		got := rowStamp(base, i)
		if want := base.Add(time.Duration(i) * time.Millisecond); !got.Equal(want) {
			t.Fatalf("rowStamp(base, %d) = %v, want %v", i, got, want)
		}
		if got.UnixMilli() != prev.UnixMilli()+1 {
			t.Fatalf("row %d is %d ms after row %d, want exactly 1 (stored milliseconds must differ)", i, got.UnixMilli()-prev.UnixMilli(), i-1)
		}
		prev = got
	}
}

func TestEventStampIsStrictlyIncreasingEvenAcrossGoroutines(t *testing.T) {
	prev := eventStamp()
	for i := 0; i < 5000; i++ {
		next := eventStamp()
		if !next.After(prev) {
			t.Fatalf("call %d returned %v after %v", i, next, prev)
		}
		prev = next
	}

	const goroutines, calls = 8, 500
	var mu sync.Mutex
	seen := make(map[int64]bool, goroutines*calls)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < calls; i++ {
				ms := eventStamp().UnixMilli()
				mu.Lock()
				if seen[ms] {
					t.Errorf("millisecond %d handed out twice", ms)
				}
				seen[ms] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
}

func TestEventStampIsAWholeMillisecondInUTCAndNeverLagsTheWallClock(t *testing.T) {
	for i := 0; i < 100; i++ {
		before := time.Now().UnixMilli()
		got := eventStamp()
		if got.UnixMilli() < before {
			t.Fatalf("eventStamp() = %d ms, before the wall clock read %d ms", got.UnixMilli(), before)
		}
		if got.Location() != time.UTC || got.Nanosecond()%int(time.Millisecond) != 0 {
			t.Fatalf("eventStamp() = %v: want a whole millisecond in UTC", got)
		}
	}
}
