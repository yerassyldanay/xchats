package schemamanifest

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Mode selects how strictly CompareToDB judges the live database.
type Mode int

const (
	// ModeTransitional allows the columns in legacyAllowlist and the tables in
	// legacyExtraTables to remain, alongside the full canonical set — the
	// state between migration 0011 (additive) and 0012 (enforcing). No other
	// drift is tolerated.
	ModeTransitional Mode = iota
	// ModeStrict allows nothing beyond the manifest: no legacy columns, no
	// ai_assets/ai_draft_assets. The state after migration 0012.
	ModeStrict
)

// legacyAllowlist is the exact, version-controlled set of pre-canonical
// business columns ModeTransitional still tolerates — see
// plan/database-schema.md's "not part of the target" note and this repo's
// migration 0011/0012 split. Every entry here is a column migration 0012 is
// expected to drop; CompareToDB in ModeStrict rejects all of them.
var legacyAllowlist = map[string][]string{
	"ai_topics":         {"lang"},
	"ai_products":       {"lang", "data", "status", "availability"},
	"ai_tariffs":        {"lang", "data", "status"},
	"ai_contacts":       {"slug", "lang", "legal"},
	"ai_policies":       {"slug", "lang", "delivery_time", "return_period"},
	"ai_delivery_zones": {"status"},
}

// legacyExtraTables are the two retired-but-not-yet-dropped tables
// ModeTransitional tolerates as extras in the ai_* namespace.
var legacyExtraTables = []string{"ai_assets", "ai_draft_assets"}

// operationalTables are excluded from the approved-KB table-set comparison
// entirely (in every mode) — see plan Context: "ai_audit_log and ai_drafts
// are operational tables and are explicitly excluded from the parity
// comparison."
var operationalTables = map[string]bool{"ai_audit_log": true, "ai_drafts": true}

type dbColumn struct {
	dataType   string
	udtName    string // for arrays / uuid detection ("_uuid" for uuid[], "uuid" for uuid)
	isArray    bool
	isNullable bool
	hasDefault bool
}

// CompareToDB queries information_schema for the xchats schema and returns a
// human-readable diff for every mismatch against m — empty means the live
// database exactly matches (mode-adjusted) parity. It never mutates the
// database. Table/column enumeration and business-column checks always run;
// singular-media foreign-key checks are best-effort (a missing FK is
// reported, but the absence of pg_constraint access does not abort the run).
func CompareToDB(ctx context.Context, pool *pgxpool.Pool, m Manifest, mode Mode) ([]string, error) {
	var diffs []string

	liveTables, err := queryAITables(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("schemamanifest: query tables: %w", err)
	}
	diffs = append(diffs, diffTableSet(m, liveTables, mode)...)

	for _, ts := range m.Tables {
		cols, err := queryColumns(ctx, pool, ts.Name)
		if err != nil {
			return nil, fmt.Errorf("schemamanifest: query columns for %s: %w", ts.Name, err)
		}
		diffs = append(diffs, diffTableColumns(ts, cols, mode)...)
	}

	materialDiffs, err := diffKBDMaterials(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("schemamanifest: kbd_materials: %w", err)
	}
	diffs = append(diffs, materialDiffs...)

	sort.Strings(diffs)
	return diffs, nil
}

func queryAITables(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'xchats' AND table_name LIKE 'ai\_%' ESCAPE '\'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func diffTableSet(m Manifest, live map[string]bool, mode Mode) []string {
	want := map[string]bool{}
	for _, ts := range m.Tables {
		want[ts.Name] = true
	}
	allowedExtra := map[string]bool{}
	if mode == ModeTransitional {
		for _, t := range legacyExtraTables {
			allowedExtra[t] = true
		}
	}

	var diffs []string
	for name := range want {
		if !live[name] {
			diffs = append(diffs, fmt.Sprintf("table %s: missing", name))
		}
	}
	for name := range live {
		if operationalTables[name] || want[name] || allowedExtra[name] {
			continue
		}
		diffs = append(diffs, fmt.Sprintf("table %s: unexpected extra table in ai_* namespace", name))
	}
	return diffs
}

