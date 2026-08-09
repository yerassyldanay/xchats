package messengerish

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadGolden(t *testing.T, name string) WebhookPayload {
	t.Helper()
	// name is always one of this file's own hardcoded literals below, never
	// external input.
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "meta", name)) // #nosec G304 -- fixed test fixture dir, literal callers only
	if err != nil {
		t.Fatalf("read golden payload %s: %v", name, err)
	}
	var p WebhookPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal golden payload %s: %v", name, err)
	}
	return p
}

func TestParseInboundTextGolden(t *testing.T) {
	p := loadGolden(t, "instagram_inbound_text.json")
	if p.Object != "instagram" {
		t.Fatalf("object = %q", p.Object)
	}
	if len(p.Entry) != 1 || p.Entry[0].ID != "17841400000000001" {
		t.Fatalf("entry = %+v", p.Entry)
	}
	events := p.Entry[0].Messaging
	if len(events) != 1 {
		t.Fatalf("messaging = %d, want 1", len(events))
	}
	e := events[0]
	if e.Sender.ID != "1234567890123456" || e.Recipient.ID != "17841400000000001" {
		t.Fatalf("event = %+v", e)
	}
	if reason, ignorable := e.Ignorable(); ignorable {
		t.Fatalf("a plain text message is ignorable: %q", reason)
	}
	if e.Body() != "Здравствуйте, есть в наличии?" {
		t.Fatalf("body = %q", e.Body())
	}
	if e.MessageKind() != "conversation" {
		t.Fatalf("message kind = %q, want conversation", e.MessageKind())
	}
	if e.Attachment() != nil {
		t.Fatal("a text message must have no attachment")
	}
	if e.Message.IsEcho {
		t.Fatal("a genuine inbound message must not be flagged as an echo")
	}
	ts := ParseTimestampMillis(e.Timestamp)
	if ts.Unix() != 1723104000 {
		t.Fatalf("parsed timestamp = %v", ts)
	}
}

func TestParseInboundImageGolden(t *testing.T) {
	p := loadGolden(t, "instagram_inbound_image.json")
	e := p.Entry[0].Messaging[0]
	att := e.Attachment()
	if att == nil {
		t.Fatal("expected an attachment")
	}
	if att.Type != "image" || att.Payload.URL != "https://cdn.instagram.com/fake/photo.jpg" {
		t.Fatalf("attachment = %+v", att)
	}
	if e.MessageKind() != "imageMessage" {
		t.Fatalf("message kind = %q, want imageMessage", e.MessageKind())
	}
	if e.Body() != "" {
		t.Fatalf("body = %q, want empty (Instagram attachments carry no caption)", e.Body())
	}
	if e.Preview() != "📷 Фото" {
		t.Fatalf("preview = %q", e.Preview())
	}
	if reason, ignorable := e.Ignorable(); ignorable {
		t.Fatalf("an image message is ignorable: %q", reason)
	}
}

func TestParseEchoGolden(t *testing.T) {
	p := loadGolden(t, "instagram_echo.json")
	e := p.Entry[0].Messaging[0]
	if !e.Message.IsEcho {
		t.Fatal("expected is_echo=true")
	}
	// On an echo, sender/recipient are SWAPPED relative to a genuine inbound
	// message — sender is now OUR OWN account.
	if e.Sender.ID != "17841400000000001" || e.Recipient.ID != "1234567890123456" {
		t.Fatalf("echo event = %+v", e)
	}
	if reason, ignorable := e.Ignorable(); ignorable {
		t.Fatalf("an echo of a text send is ignorable: %q", reason)
	}
}

func TestIgnorableNonMessageEvent(t *testing.T) {
	e := MessagingEvent{Sender: Party{ID: "1"}, Recipient: Party{ID: "2"}}
	reason, ignorable := e.Ignorable()
	if !ignorable || reason != "non_message_event" {
		t.Fatalf("reason=%q ignorable=%v, want non_message_event/true", reason, ignorable)
	}
}

func TestIgnorableDeletedMessage(t *testing.T) {
	e := MessagingEvent{Message: &Message{MID: "x", Text: "gone", IsDeleted: true}}
	reason, ignorable := e.Ignorable()
	if !ignorable || reason != "message_deleted" {
		t.Fatalf("reason=%q ignorable=%v, want message_deleted/true", reason, ignorable)
	}
}

func TestIgnorableUnsupportedAttachmentTypes(t *testing.T) {
	for _, typ := range []string{"share", "location", "fallback", "template"} {
		e := MessagingEvent{Message: &Message{MID: "x", Attachments: []Attachment{{Type: typ}}}}
		reason, ignorable := e.Ignorable()
		if !ignorable {
			t.Errorf("type %q should be ignorable", typ)
		}
		if reason == "" {
			t.Errorf("type %q: expected a non-empty ignore reason", typ)
		}
	}
}

func TestIgnorableEmptyMessage(t *testing.T) {
	e := MessagingEvent{Message: &Message{MID: "x"}}
	reason, ignorable := e.Ignorable()
	if !ignorable || reason != "empty_message" {
		t.Fatalf("reason=%q ignorable=%v, want empty_message/true", reason, ignorable)
	}
}

func TestParseTimestampMillisNonPositiveFallsBackToZero(t *testing.T) {
	if got := ParseTimestampMillis(0); !got.IsZero() {
		t.Fatalf("ParseTimestampMillis(0) = %v, want zero", got)
	}
	if got := ParseTimestampMillis(-1); !got.IsZero() {
		t.Fatalf("ParseTimestampMillis(-1) = %v, want zero", got)
	}
}
