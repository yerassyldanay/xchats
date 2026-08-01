package mcpserver

// Tool is one MCP tool definition (the tools/list entry shape).
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	// SecuritySchemes declares this tool's OAuth scope requirement (set by
	// Tools() from the SAME requiredScope map dispatchToolsCall enforces, so
	// it can never drift from actual enforcement) — lets a capable host
	// reason about auth/consent up front instead of discovering a scope
	// failure by trial.
	//
	// A LIST, not an OpenAPI-style {schemeName: {...}} object. This was
	// originally guessed as an object and ChatGPT rejected the whole
	// tools/list response with a schema error naming the type it wants:
	//
	//	list[tagged-union[NoAuthSecurityScheme, OAuthSecurityScheme]]
	//
	// i.e. an array whose entries are discriminated on "type". Getting this
	// wrong is not a soft degradation — the connector fails to install.
	SecuritySchemes []map[string]any `json:"securitySchemes,omitempty"`
	// Annotations are the standard MCP tool-annotation hints (readOnlyHint,
	// destructiveHint, idempotentHint, ...) — see readOnlyAnnotations /
	// destructiveAnnotations / notIdempotentAnnotations below for exactly
	// which tools set which, and why.
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
	// OutputSchema documents the structuredContent shape a widget or a
	// schema-validating host can bind to (plan Task 9) — every tool
	// declares one, since every one of them (including kb_delete, via
	// kbstore.DeleteResult) sets structuredContent through handlers.go's
	// toolResult.
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	// Meta carries the widget hint on tools whose result the KB Manager
	// resource can render (plan/mcp.md §5: "kb_read, kb_summary, kb_info,
	// and mutation results may attach the shared KB Manager resource").
	// Keyed defensively under BOTH the ChatGPT Apps SDK convention
	// (openai/outputTemplate) and the standard MCP Apps shape (a nested
	// "ui" object, not the flat "ui/resourceUri" an earlier draft used),
	// since which one a given host actually honors is exactly what
	// plan/mcp.md §9's MCP-Inspector/ChatGPT/Claude interop pass is for —
	// see resources.go's package doc for the full caveat.
	Meta map[string]any `json:"_meta,omitempty"`
}

// ToolAnnotations are the standard MCP tool-annotation hints. Pointers (not
// plain bool) so idempotentHint:false serializes explicitly — omitting the
// field entirely would mean "unknown", not "confirmed not idempotent", and
// those are different signals to a host deciding whether to prompt before
// a retry.
type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

// readOnlyAnnotations marks kb_read/kb_summary/kb_info: they never write.
func readOnlyAnnotations() *ToolAnnotations {
	return &ToolAnnotations{ReadOnlyHint: boolPtr(true)}
}

// destructiveAnnotations marks kb_delete: it stages removal of a record
// (behind human review before publish, but still the closest thing this
// contract has to a destructive action).
func destructiveAnnotations() *ToolAnnotations {
	return &ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: boolPtr(false)}
}

// notIdempotentAnnotations marks the seven typed upserts: writeDraftBlobVersioned
// unconditionally bumps base_version and rewrites updated_at (and updated_by)
// on every call, so calling the same upsert twice is NOT a no-op at the
// storage layer even when the resulting content is identical. No-op
// detection is out of scope (see plan Task 9) — this hint stays false, never
// true, until that changes.
func notIdempotentAnnotations() *ToolAnnotations {
	return &ToolAnnotations{IdempotentHint: boolPtr(false)}
}

// oauthSecurityScheme is the securitySchemes value for a tool gated on scope:
// a single-element list holding the OAuthSecurityScheme variant ("type":
// "oauth2" is the union discriminator) naming the scope this tool requires.
// See Tool.SecuritySchemes for why this is a list rather than a keyed object.
func oauthSecurityScheme(scope string) []map[string]any {
	return []map[string]any{
		{"type": "oauth2", "scopes": []string{scope}},
	}
}

func widgetMeta() map[string]any {
	return map[string]any{
		"openai/outputTemplate": widgetResourceURI,
		"ui":                    map[string]any{"resourceUri": widgetResourceURI},
	}
}

