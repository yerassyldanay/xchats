// Package sttmock is an in-memory stt.Transcriber for cmd/xchats'
// mock-externals mode.
package sttmock

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/internal/stt"
)

// Transcriber answers every call with a fixed, plausible transcript
// instantly — never a placeholder error, per the hermetic profiling
// harness's own contract for every fake.
type Transcriber struct{}

// New returns a ready-to-use mock Transcriber.
func New() *Transcriber { return &Transcriber{} }

var _ stt.Transcriber = (*Transcriber)(nil)

func (t *Transcriber) Transcribe(ctx context.Context, audio []byte, filename, mime string, opts stt.TranscribeOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "Это тестовая расшифровка голосового сообщения (режим mock-externals).", nil
}
