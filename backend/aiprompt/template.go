package aiprompt

import (
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Editable prompt templates.
//
// A template is the operator-editable BEHAVIOR text of the system prompt: the
// persona/profile focus, what each data block means, the escalation table, how
// to treat add-ons, schedules, payments, media, language and so on. Everything
// technical stays in code and is added by ComposeFrame around the template:
// the channel fact line, the strict-JSON contract line and %%RESPONSE_SCHEMA%%
// (whose field descriptions ValidateResponseV7 enforces), %%ASSISTANT%%, and
// the KB data sections with bare labels. Because the slots and fact
// placeholders never appear in editable text (ValidateTemplateInstructions
// rejects them), an edit cannot break the render, the schema or validation.

// Template profile IDs. They are the primary-key text IDs of
// ai_prompt_templates rows and the value of ai_assistants.prompt_template_id.
const (
	TemplateGeneral         = "general"
	TemplateOnlineShop      = "online-shop"
	TemplateServiceBusiness = "service-business"
	TemplateOnlineService   = "online-service"
)

// DefaultTemplateID is the profile every business starts on.
const DefaultTemplateID = TemplateGeneral

// MaxTemplateRunes bounds an instruction text: far above any real template
// (the defaults are ~15k characters) yet small enough to keep a runaway paste
// out of every prompt.
const MaxTemplateRunes = 40000

// PromptTemplateIDs returns every template ID in display order, General first.
func PromptTemplateIDs() []string {
	return []string{TemplateGeneral, TemplateOnlineShop, TemplateServiceBusiness, TemplateOnlineService}
}

// IsPromptTemplateID reports whether id names one of the four templates.
func IsPromptTemplateID(id string) bool {
	for _, known := range PromptTemplateIDs() {
		if id == known {
			return true
		}
	}
	return false
}

//go:embed templates/*.txt
var defaultTemplateFS embed.FS

// DefaultTemplateInstructions returns the shipped starting text of a template.
// These files are the single source of the migration's seed literals and of
// the rows SeedOrganization creates for a new business (a drift test in
// internal/dbtest pins the migration to them).
func DefaultTemplateInstructions(id string) (string, bool) {
	if !IsPromptTemplateID(id) {
		return "", false
	}
	b, err := defaultTemplateFS.ReadFile("templates/" + id + "-ru.txt")
	if err != nil {
		return "", false
	}
	return string(b), true
}

// DefaultPromptTemplate returns the shipped default of template id as an
// in-memory PromptTemplate (zero UpdatedAt, never persisted), or nil for an
// unknown ID. It is the fallback for an organization whose rows were never
// seeded, and a ready fixture for tests that build a KB without a database.
func DefaultPromptTemplate(id string) *PromptTemplate {
	text, ok := DefaultTemplateInstructions(id)
	if !ok {
		return nil
	}
	return &PromptTemplate{ID: id, Instructions: text}
}

// PromptTemplate is one business's template row as the prompt builder sees it.
type PromptTemplate struct {
	ID           string
	Instructions string
	UpdatedAt    time.Time
}

// Validation errors for operator-edited instruction text. They are sentinels so
// the HTTP layer can show a specific message for each.
var (
	ErrTemplateEmpty            = errors.New("template instructions are empty")
	ErrTemplateTooLong          = fmt.Errorf("template instructions exceed %d characters", MaxTemplateRunes)
	ErrTemplateReservedSyntax   = errors.New("template instructions must not contain %% slot markers or {{ }} placeholders: the response schema and KB values are added automatically")
	ErrTemplateLeakShapedString = errors.New("template instructions must not contain identifiers, file names or links (UUIDs, file extensions, scheme://… URLs)")
)

// ValidateTemplateInstructions is the blocking gate on edited text. It rejects
// only what would corrupt the render or the trust boundary: empty text, absurd
// length, reserved slot/placeholder syntax, and UUID/file-extension-shaped
// strings and URL/storage-locator-shaped spans that ValidatePrompt and
// ValidateNoStorageLocatorLeak would reject on EVERY later render (so a link in
// a template would turn every customer reply into a holding draft). It
// deliberately does NOT judge whether the rules are good — operators own that.
func ValidateTemplateInstructions(text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrTemplateEmpty
	}
	if utf8.RuneCountInString(text) > MaxTemplateRunes {
		return ErrTemplateTooLong
	}
	if strings.Contains(text, "%%") || strings.Contains(text, "{{") || strings.Contains(text, "}}") {
		return ErrTemplateReservedSyntax
	}
	if uuidPattern.MatchString(text) || fileExtPattern.MatchString(text) || storageLocatorPattern.MatchString(text) {
		return ErrTemplateLeakShapedString
	}
	return nil
}

