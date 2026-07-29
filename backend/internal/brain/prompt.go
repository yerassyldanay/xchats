// Package brain is the ported assistant core: it builds the cache-stable system
// prompt + the dynamic user block from a published Snapshot, and post-processes
// the model's emit_draft output into a final Draft (escalate → inject prices →
// clean profile → flatten status). It performs no writes and has no transport
// dependency — the caller (internal/assistant.RealDrafter) wires it to the LLM
// and the store. Ported from xpayment-crm (module path rewritten).
package brain

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// Prompt is the assembled LLM input: a cache-stable System prefix + a dynamic User block.
type Prompt struct {
	System string
	User   string
}

// ErrNoPublishedConfig is returned when drafting runs before any snapshot is loaded.
var ErrNoPublishedConfig = errors.New("brain: no published config snapshot loaded")

// frame is the code-owned [A] block — the hard rules, never editable (docs/10 · [A]).
const frame = `You are the drafting engine for an online shop's WhatsApp sales assistant. You write ONE reply
draft that a human will review and send. You never send messages yourself.

Rules (hard, non-negotiable):
1. Answer ONLY from the KNOWLEDGE BASE below. If the answer is not there, do not guess —
   set "escalate": true with a short escalation_reason and a brief holding reply.
2. When the customer asks for an exact fact (price, limit, fee, phone, e-mail, address), ANSWER IT
   DIRECTLY — do not deflect to a media card, do not say "уточните" or "посмотрите карточку", do not
   ask them to look it up. State the fact by emitting its token from the FACTS list, exactly as written,
   e.g. "Тариф Стандарт стоит {{tariff.standard.price}}." NEVER write the number/contact as a digit or
   literal — always the token; code fills the real value in after you. Only if a fact is truly absent
   from the FACTS list may you escalate.
3. Reply in the customer's language. If the latest message mixes Kazakh and Russian, reply in Russian.
4. Keep the reply under ~120 words, warm and concrete. One clear next step or question.
5. Never ask for or repeat passwords or other secrets.
6. Extract into profile_patch ONLY facts you are newly confident about. Do not invent fields.

You MUST respond by calling the emit_draft tool with the required JSON. No prose outside the tool call.`

// BuildSystem renders the cache-stable system prefix [A]–[E] from the snapshot
// (docs/10 · implementation checklist step 1). Rebuilt only on publish.
func BuildSystem(s *domain.Snapshot) string {
	var b strings.Builder
	b.WriteString(frame)
	b.WriteString("\n\n# IDENTITY\n")
	b.WriteString(s.Config.Persona)
	if s.Config.Mission != "" {
		b.WriteString("\nMission: ")
		b.WriteString(s.Config.Mission)
	}
	b.WriteString("\n\n# GUARDRAILS\n")
	b.WriteString(s.Config.Guardrails)
	if s.Config.LanguagePolicy != "" {
		b.WriteString("\n")
		b.WriteString(s.Config.LanguagePolicy)
	}

	b.WriteString("\n\nKNOWLEDGE BASE:\n")
	for _, t := range s.Topics {
		fmt.Fprintf(&b, "\n# topic: %s (%s)\n", t.Slug, t.Language)
		fmt.Fprintf(&b, "%s\n", t.BodyMD)
	}

	// [F] FACTS — the Facts lane. Per fact: the token the model must emit, its
	// meaning, and the current value (shown so the model picks the right one; it
	// still outputs the token, never the number). v1 is ru-only, so this is
	// single-valued and cache-stable.
	facts := s.Facts.List()
	if len(facts) > 0 {
		b.WriteString("\nFACTS — when the customer asks about one of these, quote it by emitting its token " +
			"exactly as written (never the value; code substitutes it). The current value is shown only so " +
			"you pick the right fact:\n")
		b.WriteString("token | meaning | current value\n")
		for _, f := range facts {
			fmt.Fprintf(&b, "%s | %s | %s\n", f.Token, f.Label, f.Value)
		}
	}
	return b.String()
}

// BuildUser builds the dynamic per-message block: an optional PROFILE block + the
// window transcript (oldest first) + the current message. When profile is empty
// (xchats passes none) the PROFILE block is omitted entirely.
func BuildUser(profile map[string]any, window []domain.Message, current domain.Message) string {
	var b strings.Builder
	if len(profile) > 0 {
		b.WriteString("PROFILE (what we already know about this contact):\n")
		b.WriteString(marshalProfile(profile))
		b.WriteString("\n\n")
	}

	b.WriteString("CONVERSATION (most recent messages, oldest first):\n")
	for _, m := range window {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
	}

	b.WriteString("\nCURRENT MESSAGE:\n")
	fmt.Fprintf(&b, "%s: %s\n", domain.RoleCustomer, current.Content)
	return b.String()
}

