package whatsmeow

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
)

func TestTranslateMessage_PhoneOnly(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat: types.JID{User: "77000000000", Server: types.DefaultUserServer},
			},
			ID: "MSG1", PushName: "Клиент", Timestamp: time.Unix(1000, 0),
		},
		Message: &waE2E.Message{Conversation: proto.String("привет")},
	}
	got := translateMessage("acct-1", evt)
	if got.PhoneJID != "77000000000@s.whatsapp.net" {
		t.Errorf("PhoneJID = %q", got.PhoneJID)
	}
	if got.LidJID != "" {
		t.Errorf("LidJID = %q, want empty for a phone-only contact", got.LidJID)
	}
	if got.ChatJID != got.PhoneJID {
		t.Errorf("ChatJID = %q, want the phone jid", got.ChatJID)
	}
	if got.Text != "привет" || got.Kind != "conversation" {
		t.Errorf("text/kind = %q/%q", got.Text, got.Kind)
	}
	if got.ExternalID != "MSG1" || got.AccountID != "acct-1" {
		t.Errorf("external id/account id = %q/%q", got.ExternalID, got.AccountID)
	}
	if got.FromMe || got.IsGroup {
		t.Errorf("FromMe/IsGroup should both be false: %+v", got)
	}
}

// TestTranslateMessage_LIDWithPhoneAlt is the production dual-identity rule:
// a contact addressed via "@lid" carries the phone number alongside as the
// alt, and the chat is keyed on the LID (never the reverse).
func TestTranslateMessage_LIDWithPhoneAlt(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:      types.JID{User: "999999", Server: types.HiddenUserServer},
				SenderAlt: types.JID{User: "77000000000", Server: types.DefaultUserServer},
			},
			ID: "MSG2",
		},
		Message: &waE2E.Message{Conversation: proto.String("hi")},
	}
	got := translateMessage("acct-1", evt)
	if got.LidJID != "999999@lid" {
		t.Errorf("LidJID = %q", got.LidJID)
	}
	if got.PhoneJID != "77000000000@s.whatsapp.net" {
		t.Errorf("PhoneJID = %q", got.PhoneJID)
	}
	if got.ChatJID != got.LidJID {
		t.Errorf("ChatJID = %q, want the LID (the production rule: LID preferred when present)", got.ChatJID)
	}
}

// TestTranslateMessage_FromMeUsesRecipientAlt: for our own echoed send the
// contact is the RECIPIENT, not the sender, so its alt identity comes from
// RecipientAlt — using SenderAlt here would resolve the wrong side.
func TestTranslateMessage_FromMeUsesRecipientAlt(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:         types.JID{User: "999999", Server: types.HiddenUserServer},
				IsFromMe:     true,
				RecipientAlt: types.JID{User: "77000000000", Server: types.DefaultUserServer},
			},
			ID: "MSG3",
		},
		Message: &waE2E.Message{Conversation: proto.String("echo")},
	}
	got := translateMessage("acct-1", evt)
	if got.PhoneJID != "77000000000@s.whatsapp.net" {
		t.Errorf("fromMe echo PhoneJID = %q, want resolved via RecipientAlt", got.PhoneJID)
	}
	if !got.FromMe {
		t.Error("FromMe = false, want true")
	}
}

// TestTranslateMessage_Group asserts a group event is labeled safely
// (IsGroup=true) without attempting dual-identity resolution — the service
// drops groups entirely (see manager.go's handleMessageEvent).
func TestTranslateMessage_Group(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:    types.JID{User: "12345", Server: types.GroupServer},
				IsGroup: true,
			},
			ID: "MSG4",
		},
		Message: &waE2E.Message{Conversation: proto.String("group msg")},
	}
	got := translateMessage("acct-1", evt)
	if !got.IsGroup {
		t.Error("IsGroup = false, want true")
	}
	if got.PhoneJID != "" || got.LidJID != "" {
		t.Errorf("group event should not resolve dual identity: phone=%q lid=%q", got.PhoneJID, got.LidJID)
	}
	if got.ChatJID != "12345@g.us" {
		t.Errorf("ChatJID = %q, want the raw group jid", got.ChatJID)
	}
}

func TestExtractContent(t *testing.T) {
	cases := []struct {
		name      string
		msg       *waE2E.Message
		wantKind  string
		wantText  string
		wantMedia *whatsapp.Media
	}{
		{"nil message", nil, "", "", nil},
		{"conversation", &waE2E.Message{Conversation: proto.String("hi")}, "conversation", "hi", nil},
		{"extended text", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hi2")}}, "conversation", "hi2", nil},
		{
			"image", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
				Caption: proto.String("cap"), Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(10),
			}},
			"imageMessage", "cap", &whatsapp.Media{Kind: "image", Mimetype: "image/jpeg", FileSize: 10},
		},
		{
			"document", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
				Caption: proto.String("doc cap"), Mimetype: proto.String("application/pdf"),
				FileName: proto.String("a.pdf"), FileLength: proto.Uint64(20),
			}},
			"documentMessage", "doc cap", &whatsapp.Media{Kind: "document", Mimetype: "application/pdf", FileName: "a.pdf", FileSize: 20},
		},
		{
			"sticker", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{Mimetype: proto.String("image/webp"), FileLength: proto.Uint64(5)}},
			"stickerMessage", "", &whatsapp.Media{Kind: "sticker", Mimetype: "image/webp", FileSize: 5},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, text, media := extractContent(tc.msg)
			if kind != tc.wantKind || text != tc.wantText {
				t.Errorf("got (%q, %q), want (%q, %q)", kind, text, tc.wantKind, tc.wantText)
			}
			if (media == nil) != (tc.wantMedia == nil) {
				t.Fatalf("media presence = %v, want %v", media != nil, tc.wantMedia != nil)
			}
			if media != nil && *media != *tc.wantMedia {
				t.Errorf("media = %+v, want %+v", *media, *tc.wantMedia)
			}
		})
	}
}

func TestDeliveryStateForReceipt(t *testing.T) {
	cases := []struct {
		in        types.ReceiptType
		wantState string
		wantOK    bool
	}{
		{types.ReceiptTypeDelivered, "delivered", true}, // "" — easy to regress by treating unset as "unknown"
		{types.ReceiptTypeRead, "read", true},
		{types.ReceiptTypePlayed, "read", true},
		{types.ReceiptTypeSender, "", false},
		{types.ReceiptTypeRetry, "", false},
		{types.ReceiptTypeReadSelf, "", false},
		{types.ReceiptTypePlayedSelf, "", false},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			state, ok := deliveryStateForReceipt(tc.in)
			if state != tc.wantState || ok != tc.wantOK {
				t.Errorf("deliveryStateForReceipt(%q) = (%q, %v), want (%q, %v)", tc.in, state, ok, tc.wantState, tc.wantOK)
			}
		})
	}
}

func TestTranslateReceipt_DropsEmptyMessageIDs(t *testing.T) {
	_, ok := translateReceipt("acct-1", &events.Receipt{Type: types.ReceiptTypeDelivered})
	if ok {
		t.Error("a receipt with no message ids should be dropped")
	}
}
