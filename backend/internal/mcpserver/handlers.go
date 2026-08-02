package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

// toolResult builds a successful tools/call result: content is what the
// model reads, structuredContent is what a widget reads (plan/mcp.md §6:
// "A widget receives tool results and can call the same tools"), and Meta
// carries the shared KB Manager resource hint so a capable host can open it
// showing the right view (plan/mcp.md §5: "kb_read, kb_summary, kb_info, and
// mutation results may attach the shared KB Manager resource with the
// appropriate initial view") — plus, when SignReviewHandoff is wired
// (Task 15), a one-time organization-bound review URL under
// _meta["xchats/reviewUrl"], so the widget's "Review and publish in Xchats"
// link always lands on the SAME organization the calling tool operated on,
// not whichever org a bare /playground link would default to for a
// multi-organization user.
func (s *Server) toolResult(summary string, structured any, view string, orgID, userID uuid.UUID) map[string]any {
	meta := merge(widgetMeta(), map[string]any{"xchats/widgetView": view})
	if s.Deps.SignReviewHandoff != nil {
		if url, err := s.Deps.SignReviewHandoff(userID, orgID); err != nil {
			if s.Deps.Log != nil {
				s.Deps.Log.Warn("mcpserver: sign review handoff failed", "err", err)
			}
		} else {
			meta["xchats/reviewUrl"] = url
		}
	}
	out := map[string]any{
		"content": []map[string]any{{"type": "text", "text": summary}},
		"isError": false,
		"_meta":   meta,
	}
	if structured != nil {
		out["structuredContent"] = structured
	}
	return out
}

func toolError(message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": message}},
		"isError": true,
	}
}

// scopeErrorResult is a scope-failure tool result carrying a structured
// runtime challenge under _meta, in addition to the model-facing text — a
// capable host can read _meta["mcp/www_authenticate"] to know PROGRAMMATICALLY
// that re-authorization with an additional scope would resolve this, the same
// role an HTTP 401's WWW-Authenticate header plays for a browser client,
// rather than only a string the model must interpret. This exact field name
// is this ecosystem's current emerging convention for an in-band auth
// challenge — best-effort pending a real interop pass, same caveat as
// tools.go's securitySchemes/widget-hint fields.
func scopeErrorResult(toolName, scope string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("this connection was not granted the %q scope required by %s", scope, toolName)}},
		"isError": true,
		"_meta": map[string]any{
			"mcp/www_authenticate": map[string]any{
				"error":             "insufficient_scope",
				"error_description": fmt.Sprintf("%s requires the %q scope", toolName, scope),
				"scope":             scope,
			},
		},
	}
}

// mapKBError translates a kbstore-domain error into tool-result text the
// model can act on WITHOUT the call looking like a transport failure — a
// duplicate conflict or an ambiguous match is guidance, not an exception.
func mapKBError(err error) map[string]any {
	var missing *kbstore.ErrRequiredFieldMissing
	if errors.As(err, &missing) {
		return toolError(missing.Error())
	}
	var conflict *kbstore.ErrDuplicateConflict
	if errors.As(err, &conflict) {
		return toolError(fmt.Sprintf("%s Call kb_read(type=%q, key=%q) to inspect it, then retry kb_%s_upsert with that key to update it instead of creating a duplicate.",
			conflict.Error(), conflict.Type, conflict.ExistingKey, conflict.Type))
	}
	var ambiguous *kbstore.ErrAmbiguousMatch
	if errors.As(err, &ambiguous) {
		return toolError(ambiguous.Error() + " Ask the user which record they mean before creating a new one.")
	}
	var mediaErr *kbstore.ErrMediaReference
	if errors.As(err, &mediaErr) {
		return toolError(mediaErr.Error())
	}
	var attachErr *kbstore.ErrAttachTarget
	if errors.As(err, &attachErr) {
		return toolError(attachErr.Error())
	}
	var enumErr *kbstore.ErrInvalidEnumValue
	if errors.As(err, &enumErr) {
		return toolError(enumErr.Error())
	}
	var cannotDelete *kbstore.ErrCannotDelete
	if errors.As(err, &cannotDelete) {
		return toolError(cannotDelete.Error())
	}
	var gateErr *kbstore.GateError
	if errors.As(err, &gateErr) {
		return toolError("This change would leave the knowledge base invalid: " + gateErr.Error())
	}
	if errors.Is(err, kbstore.ErrStale) {
		return toolError("expected_draft_version is stale — the draft changed since you last read it. Call kb_summary or kb_read again and retry with the current draft_version.")
	}
	return toolError("internal error: " + err.Error())
}