func marshalProfile(profile map[string]any) string {
	if len(profile) == 0 {
		return "{}"
	}
	// json.Marshal sorts map keys, so the block is deterministic across calls.
	out, err := json.Marshal(profile)
	if err != nil {
		return "{}"
	}
	return string(out)
}

// PostProcess runs the pipeline (docs/02 · post-processing), in order: escalate
// gate → inject prices → clean profile_patch → flatten status. It never returns
// an error: a price-render failure becomes a manual-check note, an escalation
// stops the pipeline.
func PostProcess(raw domain.RawDraft, snap *domain.Snapshot, log *slog.Logger) domain.Draft {
	if log == nil {
		log = slog.Default()
	}

	// 2. Escalate gate — flag for a human and stop (no media, no auto-send).
	//
	// The customer-facing text on this path is a TRUSTED holding reply, never
	// raw.ReplyText: a model that correctly sets escalate=true can still write an
	// invented claim in the same breath (observed in eval canaries — e.g. asserting
	// "we don't deliver to Astana", a claim the knowledge base never makes). The
	// prompt asking the model not to do that is a quality measure, not a safety
	// guarantee; this substitution is the guarantee. EscalationReason (internal,
	// never shown to the customer) and Confidence still come from the model — only
	// the customer-facing ReplyText is replaced.
	if raw.Escalate {
		d := escalationDraft(holdingReplyFor(raw.ReplyLanguage), raw.EscalationReason)
		d.Confidence = raw.Confidence
		return d
	}

	d := domain.Draft{
		Confidence:        raw.Confidence,
		SuggestedCallback: raw.SuggestedCallback,
	}

	// 3. Inject facts — never ship a half-rendered fact token (Decision 8).
	lang := raw.ReplyLanguage
	if lang == "" {
		lang = "ru"
	}
	rendered, err := snap.Facts.Render(raw.ReplyText, lang)
	if err != nil {
		log.Warn("price render failed; posting check-pricing note", "err", err)
		d.PricingError = true
		d.ReplyText = pricingManualNote
		return d
	}
	d.ReplyText = rendered

	// 4. profile_patch — drop the stage key (that is status, handled next).
	d.ProfilePatch = cleanProfilePatch(raw.ProfilePatch)

	// 5. status — flatten suggested_status.stage to a label.
	if raw.SuggestedStatus != nil {
		d.SuggestedStatus = raw.SuggestedStatus.Stage
	}

	return d
}

// cleanProfilePatch copies the patch minus the reserved "stage" key (Decision 9).
func cleanProfilePatch(patch map[string]any) map[string]any {
	if len(patch) == 0 {
		return nil
	}
	out := make(map[string]any, len(patch))
	for k, v := range patch {
		if k == "stage" {
			continue
		}
		out[k] = v
	}
	return out
}

func escalationDraft(reply, reason string) domain.Draft {
	return domain.Draft{ReplyText: reply, Escalate: true, EscalationReason: reason}
}

const (
	// HoldingReply is the brief reply shipped when drafting fails or the model
	// escalates without its own text (the LLM-error path in RealDrafter uses it).
	// Also holdingReplyByLanguage["ru"] below — same wording, one source of truth
	// for what "ru" means, since RealDrafter's error path has no reply_language to
	// consult and must pick something.
	HoldingReply      = "Уточню это у коллеги и вернусь с точным ответом — буквально пару минут."
	pricingManualNote = "⚠️ Не удалось подставить цену автоматически — проверьте перед отправкой."
)

// holdingReplyByLanguage is the trusted, deterministic copy shown to a customer
// whenever the model escalates (see PostProcess's escalate gate) — the only two
// reply_language values the emit_draft tool schema allows (openrouter.go's tool
// schema enum: "ru", "kk"). Never add a model-supplied string to this map at
// runtime; it must stay a fixed, reviewed set.
var holdingReplyByLanguage = map[string]string{
	"ru": HoldingReply,
	"kk": "Мұны әріптесіммен нақтылап, бірнеше минут ішінде нақты жауап беремін.",
}

// holdingReplyFor returns the trusted holding reply for lang, defaulting to "ru"
// for an empty or unrecognized value — the same default PostProcess's fact-
// injection path already uses for raw.ReplyLanguage.
func holdingReplyFor(lang string) string {
	if reply, ok := holdingReplyByLanguage[lang]; ok {
		return reply
	}
	return holdingReplyByLanguage["ru"]
}
