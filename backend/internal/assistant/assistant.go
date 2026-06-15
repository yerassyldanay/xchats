// Package assistant is the AI brain seam. Build 0 ships a hardcoded stub Drafter
// (B8): it ignores the vendored brain and returns 1–3 constant options, each with
// a sample media file drawn from a small committed catalog. The real ported brain
// is a later phase; everything downstream rides this same Drafter interface.
package assistant

import (
	"context"
	"embed"
	"path"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
)

//go:embed sample-media/*
var sampleFS embed.FS

// Media is a suggested attachment on a draft option.
type Media struct {
	Ref  string // blob/media id (also used as media_id when approving)
	Kind string // image|document|audio
	URL  string // /xchats/api/v1/media/{ref}
}

// Option is one suggested reply.
type Option struct {
	Ordinal    int
	Text       string
	Confidence *float64
	Escalate   bool
	Reason     string
	Media      []Media
}

// Input is the (minimal) context the stub is handed.
type Input struct {
	ChatID          string
	ContactName     string
	LastInboundText string
}

// Drafter produces reply options for a chat.
type Drafter interface {
	Draft(ctx context.Context, in Input) ([]Option, error)
}

// Stub is the Build 0 hardcoded Drafter.
type Stub struct {
	apiBase string
	catalog []Media
}

const stubText = "Это тестовый вариант ответа"

type sampleSpec struct {
	id, file, kind, mime string
}

var samples = []sampleSpec{
	{"sample-image", "sample-image.png", "image", "image/png"},
	{"sample-doc", "sample-doc.pdf", "document", "application/pdf"},
	{"sample-audio", "sample-audio.wav", "audio", "audio/wav"},
}

// NewStub loads the committed sample media into the blob store (idempotent,
// deterministic ids) so approving a draft can send them, and returns the stub.
func NewStub(store blob.Store, apiBase string) (*Stub, error) {
	s := &Stub{apiBase: apiBase}
	for _, sp := range samples {
		data, err := sampleFS.ReadFile(path.Join("sample-media", sp.file))
		if err != nil {
			return nil, err
		}
		if _, err := store.Put(sp.id, data, blob.Meta{
			MediaType: sp.kind, Mimetype: sp.mime, FileName: sp.file, FileSize: int64(len(data)),
		}); err != nil {
			return nil, err
		}
		s.catalog = append(s.catalog, Media{
			Ref: sp.id, Kind: sp.kind, URL: apiBase + "/xchats/api/v1/media/" + sp.id,
		})
	}
	return s, nil
}

func f64(v float64) *float64 { return &v }

// Draft returns three constant options, each with one rotating sample attachment.
func (s *Stub) Draft(ctx context.Context, in Input) ([]Option, error) {
	opts := make([]Option, 0, 3)
	for i := 0; i < 3; i++ {
		opts = append(opts, Option{
			Ordinal:    i + 1,
			Text:       stubText,
			Confidence: f64(0.8 - float64(i)*0.1),
			Media:      []Media{s.catalog[i%len(s.catalog)]},
		})
	}
	return opts, nil
}
