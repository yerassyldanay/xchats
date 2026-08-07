// Package automation is the channel-neutral domain logic for per-channel
// debounce and scheduled auto-reply: the three modes, recurring UTC schedule
// windows, and the pure decisions built from them. Like backend/messaging
// and backend/response, this package depends on nothing database-, queue-,
// or HTTP-shaped — internal/store persists its types, internal/automation
// orchestrates the restart-safe scheduler and dispatch around them, and
// internal/httpapi/internal/whatsmeow/internal/tgingest call in at the
// edges. Keeping the decisions here means the trickiest behavior — schedule
// matching across an overnight or week wrap, effective-wait resolution — is
// exercised by plain unit tests with no database at all.
package automation

import (
	"fmt"
	"time"
)

// Mode is one channel's automation mode.
type Mode string

const (
	// ModeOff generates no automatic or manual drafts at all.
	ModeOff Mode = "off"
	// ModeSuggestions generates and displays drafts without ever sending.
	ModeSuggestions Mode = "suggestions"
	// ModeScheduledAuto generates drafts and auto-sends them immediately
	// when the current UTC time falls inside the channel's schedule;
	// outside it, a draft is generated and displayed exactly like
	// ModeSuggestions.
	ModeScheduledAuto Mode = "scheduled_auto"
)

// DefaultMode is what an account with no automation_settings row uses — new
// and pre-existing channels alike default to generating suggestions, never
// auto-sending and never staying silent.
const DefaultMode = ModeSuggestions

// ValidMode reports whether s is one of the three closed mode values.
func ValidMode(s string) bool {
	switch Mode(s) {
	case ModeOff, ModeSuggestions, ModeScheduledAuto:
		return true
	}
	return false
}

// MinWaitSeconds and MaxWaitSeconds bound a channel's wait-time override —
// the inclusive range the API and the database CHECK constraint both
// enforce.
const (
	MinWaitSeconds = 0
	MaxWaitSeconds = 60
)

// ValidateWaitOverride reports whether an override is either unset (nil —
// "use the system default") or within [MinWaitSeconds, MaxWaitSeconds].
func ValidateWaitOverride(seconds *int) error {
	if seconds == nil {
		return nil
	}
	if *seconds < MinWaitSeconds || *seconds > MaxWaitSeconds {
		return fmt.Errorf("automation: wait_seconds_override must be between %d and %d, got %d", MinWaitSeconds, MaxWaitSeconds, *seconds)
	}
	return nil
}

// EffectiveWaitSeconds resolves a channel's actual debounce wait: its own
// override when set, else the system default.
func EffectiveWaitSeconds(systemDefault int, override *int) int {
	if override != nil {
		return *override
	}
	return systemDefault
}

// Deadline is lastMessageAt pushed out by waitSeconds — the debounce clock
// every inbound customer message resets. There is no maximum burst cap: a
// chat that keeps receiving messages simply keeps pushing this forward.
func Deadline(lastMessageAt time.Time, waitSeconds int) time.Time {
	return lastMessageAt.Add(time.Duration(waitSeconds) * time.Second)
}

// Window is one recurring UTC weekday/time-of-day range a scheduled_auto
// channel auto-sends within. Weekday is the window's START day
// (time.Sunday..time.Saturday, matching Go's own numbering, which the
// database's weekday column mirrors 0..6). StartMinute/EndMinute are
// minutes since UTC midnight on that day, in [0,1440). EndMinute <=
// StartMinute means the window continues past UTC midnight into the next
// day (weekday+1)%7 — an "overnight" range — rather than being empty or
// backwards.
type Window struct {
	Weekday     time.Weekday
	StartMinute int
	EndMinute   int
}

// ValidateWindow reports whether w's fields are each in their valid range
// and non-degenerate (a zero-length window is rejected rather than silently
// interpreted as either "never" or "always").
func ValidateWindow(w Window) error {
	if w.Weekday < time.Sunday || w.Weekday > time.Saturday {
		return fmt.Errorf("automation: weekday must be between 0 and 6, got %d", w.Weekday)
	}
	if w.StartMinute < 0 || w.StartMinute > 1439 {
		return fmt.Errorf("automation: start_minute must be between 0 and 1439, got %d", w.StartMinute)
	}
	if w.EndMinute < 1 || w.EndMinute > 1440 {
		return fmt.Errorf("automation: end_minute must be between 1 and 1440, got %d", w.EndMinute)
	}
	if w.EndMinute == w.StartMinute {
		return fmt.Errorf("automation: end_minute must not equal start_minute")
	}
	return nil
}

// Contains reports whether t (converted to UTC) falls inside w. Overnight
// wrap: when EndMinute <= StartMinute, w is active from
// [Weekday@StartMinute, midnight) plus [(Weekday+1)%7@0, (Weekday+1)%7@
// EndMinute) — so a Saturday-night-into-Sunday-morning window correctly
// wraps the week boundary too, since Go's time.Weekday is already a mod-7
// value.
func (w Window) Contains(t time.Time) bool {
	t = t.UTC()
	minuteOfDay := t.Hour()*60 + t.Minute()
	wd := t.Weekday()

	if w.EndMinute > w.StartMinute {
		return wd == w.Weekday && minuteOfDay >= w.StartMinute && minuteOfDay < w.EndMinute
	}
	// Overnight wrap.
	if wd == w.Weekday && minuteOfDay >= w.StartMinute {
		return true
	}
	nextDay := (w.Weekday + 1) % 7
	return wd == nextDay && minuteOfDay < w.EndMinute
}

// InSchedule reports whether t falls inside any of windows — an
// account with zero windows is never in-schedule, so a freshly switched-to
// scheduled_auto channel with no configured hours displays suggestions
// only, until an operator adds at least one window.
func InSchedule(windows []Window, t time.Time) bool {
	for _, w := range windows {
		if w.Contains(t) {
			return true
		}
	}
	return false
}
