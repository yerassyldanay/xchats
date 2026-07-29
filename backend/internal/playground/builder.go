package playground

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
)

// Synthesizer is Stage 2: it reasons over the uniform materials + a draft summary
// and returns a Plan of draft mutations. It never sees input types — only
// extracted_text + provenance. RuleSynthesizer is the deterministic default; an
// LLM-backed implementation drops in behind this same interface.
type Synthesizer interface {
	Plan(ctx context.Context, in SynthInput) (Plan, error)
}

// SynthInput is everything the synthesizer reasons over for one chat turn.
type SynthInput struct {
	Instruction string
	Materials   []kbstore.Material // extraction-ready materials for this build
	Draft       *kbstore.DraftView // current draft (for diff: update vs create)
}

// Plan is the synthesizer's output: KB mutations + human-in-the-loop popups.
// Numbers are never baked into a body — they leave as FactConfirm popups and the
// body prose has the amount excised. There is no generic media/asset section:
// a file stays a kbd_materials row and its extracted/confirmed description may
// contribute source prose, but the builder never turns it into sendable media
// (see plan/database-schema.md — the canonical semantic-media assignment flow
// lands once the typed media columns exist).
type Plan struct {
	Topics   []TopicProposal
	Confirm  []FactConfirm // → confirm_fact popups
	Describe []DescribeFile // → describe_file popups
	Comment  string
}

// TopicProposal upserts a draft topic (proposed for review).
type TopicProposal struct {
	Slug, Lang, Title, BodyMD string
	MaterialID                string
}

// FactConfirm raises a confirm_fact popup (a price/amount detected in material)
// targeting a concrete typed-fact column (e.g. tariff <slug>.price).
type FactConfirm struct {
	Table, Slug, Field, Lang, Suggested, Description string
	MaterialID                                       string
}

// DescribeFile raises a describe_file popup for a material with no description.
type DescribeFile struct {
	MaterialID string
	Prompt     string
}

// Builder runs one synthesis turn: gather ready materials → plan → apply to the
// draft (proposed rows) + raise popups, all broadcast so the editor updates live.
type Builder struct {
	synth Synthesizer
	hub   *realtime.Hub
}

// NewBuilder wires a Builder to a synthesizer (RuleSynthesizer if synth is nil).
func NewBuilder(synth Synthesizer, hub *realtime.Hub) *Builder {
	if synth == nil {
		synth = RuleSynthesizer{}
	}
	return &Builder{synth: synth, hub: hub}
}

// TurnResult summarizes what a build turn produced (for the chat reply).
type TurnResult struct {
	TopicsUpserted int    `json:"topics_upserted"`
	Confirms       int    `json:"confirms"`
	Describes      int    `json:"describes"`
	Comment        string `json:"comment"`
}

// RunTurn executes a builder-chat turn against the org's open draft.
func (b *Builder) RunTurn(ctx context.Context, kb *kbstore.Store, orgID uuid.UUID, instruction string) (TurnResult, error) {
	ready, err := kb.ReadyMaterials(ctx, orgID)
	if err != nil {
		return TurnResult{}, err
	}
	draft, err := kb.Draft(ctx, orgID)
	if err != nil {
		return TurnResult{}, err
	}
	plan, err := b.synth.Plan(ctx, SynthInput{Instruction: instruction, Materials: ready, Draft: draft})
	if err != nil {
		return TurnResult{}, err
	}

	var res TurnResult
	res.Comment = plan.Comment
	prov := jsonObj(map[string]any{"source": "builder"})

	for _, t := range plan.Topics {
		body := t.BodyMD
		// Diff vs the draft: an existing slug is an UPDATE (append), not a twin.
		// Guard the append so a re-run with the same body is a no-op, not a duplicate.
		if existing := findTopic(draft, t.Slug); existing != nil && existing.BodyMD != "" {
			if strings.Contains(existing.BodyMD, strings.TrimSpace(t.BodyMD)) {
				body = existing.BodyMD
			} else {
				body = existing.BodyMD + "\n\n" + t.BodyMD
			}
		}
		if err := kb.UpsertTopic(ctx, orgID, kbstore.TopicInput{
			Slug: t.Slug, Lang: t.Lang, Title: t.Title, BodyMD: body,
			Provenance: prov,
		}); err != nil {
			return res, err
		}
		res.TopicsUpserted++
	}
	for _, cv := range plan.Confirm {
		mid := materialPtr(cv.MaterialID)
		req, err := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{
			MaterialID: mid, ReqType: "confirm_fact",
			Prompt:  fmt.Sprintf("Подтвердите цену: «%s»?", cv.Suggested),
			Context: jsonObj(map[string]any{"suggested": cv.Suggested, "description": cv.Description}),
			Target: jsonObj(map[string]any{
				"table": cv.Table, "slug": cv.Slug, "field": cv.Field, "lang": orElse(cv.Lang, "ru"),
			}),
		})
		if err != nil {
			return res, err
		}
		b.broadcastRequest(req)
		res.Confirms++
	}
	for _, d := range plan.Describe {
		mid := materialPtr(d.MaterialID)
		req, err := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{
			MaterialID: mid, ReqType: "describe_file",
			Prompt: orElse(d.Prompt, "Опишите этот материал."),
			Target: jsonObj(map[string]any{"material_id": d.MaterialID}),
		})
		if err != nil {
			return res, err
		}
		b.broadcastRequest(req)
		res.Describes++
	}

	// Consume the synthesized materials so a repeat turn does not re-process them.
	builtIDs := make([]uuid.UUID, 0, len(ready))
	for _, m := range ready {
		builtIDs = append(builtIDs, m.ID)
	}
	if err := kb.MarkMaterialsBuilt(ctx, builtIDs); err != nil {
		return res, err
	}

	if b.hub != nil {
		b.hub.Broadcast("kb.row.changed", map[string]any{"turn": res})
	}
	return res, nil
}

