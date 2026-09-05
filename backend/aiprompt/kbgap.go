package aiprompt

import (
	"encoding/json"
)

// KBGapDiagnostic is the optional, v7+ structured escalation diagnostic
// (the response contract's "kb_gap" property): WHY the knowledge base could
// not answer, as a closed reason code plus the specific KB entity/field
// involved. It is telemetry only, exactly like EscalationReason/Confidence
// on Response: absent or invalid never affects reply_text/escalate, and a
// caller must check Escalate itself before acting on it — ValidateResponseV7
// decodes and sanitizes whatever the model sent regardless of Escalate's
// value, the same way escalation_reason is decoded unconditionally today.
//
// Source distinguishes a model-authored diagnostic (decoded from a
// validated model response) from one the engine stamps itself on a hard
// failure (response/service.go's holdingDraft) — it is never decoded from
// model JSON (see decodeKBGap): the model has no "source" property to set
// in the schema at all, so there is nothing for it to invent here.
type KBGapDiagnostic struct {
	ReasonCode       string   `json:"reason_code"`
	TargetEntityType string   `json:"target_entity_type,omitempty"`
	TargetEntityRef  string   `json:"target_entity_ref,omitempty"`
	MissingFields    []string `json:"missing_fields,omitempty"`
	Source           string   `json:"source,omitempty"`
}

// Reason codes are the closed KB-gap vocabulary (DECISIONS.md-equivalent for
// this contract): the first four are genuine knowledge-gap categories and
// are the only ones the default "KB gaps" report aggregates; the remaining
// four are operational/non-gap escalations that must stay distinguishable
// from an actual content gap, never blended into the same count.
//
// There is deliberately no "unavailable_entity": a known-but-currently-
// unavailable product (rule 1a) and a known delivery zone with no coverage
// (rule 2) are BOTH answered directly, never escalated — and kb_gap is only
// ever filled in on escalation — so no reachable, non-conflicting trigger
// for it exists in this contract's design. Removed rather than shipped dead
// (see the v7 frames' rule 9a decision table).
const (
	KBGapReasonMissingEntity     = "missing_entity"
	KBGapReasonMissingField      = "missing_field"
	KBGapReasonAmbiguousEntity   = "ambiguous_entity"
	KBGapReasonConflictingKBData = "conflicting_kb_data"

	KBGapReasonUnsupportedRequest = "unsupported_request"
	KBGapReasonHumanRequested     = "human_requested"
	KBGapReasonEngineError        = "engine_error"
	KBGapReasonOther              = "other"
)

// KBGapSourceModel and KBGapSourceEngine are the only valid values of
// KBGapDiagnostic.Source: whether the diagnostic was decoded from a
// validated model response, or stamped by the engine itself on a hard
// failure that never reached the model contract at all.
const (
	KBGapSourceModel  = "model"
	KBGapSourceEngine = "engine"
)

// DefaultReportReasonCodes are the reason codes the default "KB gaps" report
// aggregates — genuine content gaps only. KBGapReasonHumanRequested and
// KBGapReasonEngineError in particular must stay excluded and distinguishable:
// an operational error or a customer's own request to speak to a human is
// not a knowledge-base gap.
func DefaultReportReasonCodes() []string {
	return []string{
		KBGapReasonMissingEntity, KBGapReasonMissingField, KBGapReasonAmbiguousEntity,
		KBGapReasonConflictingKBData,
	}
}

// AllKBGapReasonCodes is the complete closed vocabulary, in vocabulary
// order — the persistence layer validates a stored reason_code (whichever
// source stamped it) against this full set, not just the model-facing subset.
func AllKBGapReasonCodes() []string {
	return []string{
		KBGapReasonMissingEntity, KBGapReasonMissingField, KBGapReasonAmbiguousEntity,
		KBGapReasonConflictingKBData,
		KBGapReasonUnsupportedRequest, KBGapReasonHumanRequested, KBGapReasonEngineError, KBGapReasonOther,
	}
}

// modelKBGapReasonCodes is the subset the MODEL may set (via the v7+ JSON
// schema's enum) — KBGapReasonEngineError is deliberately excluded: it is
// stamped only by response/service.go's holdingDraft on a hard engine
// failure, a path that never calls the model at all, so the model has no
// legitimate occasion to claim it. sanitizeKBGap enforces this same set
// server-side, fail-closed, independent of what the schema merely asked for.
var modelKBGapReasonCodes = map[string]bool{
	KBGapReasonMissingEntity:      true,
	KBGapReasonMissingField:       true,
	KBGapReasonAmbiguousEntity:    true,
	KBGapReasonConflictingKBData:  true,
	KBGapReasonUnsupportedRequest: true,
	KBGapReasonHumanRequested:     true,
	KBGapReasonOther:              true,
}

