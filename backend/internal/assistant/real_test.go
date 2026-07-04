package assistant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

type fakeWindow struct{ msgs []store.Message }

func (f fakeWindow) MessagesForChat(context.Context, uuid.UUID, time.Time, int) ([]store.Message, *time.Time, error) {
	return f.msgs, nil, nil
}

type fakeLLM struct {
	raw domain.RawDraft
	err error
}

func (f fakeLLM) Draft(context.Context, brain.Prompt) (domain.RawDraft, error) {
	return f.raw, f.err
}

func inboundWindow(text string) fakeWindow {
	ts := time.Now()
	return fakeWindow{msgs: []store.Message{
		{ID: uuid.New(), Direction: "in", Body: text, MessageTS: &ts},
	}}
}

// A grounded answer: the RealDrafter renders price tokens and resolves catalog
// media against the embedded Demo Shop snapshot, returning one option.
func TestRealDrafter_GroundedOption(t *testing.T) {
	r := NewReal(
		inboundWindow("Сколько стоит?"),
		fakeLLM{raw: domain.RawDraft{
			ReplyText:     "Стандарт — {{tariff.standard.price}}.",
			ReplyLanguage: "ru",
			AssetRefs:     []string{brain.RefPricingCard},
			Confidence:    0.8,
		}},
		brain.SeedSnapshot(), nil,
	)
	opts, err := r.Draft(context.Background(), Input{ChatID: uuid.New().String()})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("want 1 option, got %d", len(opts))
	}
	if opts[0].Text != "Стандарт — 19 900 ₸." {
		t.Fatalf("price not injected: %q", opts[0].Text)
	}
	if len(opts[0].Media) != 1 || opts[0].Media[0].Ref != brain.RefPricingCard {
		t.Fatalf("media not resolved: %+v", opts[0].Media)
	}
	if opts[0].Media[0].URL != "/xchats/api/v1/media/"+brain.RefPricingCard {
		t.Fatalf("unexpected media url: %q", opts[0].Media[0].URL)
	}
	if opts[0].Escalate {
		t.Fatal("should not escalate")
	}
}

// An LLM failure becomes a single escalation option (holding reply), never an error.
func TestRealDrafter_LLMErrorEscalates(t *testing.T) {
	r := NewReal(inboundWindow("Сколько стоит?"), fakeLLM{err: errors.New("boom")}, brain.SeedSnapshot(), nil)
	opts, err := r.Draft(context.Background(), Input{ChatID: uuid.New().String()})
	if err != nil {
		t.Fatalf("LLM error must not bubble up: %v", err)
	}
	if len(opts) != 1 || !opts[0].Escalate {
		t.Fatalf("expected one escalation option, got %+v", opts)
	}
	if opts[0].Text != brain.HoldingReply {
		t.Fatalf("expected holding reply, got %q", opts[0].Text)
	}
}

func TestRealDrafter_BadChatID(t *testing.T) {
	r := NewReal(fakeWindow{}, fakeLLM{}, brain.SeedSnapshot(), nil)
	if _, err := r.Draft(context.Background(), Input{ChatID: "not-a-uuid"}); err == nil {
		t.Fatal("expected error for bad chat id")
	}
}
