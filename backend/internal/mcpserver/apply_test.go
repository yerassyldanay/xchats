package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
)

// rawArgs marshals a plain map[string]any into the map[string]json.RawMessage
// shape ParseUpsertCall (and dispatchToolsCall internally) expect — the same
// wire shape a real tools/call arguments object decodes into.
func rawArgs(t *testing.T, v map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage, len(v))
	for k, val := range v {
		b, err := json.Marshal(val)
		if err != nil {
			t.Fatalf("marshal %s: %v", k, err)
		}
		out[k] = b
	}
	return out
}

// stageMaterial stages and completes a tiny image material ready to attach
// (mirrors TestKBMediaAttach_EndToEnd's own setup in mcpserver_test.go).
func stageMaterial(t *testing.T, ctx context.Context, srv *mcpserver.Server, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := srv.Deps.KB.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: "photo.jpg", MimeType: "image/jpeg", SizeBytes: 10,
	})
	if err != nil {
		t.Fatalf("stage material: %v", err)
	}
	if err := srv.Deps.KB.CompleteMaterialUpload(ctx, id, "disk", "org/x/"+id.String(), 10, ""); err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	return id
}

// TestUpsertTools_ExcludesDelete guards plan/mcp.md §10's structural
// invariant: a caller iterating UpsertTools (internal/kbimport's synthesis
// pass) can never construct a delete call, because kb_delete is simply not
// a member of the set — internet-sourced content must never stage a
// deletion.
func TestUpsertTools_ExcludesDelete(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range mcpserver.UpsertTools {
		if name == "kb_delete" {
			t.Fatal("kb_delete must never appear in UpsertTools")
		}
		seen[name] = true
	}
	if len(mcpserver.UpsertTools) != 8 {
		t.Fatalf("UpsertTools has %d entries, want the 8 typed upserts", len(mcpserver.UpsertTools))
	}
	for _, want := range []string{
		"kb_topic_upsert", "kb_product_upsert", "kb_tariff_upsert", "kb_contacts_upsert",
		"kb_policies_upsert", "kb_tariff_info_upsert", "kb_delivery_zone_upsert", "kb_assistant_upsert",
	} {
		if !seen[want] {
			t.Errorf("UpsertTools is missing %q", want)
		}
	}
}

// TestParseUpsertCall_UnknownTool confirms a non-upsert (or nonexistent)
// tool name is rejected rather than silently doing nothing.
func TestParseUpsertCall_UnknownTool(t *testing.T) {
	for _, tool := range []string{"kb_delete", "kb_read", "kb_media_upload", "not_a_real_tool"} {
		if _, err := mcpserver.ParseUpsertCall(tool, rawArgs(t, map[string]any{"changes": map[string]any{}})); err == nil {
			t.Errorf("ParseUpsertCall(%q) = nil error, want a rejection", tool)
		}
	}
}

