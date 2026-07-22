package aiprompt

import (
	"fmt"
	"regexp"
	"strings"
)

// Prompt slots a frame may carry. The frame authors all human wording
// (headers, rules); the renderers emit content rows only. Slots present in a
// frame are always filled; a leftover %%…%% marker fails validation.
const (
	SlotAssistant      = "%%ASSISTANT%%"
	SlotKnowledgeBase  = "%%KNOWLEDGE_BASE%%"
	SlotDescriptions   = "%%DESCRIPTIONS%%"
	SlotFacts          = "%%FACTS%%"
	SlotMedia          = "%%MEDIA%%"
	SlotMediaAbsent    = "%%MEDIA_ABSENT%%"
	SlotResponseSchema = "%%RESPONSE_SCHEMA%%"
)

// BuildPrompt is the explicit two-step orchestration: BuildCatalog validates
// and resolves kbd_materials (private, KB-only), then RenderPrompt renders
// from the material-free PromptInput plus the resulting public Catalog
// projection, and finally the rendered text is checked against kbd_materials
// once more for defense in depth. It returns the catalog alongside so callers
// validate responses against exactly what the model saw.
//
// Callers that already hold a built Catalog (for example to validate a
// response against exactly what an earlier render produced) call RenderPrompt
// directly; production and the eval harness are expected to call BuildCatalog
// and RenderPrompt as the same explicit sequence this function runs.
func BuildPrompt(frame string, kb *KB) (string, *Catalog, error) {
	cat, err := BuildCatalog(kb)
	if err != nil {
		return "", nil, err
	}
	out, err := RenderPrompt(frame, kb.PromptInput(), cat)
	if err != nil {
		return "", nil, err
	}
	if err := ValidateNoMaterialLeak(out, kb.Materials); err != nil {
		return "", nil, err
	}
	return out, cat, nil
}

// RenderPrompt renders the stable prompt prefix — frame + assistant config +
// approved prose + fact-placeholder catalog + semantic media catalog +
// media-absence list + response schema — from approved ai_* content (input)
// and an already-built public catalog (cat). It deliberately does not accept
// a KB or []Material: kbd_materials must already have been validated and
// reduced to a Catalog by BuildCatalog before this is called, so the renderer
// itself has no route to read a material record or ID.
func RenderPrompt(frame string, input *PromptInput, cat *Catalog) (string, error) {
	out := frame
	out = strings.ReplaceAll(out, SlotAssistant, renderAssistant(input.Assistant))
	out = strings.ReplaceAll(out, SlotKnowledgeBase, renderTopics(input.Topics))
	out = strings.ReplaceAll(out, SlotDescriptions, renderDescriptions(input))
	out = strings.ReplaceAll(out, SlotFacts, renderFacts(cat.Facts))
	out = strings.ReplaceAll(out, SlotMedia, renderMediaCatalog(cat.Media))
	out = strings.ReplaceAll(out, SlotMediaAbsent, renderMediaAbsent(cat.Absent))
	out = strings.ReplaceAll(out, SlotResponseSchema, RenderResponseSchema())
	if err := ValidatePrompt(out, cat); err != nil {
		return "", err
	}
	return out, nil
}

// renderAssistant renders ai_assistants config; missing prose is omitted —
// never a blank line.
func renderAssistant(a *Assistant) string {
	if a == nil {
		return ""
	}
	var b []string
	if s := strings.TrimSpace(a.Persona); s != "" {
		b = append(b, "# РОЛЬ\n"+s)
	}
	if s := strings.TrimSpace(a.Mission); s != "" {
		b = append(b, "# ЗАДАЧА\n"+s)
	}
	if s := strings.TrimSpace(a.Guardrails); s != "" {
		b = append(b, "# ОГРАНИЧЕНИЯ\n"+s)
	}
	if s := strings.TrimSpace(a.LanguagePolicy); s != "" {
		b = append(b, "# ЯЗЫК\n"+s)
	}
	return strings.Join(b, "\n\n")
}

func renderTopics(topics []Topic) string {
	var lines []string
	for _, t := range topics {
		if strings.TrimSpace(t.BodyMD) == "" {
			continue // missing prose is omitted, not a blank entry
		}
		head := "# topic: " + t.Slug
		if strings.TrimSpace(t.Title) != "" {
			head += " — " + t.Title
		}
		if len(t.Keywords) > 0 {
			head += "\nключевые слова: " + strings.Join(t.Keywords, ", ")
		}
		lines = append(lines, head+"\n"+strings.TrimSpace(t.BodyMD))
	}
	return strings.Join(lines, "\n\n")
}

