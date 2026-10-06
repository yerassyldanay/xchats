package dbtest

import (
	"context"
	"testing"
)

const salonTestOrgSQL = `INSERT INTO organizations (id, name) VALUES ('44444444-4444-4444-4444-444444444444', 'Salon Test Org')`

func TestBaseline_ContactsSchemaValidJSON(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)
	if _, err := db.Exec(ctx, `INSERT INTO ai_contacts (organization_id, schedule) VALUES ('44444444-4444-4444-4444-444444444444', 'not json')`); err == nil {
		t.Fatal("want an error inserting invalid JSON into ai_contacts.schedule")
	}
	mustExec(t, db, ctx, `INSERT INTO ai_contacts (organization_id, booking_url, schedule)
		VALUES ('44444444-4444-4444-4444-444444444444', 'https://example.com/book', '[{"ref":"mon","day":"Понедельник","start":"10:00","end":"19:00","breaks":[]}]')`)
}

func TestBaseline_SpecialistsTableShape(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)

	var id, salesStatus string
	if err := db.QueryRow(ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name)
		VALUES ('44444444-4444-4444-4444-444444444444', 'alina-kim', 'Алина Ким')
		RETURNING id, sales_status`).Scan(&id, &salesStatus); err != nil {
		t.Fatalf("insert with only required columns: %v", err)
	}
	if id == "" {
		t.Error("want a generated id")
	}
	if salesStatus != "active" {
		t.Errorf("sales_status default = %q, want active", salesStatus)
	}

	// UNIQUE(organization_id, ref).
	if _, err := db.Exec(ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name)
		VALUES ('44444444-4444-4444-4444-444444444444', 'alina-kim', 'Дубликат')`); err == nil {
		t.Fatal("want a duplicate (organization_id, ref) to be rejected")
	}

	// The same ref in a DIFFERENT organization is fine — org-scoped uniqueness only.
	mustExec(t, db, ctx, `INSERT INTO organizations (id, name) VALUES ('55555555-5555-5555-5555-555555555555', 'Other Org')`)
	mustExec(t, db, ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name)
		VALUES ('55555555-5555-5555-5555-555555555555', 'alina-kim', 'Тёзка из другого салона')`)

	// json_valid CHECK on schedule/portfolio_images.
	if _, err := db.Exec(ctx, `INSERT INTO ai_specialists (organization_id, ref, schedule)
		VALUES ('44444444-4444-4444-4444-444444444444', 'bad-schedule', 'not json')`); err == nil {
		t.Fatal("want invalid JSON in schedule to be rejected")
	}
	if _, err := db.Exec(ctx, `INSERT INTO ai_specialists (organization_id, ref, portfolio_images)
		VALUES ('44444444-4444-4444-4444-444444444444', 'bad-portfolio', 'not json')`); err == nil {
		t.Fatal("want invalid JSON in portfolio_images to be rejected")
	}

	var schedule, portfolio string
	if err := db.QueryRow(ctx, `SELECT schedule, portfolio_images FROM ai_specialists WHERE id = $1`, id).Scan(&schedule, &portfolio); err != nil {
		t.Fatalf("read back defaults: %v", err)
	}
	if schedule != "[]" || portfolio != "[]" {
		t.Errorf("schedule/portfolio_images defaults = %q/%q, want '[]'/'[]'", schedule, portfolio)
	}
}

func TestBaseline_ServicesTableShape(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)

	var id, serviceType, salesStatus, parentRef string
	if err := db.QueryRow(ctx, `INSERT INTO ai_services (organization_id, ref, name)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-women', 'Женская стрижка')
		RETURNING id, service_type, sales_status, parent_ref`).Scan(&id, &serviceType, &salesStatus, &parentRef); err != nil {
		t.Fatalf("insert with only required columns: %v", err)
	}
	if serviceType != "base" {
		t.Errorf("service_type default = %q, want base", serviceType)
	}
	if salesStatus != "active" {
		t.Errorf("sales_status default = %q, want active", salesStatus)
	}
	if parentRef != "" {
		t.Errorf("parent_ref default = %q, want empty string", parentRef)
	}

	// duration is nullable.
	var duration *int
	if err := db.QueryRow(ctx, `SELECT duration FROM ai_services WHERE id = $1`, id).Scan(&duration); err != nil {
		t.Fatalf("read back duration: %v", err)
	}
	if duration != nil {
		t.Errorf("duration default = %v, want NULL", *duration)
	}

	mustExec(t, db, ctx, `INSERT INTO ai_services (organization_id, ref, parent_ref, service_type, name, duration, specialist_refs)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-short', 'haircut-women', 'variant', 'Короткая стрижка', 45, '["alina-kim"]')`)

	// UNIQUE(organization_id, ref).
	if _, err := db.Exec(ctx, `INSERT INTO ai_services (organization_id, ref, name)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-women', 'Дубликат')`); err == nil {
		t.Fatal("want a duplicate (organization_id, ref) to be rejected")
	}

	// json_valid CHECK on specialist_refs.
	if _, err := db.Exec(ctx, `INSERT INTO ai_services (organization_id, ref, specialist_refs)
		VALUES ('44444444-4444-4444-4444-444444444444', 'bad-refs', 'not json')`); err == nil {
		t.Fatal("want invalid JSON in specialist_refs to be rejected")
	}
}

// TestBaseline_OrganizationCascadeDelete mirrors every other ai_*
// table's ON DELETE CASCADE from organizations (see ai_products in
// 20261006000003_ai_knowledge_base.sql) — deleting an organization must remove its
// specialists and services, not leave them orphaned.
func TestBaseline_OrganizationCascadeDelete(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)
	mustExec(t, db, ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name)
		VALUES ('44444444-4444-4444-4444-444444444444', 'alina-kim', 'Алина Ким')`)
	mustExec(t, db, ctx, `INSERT INTO ai_services (organization_id, ref, name)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-women', 'Женская стрижка')`)

	mustExec(t, db, ctx, `DELETE FROM organizations WHERE id = '44444444-4444-4444-4444-444444444444'`)

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_specialists WHERE organization_id = '44444444-4444-4444-4444-444444444444'`).Scan(&n); err != nil {
		t.Fatalf("count specialists after org delete: %v", err)
	}
	if n != 0 {
		t.Errorf("expected ON DELETE CASCADE to remove specialists, %d remain", n)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_services WHERE organization_id = '44444444-4444-4444-4444-444444444444'`).Scan(&n); err != nil {
		t.Fatalf("count services after org delete: %v", err)
	}
	if n != 0 {
		t.Errorf("expected ON DELETE CASCADE to remove services, %d remain", n)
	}
}
