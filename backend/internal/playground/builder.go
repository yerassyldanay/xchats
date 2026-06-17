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
// Numbers are never baked into a body — they leave as ConfirmValue popups.
type Plan struct {
	Topics   []TopicProposal
	Assets   []AssetProposal
	Confirm  []ValueConfirm  // → confirm_value popups
	Describe []DescribeMedia // → describe_media popups
	Comment  string
}

// TopicProposal upserts a draft topic (proposed for review).
type TopicProposal struct {
	Slug, Lang, Title, Keywords, BodyMD string
	MaterialID                          string
}

// AssetProposal attaches a material's bytes as a draft asset (proposed).
type AssetProposal struct {
	Ref, Kind, TopicSlug, Title, Description, URL string
	MaterialID                                    string
}

// ValueConfirm raises a confirm_value popup (a price/number detected in material).
type ValueConfirm struct {
	Token, Lang, Suggested, Description string
	MaterialID                          string
}

// DescribeMedia raises a describe_media popup for a material with no description.
type DescribeMedia struct {
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
	AssetsAttached int    `json:"assets_attached"`
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
	draft, err := kb.GetDraft(ctx, orgID)
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
			Slug: t.Slug, Lang: t.Lang, Title: t.Title, Keywords: t.Keywords, BodyMD: body,
			ReviewState: "proposed", Provenance: prov,
		}); err != nil {
			return res, err
		}
		res.TopicsUpserted++
	}
	for _, a := range plan.Assets {
		if err := kb.UpsertAsset(ctx, orgID, kbstore.AssetInput{
			Ref: a.Ref, Kind: a.Kind, TopicSlug: a.TopicSlug, Title: a.Title,
			Description: a.Description, URL: a.URL, ReviewState: "proposed", Provenance: prov,
		}); err != nil {
			return res, err
		}
		res.AssetsAttached++
		if strings.TrimSpace(a.Description) == "" {
			_, _ = kb.CreateRequest(ctx, orgID, kbstore.RequestInput{
				ReqType: "describe_media", Prompt: "Опишите, что показывает этот файл и когда его отправлять.",
				Target: jsonObj(map[string]any{"asset_ref": a.Ref}),
			})
			res.Describes++
		}
	}
	for _, cv := range plan.Confirm {
		mid := materialPtr(cv.MaterialID)
		if _, err := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{
			MaterialID: mid, ReqType: "confirm_value",
			Prompt:  fmt.Sprintf("Подтвердите значение для %s: «%s»?", cv.Token, cv.Suggested),
			Context: jsonObj(map[string]any{"suggested": cv.Suggested, "description": cv.Description}),
			Target:  jsonObj(map[string]any{"token": cv.Token, "lang": orElse(cv.Lang, "ru")}),
		}); err != nil {
			return res, err
		}
		res.Confirms++
	}
	for _, d := range plan.Describe {
		mid := materialPtr(d.MaterialID)
		if _, err := kb.CreateRequest(ctx, orgID, kbstore.RequestInput{
			MaterialID: mid, ReqType: "describe_media",
			Prompt: orElse(d.Prompt, "Опишите этот материал."),
			Target: jsonObj(map[string]any{"material_id": d.MaterialID}),
		}); err != nil {
			return res, err
		}
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
// LLM: it cross-references the ready text into a single topic body, tokenizes
// detected prices (raising confirm_value popups so a digit never enters the body),
// and proposes an asset per media file (raising describe_media when undescribed).
// It is the deterministic proof of the build contract and the test baseline; an
// LLM synthesizer implements the same Synthesizer interface for richer judgment.
type RuleSynthesizer struct{}

// priceRE matches "25 000 ₸" / "9 900 ₸/мес" — a number (with space/nbsp groups)
// suffixed by ₸ and an optional unit. These become tokens, never body digits.
var priceRE = regexp.MustCompile(`[0-9][0-9 \x{00a0}]*₸(?:/[A-Za-zА-Яа-я]+)?`)

// Plan implements Synthesizer.
func (RuleSynthesizer) Plan(_ context.Context, in SynthInput) (Plan, error) {
	var plan Plan
	var textParts []string
	for _, m := range in.Materials {
		if m.BlobID == "" && strings.TrimSpace(m.ExtractedText) != "" {
			textParts = append(textParts, m.ExtractedText)
		}
	}

	// Tokenize prices across the combined text → confirm_value popups.
	combined := strings.Join(textParts, "\n\n")
	seen := map[string]string{} // price text → token
	n := 0
	body := priceRE.ReplaceAllStringFunc(combined, func(match string) string {
		match = strings.TrimSpace(match)
		token, ok := seen[match]
		if !ok {
			n++
			token = fmt.Sprintf("price.item_%d", n)
			seen[match] = token
			plan.Confirm = append(plan.Confirm, ValueConfirm{
				Token: token, Lang: "ru", Suggested: match, Description: "Обнаружено в материалах",
			})
		}
		return "{{" + token + "}}"
	})

	slug := slugify(in.Instruction)
	if slug == "" {
		slug = "imported"
	}
	if strings.TrimSpace(body) != "" {
		plan.Topics = append(plan.Topics, TopicProposal{
			Slug: slug, Lang: "ru", Title: firstLine(in.Instruction),
			Keywords: keywordsOf(in.Instruction), BodyMD: body,
		})
	}

	// One asset per media material; vision-captioned ones carry a description.
	for _, m := range in.Materials {
		if m.BlobID == "" {
			continue
		}
		plan.Assets = append(plan.Assets, AssetProposal{
			Ref: m.BlobID, Kind: orElse(m.MediaKind, "image"), TopicSlug: slug,
			Title: m.SourceRef, Description: m.ExtractedText,
			URL: "/xchats/api/v1/media/" + m.BlobID, MaterialID: m.ID.String(),
		})
	}

	if len(plan.Topics) == 0 && len(plan.Assets) == 0 {
		plan.Comment = "Нет готовых материалов для сборки — добавьте текст, ссылку или файлы."
	} else {
		plan.Comment = fmt.Sprintf("Собрал черновик: тем — %d, файлов — %d, значений на подтверждение — %d.",
			len(plan.Topics), len(plan.Assets), len(plan.Confirm))
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

func keywordsOf(s string) string {
	fields := strings.Fields(strings.ToLower(s))
	var kw []string
	for _, f := range fields {
		if len([]rune(f)) >= 4 {
			kw = append(kw, f)
		}
		if len(kw) >= 8 {
			break
		}
	}
	return strings.Join(kw, ", ")
}