// dispatchToolsCall parses the common {name, arguments} envelope, enforces
// the tool's required scope, and routes to the specific handler.
func (s *Server) dispatchToolsCall(ctx context.Context, principal mcpauth.Principal, req Request) Response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := unmarshalParams(req.Params, &params); err != nil {
		return errorResponse(req.ID, codeInvalidParams, err.Error())
	}
	args, err := rawObject(params.Arguments)
	if err != nil {
		return errorResponse(req.ID, codeInvalidParams, err.Error())
	}

	scope, ok := requiredScope[params.Name]
	if !ok {
		return errorResponse(req.ID, codeMethodNotFound, "unknown tool: "+params.Name)
	}
	// kb_media_upload is app-only (the widget invokes it, not the model) but
	// still travels over the same bearer token, so the same scope gate below
	// applies to it — there is no separate enforcement path.
	if !principal.HasScope(scope) {
		return resultResponse(req.ID, scopeErrorResult(params.Name, scope))
	}

	orgID := principal.OrganizationID
	result, herr := s.callTool(ctx, orgID, principal.UserID, params.Name, args)
	if herr != nil {
		return errorResponse(req.ID, codeInvalidParams, herr.Error())
	}
	return resultResponse(req.ID, result)
}

const (
	toolKBAssistantUpsert    = "kb_assistant_upsert"
	toolKBTopicUpsert        = "kb_topic_upsert"
	toolKBProductUpsert      = "kb_product_upsert"
	toolKBTariffUpsert       = "kb_tariff_upsert"
	toolKBContactsUpsert     = "kb_contacts_upsert"
	toolKBPoliciesUpsert     = "kb_policies_upsert"
	toolKBDeliveryZoneUpsert = "kb_delivery_zone_upsert"
	toolKBRead               = "kb_read"
	toolKBDelete             = "kb_delete"
	toolKBSummary            = "kb_summary"
	toolKBInfo               = "kb_info"
	toolKBMediaUpload        = "kb_media_upload"
	toolKBMediaAttach        = "kb_media_attach"
)

var requiredScope = map[string]string{
	toolKBAssistantUpsert:    mcpauth.ScopeKBDraftWrite,
	toolKBTopicUpsert:        mcpauth.ScopeKBDraftWrite,
	toolKBProductUpsert:      mcpauth.ScopeKBDraftWrite,
	toolKBTariffUpsert:       mcpauth.ScopeKBDraftWrite,
	toolKBContactsUpsert:     mcpauth.ScopeKBDraftWrite,
	toolKBPoliciesUpsert:     mcpauth.ScopeKBDraftWrite,
	toolKBDeliveryZoneUpsert: mcpauth.ScopeKBDraftWrite,
	toolKBRead:               mcpauth.ScopeKBRead,
	toolKBDelete:             mcpauth.ScopeKBDraftWrite,
	toolKBSummary:            mcpauth.ScopeKBRead,
	toolKBInfo:               mcpauth.ScopeKBRead,
	toolKBMediaUpload:        mcpauth.ScopeMediaWrite,
	// kb_media_attach WRITES THE DRAFT (appends/replaces a media field on an
	// existing record) — kb:draft:write, the same scope every kb_*_upsert
	// requires, not media:write (kb_media_upload's scope: staging bytes is a
	// different privilege than attaching an already-staged material to a KB
	// record).
	toolKBMediaAttach: mcpauth.ScopeKBDraftWrite,
}