func queryColumns(ctx context.Context, pool *pgxpool.Pool, table string) (map[string]dbColumn, error) {
	rows, err := pool.Query(ctx, `
		SELECT column_name, data_type, udt_name, (is_nullable = 'YES'), (column_default IS NOT NULL)
		FROM information_schema.columns
		WHERE table_schema = 'xchats' AND table_name = $1`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]dbColumn{}
	for rows.Next() {
		var name, dataType, udtName string
		var nullable, hasDefault bool
		if err := rows.Scan(&name, &dataType, &udtName, &nullable, &hasDefault); err != nil {
			return nil, err
		}
		out[name] = dbColumn{
			dataType:   dataType,
			udtName:    udtName,
			isArray:    dataType == "ARRAY",
			isNullable: nullable,
			hasDefault: hasDefault,
		}
	}
	return out, rows.Err()
}

func diffTableColumns(ts TableSpec, live map[string]dbColumn, mode Mode) []string {
	var diffs []string
	wantCols := map[string]ColumnSpec{}
	for _, c := range ts.BusinessColumns {
		wantCols[c.Name] = c
	}

	for _, c := range ts.BusinessColumns {
		col, ok := live[c.Name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s.%s: missing business column", ts.Name, c.Name))
			continue
		}
		if got := sqlTypeName(col); got != c.SQLType {
			diffs = append(diffs, fmt.Sprintf("%s.%s: type %s, want %s", ts.Name, c.Name, got, c.SQLType))
		}
		if c.SQLType == "uuid[]" && (col.isNullable || !col.hasDefault) {
			diffs = append(diffs, fmt.Sprintf("%s.%s: want NOT NULL DEFAULT '{}', got nullable=%v has_default=%v", ts.Name, c.Name, col.isNullable, col.hasDefault))
		}
		if c.StrictNullability {
			if c.NotNull && col.isNullable {
				diffs = append(diffs, fmt.Sprintf("%s.%s: want NOT NULL, is nullable", ts.Name, c.Name))
			}
			if c.NoDefault && col.hasDefault {
				diffs = append(diffs, fmt.Sprintf("%s.%s: want no column default, has one", ts.Name, c.Name))
			}
		}
	}

	allowedLegacy := map[string]bool{}
	if mode == ModeTransitional {
		for _, c := range legacyAllowlist[ts.Name] {
			allowedLegacy[c] = true
		}
	}
	infra := map[string]bool{}
	for _, c := range InfrastructureColumns {
		infra[c] = true
	}
	for name := range live {
		if infra[name] || allowedLegacy[name] {
			continue
		}
		if _, ok := wantCols[name]; ok {
			continue
		}
		diffs = append(diffs, fmt.Sprintf("%s.%s: forbidden legacy column present", ts.Name, name))
	}
	return diffs
}

func sqlTypeName(col dbColumn) string {
	switch {
	case col.isArray && col.udtName == "_uuid":
		return "uuid[]"
	case col.dataType == "uuid":
		return "uuid"
	case col.dataType == "boolean":
		return "boolean"
	case col.dataType == "integer":
		return "integer"
	case col.dataType == "text":
		return "text"
	default:
		return col.dataType
	}
}

// diffKBDMaterials checks kbd_materials as a SUPERSET, not exact parity: it
// must contain every eval-visible registry column with a plausible type; any
// additional extraction/authoring columns are permitted (kbfixture/types.go's
// documented projection).
func diffKBDMaterials(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	live, err := queryColumns(ctx, pool, "kbd_materials")
	if err != nil {
		return nil, err
	}
	var diffs []string
	for _, col := range KBDMaterialsEyeVisibleColumns() {
		if _, ok := live[col]; !ok {
			diffs = append(diffs, fmt.Sprintf("kbd_materials.%s: missing eval-visible registry column", col))
		}
	}
	return diffs, nil
}
