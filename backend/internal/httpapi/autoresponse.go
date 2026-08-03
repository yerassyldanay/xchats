package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/autoresponse"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
)

type autoResponseWindowReq struct {
	Weekday int    `json:"weekday"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

type putAutoResponseReq struct {
	Enabled           bool                    `json:"enabled"`
	ReplyMode         string                  `json:"reply_mode"`
	FixedText         string                  `json:"fixed_text"`
	Timezone          string                  `json:"timezone"`
	DelaySeconds      int                     `json:"delay_seconds"`
	CooldownSeconds   int                     `json:"cooldown_seconds"`
	SkipWhenEscalated bool                    `json:"skip_when_escalated"`
	PauseWhenAssigned bool                    `json:"pause_when_assigned"`
	Windows           []autoResponseWindowReq `json:"windows"`
}

// handlePutAutoResponse writes an account's auto-response policy. The route
// is channel-neutral (orgAnyAccount, not orgAccount) — a Telegram bot needs
// this exactly as much as a WhatsApp number does. No rate limiting: the
// route sits behind requireSession, and toggling the policy creates no jobs
// by itself (arming happens on the next inbound message), so repeated calls
// have bounded effect.
func (s *Server) handlePutAutoResponse(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAnyAccount(c, id)
	if !okAcct {
		return
	}
	var req putAutoResponseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}

	policy, reason := parseAutoResponsePolicy(req)
	if reason != "" {
		fail(c, http.StatusBadRequest, ErrValidation, reason)
		return
	}

	u := currentUser(c)
	if err := s.store.UpsertAutoResponse(ctx(c), acct.ID, acct.OrganizationID.UUID, u.ID, policy); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"account_id": acct.ID.String(), "auto_response": dto.MapAutoResponse(policy)})
}

// parseAutoResponsePolicy validates req and converts it into a policy, or
// returns a non-empty human-readable rejection reason. Windows are parsed
// with autoresponse.ParseClock (so "24:00" is accepted) and run through
// autoresponse.Normalize before the "enabled needs coverage" check, so that
// check reflects actual coverage rather than raw submission count — a
// same-weekday wrap ("outside 09:00-18:00") still counts once it is split.
func parseAutoResponsePolicy(req putAutoResponseReq) (autoresponse.Policy, string) {
	p := autoresponse.Policy{
		Enabled:           req.Enabled,
		ReplyMode:         req.ReplyMode,
		FixedText:         strings.TrimSpace(req.FixedText),
		Timezone:          strings.TrimSpace(req.Timezone),
		DelaySeconds:      req.DelaySeconds,
		CooldownSeconds:   req.CooldownSeconds,
		SkipWhenEscalated: req.SkipWhenEscalated,
		PauseWhenAssigned: req.PauseWhenAssigned,
	}
	if p.ReplyMode != autoresponse.ReplyModeAI && p.ReplyMode != autoresponse.ReplyModeFixed {
		return p, `reply_mode must be "ai" or "fixed"`
	}
	if p.Timezone == "" {
		return p, "timezone is required"
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil {
		return p, "unknown timezone: " + p.Timezone
	}
	if p.DelaySeconds < 0 || p.DelaySeconds > 3600 {
		return p, "delay_seconds must be between 0 and 3600"
	}
	if p.CooldownSeconds < 0 || p.CooldownSeconds > 86400 {
		return p, "cooldown_seconds must be between 0 and 86400"
	}

	windows := make([]autoresponse.Window, 0, len(req.Windows))
	for _, w := range req.Windows {
		if w.Weekday < 0 || w.Weekday > 6 {
			return p, "weekday must be between 0 (Sunday) and 6 (Saturday)"
		}
		start, err := autoresponse.ParseClock(w.Start)
		if err != nil {
			return p, "invalid window start " + w.Start
		}
		end, err := autoresponse.ParseClock(w.End)
		if err != nil {
			return p, "invalid window end " + w.End
		}
		windows = append(windows, autoresponse.Window{Weekday: time.Weekday(w.Weekday), StartMin: start, EndMin: end})
	}
	p.Windows = autoresponse.Normalize(windows)

	if p.Enabled {
		if len(p.Windows) == 0 {
			return p, "enabled requires at least one schedule window"
		}
		if p.ReplyMode == autoresponse.ReplyModeFixed && p.FixedText == "" {
			return p, "fixed_text is required in fixed mode"
		}
	}
	return p, ""
}
