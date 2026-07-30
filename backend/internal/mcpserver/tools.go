package mcpserver

// Tool is one MCP tool definition (the tools/list entry shape).
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	// Meta carries the widget hint on tools whose result the KB Manager
	// resource can render (plan/mcp.md §5: "kb_read, kb_summary, kb_info,
	// and mutation results may attach the shared KB Manager resource").
	// Keyed defensively under BOTH the ChatGPT Apps SDK convention
	// (openai/outputTemplate) and a generic MCP-UI-style hint, since which
	// one a given host actually honors is exactly what plan/mcp.md §9's
	// MCP-Inspector/ChatGPT/Claude interop pass is for — see resources.go's
	// package doc for the full caveat.
	Meta map[string]any `json:"_meta,omitempty"`
}

func widgetMeta() map[string]any {
	return map[string]any{
		"openai/outputTemplate": widgetResourceURI,
		"ui/resourceUri":        widgetResourceURI,
	}
}

func obj(properties map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
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

// Tools is the closed, ordered 12-tool contract (plan/mcp.md §5).
func Tools() []Tool {
	return []Tool{
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
	}
}

func assistantUpsertTool() Tool {
	return Tool{
		Name:        "kb_assistant_upsert",
		Description: "Create or patch the assistant configuration singleton (persona, mission, guardrails, language policy, reply length) in the draft. There is exactly one assistant record per organization, key \"main\".",
		InputSchema: obj(merge(map[string]any{
			"changes": changesObject(map[string]any{
				"persona":         str("Russian role and voice of the assistant."),
				"mission":         str("Russian business objective for replies."),
				"guardrails":      str("Russian behavior and safety rules."),
				"language_policy": str("Russian rule for reply language."),
				"reply_max_words": integer("Maximum suggested reply length."),
			}),
		}, upsertCommon()), "changes"),
		Meta: widgetMeta(),
	}
}

func topicUpsertTool() Tool {
	return Tool{
		Name:        "kb_topic_upsert",
		Description: "Create or patch a topic (explanatory knowledge, e.g. how to order, warranty policy). Key is `slug`. Provide slug to update an existing topic or create with that exact slug; omit slug to create with a slug derived from the title (after duplicate checks).",
		InputSchema: obj(merge(map[string]any{
			"slug": str("Existing topic's slug to update, or a new slug to create with. Omit to derive a slug from changes.title."),
			"changes": changesObject(map[string]any{
				"title":                 str("Russian topic title. Required when creating."),
				"body_md":               str("Approved Russian Markdown knowledge — pure prose, no {{...}} tokens or literal prices."),
				"featured_image":        materialID("Single main topic image."),
				"illustration_images":   materialIDs("Supporting topic illustrations."),
				"explainer_videos":      materialIDs("Videos that explain the topic."),
				"narration_audio_files": materialIDs("Customer-sendable spoken explanations."),
				"reference_documents":   materialIDs("Documents supporting the topic."),
			}),
		}, upsertCommon()), "changes"),
		Meta: widgetMeta(),
	}
}

func productUpsertTool() Tool {
	return Tool{
		Name:        "kb_product_upsert",
		Description: "Create or patch a sellable product. Key is `ref`. Provide ref to update an existing product or create with that exact ref; omit ref to derive one from the name (after duplicate checks). `in_stock` is required when creating.",
		InputSchema: obj(merge(map[string]any{
			"ref": str("Existing product's ref to update, or a new ref to create with. Omit to derive a ref from changes.name."),
			"changes": changesObject(map[string]any{
				"name":                    str("Russian product name. Required when creating."),
				"price":                   str("Exact approved price, including currency/range formatting."),
				"description":             str("Trusted Russian product description."),
				"category":                str("Product category."),
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
		Meta: widgetMeta(),
	}
}

func tariffUpsertTool() Tool {
	return Tool{
		Name:        "kb_tariff_upsert",
		Description: "Create or patch a pricing plan/tariff. Key is `ref`. Provide ref to update an existing tariff or create with that exact ref; omit ref to derive one from the name (after duplicate checks). `pricing_type` is required when creating.",
		InputSchema: obj(merge(map[string]any{
			"ref": str("Existing tariff's ref to update, or a new ref to create with. Omit to derive a ref from changes.name."),
			"changes": changesObject(map[string]any{
				"name":             str("Russian tariff name. Required when creating."),
				"price":            str("Exact approved tariff price."),
				"limit_text":       str("Trusted Russian explanation of usage limits."),
				"fee":              str("Exact approved fee, when applicable."),
				"summary":          str("Short trusted Russian tariff summary."),
				"pricing_type":     enumStr("Required when creating.", "fixed", "percentage", "tiered", "hybrid"),
				"advantages":       str("Trusted Russian advantages."),
				"disadvantages":    str("Trusted Russian limitations/disadvantages."),
				"sales_status":     enumStr("active or inactive.", "active", "inactive"),
				"featured_image":   materialID("Single main tariff image."),
				"pricing_images":   materialIDs("Price cards and pricing illustrations."),
				"explainer_videos": materialIDs("Videos explaining the tariff."),
				"terms_documents":  materialIDs("Tariff terms and conditions documents."),
			}),
		}, upsertCommon()), "changes"),
		Meta: widgetMeta(),
	}
}

func contactsUpsertTool() Tool {
	return Tool{
		Name:        "kb_contacts_upsert",
		Description: "Create or patch the organization contacts singleton. Key is \"main\".",
		InputSchema: obj(merge(map[string]any{
			"changes": changesObject(map[string]any{
				"whatsapp":                str("Exact approved WhatsApp contact."),
				"email":                   str("Exact approved support email."),
				"address":                 str("Trusted Russian business address."),
				"legal_information":       str("Trusted Russian legal/company details."),
				"callback_time":           str("Trusted Russian callback expectation."),
				"working_hours":           str("Exact approved working-hours display."),
				"phone":                   str("Exact approved support phone."),
				"website":                 str("Exact approved website."),
				"instagram":               str("Exact approved Instagram account."),
				"contact_card_image":      materialID("Single contact-card image."),
				"location_map_image":      materialID("Single location/map image."),
				"company_legal_documents": materialIDs("Customer-sendable company/legal documents."),
			}),
		}, upsertCommon()), "changes"),
		Meta: widgetMeta(),
	}
}

func policiesUpsertTool() Tool {
	return Tool{
		Name:        "kb_policies_upsert",
		Description: "Create or patch the commerce policies singleton (delivery, payment, returns, warranty). Key is \"main\". When per-zone delivery zones exist, delivery_cost/delivery_in_days must stay blank and outside_zones_note is required — use kb_delivery_zone_upsert for per-zone pricing instead.",
		InputSchema: obj(merge(map[string]any{
			"changes": changesObject(map[string]any{
				"delivery_cost":             str("Exact approved flat delivery price. Must stay blank while delivery zones exist."),
				"delivery_in_days":          str("Exact delivery duration/range in days. Must stay blank while delivery zones exist."),
				"free_delivery_from":        str("Exact order value that qualifies for free delivery."),
				"min_order":                 str("Exact minimum order value."),
				"prepayment":                str("Trusted Russian prepayment policy."),
				"installment":               str("Trusted Russian installment policy."),
				"return_period_in_days":     str("Exact return duration in days."),
				"warranty":                  str("Trusted Russian warranty policy."),
				"outside_zones_note":        str("Exact approved refusal for a direction matching no delivery zone. Required while delivery zones exist."),
				"commerce_policy_documents": materialIDs("Customer-sendable commerce-policy documents."),
			}),
		}, upsertCommon()), "changes"),
		Meta: widgetMeta(),
	}
}

func deliveryZoneUpsertTool() Tool {
	return Tool{
		Name:        "kb_delivery_zone_upsert",
		Description: "Create or patch a delivery-coverage zone. Key is `ref`. Provide ref to update an existing zone or create with that exact ref; omit ref to derive one from the name (after duplicate checks). `delivery_available` is required when creating. A zone with delivery_available=true must have delivery_cost and delivery_in_days both set; one with delivery_available=false must have both blank.",
		InputSchema: obj(merge(map[string]any{
			"ref": str("Existing zone's ref to update, or a new ref to create with. Omit to derive a ref from changes.name."),
			"changes": changesObject(map[string]any{
				"name":               str("Russian zone display name. Required when creating."),
				"zone_level":         enumStr("Required when creating.", "city", "region", "country"),
				"parent_ref":         str("This organization's own delivery-zone ref this zone nests under (city → region → country). Empty for a top-level zone."),
				"delivery_available": boolean("Whether this zone is served at all. Required when creating."),
				"delivery_cost":      str("Exact approved delivery price for this zone. Required iff delivery_available."),
				"delivery_in_days":   str("Exact delivery duration/range in days for this zone. Required iff delivery_available."),
				"notes":              str("Trusted Russian prose about this zone."),
				"sales_status":       enumStr("active or inactive.", "active", "inactive"),
			}),
		}, upsertCommon()), "changes"),
		Meta: widgetMeta(),
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
		Meta: widgetMeta(),
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
		Meta: widgetMeta(),
	}
}

func kbInfoTool() Tool {
	return Tool{
		Name:        "kb_info",
		Description: "Explain the KB types, their natural keys, required/supported fields, the duplicate-check workflow, the draft-only mutation rule, media-field meanings, and how to review and publish. The server's own instructions already summarize this — call kb_info only if you need more detail.",
		InputSchema: obj(map[string]any{}),
		Meta:        widgetMeta(),
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
				"field": str("The semantic media field this upload is intended for, e.g. featured_image, gallery_images."),
			}, "type", "field"),
		}, "filename", "mime_type", "size_bytes"),
	}
}
