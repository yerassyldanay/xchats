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

// testUploadBaseURL is the API origin newTestServer configures — used by
// the widget resource CSP tests to confirm the domain lists are actually
// derived from Deps.UploadBaseURL, not hard-coded or empty.
const testUploadBaseURL = "https://api.xchats.test"

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

	srv := mcpserver.New(mcpserver.Deps{KB: kbstore.New(st.Pool()), UploadBaseURL: testUploadBaseURL})
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

// TestToolsList_HasAllThirteenTools confirms the closed contract's shape.
func TestToolsList_HasAllThirteenTools(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	if resp.Error != nil {
		t.Fatalf("tools/list returned an error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]mcpserver.Tool)
	if len(tools) != 13 {
		t.Fatalf("expected 13 tools, got %d", len(tools))
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
		"kb_read", "kb_delete", "kb_summary", "kb_info", "kb_media_upload", "kb_media_attach",
	} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
}

// TestToolsList_DeclaresSecuritySchemes is Task 11's regression guard: every
// scope-gated tool must declare its OAuth requirement in securitySchemes, not
// only enforce it silently at call time.
func TestToolsList_DeclaresSecuritySchemes(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	if resp.Error != nil {
		t.Fatalf("tools/list returned an error: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]mcpserver.Tool)
	wantScope := map[string]string{
		"kb_read":         mcpauth.ScopeKBRead,
		"kb_summary":       mcpauth.ScopeKBRead,
		"kb_info":          mcpauth.ScopeKBRead,
		"kb_topic_upsert":  mcpauth.ScopeKBDraftWrite,
		"kb_delete":        mcpauth.ScopeKBDraftWrite,
		"kb_media_upload":  mcpauth.ScopeMediaWrite,
	}
	found := map[string]bool{}
	for _, tool := range tools {
		found[tool.Name] = true
		want, ok := wantScope[tool.Name]
		if !ok {
			continue
		}
		// A LIST of tagged-union entries, not a keyed object — ChatGPT
		// rejects the entire tools/list response otherwise (see
		// Tool.SecuritySchemes). Asserting the container type here is the
		// point of this check, not an incidental detail.
		if len(tool.SecuritySchemes) != 1 {
			t.Fatalf("tool %s: expected exactly one securitySchemes entry, got %+v", tool.Name, tool.SecuritySchemes)
		}
		scheme := tool.SecuritySchemes[0]
		if scheme["type"] != "oauth2" {
			t.Fatalf("tool %s: expected securitySchemes[0].type == \"oauth2\" (the union discriminator), got %+v", tool.Name, scheme["type"])
		}
		scopes, ok := scheme["scopes"].([]string)
		if !ok || len(scopes) != 1 || scopes[0] != want {
			t.Fatalf("tool %s: expected securitySchemes[0].scopes == [%q], got %+v", tool.Name, want, scheme["scopes"])
		}
	}
	for name := range wantScope {
		if !found[name] {
			t.Fatalf("expected tool %q in tools/list", name)
		}
	}
}

// TestToolsList_SecuritySchemesSerializeAsJSONArray guards the exact wire
// shape a host parses, not just the Go type. ChatGPT rejected the entire
// tools/list response — the connector would not install at all — when
// securitySchemes marshalled as a keyed object instead of a list:
//
//	1 validation error for list[tagged-union[NoAuthSecurityScheme,OAuthSecurityScheme]]
//	Input should be a valid list [type=list_type,
//	input_value={'oauth2': {'scopes': [...], 'type': 'oauth2'}}, input_type=dict]
//
// The typed test above would keep passing if the field were ever widened
// back to `any` and populated with a map, so assert the marshalled JSON.
// Needs no database: Tools() is pure.
func TestToolsList_SecuritySchemesSerializeAsJSONArray(t *testing.T) {
	raw, err := json.Marshal(mcpserver.Tools())
	if err != nil {
		t.Fatalf("marshal tools: %v", err)
	}
	var tools []map[string]any
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatalf("unmarshal tools: %v", err)
	}

	checked := 0
	for _, tool := range tools {
		schemes, present := tool["securitySchemes"]
		if !present {
			continue
		}
		list, ok := schemes.([]any)
		if !ok {
			t.Fatalf("tool %v: securitySchemes must marshal as a JSON array, got %T (%v)", tool["name"], schemes, schemes)
		}
		if len(list) == 0 {
			t.Fatalf("tool %v: securitySchemes array is empty", tool["name"])
		}
		entry, ok := list[0].(map[string]any)
		if !ok {
			t.Fatalf("tool %v: securitySchemes[0] must be an object, got %T", tool["name"], list[0])
		}
		if entry["type"] != "oauth2" {
			t.Fatalf("tool %v: securitySchemes[0].type must be the union discriminator \"oauth2\", got %v", tool["name"], entry["type"])
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no tool declared securitySchemes — the assertion never ran")
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
	// Task 11: a scope failure also carries a structured runtime challenge
	// under _meta, not just a text error the model must interpret.
	meta, ok := result["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected a _meta object on a scope-failure result, got %+v", result)
	}
	challenge, ok := meta["mcp/www_authenticate"].(map[string]any)
	if !ok {
		t.Fatalf("expected _meta[\"mcp/www_authenticate\"] on a scope-failure result, got %+v", meta)
	}
	if challenge["scope"] != mcpauth.ScopeKBDraftWrite {
		t.Fatalf("expected the challenge to name the missing scope %q, got %+v", mcpauth.ScopeKBDraftWrite, challenge)
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

// The following six tests are the release-gate "one end-to-end mutation per
// write tool — all seven, not only zones" verification item: each drives a
// create through the ACTUAL MCP tools/call protocol layer (not kbstore
// directly — that's mcp_test.go's job) and confirms the write is readable
// back through kb_read(source=draft), mirroring TestProductUpsert_EndToEnd's
// pattern for the tool it doesn't cover.

func TestAssistantUpsert_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_assistant_upsert", map[string]any{
		"changes": map[string]any{"persona": "Дружелюбный консультант", "reply_max_words": 80},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []any{"assistant"}, "source": "draft"})
	page := read["structuredContent"].(kbstore.ReadPage)
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 draft assistant record, got %d: %+v", len(page.Items), page.Items)
	}
	cfg := page.Items[0].Data.(kbstore.DraftConfig)
	if cfg.Persona != "Дружелюбный консультант" || cfg.ReplyMaxWords != 80 {
		t.Fatalf("persisted assistant config mismatch: %+v", cfg)
	}
}

func TestTopicUpsert_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_topic_upsert", map[string]any{
		"changes": map[string]any{"title": "Как оформить заказ", "body_md": "Оформление заказа занимает две минуты."},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}
	key := res["structuredContent"].(kbstore.UpsertResult).Key
	if key == "" {
		t.Fatal("expected a non-empty derived slug")
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []any{"topic"}, "source": "draft", "key": key})
	page := read["structuredContent"].(kbstore.ReadPage)
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 draft topic record, got %d: %+v", len(page.Items), page.Items)
	}
	row := page.Items[0].Data.(kbstore.TopicRow)
	if row.Title != "Как оформить заказ" || row.BodyMD != "Оформление заказа занимает две минуты." {
		t.Fatalf("persisted topic mismatch: %+v", row)
	}
}