// Bare section labels. They name a data block and describe its line format
// only — never a rule. The template text refers to blocks by these names.
const (
	LabelProductsAvailable   = "ТОВАРЫ ДОСТУПНЫЕ:"
	LabelProductsUnavailable = "ТОВАРЫ НЕДОСТУПНЫЕ (только названия):"
	LabelTariffs             = "ТАРИФЫ:"
	LabelTariffInfo          = "ТАРИФНАЯ ИНФОРМАЦИЯ:"
	LabelServices            = "УСЛУГИ:"
	LabelSpecialists         = "СПЕЦИАЛИСТЫ:"
	LabelBusinessTerms       = "УСЛОВИЯ И КОНТАКТЫ:"
	LabelTopics              = "ТЕМЫ:"
	LabelDeliveryZones       = "ЗОНЫ ДОСТАВКИ (ref | название | zone_level | parent_ref | примечание):"
	LabelBusinessFacts       = "BUSINESS FACTS (токен | факт | вид | состояние | примечание по использованию):"

	// ChannelLinePrefix and OutputContractLine are the protected technical
	// lines ComposeFrame adds around the editable text.
	ChannelLinePrefix  = "Канал переписки с клиентом: "
	OutputContractLine = "Ответ — строго JSON по схеме ниже (лишние поля запрещены)."
)

// ComposeFrame builds the full frame RenderPromptV7 fills: the operator's
// instructions followed by the protected structure. Data sections are included
// by what the KB actually holds, regardless of which profile is active — a
// business with products AND services gets both. channelLabel (for example
// "WhatsApp") adds the channel fact line; empty adds none.
func ComposeFrame(instructions string, kb *KB, channelLabel string) string {
	var b strings.Builder
	if s := strings.TrimSpace(instructions); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	if channelLabel != "" {
		b.WriteString(ChannelLinePrefix + channelLabel + ".\n\n")
	}
	b.WriteString(OutputContractLine + "\n\n" + SlotResponseSchema + "\n\n" + SlotAssistant)
	section := func(label, slot string) {
		b.WriteString("\n\n" + label + "\n\n" + slot)
	}
	if hasActiveProduct(kb, false) {
		section(LabelProductsAvailable, SlotProductsAvailable)
	}
	if hasActiveProduct(kb, true) {
		section(LabelProductsUnavailable, SlotProductsUnavailable)
	}
	if hasActiveTariff(kb) {
		section(LabelTariffs, SlotTariffCatalog)
	}
	if kb != nil && kb.TariffInfo != nil && len(kb.TariffInfo.AdditionalFacts) > 0 {
		section(LabelTariffInfo, SlotTariffInfo)
	}
	if hasActiveService(kb) {
		section(LabelServices, SlotServiceCatalog)
	}
	if hasActiveSpecialist(kb) {
		section(LabelSpecialists, SlotSpecialists)
	}
	if hasBusinessTerms(kb) {
		section(LabelBusinessTerms, SlotBusinessTerms)
	}
	section(LabelTopics, SlotTopics)
	if hasActiveZone(kb) {
		section(LabelDeliveryZones, SlotDeliveryZones)
	}
	section(LabelBusinessFacts, SlotBusinessFacts)
	b.WriteString("\n")
	return b.String()
}

// hasActiveProduct reports an active product; unavailableOnly narrows it to
// the name-only "unavailable" ones.
func hasActiveProduct(kb *KB, unavailableOnly bool) bool {
	if kb == nil {
		return false
	}
	for i := range kb.Products {
		p := &kb.Products[i]
		if !active(p.SalesStatus) {
			continue
		}
		if !unavailableOnly || p.AvailabilityStatus == "unavailable" {
			if unavailableOnly || productVisible(p) {
				return true
			}
		}
	}
	return false
}

func hasActiveTariff(kb *KB) bool {
	if kb == nil {
		return false
	}
	for i := range kb.Tariffs {
		if active(kb.Tariffs[i].SalesStatus) {
			return true
		}
	}
	return false
}

func hasActiveService(kb *KB) bool {
	if kb == nil {
		return false
	}
	for i := range kb.Services {
		if active(kb.Services[i].SalesStatus) {
			return true
		}
	}
	return false
}

func hasActiveSpecialist(kb *KB) bool {
	if kb == nil {
		return false
	}
	for i := range kb.Specialists {
		if active(kb.Specialists[i].SalesStatus) {
			return true
		}
	}
	return false
}

func hasActiveZone(kb *KB) bool {
	if kb == nil {
		return false
	}
	for i := range kb.DeliveryZones {
		if active(kb.DeliveryZones[i].SalesStatus) {
			return true
		}
	}
	return false
}

func hasBusinessTerms(kb *KB) bool {
	if kb == nil {
		return false
	}
	for _, group := range [][]termField{contactTerms(kb.Contacts), policyTerms(kb.Policies)} {
		for _, t := range group {
			if strings.TrimSpace(t.text) != "" {
				return true
			}
		}
	}
	return false
}
