package telegram

import (
	"context"
	"testing"
)

func TestFakeMessageIDsStartAtNineThousandAndOneByDefault(t *testing.T) {
	f := NewFake(1, "bot")
	for _, want := range []int64{9001, 9002} {
		sent, err := f.SendMessage(context.Background(), "1:token", 42, "hi")
		if err != nil {
			t.Fatal(err)
		}
		if sent.MessageID != want {
			t.Fatalf("message id = %d, want %d", sent.MessageID, want)
		}
	}
}

func TestFakeMessageIDBaseOffsetsEverySend(t *testing.T) {
	f := NewFake(1, "bot")
	f.MessageIDBase = 5000000
	text, err := f.SendMessage(context.Background(), "1:token", 42, "hi")
	if err != nil {
		t.Fatal(err)
	}
	media, err := f.SendMedia(context.Background(), "1:token", 42, Upload{Kind: "photo"})
	if err != nil {
		t.Fatal(err)
	}
	if text.MessageID != 5000001 || media.MessageID != 5000002 {
		t.Fatalf("ids = %d, %d; want 5000001, 5000002", text.MessageID, media.MessageID)
	}
}