func TestTariffUpsert_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_tariff_upsert", map[string]any{
		"changes": map[string]any{"name": "Стандарт", "price": "4990", "pricing_type": "fixed"},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}
	key := res["structuredContent"].(kbstore.UpsertResult).Key
	if key == "" {
		t.Fatal("expected a non-empty derived ref")
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []any{"tariff"}, "source": "draft", "key": key})
	page := read["structuredContent"].(kbstore.ReadPage)
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 draft tariff record, got %d: %+v", len(page.Items), page.Items)
	}
	row := page.Items[0].Data.(kbstore.TariffRow)
	if row.Name != "Стандарт" || row.Price != "4990" || row.PricingType != "fixed" {
		t.Fatalf("persisted tariff mismatch: %+v", row)
	}
}

func TestContactsUpsert_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_contacts_upsert", map[string]any{
		"changes": map[string]any{"whatsapp": "+7 700 000 00 00", "email": "support@example.com"},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []any{"contacts"}, "source": "draft"})
	page := read["structuredContent"].(kbstore.ReadPage)
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 draft contacts record, got %d: %+v", len(page.Items), page.Items)
	}
	row := page.Items[0].Data.(kbstore.ContactRow)
	if row.WhatsApp != "+7 700 000 00 00" || row.Email != "support@example.com" {
		t.Fatalf("persisted contacts mismatch: %+v", row)
	}
}

