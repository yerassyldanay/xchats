package dbtest

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

const salonTestOrgSQL = `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ('44444444-4444-4444-4444-444444444444', 'Salon Test Org', 1791244800000, 1791244800000)`

// ai_contacts.schedule is plain TEXT holding a JSON document: the database does not
// validate it (Go does, on write), and the bytes come back exactly as stored.
func TestBaseline_ContactsScheduleRoundTripsAsText(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)
	const schedule = `[{"ref":"mon","day":"Понедельник","start":"10:00","end":"19:00","breaks":[]}]`
	mustExec(t, db, ctx, `INSERT INTO ai_contacts (id, organization_id, booking_url, schedule, created_at, updated_at)
		VALUES ($1, '44444444-4444-4444-4444-444444444444', 'https://example.com/book', $2, $3, $3)`, uuid.New(), schedule, now)

	var got string
	if err := db.QueryRow(ctx, `SELECT schedule FROM ai_contacts WHERE organization_id = '44444444-4444-4444-4444-444444444444'`).Scan(&got); err != nil {
		t.Fatalf("read back schedule: %v", err)
	}
	if got != schedule {
		t.Errorf("schedule = %q, want the stored bytes %q", got, schedule)
	}
}

func TestBaseline_SpecialistsTableShape(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)

	wantID := uuid.NewString()
	var id, salesStatus string
	if err := db.QueryRow(ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name, id, created_at, updated_at)
		VALUES ('44444444-4444-4444-4444-444444444444', 'alina-kim', 'Алина Ким', $1, $2, $2)
		RETURNING id, sales_status`, wantID, now).Scan(&id, &salesStatus); err != nil {
		t.Fatalf("insert with only required columns: %v", err)
	}
	if id != wantID {
		t.Errorf("id = %q, want the caller-supplied %q (the database generates none)", id, wantID)
	}
	if salesStatus != "active" {
		t.Errorf("sales_status default = %q, want active", salesStatus)
	}

	// UNIQUE(organization_id, ref).
	if _, err := db.Exec(ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name, id, created_at, updated_at)
		VALUES ('44444444-4444-4444-4444-444444444444', 'alina-kim', 'Дубликат', $1, $2, $2)`, uuid.New(), now); !dbx.IsUniqueViolation(err) {
		t.Fatalf("duplicate (organization_id, ref): err = %v, want a uniqueness violation", err)
	}

	// The same ref in a DIFFERENT organization is fine — org-scoped uniqueness only.
	mustExec(t, db, ctx, `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ('55555555-5555-5555-5555-555555555555', 'Other Org', $1, $1)`, now)
	mustExec(t, db, ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name, id, created_at, updated_at)
		VALUES ('55555555-5555-5555-5555-555555555555', 'alina-kim', 'Тёзка из другого салона', $1, $2, $2)`, uuid.New(), now)

	var schedule, portfolio string
	if err := db.QueryRow(ctx, `SELECT schedule, portfolio_images FROM ai_specialists WHERE id = $1`, id).Scan(&schedule, &portfolio); err != nil {
		t.Fatalf("read back defaults: %v", err)
	}
	if schedule != "[]" || portfolio != "[]" {
		t.Errorf("schedule/portfolio_images defaults = %q/%q, want '[]'/'[]'", schedule, portfolio)
	}
}

func TestBaseline_ServicesTableShape(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)

	var id, serviceType, salesStatus, parentRef string
	if err := db.QueryRow(ctx, `INSERT INTO ai_services (organization_id, ref, name, id, created_at, updated_at)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-women', 'Женская стрижка', $1, $2, $2)
		RETURNING id, service_type, sales_status, parent_ref`, uuid.New(), now).Scan(&id, &serviceType, &salesStatus, &parentRef); err != nil {
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

	mustExec(t, db, ctx, `INSERT INTO ai_services (organization_id, ref, parent_ref, service_type, name, duration, specialist_refs, id, created_at, updated_at)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-short', 'haircut-women', 'variant', 'Короткая стрижка', 45, '["alina-kim"]', $1, $2, $2)`, uuid.New(), now)

	// UNIQUE(organization_id, ref).
	if _, err := db.Exec(ctx, `INSERT INTO ai_services (organization_id, ref, name, id, created_at, updated_at)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-women', 'Дубликат', $1, $2, $2)`, uuid.New(), now); !dbx.IsUniqueViolation(err) {
		t.Fatalf("duplicate (organization_id, ref): err = %v, want a uniqueness violation", err)
	}
}

// TestBaseline_OrganizationCascadeDelete mirrors every other ai_*
// table's ON DELETE CASCADE from organizations (see ai_products in
// 20261006000003_ai_knowledge_base.sql) — deleting an organization must remove its
// specialists and services, not leave them orphaned.
func TestBaseline_OrganizationCascadeDelete(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, salonTestOrgSQL)
	mustExec(t, db, ctx, `INSERT INTO ai_specialists (organization_id, ref, full_name, id, created_at, updated_at)
		VALUES ('44444444-4444-4444-4444-444444444444', 'alina-kim', 'Алина Ким', $1, $2, $2)`, uuid.New(), now)
	mustExec(t, db, ctx, `INSERT INTO ai_services (organization_id, ref, name, id, created_at, updated_at)
		VALUES ('44444444-4444-4444-4444-444444444444', 'haircut-women', 'Женская стрижка', $1, $2, $2)`, uuid.New(), now)

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
