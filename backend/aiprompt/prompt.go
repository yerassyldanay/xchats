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

// BuildPrompt renders the stable prompt prefix: frame + assistant config +
// approved prose + fact-placeholder catalog + semantic media catalog +
// media-absence list + response schema. It returns the catalog alongside so
// callers validate responses against exactly what the model saw.
func BuildPrompt(frame string, kb *KB) (string, *Catalog, error) {
	cat, err := BuildCatalog(kb)
	if err != nil {
		return "", nil, err
	}
	out := frame
	out = strings.ReplaceAll(out, SlotAssistant, renderAssistant(kb.Assistant))
	out = strings.ReplaceAll(out, SlotKnowledgeBase, renderTopics(kb.Topics))
	out = strings.ReplaceAll(out, SlotDescriptions, renderDescriptions(kb))
	out = strings.ReplaceAll(out, SlotFacts, renderFacts(cat.Facts))
	out = strings.ReplaceAll(out, SlotMedia, renderMediaCatalog(cat.Media))
	out = strings.ReplaceAll(out, SlotMediaAbsent, renderMediaAbsent(cat.Absent))
	out = strings.ReplaceAll(out, SlotResponseSchema, RenderResponseSchema())
	if err := ValidatePrompt(out, kb, cat); err != nil {
		return "", nil, err
	}
	return out, cat, nil
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
func renderDescriptions(kb *KB) string {
	var lines []string
	add := func(name, text string) {
		if s := strings.TrimSpace(text); s != "" {
			lines = append(lines, "- "+name+": "+s)
		}
	}
	for _, p := range kb.Products {
		if !active(p.SalesStatus) {
			continue
		}
		name := p.Name
		if strings.TrimSpace(p.Category) != "" {
			name += " (" + p.Category + ")"
		}
		add(name, p.Description)
	}
	for _, t := range kb.Tariffs {
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
	if c := kb.Contacts; c != nil {
		add("Адрес", c.Address)
		add("Реквизиты", c.LegalInformation)
		add("Обратный звонок", c.CallbackTime)
	}
	if p := kb.Policies; p != nil {
		add("Предоплата", p.Prepayment)
		add("Рассрочка", p.Installment)
		add("Гарантия", p.Warranty)
	}
	return strings.Join(lines, "\n")
}

func renderFacts(facts []FactEntry) string {
	var lines []string
	for _, f := range facts {
		line := f.Token + " | " + factLabel(f) + " | " + f.Value
		if f.UsageNote != "" {
			line += " | " + f.UsageNote
		}
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

// ValidatePrompt is the trust-boundary gate: no leftover slots, no unresolved
// placeholders outside the catalog, and no leakage of anything from
// kbd_materials — UUIDs, filenames, extensions, storage backends or keys.
// A violation is a hard render failure.
func ValidatePrompt(prompt string, kb *KB, cat *Catalog) error {
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
	for _, mat := range kb.Materials {
		for _, secret := range []string{mat.ID, mat.Filename, mat.StorageKey} {
			if secret != "" && strings.Contains(prompt, secret) {
				return fmt.Errorf("aiprompt: prompt leaks kbd_materials value %q", secret)
			}
		}
	}
	return nil
}
