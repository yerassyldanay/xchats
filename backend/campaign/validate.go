package campaign

import (
	"fmt"
	"strings"
)

// MinIntervalSecondsBounds/MaxIntervalSecondsBounds bound a campaign
// account's minimum send interval — wide enough to allow a very cautious
// operator (an hour between sends) without allowing zero, which would
// defeat pacing entirely.
const (
	MinIntervalSecondsBounds = 1
	MaxIntervalSecondsBounds = 3600
)

// MaxTierWindowSeconds/MaxTierMaxSends bound one custom tier row.
// MaxTierWindowSeconds is capped at 7 days — NOT an arbitrary sanity bound
// like MaxTierMaxSends below, but load-bearing: campaign_send_log (the
// ledger Budget's attempts slice is read from) is pruned after 7 days, so a
// tier wider than that would silently undercount once its own oldest
// attempts age out of the ledger before they age out of the tier's own
// window, letting an account send MORE than the configured cap. 100000
// sends in a single window is, by contrast, just a fat-fingered-value
// guard — far past anything a real account could survive anyway.
const (
	MinTierWindowSeconds = 60
	MaxTierWindowSeconds = 7 * 24 * 3600
	MinTierMaxSends      = 1
	MaxTierMaxSends      = 100000
)

// ValidateName reports whether a campaign name is non-empty once trimmed —
// the one required field with no other shape constraint.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return invalidf("name is required")
	}
	return nil
}

// ValidateMessageBody reports whether a campaign's message template is
// non-empty once trimmed. Render's own empty-variable collapse can still
// legitimately produce a short (even near-empty) rendered message for a
// specific recipient; this only guards the TEMPLATE itself.
func ValidateMessageBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return invalidf("message body is required")
	}
	return nil
}

// ValidateCountryCode reports whether a default country code (digits only,
// no leading "+", e.g. "7") is a plausible calling-code prefix.
func ValidateCountryCode(code string) error {
	if code == "" {
		return invalidf("default country code is required")
	}
	if len(code) > 3 {
		return invalidf("default country code must be 1-3 digits, got %q", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return invalidf("default country code must be digits only, got %q", code)
		}
	}
	return nil
}

// ValidatePace reports whether a min-interval/jitter pair is sane: the
// interval itself in bounds, jitter non-negative, and jitter never
// exceeding the interval — a jitter larger than the interval could produce
// a negative gap between sends, which is not "extra randomness", it is
// pacing turned off.
func ValidatePace(minIntervalSeconds, jitterSeconds int) error {
	if minIntervalSeconds < MinIntervalSecondsBounds || minIntervalSeconds > MaxIntervalSecondsBounds {
		return invalidf("min_interval_seconds must be between %d and %d, got %d", MinIntervalSecondsBounds, MaxIntervalSecondsBounds, minIntervalSeconds)
	}
	if jitterSeconds < 0 {
		return invalidf("jitter_seconds must not be negative, got %d", jitterSeconds)
	}
	if jitterSeconds > minIntervalSeconds {
		return invalidf("jitter_seconds (%d) must not exceed min_interval_seconds (%d)", jitterSeconds, minIntervalSeconds)
	}
	return nil
}

// ValidateTier reports whether one custom rolling-window tier's shape is
// sane — both fields strictly positive and within a generous sanity bound.
func ValidateTier(t Tier) error {
	if t.WindowSeconds < MinTierWindowSeconds || t.WindowSeconds > MaxTierWindowSeconds {
		return invalidf("tier window_seconds must be between %d and %d, got %d", MinTierWindowSeconds, MaxTierWindowSeconds, t.WindowSeconds)
	}
	if t.MaxSends < MinTierMaxSends || t.MaxSends > MaxTierMaxSends {
		return invalidf("tier max_sends must be between %d and %d, got %d", MinTierMaxSends, MaxTierMaxSends, t.MaxSends)
	}
	return nil
}

// ValidateTiers reports the first invalid tier, and rejects two tiers
// sharing the same window_seconds — campaign_account_limits' PRIMARY KEY is
// (account_id, window_seconds), so a duplicate window can never actually
// persist, and surfacing that as a validation error is clearer than a raw
// constraint-violation message.
func ValidateTiers(tiers []Tier) error {
	seen := make(map[int]bool, len(tiers))
	for i, t := range tiers {
		if err := ValidateTier(t); err != nil {
			return fmt.Errorf("tiers[%d]: %w", i, err)
		}
		if seen[t.WindowSeconds] {
			return invalidf("tiers[%d]: duplicate window_seconds %d", i, t.WindowSeconds)
		}
		seen[t.WindowSeconds] = true
	}
	return nil
}
