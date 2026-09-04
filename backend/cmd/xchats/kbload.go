package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// kbLoadDoc is the on-disk shape `kb-load` reads — see testdata/demo_kb.json.
// Zero KB values are compiled into this binary: every field here is read from
// the file at runtime, never a Go literal.
type kbLoadDoc struct {
	Assistant *kbLoadAssistant `json:"assistant"`
	Contacts  *kbLoadContacts  `json:"contacts"`
	Policies  *kbLoadPolicies  `json:"policies"`
	Topics    []kbLoadTopic    `json:"topics"`
	Products  []kbLoadProduct  `json:"products"`
	Tariffs   []kbLoadTariff   `json:"tariffs"`
	// Zones is loaded LAST (after policies): PutLiveZone re-validates the
	// org's whole zone/policy world on every write (validateZoneWorld), so
	// the policies row must already be zone-compatible (blank flat delivery
	// fields, a non-blank outside_zones_note) before the first zone lands.
	Zones []kbLoadZone `json:"zones"`
}

type kbLoadAssistant struct {
	Persona        string `json:"persona"`
	Mission        string `json:"mission"`
	Guardrails     string `json:"guardrails"`
	LanguagePolicy string `json:"language_policy"`
	ReplyMaxWords  int    `json:"reply_max_words"`
}

type kbLoadContacts struct {
	WhatsApp         string `json:"whatsapp"`
	Email            string `json:"email"`
	Address          string `json:"address"`
	LegalInformation string `json:"legal_information"`
	CallbackTime     string `json:"callback_time"`
	WorkingHours     string `json:"working_hours"`
	Phone            string `json:"phone"`
	Website          string `json:"website"`
	Instagram        string `json:"instagram"`
}

type kbLoadPolicies struct {
	DeliveryCost       string `json:"delivery_cost"`
	DeliveryInDays     string `json:"delivery_in_days"`
	FreeDeliveryFrom   string `json:"free_delivery_from"`
	MinOrder           string `json:"min_order"`
	Prepayment         string `json:"prepayment"`
	Installment        string `json:"installment"`
	ReturnPeriodInDays string `json:"return_period_in_days"`
	Warranty           string `json:"warranty"`
	OutsideZonesNote   string `json:"outside_zones_note"`
}

type kbLoadTopic struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	BodyMD string `json:"body_md"`
}

