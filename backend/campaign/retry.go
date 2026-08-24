package campaign

import "time"

// TransientBackoff is the fixed wait ladder a transient send failure steps
// through before its recipient is given up on: 1m, then 5m, then 25m. A
// campaign send is not worth retrying indefinitely (unlike, say, a queue
// consumer) — three bounded attempts past the first is the plan's own fixed
// schedule, not an open-ended exponential backoff.
var TransientBackoff = []time.Duration{1 * time.Minute, 5 * time.Minute, 25 * time.Minute}

// NextRetry returns the backoff to wait before retrying a recipient whose
// attemptNumber-th attempt (1-based, matching store.Claim.Attempts /
// campaign_recipients.attempts) just failed transiently. ok is false once
// attemptNumber exceeds len(TransientBackoff) — the recipient has exhausted
// its retry ladder and must be finalized as terminally failed instead of
// scheduled for another attempt.
func NextRetry(attemptNumber int) (wait time.Duration, ok bool) {
	if attemptNumber < 1 || attemptNumber > len(TransientBackoff) {
		return 0, false
	}
	return TransientBackoff[attemptNumber-1], true
}
