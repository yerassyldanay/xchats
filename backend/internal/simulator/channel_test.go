package simulator

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

func TestChannelDecoder_DecodesInboundRequest(t *testing.T) {
	d := NewChannelDecoder()
	msg, ok, err := d.Decode(context.Background(), InboundRequest{
		ContactRef: "demo-contact", ConversationRef: "demo-conversation", Text: "Привет",
	})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !ok {
		t.Fatal("want ok=true for a non-empty message")
	}
	if msg.Channel != messaging.ChannelSimulator {
		t.Errorf("channel = %q, want %q", msg.Channel, messaging.ChannelSimulator)
	}
	if msg.Direction != "in" {
		t.Errorf("direction = %q, want in", msg.Direction)
	}
	if msg.Text != "Привет" {
		t.Errorf("text = %q, want Привет", msg.Text)
	}
}

func TestChannelDecoder_EmptyTextIsNotOK(t *testing.T) {
	d := NewChannelDecoder()
	_, ok, err := d.Decode(context.Background(), InboundRequest{ContactRef: "x", ConversationRef: "y"})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if ok {
		t.Fatal("want ok=false for empty text")
	}
}

func TestChannelDecoder_RejectsWrongInputType(t *testing.T) {
	d := NewChannelDecoder()
	if _, _, err := d.Decode(context.Background(), "not an InboundRequest"); err == nil {
		t.Fatal("want an error for the wrong input type")
	}
}

func TestChannelSender_RecordsNoNetworkCallAndReportsDelivered(t *testing.T) {
	s := NewChannelSender()
	res, err := s.Send(context.Background(), messaging.OutboundMessage{
		MessageID: "11111111-1111-1111-1111-111111111111", Channel: messaging.ChannelSimulator, Text: "ok",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !res.Delivered {
		t.Fatal("want Delivered=true")
	}
	if res.ExternalID != "sim-11111111-1111-1111-1111-111111111111" {
		t.Errorf("external id = %q, want a deterministic sim-<message id>", res.ExternalID)
	}
}

func TestChannelSender_DeterministicAcrossCalls(t *testing.T) {
	s := NewChannelSender()
	out := messaging.OutboundMessage{MessageID: "same-id", Text: "x"}
	r1, _ := s.Send(context.Background(), out)
	r2, _ := s.Send(context.Background(), out)
	if r1.ExternalID != r2.ExternalID {
		t.Fatalf("external id not deterministic: %q vs %q", r1.ExternalID, r2.ExternalID)
	}
}
