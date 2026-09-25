package kbstore

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// salon_validate.go is facts_validate.go's counterpart for the beauty-salon
// vertical's two catalog entities (PLAN.md "Beauty Salon Knowledge Base
// Extension"): the shared write-time rules UpsertSpecialist/PutLiveSpecialist/
// MCPUpsertSpecialist and UpsertService/PutLiveService/MCPUpsertService all
// call identically, so the three lanes can never drift (the same
// belt-and-suspenders relationship facts_validate.go's validateProductFacts
// has with its own three callers). aiprompt.BuildCatalog
// (buildSpecialistFacts/buildServiceFacts) re-enforces the SAME rules again
// at prompt-render time — defense in depth, never the only gate.

// refSlugPattern mirrors aiprompt's own private refPattern (catalog.go) —
// the closed natural-ref shape validRef enforces on every Product/Tariff/
// DeliveryZone/Specialist/Service ref at BuildCatalog time. kbstore itself
// has no equivalent check today for Product/Tariff/DeliveryZone refs (they
// are either derived via slugify, mcp_write.go, or accepted verbatim and
// only ever validated this strictly once BuildCatalog runs at prompt-render
// time) — duplicated here as an explicit second literal, the same
// productConcreteRefs/tariffConcreteRefs duplicate aiprompt's own unexported
// slices (facts_validate.go's doc comment), so a bad specialist/service ref
// is rejected at WRITE time instead of only being discovered later when a
// customer reply is being rendered (PLAN.md: "Service refs use lowercase
// dash-separated slugs such as manicure-french").
var refSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func validSalonRef(field, ref string) error {
	if !refSlugPattern.MatchString(ref) {
		return fmt.Errorf("kbstore: %s ref %q must match %s", field, ref, refSlugPattern.String())
	}
	return nil
}

// currentSpecialistIfAny resolves one specialist's merged current shape by
// ref — the pending blob entry if one exists, else the live row, else
// ok=false. Unlike currentSpecialist/currentService (draft.go/salon.go),
// whose callers are about to WRITE that exact ref and want a blank scaffold
// for "not found," this is used to validate a CROSS-REFERENCE from a
// DIFFERENT record (a service's specialist_refs) — "not found" is itself
// the validation failure there, so it must be reported explicitly rather
// than papered over with a scaffold. A specialist staged for deletion in
// b.Deletes still resolves here, the same way currentProduct/currentTariff
// never filter by delete markers either — see this file's own doc comment.
func (s *Store) currentSpecialistIfAny(ctx context.Context, db dbtx, orgID uuid.UUID, ref string, b *DraftBlob) (DraftSpecialist, bool, error) {
	for _, sp := range b.Specialists {
		if sp.Ref == ref {
			return sp, true, nil
		}
	}
	var sp DraftSpecialist
	err := db.QueryRow(ctx, `SELECT ref, full_name, title, experience, schedule, booking_url, portfolio_images, sales_status
		FROM ai_specialists WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&sp.Ref, &sp.FullName, &sp.Title, &sp.Experience, (*aiprompt.ScheduleColumn)(&sp.Schedule), &sp.BookingURL,
			(*dbx.UUIDArray)(&sp.PortfolioImages), &sp.SalesStatus)
	if errors.Is(err, dbx.ErrNoRows) {
		return DraftSpecialist{}, false, nil
	}
	if err != nil {
		return DraftSpecialist{}, false, err
	}
	return sp, true, nil
}

// currentServiceIfAny is currentSpecialistIfAny's twin for ai_services — see
// its doc comment.
func (s *Store) currentServiceIfAny(ctx context.Context, db dbtx, orgID uuid.UUID, ref string, b *DraftBlob) (DraftService, bool, error) {
	for _, sv := range b.Services {
		if sv.Ref == ref {
			return sv, true, nil
		}
	}
	var sv DraftService
	err := db.QueryRow(ctx, `SELECT ref, parent_ref, service_type, category, name, price, duration, description, specialist_refs, sales_status
		FROM ai_services WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&sv.Ref, &sv.ParentRef, &sv.ServiceType, &sv.Category, &sv.Name, &sv.Price, &sv.Duration, &sv.Description,
			(*dbx.StringArray)(&sv.SpecialistRefs), &sv.SalesStatus)
	if errors.Is(err, dbx.ErrNoRows) {
		return DraftService{}, false, nil
	}
	if err != nil {
		return DraftService{}, false, err
	}
	return sv, true, nil
}

