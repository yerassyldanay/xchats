package whatsapp

import (
	"strings"

	"github.com/yerassyldanay/xchats/backend/internal/config"
)

// IsLID reports whether jid is a Linked-Device (privacy) identifier rather
// than a phone-number JID — one half of WhatsApp's LID/phone dual identity.
func IsLID(jid string) bool { return strings.HasSuffix(jid, "@lid") }

// IsPhoneJID reports whether jid is a phone-number JID
// ("<digits>@s.whatsapp.net").
func IsPhoneJID(jid string) bool { return strings.HasSuffix(jid, "@s.whatsapp.net") }

// IsGroupJID reports whether jid addresses a group chat rather than a 1:1 contact.
func IsGroupJID(jid string) bool { return strings.HasSuffix(jid, "@g.us") }

// ChatKey picks the identity a chat is keyed on: the LID when the contact has
// one (WhatsApp's default under its privacy scheme), else the phone JID. The
// phone JID is always carried alongside as the alt identity — see SendTarget.
func ChatKey(phoneJID, lidJID string) string {
	if lidJID != "" {
		return lidJID
	}
	return phoneJID
}

// SendTarget returns the JID an outbound message must actually be addressed
// to. WhatsApp does not accept sends to an "@lid" address, so the phone JID
// always wins when both are known; a LID-only contact (no phone on file) is a
// known limitation — see the integration plan's Risks section.
func SendTarget(phoneJID, lidJID string) string {
	if phoneJID != "" {
		return phoneJID
	}
	return lidJID
}

// CanonicalJID lowercases/trims a JID and coerces a bare phone number to
// phone-JID form. It delegates to internal/config so account-id derivation
// never drifts between the WhatsApp channel and the whatsmeow adapter.
func CanonicalJID(jid string) string { return config.CanonicalJID(jid) }

// PhoneFromJID returns the numeric phone part of a phone JID
// ("7700...@s.whatsapp.net" -> "7700...").
func PhoneFromJID(jid string) string { return config.PhoneFromJID(jid) }