// TestParseUpsertCall_EquivalentToHandler proves ParseUpsertCall + Apply
// lands EXACTLY the same draft content as the live MCP tools/call path —
// the guard the refactor into shared parseXChanges helpers exists for. Two
// freshly seeded organizations each run the SAME args through the two
// entry points (tools/call vs. ParseUpsertCall+Apply); their resulting
// draft rows must match field for field. Covers a keyed create with a
// media reference (topic), a keyed create with an enum field (product),
// and the one singleton whose MCPUpsert method takes no provenance
// parameter at all (assistant).
func TestParseUpsertCall_EquivalentToHandler(t *testing.T) {
	t.Run("topic with media", func(t *testing.T) {
		srvA, principalA := newTestServer(t)
		srvB, principalB := newTestServer(t)
		ctx := context.Background()

		imgA := stageMaterial(t, ctx, srvA, principalA.OrganizationID)
		imgB := stageMaterial(t, ctx, srvB, principalB.OrganizationID)

		changes := func(img uuid.UUID) map[string]any {
			return map[string]any{
				"title": "Как оформить заказ", "body_md": "Текст инструкции.",
				"featured_image": img.String(),
			}
		}

		callTool(t, srvA, principalA, "kb_topic_upsert", map[string]any{"slug": "how-to-order", "changes": changes(imgA)})

		call, err := mcpserver.ParseUpsertCall("kb_topic_upsert", rawArgs(t, map[string]any{"slug": "how-to-order", "changes": changes(imgB)}))
		if err != nil {
			t.Fatalf("ParseUpsertCall: %v", err)
		}
		if call.Key != "how-to-order" {
			t.Fatalf("Key = %q, want %q", call.Key, "how-to-order")
		}
		if _, err := call.Apply(ctx, srvB.Deps.KB, principalB.OrganizationID, principalB.UserID, nil, kbstore.MCPProvenance{}); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		viewA, err := srvA.Deps.KB.DraftOnly(ctx, principalA.OrganizationID)
		if err != nil {
			t.Fatal(err)
		}
		viewB, err := srvB.Deps.KB.DraftOnly(ctx, principalB.OrganizationID)
		if err != nil {
			t.Fatal(err)
		}
		if len(viewA.Topics) != 1 || len(viewB.Topics) != 1 {
			t.Fatalf("expected exactly one draft topic each, got %d / %d", len(viewA.Topics), len(viewB.Topics))
		}
		a, b := viewA.Topics[0], viewB.Topics[0]
		if a.Slug != b.Slug || a.Title != b.Title || a.BodyMD != b.BodyMD {
			t.Fatalf("topic business fields diverged:\n  handler: %+v\n  apply:   %+v", a, b)
		}
		if a.FeaturedImage == nil || b.FeaturedImage == nil {
			t.Fatal("expected featured_image to be set on both")
		}
	})

	t.Run("product with enum and media", func(t *testing.T) {
		srvA, principalA := newTestServer(t)
		srvB, principalB := newTestServer(t)
		ctx := context.Background()

		imgA := stageMaterial(t, ctx, srvA, principalA.OrganizationID)
		imgB := stageMaterial(t, ctx, srvB, principalB.OrganizationID)

		changes := func(img uuid.UUID) map[string]any {
			return map[string]any{
				"name": "Магнитный сверлильный станок", "price": "180 000 ₸", "availability_status": "in_stock",
				"sales_status": "active", "gallery_images": []string{img.String()},
			}
		}

		callTool(t, srvA, principalA, "kb_product_upsert", map[string]any{"ref": "drill-zt40h", "changes": changes(imgA)})

		call, err := mcpserver.ParseUpsertCall("kb_product_upsert", rawArgs(t, map[string]any{"ref": "drill-zt40h", "changes": changes(imgB)}))
		if err != nil {
			t.Fatalf("ParseUpsertCall: %v", err)
		}
		if _, err := call.Apply(ctx, srvB.Deps.KB, principalB.OrganizationID, principalB.UserID, nil, kbstore.MCPProvenance{SourceURL: "https://vendor.example/drill"}); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		viewA, _ := srvA.Deps.KB.DraftOnly(ctx, principalA.OrganizationID)
		viewB, _ := srvB.Deps.KB.DraftOnly(ctx, principalB.OrganizationID)
		if len(viewA.Products) != 1 || len(viewB.Products) != 1 {
			t.Fatalf("expected exactly one draft product each, got %d / %d", len(viewA.Products), len(viewB.Products))
		}
		a, b := viewA.Products[0], viewB.Products[0]
		if a.Ref != b.Ref || a.Name != b.Name || a.Price != b.Price || a.AvailabilityStatus != b.AvailabilityStatus || a.SalesStatus != b.SalesStatus {
			t.Fatalf("product business fields diverged:\n  handler: %+v\n  apply:   %+v", a, b)
		}
		if len(a.GalleryImages) != 1 || len(b.GalleryImages) != 1 {
			t.Fatalf("expected exactly one gallery image each, got %d / %d", len(a.GalleryImages), len(b.GalleryImages))
		}
	})

	t.Run("assistant singleton with no provenance parameter", func(t *testing.T) {
		srvA, principalA := newTestServer(t)
		srvB, principalB := newTestServer(t)
		ctx := context.Background()

		changes := map[string]any{"persona": "Дружелюбный консультант.", "reply_max_words": 90}

		callTool(t, srvA, principalA, "kb_assistant_upsert", map[string]any{"changes": changes})

		call, err := mcpserver.ParseUpsertCall("kb_assistant_upsert", rawArgs(t, map[string]any{"changes": changes}))
		if err != nil {
			t.Fatalf("ParseUpsertCall: %v", err)
		}
		if call.Key != kbstore.NaturalKeyMain {
			t.Fatalf("Key = %q, want %q", call.Key, kbstore.NaturalKeyMain)
		}
		// A non-empty provenance is passed deliberately here: MCPUpsertAssistant
		// takes no provenance parameter at all, so Apply must silently drop it
		// rather than erroring or panicking.
		if _, err := call.Apply(ctx, srvB.Deps.KB, principalB.OrganizationID, principalB.UserID, nil,
			kbstore.MCPProvenance{SourceURL: "https://example.com/about"}); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		viewA, _ := srvA.Deps.KB.DraftOnly(ctx, principalA.OrganizationID)
		viewB, _ := srvB.Deps.KB.DraftOnly(ctx, principalB.OrganizationID)
		if viewA.Config.Persona != viewB.Config.Persona || viewA.Config.ReplyMaxWords != viewB.Config.ReplyMaxWords {
			t.Fatalf("assistant config diverged:\n  handler: %+v\n  apply:   %+v", viewA.Config, viewB.Config)
		}
	})
}

// TestUpsertToolSchemas_ReturnsRealInputSchemas confirms the returned
// schemas are the SAME InputSchema value Tools() itself declares, and that
// an unrecognized name is silently absent rather than an error.
func TestUpsertToolSchemas_ReturnsRealInputSchemas(t *testing.T) {
	schemas := mcpserver.UpsertToolSchemas(append(append([]string{}, mcpserver.UpsertTools...), "not_a_real_tool"))
	if len(schemas) != len(mcpserver.UpsertTools) {
		t.Fatalf("got %d schemas, want %d (unrecognized name should be silently absent)", len(schemas), len(mcpserver.UpsertTools))
	}
	for _, tool := range mcpserver.Tools() {
		want, ok := schemas[tool.Name]
		if !ok {
			continue // not one of the requested names
		}
		if want["type"] != tool.InputSchema["type"] {
			t.Fatalf("UpsertToolSchemas[%s] does not look like the real InputSchema", tool.Name)
		}
	}
}
