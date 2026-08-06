// Package providerhealth tracks whether each LLM provider's credential is
// currently working in production, live — distinct from
// internal/settings.ProviderSettings' LastVerifiedAt/LastError, which only
// reflects the last time an admin explicitly saved or tested a credential in
// the Settings UI. A key that validated fine can still fail hours later (revoked,
// expired, quota-cut) — this package is what notices that without anyone
// having to go check.
package providerhealth

import (
	"errors"
	"sync"
	"time"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// Status is one provider's current health — what a Tracker's OnChange
// callback receives and what Snapshot returns.
type Status struct {
	Provider string    `json:"provider"`
	Healthy  bool      `json:"healthy"`
	At       time.Time `json:"at"`
}

// Tracker records the outcome of every LLM call, per provider, and reports
// only genuine transitions — a provider that has been failing for an hour
// does not re-broadcast on every single request. Safe for concurrent use:
// Report is called from the response engine's request-handling goroutines,
// Snapshot from an HTTP handler goroutine.
type Tracker struct {
	mu       sync.Mutex
	status   map[string]Status
	onChange func(Status)
}

// NewTracker returns a Tracker that calls onChange (if non-nil) exactly once
// per provider health transition — never on every Report call, and never for
// a provider whose health has not yet been established either way.
func NewTracker(onChange func(Status)) *Tracker {
	return &Tracker{status: make(map[string]Status), onChange: onChange}
}

// Report records the outcome of one LLM call for provider. A nil err marks
// it healthy. Only llm.ErrProviderAuth marks it unhealthy — every other
// error (a network blip, a timeout, a rate limit, a malformed response) says
// nothing about whether the CREDENTIAL still works, so Report ignores it
// entirely rather than let a routine transient failure flap the badge.
func (t *Tracker) Report(provider string, err error) {
	var healthy bool
	switch {
	case err == nil:
		healthy = true
	case errors.Is(err, llm.ErrProviderAuth):
		healthy = false
	default:
		return
	}
	next := Status{Provider: provider, Healthy: healthy, At: time.Now().UTC()}

	t.mu.Lock()
	prev, known := t.status[provider]
	changed := !known || prev.Healthy != healthy
	t.status[provider] = next
	t.mu.Unlock()

	if changed && t.onChange != nil {
		t.onChange(next)
	}
}

// Snapshot returns the current known health of every provider Report has
// ever been called for, in no particular order, each with the timestamp of
// its last actual transition. A provider Report has never been called for
// is simply absent — "unknown" is not "unhealthy."
func (t *Tracker) Snapshot() []Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Status, 0, len(t.status))
	for _, s := range t.status {
		out = append(out, s)
	}
	return out
}
