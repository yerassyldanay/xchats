package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/automation"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// orgAnyAccount resolves an account id and enforces org ownership, on ANY
// channel — unlike orgAccount (accounts.go), which deliberately excludes
// Telegram because it backs WhatsApp-only pairing/logout/status routes.
// Automation applies uniformly to every channel, so this guard must not
// exclude one.
func (s *Server) orgAnyAccount(c *gin.Context, id uuid.UUID) (store.Account, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.Account{}, false
	}
	acct, err := s.store.AccountByID(ctx(c), id)
	if err != nil || !acct.OrganizationID.Valid || acct.OrganizationID.UUID != org.ID {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return store.Account{}, false
	}
	return acct, true
}

type scheduleWindowReq struct {
	Weekday     int `json:"weekday"`
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

type updateAutomationReq struct {
	Mode string `json:"mode"`
	// WaitSecondsOverride is a double pointer's usual JSON trick made
	// explicit: absent from the body -> nil (leave the override
	// unchanged... except this endpoint always replaces the full settings
	// row, so "absent" and "null" are treated identically — clear the
	// override, use the system default). Present with a number -> that
	// override.
	WaitSecondsOverride *int                `json:"wait_seconds_override"`
	Schedule            []scheduleWindowReq `json:"schedule"`
}

// handleUpdateAccountAutomation is PUT /accounts/:id/automation: replaces a
// channel's mode, wait-time override, and full recurring UTC schedule in one
// call. Every window must already be in UTC — converting from the browser's
// local time zone is the frontend's job (see frontend/src/lib/schedule.ts),
// so this handler only validates ranges, never shifts anything.
func (s *Server) handleUpdateAccountAutomation(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAnyAccount(c, id)
	if !okAcct {
		return
	}
	var req updateAutomationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}
	if !automation.ValidMode(req.Mode) {
		fail(c, http.StatusBadRequest, ErrValidation, "mode must be one of: off, suggestions, scheduled_auto")
		return
	}
	if err := automation.ValidateWaitOverride(req.WaitSecondsOverride); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	windows := make([]store.AutomationWindowInput, 0, len(req.Schedule))
	for i, w := range req.Schedule {
		win := automation.Window{Weekday: time.Weekday(w.Weekday), StartMinute: w.StartMinute, EndMinute: w.EndMinute}
		if err := automation.ValidateWindow(win); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "schedule["+strconv.Itoa(i)+"]: "+err.Error())
			return
		}
		windows = append(windows, store.AutomationWindowInput{Weekday: w.Weekday, StartMinute: w.StartMinute, EndMinute: w.EndMinute})
	}

	settings, savedWindows, err := s.store.SetAutomationSettings(ctx(c), acct.ID, req.Mode, req.WaitSecondsOverride, windows)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	item := dto.MapNeutralAccount(acct, "")
	item.Automation = dto.MapAccountAutomation(settings, savedWindows, s.cfg.System.CustomerMessageWaitSeconds)
	ok(c, item)
}
