package automation

import (
	"testing"
	"time"
)

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm.UTC()
}

func TestValidMode(t *testing.T) {
	for _, m := range []string{"off", "suggestions", "scheduled_auto"} {
		if !ValidMode(m) {
			t.Errorf("ValidMode(%q) = false, want true", m)
		}
	}
	for _, m := range []string{"", "OFF", "always", "auto"} {
		if ValidMode(m) {
			t.Errorf("ValidMode(%q) = true, want false", m)
		}
	}
}

func TestValidateWaitOverride(t *testing.T) {
	ptr := func(i int) *int { return &i }
	cases := []struct {
		name    string
		value   *int
		wantErr bool
	}{
		{"nil is valid", nil, false},
		{"zero is valid", ptr(0), false},
		{"max is valid", ptr(60), false},
		{"mid is valid", ptr(30), false},
		{"negative rejected", ptr(-1), true},
		{"over max rejected", ptr(61), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateWaitOverride(c.value)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateWaitOverride(%v) err = %v, wantErr %v", c.value, err, c.wantErr)
			}
		})
	}
}

func TestEffectiveWaitSeconds(t *testing.T) {
	ptr := func(i int) *int { return &i }
	if got := EffectiveWaitSeconds(5, nil); got != 5 {
		t.Errorf("no override: got %d, want 5 (system default)", got)
	}
	if got := EffectiveWaitSeconds(5, ptr(0)); got != 0 {
		t.Errorf("override=0: got %d, want 0", got)
	}
	if got := EffectiveWaitSeconds(5, ptr(45)); got != 45 {
		t.Errorf("override=45: got %d, want 45", got)
	}
}

func TestDeadline(t *testing.T) {
	last := mustUTC(t, "2026-08-06 10:00")
	got := Deadline(last, 5)
	want := mustUTC(t, "2026-08-06 10:00").Add(5 * time.Second)
	if !got.Equal(want) {
		t.Errorf("Deadline = %v, want %v", got, want)
	}
	// No cap: repeated resets just keep pushing forward.
	got2 := Deadline(got, 5)
	if !got2.After(got) {
		t.Errorf("Deadline should keep advancing with each reset")
	}
}

func TestValidateWindow(t *testing.T) {
	cases := []struct {
		name    string
		w       Window
		wantErr bool
	}{
		{"same-day valid", Window{time.Monday, 0, 1440}, false},
		{"overnight valid", Window{time.Friday, 1320, 360}, false},
		{"weekday too low", Window{-1, 0, 60}, true},
		{"weekday too high", Window{7, 0, 60}, true},
		{"start negative", Window{time.Monday, -1, 60}, true},
		{"start too high", Window{time.Monday, 1440, 1500}, true},
		{"end zero", Window{time.Monday, 0, 0}, true},
		{"end too high", Window{time.Monday, 0, 1441}, true},
		{"end equals start", Window{time.Monday, 600, 600}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateWindow(c.w)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateWindow(%+v) err = %v, wantErr %v", c.w, err, c.wantErr)
			}
		})
	}
}

func TestWindowContainsSameDay(t *testing.T) {
	// Monday 09:00-17:00 UTC.
	w := Window{Weekday: time.Monday, StartMinute: 9 * 60, EndMinute: 17 * 60}

	cases := []struct {
		name string
		t    string
		want bool
	}{
		{"before window", "2026-08-03 08:59", false}, // 2026-08-03 is a Monday
		{"at start (inclusive)", "2026-08-03 09:00", true},
		{"inside", "2026-08-03 12:00", true},
		{"just before end", "2026-08-03 16:59", true},
		{"at end (exclusive)", "2026-08-03 17:00", false},
		{"after window", "2026-08-03 18:00", false},
		{"right weekday, right time, but Tuesday", "2026-08-04 12:00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := w.Contains(mustUTC(t, c.t))
			if got != c.want {
				t.Errorf("Contains(%s) = %v, want %v", c.t, got, c.want)
			}
		})
	}
}

