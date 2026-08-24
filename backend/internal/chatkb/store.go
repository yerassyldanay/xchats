package chatkb

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// StoreService is v1's Service: full-context retrieval straight out of
// kbstore. SearchReal reads the live tables; SearchDraft reads the EFFECTIVE
// draft state — live rows overlaid by whatever is staged in kbd_draft, with
// rows marked for deletion removed.
//
// The effective state (kbstore.Draft) is deliberately what draft means here,
// rather than the pending-only change set (kbstore.DraftOnly). "What is the
// draft price of X?" has an answer for every product, not only for the ones
// somebody happens to have edited: for an untouched product the draft price
// IS the live price, and a retrieval that omitted it would make the
// assistant say the product does not exist in the draft. Difference() is
// what recovers "and here is what actually changed" from the two states.
type StoreService struct {
	KB *kbstore.Store
}

// NewStoreService builds the kbstore-backed retrieval service.
func NewStoreService(kb *kbstore.Store) *StoreService { return &StoreService{KB: kb} }

// SearchReal implements Service. query is unused in v1 — see the package doc
// comment for why the parameter exists anyway.
func (s *StoreService) SearchReal(ctx context.Context, orgID uuid.UUID, _ string) (Snapshot, error) {
	if s.KB == nil {
		return Snapshot{}, fmt.Errorf("chatkb: no knowledge base configured")
	}
	view, err := s.KB.LiveView(ctx, orgID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("chatkb: load live knowledge base: %w", err)
	}
	return Snapshot{Source: SourceReal, Records: recordsFromView(view, SourceReal)}, nil
}

// SearchDraft implements Service. See StoreService's doc comment for why
// this is the merged effective draft rather than the pending-only set.
func (s *StoreService) SearchDraft(ctx context.Context, orgID uuid.UUID, _ string) (Snapshot, error) {
	if s.KB == nil {
		return Snapshot{}, fmt.Errorf("chatkb: no knowledge base configured")
	}
	view, err := s.KB.Draft(ctx, orgID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("chatkb: load draft knowledge base: %w", err)
	}
	return Snapshot{Source: SourceDraft, Records: recordsFromView(view, SourceDraft)}, nil
}

