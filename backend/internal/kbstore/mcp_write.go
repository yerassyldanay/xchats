package kbstore

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// UpsertResult is what every MCP upsert tool returns to the model: which key
// the record now lives under (echoed back so the model can attach it to a
// later media upsert or reference it again), whether this call created a new
// record, and the draft's new version (for the next call's
// expected_draft_version).
type UpsertResult struct {
	Type         string `json:"type"`
	Key          string `json:"key"`
	Created      bool   `json:"created"`
	DraftVersion int64  `json:"draft_version"`
}

// ---------------------------------------------------------------------------
// Natural-key resolution + duplicate detection (plan/mcp.md §4)
// ---------------------------------------------------------------------------

// resolveUpsertKey runs the plan/mcp.md §4 decision sequence:
//  1. an existing key (live or draft) → update. If the update ALSO renames
//     the record (title is non-blank), the rename itself is still checked
//     against every OTHER key's title — renaming record A onto record B's
//     exact title would otherwise silently merge their identities in every
//     future kb_summary/duplicate-detection view; checkDuplicate never
//     collides a key with itself, so this cannot false-positive on an
//     unchanged or self-only title.
//  2. a new/missing key whose title exactly matches another key's title →
//     ErrDuplicateConflict.
//  3. a new/missing key whose title is merely SIMILAR to another key's title
//     → ErrAmbiguousMatch.
//  4. otherwise → create, generating a stable key from title when key is "".
//
// index must already be scoped to this kbType (IdentityIndex(ctx, org,
// []string{kbType})) and is read within the caller's writeDraftBlobVersioned
// mutate callback, so it observes a consistent snapshot with respect to any
// concurrent MCP writer on the SAME organization (they serialize on the
// kbd_draft row lock) — see this file's package doc.
func resolveUpsertKey(kbType, key, title string, index []Identity) (resolvedKey string, creating bool, err error) {
	if key != "" {
		for _, id := range index {
			if id.Key == key {
				if strings.TrimSpace(title) != "" {
					if verdict := checkDuplicate(kbType, key, title, index); verdict.Conflict != nil {
						return "", false, &ErrDuplicateConflict{Type: kbType, ExistingKey: verdict.Conflict.Key, ExistingTitle: verdict.Conflict.Title}
					}
				}
				return key, false, nil // step 1
			}
		}
	}
	if strings.TrimSpace(title) == "" {
		if key == "" {
			return "", false, &ErrRequiredFieldMissing{Field: "a title/name (to derive a key) or an explicit key"}
		}
		return key, true, nil // explicit new key, nothing to duplicate-check against
	}
	verdict := checkDuplicate(kbType, key, title, index)
	if verdict.Conflict != nil {
		return "", false, &ErrDuplicateConflict{Type: kbType, ExistingKey: verdict.Conflict.Key, ExistingTitle: verdict.Conflict.Title}
	}
	if len(verdict.Candidates) > 0 {
		return "", false, &ErrAmbiguousMatch{Type: kbType, Candidates: verdict.Candidates}
	}
	if key != "" {
		return key, true, nil // step 4, explicit key
	}
	taken := map[string]bool{}
	for _, id := range index {
		taken[id.Key] = true
	}
	return uniqueKey(slugifyOrFallback(title), taken), true, nil // step 4, derived key
}

var dashCollapseRE = regexp.MustCompile(`-+`)

