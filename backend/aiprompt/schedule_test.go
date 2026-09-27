package aiprompt

import (
	"reflect"
	"testing"
)

func day(ref WeekdayRef, start, end string, breaks ...ScheduleBreak) ScheduleDay {
	return ScheduleDay{Ref: ref, Day: string(ref), Start: start, End: end, Breaks: breaks}
}

func TestNormalizeSchedule_Valid(t *testing.T) {
	t.Run("empty schedule", func(t *testing.T) {
		got, err := NormalizeSchedule(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("want empty schedule, got %+v", got)
		}
	})

	t.Run("single day no breaks", func(t *testing.T) {
		got, err := NormalizeSchedule(Schedule{day(Tuesday, "10:00", "19:00")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].Ref != Tuesday || len(got[0].Breaks) != 0 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("reorders days Monday-first and sorts breaks by start", func(t *testing.T) {
		in := Schedule{
			day(Friday, "10:00", "19:00", ScheduleBreak{Start: "15:00", End: "15:30"}, ScheduleBreak{Start: "13:00", End: "14:00"}),
			day(Monday, "09:00", "18:00"),
			day(Wednesday, "09:00", "18:00"),
		}
		got, err := NormalizeSchedule(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantOrder := []WeekdayRef{Monday, Wednesday, Friday}
		for i, ref := range wantOrder {
			if got[i].Ref != ref {
				t.Fatalf("day %d: got ref %q, want %q (full: %+v)", i, got[i].Ref, ref, got)
			}
		}
		fri := got[2]
		if fri.Breaks[0].Start != "13:00" || fri.Breaks[1].Start != "15:00" {
			t.Fatalf("breaks not sorted by start: %+v", fri.Breaks)
		}
	})

	t.Run("break touching shift/break boundaries without overlap is valid", func(t *testing.T) {
		// break at the very start and one at the very end, and two
		// back-to-back breaks that only touch, never overlap.
		got, err := NormalizeSchedule(Schedule{
			day(Monday, "10:00", "19:00",
				ScheduleBreak{Start: "10:00", End: "10:15"},
				ScheduleBreak{Start: "13:00", End: "14:00"},
				ScheduleBreak{Start: "14:00", End: "14:30"}, // touches the previous break's end, does not overlap
				ScheduleBreak{Start: "18:45", End: "19:00"},
			),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got[0].Breaks) != 4 {
			t.Fatalf("want 4 breaks preserved, got %+v", got[0].Breaks)
		}
	})
}

func TestNormalizeSchedule_Invalid(t *testing.T) {
	cases := []struct {
		name string
		in   Schedule
	}{
		{"duplicate weekday", Schedule{day(Monday, "10:00", "18:00"), day(Monday, "09:00", "17:00")}},
		{"unknown weekday ref", Schedule{{Ref: "someday", Day: "someday", Start: "10:00", End: "18:00"}}},
		{"malformed start time", Schedule{{Ref: Monday, Start: "10:0", End: "18:00"}}},
		{"malformed end time", Schedule{{Ref: Monday, Start: "10:00", End: "25:00"}}},
		{"hour out of range", Schedule{{Ref: Monday, Start: "24:00", End: "25:00"}}},
		{"minute out of range", Schedule{{Ref: Monday, Start: "10:60", End: "18:00"}}},
		{"overnight shift (start == end)", Schedule{{Ref: Monday, Start: "10:00", End: "10:00"}}},
		{"overnight shift (start > end)", Schedule{{Ref: Monday, Start: "20:00", End: "08:00"}}},
		{"break starts before shift", Schedule{day(Monday, "10:00", "18:00", ScheduleBreak{Start: "09:00", End: "10:30"})}},
		{"break ends after shift", Schedule{day(Monday, "10:00", "18:00", ScheduleBreak{Start: "17:30", End: "18:30"})}},
		{"break start after break end", Schedule{day(Monday, "10:00", "18:00", ScheduleBreak{Start: "14:00", End: "13:00"})}},
		{"overlapping breaks", Schedule{day(Monday, "10:00", "18:00",
			ScheduleBreak{Start: "13:00", End: "14:00"}, ScheduleBreak{Start: "13:30", End: "14:30"})}},
		{"more entries than weekdays", Schedule{
			day(Monday, "10:00", "18:00"), day(Tuesday, "10:00", "18:00"), day(Wednesday, "10:00", "18:00"),
			day(Thursday, "10:00", "18:00"), day(Friday, "10:00", "18:00"), day(Saturday, "10:00", "18:00"),
			day(Sunday, "10:00", "18:00"), {Ref: Monday, Start: "10:00", End: "11:00"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NormalizeSchedule(tc.in); err == nil {
				t.Fatalf("want an error, got none")
			}
		})
	}
}

func TestSchedule_DayByRef(t *testing.T) {
	s := Schedule{day(Tuesday, "10:00", "19:00")}
	if s.DayByRef(Tuesday) == nil {
		t.Fatal("want Tuesday to be found")
	}
	if s.DayByRef(Monday) != nil {
		t.Fatal("want Monday to be absent (off-day)")
	}
}

// TestShiftBoundaryCheck is the table-driven reference-implementation test
// TEST.md's verification matrix names, covering PLAN.md's half-open-interval
// rule across every weekday and every classification (on-shift, break
// overlap, off/outside) — including the TEST.md Category C scenarios for
// Alina Kim (тt-сб, 10:00-19:00) almost verbatim.
func TestShiftBoundaryCheck(t *testing.T) {
	alina := day(Tuesday, "10:00", "19:00")

	t.Run("EvaluateShiftPoint", func(t *testing.T) {
		cases := []struct {
			name string
			day  *ScheduleDay
			at   string
			want ShiftState
		}{
			{"C1 within shift (Tuesday 14:00)", &alina, "14:00", ShiftOnDuty},
			{"C2 non-working day (nil day)", nil, "14:00", ShiftOff},
			{"C3 before shift start (09:00)", &alina, "09:00", ShiftOff},
			{"exact shift start is on-shift (half-open)", &alina, "10:00", ShiftOnDuty},
			{"exact shift end is OFF (half-open, end excluded)", &alina, "19:00", ShiftOff},
			{"C3/C4 after shift end (20:30)", &alina, "20:30", ShiftOff},
			{"one minute before end is on-shift", &alina, "18:59", ShiftOnDuty},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := EvaluateShiftPoint(tc.day, tc.at)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("EvaluateShiftPoint with a break", func(t *testing.T) {
		withBreak := day(Tuesday, "10:00", "19:00", ScheduleBreak{Start: "13:00", End: "14:00"})
		cases := []struct {
			name string
			at   string
			want ShiftState
		}{
			{"before break", "12:59", ShiftOnDuty},
			{"break start is inside the break (half-open)", "13:00", ShiftBreakOverlap},
			{"middle of break", "13:30", ShiftBreakOverlap},
			{"break end is OUTSIDE the break (half-open, resumes on-shift)", "14:00", ShiftOnDuty},
			{"after break", "15:00", ShiftOnDuty},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := EvaluateShiftPoint(&withBreak, tc.at)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("EvaluateShift (interval)", func(t *testing.T) {
		cases := []struct {
			name           string
			day            *ScheduleDay
			start, end     string
			want           ShiftState
		}{
			{"C1 fully inside shift", &alina, "14:00", "15:00", ShiftOnDuty},
			{"exact shift bounds", &alina, "10:00", "19:00", ShiftOnDuty},
			{"C2 non-working day", nil, "14:00", "15:00", ShiftOff},
			{"C3 fully before shift", &alina, "08:00", "09:30", ShiftOff},
			{"C4/C5 partially overlaps end — not a valid complete interval", &alina, "18:30", "20:00", ShiftOff},
			{"partially overlaps start", &alina, "09:00", "10:30", ShiftOff},
			{"fully after shift", &alina, "19:00", "20:00", ShiftOff},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := EvaluateShift(tc.day, tc.start, tc.end)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("EvaluateShift with a break", func(t *testing.T) {
		withBreak := day(Tuesday, "10:00", "19:00", ScheduleBreak{Start: "13:00", End: "14:00"})
		cases := []struct {
			name       string
			start, end string
			want       ShiftState
		}{
			{"ends exactly at break start — touches, no overlap", "12:00", "13:00", ShiftOnDuty},
			{"starts exactly at break end — touches, no overlap", "14:00", "15:00", ShiftOnDuty},
			{"fully inside the break", "13:15", "13:45", ShiftBreakOverlap},
			{"overlaps break start", "12:30", "13:30", ShiftBreakOverlap},
			{"overlaps break end", "13:30", "14:30", ShiftBreakOverlap},
			{"exactly matches the break", "13:00", "14:00", ShiftBreakOverlap},
			{"spans the whole break", "11:00", "16:00", ShiftBreakOverlap},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := EvaluateShift(&withBreak, tc.start, tc.end)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("invalid input fails closed", func(t *testing.T) {
		if _, err := EvaluateShiftPoint(&alina, "not-a-time"); err == nil {
			t.Fatal("want an error for a malformed time")
		}
		if _, err := EvaluateShift(&alina, "15:00", "14:00"); err == nil {
			t.Fatal("want an error when interval start is not before end")
		}
		if _, err := EvaluateShift(&alina, "14:00", "14:00"); err == nil {
			t.Fatal("want an error when interval start equals end")
		}
	})
}

func TestScheduleDayText(t *testing.T) {
	d := day(Tuesday, "10:00", "19:00")
	got := scheduleDayText(d, "ru")
	want := "Вторник: 10:00–19:00"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	withOneBreak := day(Tuesday, "10:00", "19:00", ScheduleBreak{Start: "13:00", End: "14:00"})
	got = scheduleDayText(withOneBreak, "ru")
	want = "Вторник: 10:00–19:00 (перерыв 13:00–14:00)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	withTwoBreaks := day(Tuesday, "10:00", "19:00",
		ScheduleBreak{Start: "13:00", End: "14:00"}, ScheduleBreak{Start: "16:00", End: "16:15"})
	got = scheduleDayText(withTwoBreaks, "ru")
	want = "Вторник: 10:00–19:00 (перерывы 13:00–14:00, 16:00–16:15)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// KK is DRAFT machine-translated (same doctrine as registry.go's
	// DeliveryWordingKK) — only assert it is non-empty, distinct from the
	// Russian rendering, and still carries the raw times verbatim.
	gotKK := scheduleDayText(d, "kk")
	if gotKK == "" || gotKK == got {
		t.Fatalf("want a distinct non-empty Kazakh rendering, got %q", gotKK)
	}
	if !reflect.DeepEqual(extractTimes(gotKK), []string{"10:00", "19:00"}) {
		t.Fatalf("Kazakh rendering must still carry the raw times verbatim: %q", gotKK)
	}
}

func TestScheduleFullText(t *testing.T) {
	s := Schedule{day(Monday, "09:00", "18:00"), day(Wednesday, "09:00", "18:00")}
	got := scheduleFullText(s, "ru")
	want := "Понедельник: 09:00–18:00; Среда: 09:00–18:00"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if scheduleFullText(nil, "ru") != "" {
		t.Fatal("want an empty schedule to render as empty text (no token)")
	}
}

func TestScheduleReasoningLines(t *testing.T) {
	s := Schedule{day(Tuesday, "10:00", "19:00", ScheduleBreak{Start: "13:00", End: "14:00"})}
	lines := scheduleReasoningLines(s)
	if len(lines) != 7 {
		t.Fatalf("want one line per weekday (7), got %d: %+v", len(lines), lines)
	}
	if lines[0] != "mon: off" {
		t.Fatalf("want off-days stated explicitly, got %q", lines[0])
	}
	if lines[1] != "tue: 10:00-19:00 break 13:00-14:00" {
		t.Fatalf("got %q", lines[1])
	}
}

// TestSpelledOutHourPattern is CodeRabbit's PR #119 finding
// (discussion_r4110715044): the original pattern treated any spelled-out
// number stem followed by "дня" as a clock-time daypart, so ordinary
// duration/advance-notice phrases like "через два дня" (in two days) or "за
// четыре дня" (within four days) — genitive uses of "день", nothing to do
// with a clock — were misclassified as a leaked spelled-out time. Verifying
// against HEAD (not the finding's prose alone) also turned up two related,
// unreported gaps fixed the same way: "до четырёх вечера" and bare "в
// полдень"/"в полночь" did not match at all (the "ё" in "четырёх" fell
// outside the old `[а-я]` class, and полдень/полночь required a daypart
// word that never follows them in real usage), and the pattern had no
// boundary on either side, so RE2's ASCII-only \b could not stop a stem or
// daypart word from matching as a mere substring of an unrelated longer
// word ("часто вечерами" matching via "вечера" as a prefix of "вечерами").
func TestSpelledOutHourPattern(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		// must still be classified as a spelled-out clock time
		{"hour + утра", "в девять утра", true},
		{"hour + утра, different stem", "с десяти утра", true},
		{"hour with ё inflection + вечера", "до четырёх вечера", true},
		{"час + дня is a genuine clock claim", "в два часа дня", true},
		{"полдень needs no trailing daypart word", "в полдень", true},
		{"полночь needs no trailing daypart word", "в полночь", true},
		{"trailing punctuation does not hide the match", "Алина работает с десяти утра.", true},
		{"часов + вечера", "девять часов вечера", true},
		{"одиннадцать + вечера", "в одиннадцать вечера", true},
		{"twelve + часов + дня", "в двенадцать часов дня", true},
		{"полночь mid-sentence", "ровно в полночь мастер закрывает салон", true},

		// must NOT be classified as a clock time: ordinary "дня" duration/
		// advance-notice phrases (genitive of "день"), not a daypart
		{"reply-time duration", "Мы ответим через два дня", false},
		{"advance-notice duration", "Запись возможна за четыре дня", false},
		{"сегодня is unrelated to a дня days-quantity", "Сегодня мастер не работает", false},
		{"duration with three days", "через три дня мы свяжемся", false},
		{"weekly cadence in days", "мастер работает пять дней в неделю", false},
		// twelve is a numeral stem, not "час" — bare "дня" after it stays a
		// duration reading, same rule as "два дня"/"четыре дня" above
		// (accepted trade-off: a colloquial "twelve, daytime" meaning noon
		// is missed here, but "полдень" above still catches that meaning
		// directly — same documented "catch the common phrasing, not a
		// complete grammar" tradeoff bookingConfirmationRE makes).
		{"twelve + bare дня (no час word) stays a duration phrase", "в двенадцать дня", false},

		// must NOT be classified as a clock time: missing boundaries would
		// otherwise catch these as substrings of unrelated words
		{"тре inside быстрее is not a stem at a word start", "Мастер закончит быстрее вечера, если будет свободное время", false},
		{"тре inside быстрее, shorter phrasing", "он всё сделает быстрее вечера", false},
		{"вечера as a prefix of вечерами", "часто вечерами она свободна", false},
		{"вечера as a prefix of вечерами, second phrasing", "мастер часто вечерами принимает клиентов", false},
		{"утра/вечера as prefixes of утрами/вечерами", "он работает утрами и вечерами", false},
		{"утра as a prefix of утраченных (lost clients, not morning)", "десять утраченных клиентов вернулись", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := spelledOutHourPattern.MatchString(tc.text)
			if got != tc.want {
				t.Errorf("spelledOutHourPattern.MatchString(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// extractTimes pulls every HH:MM-shaped substring out of s, in order — used
// to assert that a wording change (e.g. a different language) never touches
// the raw clock times themselves.
func extractTimes(s string) []string {
	return scheduleTimePattern.FindAllString(s, -1)
}
