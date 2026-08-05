package whatsmeow

import (
	"go.mau.fi/whatsmeow/types"

	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
)

// canonicalRef lowercases a whatsmeow JID into the same string form
// config.CanonicalJID/whatsapp.CanonicalJID produce, with the AD (agent/
// device) suffix stripped — a device-specific JID must never leak into
// account/contact identity derivation.
func canonicalRef(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	return whatsapp.CanonicalJID(jid.ToNonAD().String())
}

// resolveOwnerJID canonicalizes a client's own device JID into the bare
// owner JID account identity is derived from.
func resolveOwnerJID(jid types.JID) string { return canonicalRef(jid) }

// resolveDMRefs resolves a 1:1 message's dual identity into (phoneJID,
// lidJID), per the production rule: chats are keyed by LID when the contact
// has one, with the phone number always carried alongside as the alt — a
// send must target the phone JID, never "@lid" (see whatsapp.SendTarget).
//
// The contact is always src.Chat (the conversation partner), regardless of
// direction; the ALT identity to consult depends on direction, since
// whatsmeow reports the alt of whichever side is not "me": SenderAlt for an
// inbound message (the contact is the sender), RecipientAlt for our own
// fromMe echo (the contact is the recipient).
func resolveDMRefs(src types.MessageSource) (phoneJID, lidJID string) {
	chat := src.Chat.ToNonAD()
	alt := src.SenderAlt
	if src.IsFromMe {
		alt = src.RecipientAlt
	}
	if chat.Server == types.HiddenUserServer {
		lidJID = canonicalRef(chat)
		if !alt.IsEmpty() {
			phoneJID = canonicalRef(alt)
		}
		return
	}
	phoneJID = canonicalRef(chat)
	if !alt.IsEmpty() && alt.Server == types.HiddenUserServer {
		lidJID = canonicalRef(alt)
	}
	return
}