// callTool routes to the specific per-tool implementation. The returned
// error is a PARSE/PROTOCOL problem (malformed arguments) — an invalid
// params JSON-RPC error; every KB-domain outcome (conflict, ambiguity,
// validation) is folded into the returned map via mapKBError instead, so it
// reaches the model as ordinary tool output.
func (s *Server) callTool(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, name string, args map[string]json.RawMessage) (map[string]any, error) {
	switch name {
	case toolKBAssistantUpsert:
		return s.handleAssistantUpsert(ctx, orgID, userID, args)
	case toolKBTopicUpsert:
		return s.handleTopicUpsert(ctx, orgID, userID, args)
	case toolKBProductUpsert:
		return s.handleProductUpsert(ctx, orgID, userID, args)
	case toolKBTariffUpsert:
		return s.handleTariffUpsert(ctx, orgID, userID, args)
	case toolKBContactsUpsert:
		return s.handleContactsUpsert(ctx, orgID, userID, args)
	case toolKBPoliciesUpsert:
		return s.handlePoliciesUpsert(ctx, orgID, userID, args)
	case toolKBDeliveryZoneUpsert:
		return s.handleDeliveryZoneUpsert(ctx, orgID, userID, args)
	case toolKBRead:
		return s.handleKBRead(ctx, orgID, userID, args)
	case toolKBDelete:
		return s.handleKBDelete(ctx, orgID, userID, args)
	case toolKBSummary:
		return s.handleKBSummary(ctx, orgID, userID, args)
	case toolKBInfo:
		return s.handleKBInfo(ctx, orgID, userID, args)
	case toolKBMediaUpload:
		return s.handleKBMediaUpload(ctx, orgID, userID, args)
	case toolKBMediaAttach:
		return s.handleKBMediaAttach(ctx, orgID, userID, args)
	default:
		return toolError("unknown tool: " + name), nil
	}
}

// --- typed upserts ----------------------------------------------------

func (s *Server) handleAssistantUpsert(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
	}
	if err := rejectUnknownFields(changes, "persona", "mission", "guardrails", "language_policy", "reply_max_words"); err != nil {
		return nil, err
	}
	ch := kbstore.AssistantChanges{}
	if ch.Persona, err = optString(changes, "persona"); err != nil {
		return nil, err
	}
	if ch.Mission, err = optString(changes, "mission"); err != nil {
		return nil, err
	}
	if ch.Guardrails, err = optString(changes, "guardrails"); err != nil {
		return nil, err
	}
	if ch.LanguagePolicy, err = optString(changes, "language_policy"); err != nil {
		return nil, err
	}
	if ch.ReplyMaxWords, err = optInt(changes, "reply_max_words"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertAssistant(ctx, orgID, userID, ch, expected)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(fmt.Sprintf("Assistant configuration updated in draft (draft_version=%d).", res.DraftVersion), res, "draft", orgID, userID), nil
}

