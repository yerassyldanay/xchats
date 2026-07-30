package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
// appropriate initial view").
func toolResult(summary string, structured any, view string) map[string]any {
	out := map[string]any{
		"content": []map[string]any{{"type": "text", "text": summary}},
		"isError": false,
		"_meta":   merge(widgetMeta(), map[string]any{"xchats/widgetView": view}),
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
		return resultResponse(req.ID, toolError(fmt.Sprintf("this connection was not granted the %q scope required by %s", scope, params.Name)))
	}

	orgID := principal.OrganizationID
	result, herr := s.callTool(ctx, orgID, params.Name, args)
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
}

// callTool routes to the specific per-tool implementation. The returned
// error is a PARSE/PROTOCOL problem (malformed arguments) — an invalid
// params JSON-RPC error; every KB-domain outcome (conflict, ambiguity,
// validation) is folded into the returned map via mapKBError instead, so it
// reaches the model as ordinary tool output.
func (s *Server) callTool(ctx context.Context, orgID uuid.UUID, name string, args map[string]json.RawMessage) (map[string]any, error) {
	switch name {
	case toolKBAssistantUpsert:
		return s.handleAssistantUpsert(ctx, orgID, args)
	case toolKBTopicUpsert:
		return s.handleTopicUpsert(ctx, orgID, args)
	case toolKBProductUpsert:
		return s.handleProductUpsert(ctx, orgID, args)
	case toolKBTariffUpsert:
		return s.handleTariffUpsert(ctx, orgID, args)
	case toolKBContactsUpsert:
		return s.handleContactsUpsert(ctx, orgID, args)
	case toolKBPoliciesUpsert:
		return s.handlePoliciesUpsert(ctx, orgID, args)
	case toolKBDeliveryZoneUpsert:
		return s.handleDeliveryZoneUpsert(ctx, orgID, args)
	case toolKBRead:
		return s.handleKBRead(ctx, orgID, args)
	case toolKBDelete:
		return s.handleKBDelete(ctx, orgID, args)
	case toolKBSummary:
		return s.handleKBSummary(ctx, orgID, args)
	case toolKBInfo:
		return s.handleKBInfo(ctx, orgID, args)
	case toolKBMediaUpload:
		return s.handleKBMediaUpload(ctx, orgID, args)
	default:
		return toolError("unknown tool: " + name), nil
	}
}

// --- typed upserts ----------------------------------------------------

func (s *Server) handleAssistantUpsert(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
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
	res, kerr := s.Deps.KB.MCPUpsertAssistant(ctx, orgID, ch, expected)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(fmt.Sprintf("Assistant configuration updated in draft (draft_version=%d).", res.DraftVersion), res, "draft"), nil
}

