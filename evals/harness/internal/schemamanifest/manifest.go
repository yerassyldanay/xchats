// Package schemamanifest turns evals/harness/internal/kbfixture's struct tags
// — the eval family's own authoring source of truth — into a structural
// description of what the eight approved-KB production tables plus
// kbd_materials must look like, with zero hand-maintained column lists.
//
// Two things feed the manifest, and they are cross-checked against each
// other rather than independently declared: kbfixture's struct tags name the
// business columns, and backend/aiprompt's registry (MediaColumnsFor) says
// which of those are semantic media columns and whether each is singular
// (nullable uuid) or plural (uuid[] NOT NULL DEFAULT '{}'). See
// manifest_test.go for the drift test that fails offline if the two
// disagree — that is what keeps "same as evals" mechanically true instead of
// a review convention.
//
// Deliberately NOT derived here: fine-grained NULL/NOT NULL for plain text
// columns. kbfixture's YAML shape has no way to distinguish "this field is
// NULL in the DB" from "this field is an empty string" — every business
// column is a plain Go string, never a pointer, except the one remaining
// StrictBool flag. Asserting a specific nullability for every text column
// from that shape would be a guess, not a derivation, so this package only
// asserts nullability/default for the one column kbfixture DOES encode
// unambiguously via *StrictBool: delivery_available.
package schemamanifest

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"xchats-evals-harness/internal/kbfixture"
)

// ColumnSpec is one business column's expected shape.
type ColumnSpec struct {
	Name string
	// SQLType is one of: text | integer | boolean | uuid | uuid[] | jsonb.
	SQLType string
	// StrictNullability is true only for the StrictBool-backed
	// delivery_available column — see the package doc comment for why plain
	// text columns don't carry an asserted nullability here.
	StrictNullability bool
	// NotNull / NoDefault apply only when StrictNullability is true.
	NotNull   bool
	NoDefault bool
}

// TableSpec is one approved-KB table's expected shape.
type TableSpec struct {
	// Name is the physical table name, e.g. "ai_products".
	Name string
	// NaturalKey is the column set (besides the surrogate id) that uniquely
	// identifies a row within an organization — not derivable from kbfixture
	// tags (YAML has no key-ness concept), so it is the one small
	// hand-maintained policy table the package doc comment refers to. Every
	// entry here is cross-checked against InfrastructureColumns +
	// BusinessColumns in manifest_test.go so it cannot silently name a column
	// that does not exist.
	NaturalKey []string
	// BusinessColumns are in kbfixture struct-field order.
	BusinessColumns []ColumnSpec
}

// Manifest is the full comparison target for the eight approved-KB tables.
type Manifest struct {
	Tables []TableSpec
	// InfrastructureColumns are the only non-business, non-natural-key
	// columns production may carry — the four the eval fixture projects away
	// (package doc comment in kbfixture/types.go).
	InfrastructureColumns []string
}

// InfrastructureColumns is exported for callers (the DB comparator, tests)
// that need the exact list without constructing a Manifest.
var InfrastructureColumns = []string{"id", "organization_id", "created_at", "updated_at"}

// naturalKeys is the one hand-maintained policy table BuildManifest needs
// beyond what kbfixture's tags encode — see TableSpec.NaturalKey's doc
// comment. ai_contacts/ai_policies are true singletons (organization_id
// alone); every other table adds its own natural ref/slug.
var naturalKeys = map[string][]string{
	"ai_assistants":     {"organization_id"},
	"ai_topics":         {"organization_id", "slug"},
	"ai_products":       {"organization_id", "ref"},
	"ai_tariffs":        {"organization_id", "ref"},
	"ai_contacts":       {"organization_id"},
	"ai_policies":       {"organization_id"},
	"ai_tariff_info":    {"organization_id"},
	"ai_delivery_zones": {"organization_id", "ref"},
}