// Target-entity types are a closed vocabulary distinct from both of
// registry.go's "table" namings (fact tables are singular, media tables are
// plural) — kb_gap identifies a KB ROW for telemetry/operator purposes, not
// a fact or media column, so it gets its own stable names.
const (
	KBGapEntityProduct      = "product"
	KBGapEntityTariff       = "tariff"
	KBGapEntityTariffInfo   = "tariff_info"
	KBGapEntityContact      = "contact"
	KBGapEntityPolicy       = "policy"
	KBGapEntityDeliveryZone = "delivery_zone"
	KBGapEntityTopic        = "topic"
)

// AllKBGapEntityTypes is the closed target_entity_type vocabulary, in
// schema/documentation order.
func AllKBGapEntityTypes() []string {
	return []string{
		KBGapEntityProduct, KBGapEntityTariff, KBGapEntityTariffInfo,
		KBGapEntityContact, KBGapEntityPolicy, KBGapEntityDeliveryZone, KBGapEntityTopic,
	}
}

func setOf(items ...string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, s := range items {
		out[s] = true
	}
	return out
}

// kbGapFieldAllowlist is the type-specific closed vocabulary for
// missing_fields: the entity's own known structural columns (registry.go's
// exact-value fact columns plus the migration-0017 prose columns a customer
// can plainly ask about), never the entity's dynamic, seller-authored
// virtual facts — a virtual fact that does not exist has no fixed name the
// model could legitimately cite. tariff_info and topic have no fixed
// columns of their own (only virtual facts / free prose), so any
// missing_fields entry for them is always dropped by sanitizeKBGap.
var kbGapFieldAllowlist = map[string]map[string]bool{
	KBGapEntityProduct: setOf("price", "availability_status", "availability_note", "brand",
		"description", "advantages", "disadvantages", "best_for", "not_for",
		"installation_terms", "warranty_terms"),
	KBGapEntityTariff: setOf("price", "fee", "pricing_type", "summary", "limit_text",
		"advantages", "disadvantages", "best_for", "not_for"),
	KBGapEntityTariffInfo: setOf(),
	KBGapEntityContact:    setOf("phone", "whatsapp", "email", "website", "instagram", "working_hours"),
	KBGapEntityPolicy: setOf("delivery_cost", "delivery_in_days", "free_delivery_from", "min_order",
		"return_period_in_days", "outside_zones_note", "warranty", "prepayment", "installment"),
	KBGapEntityDeliveryZone: setOf("delivery_cost", "delivery_in_days", "delivery_available"),
	KBGapEntityTopic:        setOf(),
}

// kbGapEntityExists validates a target ref against the LOADED KB (every row
// the organization owns), not the customer-visible catalog: an unavailable
// product is still a real KB entity an operator can act on (e.g. a
// missing_field diagnostic about a currently-unavailable product's
// installation_terms), so it must not be scrubbed as invented.
func kbGapEntityExists(kb *KB, entityType, ref string) bool {
	if kb == nil || ref == "" {
		return false
	}
	switch entityType {
	case KBGapEntityProduct:
		for i := range kb.Products {
			if kb.Products[i].Ref == ref {
				return true
			}
		}
	case KBGapEntityTariff:
		for i := range kb.Tariffs {
			if kb.Tariffs[i].Ref == ref {
				return true
			}
		}
	case KBGapEntityTariffInfo:
		return ref == SingletonRef && kb.TariffInfo != nil
	case KBGapEntityContact:
		return ref == SingletonRef && kb.Contacts != nil
	case KBGapEntityPolicy:
		return ref == SingletonRef && kb.Policies != nil
	case KBGapEntityDeliveryZone:
		for i := range kb.DeliveryZones {
			if kb.DeliveryZones[i].Ref == ref {
				return true
			}
		}
	case KBGapEntityTopic:
		for i := range kb.Topics {
			if kb.Topics[i].Slug == ref {
				return true
			}
		}
	}
	return false
}

// rawKBGap is kb_gap's model-facing wire shape — deliberately carries no
// "source" property: the model has nothing to set there (see
// KBGapDiagnostic's doc comment).
type rawKBGap struct {
	ReasonCode       string   `json:"reason_code"`
	TargetEntityType string   `json:"target_entity_type"`
	TargetEntityRef  string   `json:"target_entity_ref"`
	MissingFields    []string `json:"missing_fields"`
}

