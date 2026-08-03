package autoresponse

import (
	"testing"
	"time"
)

func TestWeekdayConventionIsGoNotISO(t *testing.T) {
	// Pinned explicitly: Postgres EXTRACT(DOW ...) also has Sunday=0, so SQL
	// and Go can never disagree on this convention — but only as long as
	// nobody "corrects" it to ISO (Monday=1..Sunday=7) by mistake.
	want := map[time.Weekday]int{
		time.Sunday: 0, time.Monday: 1, time.Tuesday: 2, time.Wednesday: 3,
		time.Thursday: 4, time.Friday: 5, time.Saturday: 6,
	}
	for wd, n := range want {
		if int(wd) != n {
			t.Errorf("int(time.%v) = %d, want %d", wd, int(wd), n)
		}
	}
}

func TestParseClock(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"00:00", 0, false},
		{"09:00", 540, false},
		{"9:00", 540, false},
		{"18:00", 1080, false},
		{"23:59", 1439, false},
		{"24:00", 1440, false},
		{"24:01", 0, true},
		{"25:00", 0, true},
		{"09:60", 0, true},
		{"-1:00", 0, true},
		{"09", 0, true},
		{"", 0, true},
		{"garbage", 0, true},
	}
	for _, c := range cases {
		got, err := ParseClock(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseClock(%q) = %d, nil; want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseClock(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseClock(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFormatClock(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "00:00"},
		{9, "00:09"},
		{540, "09:00"},
		{1080, "18:00"},
		{1439, "23:59"},
		{1440, "24:00"},
	}
	for _, c := range cases {
		if got := FormatClock(c.in); got != c.want {
			t.Errorf("FormatClock(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClockRoundTrip(t *testing.T) {
	for _, s := range []string{"00:00", "00:01", "01:23", "09:00", "18:00", "23:59", "24:00"} {
		min, err := ParseClock(s)
		if err != nil {
			t.Fatalf("ParseClock(%q): %v", s, err)
		}
		if got := FormatClock(min); got != s {
			t.Errorf("FormatClock(ParseClock(%q)) = %q, want %q", s, got, s)
		}
	}
}

func TestNormalizeSplitsWrapIntoSameWeekdayPair(t *testing.T) {
	// "18:00-09:00 on Monday" is the same-weekday complement of [09:00,18:00),
	// never a span into Tuesday.
	in := []Window{{Weekday: time.Monday, StartMin: 1080, EndMin: 540}}
	got := Normalize(in)
	want := []Window{
		{Weekday: time.Monday, StartMin: 0, EndMin: 540},
		{Weekday: time.Monday, StartMin: 1080, EndMin: 1440},
	}
	assertWindowsEqual(t, got, want)
}

func TestNormalizeKeepsNonWrappingWindowAsIs(t *testing.T) {
	in := []Window{{Weekday: time.Saturday, StartMin: 0, EndMin: 1440}}
	assertWindowsEqual(t, Normalize(in), in)
}

func TestNormalizeMergesOverlaps(t *testing.T) {
	in := []Window{
		{Weekday: time.Tuesday, StartMin: 540, EndMin: 720}, // 09:00-12:00
		{Weekday: time.Tuesday, StartMin: 660, EndMin: 900}, // 11:00-15:00, overlaps
	}
	want := []Window{{Weekday: time.Tuesday, StartMin: 540, EndMin: 900}}
	assertWindowsEqual(t, Normalize(in), want)
}

func TestNormalizeMergesTouchingIntervals(t *testing.T) {
	in := []Window{
		{Weekday: time.Wednesday, StartMin: 0, EndMin: 540},
		{Weekday: time.Wednesday, StartMin: 540, EndMin: 1440},
	}
	want := []Window{{Weekday: time.Wednesday, StartMin: 0, EndMin: 1440}}
	assertWindowsEqual(t, Normalize(in), want)
}

func TestNormalizeDoesNotMergeAcrossWeekdays(t *testing.T) {
	// Fri [18:00,24:00) and Sat [00:00,24:00) are legitimate adjacent-looking
	// windows on DIFFERENT weekdays — Normalize must never fuse them, since
	// there is no wrap/spill concept in this design at all.
	in := []Window{
		{Weekday: time.Friday, StartMin: 1080, EndMin: 1440},
		{Weekday: time.Saturday, StartMin: 0, EndMin: 1440},
	}
	assertWindowsEqual(t, Normalize(in), in)
}

func TestNormalizeDropsZeroLengthWindow(t *testing.T) {
	in := []Window{{Weekday: time.Sunday, StartMin: 600, EndMin: 600}}
	if got := Normalize(in); len(got) != 0 {
		t.Fatalf("Normalize(zero-length) = %v, want empty", got)
	}
}

func TestNormalizeSortsOutput(t *testing.T) {
	in := []Window{
		{Weekday: time.Saturday, StartMin: 0, EndMin: 1440},
		{Weekday: time.Monday, StartMin: 1080, EndMin: 1440},
		{Weekday: time.Monday, StartMin: 0, EndMin: 540},
	}
	want := []Window{
		{Weekday: time.Monday, StartMin: 0, EndMin: 540},
		{Weekday: time.Monday, StartMin: 1080, EndMin: 1440},
		{Weekday: time.Saturday, StartMin: 0, EndMin: 1440},
	}
	assertWindowsEqual(t, Normalize(in), want)
}

func TestNormalizeIsIdempotent(t *testing.T) {
	in := []Window{
		{Weekday: time.Monday, StartMin: 1080, EndMin: 540},
		{Weekday: time.Saturday, StartMin: 0, EndMin: 1440},
	}
	once := Normalize(in)
	twice := Normalize(once)
	assertWindowsEqual(t, once, twice)
}

func TestNormalizeEmptyInput(t *testing.T) {
	if got := Normalize(nil); len(got) != 0 {
		t.Fatalf("Normalize(nil) = %v, want empty", got)
	}
}

// TestCoverage168Hours is the exhaustive assertion the design doc calls out
// by name: every hour of the week, for the user's own example schedule
// (Пн-Пт вне интервала 09:00-18:00 + Сб/Вс круглосуточно), must classify
// correctly. This is the only test that would catch the "Monday
// 00:00-09:00 matches nothing" bug class a wrap-anchored encoding produces.
func TestCoverage168Hours(t *testing.T) {
	var raw []Window
	for _, wd := range []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday} {
		raw = append(raw, Window{Weekday: wd, StartMin: 1080, EndMin: 540}) // outside 09:00-18:00
	}
	raw = append(raw,
		Window{Weekday: time.Saturday, StartMin: 0, EndMin: 1440},
		Window{Weekday: time.Sunday, StartMin: 0, EndMin: 1440},
	)
	windows := Normalize(raw)

	loc := time.UTC
	// Any anchor works: 7 consecutive days hit every weekday exactly once,
	// and "expected" below is derived from each constructed instant's OWN
	// Weekday()/Hour(), never from an independently memorized calendar fact.
	anchor := time.Date(2026, 1, 4, 0, 0, 0, 0, loc)
	for day := 0; day < 7; day++ {
		for hour := 0; hour < 24; hour++ {
			at := anchor.AddDate(0, 0, day).Add(time.Duration(hour) * time.Hour)
			wd := at.Weekday()
			want := wd == time.Saturday || wd == time.Sunday || hour < 9 || hour >= 18
			if got := Covers(at, loc, windows); got != want {
				t.Errorf("Covers(%v %02d:00 %s) = %v, want %v", wd, hour, at.Format(time.RFC3339), got, want)
			}
		}
	}
}

// TestCoversDSTSpringForward covers the skipped hour: 2026-03-29 is the
// Sunday EU clocks jump 02:00 CET -> 03:00 CEST at 2026-03-29T01:00:00Z.
// Covers must classify correctly across the jump without ever constructing
// (and thus never being confused by) the wall-clock hour that never happens.
func TestCoversDSTSpringForward(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("Europe/Berlin tzdata unavailable: %v", err)
	}
	windows := []Window{{Weekday: time.Sunday, StartMin: 60, EndMin: 240}} // 01:00-04:00 local

	beforeJump := time.Date(2026, 3, 29, 0, 30, 0, 0, time.UTC) // 01:30 CET
	atJump := time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC)      // reads 03:00:00 CEST, not 02:00
	afterJump := time.Date(2026, 3, 29, 1, 30, 0, 0, time.UTC)  // 03:30 CEST

	if local := atJump.In(loc); local.Hour() != 3 || local.Minute() != 0 {
		t.Fatalf("sanity check: local time at the transition = %v, want 03:00", local)
	}
	for _, at := range []time.Time{beforeJump, atJump, afterJump} {
		if !Covers(at, loc, windows) {
			t.Errorf("Covers(%v) = false, want true (spans the spring-forward jump)", at)
		}
	}
}

// TestCoversDSTFallBack covers the doubled hour: 2026-10-25 is the Sunday EU
// clocks fall back 03:00 CEST -> 02:00 CET at 2026-10-25T01:00:00Z, so
// wall-clock 02:30 happens twice, one real hour apart. Covers has no notion
// of "already answered this wall-clock reading" (it is pure and stateless),
// so both passes must classify identically.
func TestCoversDSTFallBack(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("Europe/Berlin tzdata unavailable: %v", err)
	}
	windows := []Window{{Weekday: time.Sunday, StartMin: 0, EndMin: 151}} // 00:00-02:31 local

	firstPass := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC)  // 02:30 CEST
	secondPass := time.Date(2026, 10, 25, 1, 30, 0, 0, time.UTC) // 02:30 CET, one hour later in real time

	f, s := firstPass.In(loc), secondPass.In(loc)
	if f.Hour() != 2 || f.Minute() != 30 || s.Hour() != 2 || s.Minute() != 30 {
		t.Fatalf("sanity check: first pass = %v, second pass = %v, want both 02:30", f, s)
	}
	if !Covers(firstPass, loc, windows) {
		t.Errorf("Covers(first pass through 02:30) = false, want true")
	}
	if !Covers(secondPass, loc, windows) {
		t.Errorf("Covers(second pass through 02:30) = false, want true")
	}
}