// tableGoType pairs each physical table name with the kbfixture Go type that
// mirrors one of its rows (or its singleton object, for ai_assistants /
// ai_contacts / ai_policies) — the reflection root BuildManifest walks.
var tableGoType = map[string]reflect.Type{
	"ai_assistants":     reflect.TypeOf(kbfixture.AIAssistant{}),
	"ai_topics":         reflect.TypeOf(kbfixture.AITopic{}),
	"ai_products":       reflect.TypeOf(kbfixture.AIProduct{}),
	"ai_tariffs":        reflect.TypeOf(kbfixture.AITariff{}),
	"ai_contacts":       reflect.TypeOf(kbfixture.AIContacts{}),
	"ai_policies":       reflect.TypeOf(kbfixture.AIPolicies{}),
	"ai_tariff_info":    reflect.TypeOf(kbfixture.AITariffInfo{}),
	"ai_delivery_zones": reflect.TypeOf(kbfixture.AIDeliveryZone{}),
}

// mediaTableSegment maps a physical table name to the model-facing table
// segment aiprompt's media registry keys on (aiprompt/registry.go: "topics"
// not "ai_topics" — the registry is keyed on the media-token vocabulary, the
// fixture/DB side is keyed on the physical table name).
var mediaTableSegment = map[string]string{
	"ai_topics":   "topics",
	"ai_products": "products",
	"ai_tariffs":  "tariffs",
	"ai_contacts": "contacts",
	"ai_policies": "policies",
}

// TableNames returns the eight approved-KB table names in a stable order.
func TableNames() []string {
	names := make([]string, 0, len(tableGoType))
	for t := range tableGoType {
		names = append(names, t)
	}
	sort.Strings(names)
	return names
}

// strictBoolType is *kbfixture.StrictBool — matched by identity so
// BuildManifest recognizes the pointer-to-StrictBool shape regardless of
// which field carries it, rather than hardcoding field names.
var strictBoolType = reflect.TypeOf((*kbfixture.StrictBool)(nil))

// additionalFactsType is []kbfixture.AIAdditionalFact — matched by identity,
// the same technique strictBoolType uses, so BuildManifest recognizes the
// virtual-fact-columns shape shared by ai_products/ai_tariffs/ai_tariff_info
// (backend/aiprompt/facts.go's AdditionalFact) regardless of field name. It
// always maps to SQLType "jsonb": a JSON array column, never a nullable
// scalar or a media-registry column.
var additionalFactsType = reflect.TypeOf([]kbfixture.AIAdditionalFact{})

// BuildManifest reflects over kbfixture's struct tags and cross-checks every
// []string / *StrictBool field against backend/aiprompt's media registry,
// returning a fully-derived Manifest. It returns an error (never panics) if
// a field's shape doesn't match anything the registry or the type-inference
// rules below recognize — that is the "fails offline" half of the drift
// guarantee the package doc comment describes; manifest_test.go exercises it.
func BuildManifest() (Manifest, error) {
	m := Manifest{InfrastructureColumns: append([]string(nil), InfrastructureColumns...)}
	for _, name := range TableNames() {
		spec, err := buildTableSpec(name)
		if err != nil {
			return Manifest{}, fmt.Errorf("schemamanifest: %s: %w", name, err)
		}
		m.Tables = append(m.Tables, spec)
	}
	return m, nil
}

