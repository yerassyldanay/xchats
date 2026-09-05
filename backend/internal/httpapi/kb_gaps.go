package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// kbGapEventView is one ai_kb_gap_events row as GET /kb/gaps returns it —
// deliberately never the customer-facing draft/message text: only the
// diagnostic metadata an operator needs to see WHAT kind of gap happened
// and on WHICH KB entity, never WHAT the customer or the assistant actually
// said. EscalationReason is the model/engine's own short internal note
// (diagnostic only, never customer-facing — see aiprompt.Response's doc
// comment on escalation_reason), kept here for operator context.
type kbGapEventView struct {
	ID               string    `json:"id"`
	Channel          string    `json:"channel"`
	ChatID           string    `json:"chat_id"`
	DraftID          string    `json:"draft_id,omitempty"`
	ReasonCode       string    `json:"reason_code"`
	TargetEntityType string    `json:"target_entity_type,omitempty"`
	TargetEntityRef  string    `json:"target_entity_ref,omitempty"`
	MissingFields    []string  `json:"missing_fields,omitempty"`
	EscalationReason string    `json:"escalation_reason,omitempty"`
	Source           string    `json:"source"`
	CreatedAt        time.Time `json:"created_at"`
}

type kbGapReasonCountView struct {
	ReasonCode string `json:"reason_code"`
	Count      int    `json:"count"`
}

// kbGapReportView is GET /kb/gaps' payload: Counts is the default report
// (genuine content gaps only, aiprompt.DefaultReportReasonCodes,
// zero-filled); OperationalCounts keeps unsupported_request/
// human_requested/engine_error/other distinguishable rather than either
// hidden or blended into Counts; Recent is a bounded, newest-first page of
// representative events under the same filter.
type kbGapReportView struct {
	Counts            []kbGapReasonCountView `json:"counts"`
	OperationalCounts []kbGapReasonCountView `json:"operational_counts"`
	Recent            []kbGapEventView       `json:"recent"`
}

// handleKBGaps answers GET /kb/gaps: aggregated counts plus a bounded page
// of recent representative events for the current organization, filtered by
// date range (from/to, RFC3339), reason code, and entity type/ref. Every
// filter is optional; an unparseable from/to or limit is silently ignored
// (treated as absent) rather than rejected, matching this codebase's
// existing best-effort query-param convention (see handleListMessages'
// "before").
func (s *Server) handleKBGaps(c *gin.Context) {
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}

	filter := store.KBGapFilter{
		OrgID:            orgID,
		ReasonCode:       c.Query("reason"),
		TargetEntityType: c.Query("entity_type"),
		TargetEntityRef:  c.Query("entity_ref"),
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = &t
		}
	}
	if limit, err := strconv.Atoi(c.Query("limit")); err == nil {
		filter.Limit = limit
	}

	report, err := s.store.KBGapReportFor(ctx(c), filter)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, mapKBGapReport(report))
}

func mapKBGapReport(r store.KBGapReport) kbGapReportView {
	return kbGapReportView{
		Counts:            mapKBGapCounts(r.Counts),
		OperationalCounts: mapKBGapCounts(r.OperationalCounts),
		Recent:            mapKBGapEvents(r.Recent),
	}
}

func mapKBGapCounts(cs []store.KBGapReasonCount) []kbGapReasonCountView {
	out := make([]kbGapReasonCountView, len(cs))
	for i, rc := range cs {
		out[i] = kbGapReasonCountView{ReasonCode: rc.ReasonCode, Count: rc.Count}
	}
	return out
}

func mapKBGapEvents(es []store.KBGapEvent) []kbGapEventView {
	out := make([]kbGapEventView, 0, len(es))
	for _, e := range es {
		v := kbGapEventView{
			ID: e.ID, Channel: e.Channel, ChatID: e.ChatID.String(),
			ReasonCode: e.ReasonCode, TargetEntityType: e.TargetEntityType, TargetEntityRef: e.TargetEntityRef,
			MissingFields: e.MissingFields, EscalationReason: e.EscalationReason,
			Source: e.Source, CreatedAt: e.CreatedAt,
		}
		if e.DraftID.Valid {
			v.DraftID = e.DraftID.UUID.String()
		}
		out = append(out, v)
	}
	return out
}
