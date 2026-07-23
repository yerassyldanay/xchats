package kbfixture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

const invalidFixtureDir = "../../testdata/kbfixture/invalid"

// TestInvalidFixtures_FailClosed is the harness-level integration proof
// behind backend/aiprompt's own exhaustive fail-closed unit tests
// (catalog_test.go's TestBuildCatalog_MaterialFailClosed): loading each tiny
// negative fixture through the SAME path a real scenario render uses
// (kbfixture.Load, then aiprompt.BuildCatalog) must fail, never silently
// drop the bad reference or produce a partial catalog. One fixture per
// fail-closed rule; none of these is ever used by a billed scenario.
func TestInvalidFixtures_FailClosed(t *testing.T) {
	cases := []struct {
		file   string
		reason string
	}{
		{"dangling_material.yaml", "media column references a material id with no kbd_materials row"},
		{"wrong_organization.yaml", "referenced material belongs to a different organization"},
		{"non_file_source.yaml", "referenced material's source_type is not \"file\""},
		{"missing_storage_locator.yaml", "referenced material has no storage_key"},
		{"zero_size.yaml", "referenced material has size_bytes: 0"},
		{"invisible_customer.yaml", "referenced material's customer_visibility is \"invisible\""},
		{"mime_mismatch.yaml", "referenced material's MIME type doesn't match the column's kind"},
		{"singular_column_multiple_refs.yaml", "a singular media column is given two refs instead of one"},
		{"delivery_zone_contradictory_row.yaml", "an available delivery zone has a blank cost or delivery days"},
		{"delivery_zone_bad_level.yaml", "a delivery zone's zone_level is outside city|region|country"},
		{"delivery_zone_missing_parent.yaml", "a delivery zone's parent_ref names a zone that does not exist"},
		{"delivery_zones_without_outside_note.yaml", "delivery zones exist but ai_policies.outside_zones_note is blank"},
		{"delivery_zones_with_flat_cost.yaml", "delivery zones exist but the flat ai_policies.delivery_cost is still set"},
	}

	entries, err := os.ReadDir(invalidFixtureDir)
	if err != nil {
		t.Fatalf("read %s: %v", invalidFixtureDir, err)
	}
	if len(entries) != len(cases) {
		t.Fatalf("%s has %d files but this test enumerates %d cases — keep them in lockstep (one fixture per fail-closed rule)", invalidFixtureDir, len(entries), len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join(invalidFixtureDir, tc.file)
			kb, err := Load(path)
			if err != nil {
				return // failed at load/decode time — still fail-closed, e.g. the YAML-shape case
			}
			if _, err := aiprompt.BuildCatalog(kb); err == nil {
				t.Errorf("want a render failure (%s), got a valid catalog", tc.reason)
			}
		})
	}
}
