package campaign

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm
}

func TestBudget_AllowsWhenEverythingHasHeadroom(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")
	d := Budget(DefaultTiers, nil, 90*time.Second, time.Time{}, now)
	if !d.Allowed {
		t.Fatalf("Decision = %+v, want Allowed", d)
	}
	if d.ThrottledBy != 0 {
		t.Errorf("ThrottledBy = %d, want 0", d.ThrottledBy)
	}
	if !d.NextSlotAt.Equal(now) {
		t.Errorf("NextSlotAt = %v, want %v", d.NextSlotAt, now)
	}
}

func TestBudget_BlocksOnMinInterval(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")
	last := now.Add(-30 * time.Second) // interval is 90s, only 30s elapsed
	d := Budget(DefaultTiers, nil, 90*time.Second, last, now)
	if d.Allowed {
		t.Fatalf("Decision = %+v, want blocked by interval", d)
	}
	want := last.Add(90 * time.Second)
	if !d.NextSlotAt.Equal(want) {
		t.Errorf("NextSlotAt = %v, want %v", d.NextSlotAt, want)
	}
}

func TestBudget_AllowsExactlyAtInterval(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")
	last := now.Add(-90 * time.Second)
	d := Budget(DefaultTiers, nil, 90*time.Second, last, now)
	if !d.Allowed {
		t.Fatalf("Decision = %+v, want allowed at exactly the interval boundary", d)
	}
}

func TestBudget_TierAtCapacityBlocks(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")
	tiers := []Tier{{WindowSeconds: 3600, MaxSends: 2}}
	attempts := []time.Time{
		now.Add(-50 * time.Minute),
		now.Add(-10 * time.Minute),
	}
	d := Budget(tiers, attempts, 0, time.Time{}, now)
	if d.Allowed {
		t.Fatalf("Decision = %+v, want blocked (tier at capacity)", d)
	}
	if d.ThrottledBy != 3600 {
		t.Errorf("ThrottledBy = %d, want 3600", d.ThrottledBy)
	}
	// The oldest of the two in-window attempts (50m ago) frees a slot once
	// it ages past the 1h window: now - 50m + 1h = now + 10m.
	want := now.Add(-50*time.Minute + time.Hour)
	if !d.NextSlotAt.Equal(want) {
		t.Errorf("NextSlotAt = %v, want %v", d.NextSlotAt, want)
	}
}

func TestBudget_AttemptOutsideWindowDoesNotCount(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")
	tiers := []Tier{{WindowSeconds: 3600, MaxSends: 1}}
	attempts := []time.Time{now.Add(-2 * time.Hour)} // outside the 1h window
	d := Budget(tiers, attempts, 0, time.Time{}, now)
	if !d.Allowed {
		t.Fatalf("Decision = %+v, want allowed (the one attempt is outside the window)", d)
	}
}

func TestBudget_MultipleTiersEvaluatedSimultaneously(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")
	tiers := []Tier{
		{WindowSeconds: 3600, MaxSends: 5},  // plenty of headroom
		{WindowSeconds: 86400, MaxSends: 2}, // at capacity
	}
	attempts := []time.Time{
		now.Add(-23 * time.Hour),
		now.Add(-1 * time.Hour),
	}
	d := Budget(tiers, attempts, 0, time.Time{}, now)
	if d.Allowed {
		t.Fatalf("Decision = %+v, want blocked by the 24h tier even though the 1h tier has headroom", d)
	}
	if d.ThrottledBy != 86400 {
		t.Errorf("ThrottledBy = %d, want 86400 (the tier that actually blocked)", d.ThrottledBy)
	}
}

func TestBudget_EmptyTiersNeverThrottle(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")
	attempts := make([]time.Time, 1000)
	for i := range attempts {
		attempts[i] = now
	}
	d := Budget(nil, attempts, 0, time.Time{}, now)
	if !d.Allowed {
		t.Fatalf("Decision = %+v, want allowed (an explicit empty tier set — e.g. a custom account override — never throttles on tiers)", d)
	}
}