// renderDescriptions renders entity trusted prose (product descriptions,
// tariff summaries, contact/policy prose). Empty fields are omitted entirely.
func renderDescriptions(input *PromptInput) string {
	var lines []string
	add := func(name, text string) {
		if s := strings.TrimSpace(text); s != "" {
			lines = append(lines, "- "+name+": "+s)
		}
	}
	for _, p := range input.Products {
		if !active(p.SalesStatus) {
			continue
		}
		name := p.Name
		if strings.TrimSpace(p.Category) != "" {
			name += " (" + p.Category + ")"
		}
		add(name, p.Description)
	}
	for _, t := range input.Tariffs {
		if !active(t.SalesStatus) {
			continue
		}
		var parts []string
		for _, s := range []string{t.Summary, t.LimitText, t.Advantages, t.Disadvantages} {
			if strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		if len(parts) > 0 {
			add("Тариф "+t.Name, strings.Join(parts, " "))
		}
	}
	if c := input.Contacts; c != nil {
		add("Адрес", c.Address)
		add("Реквизиты", c.LegalInformation)
		add("Обратный звонок", c.CallbackTime)
	}
	if p := input.Policies; p != nil {
		add("Предоплата", p.Prepayment)
		add("Рассрочка", p.Installment)
		add("Гарантия", p.Warranty)
	}
	return strings.Join(lines, "\n")
}

func renderFacts(facts []FactEntry) string {
	var lines []string
	for _, f := range facts {
		state := "—"
		if f.ReasoningState != "" {
			state = f.ReasoningState
		}
		note := "—"
		if f.UsageNote != "" {
			note = f.UsageNote
		}
		line := strings.Join([]string{f.Token, factLabel(f), string(f.Kind), state, note}, " | ")
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func factLabel(f FactEntry) string {
	switch f.Table {
	case "product", "tariff":
		return f.Ref + " — " + f.Label
	default:
		return f.Label
	}
}

func renderMediaCatalog(media []MediaEntry) string {
	if len(media) == 0 {
		return "—"
	}
	var lines []string
	for _, m := range media {
		lines = append(lines, fmt.Sprintf("%s — %d — %s", m.Token, m.Count, m.Label))
	}
	return strings.Join(lines, "\n")
}

// renderMediaAbsent lists records with NO media in any column, as 2-segment
// references (never mistakable for a 3-segment attachable token). "—" when
// every record has at least one populated column, so the slot never renders
// blank under its frame header.
func renderMediaAbsent(absent []AbsentEntry) string {
	if len(absent) == 0 {
		return "—"
	}
	var lines []string
	for _, a := range absent {
		lines = append(lines, a.Table+"."+a.Ref+" — "+a.DisplayName)
	}
	return strings.Join(lines, "\n")
}

var (
	leftoverSlotPattern = regexp.MustCompile(`%%[A-Z_]+%%`)
	fileExtPattern      = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|svg|mp4|mov|avi|mkv|pdf|docx?|xlsx?|pptx?|mp3|wav|ogg)\b`)
	uuidPattern         = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
)

// ValidatePrompt is the structural trust-boundary gate applied while
// rendering: no leftover %%SLOT%% markers, no placeholder outside the fact
// catalog, and no UUID- or file-extension-shaped substring anywhere in the
// text (defense in depth against a catalog or registry bug). It needs only
// the rendered text and the catalog the model was shown — never
// kbd_materials — because RenderPrompt itself never receives kbd_materials.
// A violation is a hard render failure.
func ValidatePrompt(prompt string, cat *Catalog) error {
	if m := leftoverSlotPattern.FindString(prompt); m != "" {
		return fmt.Errorf("aiprompt: unfilled prompt slot %s", m)
	}
	for _, tok := range placeholderPattern.FindAllString(prompt, -1) {
		if cat.FactByToken(tok) == nil {
			return fmt.Errorf("aiprompt: prompt contains placeholder %s that is not in the fact catalog", tok)
		}
	}
	if m := uuidPattern.FindString(prompt); m != "" {
		return fmt.Errorf("aiprompt: prompt leaks a UUID: %s", m)
	}
	if m := fileExtPattern.FindString(prompt); m != "" {
		return fmt.Errorf("aiprompt: prompt leaks a filename extension: %s", m)
	}
	return nil
}

// ValidateNoMaterialLeak re-checks a rendered prompt against the actual
// kbd_materials rows behind it: every distinguishing identifying field (id,
// source ref, filename, storage backend, storage key, MIME type) must be
// absent from the text verbatim. Short values (len < 6) are skipped because a
// short generic token cannot be reliably distinguished from ordinary prompt
// vocabulary — real material identifiers, filenames, and storage keys are
// always longer than that in practice, and tests use deliberately distinctive
// sentinel values. BuildPrompt runs this automatically; callers that render
// via the split BuildCatalog+RenderPrompt sequence should call it explicitly
// with the same materials passed to BuildCatalog.
func ValidateNoMaterialLeak(prompt string, materials []Material) error {
	for _, mat := range materials {
		for _, secret := range []string{mat.ID, mat.SourceRef, mat.Filename, mat.StorageBackend, mat.StorageKey, mat.MimeType} {
			if len(secret) < 6 {
				continue
			}
			if strings.Contains(prompt, secret) {
				return fmt.Errorf("aiprompt: prompt leaks kbd_materials value %q", secret)
			}
		}
	}
	return nil
}
