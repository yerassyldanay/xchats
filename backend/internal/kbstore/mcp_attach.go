package kbstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MediaAttachInput is kb_media_attach's arguments.
type MediaAttachInput struct {
	MaterialID uuid.UUID
	Type       string
	Key        string
	Field      string
}

// MediaAttachResult is kb_media_attach's result.
type MediaAttachResult struct {
	Type           string `json:"type"`
	Key            string `json:"key"`
	Field          string `json:"field"`
	MaterialID     string `json:"material_id"`
	DraftVersion   int64  `json:"draft_version"`
	AlreadyPresent bool   `json:"already_present"`
}

// ErrAttachTarget means {type, key, field} cannot receive an attachment: the
// field is not a valid attachment target for type (this is what rejects
// featured_image — a real, readable media column, but never an attachment
// target, see mediaAttachmentFields' doc comment), the record does not exist
// in either live or draft, or it is staged for deletion. Distinct from
// ErrMediaReference, which is reserved for problems with the MATERIAL itself
// (organization, upload completeness, visibility, MIME).
type ErrAttachTarget struct {
	Type, Key, Field, Reason string
}

func (e *ErrAttachTarget) Error() string {
	loc := e.Type
	if e.Key != "" {
		loc = fmt.Sprintf("%s %q", e.Type, e.Key)
	}
	if e.Field == "" {
		return fmt.Sprintf("kbstore: cannot attach to %s: %s", loc, e.Reason)
	}
	return fmt.Sprintf("kbstore: cannot attach to %s.%s: %s", loc, e.Field, e.Reason)
}

// MCPAttachMedia implements kb_media_attach: attaches an already-uploaded,
// already-validated material to one field of an EXISTING record, atomically.
// A plural field appends unless the material is already present (no
// duplicates); a singular field is replaced outright. The record is never
// created — attaching to a key that matches nothing is ErrAttachTarget, not
// a fallthrough to create (that fallthrough is exactly what produced the
// "pricing_type is required" bug the widget's own attach flow used to hit).
//
// Runs inside writeDraftBlobVersioned's single locked transaction like every
// other MCP write, so the existence check, the append/replace, and the
// write are one atomic step with respect to a concurrent writer on the same
// organization — there is nothing for expectedVersion to protect against
// that the lock doesn't already cover, but a caller may still pass one (e.g.
// a widget that just displayed draft_version) for the usual ErrStale
// feedback loop on a genuinely stale read.
func (s *Store) MCPAttachMedia(ctx context.Context, orgID, userID uuid.UUID, in MediaAttachInput, expectedVersion *int64) (MediaAttachResult, error) {
	if _, ok := LookupMediaAttachmentField(in.Type, in.Field); !ok {
		return MediaAttachResult{}, &ErrAttachTarget{Type: in.Type, Key: in.Key, Field: in.Field,
			Reason: "not a valid attachment field for this type"}
	}

	result := MediaAttachResult{Type: in.Type, Field: in.Field, MaterialID: in.MaterialID.String()}
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, userID, func(db dbtx, b *DraftBlob) error {
		key := in.Key
		switch in.Type {
		case KBTypeContacts, KBTypePolicies:
			// True singletons: unlike every other type, "no row yet" is the
			// LEGITIMATE first write (MCPUpsertContacts/MCPUpsertPolicies
			// rely on the exact same currentContact/currentPolicy blank
			// scaffold on ErrNoRows). There is nothing per-org to look up in
			// identityIndex until one has been written, so an org that has
			// never touched contacts/policies must still be able to attach
			// its first contact_card_image — skip the existence check
			// rather than reject a record that has every right not to
			// exist yet.
			key = NaturalKeyMain
		default:
			index, ierr := s.identityIndex(ctx, db, orgID, []string{in.Type}, "both", "")
			if ierr != nil {
				return ierr
			}
			found := false
			for _, id := range index {
				if id.Key != in.Key {
					continue
				}
				found = true
				if id.ToDelete {
					return &ErrAttachTarget{Type: in.Type, Key: in.Key, Field: in.Field, Reason: "record is staged for deletion"}
				}
				break
			}
			if !found {
				return &ErrAttachTarget{Type: in.Type, Key: in.Key, Field: in.Field, Reason: "record does not exist in live or draft"}
			}
		}
		result.Key = key

		if err := validateMediaRef(ctx, db, orgID, in.Field, in.MaterialID); err != nil {
			return err
		}

		switch in.Type {
		case KBTypeTopic:
			cur, err := s.currentTopic(ctx, db, orgID, key, b)
			if err != nil {
				return err
			}
			target, ok := topicMediaFieldTarget(&cur, in.Field)
			if !ok {
				return fmt.Errorf("kbstore: unhandled attachment field %q for topic", in.Field)
			}
			*target, result.AlreadyPresent = appendUnique(*target, in.MaterialID)
			b.upsertTopic(cur)
			b.removeDelete(deleteKindFor(KBTypeTopic), key)
		case KBTypeProduct:
			cur, err := s.currentProduct(ctx, db, orgID, key, b)
			if err != nil {
				return err
			}
			target, ok := productMediaFieldTarget(&cur, in.Field)
			if !ok {
				return fmt.Errorf("kbstore: unhandled attachment field %q for product", in.Field)
			}
			*target, result.AlreadyPresent = appendUnique(*target, in.MaterialID)
			b.upsertProduct(cur)
			b.removeDelete(deleteKindFor(KBTypeProduct), key)
		case KBTypeTariff:
			cur, err := s.currentTariff(ctx, db, orgID, key, b)
			if err != nil {
				return err
			}
			target, ok := tariffMediaFieldTarget(&cur, in.Field)
			if !ok {
				return fmt.Errorf("kbstore: unhandled attachment field %q for tariff", in.Field)
			}
			*target, result.AlreadyPresent = appendUnique(*target, in.MaterialID)
			b.upsertTariff(cur)
			b.removeDelete(deleteKindFor(KBTypeTariff), key)
		case KBTypeContacts:
			cur, err := s.currentContact(ctx, db, orgID, b)
			if err != nil {
				return err
			}
			switch in.Field {
			case "contact_card_image":
				result.AlreadyPresent = cur.ContactCardImage != nil && *cur.ContactCardImage == in.MaterialID
				cur.ContactCardImage = &in.MaterialID
			case "location_map_image":
				result.AlreadyPresent = cur.LocationMapImage != nil && *cur.LocationMapImage == in.MaterialID
				cur.LocationMapImage = &in.MaterialID
			case "company_legal_documents":
				cur.CompanyLegalDocuments, result.AlreadyPresent = appendUnique(cur.CompanyLegalDocuments, in.MaterialID)
			default:
				return fmt.Errorf("kbstore: unhandled attachment field %q for contacts", in.Field)
			}
			b.upsertContact(cur)
			b.removeDelete(deleteKindFor(KBTypeContacts), NaturalKeyMain)
		case KBTypePolicies:
			cur, err := s.currentPolicy(ctx, db, orgID, b)
			if err != nil {
				return err
			}
			if in.Field != "commerce_policy_documents" {
				return fmt.Errorf("kbstore: unhandled attachment field %q for policies", in.Field)
			}
			cur.CommercePolicyDocuments, result.AlreadyPresent = appendUnique(cur.CommercePolicyDocuments, in.MaterialID)
			b.upsertPolicy(cur)
			b.removeDelete(deleteKindFor(KBTypePolicies), NaturalKeyMain)
		default:
			return fmt.Errorf("kbstore: unhandled attachment type %q", in.Type)
		}
		return nil
	})
	if err != nil {
		return MediaAttachResult{}, err
	}
	result.DraftVersion = newVersion
	return result, nil
}