type kbLoadProduct struct {
	Ref         string `json:"ref"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	Description string `json:"description"`
	Category    string `json:"category"`
	// AvailabilityStatus is one of in_stock|preorder|on_demand|unavailable
	// (replaces the legacy in_stock boolean — migration
	// 0017_kb_virtual_facts); empty defaults to "in_stock".
	AvailabilityStatus string `json:"availability_status"`
}

type kbLoadTariff struct {
	Ref           string `json:"ref"`
	Name          string `json:"name"`
	Price         string `json:"price"`
	LimitText     string `json:"limit_text"`
	Fee           string `json:"fee"`
	Summary       string `json:"summary"`
	PricingType   string `json:"pricing_type"`
	Advantages    string `json:"advantages"`
	Disadvantages string `json:"disadvantages"`
}

type kbLoadZone struct {
	Ref               string `json:"ref"`
	Name              string `json:"name"`
	ZoneLevel         string `json:"zone_level"`
	ParentRef         string `json:"parent_ref"`
	DeliveryAvailable bool   `json:"delivery_available"`
	DeliveryCost      string `json:"delivery_cost"`
	DeliveryInDays    string `json:"delivery_in_days"`
	Notes             string `json:"notes"`
	SalesStatus       string `json:"sales_status"`
}

// runKBLoad implements `xchats kb-load -file <path> [-remove]`: reads a
// demo_kb.json-shaped file and writes it into the org's LIVE knowledge base
// through the exact same kbstore functions the /kb/* HTTP editor uses, so
// every content gate and the zones invariant apply exactly as they would to
// a human operator's edit — never a bypass path. Idempotent: re-running the
// same file upserts the same rows by their natural key (ref/slug/lang), never
// duplicating. actor is uuid.Nil throughout (a CLI run has no authenticated
// user); auditRow stores that as a NULL actor_user_id.
func runKBLoad(cfg *config.Config, log *slog.Logger, args []string) {
	fs := flag.NewFlagSet("kb-load", flag.ExitOnError)
	filePath := fs.String("file", "", "path to a demo-kb JSON file (see testdata/demo_kb.json)")
	remove := fs.Bool("remove", false, "delete the refs/slugs listed in -file instead of loading them")
	_ = fs.Parse(args)
	if *filePath == "" {
		fatal("kb-load", errString("-file is required"))
	}

	raw, err := os.ReadFile(*filePath)
	if err != nil {
		fatal("kb-load: read file", err)
	}
	var doc kbLoadDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		fatal("kb-load: parse file", err)
	}

	ctx := context.Background()
	st := mustStore(cfg, log)
	defer st.Close()
	org, err := st.SeedOrganization(ctx, cfg.OrgName)
	if err != nil {
		fatal("kb-load: resolve organization", err)
	}
	kb, err := kbstore.New(ctx, cfg.Storage.DBPath)
	if err != nil {
		fatal("kb-load: open knowledge base", err)
	}
	defer kb.Close()

	if *remove {
		removeKBLoadDoc(ctx, kb, org.ID, doc, log)
		return
	}
	applyKBLoadDoc(ctx, kb, org.ID, doc, log)
}

func applyKBLoadDoc(ctx context.Context, kb *kbstore.Store, orgID uuid.UUID, doc kbLoadDoc, log *slog.Logger) {
	if a := doc.Assistant; a != nil {
		if err := kb.PatchLiveConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{
			Persona: &a.Persona, Mission: &a.Mission, Guardrails: &a.Guardrails,
			LanguagePolicy: &a.LanguagePolicy, ReplyMaxWords: &a.ReplyMaxWords,
		}); err != nil {
			fatal("kb-load: assistant", err)
		}
		log.Info("kb-load: assistant config upserted")
	}
	if c := doc.Contacts; c != nil {
		if err := kb.PatchLiveContacts(ctx, orgID, uuid.Nil, kbstore.ContactPatch{
			WhatsApp: &c.WhatsApp, Email: &c.Email, Address: &c.Address, LegalInformation: &c.LegalInformation,
			CallbackTime: &c.CallbackTime, WorkingHours: &c.WorkingHours, Phone: &c.Phone,
			Website: &c.Website, Instagram: &c.Instagram,
		}); err != nil {
			fatal("kb-load: contacts", err)
		}
		log.Info("kb-load: contacts upserted")
	}
	if p := doc.Policies; p != nil {
		if err := kb.PatchLivePolicies(ctx, orgID, uuid.Nil, kbstore.PolicyPatch{
			DeliveryCost: &p.DeliveryCost, DeliveryInDays: &p.DeliveryInDays,
			FreeDeliveryFrom: &p.FreeDeliveryFrom, MinOrder: &p.MinOrder, Prepayment: &p.Prepayment,
			Installment: &p.Installment, ReturnPeriodInDays: &p.ReturnPeriodInDays, Warranty: &p.Warranty,
			OutsideZonesNote: &p.OutsideZonesNote,
		}); err != nil {
			fatal("kb-load: policies", err)
		}
		log.Info("kb-load: policies upserted")
	}
	for _, t := range doc.Topics {
		if err := kb.PutLiveTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{
			Slug: t.Slug, Title: t.Title, BodyMD: t.BodyMD,
		}); err != nil {
			fatal("kb-load: topic "+t.Slug, err)
		}
	}
	log.Info("kb-load: topics upserted", "count", len(doc.Topics))
	for _, p := range doc.Products {
		status := p.AvailabilityStatus
		if status == "" {
			status = "in_stock"
		}
		if err := kb.PutLiveProduct(ctx, orgID, uuid.Nil, kbstore.ProductInput{
			Ref: p.Ref, Name: p.Name, Price: p.Price, Description: p.Description,
			Category: p.Category, AvailabilityStatus: &status,
		}); err != nil {
			fatal("kb-load: product "+p.Ref, err)
		}
	}
	log.Info("kb-load: products upserted", "count", len(doc.Products))
	for _, t := range doc.Tariffs {
		if err := kb.PutLiveTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{
			Ref: t.Ref, Name: t.Name, Price: t.Price, LimitText: t.LimitText, Fee: t.Fee,
			Summary: t.Summary, PricingType: t.PricingType, Advantages: t.Advantages, Disadvantages: t.Disadvantages,
		}); err != nil {
			fatal("kb-load: tariff "+t.Ref, err)
		}
	}
	log.Info("kb-load: tariffs upserted", "count", len(doc.Tariffs))
	for _, z := range doc.Zones {
		if err := kb.PutLiveZone(ctx, orgID, uuid.Nil, kbstore.ZoneInput{
			Ref: z.Ref, Name: z.Name, ZoneLevel: z.ZoneLevel, ParentRef: z.ParentRef,
			DeliveryAvailable: z.DeliveryAvailable, DeliveryCost: z.DeliveryCost, DeliveryInDays: z.DeliveryInDays,
			Notes: z.Notes, SalesStatus: z.SalesStatus,
		}); err != nil {
			fatal("kb-load: zone "+z.Ref, err)
		}
	}
	log.Info("kb-load: zones upserted", "count", len(doc.Zones))
	log.Info("kb-load: done")
}

// removeKBLoadDoc undoes applyKBLoadDoc, so the demo dataset is fully
// removable and leaves nothing behind. Order matters for zones (removeZones
// deletes leaves before parents — deleting a parent_ref's target while a
// child zone still references it would fail validateZoneWorld) and for the
// singletons (assistant/contacts/policies): the live editor's kbstore API
// only ever PATCHES these (no DeleteLiveContacts/DeleteLivePolicies/
// DeleteLiveConfig — an operator never "deletes" their own shop's contact
// card), but -remove needs the KB to go fully blank again so GET /kb/prompt
// returns to its honest "not configured" error state, so this deletes those
// rows directly via SQL rather than growing kbstore's live-editor surface
// with delete methods the UI itself would have no use for.
func removeKBLoadDoc(ctx context.Context, kb *kbstore.Store, orgID uuid.UUID, doc kbLoadDoc, log *slog.Logger) {
	if err := removeZones(ctx, kb, orgID, doc.Zones); err != nil {
		fatal("kb-load -remove: zones", err)
	}
	for _, t := range doc.Tariffs {
		if err := kb.DeleteLiveTariff(ctx, orgID, uuid.Nil, t.Ref); err != nil {
			fatal("kb-load -remove: tariff "+t.Ref, err)
		}
	}
	for _, p := range doc.Products {
		if err := kb.DeleteLiveProduct(ctx, orgID, uuid.Nil, p.Ref); err != nil {
			fatal("kb-load -remove: product "+p.Ref, err)
		}
	}
	for _, t := range doc.Topics {
		if err := kb.DeleteLiveTopic(ctx, orgID, uuid.Nil, t.Slug); err != nil {
			fatal("kb-load -remove: topic "+t.Slug, err)
		}
	}
	if doc.Assistant != nil {
		if err := kb.DeleteLiveConfig(ctx, orgID, uuid.Nil); err != nil {
			fatal("kb-load -remove: assistant", err)
		}
	}
	if doc.Contacts != nil {
		if err := kb.DeleteLiveContacts(ctx, orgID, uuid.Nil); err != nil {
			fatal("kb-load -remove: contacts", err)
		}
	}
	if doc.Policies != nil {
		if err := kb.DeleteLivePolicies(ctx, orgID, uuid.Nil); err != nil {
			fatal("kb-load -remove: policies", err)
		}
	}
	log.Info("kb-load -remove: done", "zones", len(doc.Zones), "tariffs", len(doc.Tariffs),
		"products", len(doc.Products), "topics", len(doc.Topics))
}

// removeZones deletes every zone in zones, leaves (zones no OTHER zone in the
// set still points to via parent_ref) before their parents — a plain
// repeated-leaf-removal topological delete, so any acyclic parent_ref
// hierarchy the file describes is removed in a validateZoneWorld-safe order
// regardless of the order the file itself lists them in.
func removeZones(ctx context.Context, kb *kbstore.Store, orgID uuid.UUID, zones []kbLoadZone) error {
	remaining := make(map[string]kbLoadZone, len(zones))
	for _, z := range zones {
		remaining[z.Ref] = z
	}
	for len(remaining) > 0 {
		isParent := map[string]bool{}
		for _, z := range remaining {
			if z.ParentRef != "" {
				isParent[z.ParentRef] = true
			}
		}
		progressed := false
		for ref, z := range remaining {
			if isParent[ref] {
				continue // still someone's parent among the remaining set — delete later
			}
			if err := kb.DeleteLiveZone(ctx, orgID, uuid.Nil, z.Ref); err != nil {
				return fmt.Errorf("zone %s: %w", ref, err)
			}
			delete(remaining, ref)
			progressed = true
		}
		if !progressed {
			refs := make([]string, 0, len(remaining))
			for ref := range remaining {
				refs = append(refs, ref)
			}
			return fmt.Errorf("zone deletion order: %d zone(s) still reference each other: %v", len(remaining), refs)
		}
	}
	return nil
}
