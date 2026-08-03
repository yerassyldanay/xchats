// Package autoresponse holds the pure scheduling logic for per-account
// auto-response: the policy shape, per-weekday window normalization, and the
// coverage test. It has no DB or HTTP dependency so it can be exhaustively
// unit tested; store, httpapi and worker each depend on it, never the other
// way around.
package autoresponse

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Window is a non-wrapping half-open interval [StartMin, EndMin) on a single
// weekday. StartMin is 0..1439, EndMin is 1..1440 (1440 == "24:00", midnight
// at the end of the day). A window never spans two weekdays: "18:00-09:00 on
// Monday" is not "Monday 18:00 through Tuesday 09:00", it is the complement
// of [09:00,18:00) on Monday alone. See Normalize.
type Window struct {
	Weekday  time.Weekday
	StartMin int
	EndMin   int
}

// Policy is one account's auto-response configuration.
type Policy struct {
	Enabled           bool
	ReplyMode         string // "ai" | "fixed"
	FixedText         string
	Timezone          string // IANA zone name
	DelaySeconds      int
	CooldownSeconds   int
	SkipWhenEscalated bool
	PauseWhenAssigned bool
	Windows           []Window
}

const (
	ReplyModeAI    = "ai"
	ReplyModeFixed = "fixed"
)

// DefaultPolicy is what every account has until an operator configures one:
// disabled, so it changes nothing about today's behaviour until opted in.
func DefaultPolicy() Policy {
	return Policy{
		Enabled:           false,
		ReplyMode:         ReplyModeAI,
		Timezone:          "Asia/Almaty",
		DelaySeconds:      120,
		CooldownSeconds:   300,
		SkipWhenEscalated: true,
		PauseWhenAssigned: false,
		Windows:           []Window{},
	}
}

// Normalize converts arbitrary input windows into the canonical stored form:
// every window is a non-wrapping interval on one weekday. A "wrap" input
// (StartMin >= EndMin, e.g. 18:00->09:00) is not a span into the next day —
// it is the same-weekday complement, split into [0,EndMin) and
// [StartMin,1440). The result is sorted by (Weekday, StartMin) and
// overlapping or touching intervals on the same weekday are merged.
// Zero-length input (StartMin == EndMin) contributes nothing. Normalize never
// rejects overlapping input; merging makes repeated edits idempotent instead
// of hostile.
func Normalize(in []Window) []Window {
	var flat []Window
	for _, w := range in {
		switch {
		case w.StartMin < w.EndMin:
			flat = append(flat, Window{Weekday: normalizeWeekday(w.Weekday), StartMin: w.StartMin, EndMin: w.EndMin})
		case w.StartMin > w.EndMin:
			wd := normalizeWeekday(w.Weekday)
			if w.EndMin > 0 {
				flat = append(flat, Window{Weekday: wd, StartMin: 0, EndMin: w.EndMin})
			}
			if w.StartMin < 1440 {
				flat = append(flat, Window{Weekday: wd, StartMin: w.StartMin, EndMin: 1440})
			}
		default:
			// StartMin == EndMin: empty interval, drop it.
		}
	}

	sort.Slice(flat, func(i, j int) bool {
		if flat[i].Weekday != flat[j].Weekday {
			return flat[i].Weekday < flat[j].Weekday
		}
		return flat[i].StartMin < flat[j].StartMin
	})

	out := make([]Window, 0, len(flat))
	for _, w := range flat {
		n := len(out)
		if n > 0 && out[n-1].Weekday == w.Weekday && w.StartMin <= out[n-1].EndMin {
			if w.EndMin > out[n-1].EndMin {
				out[n-1].EndMin = w.EndMin
			}
			continue
		}
		out = append(out, w)
	}
	return out
}

func normalizeWeekday(wd time.Weekday) time.Weekday {
	w := int(wd) % 7
	if w < 0 {
		w += 7
	}
	return time.Weekday(w)
}

// Covers reports whether instant at, viewed as local wall-clock time in loc,
// falls inside one of ws. It converts the instant to local time and reads
// off weekday + minute-of-day — it never constructs a local time from
// components, which is what keeps it well-defined through both the skipped
// and the doubled hour of a DST transition (at.In(loc) is always exact for a
// real instant; only the reverse direction, wall-clock -> instant, is
// ambiguous).
func Covers(at time.Time, loc *time.Location, ws []Window) bool {
	local := at.In(loc)
	wd := local.Weekday()
	minute := local.Hour()*60 + local.Minute()
	for _, w := range ws {
		if w.Weekday == wd && minute >= w.StartMin && minute < w.EndMin {
			return true
		}
	}
	return false
}

// ParseClock parses "HH:MM" wire format into minutes since midnight, 0..1440.
// "24:00" is the one allowed value equal to a full day and parses to 1440;
// every other hour must be 0..23.
func ParseClock(s string) (int, error) {
	h, m, err := splitClock(s)
	if err != nil {
		return 0, err
	}
	if h == 24 && m == 0 {
		return 1440, nil
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("autoresponse: clock %q out of range", s)
	}
	return h*60 + m, nil
}

func splitClock(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("autoresponse: invalid clock %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("autoresponse: invalid clock %q: %w", s, err)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("autoresponse: invalid clock %q: %w", s, err)
	}
	return h, m, nil
}

// FormatClock is ParseClock's inverse for values already known to be valid
// (0..1440); it renders 1440 back to "24:00".
func FormatClock(min int) string {
	if min == 1440 {
		return "24:00"
	}
	h := (min / 60) % 24
	m := min % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}