// topicMediaFieldTarget / productMediaFieldTarget / tariffMediaFieldTarget
// resolve a registry field name to the specific plural slice on an
// already-loaded draft row — the same "current row, patch one column"
// shape MCPUpsertTopic/Product/Tariff's applyImageSet closures use, reused
// here for a single named field instead of a whole change-set.
func topicMediaFieldTarget(t *DraftTopic, field string) (*[]uuid.UUID, bool) {
	switch field {
	case "illustration_images":
		return &t.IllustrationImages, true
	case "explainer_videos":
		return &t.ExplainerVideos, true
	case "reference_documents":
		return &t.ReferenceDocuments, true
	default:
		return nil, false
	}
}

func productMediaFieldTarget(p *DraftProduct, field string) (*[]uuid.UUID, bool) {
	switch field {
	case "gallery_images":
		return &p.GalleryImages, true
	case "demo_videos":
		return &p.DemoVideos, true
	case "certificate_documents":
		return &p.CertificateDocuments, true
	case "guarantee_documents":
		return &p.GuaranteeDocuments, true
	default:
		return nil, false
	}
}

func tariffMediaFieldTarget(t *DraftTariff, field string) (*[]uuid.UUID, bool) {
	switch field {
	case "pricing_images":
		return &t.PricingImages, true
	case "explainer_videos":
		return &t.ExplainerVideos, true
	case "terms_documents":
		return &t.TermsDocuments, true
	default:
		return nil, false
	}
}

// appendUnique appends id to ids unless it is already present, returning the
// (possibly unchanged) slice and whether id was already there.
func appendUnique(ids []uuid.UUID, id uuid.UUID) ([]uuid.UUID, bool) {
	for _, existing := range ids {
		if existing == id {
			return ids, true
		}
	}
	return append(ids, id), false
}