func buildTableSpec(table string) (TableSpec, error) {
	goType := tableGoType[table]
	segment, hasMedia := mediaTableSegment[table]
	var mediaByColumn map[string]aiprompt.MediaColumn
	if hasMedia {
		mediaByColumn = map[string]aiprompt.MediaColumn{}
		for _, mc := range aiprompt.MediaColumnsFor(segment) {
			mediaByColumn[mc.Column] = mc
		}
	}

	spec := TableSpec{Name: table, NaturalKey: naturalKeys[table]}
	seenMedia := map[string]bool{}
	for i := 0; i < goType.NumField(); i++ {
		field := goType.Field(i)
		colName := field.Tag.Get("yaml")
		if colName == "" {
			return TableSpec{}, fmt.Errorf("field %s has no yaml tag", field.Name)
		}

		if mc, ok := mediaByColumn[colName]; ok {
			seenMedia[colName] = true
			col, err := mediaColumnSpec(field, colName, mc)
			if err != nil {
				return TableSpec{}, err
			}
			spec.BusinessColumns = append(spec.BusinessColumns, col)
			continue
		}

		col, err := scalarColumnSpec(field, colName)
		if err != nil {
			return TableSpec{}, err
		}
		spec.BusinessColumns = append(spec.BusinessColumns, col)
	}

	if hasMedia {
		for col := range mediaByColumn {
			if !seenMedia[col] {
				return TableSpec{}, fmt.Errorf("aiprompt registry declares media column %q.%q with no matching kbfixture field", table, col)
			}
		}
	}
	return spec, nil
}

// mediaColumnSpec cross-checks a kbfixture field believed to be a media
// column against the registry entry of the same name: a singular column
// must be a plain string field (holds one material id, or "" for none), a
// plural column must be a []string field (ordered material ids) — any other
// shape is a drift error, not a silent coercion.
func mediaColumnSpec(field reflect.StructField, colName string, mc aiprompt.MediaColumn) (ColumnSpec, error) {
	if mc.Singular {
		if field.Type.Kind() != reflect.String {
			return ColumnSpec{}, fmt.Errorf("media column %q is Singular in the registry but kbfixture field %s is %s, want string",
				colName, field.Name, field.Type)
		}
		return ColumnSpec{Name: colName, SQLType: "uuid"}, nil
	}
	if field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.String {
		return ColumnSpec{}, fmt.Errorf("media column %q is plural in the registry but kbfixture field %s is %s, want []string",
			colName, field.Name, field.Type)
	}
	return ColumnSpec{Name: colName, SQLType: "uuid[]"}, nil
}

// scalarColumnSpec infers a non-media business column's SQL type from its Go
// kind. *StrictBool is matched by type identity (not by field name), so it
// is the one shape this package asserts strict nullability/default for.
func scalarColumnSpec(field reflect.StructField, colName string) (ColumnSpec, error) {
	switch {
	case field.Type == strictBoolType:
		return ColumnSpec{Name: colName, SQLType: "boolean", StrictNullability: true, NotNull: true, NoDefault: true}, nil
	case field.Type == additionalFactsType:
		return ColumnSpec{Name: colName, SQLType: "jsonb"}, nil
	case field.Type.Kind() == reflect.String:
		return ColumnSpec{Name: colName, SQLType: "text"}, nil
	case field.Type.Kind() == reflect.Int || field.Type.Kind() == reflect.Int64:
		return ColumnSpec{Name: colName, SQLType: "integer"}, nil
	default:
		return ColumnSpec{}, fmt.Errorf("field %s (column %q) has unrecognized type %s — not a media column, string, int, *StrictBool, or []AIAdditionalFact",
			field.Name, colName, field.Type)
	}
}

// KBDMaterialsEyeVisibleColumns are the kbd_materials columns the eval
// fixture actually exercises (KBDMaterial's yaml tags) — the subset
// production's kbd_materials MUST contain, per its documented superset
// relationship (kbfixture/types.go's package doc comment: extraction/
// authoring columns are deliberately excluded from the fixture but remain
// allowed in production). BuildManifest does not include kbd_materials in
// Manifest.Tables because it is checked as a superset, not exact parity —
// see the DB comparator.
func KBDMaterialsEyeVisibleColumns() []string {
	t := reflect.TypeOf(kbfixture.KBDMaterial{})
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("yaml")
		if tag == "" || tag == "organization_id" {
			continue // fixture-level / infrastructure, not a superset check target
		}
		out = append(out, tag)
	}
	return out
}