// appOnlyMeta marks kb_media_upload: the standard MCP Apps visibility hint
// for a tool only the widget itself should ever invoke (plan/mcp.md §5 —
// "The model never calls this tool itself and never sees file bytes"). No
// resourceUri: its result isn't rendered through the KB Manager resource,
// it's an upload target the widget PUTs bytes to.
func appOnlyMeta() map[string]any {
	return map[string]any{"ui": map[string]any{"visibility": []string{"app"}}}
}

func obj(properties map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

// clearableStr is str for a `changes` field that goes through optString
// (args.go): omit leaves it unchanged, an explicit JSON null clears it to ""
// (the live tables' universal "no value" sentinel — see optString's own doc
// comment), a string sets it. Declaring the real ["string","null"] type,
// instead of bare "string", makes that tri-state contract visible to the
// calling model instead of leaving it to guess whether null is accepted.
// Reserved for plain free-text changes fields — never an identifier param
// (slug/ref/key/query/...), an enum (sales_status/pricing_type/zone_level:
// none of these has a sensible "blank" enum value), or a boolean/integer
// (optBool/optInt reject null outright; see their own doc comments).
func clearableStr(desc string) map[string]any {
	return map[string]any{"type": []string{"string", "null"}, "description": desc + " (or null to clear)"}
}
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func enumStr(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func strArray(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

// materialID is a nullable single media reference: null explicitly clears
// it, a string is a kb_media_upload material_id, omitted leaves it unchanged.
func materialID(desc string) map[string]any {
	return map[string]any{"type": []string{"string", "null"}, "description": desc + " (a kb_media_upload material_id, or null to clear)"}
}

func materialIDs(desc string) map[string]any {
	return strArray(desc + " (kb_media_upload material_id values, in send order)")
}

// changesObject wraps one tool's editable-fields schema into the shared
// `changes` parameter shape every typed upsert tool uses.
func changesObject(fields map[string]any) map[string]any {
	return map[string]any{
		"type": "object", "description": "Fields to create or patch. Omitted fields stay unchanged; explicit null clears a clearable field.",
		"properties": fields,
	}
}

// upsertCommon is appended to every typed upsert tool's top-level
// properties (plan/mcp.md "Common write behavior").
func upsertCommon() map[string]any {
	return map[string]any{
		"expected_draft_version": integer("Optimistic-concurrency token from a prior kb_summary/kb_read/upsert result. Omit on the first write to a record."),
		"provenance": obj(map[string]any{
			"source_url":   str("URL this content was derived from, if any."),
			"material_ids": materialIDs("Materials this content was derived from, if any (for audit/authoring trail, not attachment)."),
		}),
	}
}

// upsertOutputSchema documents kbstore.UpsertResult — the shared
// structuredContent shape all seven typed upsert tools return (including
// kb_assistant_upsert/kb_contacts_upsert/kb_policies_upsert, whose message
// text only mentions draft_version but whose result is the same struct).
func upsertOutputSchema() map[string]any {
	return obj(map[string]any{
		"type":          str("The KB type written."),
		"key":           str("The record's natural key (ref/slug/\"main\")."),
		"created":       boolean("true if this call created the record, false if it updated an existing one."),
		"draft_version": integer("The draft's new base_version — pass as expected_draft_version on the next write to this org's draft."),
	}, "type", "key", "created", "draft_version")
}

// deleteOutputSchema documents kbstore.DeleteResult.
func deleteOutputSchema() map[string]any {
	return obj(map[string]any{
		"type":          str("The KB type marked for deletion."),
		"key":           str("The record's natural key."),
		"draft_version": integer("The draft's new base_version."),
	}, "type", "key", "draft_version")
}

// readPageOutputSchema documents kbstore.ReadPage.
func readPageOutputSchema() map[string]any {
	return obj(map[string]any{
		"items": map[string]any{
			"type": "array",
			"items": obj(map[string]any{
				"type":   str("The record's KB type."),
				"source": enumStr("Which table this record came from.", "live", "draft"),
				"data":   map[string]any{"type": "object", "description": "The record's full canonical fields — shape varies by type."},
			}, "type", "source", "data"),
			"description": "The matched records.",
		},
		"next_cursor": str("Pass to the next kb_read call to continue; absent when this is the last page."),
	}, "items")
}

// summaryPageOutputSchema documents handleKBSummary's inline page shape.
func summaryPageOutputSchema() map[string]any {
	return obj(map[string]any{
		"draft_version": integer("The draft's current base_version."),
		"items": map[string]any{
			"type": "array",
			"items": obj(map[string]any{
				"type":            str("The record's KB type."),
				"key":             str("The record's natural key."),
				"title":           str("Display title/name."),
				"exists_in_live":  boolean("Whether a published version of this record exists."),
				"exists_in_draft": boolean("Whether a pending draft change exists for this record."),
				"state":           enumStr("Review state.", "published", "new", "changed", "to_delete"),
			}, "type", "key", "title", "exists_in_live", "exists_in_draft", "state"),
			"description": "The matched records' identity summary (no full content — call kb_read for that).",
		},
		"next_cursor": str("Pass to the next kb_summary call to continue; absent when this is the last page."),
	}, "draft_version", "items")
}

// kbInfoOutputSchema documents handleKBInfo's structuredContent.
func kbInfoOutputSchema() map[string]any {
	return obj(map[string]any{
		"types":            strArray("Every KB type this connector supports."),
		"natural_key_main": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "KB types whose natural key is the literal string \"main\" (singletons)."},
		"media_attachment_fields": map[string]any{
			"type": "object",
			"additionalProperties": map[string]any{
				"type": "array",
				"items": obj(map[string]any{
					"field":    str("The media column's name."),
					"kind":     enumStr("The broad MIME category this field accepts.", "image", "video", "audio", "document"),
					"multiple": boolean("True when the field holds a list and an attach appends; false when it holds one reference and an attach replaces."),
				}, "field", "kind", "multiple"),
			},
			"description": "Attachable media fields grouped by KB type — what kb_media_attach accepts. Types with no media columns are absent.",
		},
		"media_field_kinds": map[string]any{"type": "object", "description": "Maps each media field name to its kind (image/video/audio/document). Superseded by media_attachment_fields, which is type-scoped and excludes derived columns."},
		"frontend_base_url": str("The Xchats app's own origin, for a fallback plain /playground link when no per-call review URL is available."),
	}, "types", "natural_key_main", "media_attachment_fields", "media_field_kinds", "frontend_base_url")
}

// mediaUploadOutputSchema documents handleKBMediaUpload's structuredContent.
func mediaUploadOutputSchema() map[string]any {
	return obj(map[string]any{
		"material_id":       str("The staged material's id — reference it from a kb_*_upsert media field once uploaded."),
		"upload_url":        str("PUT the file bytes here."),
		"upload_method":     str("Always \"PUT\"."),
		"upload_headers":    map[string]any{"type": "object", "description": "Headers to send with the PUT (at minimum Content-Type)."},
		"expires_at":        str("RFC 3339 timestamp; the upload_url stops working after this."),
		"max_size_bytes":    integer("The declared size this upload was staged for — the PUT is rejected above it."),
		"processing_status": str("The material's current processing status."),
	}, "material_id", "upload_url", "upload_method", "upload_headers", "expires_at", "max_size_bytes", "processing_status")
}

func merge(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// Tools is the closed, ordered 13-tool contract (plan/mcp.md §5 plus
// kb_media_attach).
func Tools() []Tool {
	tools := []Tool{
		assistantUpsertTool(),
		topicUpsertTool(),
		productUpsertTool(),
		tariffUpsertTool(),
		contactsUpsertTool(),
		policiesUpsertTool(),
		deliveryZoneUpsertTool(),
		kbReadTool(),
		kbDeleteTool(),
		kbSummaryTool(),
		kbInfoTool(),
		kbMediaUploadTool(),
		kbMediaAttachTool(),
	}
	// SecuritySchemes is derived from requiredScope (handlers.go) — the SAME
	// map dispatchToolsCall enforces against — rather than repeated as a
	// literal on each tool function, so declaration and enforcement can never
	// silently drift apart.
	for i := range tools {
		if scope, ok := requiredScope[tools[i].Name]; ok {
			tools[i].SecuritySchemes = oauthSecurityScheme(scope)
		}
	}
	return tools
}

func assistantUpsertTool() Tool {
	return Tool{
		Name:        "kb_assistant_upsert",
		Description: "Create or patch the assistant configuration singleton (persona, mission, guardrails, language policy, reply length) in the draft. There is exactly one assistant record per organization, key \"main\".",
		InputSchema: obj(merge(map[string]any{
			"changes": changesObject(map[string]any{
				"persona":         clearableStr("Russian role and voice of the assistant."),
				"mission":         clearableStr("Russian business objective for replies."),
				"guardrails":      clearableStr("Russian behavior and safety rules."),
				"language_policy": clearableStr("Russian rule for reply language."),
				"reply_max_words": integer("Maximum suggested reply length."),
			}),
		}, upsertCommon()), "changes"),
		Meta:         widgetMeta(),
		Annotations:  notIdempotentAnnotations(),
		OutputSchema: upsertOutputSchema(),
	}
}

func topicUpsertTool() Tool {
	return Tool{
		Name:        "kb_topic_upsert",
		Description: "Create or patch a topic (explanatory knowledge, e.g. how to order, warranty policy). Key is `slug`. Provide slug to update an existing topic or create with that exact slug; omit slug to create with a slug derived from the title (after duplicate checks).",
		InputSchema: obj(merge(map[string]any{
			"slug": str("Existing topic's slug to update, or a new slug to create with. Omit to derive a slug from changes.title."),
			"changes": changesObject(map[string]any{
				"title":                 clearableStr("Russian topic title. Required when creating."),
				"body_md":               clearableStr("Approved Russian Markdown knowledge — pure prose, no {{...}} tokens or literal prices."),
				"featured_image":        materialID("Single main topic image."),
				"illustration_images":   materialIDs("Supporting topic illustrations."),
				"explainer_videos":      materialIDs("Videos that explain the topic."),
				"narration_audio_files": materialIDs("Customer-sendable spoken explanations."),
				"reference_documents":   materialIDs("Documents supporting the topic."),
			}),
		}, upsertCommon()), "changes"),
		Meta:         widgetMeta(),
		Annotations:  notIdempotentAnnotations(),
		OutputSchema: upsertOutputSchema(),
	}
}

func productUpsertTool() Tool {
	return Tool{
		Name:        "kb_product_upsert",
		Description: "Create or patch a sellable product. Key is `ref`. Provide ref to update an existing product or create with that exact ref; omit ref to derive one from the name (after duplicate checks). `in_stock` is required when creating.",
		InputSchema: obj(merge(map[string]any{
			"ref": str("Existing product's ref to update, or a new ref to create with. Omit to derive a ref from changes.name."),
			"changes": changesObject(map[string]any{
				"name":                    clearableStr("Russian product name. Required when creating."),
				"price":                   clearableStr("Exact approved price, including currency/range formatting."),
				"description":             clearableStr("Trusted Russian product description."),
				"category":                clearableStr("Product category."),
				"in_stock":                boolean("Stock state. Required when creating."),
				"sales_status":            enumStr("active or inactive.", "active", "inactive"),
				"featured_image":          materialID("Single main product image."),
				"gallery_images":          materialIDs("Product gallery images."),
				"demo_videos":             materialIDs("Product demonstration videos."),
				"audio_description_files": materialIDs("Spoken product descriptions."),
				"certificate_documents":   materialIDs("Product certificates."),
				"manual_documents":        materialIDs("Product user/installation manuals."),
				"guarantee_documents":     materialIDs("Product guarantee documents."),
				"specification_documents": materialIDs("Technical product specifications."),
			}),
		}, upsertCommon()), "changes"),
		Meta:         widgetMeta(),
		Annotations:  notIdempotentAnnotations(),
		OutputSchema: upsertOutputSchema(),
	}
}

func tariffUpsertTool() Tool {
	return Tool{
		Name:        "kb_tariff_upsert",
		Description: "Create or patch a pricing plan/tariff. Key is `ref`. Provide ref to update an existing tariff or create with that exact ref; omit ref to derive one from the name (after duplicate checks). `pricing_type` is required when creating.",
		InputSchema: obj(merge(map[string]any{
			"ref": str("Existing tariff's ref to update, or a new ref to create with. Omit to derive a ref from changes.name."),
			"changes": changesObject(map[string]any{
				"name":             clearableStr("Russian tariff name. Required when creating."),
				"price":            clearableStr("Exact approved tariff price."),
				"limit_text":       clearableStr("Trusted Russian explanation of usage limits."),
				"fee":              clearableStr("Exact approved fee, when applicable."),
				"summary":          clearableStr("Short trusted Russian tariff summary."),
				"pricing_type":     enumStr("Required when creating.", "fixed", "percentage", "tiered", "hybrid"),
				"advantages":       clearableStr("Trusted Russian advantages."),
				"disadvantages":    clearableStr("Trusted Russian limitations/disadvantages."),
				"sales_status":     enumStr("active or inactive.", "active", "inactive"),
				"featured_image":   materialID("Single main tariff image."),
				"pricing_images":   materialIDs("Price cards and pricing illustrations."),
				"explainer_videos": materialIDs("Videos explaining the tariff."),
				"terms_documents":  materialIDs("Tariff terms and conditions documents."),
			}),
		}, upsertCommon()), "changes"),
		Meta:         widgetMeta(),
		Annotations:  notIdempotentAnnotations(),
		OutputSchema: upsertOutputSchema(),
	}
}

func contactsUpsertTool() Tool {
	return Tool{
		Name:        "kb_contacts_upsert",
		Description: "Create or patch the organization contacts singleton. Key is \"main\".",
		InputSchema: obj(merge(map[string]any{
			"changes": changesObject(map[string]any{
				"whatsapp":                clearableStr("Exact approved WhatsApp contact."),
				"email":                   clearableStr("Exact approved support email."),
				"address":                 clearableStr("Trusted Russian business address."),
				"legal_information":       clearableStr("Trusted Russian legal/company details."),
				"callback_time":           clearableStr("Trusted Russian callback expectation."),
				"working_hours":           clearableStr("Exact approved working-hours display."),
				"phone":                   clearableStr("Exact approved support phone."),
				"website":                 clearableStr("Exact approved website."),
				"instagram":               clearableStr("Exact approved Instagram account."),
				"contact_card_image":      materialID("Single contact-card image."),
				"location_map_image":      materialID("Single location/map image."),
				"company_legal_documents": materialIDs("Customer-sendable company/legal documents."),
			}),
		}, upsertCommon()), "changes"),
		Meta:         widgetMeta(),
		Annotations:  notIdempotentAnnotations(),
		OutputSchema: upsertOutputSchema(),
	}
}

func policiesUpsertTool() Tool {
	return Tool{
		Name:        "kb_policies_upsert",
		Description: "Create or patch the commerce policies singleton (delivery, payment, returns, warranty). Key is \"main\". When per-zone delivery zones exist, delivery_cost/delivery_in_days must stay blank and outside_zones_note is required — use kb_delivery_zone_upsert for per-zone pricing instead.",
		InputSchema: obj(merge(map[string]any{
			"changes": changesObject(map[string]any{
				"delivery_cost":             clearableStr("Exact approved flat delivery price. Must stay blank while delivery zones exist."),
				"delivery_in_days":          clearableStr("Exact delivery duration/range in days. Must stay blank while delivery zones exist."),
				"free_delivery_from":        clearableStr("Exact order value that qualifies for free delivery."),
				"min_order":                 clearableStr("Exact minimum order value."),
				"prepayment":                clearableStr("Trusted Russian prepayment policy."),
				"installment":               clearableStr("Trusted Russian installment policy."),
				"return_period_in_days":     clearableStr("Exact return duration in days."),
				"warranty":                  clearableStr("Trusted Russian warranty policy."),
				"outside_zones_note":        clearableStr("Exact approved refusal for a direction matching no delivery zone. Required while delivery zones exist."),
				"commerce_policy_documents": materialIDs("Customer-sendable commerce-policy documents."),
			}),
		}, upsertCommon()), "changes"),
		Meta:         widgetMeta(),
		Annotations:  notIdempotentAnnotations(),
		OutputSchema: upsertOutputSchema(),
	}
}

func deliveryZoneUpsertTool() Tool {
	return Tool{
		Name:        "kb_delivery_zone_upsert",
		Description: "Create or patch a delivery-coverage zone. Key is `ref`. Provide ref to update an existing zone or create with that exact ref; omit ref to derive one from the name (after duplicate checks). `delivery_available` is required when creating. A zone with delivery_available=true must have delivery_cost and delivery_in_days both set; one with delivery_available=false must have both blank.",
		InputSchema: obj(merge(map[string]any{
			"ref": str("Existing zone's ref to update, or a new ref to create with. Omit to derive a ref from changes.name."),
			"changes": changesObject(map[string]any{
				"name":               clearableStr("Russian zone display name. Required when creating."),
				"zone_level":         enumStr("Required when creating.", "city", "region", "country"),
				"parent_ref":         clearableStr("This organization's own delivery-zone ref this zone nests under (city → region → country). Empty for a top-level zone."),
				"delivery_available": boolean("Whether this zone is served at all. Required when creating."),
				"delivery_cost":      clearableStr("Exact approved delivery price for this zone. Required iff delivery_available."),
				"delivery_in_days":   clearableStr("Exact delivery duration/range in days for this zone. Required iff delivery_available."),
				"notes":              clearableStr("Trusted Russian prose about this zone."),
				"sales_status":       enumStr("active or inactive.", "active", "inactive"),
			}),
		}, upsertCommon()), "changes"),
		Meta:         widgetMeta(),
		Annotations:  notIdempotentAnnotations(),
		OutputSchema: upsertOutputSchema(),
	}
}

func kbReadTool() Tool {
	return Tool{
		Name:        "kb_read",
		Description: "Read complete KB records. Use after kb_summary identifies a possible match, to confirm the exact record before upserting. source=both shows the live and draft records SEPARATELY (never a merged view) when both exist.",
		InputSchema: obj(map[string]any{
			"types":  strArray("KB types to read: assistant, topic, product, tariff, contacts, policies, delivery_zone. Omit for all types."),
			"source": enumStr("Defaults to both.", "live", "draft", "both"),
			"key":    str("Exact ref, slug, or \"main\" to read one record."),
			"query":  str("Case-insensitive substring match against title/name."),
			"limit":  integer("Max records to return. Default 50, maximum 100."),
			"cursor": str("Pagination cursor from a previous kb_read result."),
		}),
		Meta:         widgetMeta(),
		Annotations:  readOnlyAnnotations(),
		OutputSchema: readPageOutputSchema(),
	}
}

func kbDeleteTool() Tool {
	return Tool{
		Name:        "kb_delete",
		Description: "Add a delete marker to the draft for one record, by natural key. The record is removed from the live KB only after human review and publish. The assistant singleton cannot be deleted.",
		InputSchema: obj(map[string]any{
			"type":                   enumStr("KB type.", "topic", "product", "tariff", "contacts", "policies", "delivery_zone"),
			"key":                    str("ref, slug, or \"main\"."),
			"expected_draft_version": integer("Optimistic-concurrency token from a prior kb_summary/kb_read/upsert result."),
		}, "type", "key"),
		Annotations:  destructiveAnnotations(),
		OutputSchema: deleteOutputSchema(),
	}
}

func kbSummaryTool() Tool {
	return Tool{
		Name:        "kb_summary",
		Description: "Return a compact identity index (type, key, title, presence in live/draft, state) for duplicate detection before writing and for widget lists. Omits prices and other full content — call kb_read for that.",
		InputSchema: obj(map[string]any{
			"types":  strArray("KB types to include. Omit for all types."),
			"source": enumStr("Defaults to both.", "live", "draft", "both"),
			"query":  str("Case-insensitive substring match against title/name."),
			"limit":  integer("Max records to return. Default 50, maximum 100."),
			"cursor": str("Pagination cursor from a previous kb_summary result."),
		}),
		Meta:         widgetMeta(),
		Annotations:  readOnlyAnnotations(),
		OutputSchema: summaryPageOutputSchema(),
	}
}

func kbInfoTool() Tool {
	return Tool{
		Name:         "kb_info",
		Description:  "Explain the KB types, their natural keys, required/supported fields, the duplicate-check workflow, the draft-only mutation rule, media-field meanings, and how to review and publish. The server's own instructions already summarize this — call kb_info only if you need more detail.",
		InputSchema:  obj(map[string]any{}),
		Meta:         widgetMeta(),
		Annotations:  readOnlyAnnotations(),
		OutputSchema: kbInfoOutputSchema(),
	}
}

func kbMediaUploadTool() Tool {
	return Tool{
		Name:        "kb_media_upload",
		Description: "App-only, widget-invoked tool: stages a pending material and returns a short-lived signed upload target the widget PUTs bytes to directly. The model never calls this tool itself and never sees file bytes.",
		InputSchema: obj(map[string]any{
			"filename":        str("Original filename."),
			"mime_type":       str("Declared MIME type."),
			"size_bytes":      integer("Declared size in bytes."),
			"sha256_checksum": str("Declared SHA-256 checksum, if known."),
			"target": obj(map[string]any{
				"type":  enumStr("KB type the media will be attached to.", "topic", "product", "tariff", "contacts", "policies"),
				"key":   str("The record's ref/slug/\"main\", if known."),
				"field": str("The semantic media field this upload is intended for, e.g. gallery_images, illustration_images — must be one of kb_info.media_attachment_fields' entries for type."),
			}, "type", "field"),
		}, "filename", "mime_type", "size_bytes"),
		Meta:         appOnlyMeta(),
		OutputSchema: mediaUploadOutputSchema(),
	}
}

func kbMediaAttachTool() Tool {
	return Tool{
		Name:        "kb_media_attach",
		Description: "App-only, widget-invoked tool: attaches an already-uploaded material (kb_media_upload's material_id, with a completed PUT) to one media field of an EXISTING draft or live record. Never creates a record — an unknown key is rejected, not created. A plural field appends (no duplicates); a singular field replaces. Writes only the draft. The model never calls this tool itself.",
		InputSchema: obj(map[string]any{
			"material_id":            str("A kb_media_upload material_id whose PUT has completed."),
			"type":                   enumStr("KB type owning the field.", "topic", "product", "tariff", "contacts", "policies"),
			"key":                    str("The record's ref/slug/\"main\". The record must already exist (live or draft) and must not be staged for deletion."),
			"field":                  str("The media field to attach to — must be one of kb_info.media_attachment_fields' entries for type. Never featured_image, which is not an attachment target."),
			"expected_draft_version": integer("Optional optimistic-concurrency token from a prior kb_summary/kb_read/upsert result."),
		}, "material_id", "type", "key", "field"),
		Meta:         appOnlyMeta(),
		OutputSchema: mediaAttachOutputSchema(),
	}
}

// mediaAttachOutputSchema documents handleKBMediaAttach's structuredContent
// (kbstore.MediaAttachResult).
func mediaAttachOutputSchema() map[string]any {
	return obj(map[string]any{
		"type":            str("The KB type attached to."),
		"key":             str("The record's natural key (ref/slug/\"main\")."),
		"field":           str("The media field attached to."),
		"material_id":     str("The attached material_id."),
		"draft_version":   integer("The draft's new base_version — pass as expected_draft_version on the next write to this org's draft."),
		"already_present": boolean("true if this exact material was already attached to this field (a no-op for a plural field, a same-value replace for a singular one)."),
	}, "type", "key", "field", "material_id", "draft_version", "already_present")
}
