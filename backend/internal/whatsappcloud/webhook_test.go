package whatsappcloud

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadGolden(t *testing.T, name string) WebhookPayload {
	t.Helper()
	// name is always one of this file's own hardcoded literals (see the
	// TestParse* callers below), never external input.
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
	p := loadGolden(t, "whatsapp_cloud_inbound_text.json")
	if p.Object != "whatsapp_business_account" {
		t.Fatalf("object = %q", p.Object)
	}
	if len(p.Entry) != 1 || p.Entry[0].ID != "waba-1000" {
		t.Fatalf("entry = %+v", p.Entry)
	}
	change := p.Entry[0].Changes[0]
	if change.Field != "messages" {
		t.Fatalf("field = %q", change.Field)
	}
	if change.Value.Metadata.PhoneNumberID != "15550001111" {
		t.Fatalf("phone_number_id = %q", change.Value.Metadata.PhoneNumberID)
	}
	if len(change.Value.Contacts) != 1 || change.Value.Contacts[0].WAID != "77011234567" {
		t.Fatalf("contacts = %+v", change.Value.Contacts)
	}
	if change.Value.Contacts[0].Profile.Name != "Клиент Тест" {
		t.Fatalf("profile name = %q", change.Value.Contacts[0].Profile.Name)
	}
	msgs := change.Value.Messages
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	m := msgs[0]
	if m.From != "77011234567" || m.Type != "text" {
		t.Fatalf("message = %+v", m)
	}
	if reason, ignorable := m.Ignorable(); ignorable {
		t.Fatalf("a plain text message is ignorable: %q", reason)
	}
	if m.Body() != "Здравствуйте, есть в наличии?" {
		t.Fatalf("body = %q", m.Body())
	}
	if m.MessageKind() != "conversation" {
		t.Fatalf("message kind = %q, want conversation", m.MessageKind())
	}
	if m.Attachment() != nil {
		t.Fatal("a text message must have no attachment")
	}
	ts := ParseTimestamp(m.Timestamp)
	if ts.Unix() != 1723104000 {
		t.Fatalf("parsed timestamp = %v", ts)
	}
}

func TestParseInboundImageGolden(t *testing.T) {
	p := loadGolden(t, "whatsapp_cloud_inbound_image.json")
	m := p.Entry[0].Changes[0].Value.Messages[0]
	if m.Type != "image" {
		t.Fatalf("type = %q", m.Type)
	}
	att := m.Attachment()
	if att == nil {
		t.Fatal("expected an attachment")
	}
	if att.ID != "media-abc123" || att.MimeType != "image/jpeg" {
		t.Fatalf("attachment = %+v", att)
	}
	if m.MessageKind() != "imageMessage" {
		t.Fatalf("message kind = %q, want imageMessage", m.MessageKind())
	}
	// The caption doubles as the body — same convention Telegram's ingest uses.
	if m.Body() != "Вот скриншот ошибки" {
		t.Fatalf("body = %q, want the caption", m.Body())
	}
	if reason, ignorable := m.Ignorable(); ignorable {
		t.Fatalf("an image message is ignorable: %q", reason)
	}
}

func TestPreviewFallsBackToPlaceholderWithNoCaption(t *testing.T) {
	m := Message{Type: "image", Image: &Media{ID: "x"}}
	if got := m.Preview(); got != "📷 Фото" {
		t.Fatalf("preview = %q, want the placeholder", got)
	}
	if got := m.Body(); got != "" {
		t.Fatalf("body = %q, want empty (no caption)", got)
	}
}

func TestParseStatusDeliveredGolden(t *testing.T) {
	p := loadGolden(t, "whatsapp_cloud_status_delivered.json")
	statuses := p.Entry[0].Changes[0].Value.Statuses
	if len(statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(statuses))
	}
	s := statuses[0]
	if s.Status != "delivered" || s.RecipientID != "77011234567" {
		t.Fatalf("status = %+v", s)
	}
	if DeliveryRank(s.Status) != 2 {
		t.Fatalf("DeliveryRank(delivered) = %d, want 2", DeliveryRank(s.Status))
	}
	if len(p.Entry[0].Changes[0].Value.Messages) != 0 {
		t.Fatal("a status-only delivery must carry no messages")
	}
}

func TestParseStatusFailedGolden(t *testing.T) {
	p := loadGolden(t, "whatsapp_cloud_status_failed.json")
	s := p.Entry[0].Changes[0].Value.Statuses[0]
	if s.Status != "failed" {
		t.Fatalf("status = %q", s.Status)
	}
	if len(s.Errors) != 1 || s.Errors[0].Code != 131053 {
		t.Fatalf("errors = %+v", s.Errors)
	}
	// The monotonic ladder deliberately ranks a late failure ABOVE read
	// (queued=0 < sent=1 < delivered=2 < read=3 < failed=4) — see
	// store.AdvanceDeliveryState's own doc comment.
	if got, want := DeliveryRank("failed"), DeliveryRank("read"); got <= want {
		t.Fatalf("DeliveryRank(failed)=%d must outrank DeliveryRank(read)=%d", got, want)
	}
}

func TestDeliveryRankUnknownStatusIsZero(t *testing.T) {
	if got := DeliveryRank("something-new-meta-invented"); got != 0 {
		t.Fatalf("DeliveryRank(unknown) = %d, want 0", got)
	}
}

func TestIgnorableUnsupportedMessageTypes(t *testing.T) {
	for _, typ := range []string{"location", "contacts", "button", "interactive", "reaction", "system", "unknown"} {
		m := Message{Type: typ}
		reason, ignorable := m.Ignorable()
		if !ignorable {
			t.Errorf("type %q should be ignorable", typ)
		}
		if reason == "" {
			t.Errorf("type %q: expected a non-empty ignore reason", typ)
		}
	}
}

func TestParseTimestampInvalidFallsBackToZero(t *testing.T) {
	if got := ParseTimestamp("not-a-number"); !got.IsZero() {
		t.Fatalf("ParseTimestamp(garbage) = %v, want zero", got)
	}
}
