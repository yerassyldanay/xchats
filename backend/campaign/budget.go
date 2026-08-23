package campaign

import (
	"sort"
	"time"

	pureautomation "github.com/yerassyldanay/xchats/backend/automation"
)

// Tier is one simultaneous rolling-window sending cap for an account — a
// ROW, not a column: campaign_account_limits stores one row per tier, and a
// send must satisfy every tier at once (see Budget).
type Tier struct {
	WindowSeconds int
	MaxSends      int
}

// DefaultTiers is the built-in cap set for a real send channel (whatsmeow,
// WhatsApp Cloud) with no campaign_account_limits rows of its own: 5/1h,
// 8/2h, 20/6h, 35/12h, 50/24h.
var DefaultTiers = []Tier{
	{WindowSeconds: 3600, MaxSends: 5},
	{WindowSeconds: 7200, MaxSends: 8},
	{WindowSeconds: 21600, MaxSends: 20},
	{WindowSeconds: 43200, MaxSends: 35},
	{WindowSeconds: 86400, MaxSends: 50},
}

// DefaultMinIntervalSeconds/DefaultJitterSeconds is the built-in pacing for
// a real send channel with no campaign_account_settings row of its own.
const (
	DefaultMinIntervalSeconds = 90
	DefaultJitterSeconds      = 30
)

// DefaultTiersFor returns the built-in tier set for a channel with no
// campaign_account_limits rows of its own — every real send channel gets
// DefaultTiers; the simulator (testing only) is unlimited (zero tiers, so
// Budget's tier loop never throttles it).
func DefaultTiersFor(channel string) []Tier {
	if channel == ChannelSimulator {
		return nil
	}
	return append([]Tier(nil), DefaultTiers...)
}

// DefaultPacingFor returns the built-in min-interval/jitter for a channel
// with no campaign_account_settings row of its own.
func DefaultPacingFor(channel string) (minIntervalSeconds, jitterSeconds int) {
	if channel == ChannelSimulator {
		return 0, 0
	}
	return DefaultMinIntervalSeconds, DefaultJitterSeconds
}

// Window is one recurring UTC weekday/time-of-day range — an alias for
// backend/automation's own type (see this package's doc comment for why).
type Window = pureautomation.Window

// Decision is Budget's verdict for one candidate send.
type Decision struct {
	Allowed bool
	// ThrottledBy is the WindowSeconds of the first tier found at capacity,
	// 0 if no tier blocked this send (it may still be Allowed=false purely
	// on the interval check).
	ThrottledBy int
	// NextSlotAt is the earliest time this exact send could be reattempted
	// — equal to now when Allowed is true.
	NextSlotAt time.Time
}

// Budget evaluates every tier and the minimum send interval simultaneously.
// attempts is every provider-attempt timestamp within the LARGEST tier's
// rolling window, sorted ascending (the ledger query the store runs before
// calling this — narrower tiers are derived from the same slice). lastSendAt
// is the zero Time when this account has never sent before, which always
// satisfies the interval check. A send is Allowed only when every tier has
// headroom AND minInterval has elapsed since lastSendAt.
func Budget(tiers []Tier, attempts []time.Time, minInterval time.Duration, lastSendAt time.Time, now time.Time) Decision {
	nextSlot := now
	intervalOK := lastSendAt.IsZero() || !now.Before(lastSendAt.Add(minInterval))
	if !intervalOK {
		if at := lastSendAt.Add(minInterval); at.After(nextSlot) {
			nextSlot = at
		}
	}

	throttledBy := 0
	for _, tier := range tiers {
		if tier.MaxSends <= 0 {
			continue
		}
		windowStart := now.Add(-time.Duration(tier.WindowSeconds) * time.Second)
		idx := sort.Search(len(attempts), func(i int) bool { return attempts[i].After(windowStart) })
		inWindow := attempts[idx:]
		if len(inWindow) < tier.MaxSends {
			continue
		}
		if throttledBy == 0 {
			throttledBy = tier.WindowSeconds
		}
		// The (count-max+1)-th oldest attempt in this window is the one
		// that must age out before a new send fits under the cap again.
		freesAt := inWindow[len(inWindow)-tier.MaxSends].Add(time.Duration(tier.WindowSeconds) * time.Second)
		if freesAt.After(nextSlot) {
			nextSlot = freesAt
		}
	}

	return Decision{Allowed: intervalOK && throttledBy == 0, ThrottledBy: throttledBy, NextSlotAt: nextSlot}
}

