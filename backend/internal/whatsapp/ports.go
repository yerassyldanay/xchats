package whatsapp

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

// Manager is the port httpapi depends on for the WhatsApp account lifecycle:
// pairing, logout, and status. internal/whatsmeow's Manager is the only
// production implementation; the interface exists so httpapi (and its tests)
// never need to import whatsmeow-specific types.
type Manager interface {
	// Pair starts a new QR pairing flow for orgID (the caller's organization
	// — resolved from the session at the HTTP layer, which is the only place
	// that has one) and returns a session id to poll via PairStatus. The
	// flow keeps running in the background; ctx bounds only the initial
	// handshake, not the whole pairing window.
	Pair(ctx context.Context, orgID string) (sessionID string, err error)
	// PairStatus returns the latest known state of a pairing session started
	// by Pair. ok is false for an unknown or expired session id.
	PairStatus(sessionID string) (update PairingUpdate, ok bool)
	// Logout disconnects an account, deletes its whatsmeow device session,
	// and marks it logged out. accountID is a wa_accounts.id.
	Logout(ctx context.Context, accountID string) error
	// Status returns an account's current connection state.
	Status(ctx context.Context, accountID string) (ConnectionChanged, error)
	// InjectDebugEvent feeds a synthetic event through the exact same
	// ingest path a real whatsmeow event takes — see DebugEvent's doc
	// comment. Gated at the HTTP layer by SIMULATOR_ENABLED.
	InjectDebugEvent(ctx context.Context, evt DebugEvent) (DebugEventResult, error)
	// IsOnWhatsApp batch-checks phone registration through accountID's live
	// connection — Campaigns' preview-time reachability check and its
	// send-time permanent-failure classification. The returned map is
	// keyed by the same digits-only, no-"+" form
	// backend/campaign.NormalizePhone produces; a phone absent from it was
	// not confirmed one way or the other and must be treated as unknown,
	// never as registered.
	IsOnWhatsApp(ctx context.Context, accountID string, phones []string) (map[string]bool, error)
}

// DebugEvent is a synthetic whatsmeow-shaped event for POST /debug/wa-event.
// EventType selects which branch fires: "message" | "receipt" | "connected"
// | "disconnected" | "logged_out". It carries only primitive fields — the
// adapter builds whatever provider-specific shape it needs internally, so
// this package (and httpapi) never touches whatsmeow types.
type DebugEvent struct {
	EventType string
	// AccountID is required here — a wa_accounts.id. It is optional in the
	// HTTP request body only: httpapi resolves (or creates) the caller's
	// simulator account when the request omits it, so the endpoint works
	// without a real paired WhatsApp number, and always passes a concrete
	// id down to this port.
	AccountID string
	// SenderJID ("message" events) is the sending contact: a raw JID or a
	// bare phone number, run through the same normalization real inbound
	// events use.
	SenderJID string
	Text      string // "message": body text
	IsFromMe  bool   // "message": exercises echo collapse when true
	// ExternalID ("message"/"receipt") pins the provider message id instead
	// of generating one — needed to test echo collapse and receipt
	// monotonicity against a specific, already-known message.
	ExternalID string
	// ReceiptType ("receipt"): "" (delivered) | "read" | "played".
	ReceiptType string
}

// DebugEventResult is what /debug/wa-event reports back for a "message" event.
type DebugEventResult struct {
	MessageID string
	ChatID    string
}

// ChannelSender is the outbound send port, restated here (as an alias, not a
// copy) so internal/whatsmeow's sender documents itself against this neutral
// package: anything satisfying messaging.ChannelSender satisfies this too.
type ChannelSender = messaging.ChannelSender
