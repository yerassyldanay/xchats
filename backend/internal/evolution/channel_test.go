package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

var errBoom = errors.New("boom")

const textWebhookBody = `{
	"event": "messages.upsert",
	"instance": "test-instance",
	"sender": "77011111111@s.whatsapp.net",
	"date_time": "2026-01-01T00:00:00.000Z",
	"data": {
		"key": {
			"remoteJid": "77000000000@s.whatsapp.net",
			"remoteJidAlt": "77000000000@s.whatsapp.net",
			"fromMe": false,
			"id": "TESTID123"
		},
		"pushName": "Test Contact",
		"messageType": "conversation",
		"message": {"conversation": "Hello"},
		"messageTimestamp": "1700000000"
	}
}`

func TestChannelDecoder_DecodesTextMessage(t *testing.T) {
	d := NewChannelDecoder()
	msg, ok, err := d.Decode(context.Background(), []byte(textWebhookBody))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !ok {
		t.Fatal("want ok=true for a message-bearing event")
	}
	if msg.Channel != messaging.ChannelWhatsApp {
		t.Errorf("channel = %q, want %q", msg.Channel, messaging.ChannelWhatsApp)
	}
	if msg.Direction != "in" {
		t.Errorf("direction = %q, want in", msg.Direction)
	}
	if msg.Text != "Hello" {
		t.Errorf("text = %q, want Hello", msg.Text)
	}
	if msg.ExternalID != "TESTID123" {
		t.Errorf("external id = %q, want TESTID123", msg.ExternalID)
	}
}

func TestChannelDecoder_NonMessageEventIsNotOK(t *testing.T) {
	d := NewChannelDecoder()
	raw := []byte(`{"event":"connection.update","instance":"x","sender":"77011111111@s.whatsapp.net","data":{"state":"open"}}`)
	msg, ok, err := d.Decode(context.Background(), raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if ok {
		t.Fatalf("want ok=false for a non-message event, got message %+v", msg)
	}
}

func TestChannelDecoder_RejectsWrongInputType(t *testing.T) {
	d := NewChannelDecoder()
	if _, _, err := d.Decode(context.Background(), "not bytes"); err == nil {
		t.Fatal("want an error for non-[]byte input")
	}
}

func TestChannelSender_SendsTextThroughClient(t *testing.T) {
	fake := NewFake("test-instance", "77011111111@s.whatsapp.net")
	sender := NewChannelSender(fake)

	res, err := sender.Send(context.Background(), messaging.OutboundMessage{
		MessageID: "m1", Channel: messaging.ChannelWhatsApp,
		Text: "hi there", To: "77000000000", Route: "test-instance",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res.ExternalID == "" || !res.Delivered {
		t.Fatalf("Send() result = %+v, want a non-empty external id and Delivered=true", res)
	}
	calls := fake.CallsFor("sendText")
	if len(calls) != 1 {
		t.Fatalf("want exactly one sendText call, got %d", len(calls))
	}
	if calls[0].Instance != "test-instance" || calls[0].Number != "77000000000" || calls[0].Text != "hi there" {
		t.Fatalf("unexpected call: %+v", calls[0])
	}
}

func TestChannelSender_PropagatesClientError(t *testing.T) {
	fake := NewFake("test-instance", "77011111111@s.whatsapp.net")
	fake.FailNext = errBoom
	sender := NewChannelSender(fake)

	if _, err := sender.Send(context.Background(), messaging.OutboundMessage{Text: "x"}); err == nil {
		t.Fatal("want the client's error to propagate")
	}
}