func TestPoliciesUpsert_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_policies_upsert", map[string]any{
		"changes": map[string]any{"delivery_cost": "990", "delivery_in_days": "1-2", "warranty": "12 месяцев"},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []any{"policies"}, "source": "draft"})
	page := read["structuredContent"].(kbstore.ReadPage)
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 draft policies record, got %d: %+v", len(page.Items), page.Items)
	}
	row := page.Items[0].Data.(kbstore.PolicyRow)
	if row.DeliveryCost != "990" || row.DeliveryInDays != "1-2" || row.Warranty != "12 месяцев" {
		t.Fatalf("persisted policies mismatch: %+v", row)
	}
}

func TestDeliveryZoneUpsert_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_delivery_zone_upsert", map[string]any{
		"changes": map[string]any{
			"name": "Алматы", "zone_level": "city",
			"delivery_available": true, "delivery_cost": "1500", "delivery_in_days": "1",
		},
	})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}
	key := res["structuredContent"].(kbstore.UpsertResult).Key
	if key == "" {
		t.Fatal("expected a non-empty derived ref")
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []any{"delivery_zone"}, "source": "draft", "key": key})
	page := read["structuredContent"].(kbstore.ReadPage)
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 draft delivery zone record, got %d: %+v", len(page.Items), page.Items)
	}
	row := page.Items[0].Data.(kbstore.ZoneRow)
	if row.Name != "Алматы" || row.ZoneLevel != "city" || !row.DeliveryAvailable || row.DeliveryCost != "1500" {
		t.Fatalf("persisted delivery zone mismatch: %+v", row)
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

// TestKBSummary_SourceAndQueryFilter is Task 3's regression guard:
// kb_summary's source and query parameters are declared in its tool schema
// (plan/mcp.md §4/§7/§8's duplicate-detection workflow explicitly calls
// kb_summary(source="both", query=...)) but were silently ignored by the
// handler — past `limit` records a model would see an unfiltered page, find
// no match, and create a duplicate. Seeds one published-only tariff, one
// draft-only tariff, and one changed (both) tariff, then confirms
// source=live/draft/both each return exactly the expected subset, and query
// narrows by a case-insensitive title substring.
func TestKBSummary_SourceAndQueryFilter(t *testing.T) {
	srv, principal := newTestServer(t)
	ctx := context.Background()

	callTool(t, srv, principal, "kb_tariff_upsert", map[string]any{
		"ref": "published-only", "changes": map[string]any{"name": "Опубликован", "pricing_type": "fixed"},
	})
	if err := srv.Deps.KB.Approve(ctx, principal.OrganizationID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	callTool(t, srv, principal, "kb_tariff_upsert", map[string]any{
		"ref": "new-only", "changes": map[string]any{"name": "Новый Тариф", "pricing_type": "fixed"},
	})
	callTool(t, srv, principal, "kb_tariff_upsert", map[string]any{
		"ref": "published-only", "changes": map[string]any{"summary": "обновлено"},
	})

	keysFor := func(source, query string) map[string]bool {
		args := map[string]any{"types": []string{"tariff"}}
		if source != "" {
			args["source"] = source
		}
		if query != "" {
			args["query"] = query
		}
		res := callTool(t, srv, principal, "kb_summary", args)
		sc := res["structuredContent"].(map[string]any)
		items := sc["items"].([]map[string]any)
		out := map[string]bool{}
		for _, it := range items {
			out[it["key"].(string)] = true
		}
		return out
	}

	if live := keysFor("live", ""); len(live) != 1 || !live["published-only"] {
		t.Fatalf("source=live: expected only published-only, got %+v", live)
	}
	if draft := keysFor("draft", ""); len(draft) != 2 || !draft["published-only"] || !draft["new-only"] {
		t.Fatalf("source=draft: expected published-only and new-only, got %+v", draft)
	}
	if both := keysFor("both", ""); len(both) != 2 {
		t.Fatalf("source=both: expected both tariffs, got %+v", both)
	}
	if matched := keysFor("both", "новый"); len(matched) != 1 || !matched["new-only"] {
		t.Fatalf("query=новый: expected only new-only, got %+v", matched)
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
		"target": map[string]any{"type": "product", "field": "gallery_images"},
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
		"target": map[string]any{"type": "product", "field": "gallery_images"},
	})
	if res["isError"] != true {
		t.Fatalf("expected a mime-mismatch isError, got %+v", res)
	}
}

// TestKBMediaUpload_RejectsInvalidTargetFieldPairing confirms target is
// validated against the type-scoped attachment registry, not just the flat
// field→kind map — a field that exists but on a DIFFERENT type, and
// featured_image (a real media column that is never an attachment target),
// must both be rejected.
func TestKBMediaUpload_RejectsInvalidTargetFieldPairing(t *testing.T) {
	srv, principal := newTestServer(t)

	res := callTool(t, srv, principal, "kb_media_upload", map[string]any{
		"filename": "hero.jpg", "mime_type": "image/jpeg", "size_bytes": 100,
		"target": map[string]any{"type": "topic", "field": "gallery_images"}, // product-only field
	})
	if res["isError"] != true {
		t.Fatalf("expected isError for a field valid on a different type, got %+v", res)
	}

	res = callTool(t, srv, principal, "kb_media_upload", map[string]any{
		"filename": "hero.jpg", "mime_type": "image/jpeg", "size_bytes": 100,
		"target": map[string]any{"type": "product", "field": "featured_image"},
	})
	if res["isError"] != true {
		t.Fatalf("expected isError for featured_image (never an attachment target), got %+v", res)
	}
}

// TestKBInfo_MediaAttachmentFields confirms kb_info's structuredContent
// carries the type-scoped registry the widget drives its attach form from,
// that it agrees with kb_media_attach's own acceptance, and that
// featured_image never appears there (it is a real media column — present
// in the older, flat media_field_kinds, kept for compatibility — but never
// an attachment target).
func TestKBInfo_MediaAttachmentFields(t *testing.T) {
	srv, principal := newTestServer(t)
	res := callTool(t, srv, principal, "kb_info", map[string]any{})
	if res["isError"] == true {
		t.Fatalf("expected success, got %+v", res)
	}
	sc := res["structuredContent"].(map[string]any)

	fields, ok := sc["media_attachment_fields"].(map[string][]kbstore.MediaAttachmentField)
	if !ok {
		t.Fatalf("expected media_attachment_fields as map[string][]kbstore.MediaAttachmentField, got %#v", sc["media_attachment_fields"])
	}
	productFields := fields["product"]
	if len(productFields) == 0 {
		t.Fatalf("expected a non-empty product entry, got %#v", productFields)
	}
	seenGallery := false
	for _, f := range productFields {
		if f.Field == "featured_image" {
			t.Fatalf("featured_image must never appear in media_attachment_fields, got %+v", productFields)
		}
		if f.Field == "gallery_images" {
			seenGallery = true
			if f.Kind != "image" || !f.Multiple {
				t.Fatalf("expected gallery_images {kind:image, multiple:true}, got %+v", f)
			}
		}
	}
	if !seenGallery {
		t.Fatalf("expected gallery_images among product's media_attachment_fields, got %+v", productFields)
	}
	// assistant and delivery_zone have no media columns at all — absent, not
	// present-and-empty.
	for _, absent := range []string{"assistant", "delivery_zone"} {
		if _, present := fields[absent]; present {
			t.Fatalf("expected %q absent from media_attachment_fields, got %+v", absent, fields[absent])
		}
	}

	kinds, ok := sc["media_field_kinds"].(map[string]string)
	if !ok {
		t.Fatalf("expected media_field_kinds as map[string]string, got %#v", sc["media_field_kinds"])
	}
	if kinds["featured_image"] != "image" {
		t.Fatalf("expected media_field_kinds to still carry featured_image (compatibility), got %+v", kinds["featured_image"])
	}
}

// TestKBMediaAttach_EndToEnd exercises the tool-call layer for kb_media_attach:
// stage a material, complete its upload (kbstore.CompleteMaterialUpload — the
// signed-PUT HTTP route lives in internal/httpapi, exercised separately by
// TestMCPMediaUploadEndToEnd), attach it to an existing product's
// gallery_images, and confirm the draft reflects it. The bulk of the
// validation matrix (invalid pairing, missing/deleted record, bad material)
// is covered at the kbstore layer (mcp_attach_test.go); this only confirms
// the tool wires through to it correctly.
func TestKBMediaAttach_EndToEnd(t *testing.T) {
	srv, principal := newTestServer(t)
	ctx := context.Background()

	callTool(t, srv, principal, "kb_product_upsert", map[string]any{
		"ref": "widget-1", "changes": map[string]any{"name": "Товар", "in_stock": true},
	})

	materialID, err := srv.Deps.KB.CreateUploadMaterial(ctx, principal.OrganizationID, kbstore.UploadMaterialInput{
		Filename: "photo.jpg", MimeType: "image/jpeg", SizeBytes: 10,
	})
	if err != nil {
		t.Fatalf("stage material: %v", err)
	}
	if err := srv.Deps.KB.CompleteMaterialUpload(ctx, materialID, "disk", "org/x/"+materialID.String(), 10, ""); err != nil {
		t.Fatalf("complete upload: %v", err)
	}

	attach := callTool(t, srv, principal, "kb_media_attach", map[string]any{
		"material_id": materialID.String(), "type": "product", "key": "widget-1", "field": "gallery_images",
	})
	if attach["isError"] == true {
		t.Fatalf("expected success, got %+v", attach)
	}
	sc := attach["structuredContent"].(kbstore.MediaAttachResult)
	if sc.MaterialID != materialID.String() || sc.Key != "widget-1" || sc.Field != "gallery_images" {
		t.Fatalf("unexpected attach result: %+v", sc)
	}

	read := callTool(t, srv, principal, "kb_read", map[string]any{"types": []string{"product"}, "key": "widget-1", "source": "draft"})
	page := read["structuredContent"].(kbstore.ReadPage)
	row := page.Items[0].Data.(kbstore.ProductRow)
	if len(row.GalleryImages) != 1 || row.GalleryImages[0].String() != materialID.String() {
		t.Fatalf("expected gallery_images=[%s], got %v", materialID, row.GalleryImages)
	}

	// Attaching to featured_image — a real field kb_read/kb_product_upsert
	// both know about, but never an attachment target — is a clean tool
	// error, not an RPC failure or a silent no-op.
	badAttach := callTool(t, srv, principal, "kb_media_attach", map[string]any{
		"material_id": materialID.String(), "type": "product", "key": "widget-1", "field": "featured_image",
	})
	if badAttach["isError"] != true {
		t.Fatalf("expected isError attaching to featured_image, got %+v", badAttach)
	}
}

// TestKBUpsert_MalformedProvenanceRejected is Task 16/6's strict-parsing
// regression guard: a provenance.material_ids entry that is not a UUID must
// be rejected as invalid params — not silently dropped into a default value
// the way this used to (args.go's old provenanceString always defaulted to
// {"source":"mcp"} on any parse failure). callTool can't be reused here
// (it fails the test on an RPC error) since this specific case IS expected
// to surface as one — a malformed argument is a protocol-level problem, not
// a KB-domain outcome.
func TestKBUpsert_MalformedProvenanceRejected(t *testing.T) {
	srv, principal := newTestServer(t)
	req := mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(1), Method: "tools/call",
		Params: mustMarshal(t, map[string]any{
			"name": "kb_product_upsert",
			"arguments": map[string]any{
				"changes":    map[string]any{"name": "Товар", "in_stock": true},
				"provenance": map[string]any{"material_ids": []string{"not-a-uuid"}},
			},
		}),
	}
	resp := srv.Handle(context.Background(), principal, req)
	if resp.Error == nil {
		t.Fatalf("expected a JSON-RPC error for a malformed provenance.material_ids, got result: %#v", resp.Result)
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
	// text/html;profile=mcp-app (Task 9), not bare text/html: the profile
	// parameter is what marks this as an interactive MCP App resource rather
	// than inert markup a host might just render read-only.
	if resources[0]["mimeType"] != "text/html;profile=mcp-app" {
		t.Fatalf("expected mimeType text/html;profile=mcp-app, got %+v", resources[0])
	}
	assertWidgetCSP(t, resources[0]["_meta"])

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
	if contents[0]["mimeType"] != "text/html;profile=mcp-app" {
		t.Fatalf("expected mimeType text/html;profile=mcp-app, got %+v", contents[0]["mimeType"])
	}
	assertWidgetCSP(t, contents[0]["_meta"])
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

// assertWidgetCSP confirms the configured upload origin (testUploadBaseURL)
// appears under both the modern domain-list keys (_meta.ui.csp.
// connectDomains/resourceDomains) and the legacy ChatGPT Apps SDK keys
// (_meta["openai/widgetCSP"].connect_domains/resource_domains) — the actual
// fix for the widget's upload PUT being blocked by its own declared policy
// (resources.go's widgetResourceMeta doc comment). Reused for both
// resources/list and resources/read, which must serve identical _meta.
func assertWidgetCSP(t *testing.T, rawMeta any) {
	t.Helper()
	meta, ok := rawMeta.(map[string]any)
	if !ok {
		t.Fatalf("expected a _meta object, got %#v", rawMeta)
	}
	ui, ok := meta["ui"].(map[string]any)
	if !ok {
		t.Fatalf("expected _meta.ui, got %+v", meta)
	}
	csp, ok := ui["csp"].(map[string]any)
	if !ok {
		t.Fatalf("expected _meta.ui.csp, got %+v", ui)
	}
	assertDomainList(t, csp["connectDomains"], "_meta.ui.csp.connectDomains")
	assertDomainList(t, csp["resourceDomains"], "_meta.ui.csp.resourceDomains")

	legacy, ok := meta["openai/widgetCSP"].(map[string]any)
	if !ok {
		t.Fatalf(`expected _meta["openai/widgetCSP"], got %+v`, meta)
	}
	assertDomainList(t, legacy["connect_domains"], `_meta["openai/widgetCSP"].connect_domains`)
	assertDomainList(t, legacy["resource_domains"], `_meta["openai/widgetCSP"].resource_domains`)
}

func assertDomainList(t *testing.T, raw any, path string) {
	t.Helper()
	list, ok := raw.([]string)
	if !ok {
		t.Fatalf("%s: expected a []string, got %#v", path, raw)
	}
	if len(list) != 1 || list[0] != testUploadBaseURL {
		t.Fatalf("%s: expected [%q], got %v", path, testUploadBaseURL, list)
	}
}

// TestResources_WidgetCSPEmptyWithoutUploadBaseURL confirms an unconfigured
// upload origin serializes as an empty domain list, not a null or a bogus
// entry — a server/test setup that never wired UploadBaseURL must not crash
// or produce a CSP a host can't parse.
func TestResources_WidgetCSPEmptyWithoutUploadBaseURL(t *testing.T) {
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
	user, err := st.SeedUser(ctx, org.ID, "op2@example.com", "hash", "Op")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(st.Close)

	srv := mcpserver.New(mcpserver.Deps{KB: kbstore.New(st.Pool())}) // UploadBaseURL left empty, deliberately
	principal := mcpauth.Principal{
		UserID: user.ID, OrganizationID: org.ID, ClientID: "test-client",
		Scopes: []string{mcpauth.ScopeKBRead, mcpauth.ScopeKBDraftWrite, mcpauth.ScopeMediaWrite},
	}

	resp := srv.Handle(ctx, principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "resources/list"})
	resources := resp.Result.(map[string]any)["resources"].([]map[string]any)
	ui := resources[0]["_meta"].(map[string]any)["ui"].(map[string]any)
	csp := ui["csp"].(map[string]any)
	list, ok := csp["connectDomains"].([]string)
	if !ok || len(list) != 0 {
		t.Fatalf("expected an empty (non-nil) connectDomains, got %#v", csp["connectDomains"])
	}
}

// TestToolsList_WidgetHintUsesNestedUIObject is Task 9's regression guard:
// the standard MCP Apps shape is a nested _meta.ui.resourceUri, not the flat
// "ui/resourceUri" key an earlier draft used.
func TestToolsList_WidgetHintUsesNestedUIObject(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	tools := resp.Result.(map[string]any)["tools"].([]mcpserver.Tool)
	for _, tool := range tools {
		if tool.Name != "kb_read" {
			continue
		}
		ui, ok := tool.Meta["ui"].(map[string]any)
		if !ok {
			t.Fatalf("kb_read: expected _meta.ui object, got %+v", tool.Meta)
		}
		if ui["resourceUri"] != "ui://xchats/kb-manager.html" {
			t.Fatalf("kb_read: expected _meta.ui.resourceUri, got %+v", ui)
		}
		if _, flat := tool.Meta["ui/resourceUri"]; flat {
			t.Fatalf("kb_read: the flat ui/resourceUri key should be gone, got %+v", tool.Meta)
		}
		return
	}
	t.Fatalf("kb_read tool not found")
}

// TestToolsList_MediaUploadIsAppOnly confirms kb_media_upload declares the
// standard app-only visibility hint (plan/mcp.md §5) and carries no
// resourceUri of its own (its result isn't rendered through the KB Manager
// resource).
func TestToolsList_MediaUploadIsAppOnly(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	tools := resp.Result.(map[string]any)["tools"].([]mcpserver.Tool)
	for _, tool := range tools {
		if tool.Name != "kb_media_upload" {
			continue
		}
		ui, ok := tool.Meta["ui"].(map[string]any)
		if !ok {
			t.Fatalf("kb_media_upload: expected _meta.ui object, got %+v", tool.Meta)
		}
		visibility, ok := ui["visibility"].([]string)
		if !ok || len(visibility) != 1 || visibility[0] != "app" {
			t.Fatalf("kb_media_upload: expected _meta.ui.visibility=[\"app\"], got %+v", ui["visibility"])
		}
		if _, has := ui["resourceUri"]; has {
			t.Fatalf("kb_media_upload: expected no resourceUri, got %+v", ui)
		}
		return
	}
	t.Fatalf("kb_media_upload tool not found")
}

// TestToolsList_MediaAttachIsAppOnly mirrors
// TestToolsList_MediaUploadIsAppOnly for kb_media_attach — it too is a
// widget-invoked tool with no resourceUri of its own, gated on
// kb:draft:write (it writes the draft) rather than media:write.
func TestToolsList_MediaAttachIsAppOnly(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	tools := resp.Result.(map[string]any)["tools"].([]mcpserver.Tool)
	for _, tool := range tools {
		if tool.Name != "kb_media_attach" {
			continue
		}
		ui, ok := tool.Meta["ui"].(map[string]any)
		if !ok {
			t.Fatalf("kb_media_attach: expected _meta.ui object, got %+v", tool.Meta)
		}
		visibility, ok := ui["visibility"].([]string)
		if !ok || len(visibility) != 1 || visibility[0] != "app" {
			t.Fatalf("kb_media_attach: expected _meta.ui.visibility=[\"app\"], got %+v", ui["visibility"])
		}
		if _, has := ui["resourceUri"]; has {
			t.Fatalf("kb_media_attach: expected no resourceUri, got %+v", ui)
		}
		if len(tool.SecuritySchemes) != 1 || tool.SecuritySchemes[0]["scopes"].([]string)[0] != mcpauth.ScopeKBDraftWrite {
			t.Fatalf("kb_media_attach: expected securitySchemes scope kb:draft:write, got %+v", tool.SecuritySchemes)
		}
		return
	}
	t.Fatalf("kb_media_attach tool not found")
}

// TestToolsList_DeclaresAnnotations checks the standard MCP tool-annotation
// hints plan Task 9 asks for: readOnlyHint on the three read-only tools,
// destructiveHint+idempotentHint:false on kb_delete, and idempotentHint:false
// on all seven typed upserts.
func TestToolsList_DeclaresAnnotations(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	tools := resp.Result.(map[string]any)["tools"].([]mcpserver.Tool)
	byName := map[string]mcpserver.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	for _, name := range []string{"kb_read", "kb_summary", "kb_info"} {
		ann := byName[name].Annotations
		if ann == nil || ann.ReadOnlyHint == nil || !*ann.ReadOnlyHint {
			t.Fatalf("%s: expected annotations.readOnlyHint=true, got %+v", name, ann)
		}
	}

	del := byName["kb_delete"].Annotations
	if del == nil || del.DestructiveHint == nil || !*del.DestructiveHint {
		t.Fatalf("kb_delete: expected annotations.destructiveHint=true, got %+v", del)
	}
	if del.IdempotentHint == nil || *del.IdempotentHint {
		t.Fatalf("kb_delete: expected annotations.idempotentHint=false, got %+v", del.IdempotentHint)
	}

	for _, name := range []string{
		"kb_assistant_upsert", "kb_topic_upsert", "kb_product_upsert", "kb_tariff_upsert",
		"kb_contacts_upsert", "kb_policies_upsert", "kb_delivery_zone_upsert",
	} {
		ann := byName[name].Annotations
		if ann == nil || ann.IdempotentHint == nil || *ann.IdempotentHint {
			t.Fatalf("%s: expected annotations.idempotentHint=false, got %+v", name, ann)
		}
	}
}

// TestToolsList_DeclaresOutputSchema confirms every one of the 13 tools
// documents its structuredContent shape (plan Task 9) — every tool sets
// structuredContent via handlers.go's toolResult, kb_delete included (via
// kbstore.DeleteResult).
func TestToolsList_DeclaresOutputSchema(t *testing.T) {
	srv, principal := newTestServer(t)
	resp := srv.Handle(context.Background(), principal, mcpserver.Request{JSONRPC: "2.0", ID: rpcID(1), Method: "tools/list"})
	tools := resp.Result.(map[string]any)["tools"].([]mcpserver.Tool)
	if len(tools) != 13 {
		t.Fatalf("expected 13 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.OutputSchema == nil {
			t.Fatalf("%s: expected a non-nil outputSchema", tool.Name)
		}
	}
}

// TestKBUpsert_UnknownChangesFieldRejected is Task 6's regression guard:
// every tool's `changes` schema declares additionalProperties:false, but
// nothing enforced it — a typo'd field name (here "titel" instead of
// "title") was silently dropped by rawObject/optString (no accessor ever
// reads a key it doesn't recognize) instead of surfaced to the caller.
func TestKBUpsert_UnknownChangesFieldRejected(t *testing.T) {
	srv, principal := newTestServer(t)
	req := mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(1), Method: "tools/call",
		Params: mustMarshal(t, map[string]any{
			"name": "kb_topic_upsert",
			"arguments": map[string]any{
				"slug":    "how-to-order",
				"changes": map[string]any{"titel": "Typo'd field name"},
			},
		}),
	}
	resp := srv.Handle(context.Background(), principal, req)
	if resp.Error == nil {
		t.Fatalf("expected a JSON-RPC error for an unknown changes field, got result: %#v", resp.Result)
	}
}

