// Package campaign is the channel-neutral domain logic for outbound
// campaigns: recipient parsing/normalization, message rendering, per-account
// sending-budget math, and the campaign/recipient lifecycle rules. Like
// backend/automation and backend/messaging, this package depends on nothing
// database-, queue-, or HTTP-shaped — internal/store persists its types,
// internal/campaign orchestrates the restart-safe scheduler and runner
// around them, and internal/httpapi calls in at the edges. Keeping the
// trickiest behavior here — phone normalization, template rendering's
// empty-variable collapse, and the simultaneous-tier budget math — means it
// is exercised by plain unit tests with no database at all.
//
// This package imports backend/automation for its recurring UTC
// weekday/time Window shape and overnight-wrap Contains/InSchedule logic
// (campaign_windows/campaign_account_windows store the identical tuple
// shape automation_schedule_windows already does) — reusing that already-
// pinned logic rather than duplicating its subtle wrap-around math.
package campaign

import "fmt"

// Status is a campaign's lifecycle state.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusScheduled Status = "scheduled"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// ValidStatus reports whether s is one of the seven closed status values —
// the vocabulary the campaigns.status CHECK constraint also enforces.
func ValidStatus(s string) bool {
	switch Status(s) {
	case StatusDraft, StatusScheduled, StatusRunning, StatusPaused, StatusCompleted, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

// IsTerminal reports whether a campaign in status s can never leave it —
// Completed, Failed and Cancelled are all dead ends; every other status has
// at least one outgoing transition CanTransition allows.
func IsTerminal(s Status) bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

// CanTransition reports whether the campaign lifecycle allows moving
// directly from `from` to `to`:
//
//	draft     -> scheduled | running | cancelled
//	scheduled -> running | cancelled
//	running   -> paused | cancelled | completed | failed
//	paused    -> running | cancelled
//
// "start" is the one action that produces either scheduled or running,
// depending on whether the campaign carries a future schedule_at — the
// scheduler itself promotes scheduled -> running once that time arrives.
// Pause is the only reversible edge (running <-> paused); cancelled,
// completed and failed are terminal (see IsTerminal). draft/scheduled ->
// cancelled covers abandoning a campaign before its first send — the UI
// labels that action "Cancel" and the identical running/paused -> cancelled
// edge "Stop" once at least one message has gone out, but both are the same
// transition here; the label is a presentation-layer decision keyed on
// sent_count, not a distinct state.
func CanTransition(from, to Status) bool {
	switch from {
	case StatusDraft:
		return to == StatusScheduled || to == StatusRunning || to == StatusCancelled
	case StatusScheduled:
		return to == StatusRunning || to == StatusCancelled
	case StatusRunning:
		return to == StatusPaused || to == StatusCancelled || to == StatusCompleted || to == StatusFailed
	case StatusPaused:
		return to == StatusRunning || to == StatusCancelled
	default:
		return false
	}
}

// CanEditContent reports whether a campaign's message body, personalization
// variables, or sending account may still change. Frozen the instant any
// message has gone out (sentCount > 0) — irreversible, and independent of
// the campaign's current status: a paused campaign that already sent 3
// messages still cannot rewrite the other 2's content.
func CanEditContent(sentCount int) bool {
	return sentCount == 0
}

// CanEditPacing reports whether a campaign's name, pace, window, or
// recipient set (adding new rows, removing not-yet-contacted ones) may
// still change. True in draft/scheduled (nothing has run yet) and paused
// (the one status where editing a live campaign is explicitly safe — see
// the plan's "Editable while paused" callout); false while running, so an
// operator pauses first rather than racing the runner's own reads.
func CanEditPacing(s Status) bool {
	return s == StatusDraft || s == StatusScheduled || s == StatusPaused
}

// RecipientStatus is a persisted campaign_recipients row's lifecycle state.
type RecipientStatus string

const (
	RecipientPending RecipientStatus = "pending"
	RecipientSending RecipientStatus = "sending"
	RecipientSent    RecipientStatus = "sent"
	RecipientFailed  RecipientStatus = "failed"
	RecipientSkipped RecipientStatus = "skipped"
)

// ValidRecipientStatus reports whether s is one of the five closed
// recipient-status values — the vocabulary the campaign_recipients.status
// CHECK constraint also enforces.
func ValidRecipientStatus(s string) bool {
	switch RecipientStatus(s) {
	case RecipientPending, RecipientSending, RecipientSent, RecipientFailed, RecipientSkipped:
		return true
	}
	return false
}

// IsRecipientTerminal reports whether a recipient in status s will never be
// automatically retried or claimed again — sent and skipped are dead ends;
// failed is terminal for the automatic retry loop (backend/campaign.Retry*)
// but stays operator-requeueable via "retry failed", which resets it to
// pending explicitly rather than through this package's own transitions.
func IsRecipientTerminal(s RecipientStatus) bool {
	return s == RecipientSent || s == RecipientFailed || s == RecipientSkipped
}

// Channel names, duplicated here as plain strings rather than imported from
// backend/messaging: this package is consumed by internal/store, which must
// not import messaging (see internal/store/ingest.go's own channelName
// type and its doc comment on the dependency direction), and backend/
// campaign stays a self-contained leaf exactly like backend/automation.
const (
	ChannelWhatsApp      = "whatsapp"
	ChannelSimulator     = "simulator"
	ChannelTelegram      = "telegram"
	ChannelInstagram     = "instagram"
	ChannelMessenger     = "messenger"
	ChannelWhatsAppCloud = "whatsapp_cloud"
)

// DefaultCountryCode is the fixed default country (bare digits, no leading
// "+") NormalizePhone normalizes an unqualified number against — a v1
// simplification (this deployment's own market) rather than a
// per-organization setting; there is no schema column to override it.
const DefaultCountryCode = "7"

// ColdSendCapable reports whether channel can message a recipient with no
// prior conversation on that account. Only whatsmeow (the QR-paired
// WhatsApp leg) and the simulator can; WhatsApp Cloud, Telegram, Instagram
// and Messenger are warm-only in v1 — a recipient there must already have a
// conversation on the sending account, checked at preview (see
// ApplyUnreachable) rather than discovered as a send-time failure.
func ColdSendCapable(channel string) bool {
	switch channel {
	case ChannelWhatsApp, ChannelSimulator:
		return true
	default:
		return false
	}
}

// invalidf builds a validation error, mirroring backend/automation's own
// fmt.Errorf convention: the offending value is always inlined in the
// message, since that text is what the HTTP layer surfaces to the caller
// verbatim (see backend/internal/httpapi/campaigns.go).
func invalidf(format string, args ...any) error {
	return fmt.Errorf("campaign: "+format, args...)
}