func (s *Server) handleTopicUpsert(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
	}
	if err := rejectUnknownFields(changes, "title", "body_md", "featured_image", "illustration_images",
		"explainer_videos", "reference_documents"); err != nil {
		return nil, err
	}
	ch := kbstore.TopicChanges{}
	if ch.Title, err = optString(changes, "title"); err != nil {
		return nil, err
	}
	if ch.BodyMD, err = optString(changes, "body_md"); err != nil {
		return nil, err
	}
	if ch.FeaturedImage, err = optMaterialID(changes, "featured_image"); err != nil {
		return nil, err
	}
	if ch.IllustrationImages, err = optMaterialIDs(changes, "illustration_images"); err != nil {
		return nil, err
	}
	if ch.ExplainerVideos, err = optMaterialIDs(changes, "explainer_videos"); err != nil {
		return nil, err
	}
	if ch.ReferenceDocuments, err = optMaterialIDs(changes, "reference_documents"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	prov, err := parseProvenance(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertTopic(ctx, orgID, userID, stringField(args, "slug"), ch, expected, prov)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(upsertSummary("topic", res), res, "draft", orgID, userID), nil
}

func (s *Server) handleProductUpsert(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
	}
	if err := rejectUnknownFields(changes, "name", "price", "description", "category", "in_stock", "sales_status",
		"featured_image", "gallery_images", "demo_videos", "certificate_documents", "guarantee_documents"); err != nil {
		return nil, err
	}
	ch := kbstore.ProductChanges{}
	if ch.Name, err = optString(changes, "name"); err != nil {
		return nil, err
	}
	if ch.Price, err = optString(changes, "price"); err != nil {
		return nil, err
	}
	if ch.Description, err = optString(changes, "description"); err != nil {
		return nil, err
	}
	if ch.Category, err = optString(changes, "category"); err != nil {
		return nil, err
	}
	if ch.InStock, err = optBool(changes, "in_stock"); err != nil {
		return nil, err
	}
	if ch.SalesStatus, err = optString(changes, "sales_status"); err != nil {
		return nil, err
	}
	if ch.FeaturedImage, err = optMaterialID(changes, "featured_image"); err != nil {
		return nil, err
	}
	if ch.GalleryImages, err = optMaterialIDs(changes, "gallery_images"); err != nil {
		return nil, err
	}
	if ch.DemoVideos, err = optMaterialIDs(changes, "demo_videos"); err != nil {
		return nil, err
	}
	if ch.CertificateDocuments, err = optMaterialIDs(changes, "certificate_documents"); err != nil {
		return nil, err
	}
	if ch.GuaranteeDocuments, err = optMaterialIDs(changes, "guarantee_documents"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	prov, err := parseProvenance(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertProduct(ctx, orgID, userID, stringField(args, "ref"), ch, expected, prov)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(upsertSummary("product", res), res, "draft", orgID, userID), nil
}

func (s *Server) handleTariffUpsert(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
	}
	if err := rejectUnknownFields(changes, "name", "price", "limit_text", "fee", "summary", "pricing_type",
		"advantages", "disadvantages", "sales_status", "featured_image", "pricing_images", "explainer_videos",
		"terms_documents"); err != nil {
		return nil, err
	}
	ch := kbstore.TariffChanges{}
	if ch.Name, err = optString(changes, "name"); err != nil {
		return nil, err
	}
	if ch.Price, err = optString(changes, "price"); err != nil {
		return nil, err
	}
	if ch.LimitText, err = optString(changes, "limit_text"); err != nil {
		return nil, err
	}
	if ch.Fee, err = optString(changes, "fee"); err != nil {
		return nil, err
	}
	if ch.Summary, err = optString(changes, "summary"); err != nil {
		return nil, err
	}
	if ch.PricingType, err = optString(changes, "pricing_type"); err != nil {
		return nil, err
	}
	if ch.Advantages, err = optString(changes, "advantages"); err != nil {
		return nil, err
	}
	if ch.Disadvantages, err = optString(changes, "disadvantages"); err != nil {
		return nil, err
	}
	if ch.SalesStatus, err = optString(changes, "sales_status"); err != nil {
		return nil, err
	}
	if ch.FeaturedImage, err = optMaterialID(changes, "featured_image"); err != nil {
		return nil, err
	}
	if ch.PricingImages, err = optMaterialIDs(changes, "pricing_images"); err != nil {
		return nil, err
	}
	if ch.ExplainerVideos, err = optMaterialIDs(changes, "explainer_videos"); err != nil {
		return nil, err
	}
	if ch.TermsDocuments, err = optMaterialIDs(changes, "terms_documents"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	prov, err := parseProvenance(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertTariff(ctx, orgID, userID, stringField(args, "ref"), ch, expected, prov)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(upsertSummary("tariff", res), res, "draft", orgID, userID), nil
}

func (s *Server) handleContactsUpsert(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
	}
	if err := rejectUnknownFields(changes, "whatsapp", "email", "address", "legal_information", "callback_time",
		"working_hours", "phone", "website", "instagram", "contact_card_image", "location_map_image",
		"company_legal_documents"); err != nil {
		return nil, err
	}
	ch := kbstore.ContactsChanges{}
	for field, dst := range map[string]**string{
		"whatsapp": &ch.WhatsApp, "email": &ch.Email, "address": &ch.Address,
		"legal_information": &ch.LegalInformation, "callback_time": &ch.CallbackTime,
		"working_hours": &ch.WorkingHours, "phone": &ch.Phone, "website": &ch.Website, "instagram": &ch.Instagram,
	} {
		v, err := optString(changes, field)
		if err != nil {
			return nil, err
		}
		*dst = v
	}
	if ch.ContactCardImage, err = optMaterialID(changes, "contact_card_image"); err != nil {
		return nil, err
	}
	if ch.LocationMapImage, err = optMaterialID(changes, "location_map_image"); err != nil {
		return nil, err
	}
	if ch.CompanyLegalDocuments, err = optMaterialIDs(changes, "company_legal_documents"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	prov, err := parseProvenance(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertContacts(ctx, orgID, userID, ch, expected, prov)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(fmt.Sprintf("Contacts updated in draft (draft_version=%d).", res.DraftVersion), res, "draft", orgID, userID), nil
}

func (s *Server) handlePoliciesUpsert(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
	}
	if err := rejectUnknownFields(changes, "delivery_cost", "delivery_in_days", "free_delivery_from", "min_order",
		"prepayment", "installment", "return_period_in_days", "warranty", "outside_zones_note",
		"commerce_policy_documents"); err != nil {
		return nil, err
	}
	ch := kbstore.PoliciesChanges{}
	for field, dst := range map[string]**string{
		"delivery_cost": &ch.DeliveryCost, "delivery_in_days": &ch.DeliveryInDays,
		"free_delivery_from": &ch.FreeDeliveryFrom, "min_order": &ch.MinOrder,
		"prepayment": &ch.Prepayment, "installment": &ch.Installment,
		"return_period_in_days": &ch.ReturnPeriodInDays, "warranty": &ch.Warranty,
		"outside_zones_note": &ch.OutsideZonesNote,
	} {
		v, err := optString(changes, field)
		if err != nil {
			return nil, err
		}
		*dst = v
	}
	if ch.CommercePolicyDocuments, err = optMaterialIDs(changes, "commerce_policy_documents"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	prov, err := parseProvenance(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertPolicies(ctx, orgID, userID, ch, expected, prov)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(fmt.Sprintf("Policies updated in draft (draft_version=%d).", res.DraftVersion), res, "draft", orgID, userID), nil
}

func (s *Server) handleDeliveryZoneUpsert(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
	}
	if err := rejectUnknownFields(changes, "name", "zone_level", "parent_ref", "delivery_available",
		"delivery_cost", "delivery_in_days", "notes", "sales_status"); err != nil {
		return nil, err
	}
	ch := kbstore.DeliveryZoneChanges{}
	for field, dst := range map[string]**string{
		"name": &ch.Name, "zone_level": &ch.ZoneLevel, "parent_ref": &ch.ParentRef,
		"delivery_cost": &ch.DeliveryCost, "delivery_in_days": &ch.DeliveryInDays,
		"notes": &ch.Notes, "sales_status": &ch.SalesStatus,
	} {
		v, err := optString(changes, field)
		if err != nil {
			return nil, err
		}
		*dst = v
	}
	if ch.DeliveryAvailable, err = optBool(changes, "delivery_available"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	prov, err := parseProvenance(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertDeliveryZone(ctx, orgID, userID, stringField(args, "ref"), ch, expected, prov)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(upsertSummary("delivery zone", res), res, "draft", orgID, userID), nil
}

func upsertSummary(label string, res kbstore.UpsertResult) string {
	verb := "updated"
	if res.Created {
		verb = "created"
	}
	return fmt.Sprintf("%s %s in draft: key=%q (draft_version=%d).", capitalize(label), verb, res.Key, res.DraftVersion)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}

// --- shared tools -------------------------------------------------------

func (s *Server) handleKBRead(ctx context.Context, orgID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	types, typeErr := normalizeTypes(stringSliceField(args, "types"))
	if typeErr != nil {
		return typeErr, nil
	}
	source := stringField(args, "source")
	if source == "" {
		source = "both"
	}
	page, err := s.Deps.KB.ReadRecords(ctx, orgID, types, source, stringField(args, "key"), stringField(args, "query"), intField(args, "limit"), stringField(args, "cursor"))
	if err != nil {
		return nil, err
	}
	res := s.toolResult(fmt.Sprintf("%d record(s).", len(page.Items)), page, "record", orgID, userID)
	// Media metadata rides in _meta, NOT in structuredContent. kb_read's
	// declared outputSchema is additionalProperties:false (readPageOutputSchema
	// in tools.go), and clients cache tools/list — so a host validating
	// structuredContent against its cached copy would start rejecting kb_read
	// itself the moment a new key appeared. _meta is outside outputSchema by
	// spec, and this widget already reads _meta for xchats/reviewUrl.
	if meta, ok := res["_meta"].(map[string]any); ok {
		if index := s.mediaIndexFor(ctx, orgID, page); len(index) > 0 {
			meta["xchats/media"] = index
		}
	}
	return res, nil
}

func (s *Server) handleKBDelete(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	kbType := stringField(args, "type")
	key := stringField(args, "key")
	if kbType == "" || key == "" {
		return nil, fmt.Errorf("type and key are required")
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPDelete(ctx, orgID, userID, kbType, key, expected)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return s.toolResult(fmt.Sprintf("%s %q marked for deletion in draft (draft_version=%d). It is removed from the live KB only after human review and publish.",
		capitalize(kbType), key, res.DraftVersion), res, "draft", orgID, userID), nil
}

func (s *Server) handleKBSummary(ctx context.Context, orgID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	types, typeErr := normalizeTypes(stringSliceField(args, "types"))
	if typeErr != nil {
		return typeErr, nil
	}
	source := stringField(args, "source")
	if source == "" {
		source = "both"
	}
	index, err := s.Deps.KB.IdentityIndex(ctx, orgID, types, source, stringField(args, "query"))
	if err != nil {
		return nil, err
	}
	version, err := s.Deps.KB.DraftBaseVersion(ctx, orgID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(index))
	for _, id := range index {
		items = append(items, map[string]any{
			"type": id.Type, "key": id.Key, "title": id.Title,
			"exists_in_live": id.ExistsInLive, "exists_in_draft": id.ExistsInDraft, "state": id.State(),
		})
	}
	limit := intField(args, "limit")
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	// A plain offset cursor, matching ReadRecords — kb_summary shares the
	// same small-KB pagination premise (see mcp_read.go's ReadPage doc).
	offset := 0
	if c := stringField(args, "cursor"); c != "" {
		fmt.Sscanf(c, "%d", &offset)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	if offset > len(items) {
		offset = len(items)
	}
	page := map[string]any{"draft_version": version, "items": items[offset:end]}
	if end < len(items) {
		page["next_cursor"] = fmt.Sprintf("%d", end)
	}
	return s.toolResult(fmt.Sprintf("%d record(s) (draft_version=%d).", len(items), version), page, "all", orgID, userID), nil
}

func (s *Server) handleKBInfo(ctx context.Context, orgID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	return s.toolResult(serverInstructions, map[string]any{
		"types":            kbstore.AllKBTypes,
		"natural_key_main": kbstore.NaturalKeyMain,
		// media_attachment_fields is the authoritative, type-scoped registry
		// (which field on which type, its kind, and its cardinality) the KB
		// Manager widget builds its attach form from. media_field_kinds is
		// the older flat field→kind view, kept for compatibility with
		// anything built against it — it still lists featured_image, which
		// media_attachment_fields deliberately never does.
		"media_attachment_fields": kbstore.MediaAttachmentFieldsByType(),
		"media_field_kinds":       kbstore.MediaFieldKinds(),
		"frontend_base_url":       s.Deps.FrontendBaseURL,
	}, "all", orgID, userID), nil
}

// typeAliases maps the plural spelling plan/mcp.md's own worked examples use
// (kb_summary(types=["tariffs"], ...), kb_read(types=["products"])) to this
// server's canonical singular vocabulary. topic/product/tariff/
// delivery_zone are singular internally (kbstore.KBTypeTopic etc.);
// contacts/policies are already plural-shaped, so only the singular four
// need an alias.
var typeAliases = map[string]string{
	"topics": kbstore.KBTypeTopic, "products": kbstore.KBTypeProduct,
	"tariffs": kbstore.KBTypeTariff, "delivery_zones": kbstore.KBTypeDeliveryZone,
}

// normalizeTypes resolves plural aliases to their canonical singular form and
// validates the result against the closed type vocabulary. An unrecognized
// type comes back as an isError:true tool result (ok!=nil), never a
// JSON-RPC error — matching this file's own stated policy (dispatchToolsCall's
// doc comment) that a caller-correctable domain problem is guidance for the
// model, not a transport failure. types==nil (omitted, meaning "every type")
// passes through unchanged.
func normalizeTypes(types []string) (normalized []string, toolErr map[string]any) {
	if len(types) == 0 {
		return types, nil
	}
	valid := map[string]bool{}
	for _, t := range kbstore.AllKBTypes {
		valid[t] = true
	}
	out := make([]string, 0, len(types))
	for _, t := range types {
		if canon, ok := typeAliases[t]; ok {
			t = canon
		}
		if !valid[t] {
			return nil, toolError(fmt.Sprintf("unknown type %q — valid types: %s", t, strings.Join(kbstore.AllKBTypes, ", ")))
		}
		out = append(out, t)
	}
	return out, nil
}
