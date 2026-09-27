package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// TestKBFail_ClassifiesErrors is CodeRabbit PR #119 discussion_r4110715047's
// direct regression test for kbFail itself, isolated from any DB/HTTP
// wiring: kbFail must map ErrInvalidEnumValue and ErrSalonValidation (the
// caller-input validation failures salon_validate.go/live.go now return) to
// HTTP 422, while an arbitrary error — the stand-in for a genuine
// DB/transaction/infrastructure failure — must still fall through to the
// 500 default. Before this fix, only GateError/ErrMediaReference were
// classified here; every salon-domain validation error, and every
// ErrInvalidEnumValue, fell into that same 500 default alongside a real
// infrastructure failure.
func TestKBFail_ClassifiesErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{}

	callKBFail := func(err error) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
		s.kbFail(c, err)
		return w.Code
	}

	t.Run("ErrInvalidEnumValue is 422", func(t *testing.T) {
		err := &kbstore.ErrInvalidEnumValue{Field: "sales_status", Value: "bogus", Allowed: []string{"active", "inactive"}}
		if got := callKBFail(err); got != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", got, http.StatusUnprocessableEntity)
		}
	})

	// ErrSalonValidation itself has an unexported field (salon_validate.go's
	// own doc comment: it is deliberately never hand-constructed, only
	// produced at a genuine validateService/validateSpecialist/
	// PatchLiveContacts rule-violation call site), so it cannot be built
	// here in isolation — kb_salon_validation_test.go proves kbFail's
	// handling of a REAL one, end to end through an actual HTTP request.
	// internal/kbstore's own salon_test.go proves every required scenario
	// (invalid schedule, active child under inactive base, deleting a base
	// with children, ...) produces exactly this type in the first place.

	t.Run("an arbitrary/unexpected error is still 500", func(t *testing.T) {
		err := errors.New("pq: connection reset by peer")
		if got := callKBFail(err); got != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d — an unrecognized error must never be reported as a client validation failure", got, http.StatusInternalServerError)
		}
	})

	t.Run("ErrUnknownKind is still 400 (unrelated existing case, unaffected)", func(t *testing.T) {
		if got := callKBFail(kbstore.ErrUnknownKind); got != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", got, http.StatusBadRequest)
		}
	})

	t.Run("ErrStale is still 409 (unrelated existing case, unaffected)", func(t *testing.T) {
		if got := callKBFail(kbstore.ErrStale); got != http.StatusConflict {
			t.Fatalf("status = %d, want %d", got, http.StatusConflict)
		}
	})
}