// TestAsiaAlmatyIsPlus5 guards against stale tzdata: Kazakhstan moved
// Asia/Almaty from UTC+6 to UTC+5 on 2024-03-01. A container built from a
// pre-2024a tzdata image would shift every schedule on this (the default)
// timezone by an hour with no error anywhere — this test is what catches
// that, since importing time/tzdata alone would not.
func TestAsiaAlmatyIsPlus5(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Fatalf("load Asia/Almaty: %v", err)
	}
	at := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC) // well clear of any transition
	_, offset := at.In(loc).Zone()
	if want := 5 * 60 * 60; offset != want {
		t.Fatalf("Asia/Almaty UTC offset = %ds, want %ds (+5h) — stale tzdata?", offset, want)
	}
}

func TestDefaultPolicyIsDisabled(t *testing.T) {
	p := DefaultPolicy()
	if p.Enabled {
		t.Fatal("DefaultPolicy().Enabled = true, want false — must change nothing until explicitly configured")
	}
	if p.ReplyMode != ReplyModeAI {
		t.Errorf("DefaultPolicy().ReplyMode = %q, want %q", p.ReplyMode, ReplyModeAI)
	}
	if len(p.Windows) != 0 {
		t.Errorf("DefaultPolicy().Windows = %v, want empty", p.Windows)
	}
}

func assertWindowsEqual(t *testing.T, got, want []Window) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d windows %+v, want %d %+v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("window[%d] = %+v, want %+v\nfull got=%+v\nfull want=%+v", i, got[i], want[i], got, want)
		}
	}
}