func TestWindowContainsOvernight(t *testing.T) {
	// "Nights": every day 22:00 -> 06:00 next day. Use Friday->Saturday to
	// also exercise an ordinary (non-week-wrapping) overnight case.
	w := Window{Weekday: time.Friday, StartMinute: 22 * 60, EndMinute: 6 * 60}

	cases := []struct {
		name string
		t    string
		want bool
	}{
		{"Friday before window", "2026-08-07 21:59", false}, // 2026-08-07 is a Friday
		{"Friday at start", "2026-08-07 22:00", true},
		{"Friday late night", "2026-08-07 23:59", true},
		{"Saturday just after midnight", "2026-08-08 00:00", true},
		{"Saturday just before end", "2026-08-08 05:59", true},
		{"Saturday at end (exclusive)", "2026-08-08 06:00", false},
		{"Saturday daytime", "2026-08-08 12:00", false},
		{"the following Friday night (recurs weekly)", "2026-08-14 23:00", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := w.Contains(mustUTC(t, c.t))
			if got != c.want {
				t.Errorf("Contains(%s) = %v, want %v", c.t, got, c.want)
			}
		})
	}
}

func TestWindowContainsWeekWrap(t *testing.T) {
	// Saturday 23:00 -> Sunday 02:00: the overnight tail crosses from
	// Saturday (weekday 6) into Sunday (weekday 0), i.e. wraps the week
	// boundary itself, not just a calendar day.
	w := Window{Weekday: time.Saturday, StartMinute: 23 * 60, EndMinute: 2 * 60}

	cases := []struct {
		name string
		t    string
		want bool
	}{
		{"Saturday before window", "2026-08-08 22:59", false},
		{"Saturday at start", "2026-08-08 23:00", true},
		{"Sunday just after midnight", "2026-08-09 00:30", true},
		{"Sunday at end (exclusive)", "2026-08-09 02:00", false},
		{"Sunday daytime", "2026-08-09 10:00", false},
		{"Friday night does not match a Saturday-anchored window", "2026-08-07 23:30", false},
		{"recurs weekly: the following Saturday night", "2026-08-15 23:30", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := w.Contains(mustUTC(t, c.t))
			if got != c.want {
				t.Errorf("Contains(%s) = %v, want %v", c.t, got, c.want)
			}
		})
	}
}

func TestInSchedulePresets(t *testing.T) {
	always := make([]Window, 7)
	for d := time.Sunday; d <= time.Saturday; d++ {
		always[d] = Window{Weekday: d, StartMinute: 0, EndMinute: 1440}
	}
	weekends := []Window{
		{Weekday: time.Saturday, StartMinute: 0, EndMinute: 1440},
		{Weekday: time.Sunday, StartMinute: 0, EndMinute: 1440},
	}

	t.Run("always is always in schedule", func(t *testing.T) {
		for _, ts := range []string{"2026-08-03 00:00", "2026-08-03 23:59", "2026-08-09 12:00"} {
			if !InSchedule(always, mustUTC(t, ts)) {
				t.Errorf("InSchedule(always, %s) = false, want true", ts)
			}
		}
	})

	t.Run("weekends excludes weekdays", func(t *testing.T) {
		// 2026-08-03 is a Monday.
		if InSchedule(weekends, mustUTC(t, "2026-08-03 12:00")) {
			t.Error("Monday should not be in the weekends schedule")
		}
		// 2026-08-08/09 are Sat/Sun.
		if !InSchedule(weekends, mustUTC(t, "2026-08-08 12:00")) {
			t.Error("Saturday should be in the weekends schedule")
		}
		if !InSchedule(weekends, mustUTC(t, "2026-08-09 12:00")) {
			t.Error("Sunday should be in the weekends schedule")
		}
	})

	t.Run("no windows configured means never in schedule", func(t *testing.T) {
		if InSchedule(nil, mustUTC(t, "2026-08-03 12:00")) {
			t.Error("InSchedule with zero windows should always be false")
		}
	})

	t.Run("multiple windows the same day (custom, non-contiguous periods)", func(t *testing.T) {
		lunchBreakExcluded := []Window{
			{Weekday: time.Monday, StartMinute: 9 * 60, EndMinute: 12 * 60},
			{Weekday: time.Monday, StartMinute: 13 * 60, EndMinute: 18 * 60},
		}
		if !InSchedule(lunchBreakExcluded, mustUTC(t, "2026-08-03 10:00")) {
			t.Error("10:00 should be in the morning period")
		}
		if InSchedule(lunchBreakExcluded, mustUTC(t, "2026-08-03 12:30")) {
			t.Error("12:30 (lunch) should not be in schedule")
		}
		if !InSchedule(lunchBreakExcluded, mustUTC(t, "2026-08-03 17:00")) {
			t.Error("17:00 should be in the afternoon period")
		}
	})
}
