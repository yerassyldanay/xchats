package assistant

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// windowReader is the slice of the store the brain needs: the recent message
// window for a chat. *store.Store satisfies it; tests use a fake.
type windowReader interface {
	MessagesForChat(ctx context.Context, chatID uuid.UUID, before time.Time, limit int) ([]store.Message, *time.Time, error)
}

// brainDrafter calls the LLM with an assembled prompt and returns the parsed
// emit_draft contract. *brain/llm.Drafter satisfies it; tests use a fake.
type brainDrafter interface {
	Draft(ctx context.Context, p brain.Prompt) (domain.RawDraft, error)
}

// windowSize is how many recent messages we feed the model (no profile in v1).
const windowSize = 15

// RealDrafter is the KB-grounded Drafter: it builds the prompt from the embedded
// snapshot + the chat's recent window, calls the LLM, post-processes the result,
// and returns ONE reply option (text + resolved catalog media). It satisfies
// assistant.Drafter, so it drops in wherever the Stub was used.
type RealDrafter struct {
	store   windowReader
	llm     brainDrafter
	content *domain.Content // hot-swappable published snapshot (reloaded on publish)
	log     *slog.Logger
}

// NewReal builds a RealDrafter. snap is the initial published KB (the DB snapshot,
// or brain.SeedSnapshot() as fallback). Call SetSnapshot to hot-swap it on publish.
func NewReal(s windowReader, llm brainDrafter, snap *domain.Snapshot, log *slog.Logger) *RealDrafter {
	if log == nil {
		log = slog.Default()
	}
	c := &domain.Content{}
	c.Set(snap)
	return &RealDrafter{store: s, llm: llm, content: c, log: log}
}

// SetSnapshot atomically swaps the published KB the brain drafts from. The
// playground calls this after a successful Publish so new replies use the new KB.
func (r *RealDrafter) SetSnapshot(snap *domain.Snapshot) { r.content.Set(snap) }

// Draft implements Drafter: window → prompt → LLM → post-process → one Option.
func (r *RealDrafter) Draft(ctx context.Context, in Input) ([]Option, error) {
	chatID, err := uuid.Parse(in.ChatID)
	if err != nil {
		return nil, fmt.Errorf("real drafter: bad chat id %q: %w", in.ChatID, err)
	}
	msgs, _, err := r.store.MessagesForChat(ctx, chatID, time.Time{}, windowSize)
	if err != nil {
		return nil, err
	}
	window, current := splitWindow(msgs, in.LastInboundText)

	snap := r.content.Get()
	prompt := brain.Prompt{
		System: brain.BuildSystem(snap),
		User:   brain.BuildUser(nil, window, current), // no profile in v1
	}
	raw, err := r.llm.Draft(ctx, prompt)
	if err != nil {
		// A failed/malformed LLM response is a low-confidence escalation, not a
		// crash (the human just replies manually) — same stance as the brain.
		r.log.WarnContext(ctx, "llm draft failed; escalating", "chat", in.ChatID, "err", err)
		return []Option{{Ordinal: 1, Text: brain.HoldingReply, Escalate: true, Reason: "llm_error: " + err.Error()}}, nil
	}

	d := brain.PostProcess(raw, snap, r.log)
	return []Option{optionFromDraft(d)}, nil
}

// splitWindow maps the store window to domain messages and picks the current
// message (the latest inbound — what "Suggest" answers). The current message is
// excluded from the window so it isn't duplicated in the transcript. If no inbound
// is present, fall back to Input.LastInboundText.
func splitWindow(msgs []store.Message, fallback string) (window []domain.Message, current domain.Message) {
	lastIn := -1
	for i, m := range msgs {
		if m.Direction == "in" {
			lastIn = i
		}
	}
	for i, m := range msgs {
		if i == lastIn {
			continue
		}
		window = append(window, toDomainMsg(m))
	}
	if lastIn >= 0 {
		current = toDomainMsg(msgs[lastIn])
		current.Role = domain.RoleCustomer
	} else {
		current = domain.Message{Role: domain.RoleCustomer, Content: fallback}
	}
	return window, current
}

func toDomainMsg(m store.Message) domain.Message {
	role := domain.RoleCustomer
	if m.Direction == "out" {
		role = domain.RoleAgent
	}
	var ts time.Time
	if m.MessageTS != nil {
		ts = *m.MessageTS
	}
	return domain.Message{ID: m.ID.String(), Role: role, Content: m.Body, CreatedAt: ts}
}

func optionFromDraft(d domain.Draft) Option {
	conf := d.Confidence
	opt := Option{
		Ordinal:    1,
		Text:       d.ReplyText,
		Confidence: &conf,
		Escalate:   d.Escalate,
		Reason:     d.EscalationReason,
	}
	for _, m := range d.Media {
		opt.Media = append(opt.Media, Media{Ref: m.Ref, Kind: m.Kind, URL: m.URL})
	}
	return opt
}
