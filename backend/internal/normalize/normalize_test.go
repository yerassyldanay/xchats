package normalize

import (
	"os"
	"path/filepath"
	"testing"
)

// capturesDir points at the package-owned Evolution v2.3.7 webhook fixtures.
const capturesDir = "testdata/evolution/webhook_bodies"

func loadCapture(t *testing.T, name string) Envelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(capturesDir, name))
	if err != nil {
		t.Fatalf("read capture %s: %v", name, err)
	}
	env, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("parse envelope %s: %v", name, err)
	}
	return env
}

func TestNormalizeText(t *testing.T) {
	env := loadCapture(t, "messages_upsert_text.json")
	if env.Event != "messages.upsert" {
		t.Fatalf("event = %q", env.Event)
	}
	m, err := env.Message()
	if err != nil || m == nil {
		t.Fatalf("Message() = %v, %v", m, err)
	}
	if m.Direction != "in" {
		t.Errorf("direction = %q, want in", m.Direction)
	}
	if m.EvolutionMessageID != "3A1FE00DBC50780B05E2" {
		t.Errorf("id = %q", m.EvolutionMessageID)
	}
	// The owner (account) JID is the top-level envelope sender, NOT the customer.
	if m.OwnerJID != "77011111111@s.whatsapp.net" {
		t.Errorf("owner = %q, want the envelope sender", m.OwnerJID)
	}
	if m.PhoneJID != "77000000000@s.whatsapp.net" {
		t.Errorf("phone = %q, want the customer remoteJidAlt", m.PhoneJID)
	}
	if m.Text != "[snapshot] webhook capture - text" {
		t.Errorf("text = %q", m.Text)
	}
	if m.Media != nil {
		t.Errorf("text message should have no media")
	}
	if m.PushName != "Test Contact" {
		t.Errorf("pushName = %q", m.PushName)
	}
	if m.IsGroup {
		t.Errorf("direct chat flagged as group")
	}
}

func TestNormalizeImage(t *testing.T) {
	env := loadCapture(t, "messages_upsert_image.json")
	m, err := env.Message()
	if err != nil || m == nil {
		t.Fatalf("Message() = %v, %v", m, err)
	}
	if m.MessageKind != "imageMessage" {
		t.Errorf("kind = %q", m.MessageKind)
	}
	if m.Media == nil {
		t.Fatalf("image message missing media")
	}
	if m.Media.Kind != "image" {
		t.Errorf("media kind = %q, want image", m.Media.Kind)
	}
	if m.Media.Mimetype != "image/jpeg" {
		t.Errorf("mimetype = %q", m.Media.Mimetype)
	}
	// fileLength arrives as a protobuf Long {low,high,unsigned}.
	if m.Media.FileSize != 87154 {
		t.Errorf("file size = %d, want 87154 (decoded Long)", m.Media.FileSize)
	}
	if m.Media.Base64 == "" {
		t.Errorf("inline base64 should be present on the image capture")
	}
	if m.Media.NeedsDownloadByID != m.EvolutionMessageID {
		t.Errorf("download id not stamped")
	}
	// A bare image has caption:null → empty body.
	if m.Text != "" {
		t.Errorf("bare image text = %q, want empty", m.Text)
	}
}

func TestNormalizeImageCaption(t *testing.T) {
	env := loadCapture(t, "messages_upsert_image_caption.json")
	m, err := env.Message()
	if err != nil || m == nil {
		t.Fatalf("Message() = %v, %v", m, err)
	}
	// image + text is ONE message; the caption carries the text.
	if m.Media == nil {
		t.Fatalf("expected media")
	}
	if m.Text == "" {
		t.Errorf("caption text should be carried on body, got empty")
	}
}

func TestNormalizeSendMessageEcho(t *testing.T) {
	env := loadCapture(t, "send_message.json")
	m, err := env.Message()
	if err != nil || m == nil {
		t.Fatalf("Message() = %v, %v", m, err)
	}
	if m.Direction != "out" {
		t.Errorf("send.message direction = %q, want out", m.Direction)
	}
	if m.EvolutionMessageID != "3EB063FA228F78C7FAE3CE" {
		t.Errorf("echo id = %q", m.EvolutionMessageID)
	}
}

func TestNormalizeStatusUpdate(t *testing.T) {
	env := loadCapture(t, "messages_update.json")
	su, err := env.StatusUpdate()
	if err != nil || su == nil {
		t.Fatalf("StatusUpdate() = %v, %v", su, err)
	}
	// Status correlates on keyId == evolution_message_id.
	if su.EvolutionMessageID != "3EB06664456CEB540ACCEB31333296464363194A" {
		t.Errorf("keyId = %q", su.EvolutionMessageID)
	}
	if su.DeliveryState != "delivered" {
		t.Errorf("DELIVERY_ACK mapped to %q, want delivered", su.DeliveryState)
	}
}

func TestDeliveryMonotonic(t *testing.T) {
	if !(DeliveryRank("queued") < DeliveryRank("sent") &&
		DeliveryRank("sent") < DeliveryRank("delivered") &&
		DeliveryRank("delivered") < DeliveryRank("read")) {
		t.Fatalf("delivery ranks are not monotonic")
	}
}

func TestParseLong(t *testing.T) {
	if got := parseLong([]byte(`1781459144`)); got != 1781459144 {
		t.Errorf("plain int Long = %d", got)
	}
	if got := parseLong([]byte(`{"low":87154,"high":0,"unsigned":true}`)); got != 87154 {
		t.Errorf("struct Long = %d", got)
	}
	if got := parseLong([]byte(`{"low":5,"high":1,"unsigned":false}`)); got != 5+(1<<32) {
		t.Errorf("high-word Long = %d", got)
	}
}
