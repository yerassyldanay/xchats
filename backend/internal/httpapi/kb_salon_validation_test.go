package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// kb_salon_validation_test.go is CodeRabbit PR #119 discussion_r4110715047's
// end-to-end regression coverage: /kb/specialists, /kb/services, and
// /kb/contacts must answer 422 (VALIDATION_ERROR) for a salon-domain or
// schedule validation failure, not the 500 every one of these used to fall
// through to before kbFail (httpapi/playground.go) learned to classify
// kbstore.ErrInvalidEnumValue and kbstore.ErrSalonValidation. These run the
// full stack (real HTTP request, real gin routing, real kbstore.Store) —
// internal/kbstore's own salon_test.go already proves the error TYPE each
// store method returns for these same scenarios; this file proves the HTTP
// status code an actual client receives for them.

type specialistPayload struct {
	Ref         string            `json:"ref"`
	FullName    string            `json:"full_name"`
	Schedule    aiprompt.Schedule `json:"schedule"`
	SalesStatus string            `json:"sales_status"`
}

type servicePayload struct {
	Ref            string   `json:"ref"`
	ParentRef      string   `json:"parent_ref,omitempty"`
	ServiceType    string   `json:"service_type"`
	Name           string   `json:"name"`
	SpecialistRefs []string `json:"specialist_refs,omitempty"`
	SalesStatus    string   `json:"sales_status"`
}

type contactsPayload struct {
	Schedule aiprompt.Schedule `json:"schedule"`
}

func TestKBUpsertSpecialist_InvalidScheduleReturns422(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/kb/specialists", specialistPayload{
		Ref: "bad-schedule", FullName: "Test",
		Schedule:    aiprompt.Schedule{{Ref: "mon", Start: "10:00", End: "09:00"}}, // overnight: invalid
		SalesStatus: "active",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode, http.StatusUnprocessableEntity, env["message"])
	}
	if errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("errcode = %q, want VALIDATION_ERROR", errcodeOf(env))
	}
}

func TestKBUpsertService_InvalidServiceTypeReturns422(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/kb/services", servicePayload{
		Ref: "x1", ServiceType: "combo", Name: "Bad type", SalesStatus: "active",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode, http.StatusUnprocessableEntity, env["message"])
	}
	if errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("errcode = %q, want VALIDATION_ERROR", errcodeOf(env))
	}
}

func TestKBUpsertService_ActiveChildUnderInactiveBaseReturns422(t *testing.T) {
	h := newHarness(t)
	mustCreateService(t, h, servicePayload{Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка", SalesStatus: "inactive"})

	resp, env := h.postJSON("/xchats/api/v1/kb/services", servicePayload{
		Ref: "haircut-short", ParentRef: "haircut-women", ServiceType: "variant", Name: "Короткая", SalesStatus: "active",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode, http.StatusUnprocessableEntity, env["message"])
	}
	if errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("errcode = %q, want VALIDATION_ERROR", errcodeOf(env))
	}
}

func TestKBDeleteService_WithChildrenReturns422(t *testing.T) {
	h := newHarness(t)
	mustCreateService(t, h, servicePayload{Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка", SalesStatus: "active"})
	mustCreateService(t, h, servicePayload{Ref: "haircut-short", ParentRef: "haircut-women", ServiceType: "variant", Name: "Короткая", SalesStatus: "active"})

	status, env := h.del("/xchats/api/v1/kb/services/haircut-women")
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body=%s", status, http.StatusUnprocessableEntity, env["message"])
	}
	if errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("errcode = %q, want VALIDATION_ERROR", errcodeOf(env))
	}
}

func TestKBPatchContacts_InvalidScheduleReturns422(t *testing.T) {
	h := newHarness(t)
	resp, env := h.patchJSON("/xchats/api/v1/kb/contacts", contactsPayload{
		Schedule: aiprompt.Schedule{{
			Ref: "mon", Start: "10:00", End: "18:00",
			Breaks: []aiprompt.ScheduleBreak{{Start: "13:00", End: "14:00"}, {Start: "13:30", End: "14:30"}}, // overlapping
		}},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode, http.StatusUnprocessableEntity, env["message"])
	}
	if errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("errcode = %q, want VALIDATION_ERROR", errcodeOf(env))
	}
}

func mustCreateService(t *testing.T, h *harness, p servicePayload) {
	t.Helper()
	resp, env := h.postJSON("/xchats/api/v1/kb/services", p)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("seed PutLiveService(%s): status=%d body=%s", p.Ref, resp.StatusCode, env["message"])
	}
}