func (s *Server) handleTopicUpsert(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
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
	if ch.NarrationAudioFiles, err = optMaterialIDs(changes, "narration_audio_files"); err != nil {
		return nil, err
	}
	if ch.ReferenceDocuments, err = optMaterialIDs(changes, "reference_documents"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertTopic(ctx, orgID, stringField(args, "slug"), ch, expected, provenanceString(args))
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(upsertSummary("topic", res), res, "draft"), nil
}

func (s *Server) handleProductUpsert(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
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
	if ch.AudioDescriptionFiles, err = optMaterialIDs(changes, "audio_description_files"); err != nil {
		return nil, err
	}
	if ch.CertificateDocuments, err = optMaterialIDs(changes, "certificate_documents"); err != nil {
		return nil, err
	}
	if ch.ManualDocuments, err = optMaterialIDs(changes, "manual_documents"); err != nil {
		return nil, err
	}
	if ch.GuaranteeDocuments, err = optMaterialIDs(changes, "guarantee_documents"); err != nil {
		return nil, err
	}
	if ch.SpecificationDocuments, err = optMaterialIDs(changes, "specification_documents"); err != nil {
		return nil, err
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPUpsertProduct(ctx, orgID, stringField(args, "ref"), ch, expected, provenanceString(args))
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(upsertSummary("product", res), res, "draft"), nil
}

func (s *Server) handleTariffUpsert(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
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
	res, kerr := s.Deps.KB.MCPUpsertTariff(ctx, orgID, stringField(args, "ref"), ch, expected, provenanceString(args))
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(upsertSummary("tariff", res), res, "draft"), nil
}

func (s *Server) handleContactsUpsert(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
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
	res, kerr := s.Deps.KB.MCPUpsertContacts(ctx, orgID, ch, expected, provenanceString(args))
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(fmt.Sprintf("Contacts updated in draft (draft_version=%d).", res.DraftVersion), res, "draft"), nil
}

func (s *Server) handlePoliciesUpsert(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
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
	res, kerr := s.Deps.KB.MCPUpsertPolicies(ctx, orgID, ch, expected, provenanceString(args))
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(fmt.Sprintf("Policies updated in draft (draft_version=%d).", res.DraftVersion), res, "draft"), nil
}

func (s *Server) handleDeliveryZoneUpsert(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return nil, fmt.Errorf("changes: %w", err)
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
	res, kerr := s.Deps.KB.MCPUpsertDeliveryZone(ctx, orgID, stringField(args, "ref"), ch, expected, provenanceString(args))
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(upsertSummary("delivery zone", res), res, "draft"), nil
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

func (s *Server) handleKBRead(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	types := stringSliceField(args, "types")
	if err := validateTypes(types); err != nil {
		return nil, err
	}
	source := stringField(args, "source")
	if source == "" {
		source = "both"
	}
	page, err := s.Deps.KB.ReadRecords(ctx, orgID, types, source, stringField(args, "key"), stringField(args, "query"), intField(args, "limit"), stringField(args, "cursor"))
	if err != nil {
		return nil, err
	}
	return toolResult(fmt.Sprintf("%d record(s).", len(page.Items)), page, "record"), nil
}

func (s *Server) handleKBDelete(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	kbType := stringField(args, "type")
	key := stringField(args, "key")
	if kbType == "" || key == "" {
		return nil, fmt.Errorf("type and key are required")
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}
	res, kerr := s.Deps.KB.MCPDelete(ctx, orgID, kbType, key, expected)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	return toolResult(fmt.Sprintf("%s %q marked for deletion in draft (draft_version=%d). It is removed from the live KB only after human review and publish.",
		capitalize(kbType), key, res.DraftVersion), res, "draft"), nil
}

func (s *Server) handleKBSummary(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	types := stringSliceField(args, "types")
	if err := validateTypes(types); err != nil {
		return nil, err
	}
	index, err := s.Deps.KB.IdentityIndex(ctx, orgID, types)
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
	return toolResult(fmt.Sprintf("%d record(s) (draft_version=%d).", len(items), version), page, "all"), nil
}

func (s *Server) handleKBInfo(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	return toolResult(serverInstructions, map[string]any{
		"types":             kbstore.AllKBTypes,
		"natural_key_main":  kbstore.NaturalKeyMain,
		"media_field_kinds": mediaFieldKindsInfo(),
	}, "all"), nil
}

func mediaFieldKindsInfo() map[string]string {
	out := map[string]string{}
	for _, f := range []string{
		"featured_image", "gallery_images", "pricing_images", "illustration_images",
		"contact_card_image", "location_map_image", "demo_videos", "explainer_videos",
		"narration_audio_files", "audio_description_files", "reference_documents",
		"certificate_documents", "manual_documents", "guarantee_documents",
		"specification_documents", "terms_documents", "company_legal_documents",
		"commerce_policy_documents",
	} {
		out[f] = kbstore.MediaFieldKind(f)
	}
	return out
}

func validateTypes(types []string) error {
	valid := map[string]bool{}
	for _, t := range kbstore.AllKBTypes {
		valid[t] = true
	}
	for _, t := range types {
		if !valid[t] {
			return fmt.Errorf("unknown type %q", t)
		}
	}
	return nil
}
