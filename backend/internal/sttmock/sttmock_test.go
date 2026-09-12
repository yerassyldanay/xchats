package sttmock

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/stt"
)

func TestTranscribeReturnsNonEmptyText(t *testing.T) {
	tr := New()
	text, err := tr.Transcribe(context.Background(), []byte{0x01, 0x02}, "voice.ogg", "audio/ogg", stt.TranscribeOptions{Language: "ru"})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text == "" {
		t.Fatal("expected a non-empty transcript")
	}
}

func TestTranscribeRespectsCanceledContext(t *testing.T) {
	tr := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tr.Transcribe(ctx, nil, "voice.ogg", "audio/ogg", stt.TranscribeOptions{}); err == nil {
		t.Fatal("expected an error for an already-canceled context")
	}
}