// WindowsOK reports whether now falls inside the account's quiet-hours
// windows AND (when the campaign has configured any of its own) inside the
// campaign's own windows too — a campaign can only NARROW the account's
// hard constraint, never widen it. Zero windows on either side means that
// side places no restriction: unlike backend/automation.InSchedule's own
// "zero windows = never in schedule" (right for a feature an operator must
// opt into), a campaign must not be silently blocked forever just because
// nobody has ever touched quiet hours — the safe default here is
// unrestricted, not closed.
func WindowsOK(accountWindows, campaignWindows []Window, now time.Time) bool {
	if len(accountWindows) > 0 && !pureautomation.InSchedule(accountWindows, now) {
		return false
	}
	if len(campaignWindows) > 0 && !pureautomation.InSchedule(campaignWindows, now) {
		return false
	}
	return true
}

// maxSimulationIterations bounds Estimate's inner search-for-an-open-slot
// loop per recipient, so a pathological configuration (e.g. two window sets
// with an empty intersection) reports a far-future estimate instead of
// hanging forever.
const maxSimulationIterations = 100000

// farFuture is Estimate's fallback answer when a valid send slot can never
// be found — a date far enough out that a caller formatting "≈ N days" will
// obviously read it as "effectively never" rather than a real projection.
var farFuture = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

// Estimate forward-simulates sends against tiers/interval/windows to
// project when the LAST of `remaining` pending recipients would go out,
// starting from now — the "≈ 10 days" figure the UI shows. priorAttempts
// seeds the simulation with whatever real attempts already count toward the
// tiers (sorted ascending, same contract as Budget's own attempts
// parameter) — pass nil for a clean-slate projection (a brand-new
// campaign's ETA); a live "when can the very next send happen" query
// against an account with real history should pass its actual recent
// attempts instead, or the projection would silently ignore an
// already-partly-consumed budget. Jitter is not simulated: Estimate reports
// the un-jittered projection, a reasonable approximation rather than a
// promise.
func Estimate(remaining int, priorAttempts []time.Time, tiers []Tier, minInterval time.Duration, accountWindows, campaignWindows []Window, now time.Time) time.Time {
	if remaining <= 0 {
		return now
	}
	maxWindow := 0
	for _, t := range tiers {
		if t.WindowSeconds > maxWindow {
			maxWindow = t.WindowSeconds
		}
	}

	attempts := append([]time.Time(nil), priorAttempts...)
	var lastSend time.Time
	if len(attempts) > 0 {
		lastSend = attempts[len(attempts)-1]
	}
	cursor := now
	for i := 0; i < remaining; i++ {
		slot, ok := nextAllowedSlot(tiers, attempts, minInterval, lastSend, accountWindows, campaignWindows, cursor)
		if !ok {
			return farFuture
		}
		cursor = slot
		if maxWindow > 0 {
			cutoff := cursor.Add(-time.Duration(maxWindow) * time.Second)
			j := 0
			for j < len(attempts) && !attempts[j].After(cutoff) {
				j++
			}
			attempts = attempts[j:]
		}
		attempts = append(attempts, cursor)
		lastSend = cursor
	}
	return lastSend
}

// nextAllowedSlot finds the earliest time at or after from that satisfies
// both the tier/interval budget and the window constraint, re-checking
// each against the other until they agree (a slot Budget frees up might
// fall outside the window, and vice versa).
func nextAllowedSlot(tiers []Tier, attempts []time.Time, minInterval time.Duration, lastSend time.Time, accountWindows, campaignWindows []Window, from time.Time) (time.Time, bool) {
	cursor := from
	for i := 0; i < maxSimulationIterations; i++ {
		if !WindowsOK(accountWindows, campaignWindows, cursor) {
			opened, ok := nextWindowOpen(accountWindows, campaignWindows, cursor)
			if !ok {
				return time.Time{}, false
			}
			cursor = opened
			continue
		}
		d := Budget(tiers, attempts, minInterval, lastSend, cursor)
		if d.Allowed {
			return cursor, true
		}
		cursor = d.NextSlotAt
	}
	return time.Time{}, false
}

// nextWindowOpen forward-searches, minute by minute over up to one week
// (windows recur weekly), for the earliest time at or after from that
// WindowsOK accepts. ok is false when no minute in that week qualifies —
// e.g. two non-empty window sets with no overlap at all — which the caller
// treats as "never" rather than looping forever.
func nextWindowOpen(accountWindows, campaignWindows []Window, from time.Time) (time.Time, bool) {
	t := from.Truncate(time.Minute)
	if t.Before(from) {
		t = t.Add(time.Minute)
	}
	for i := 0; i < 7*24*60+1; i++ {
		if WindowsOK(accountWindows, campaignWindows, t) {
			return t, true
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, false
}