// broadcastRequest emits kb.request.created for a freshly-raised popup so the
// Конструктор chat and the Редактор editor both update live. A zero-value
// Request (CreateRequest failed) is silently skipped.
func (b *Builder) broadcastRequest(req kbstore.Request) {
	if b.hub == nil || req.ID == uuid.Nil {
		return
	}
	b.hub.Broadcast("kb.request.created", map[string]any{"id": req.ID, "req_type": req.ReqType})
}

// jsonObj marshals a small map to a compact JSON string for a jsonb column. Using
// the encoder (not fmt %q) keeps the output valid for any value — quotes, newlines,
// non-ASCII — instead of only the safe-charset inputs string formatting handles.
func jsonObj(m map[string]any) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func findTopic(d *kbstore.DraftView, slug string) *kbstore.TopicRow {
	if d == nil {
		return nil
	}
	for i := range d.Topics {
		if d.Topics[i].Slug == slug {
			return &d.Topics[i]
		}
	}
	return nil
}

func materialPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

func orElse(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// ---------------------------------------------------------------------------
// RuleSynthesizer — the deterministic default
// ---------------------------------------------------------------------------

// RuleSynthesizer turns a batch of materials into one coherent topic without an
// LLM: it cross-references the ready text into a single topic body (PURE PROSE)
// and EXCISES detected prices (raising confirm_fact popups so a digit never
// enters a body — the confirmed amount lands in a typed tariff column). A file
// material's extracted/confirmed description folds into the same prose like
// any other material; there is no separate sendable-media output (see the Plan
// doc comment). It is the deterministic proof of the build contract and the
// test baseline; an LLM synthesizer implements the same Synthesizer interface
// for richer judgment.
type RuleSynthesizer struct{}

// priceRE matches "25 000 ₸" / "9 900 ₸/мес" — a number (with space/nbsp groups)
// suffixed by ₸ and an optional unit. These are excised from bodies and confirmed
// into typed tariff price columns, never left as digits in prose.
var priceRE = regexp.MustCompile(`[0-9][0-9 \x{00a0}]*₸(?:/[A-Za-zА-Яа-я]+)?`)

// Plan implements Synthesizer.
func (RuleSynthesizer) Plan(_ context.Context, in SynthInput) (Plan, error) {
	var plan Plan
	var textParts []string
	for _, m := range in.Materials {
		if strings.TrimSpace(m.ExtractedText) != "" {
			textParts = append(textParts, m.ExtractedText)
		}
	}

	// Excise prices from the prose body (bodies are pure prose — 14 D3) and raise a
	// confirm_fact per unique amount, targeting a new tariff's price column. The
	// operator confirms + names the tariff in the editor; the body keeps only prose.
	combined := strings.Join(textParts, "\n\n")
	seen := map[string]string{} // price text → tariff slug
	n := 0
	body := priceRE.ReplaceAllStringFunc(combined, func(match string) string {
		match = strings.TrimSpace(match)
		slug, ok := seen[match]
		if !ok {
			n++
			slug = fmt.Sprintf("item_%d", n)
			seen[match] = slug
			plan.Confirm = append(plan.Confirm, FactConfirm{
				Table: "tariff", Slug: slug, Field: "price", Lang: "ru",
				Suggested: match, Description: "Цена, обнаруженная в материалах",
			})
		}
		return "—"
	})

	slug := slugify(in.Instruction)
	if slug == "" {
		slug = "imported"
	}
	if strings.TrimSpace(body) != "" {
		plan.Topics = append(plan.Topics, TopicProposal{
			Slug: slug, Lang: "ru", Title: firstLine(in.Instruction), BodyMD: body,
		})
	}

	if len(plan.Topics) == 0 {
		plan.Comment = "Нет готовых материалов для сборки — добавьте текст, ссылку или файлы."
	} else {
		plan.Comment = fmt.Sprintf("Собрал черновик: тем — %d, значений на подтверждение — %d.",
			len(plan.Topics), len(plan.Confirm))
	}
	return plan, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlug.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