// recordsFromView flattens a kbstore view into provenance-tagged records.
// Order is fixed (config, topics, products, tariffs, zones, contacts,
// policies) and mirrors KB_ENTITY_ORDER on the frontend, so the prompt reads
// the same way every time — a stable prompt is a cacheable prompt.
func recordsFromView(v *kbstore.DraftView, src Source) []Record {
	if v == nil {
		return nil
	}
	out := make([]Record, 0, 1+len(v.Topics)+len(v.Products)+len(v.Tariffs)+len(v.Zones)+len(v.Contacts)+len(v.Policies))

	out = append(out, Record{
		Kind: KindConfig, Key: kbstore.NaturalKeyMain, Title: "Assistant configuration", Source: src,
		Fields: nonEmpty([]Field{
			{Key: "persona", Label: "Persona", Value: v.Config.Persona},
			{Key: "mission", Label: "Mission", Value: v.Config.Mission},
			{Key: "guardrails", Label: "Guardrails", Value: v.Config.Guardrails},
			{Key: "language_policy", Label: "Language policy", Value: v.Config.LanguagePolicy},
			{Key: "reply_max_words", Label: "Reply max words", Value: strconv.Itoa(v.Config.ReplyMaxWords)},
		}),
	})

	for _, t := range v.Topics {
		out = append(out, Record{
			Kind: KindTopics, Key: t.Slug, Title: orKey(t.Title, t.Slug), Source: src,
			Fields: nonEmpty([]Field{
				{Key: "title", Label: "Title", Value: t.Title},
				{Key: "body_md", Label: "Body", Value: t.BodyMD},
			}),
		})
	}

	for _, p := range v.Products {
		out = append(out, Record{
			Kind: KindProducts, Key: p.Ref, Title: orKey(p.Name, p.Ref), Source: src,
			Fields: nonEmpty([]Field{
				{Key: "name", Label: "Name", Value: p.Name},
				{Key: "price", Label: "Price", Value: p.Price},
				{Key: "description", Label: "Description", Value: p.Description},
				{Key: "category", Label: "Category", Value: p.Category},
				{Key: "in_stock", Label: "In stock", Value: yesNo(p.InStock)},
				{Key: "sales_status", Label: "Sales status", Value: p.SalesStatus},
			}),
		})
	}

	for _, t := range v.Tariffs {
		out = append(out, Record{
			Kind: KindTariffs, Key: t.Ref, Title: orKey(t.Name, t.Ref), Source: src,
			Fields: nonEmpty([]Field{
				{Key: "name", Label: "Name", Value: t.Name},
				{Key: "price", Label: "Price", Value: t.Price},
				{Key: "pricing_type", Label: "Pricing type", Value: t.PricingType},
				{Key: "fee", Label: "Fee", Value: t.Fee},
				{Key: "limit_text", Label: "Limits", Value: t.LimitText},
				{Key: "summary", Label: "Summary", Value: t.Summary},
				{Key: "advantages", Label: "Advantages", Value: t.Advantages},
				{Key: "disadvantages", Label: "Disadvantages", Value: t.Disadvantages},
				{Key: "sales_status", Label: "Sales status", Value: t.SalesStatus},
			}),
		})
	}

	for _, z := range v.Zones {
		out = append(out, Record{
			Kind: KindZones, Key: z.Ref, Title: orKey(z.Name, z.Ref), Source: src,
			Fields: nonEmpty([]Field{
				{Key: "name", Label: "Name", Value: z.Name},
				{Key: "zone_level", Label: "Zone level", Value: z.ZoneLevel},
				{Key: "parent_ref", Label: "Parent zone", Value: z.ParentRef},
				{Key: "delivery_available", Label: "Delivery available", Value: yesNo(z.DeliveryAvailable)},
				{Key: "delivery_cost", Label: "Delivery cost", Value: z.DeliveryCost},
				{Key: "delivery_in_days", Label: "Delivery in days", Value: z.DeliveryInDays},
				{Key: "notes", Label: "Notes", Value: z.Notes},
				{Key: "sales_status", Label: "Sales status", Value: z.SalesStatus},
			}),
		})
	}

	for _, c := range v.Contacts {
		out = append(out, Record{
			Kind: KindContacts, Key: c.Slug, Title: "Contacts", Source: src,
			Fields: nonEmpty([]Field{
				{Key: "phone", Label: "Phone", Value: c.Phone},
				{Key: "whatsapp", Label: "WhatsApp", Value: c.WhatsApp},
				{Key: "email", Label: "Email", Value: c.Email},
				{Key: "website", Label: "Website", Value: c.Website},
				{Key: "instagram", Label: "Instagram", Value: c.Instagram},
				{Key: "address", Label: "Address", Value: c.Address},
				{Key: "working_hours", Label: "Working hours", Value: c.WorkingHours},
				{Key: "callback_time", Label: "Callback time", Value: c.CallbackTime},
				{Key: "legal_information", Label: "Legal information", Value: c.LegalInformation},
			}),
		})
	}

	for _, p := range v.Policies {
		out = append(out, Record{
			Kind: KindPolicies, Key: p.Slug, Title: "Policies", Source: src,
			Fields: nonEmpty([]Field{
				{Key: "delivery_cost", Label: "Delivery cost", Value: p.DeliveryCost},
				{Key: "delivery_in_days", Label: "Delivery in days", Value: p.DeliveryInDays},
				{Key: "free_delivery_from", Label: "Free delivery from", Value: p.FreeDeliveryFrom},
				{Key: "min_order", Label: "Minimum order", Value: p.MinOrder},
				{Key: "prepayment", Label: "Prepayment", Value: p.Prepayment},
				{Key: "installment", Label: "Installment", Value: p.Installment},
				{Key: "return_period_in_days", Label: "Return period in days", Value: p.ReturnPeriodInDays},
				{Key: "warranty", Label: "Warranty", Value: p.Warranty},
				{Key: "outside_zones_note", Label: "Outside delivery zones", Value: p.OutsideZonesNote},
			}),
		})
	}

	return out
}

// nonEmpty drops fields the org never filled in. A prompt line reading
// "Warranty: " teaches the model nothing and costs tokens; more importantly,
// an absent field and an empty one compare identically in Difference(), so
// dropping them here keeps "unset in both states" from ever reading as a
// change.
func nonEmpty(fields []Field) []Field {
	out := make([]Field, 0, len(fields))
	for _, f := range fields {
		if f.Value != "" {
			out = append(out, f)
		}
	}
	return out
}

func orKey(title, key string) string {
	if title != "" {
		return title
	}
	return key
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
