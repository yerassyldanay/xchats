package mcpserver_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

func newTestServer(t *testing.T) (*mcpserver.Server, mcpauth.Principal) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping mcpserver DB test")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `DROP SCHEMA IF EXISTS xchats CASCADE; DROP TABLE IF EXISTS public.xchats_schema_migrations`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, "op@example.com", "hash", "Op")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(st.Close)

	srv := mcpserver.New(mcpserver.Deps{KB: kbstore.New(st.Pool())})
	principal := mcpauth.Principal{
		UserID: user.ID, OrganizationID: org.ID, ClientID: "test-client",
		Scopes: []string{mcpauth.ScopeKBRead, mcpauth.ScopeKBDraftWrite, mcpauth.ScopeMediaWrite},
	}
	return srv, principal
}

func rpcID(n int) json.RawMessage { b, _ := json.Marshal(n); return b }

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func callTool(t *testing.T, srv *mcpserver.Server, principal mcpauth.Principal, name string, args map[string]any) map[string]any {
	t.Helper()
	req := mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(1), Method: "tools/call",
		Params: mustMarshal(t, map[string]any{"name": name, "arguments": args}),
	}
	resp := srv.Handle(context.Background(), principal, req)
	if resp.Error != nil {
		t.Fatalf("tools/call %s returned an RPC error: %+v", name, resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("tools/call %s: result is not a map: %#v", name, resp.Result)
	}
	return result
}

// TestInitialize_ReturnsProtocolShape is a basic handshake smoke test.
func TestInitialize_ReturnsProtocolShape(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "initialize"})
	if resp.Error != nil {
		t.Fatalf("initialize returned an error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	if result["protocolVersion"] == "" {
		t.Fatal("expected a non-empty protocolVersion")
	}
	if result["instructions"] == "" {
		t.Fatal("expected non-empty server instructions")
	}
}

// TestToolsList_HasAllTwelveTools confirms the closed contract's shape.
func TestToolsList_HasAllTwelveTools(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	if resp.Error != nil {
		t.Fatalf("tools/list returned an error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]mcpserver.Tool)
	if len(tools) != 12 {
		t.Fatalf("expected 12 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.InputSchema == nil {
			t.Fatalf("tool %s has no inputSchema", tool.Name)
		}
	}
	for _, want := range []string{
		"kb_assistant_upsert", "kb_topic_upsert", "kb_product_upsert", "kb_tariff_upsert",
		"kb_contacts_upsert", "kb_policies_upsert", "kb_delivery_zone_upsert",
		"kb_read", "kb_delete", "kb_summary", "kb_info", "kb_media_upload",
	} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
}

// TestToolsCall_MissingScopeIsRejectedAsToolError confirms a scope check
// failure surfaces as a normal (non-RPC-error) tool result the model can
// read, per plan/mcp.md's minimum-scopes model.
func TestToolsCall_MissingScopeIsRejectedAsToolError(t *testing.T) {
	srv, principal := newTestServer(t)
	principal.Scopes = []string{mcpauth.ScopeKBRead} // no kb:draft:write
	req := mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(1), Method: "tools/call",
		Params: mustMarshal(t, map[string]any{
			"name":      "kb_topic_upsert",
			"arguments": map[string]any{"changes": map[string]any{"title": "x", "body_md": "y"}},
		}),
	}
	resp := srv.Handle(context.Background(), principal, req)
	if resp.Error != nil {
		t.Fatalf("expected a successful RPC envelope with isError:true, got RPC error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError:true for a missing scope, got %+v", result)
	}
}

// TestProductUpsert_EndToEnd exercises the tool-call layer's argument
// parsing (tri-state changes) and error mapping against a real kbstore.
func TestProductUpsert_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)

	// Missing in_stock on create → isError, not a hard failure.
	res := callTool(t, srv, principal, "kb_product_upsert", map[string]any{
		"changes": map[string]any{"name": "Кофемашина"},
	})
	if res["isError"] != true {
		t.Fatalf("expected isError for missing in_stock, got %+v", res)
	}

	// A clean create.
	res = callTool(t, srv, principal, "kb_product_upsert", map[string]any{
		"changes": map[string]any{"name": "Кофемашина", "price": "129900", "in_stock": true},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}
	structured := res["structuredContent"].(kbstore.UpsertResult)
	key := structured.Key
	if key == "" {
		t.Fatal("expected a non-empty derived key")
	}

	// Exact duplicate name under a different explicit ref → isError with
	// actionable guidance mentioning the existing key.
	res = callTool(t, srv, principal, "kb_product_upsert", map[string]any{
		"ref":     "another-ref",
		"changes": map[string]any{"name": "Кофемашина", "in_stock": true},
	})
	if res["isError"] != true {
		t.Fatalf("expected a duplicate-conflict isError, got %+v", res)
	}
	text := res["content"].([]map[string]any)[0]["text"].(string)
	if !strings.Contains(text, key) {
		t.Fatalf("expected the conflict message to mention the existing key %q, got %q", key, text)
	}
}

// TestKBSummaryAndRead_RoundTrip exercises the two read tools together.
func TestKBSummaryAndRead_RoundTrip(t *testing.T) {
	srv, principal := newTestServer(t)
	callTool(t, srv, principal, "kb_tariff_upsert", map[string]any{
		"ref":     "biz",
		"changes": map[string]any{"name": "Business", "pricing_type": "fixed"},
	})

	summary := callTool(t, srv, principal, "kb_summary", map[string]any{"types": []string{"tariff"}})
	sc := summary["structuredContent"].(map[string]any)
	items := sc["items"].([]map[string]any)
	if len(items) != 1 || items[0]["key"] != "biz" || items[0]["state"] != "new" {
		t.Fatalf("unexpected summary items: %+v", items)
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []string{"tariff"}, "key": "biz", "source": "draft"})
	page := read["structuredContent"].(kbstore.ReadPage)
	if len(page.Items) != 1 || page.Items[0].Source != "draft" {
		t.Fatalf("unexpected read page: %+v", page)
	}
}

