package kbstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Delivery zones (ai_delivery_zones) — live-only editor surface (/kb/zones),
// same shape as every other /kb/* entity (kb_live.go), plus the deterministic
// invariant aiprompt.BuildCatalog itself enforces at render time
// (backend/aiprompt/types.go · DeliveryZone/Policies doc comments): a zone
// world and the org's flat policy delivery fields can never both be "live" at
// once. zoneGateReasons/validateZoneWorld enforce that same rule HERE, at
// write time, so a live edit can never leave the KB in a shape the response
// path would later fail closed on.
// ---------------------------------------------------------------------------

// ZoneInput is an upsert payload for a live delivery zone.
type ZoneInput struct {
	Ref               string
	Name              string
	ZoneLevel         string // city | region | country
	ParentRef         string // "" for a top-level zone
	DeliveryAvailable bool
	DeliveryCost      string
	DeliveryInDays    string
	Notes             string
	SalesStatus       string // active | inactive; "" -> active
}

// ZoneRow is the editor-facing ai_delivery_zones row — json shape mirrors the
// other /kb/* view rows (id/draft/updated_at alongside the entity's own
// columns). Draft is true when a pending kbd_draft.delivery_zones entry
// shadows (or newly proposes) this ref — the same overlay-by-natural-key
// pattern as topics/tariffs/products (see mergedView).
type ZoneRow struct {
	ID                string    `json:"id"` // = ref
	Ref               string    `json:"ref"`
	Name              string    `json:"name"`
	ZoneLevel         string    `json:"zone_level"`
	ParentRef         string    `json:"parent_ref"`
	DeliveryAvailable bool      `json:"delivery_available"`
	DeliveryCost      string    `json:"delivery_cost"`
	DeliveryInDays    string    `json:"delivery_in_days"`
	Notes             string    `json:"notes"`
	SalesStatus       string    `json:"sales_status"`
	Draft             bool      `json:"draft"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// PutLiveZone upserts a delivery zone directly into the live table, then
// re-validates the org's whole zone/policy world in the SAME transaction
// (validateZoneWorld) before committing — an invalid zone (bad parent_ref,
// contradictory delivery_available/cost/days) or one that leaves an existing
// policy row incompatible (non-blank flat delivery fields) is rejected
// atomically, never landing half-written.
func (s *Store) PutLiveZone(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in ZoneInput) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := upsertZoneRow(ctx, tx, orgID, in); err != nil {
		return err
	}
	if err := validateZoneWorld(ctx, tx, orgID); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "zone:"+in.Ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteLiveZone removes a live zone by ref, then re-validates — deleting a
// zone another zone's parent_ref still points to is rejected, same as
// creating that dangling reference in the first place would be.
func (s *Store) DeleteLiveZone(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM ai_delivery_zones WHERE organization_id=$1 AND ref=$2`, orgID, ref); err != nil {
		return err
	}
	if err := validateZoneWorld(ctx, tx, orgID); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "zone:"+ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertZoneRow(ctx context.Context, db dbtx, orgID uuid.UUID, in ZoneInput) error {
	if _, err := db.Exec(ctx, `INSERT INTO ai_delivery_zones
		(organization_id, ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, notes, sales_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (organization_id, ref) DO UPDATE SET
			name=EXCLUDED.name, zone_level=EXCLUDED.zone_level, parent_ref=EXCLUDED.parent_ref,
			delivery_available=EXCLUDED.delivery_available, delivery_cost=EXCLUDED.delivery_cost,
			delivery_in_days=EXCLUDED.delivery_in_days, notes=EXCLUDED.notes, sales_status=EXCLUDED.sales_status, updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, in.Ref, in.Name, in.ZoneLevel, in.ParentRef, in.DeliveryAvailable, in.DeliveryCost, in.DeliveryInDays,
		in.Notes, orDefault(in.SalesStatus, "active")); err != nil {
		return fmt.Errorf("insert zone %s: %w", in.Ref, err)
	}
	return nil
}

// loadZoneRows reads every zone for the org, live-only (no draft concept).
// Free function (not a *Store method): db is always passed explicitly —
// either s.db for a side-effect-free read, or a tx when the read must
// see that same transaction's own uncommitted write (validateZoneWorld).
func loadZoneRows(ctx context.Context, db dbtx, orgID uuid.UUID) ([]ZoneRow, error) {
	rows, err := db.Query(ctx, `SELECT ref, name, zone_level, parent_ref, delivery_available, delivery_cost,
		delivery_in_days, notes, sales_status, updated_at
		FROM ai_delivery_zones WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ZoneRow
	for rows.Next() {
		var z ZoneRow
		if err := rows.Scan(&z.Ref, &z.Name, &z.ZoneLevel, &z.ParentRef, &z.DeliveryAvailable, &z.DeliveryCost,
			&z.DeliveryInDays, &z.Notes, &z.SalesStatus, &z.UpdatedAt); err != nil {
			return nil, err
		}
		z.ID = z.Ref
		out = append(out, z)
	}
	return out, rows.Err()
}

// loadPolicyRowsForGate reads every ai_policies row for the org — a narrower,
// tx-safe sibling of draft.go's mergedView policy query (no blob overlay, no
// Slug/ID convenience fields), used only to feed zoneGateReasons.
func loadPolicyRowsForGate(ctx context.Context, db dbtx, orgID uuid.UUID) ([]PolicyRow, error) {
	rows, err := db.Query(ctx, `SELECT delivery_cost, delivery_in_days, outside_zones_note
		FROM ai_policies WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PolicyRow
	for rows.Next() {
		var p PolicyRow
		var deliveryInDays *string
		if err := rows.Scan(&p.DeliveryCost, &deliveryInDays, &p.OutsideZonesNote); err != nil {
			return nil, err
		}
		p.DeliveryInDays = strOrEmpty(deliveryInDays)
		out = append(out, p)
	}
	return out, rows.Err()
}

// resultingZonesForGate computes the RESULTING zone set an Approve call would
// publish — live zones, overlaid by pending zone upserts, minus matching
// deletes — the same in-memory-merge idiom draft.go's mergeForGate uses for
// topics, so Approve validates the publishable state before writing anything.
func resultingZonesForGate(live []ZoneRow, upserts []DraftDeliveryZone, deletes []DraftDelete) []ZoneRow {
	del := map[string]bool{}
	for _, d := range deletes {
		if d.Kind == "delivery_zone" {
			del[d.Key] = true
		}
	}
	idx := map[string]int{}
	var out []ZoneRow
	for _, z := range live {
		if del[z.Ref] {
			continue
		}
		out = append(out, z)
		idx[z.Ref] = len(out) - 1
	}
	for _, u := range upserts {
		row := ZoneRow{
			Ref: u.Ref, Name: u.Name, ZoneLevel: u.ZoneLevel, ParentRef: u.ParentRef,
			DeliveryAvailable: u.DeliveryAvailable, DeliveryCost: u.DeliveryCost,
			DeliveryInDays: u.DeliveryInDays, Notes: u.Notes, SalesStatus: orDefault(u.SalesStatus, "active"),
		}
		if i, ok := idx[u.Ref]; ok {
			out[i] = row
		} else {
			out = append(out, row)
			idx[u.Ref] = len(out) - 1
		}
	}
	return out
}

// resultingPolicyForGate computes the resulting ai_policies row's
// gate-relevant fields: the live row, overridden by a pending policy draft
// entry in the SAME approve batch (there is at most one — policies is a
// singleton, so a pending entry fully replaces the live row for gate purposes).
func resultingPolicyForGate(ctx context.Context, db dbtx, orgID uuid.UUID, upserts []DraftPolicy) ([]PolicyRow, error) {
	if len(upserts) > 0 {
		p := upserts[0]
		return []PolicyRow{{DeliveryCost: p.DeliveryCost, DeliveryInDays: p.DeliveryInDays, OutsideZonesNote: p.OutsideZonesNote}}, nil
	}
	return loadPolicyRowsForGate(ctx, db, orgID)
}

// validateZoneWorld re-derives the org's current zone/policy world INSIDE the
// given transaction (so it sees that same transaction's own uncommitted
// write) and wraps any zoneGateReasons violation into a *GateError — the
// existing 422 mapping (httpapi.kbFail) every other live write already uses.
func validateZoneWorld(ctx context.Context, tx dbtx, orgID uuid.UUID) error {
	zones, err := loadZoneRows(ctx, tx, orgID)
	if err != nil {
		return err
	}
	policies, err := loadPolicyRowsForGate(ctx, tx, orgID)
	if err != nil {
		return err
	}
	if reasons := zoneGateReasons(zones, policies); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}
	return nil
}

// zoneGateReasons is the pure, table-tested invariant over one org's whole
// zone/policy world — mirrors aiprompt.BuildCatalog's own fail-closed rules
// (backend/aiprompt/types.go), enforced here at write time instead of only
// being discovered later when a customer reply is being rendered:
//
//   - a zone with delivery_available=true must carry non-blank
//     delivery_cost AND delivery_in_days; one with delivery_available=false
//     must carry neither.
//   - parent_ref, when set, must resolve to an existing zone of the SAME org,
//     must not reference itself, and must not form a cycle.
//   - the moment any zone exists, EVERY policy language row must have blank
//     delivery_cost and delivery_time (the flat KB-wide delivery answer) and
//     a non-blank outside_zones_note (the closed-world refusal for a
//     direction matching no zone).
func zoneGateReasons(zones []ZoneRow, policies []PolicyRow) []string {
	var reasons []string
	byRef := make(map[string]ZoneRow, len(zones))
	for _, z := range zones {
		byRef[z.Ref] = z
	}
	for _, z := range zones {
		if z.DeliveryAvailable {
			if strings.TrimSpace(z.DeliveryCost) == "" || strings.TrimSpace(z.DeliveryInDays) == "" {
				reasons = append(reasons, fmt.Sprintf(
					"zone %q: delivery_available is on, so delivery_cost and delivery_in_days are both required", z.Ref))
			}
		} else if strings.TrimSpace(z.DeliveryCost) != "" || strings.TrimSpace(z.DeliveryInDays) != "" {
			reasons = append(reasons, fmt.Sprintf(
				"zone %q: delivery_available is off, so delivery_cost and delivery_in_days must both be blank", z.Ref))
		}
		if z.ParentRef == "" {
			continue
		}
		if z.ParentRef == z.Ref {
			reasons = append(reasons, fmt.Sprintf("zone %q: parent_ref cannot reference itself", z.Ref))
			continue
		}
		if _, ok := byRef[z.ParentRef]; !ok {
			reasons = append(reasons, fmt.Sprintf("zone %q: parent_ref %q does not resolve to an existing zone", z.Ref, z.ParentRef))
			continue
		}
		if zoneParentCycle(z.Ref, byRef) {
			reasons = append(reasons, fmt.Sprintf("zone %q: parent_ref chain forms a cycle", z.Ref))
		}
	}
	if len(zones) == 0 {
		return reasons
	}
	if len(policies) == 0 {
		reasons = append(reasons, "at least one policy row with outside_zones_note is required while delivery zones exist")
	}
	for _, p := range policies {
		if strings.TrimSpace(p.DeliveryCost) != "" || strings.TrimSpace(p.DeliveryInDays) != "" {
			reasons = append(reasons, "policy: delivery_cost and delivery_in_days must be blank while delivery zones exist — set cost/days per zone instead")
		}
		if strings.TrimSpace(p.OutsideZonesNote) == "" {
			reasons = append(reasons, "policy: outside_zones_note is required while delivery zones exist")
		}
	}
	return reasons
}

// zoneParentCycle walks the parent_ref chain from ref upward, reporting true
// the moment a zone re-appears in its own ancestry (a dangling — as opposed
// to cyclic — parent_ref is reported separately by the caller, so this never
// needs to treat a missing ref as a cycle).
func zoneParentCycle(ref string, byRef map[string]ZoneRow) bool {
	visited := map[string]bool{ref: true}
	cur := byRef[ref].ParentRef
	for cur != "" {
		if visited[cur] {
			return true
		}
		visited[cur] = true
		next, ok := byRef[cur]
		if !ok {
			return false
		}
		cur = next.ParentRef
	}
	return false
}