// validateSpecialist enforces every write-time rule a specialist's ref/
// schedule/sales_status must satisfy, and returns sv with Schedule replaced
// by aiprompt.NormalizeSchedule's canonical result — the NORMALIZED value is
// what every lane stores, never the caller's raw input (PLAN.md's schedule
// validation rules; aiprompt.NormalizeSchedule's own doc comment). Media
// (PortfolioImages) is validated separately by the caller via
// validateMediaRefs, same as every other entity's media columns — this
// function only covers the scalar/enum/schedule rules.
func validateSpecialist(sv DraftSpecialist) (DraftSpecialist, error) {
	if err := validSalonRef("specialist", sv.Ref); err != nil {
		return DraftSpecialist{}, err
	}
	if err := validateEnum("sales_status", sv.SalesStatus, "active", "inactive"); err != nil {
		return DraftSpecialist{}, err
	}
	schedule, err := aiprompt.NormalizeSchedule(sv.Schedule)
	if err != nil {
		return DraftSpecialist{}, fmt.Errorf("kbstore: specialist %s: %w", sv.Ref, err)
	}
	sv.Schedule = schedule
	return sv, nil
}

// validateService enforces PLAN.md's service-hierarchy invariants — the
// same rules aiprompt.buildServiceFacts (catalog.go) encodes at
// prompt-render time, checked here at write time against the CURRENT merged
// view (db, overlaid by b — an empty *DraftBlob for the live lane, which has
// no blob overlay of its own; see PutLiveService):
//   - service_type is exactly one of base/variant/addon.
//   - a base service has no parent_ref.
//   - a variant/addon's parent_ref is non-empty and resolves, in the
//     current merged view, to an existing service whose service_type is
//     base — only one hierarchy level is supported.
//   - duration is nil, or a positive minute count.
//   - every specialist_refs entry resolves, in the current merged view, to
//     an EXISTING specialist row in the same organization — regardless of
//     that specialist's own sales_status (PLAN.md: archiving a specialist
//     later must not retroactively invalidate a service's specialist_refs;
//     only creation/edit time is checked here).
//
// Returns sv unchanged (schedule has no place on a service) — the return
// value exists only so this reads symmetrically with validateSpecialist at
// each of its three call sites.
func (s *Store) validateService(ctx context.Context, db dbtx, orgID uuid.UUID, b *DraftBlob, sv DraftService) (DraftService, error) {
	if err := validSalonRef("service", sv.Ref); err != nil {
		return DraftService{}, err
	}
	if err := validateEnum("service_type", sv.ServiceType, "base", "variant", "addon"); err != nil {
		return DraftService{}, err
	}
	if err := validateEnum("sales_status", sv.SalesStatus, "active", "inactive"); err != nil {
		return DraftService{}, err
	}
	if sv.ServiceType == "base" {
		if sv.ParentRef != "" {
			return DraftService{}, fmt.Errorf("kbstore: base service %q must not have a parent_ref", sv.Ref)
		}
	} else {
		if sv.ParentRef == "" {
			return DraftService{}, fmt.Errorf("kbstore: %s service %q requires a parent_ref", sv.ServiceType, sv.Ref)
		}
		parent, ok, err := s.currentServiceIfAny(ctx, db, orgID, sv.ParentRef, b)
		if err != nil {
			return DraftService{}, err
		}
		if !ok {
			return DraftService{}, fmt.Errorf("kbstore: service %q references unknown parent_ref %q", sv.Ref, sv.ParentRef)
		}
		if parent.ServiceType != "base" {
			return DraftService{}, fmt.Errorf("kbstore: service %q's parent_ref %q is not a base service — only one hierarchy level is supported", sv.Ref, sv.ParentRef)
		}
	}
	if sv.Duration != nil && *sv.Duration <= 0 {
		return DraftService{}, fmt.Errorf("kbstore: service %q: duration must be a positive minute count", sv.Ref)
	}
	for _, ref := range sv.SpecialistRefs {
		if _, ok, err := s.currentSpecialistIfAny(ctx, db, orgID, ref, b); err != nil {
			return DraftService{}, err
		} else if !ok {
			return DraftService{}, fmt.Errorf("kbstore: service %q references unknown specialist_ref %q", sv.Ref, ref)
		}
	}
	return sv, nil
}