// decodeKBGap best-effort decodes the "kb_gap" property's raw bytes.
// Anything that does not even parse as an object (a string, a number,
// malformed JSON) yields nil, exactly like a mistyped escalation_reason
// leaves that field at its zero value elsewhere in this package — never an
// issue, never a reason to reject the reply.
func decodeKBGap(raw json.RawMessage) *KBGapDiagnostic {
	var r rawKBGap
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	if r.ReasonCode == "" && r.TargetEntityType == "" && r.TargetEntityRef == "" && len(r.MissingFields) == 0 {
		return nil
	}
	return &KBGapDiagnostic{
		ReasonCode:       r.ReasonCode,
		TargetEntityType: r.TargetEntityType,
		TargetEntityRef:  r.TargetEntityRef,
		MissingFields:    r.MissingFields,
		Source:           KBGapSourceModel,
	}
}

// sanitizeKBGap is the fail-closed gate between "whatever the model claimed"
// and "what this package will hand a caller as an authoritative diagnostic":
// an unrecognized reason code drops the whole diagnostic (never reinterpreted
// as KBGapReasonOther here — that fallback, when one is needed at all, is a
// PERSISTENCE-layer policy for guaranteeing one telemetry event per
// escalation, not something aiprompt invents on the model's behalf); a
// target ref that does not resolve against the loaded KB drops the target
// (and, with it, any missing_fields — a field cannot be "missing" from an
// entity that was never established as real); and any missing_fields entry
// outside that entity type's closed allowlist is dropped individually. The
// reason code alone, with no target, is still kept — it is useful telemetry
// on its own.
func sanitizeKBGap(gap *KBGapDiagnostic, kb *KB) *KBGapDiagnostic {
	if gap == nil || !modelKBGapReasonCodes[gap.ReasonCode] {
		return nil
	}
	out := &KBGapDiagnostic{ReasonCode: gap.ReasonCode, Source: gap.Source}
	if gap.TargetEntityType != "" && gap.TargetEntityRef != "" &&
		kbGapEntityExists(kb, gap.TargetEntityType, gap.TargetEntityRef) {
		out.TargetEntityType = gap.TargetEntityType
		out.TargetEntityRef = gap.TargetEntityRef
		if allowed := kbGapFieldAllowlist[gap.TargetEntityType]; allowed != nil {
			seen := make(map[string]bool, len(gap.MissingFields))
			for _, f := range gap.MissingFields {
				// The v7 schema has no uniqueItems constraint, so a
				// schema-valid model response can repeat an allowed field
				// (e.g. ["price","price"]); ai_kb_gap_missing_fields has
				// UNIQUE(event_id, field_name), and this diagnostic must
				// never be able to roll back the draft it rides in on.
				if allowed[f] && !seen[f] {
					seen[f] = true
					out.MissingFields = append(out.MissingFields, f)
				}
			}
		}
	}
	return out
}

// kbGapDescription documents the kb_gap property for both the rendered
// JSON Schema and this file's own contract.
const kbGapDescription = "Optional structured escalation diagnostic. Diagnostic only: never shown to the customer, never a reason to change reply_text or escalate. Fill it in only when escalate is true. Omit the whole object rather than guess any part of it."

// kbGapJSONSchema is the "kb_gap" property schema ResponseJSONSchemaV7 adds
// to the shared v6 schema (see that function).
func kbGapJSONSchema() map[string]any {
	reasonCodes := []string{
		KBGapReasonMissingEntity, KBGapReasonMissingField, KBGapReasonAmbiguousEntity,
		KBGapReasonConflictingKBData, KBGapReasonUnsupportedRequest,
		KBGapReasonHumanRequested, KBGapReasonOther,
	}
	return map[string]any{
		"type":                 "object",
		"description":          kbGapDescription,
		"additionalProperties": false,
		"required":             []string{"reason_code"},
		"properties": map[string]any{
			"reason_code": map[string]any{
				"type":        "string",
				"enum":        reasonCodes,
				"description": "Closed reason code for why the knowledge base could not answer.",
			},
			"target_entity_type": map[string]any{
				"type":        "string",
				"enum":        AllKBGapEntityTypes(),
				"description": "The kind of KB entity involved. Copy exactly — never invent.",
			},
			"target_entity_ref": map[string]any{
				"type":        "string",
				"description": "The exact ref/slug of the KB entity involved, copied exactly from the blocks below. Never invent one.",
			},
			"missing_fields": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "The specific field name(s) the customer asked about that are missing, written exactly as named in this prompt. Never invent a field name.",
			},
		},
	}
}