// cyrillicTranslit is a practical (not linguistically exhaustive)
// transliteration table: KB titles/names are Russian prose
// (plan/DECISIONS.md — "V1 KB prose and replies are Russian only"), while
// every existing natural key in this codebase is ASCII
// ("coffee-machine", "how-to-order", "business").
var cyrillicTranslit = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "i", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// slugify lowercases, transliterates Cyrillic, and folds every other
// non-alphanumeric run into a single '-'.
func slugify(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	for _, r := range lower {
		if lat, ok := cyrillicTranslit[r]; ok {
			b.WriteString(lat)
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.Trim(dashCollapseRE.ReplaceAllString(b.String(), "-"), "-")
}

// slugifyOrFallback guarantees a non-empty slug even for a title that
// transliterates to nothing (emoji-only, punctuation-only) — a short random
// suffix beats a blank/duplicate key.
func slugifyOrFallback(title string) string {
	if s := slugify(title); s != "" {
		return s
	}
	return "item-" + randomID(6)
}

// uniqueKey appends -2, -3, ... until base does not collide with an already
// taken key — natural-key uniqueness is enforced at the database level
// too (UNIQUE(organization_id, ref)), this just avoids the round-trip of
// discovering that the obvious way.
func uniqueKey(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if !taken[candidate] {
			return candidate
		}
	}
}

func randomID(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	seed := uuid.New() // crypto/rand-backed (google/uuid uses crypto/rand by default)
	for i := range b {
		b[i] = alphabet[int(seed[i%len(seed)])%len(alphabet)]
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Assistant (singleton, patch-only — no duplicate concept)
// ---------------------------------------------------------------------------

// AssistantChanges is kb_assistant_upsert's `changes` (plan/mcp.md §5).
type AssistantChanges struct {
	Persona, Mission, Guardrails, LanguagePolicy *string
	ReplyMaxWords                                *int
}

// MCPUpsertAssistant patches the assistant singleton in the draft. userID is
// the verified Principal.UserID, recorded as kbd_draft.updated_by.
func (s *Store) MCPUpsertAssistant(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, ch AssistantChanges, expectedVersion *int64) (UpsertResult, error) {
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		if ch.Persona != nil {
			b.Config.Persona = ch.Persona
		}
		if ch.Mission != nil {
			b.Config.Mission = ch.Mission
		}
		if ch.Guardrails != nil {
			b.Config.Guardrails = ch.Guardrails
		}
		if ch.LanguagePolicy != nil {
			b.Config.LanguagePolicy = ch.LanguagePolicy
		}
		if ch.ReplyMaxWords != nil {
			b.Config.ReplyMaxWords = ch.ReplyMaxWords
		}
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{Type: KBTypeAssistant, Key: NaturalKeyMain, DraftVersion: newVersion}, nil
}

// ---------------------------------------------------------------------------
// Topic
// ---------------------------------------------------------------------------

// TopicChanges is kb_topic_upsert's `changes`.
type TopicChanges struct {
	Title, BodyMD                                           *string
	FeaturedImage                                           **uuid.UUID
	IllustrationImages, ExplainerVideos, ReferenceDocuments *[]uuid.UUID
}

// MCPUpsertTopic creates or patches a topic (plan/mcp.md §5's kb_topic_upsert).
func (s *Store) MCPUpsertTopic(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, slug string, ch TopicChanges, expectedVersion *int64, provenance MCPProvenance) (UpsertResult, error) {
	var result UpsertResult
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		index, err := s.identityIndex(ctx, db, orgID, []string{KBTypeTopic}, "both", "")
		if err != nil {
			return err
		}
		title := ""
		if ch.Title != nil {
			title = *ch.Title
		}
		key, creating, err := resolveUpsertKey(KBTypeTopic, slug, title, index)
		if err != nil {
			return err
		}
		cur, err := s.currentTopic(ctx, db, orgID, key, b)
		if err != nil {
			return err
		}
		cur.Slug = key
		if ch.Title != nil {
			cur.Title = *ch.Title
		}
		if ch.BodyMD != nil {
			cur.BodyMD = *ch.BodyMD
		}
		refs := applyTopicMedia(&cur, TopicMedia{
			FeaturedImage:      ch.FeaturedImage,
			IllustrationImages: ch.IllustrationImages,
			ExplainerVideos:    ch.ExplainerVideos,
			ReferenceDocuments: ch.ReferenceDocuments,
		})
		if creating && strings.TrimSpace(cur.Title) == "" {
			return &ErrRequiredFieldMissing{Field: "title"}
		}
		if reasons := gateTopicBody(key, cur.BodyMD); len(reasons) > 0 {
			return &GateError{Reasons: reasons}
		}
		if err := validateMediaRefs(ctx, db, orgID, refs); err != nil {
			return err
		}
		if err := s.recordProvenance(ctx, db, orgID, KBTypeTopic, key, provenance); err != nil {
			return err
		}
		b.upsertTopic(cur)
		// An upsert on a key currently staged for deletion means the caller
		// is choosing to keep/update it, not delete it — cancel the pending
		// delete so it and this patch don't both survive to Approve (where
		// the delete would silently discard the update immediately after
		// it materializes).
		b.removeDelete(deleteKindFor(KBTypeTopic), key)
		result = UpsertResult{Type: KBTypeTopic, Key: key, Created: creating}
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	result.DraftVersion = newVersion
	return result, nil
}

// ---------------------------------------------------------------------------
// Product
// ---------------------------------------------------------------------------

// ProductChanges is kb_product_upsert's `changes`. AvailabilityStatus
// replaces the legacy InStock *bool (migration 0017_kb_virtual_facts) —
// required on create, same as InStock was. AdditionalFacts is nil-means-
// unchanged, non-nil (even empty) means replace (media_apply.go's
// pointer-to-slice convention, applied here to a fact list instead of a
// uuid list).
type ProductChanges struct {
	Name, Price, Description, Category, Brand, Advantages, Disadvantages, BestFor, NotFor *string
	AvailabilityStatus                                                                    *string // required on create
	AvailabilityNote, InstallationTerms, WarrantyTerms                                    *string
	AdditionalFacts                                                                       *[]aiprompt.AdditionalFact
	SalesStatus                                                                           *string
	FeaturedImage                                                                         **uuid.UUID
	GalleryImages, DemoVideos, CertificateDocuments, GuaranteeDocuments                   *[]uuid.UUID
}

// MCPUpsertProduct creates or patches a product (kb_product_upsert).
func (s *Store) MCPUpsertProduct(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, ref string, ch ProductChanges, expectedVersion *int64, provenance MCPProvenance) (UpsertResult, error) {
	var result UpsertResult
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		index, err := s.identityIndex(ctx, db, orgID, []string{KBTypeProduct}, "both", "")
		if err != nil {
			return err
		}
		title := ""
		if ch.Name != nil {
			title = *ch.Name
		}
		key, creating, err := resolveUpsertKey(KBTypeProduct, ref, title, index)
		if err != nil {
			return err
		}
		cur, err := s.currentProduct(ctx, db, orgID, key, b)
		if err != nil {
			return err
		}
		cur.Ref = key
		if ch.Name != nil {
			cur.Name = *ch.Name
		}
		if ch.Price != nil {
			cur.Price = *ch.Price
		}
		if ch.Description != nil {
			cur.Description = *ch.Description
		}
		if ch.Category != nil {
			cur.Category = *ch.Category
		}
		if ch.Brand != nil {
			cur.Brand = *ch.Brand
		}
		if ch.Advantages != nil {
			cur.Advantages = *ch.Advantages
		}
		if ch.Disadvantages != nil {
			cur.Disadvantages = *ch.Disadvantages
		}
		if ch.BestFor != nil {
			cur.BestFor = *ch.BestFor
		}
		if ch.NotFor != nil {
			cur.NotFor = *ch.NotFor
		}
		if ch.AvailabilityNote != nil {
			cur.AvailabilityNote = *ch.AvailabilityNote
		}
		if ch.InstallationTerms != nil {
			cur.InstallationTerms = *ch.InstallationTerms
		}
		if ch.WarrantyTerms != nil {
			cur.WarrantyTerms = *ch.WarrantyTerms
		}
		if ch.SalesStatus != nil {
			if err := validateEnum("sales_status", *ch.SalesStatus, "active", "inactive"); err != nil {
				return err
			}
			cur.SalesStatus = *ch.SalesStatus
		}
		if creating && ch.AvailabilityStatus == nil {
			return &ErrRequiredFieldMissing{Field: "availability_status"}
		}
		if ch.AvailabilityStatus != nil {
			if err := validateEnum("availability_status", *ch.AvailabilityStatus, "in_stock", "preorder", "on_demand", "unavailable"); err != nil {
				return err
			}
			cur.AvailabilityStatus = *ch.AvailabilityStatus
		}
		if ch.AdditionalFacts != nil {
			cur.AdditionalFacts = *ch.AdditionalFacts
		}
		if err := validateProductFacts(cur); err != nil {
			return err
		}
		refs := applyProductMedia(&cur, ProductMedia{
			FeaturedImage:        ch.FeaturedImage,
			GalleryImages:        ch.GalleryImages,
			DemoVideos:           ch.DemoVideos,
			CertificateDocuments: ch.CertificateDocuments,
			GuaranteeDocuments:   ch.GuaranteeDocuments,
		})
		if creating && strings.TrimSpace(cur.Name) == "" {
			return &ErrRequiredFieldMissing{Field: "name"}
		}
		if err := validateMediaRefs(ctx, db, orgID, refs); err != nil {
			return err
		}
		if err := s.recordProvenance(ctx, db, orgID, KBTypeProduct, key, provenance); err != nil {
			return err
		}
		b.upsertProduct(cur)
		b.removeDelete(deleteKindFor(KBTypeProduct), key) // upserting cancels a pending delete — see MCPUpsertTopic's comment
		result = UpsertResult{Type: KBTypeProduct, Key: key, Created: creating}
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	result.DraftVersion = newVersion
	return result, nil
}

// ---------------------------------------------------------------------------
// Tariff
// ---------------------------------------------------------------------------

// TariffChanges is kb_tariff_upsert's `changes`. AdditionalFacts is
// nil-means-unchanged, non-nil (even empty) means replace (ProductChanges'
// doc comment).
type TariffChanges struct {
	Name, Price, LimitText, Fee, Summary, PricingType, Advantages, Disadvantages, BestFor, NotFor *string
	AdditionalFacts                                                                               *[]aiprompt.AdditionalFact
	SalesStatus                                                                                   *string
	FeaturedImage                                                                                 **uuid.UUID
	PricingImages, ExplainerVideos, TermsDocuments                                                *[]uuid.UUID
}

// MCPUpsertTariff creates or patches a tariff (kb_tariff_upsert).
func (s *Store) MCPUpsertTariff(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, ref string, ch TariffChanges, expectedVersion *int64, provenance MCPProvenance) (UpsertResult, error) {
	var result UpsertResult
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		index, err := s.identityIndex(ctx, db, orgID, []string{KBTypeTariff}, "both", "")
		if err != nil {
			return err
		}
		title := ""
		if ch.Name != nil {
			title = *ch.Name
		}
		key, creating, err := resolveUpsertKey(KBTypeTariff, ref, title, index)
		if err != nil {
			return err
		}
		cur, err := s.currentTariff(ctx, db, orgID, key, b)
		if err != nil {
			return err
		}
		cur.Ref = key
		if ch.Name != nil {
			cur.Name = *ch.Name
		}
		if ch.Price != nil {
			cur.Price = *ch.Price
		}
		if ch.LimitText != nil {
			cur.LimitText = *ch.LimitText
		}
		if ch.Fee != nil {
			cur.Fee = *ch.Fee
		}
		if ch.Summary != nil {
			cur.Summary = *ch.Summary
		}
		if creating && (ch.PricingType == nil || strings.TrimSpace(*ch.PricingType) == "") {
			return &ErrRequiredFieldMissing{Field: "pricing_type"}
		}
		if ch.PricingType != nil {
			if err := validateEnum("pricing_type", *ch.PricingType, "fixed", "percentage", "tiered", "hybrid"); err != nil {
				return err
			}
			cur.PricingType = *ch.PricingType
		}
		if ch.Advantages != nil {
			cur.Advantages = *ch.Advantages
		}
		if ch.Disadvantages != nil {
			cur.Disadvantages = *ch.Disadvantages
		}
		if ch.BestFor != nil {
			cur.BestFor = *ch.BestFor
		}
		if ch.NotFor != nil {
			cur.NotFor = *ch.NotFor
		}
		if ch.AdditionalFacts != nil {
			cur.AdditionalFacts = *ch.AdditionalFacts
		}
		if ch.SalesStatus != nil {
			if err := validateEnum("sales_status", *ch.SalesStatus, "active", "inactive"); err != nil {
				return err
			}
			cur.SalesStatus = *ch.SalesStatus
		}
		if err := validateTariffFacts(cur); err != nil {
			return err
		}
		refs := applyTariffMedia(&cur, TariffMedia{
			FeaturedImage:   ch.FeaturedImage,
			PricingImages:   ch.PricingImages,
			ExplainerVideos: ch.ExplainerVideos,
			TermsDocuments:  ch.TermsDocuments,
		})
		if creating && strings.TrimSpace(cur.Name) == "" {
			return &ErrRequiredFieldMissing{Field: "name"}
		}
		if err := validateMediaRefs(ctx, db, orgID, refs); err != nil {
			return err
		}
		if err := s.recordProvenance(ctx, db, orgID, KBTypeTariff, key, provenance); err != nil {
			return err
		}
		b.upsertTariff(cur)
		b.removeDelete(deleteKindFor(KBTypeTariff), key) // upserting cancels a pending delete — see MCPUpsertTopic's comment
		result = UpsertResult{Type: KBTypeTariff, Key: key, Created: creating}
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	result.DraftVersion = newVersion
	return result, nil
}

// ---------------------------------------------------------------------------
// Contacts (singleton, patch-only)
// ---------------------------------------------------------------------------

// ContactsChanges is kb_contacts_upsert's `changes`.
type ContactsChanges struct {
	WhatsApp, Email, Address, LegalInformation, CallbackTime, WorkingHours, Phone, Website, Instagram *string
	ContactCardImage, LocationMapImage                                                                **uuid.UUID
	CompanyLegalDocuments                                                                             *[]uuid.UUID
}

// MCPUpsertContacts patches the contacts singleton.
func (s *Store) MCPUpsertContacts(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, ch ContactsChanges, expectedVersion *int64, provenance MCPProvenance) (UpsertResult, error) {
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentContact(ctx, db, orgID, b)
		if err != nil {
			return err
		}
		if ch.WhatsApp != nil {
			cur.WhatsApp = *ch.WhatsApp
		}
		if ch.Email != nil {
			cur.Email = *ch.Email
		}
		if ch.Address != nil {
			cur.Address = *ch.Address
		}
		if ch.LegalInformation != nil {
			cur.LegalInformation = *ch.LegalInformation
		}
		if ch.CallbackTime != nil {
			cur.CallbackTime = *ch.CallbackTime
		}
		if ch.WorkingHours != nil {
			cur.WorkingHours = *ch.WorkingHours
		}
		if ch.Phone != nil {
			cur.Phone = *ch.Phone
		}
		if ch.Website != nil {
			cur.Website = *ch.Website
		}
		if ch.Instagram != nil {
			cur.Instagram = *ch.Instagram
		}
		refs := applyContactsMedia(&cur, ContactsMedia{
			ContactCardImage:      ch.ContactCardImage,
			LocationMapImage:      ch.LocationMapImage,
			CompanyLegalDocuments: ch.CompanyLegalDocuments,
		})
		if err := validateMediaRefs(ctx, db, orgID, refs); err != nil {
			return err
		}
		if err := s.recordProvenance(ctx, db, orgID, KBTypeContacts, NaturalKeyMain, provenance); err != nil {
			return err
		}
		b.upsertContact(cur)
		b.removeDelete(deleteKindFor(KBTypeContacts), NaturalKeyMain) // upserting cancels a pending delete — see MCPUpsertTopic's comment
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{Type: KBTypeContacts, Key: NaturalKeyMain, DraftVersion: newVersion}, nil
}

// ---------------------------------------------------------------------------
// Policies (singleton, patch-only)
// ---------------------------------------------------------------------------

// PoliciesChanges is kb_policies_upsert's `changes`.
type PoliciesChanges struct {
	DeliveryCost, DeliveryInDays, FreeDeliveryFrom, MinOrder, Prepayment,
	Installment, ReturnPeriodInDays, Warranty, OutsideZonesNote *string
	CommercePolicyDocuments *[]uuid.UUID
}

// MCPUpsertPolicies patches the policies singleton.
func (s *Store) MCPUpsertPolicies(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, ch PoliciesChanges, expectedVersion *int64, provenance MCPProvenance) (UpsertResult, error) {
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentPolicy(ctx, db, orgID, b)
		if err != nil {
			return err
		}
		if ch.DeliveryCost != nil {
			cur.DeliveryCost = *ch.DeliveryCost
		}
		if ch.DeliveryInDays != nil {
			cur.DeliveryInDays = *ch.DeliveryInDays
		}
		if ch.FreeDeliveryFrom != nil {
			cur.FreeDeliveryFrom = *ch.FreeDeliveryFrom
		}
		if ch.MinOrder != nil {
			cur.MinOrder = *ch.MinOrder
		}
		if ch.Prepayment != nil {
			cur.Prepayment = *ch.Prepayment
		}
		if ch.Installment != nil {
			cur.Installment = *ch.Installment
		}
		if ch.ReturnPeriodInDays != nil {
			cur.ReturnPeriodInDays = *ch.ReturnPeriodInDays
		}
		if ch.Warranty != nil {
			cur.Warranty = *ch.Warranty
		}
		if ch.OutsideZonesNote != nil {
			cur.OutsideZonesNote = *ch.OutsideZonesNote
		}
		refs := applyPoliciesMedia(&cur, PoliciesMedia{CommercePolicyDocuments: ch.CommercePolicyDocuments})
		if err := validateMediaRefs(ctx, db, orgID, refs); err != nil {
			return err
		}
		if err := s.recordProvenance(ctx, db, orgID, KBTypePolicies, NaturalKeyMain, provenance); err != nil {
			return err
		}
		b.upsertPolicy(cur)
		b.removeDelete(deleteKindFor(KBTypePolicies), NaturalKeyMain) // upserting cancels a pending delete — see MCPUpsertTopic's comment
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{Type: KBTypePolicies, Key: NaturalKeyMain, DraftVersion: newVersion}, nil
}

// ---------------------------------------------------------------------------
// TariffInfo (singleton, patch-only) — organization-wide tariff facts that
// belong to no single plan (aiprompt.TariffInfo's doc comment).
// ---------------------------------------------------------------------------

// TariffInfoChanges is kb_tariff_info_upsert's `changes`. AdditionalFacts is
// nil-means-unchanged, non-nil (even empty) means replace, the only field
// this singleton carries.
type TariffInfoChanges struct {
	AdditionalFacts *[]aiprompt.AdditionalFact
}

// MCPUpsertTariffInfo patches the tariff_info singleton.
func (s *Store) MCPUpsertTariffInfo(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, ch TariffInfoChanges, expectedVersion *int64, provenance MCPProvenance) (UpsertResult, error) {
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentTariffInfo(ctx, db, orgID, b)
		if err != nil {
			return err
		}
		if ch.AdditionalFacts != nil {
			cur.AdditionalFacts = *ch.AdditionalFacts
		}
		if err := validateTariffInfoFacts(cur); err != nil {
			return err
		}
		if err := s.recordProvenance(ctx, db, orgID, KBTypeTariffInfo, NaturalKeyMain, provenance); err != nil {
			return err
		}
		b.upsertTariffInfo(cur)
		b.removeDelete(deleteKindFor(KBTypeTariffInfo), NaturalKeyMain) // upserting cancels a pending delete — see MCPUpsertTopic's comment
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{Type: KBTypeTariffInfo, Key: NaturalKeyMain, DraftVersion: newVersion}, nil
}

// ---------------------------------------------------------------------------
// Delivery zone
// ---------------------------------------------------------------------------

// DeliveryZoneChanges is kb_delivery_zone_upsert's `changes`. No media
// columns exist on this table in v1 (plan/DECISIONS.md).
type DeliveryZoneChanges struct {
	Name, ZoneLevel, ParentRef, DeliveryCost, DeliveryInDays, Notes, SalesStatus *string
	DeliveryAvailable                                                            *bool // required on create
}

// MCPUpsertDeliveryZone creates or patches a delivery zone
// (kb_delivery_zone_upsert). Zone-world validation (delivery_available
// requires cost+days, parent_ref must resolve with no cycle, policy
// exclusivity) is enforced at Approve time (draft.go), matching
// zoneGateReasons — the same discipline as every existing zone write path
// (zones.go); it does not need to be repeated here for a merely-staged draft
// entry that a human still reviews before it can affect the live KB.
func (s *Store) MCPUpsertDeliveryZone(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, ref string, ch DeliveryZoneChanges, expectedVersion *int64, provenance MCPProvenance) (UpsertResult, error) {
	var result UpsertResult
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		index, err := s.identityIndex(ctx, db, orgID, []string{KBTypeDeliveryZone}, "both", "")
		if err != nil {
			return err
		}
		title := ""
		if ch.Name != nil {
			title = *ch.Name
		}
		key, creating, err := resolveUpsertKey(KBTypeDeliveryZone, ref, title, index)
		if err != nil {
			return err
		}
		cur, err := s.currentZone(ctx, db, orgID, key, b)
		if err != nil {
			return err
		}
		cur.Ref = key
		if ch.Name != nil {
			cur.Name = *ch.Name
		}
		if ch.ZoneLevel != nil {
			if err := validateEnum("zone_level", *ch.ZoneLevel, "city", "region", "country"); err != nil {
				return err
			}
			cur.ZoneLevel = *ch.ZoneLevel
		}
		if ch.ParentRef != nil {
			cur.ParentRef = *ch.ParentRef
		}
		if creating && ch.DeliveryAvailable == nil {
			return &ErrRequiredFieldMissing{Field: "delivery_available"}
		}
		if ch.DeliveryAvailable != nil {
			cur.DeliveryAvailable = *ch.DeliveryAvailable
		}
		if ch.DeliveryCost != nil {
			cur.DeliveryCost = *ch.DeliveryCost
		}
		if ch.DeliveryInDays != nil {
			cur.DeliveryInDays = *ch.DeliveryInDays
		}
		if ch.Notes != nil {
			cur.Notes = *ch.Notes
		}
		if ch.SalesStatus != nil {
			if err := validateEnum("sales_status", *ch.SalesStatus, "active", "inactive"); err != nil {
				return err
			}
			cur.SalesStatus = *ch.SalesStatus
		}
		if creating && strings.TrimSpace(cur.Name) == "" {
			return &ErrRequiredFieldMissing{Field: "name"}
		}
		if creating && strings.TrimSpace(cur.ZoneLevel) == "" {
			return &ErrRequiredFieldMissing{Field: "zone_level"}
		}
		if err := s.recordProvenance(ctx, db, orgID, KBTypeDeliveryZone, key, provenance); err != nil {
			return err
		}
		b.upsertZone(cur)
		b.removeDelete(deleteKindFor(KBTypeDeliveryZone), key) // upserting cancels a pending delete — see MCPUpsertTopic's comment
		result = UpsertResult{Type: KBTypeDeliveryZone, Key: key, Created: creating}
		return nil
	})
	if err != nil {
		return UpsertResult{}, err
	}
	result.DraftVersion = newVersion
	return result, nil
}

// ---------------------------------------------------------------------------
// kb_delete
// ---------------------------------------------------------------------------

// DeleteResult mirrors UpsertResult's version-reporting shape.
type DeleteResult struct {
	Type         string `json:"type"`
	Key          string `json:"key"`
	DraftVersion int64  `json:"draft_version"`
}

// ErrCannotDelete means the type/key combination cannot be marked for
// deletion (an unknown type, a wrong key for a singleton, or the assistant
// singleton — the KB is never publishable without one).
type ErrCannotDelete struct{ Reason string }

func (e *ErrCannotDelete) Error() string { return "kbstore: cannot delete: " + e.Reason }

// MCPDelete implements kb_delete's transition matrix (plan/mcp.md §5):
//
//   - nonexistent key (neither live nor draft)   → ErrCannotDelete.
//   - draft-only (never published)               → cancel the pending
//     creation: clear the draft entry, stage NO delete marker (there is no
//     live row for one to ever apply to — see below for why a marker here
//     would be a permanent phantom).
//   - live (with or without a shadowing draft
//     edit)                                      → drop the draft edit, if
//     any, and stage a delete marker for Approve to apply to the live row.
//   - repeat delete on an already-marked live
//     entity                                     → idempotent no-op (falls
//     out of the same "live" branch; addDelete already de-dupes).
//
// "The backend validates the type/key and blocks a delete that would make
// the publishable KB invalid" — validated here for the assistant singleton
// (never deletable), an unknown type/key, and now existence; the zone/
// policy exclusivity invariant is re-validated at Approve time, same as
// every zone write.
func (s *Store) MCPDelete(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, kbType, key string, expectedVersion *int64) (DeleteResult, error) {
	switch kbType {
	case KBTypeAssistant:
		return DeleteResult{}, &ErrCannotDelete{Reason: "the assistant configuration cannot be deleted"}
	case KBTypeContacts, KBTypePolicies, KBTypeTariffInfo:
		if key != NaturalKeyMain {
			return DeleteResult{}, &ErrCannotDelete{Reason: fmt.Sprintf("key must be %q for %s", NaturalKeyMain, kbType)}
		}
	case KBTypeTopic, KBTypeProduct, KBTypeTariff, KBTypeDeliveryZone:
		if strings.TrimSpace(key) == "" {
			return DeleteResult{}, &ErrCannotDelete{Reason: "key is required"}
		}
	default:
		return DeleteResult{}, &ErrCannotDelete{Reason: fmt.Sprintf("unknown type %q", kbType)}
	}

	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		index, err := s.identityIndex(ctx, db, orgID, []string{kbType}, "both", "")
		if err != nil {
			return err
		}
		var found *Identity
		for i := range index {
			if index[i].Key == key {
				found = &index[i]
				break
			}
		}
		if found == nil {
			return &ErrCannotDelete{Reason: fmt.Sprintf("no %s with key %q exists (live or draft)", kbType, key)}
		}

		switch kbType {
		case KBTypeTopic:
			b.removeTopic(key)
		case KBTypeProduct:
			b.removeProduct(key)
		case KBTypeTariff:
			b.removeTariff(key)
		case KBTypeDeliveryZone:
			b.removeZone(key)
		case KBTypeContacts:
			b.removeContact()
		case KBTypePolicies:
			b.removePolicy()
		case KBTypeTariffInfo:
			b.removeTariffInfo()
		}
		if found.ExistsInLive {
			b.addDelete(deleteKindFor(kbType), key)
		}
		return nil
	})
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{Type: kbType, Key: key, DraftVersion: newVersion}, nil
}
