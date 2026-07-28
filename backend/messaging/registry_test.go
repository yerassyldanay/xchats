package messaging

import (
	"context"
	"testing"
)

type fakeDecoder struct{}

func (fakeDecoder) Decode(ctx context.Context, input any) (Message, bool, error) {
	return Message{Channel: ChannelSimulator, Text: "x"}, true, nil
}

type fakeSender struct{ sent []OutboundMessage }

func (f *fakeSender) Send(ctx context.Context, out OutboundMessage) (SendResult, error) {
	f.sent = append(f.sent, out)
	return SendResult{ExternalID: "fake-1", Delivered: true}, nil
}

func TestDecoderRegistry_RegisterAndResolve(t *testing.T) {
	r := NewDecoderRegistry()
	r.Register(ChannelSimulator, fakeDecoder{})

	d, err := r.Decoder(ChannelSimulator)
	if err != nil {
		t.Fatalf("Decoder(%q): unexpected error: %v", ChannelSimulator, err)
	}
	msg, ok, err := d.Decode(context.Background(), nil)
	if err != nil || !ok || msg.Text != "x" {
		t.Fatalf("Decode() = %+v, %v, %v", msg, ok, err)
	}
}

func TestDecoderRegistry_UnknownChannelFailsExplicitly(t *testing.T) {
	r := NewDecoderRegistry()
	if _, err := r.Decoder(ChannelWhatsApp); err == nil {
		t.Fatal("want an error for an unregistered channel, got nil")
	}
}

func TestSenderRegistry_RegisterAndResolve(t *testing.T) {
	r := NewSenderRegistry()
	fs := &fakeSender{}
	r.Register(ChannelWhatsApp, fs)

	s, err := r.Sender(ChannelWhatsApp)
	if err != nil {
		t.Fatalf("Sender(%q): unexpected error: %v", ChannelWhatsApp, err)
	}
	res, err := s.Send(context.Background(), OutboundMessage{Text: "hi"})
	if err != nil || res.ExternalID != "fake-1" || !res.Delivered {
		t.Fatalf("Send() = %+v, %v", res, err)
	}
	if len(fs.sent) != 1 || fs.sent[0].Text != "hi" {
		t.Fatalf("sender did not record the send: %+v", fs.sent)
	}
}

func TestSenderRegistry_UnknownChannelFailsExplicitly(t *testing.T) {
	r := NewSenderRegistry()
	if _, err := r.Sender(ChannelSimulator); err == nil {
		t.Fatal("want an error for an unregistered channel, got nil")
	}
}