func TestWindowsOK(t *testing.T) {
	// Monday 09:00-17:00 UTC.
	work := []Window{{Weekday: time.Monday, StartMinute: 9 * 60, EndMinute: 17 * 60}}
	monday9am := mustTime(t, "2026-01-05T09:00:00Z") // a Monday
	mondayNoon := mustTime(t, "2026-01-05T12:00:00Z")
	monday8am := mustTime(t, "2026-01-05T08:00:00Z")
	tuesdayNoon := mustTime(t, "2026-01-06T12:00:00Z")

	tests := []struct {
		name              string
		account, campaign []Window
		now               time.Time
		want              bool
	}{
		{"no windows on either side is unrestricted", nil, nil, monday8am, true},
		{"inside the only (account) window", work, nil, mondayNoon, true},
		{"outside the only (account) window", work, nil, monday8am, false},
		{"outside on a different day", work, nil, tuesdayNoon, false},
		{"campaign narrows further and is satisfied", work, work, mondayNoon, true},
		{"campaign narrows further and is NOT satisfied even though account is", work, []Window{{Weekday: time.Tuesday, StartMinute: 0, EndMinute: 1440}}, mondayNoon, false},
		{"account empty, campaign restricts alone", nil, work, monday8am, false},
		{"account empty, campaign restricts alone and is satisfied", nil, work, monday9am, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WindowsOK(tt.account, tt.campaign, tt.now)
			if got != tt.want {
				t.Errorf("WindowsOK(...) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimate_NoWindowsSimpleTier(t *testing.T) {
	now := mustTime(t, "2026-01-01T00:00:00Z")
	tiers := []Tier{{WindowSeconds: 86400, MaxSends: 50}}
	// 500 recipients at 50/day, no interval, no windows -> exactly 10 days
	// of sends: the 500th send lands on day 10 (0-indexed days 0..9).
	got := Estimate(500, nil, tiers, 0, nil, nil, now)
	wantDay := now.AddDate(0, 0, 9)
	if got.Year() != wantDay.Year() || got.YearDay() != wantDay.YearDay() {
		t.Errorf("Estimate = %v, want on day %v (the 10th day of sending)", got, wantDay)
	}
}

func TestEstimate_SeedsFromPriorAttempts(t *testing.T) {
	now := mustTime(t, "2026-01-01T00:00:00Z")
	tiers := []Tier{{WindowSeconds: 3600, MaxSends: 2}}
	// The tier is already exhausted by two real attempts moments ago — the
	// next slot must wait for the OLDER of the two to age out of the 1h
	// window, not be reported as immediately available just because the
	// simulation itself hasn't sent anything yet.
	prior := []time.Time{now.Add(-50 * time.Minute), now.Add(-40 * time.Minute)}
	got := Estimate(1, prior, tiers, 0, nil, nil, now)
	want := prior[0].Add(time.Hour)
	if !got.Equal(want) {
		t.Errorf("Estimate with prior attempts = %v, want %v (when the oldest attempt ages out)", got, want)
	}

	// The same tiers with NO prior attempts have immediate headroom.
	gotClean := Estimate(1, nil, tiers, 0, nil, nil, now)
	if !gotClean.Equal(now) {
		t.Errorf("Estimate with no prior attempts = %v, want %v", gotClean, now)
	}
}

func TestEstimate_ZeroRemainingIsNow(t *testing.T) {
	now := mustTime(t, "2026-01-01T00:00:00Z")
	got := Estimate(0, nil, DefaultTiers, 90*time.Second, nil, nil, now)
	if !got.Equal(now) {
		t.Errorf("Estimate(0, ...) = %v, want %v", got, now)
	}
}

func TestEstimate_RespectsMinInterval(t *testing.T) {
	now := mustTime(t, "2026-01-01T00:00:00Z")
	// No tier limits at all, pure interval pacing: 3 recipients at 90s apart
	// means the 3rd goes out 180s after the first (2 gaps).
	got := Estimate(3, nil, nil, 90*time.Second, nil, nil, now)
	want := now.Add(180 * time.Second)
	if !got.Equal(want) {
		t.Errorf("Estimate = %v, want %v", got, want)
	}
}

func TestEstimate_ImpossibleWindowsReturnsFarFuture(t *testing.T) {
	now := mustTime(t, "2026-01-05T00:00:00Z") // a Monday
	account := []Window{{Weekday: time.Monday, StartMinute: 0, EndMinute: 60}}
	campaign := []Window{{Weekday: time.Tuesday, StartMinute: 0, EndMinute: 60}}
	got := Estimate(1, nil, DefaultTiers, 0, account, campaign, now)
	if !got.Equal(farFuture) {
		t.Errorf("Estimate = %v, want the farFuture sentinel (account/campaign windows never overlap)", got)
	}
}