// TestKBSummary_AcceptsPluralTypeAliases is Task 12's regression guard:
// plan/mcp.md's own worked examples call kb_summary(types=["tariffs"]) and
// kb_read(types=["products"]) — plural — but the server accepted only the
// singular canonical form and turned the mismatch into a JSON-RPC
// -32602, contradicting handlers.go's own stated policy that a
// caller-correctable domain problem (like this) should reach the model as
// tool output, not a transport failure.
func TestKBSummary_AcceptsPluralTypeAliases(t *testing.T) {
	srv, principal := newTestServer(t)
	callTool(t, srv, principal, "kb_tariff_upsert", map[string]any{
		"ref": "biz", "changes": map[string]any{"name": "Business", "pricing_type": "fixed"},
	})

	// The plural form plan/mcp.md itself uses must work, not just the
	// singular internal vocabulary.
	res := callTool(t, srv, principal, "kb_summary", map[string]any{"types": []string{"tariffs"}})
	if res["isError"] == true {
		t.Fatalf("kb_summary(types=[\"tariffs\"]) returned isError: %#v", res)
	}
	sc := res["structuredContent"].(map[string]any)
	items := sc["items"].([]map[string]any)
	if len(items) != 1 || items[0]["key"] != "biz" {
		t.Fatalf("expected the plural alias to resolve to the tariff type, got %+v", items)
	}

	// A genuinely unknown type must be a clean tool-level isError, not an
	// RPC error the model has no way to recover from in-context.
	req := mcpserver.Request{
		JSONRPC: "2.0", ID: rpcID(1), Method: "tools/call",
		Params: mustMarshal(t, map[string]any{
			"name":      "kb_summary",
			"arguments": map[string]any{"types": []string{"not-a-real-type"}},
		}),
	}
	resp := srv.Handle(context.Background(), principal, req)
	if resp.Error != nil {
		t.Fatalf("expected an isError tool result for an unknown type, got an RPC error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok || result["isError"] != true {
		t.Fatalf("expected isError:true for an unknown type, got %#v", resp.Result)
	}
}
