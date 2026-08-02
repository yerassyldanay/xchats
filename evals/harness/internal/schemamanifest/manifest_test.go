package schemamanifest

import (
	"testing"
)

// TestBuildManifest_Succeeds is the drift guard: if kbfixture gains, loses,
// or retypes a field in a way the registry cross-check or type-inference
// rules don't recognize, BuildManifest returns an error and this test fails
// offline — no DB needed. This is deliberately the ONLY assertion that
// matters for drift detection; the tests below pin manifest CONTENT so a
// silent wrong-but-non-erroring derivation is also caught.
func TestBuildManifest_Succeeds(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatalf("BuildManifest: %v — kbfixture and aiprompt's media registry have drifted apart, see the package doc comment", err)
	}
	if len(m.Tables) != 7 {
		t.Fatalf("want 7 approved-KB tables, got %d: %+v", len(m.Tables), m.Tables)
	}
}

func TestBuildManifest_TableNames(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	want := map[string]bool{
		"ai_assistants": true, "ai_topics": true, "ai_products": true, "ai_tariffs": true,
		"ai_contacts": true, "ai_policies": true, "ai_delivery_zones": true,
	}
	got := map[string]bool{}
	for _, ts := range m.Tables {
		got[ts.Name] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing table %q in manifest", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("unexpected table %q in manifest", name)
		}
	}
}

func columnsOf(t *testing.T, m Manifest, table string) map[string]ColumnSpec {
	t.Helper()
	for _, ts := range m.Tables {
		if ts.Name == table {
			out := map[string]ColumnSpec{}
			for _, c := range ts.BusinessColumns {
				out[c.Name] = c
			}
			return out
		}
	}
	t.Fatalf("table %q not in manifest", table)
	return nil
}

// TestBuildManifest_ProductsMediaColumns pins the exact set + typing of
// ai_products' 5 semantic media columns (1 singular + 4 plural) — the
// highest-count table, so the most likely place a registry/kbfixture drift
// would first surface.
func TestBuildManifest_ProductsMediaColumns(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	cols := columnsOf(t, m, "ai_products")

	singular := []string{"featured_image"}
	plural := []string{
		"gallery_images", "demo_videos", "certificate_documents", "guarantee_documents",
	}
	for _, name := range singular {
		c, ok := cols[name]
		if !ok || c.SQLType != "uuid" {
			t.Errorf("%s: want SQLType uuid, got %+v (present=%v)", name, c, ok)
		}
	}
	for _, name := range plural {
		c, ok := cols[name]
		if !ok || c.SQLType != "uuid[]" {
			t.Errorf("%s: want SQLType uuid[], got %+v (present=%v)", name, c, ok)
		}
	}
}

// TestBuildManifest_StrictBoolColumns pins the two columns kbfixture encodes
// unambiguously as NOT NULL / no-default booleans.
func TestBuildManifest_StrictBoolColumns(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	cases := []struct{ table, column string }{
		{"ai_products", "in_stock"},
		{"ai_delivery_zones", "delivery_available"},
	}
	for _, c := range cases {
		col, ok := columnsOf(t, m, c.table)[c.column]
		if !ok {
			t.Fatalf("%s.%s: missing from manifest", c.table, c.column)
		}
		if col.SQLType != "boolean" || !col.StrictNullability || !col.NotNull || !col.NoDefault {
			t.Errorf("%s.%s: want boolean/NOT NULL/no-default, got %+v", c.table, c.column, col)
		}
	}
}

// TestBuildManifest_AllMediaColumnsAccountedFor cross-checks in the other
// direction: every registry entry for a media-bearing table must have
// surfaced as a business column — buildTableSpec already errors on this
// (see the "no matching kbfixture field" branch), so this test only needs to
// confirm BuildManifest actually reaches that success path today, pinning
// the total media-column count across all five media-bearing tables (4 + 5 +
// 4 + 3 + 1 = 17, matching plan/DECISIONS.md's "Concrete media columns").
func TestBuildManifest_AllMediaColumnsAccountedFor(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	total := 0
	for _, ts := range m.Tables {
		for _, c := range ts.BusinessColumns {
			if c.SQLType == "uuid" || c.SQLType == "uuid[]" {
				total++
			}
		}
	}
	if total != 17 {
		t.Fatalf("want 17 semantic media columns across the manifest, got %d", total)
	}
}

func TestBuildManifest_NaturalKeysReferenceRealColumns(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	for _, ts := range m.Tables {
		cols := columnsOf(t, m, ts.Name)
		for _, k := range ts.NaturalKey {
			if k == "organization_id" {
				continue // infrastructure, not a business column
			}
			if _, ok := cols[k]; !ok {
				t.Errorf("table %s: NaturalKey column %q is not a business column", ts.Name, k)
			}
		}
	}
}

func TestKBDMaterialsEyeVisibleColumns(t *testing.T) {
	got := KBDMaterialsEyeVisibleColumns()
	want := map[string]bool{
		"id": true, "source_type": true, "source_ref": true, "filename": true, "mime_type": true,
		"size_bytes": true, "storage_backend": true, "storage_key": true,
		"processing_status": true, "customer_visibility": true,
	}
	if len(got) != len(want) {
		t.Fatalf("want %d eval-visible kbd_materials columns, got %d: %v", len(want), len(got), got)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("unexpected eval-visible kbd_materials column %q", c)
		}
	}
}