// TestKBMediaUpload_ReturnsSignedTarget confirms the upload tool creates a
// material and returns a usable target shape, wiring SignUpload through.
func TestKBMediaUpload_ReturnsSignedTarget(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_media_upload", map[string]any{
		"filename": "hero.jpg", "mime_type": "image/jpeg", "size_bytes": 12345,
		"target": map[string]any{"type": "product", "field": "featured_image"},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}
	sc := res["structuredContent"].(map[string]any)
	if sc["material_id"] == "" || sc["upload_url"] == "" || sc["upload_method"] != "PUT" {
		t.Fatalf("incomplete upload target: %+v", sc)
	}
}

// TestProductUpsert_JSONWireFormat verifies the ACTUAL bytes a real MCP host
// receives — every earlier test reads resp.Result as an in-process Go value,
// which never proves the response marshals to sane JSON (a struct with no
// json tags would silently serialize under its exported Go field names
// instead of the documented snake_case wire keys).
func TestProductUpsert_JSONWireFormat(t *testing.T) {
	srv, principal := newTestServer(t)
	req := mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(7), Method: "tools/call",
		Params: mustMarshal(t, map[string]any{
			"name":      "kb_product_upsert",
			"arguments": map[string]any{"changes": map[string]any{"name": "Товар", "in_stock": true}},
		}),
	}
	resp := srv.Handle(context.Background(), principal, req)
	wire, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("unmarshal wire bytes: %v", err)
	}
	result, ok := decoded["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected a result object, got %s", wire)
	}
	sc, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("expected structuredContent object, got %s", wire)
	}
	for _, key := range []string{"type", "key", "created", "draft_version"} {
		if _, ok := sc[key]; !ok {
			t.Fatalf("expected snake_case key %q in structuredContent, got %s", key, wire)
		}
	}
}

// TestKBMediaUpload_RejectsMismatchedTargetMime confirms the target-field
// mime hint is enforced.
func TestKBMediaUpload_RejectsMismatchedTargetMime(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_media_upload", map[string]any{
		"filename": "doc.pdf", "mime_type": "application/pdf", "size_bytes": 100,
		"target": map[string]any{"type": "product", "field": "featured_image"},
	})
	if res["isError"] != true {
		t.Fatalf("expected a mime-mismatch isError, got %+v", res)
	}
}

// TestResources_ListAndReadTheKBManagerWidget is a regression guard for the
// embedded widget resource (plan/mcp.md §6): resources/list must advertise
// exactly the one reusable ui://xchats/kb-manager.html resource, and
// resources/read must serve real HTML+JS content, not the old placeholder
// (`<p>KB Manager widget placeholder ...`) and not an empty/truncated embed.
// This does not replace exercising the widget in an actual browser (done
// manually against a mocked host bridge, matching the real Go tool result
// shapes, for this change) — it only catches the file going missing, empty,
// or reverting to the placeholder in a future edit.
func TestResources_ListAndReadTheKBManagerWidget(t *testing.T) {
	srv, principal := newTestServer(t)

	listResp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "resources/list"})
	if listResp.Error != nil {
		t.Fatalf("resources/list returned an error: %+v", listResp.Error)
	}
	listResult := listResp.Result.(map[string]any)
	resources := listResult["resources"].([]map[string]any)
	if len(resources) != 1 || resources[0]["uri"] != "ui://xchats/kb-manager.html" {
		t.Fatalf("expected exactly the one KB Manager resource, got %+v", resources)
	}
	if resources[0]["mimeType"] != "text/html" {
		t.Fatalf("expected mimeType text/html, got %+v", resources[0])
	}

	readResp := srv.Handle(context.Background(), principal, mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(2), Method: "resources/read",
		Params: mustMarshal(t, map[string]any{"uri": "ui://xchats/kb-manager.html"}),
	})
	if readResp.Error != nil {
		t.Fatalf("resources/read returned an error: %+v", readResp.Error)
	}
	readResult := readResp.Result.(map[string]any)
	contents := readResult["contents"].([]map[string]any)
	if len(contents) != 1 {
		t.Fatalf("expected exactly one content entry, got %+v", contents)
	}
	html, _ := contents[0]["text"].(string)
	if strings.Contains(html, "KB Manager widget placeholder") {
		t.Fatalf("widget resource still serves the old stub, not the real implementation")
	}
	for _, want := range []string{"<script>", "</script>", "callTool", "window.openai"} {
		if !strings.Contains(html, want) {
			t.Fatalf("widget HTML missing expected content %q (len=%d)", want, len(html))
		}
	}
	if len(html) < 5000 {
		t.Fatalf("widget HTML looks truncated: only %d bytes", len(html))
	}

	// An unknown resource URI must be a clean invalid-params error, not a panic.
	badResp := srv.Handle(context.Background(), principal, mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(3), Method: "resources/read",
		Params: mustMarshal(t, map[string]any{"uri": "ui://xchats/does-not-exist.html"}),
	})
	if badResp.Error == nil {
		t.Fatalf("expected an error for an unknown resource uri")
	}
}
