// campaign_tools.go implements the four read-only campaign/WhatsApp
// diagnostic tools (diagnose_campaign, campaign_recipients,
// whatsapp_connection_status, campaign_events). See each Tool builder's own
// doc comment for its contract. Every handler here:
//   - re-derives orgID from the verified Principal (dispatchToolsCall),
//     never from a tool argument;
//   - resolves the target campaign/account through an org-scoped store
//     lookup FIRST, returning the identical not-found tool error for a
//     truly-missing id and a cross-org id (never a distinguishable
//     response — see orgScopedCampaign/orgScopedAccount);
//   - returns masked identities and opaque ids only;
//   - never mutates application state (no tool in this file calls a store
//     write method, sends a message, retries a campaign, or reconnects an
//     account).
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	campaigndiag "github.com/yerassyldanay/xchats/backend/internal/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

const (
	toolDiagnoseCampaign         = "diagnose_campaign"
	toolCampaignRecipients       = "campaign_recipients"
	toolWhatsAppConnectionStatus = "whatsapp_connection_status"
	toolCampaignEvents           = "campaign_events"
)

// --- pagination (shared "opaque decimal string" cursor contract — see
// kbstore/mcp_read.go's own doc comment on why this project's KB tools use a
// plain offset cursor; these diagnostic tools follow the identical contract
// for consistency across every MCP tool this connector exposes) ------------

const (
	defaultDiagLimit = 50
	maxDiagLimit     = 100
)

func clampLimit(n int) int {
	if n <= 0 {
		return defaultDiagLimit
	}
	if n > maxDiagLimit {
		return maxDiagLimit
	}
	return n
}

func parseCursorOffset(cursor string) int {
	if cursor == "" {
		return 0
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func nextCursor(offset, count, total int) string {
	end := offset + count
	if end >= total {
		return ""
	}
	return strconv.Itoa(end)
}

// --- authorization helpers --------------------------------------------------

const errCampaignNotFound = "campaign not found or not accessible"
const errAccountNotFound = "account not found or not accessible"

// orgScopedCampaign resolves id under orgID, returning the SAME tool error
// for a nonexistent id and a cross-org one — a caller must never be able to
// distinguish "this campaign exists in another organization" from "this
// campaign does not exist at all" by response shape.
func orgScopedCampaign(ctx context.Context, st *store.Store, orgID, id uuid.UUID) (store.Campaign, error) {
	c, err := st.CampaignByIDForOrg(ctx, id, orgID)
	if err != nil {
		return store.Campaign{}, fmt.Errorf(errCampaignNotFound)
	}
	return c, nil
}

// orgScopedAccount mirrors orgScopedCampaign for an account id — the same
// ownership check internal/httpapi's orgAnyAccount performs, reimplemented
// here since mcpserver's Store dependency is the same *store.Store but this
// package does not import internal/httpapi.
func orgScopedAccount(ctx context.Context, st *store.Store, orgID, id uuid.UUID) (store.Account, error) {
	acct, err := st.AccountByID(ctx, id)
	if err != nil || !acct.OrganizationID.Valid || acct.OrganizationID.UUID != orgID {
		return store.Account{}, fmt.Errorf(errAccountNotFound)
	}
	return acct, nil
}

// --- shared formatting -----------------------------------------------------

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func tsPtrOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ts(*t)
}

// maskPhoneLikeIdentity keeps only the last 4 characters of a normalized
// identity (a bare phone number for WhatsApp/Simulator, a numeric chat id
// for other channels) — the same "last 4, mask the rest" convention
// internal/outbound.maskDestination already uses for logs, applied here to
// a tool RESPONSE instead: this connector's masked-identity default (see
// this feature's own authorization requirement) reuses that existing
// convention rather than inventing a second one.
func maskPhoneLikeIdentity(identity string) string {
	if len(identity) <= 4 {
		return strings.Repeat("*", len(identity))
	}
	return strings.Repeat("*", len(identity)-4) + identity[len(identity)-4:]
}

func nullUUIDStr(u uuid.NullUUID) any {
	if !u.Valid {
		return nil
	}
	return u.UUID.String()
}

func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// --- diagnose_campaign -------------------------------------------------

func diagnoseCampaignTool() Tool {
	return Tool{
		Name: toolDiagnoseCampaign,
		Description: "Diagnose one campaign: lifecycle status, non-overlapping recipient outcome counts " +
			"(pending/sending/sent=provider-accepted/failed/skipped/unknown), separate delivery/read counts, " +
			"failures grouped by structured reason, a retry summary, the sending account's connection history and " +
			"current status, and recent events — with observed facts, evidence-linked likely causes, and named " +
			"missing evidence kept separate. Read-only.",
		InputSchema: obj(map[string]any{
			"campaign_id": str("The campaign's id."),
		}, "campaign_id"),
		Annotations:  readOnlyAnnotations(),
		OutputSchema: diagnoseCampaignOutputSchema(),
	}
}

func diagnoseCampaignOutputSchema() map[string]any {
	countsSchema := obj(map[string]any{
		"pending": integer("Not yet attempted."),
		"sending": integer("Currently mid-attempt (should be transient; a large, stuck count across repeated observations may indicate a crashed worker)."),
		"sent":    integer("Provider accepted at least one attempt. This is NOT confirmed delivery — see delivery below."),
		"failed":  integer("Reached a terminal failure (permanent cause, or the retry ladder was exhausted). Includes unresolved/unknown-outcome recipients — see unknown_outcome."),
		"skipped": integer("Suppressed, usually because the recipient replied or was messaged out of band during the campaign."),
		// unknown_outcome intentionally overlaps with `failed` above (see its
		// own description) rather than being invented as a mutually
		// exclusive sixth bucket — see this tool's own field comment.
		"unknown_outcome": integer("Recipients whose most recent send attempt ended ambiguously (a timeout, or a crash-interrupted send) and were therefore NEVER automatically retried. Counted within `failed` above (the persisted recipient status), but broken out here so an unresolved outcome is never silently read as a confirmed failure or a success."),
	}, "pending", "sending", "sent", "failed", "skipped", "unknown_outcome")
	deliverySchema := obj(map[string]any{
		"delivered": integer("Recipients whose linked message has a provider delivery receipt (delivered or read). Always <= sent, and requires provider support — 0 does not mean confirmed non-delivery if this channel/account never reports receipts."),
		"read":      integer("Recipients whose linked message has a provider READ receipt specifically."),
	}, "delivered", "read")
	retrySchema := obj(map[string]any{
		"recovered":  integer("Now provider-accepted, but only after at least one prior failed attempt."),
		"scheduled":  integer("Currently pending with an automatic retry scheduled."),
		"exhausted":  integer("Failed because the bounded retry ladder was fully used."),
		"unresolved": integer("Same set as unknown_outcome above — an ambiguous outcome that was deliberately never auto-retried, repeated here under the retry-specific framing."),
	}, "recovered", "scheduled", "exhausted", "unresolved")
	return obj(map[string]any{
		"campaign_id":      str("Echoes the request."),
		"name":             str("Campaign name."),
		"status":           enumStr("Campaign lifecycle status.", "draft", "scheduled", "running", "paused", "completed", "failed", "cancelled"),
		"channel":          str("Sending channel."),
		"account_ids":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The sending account id(s). Always exactly one today — this application's campaigns support a single sending account per campaign — kept as an array so a future multi-sender campaign needs no schema change."},
		"created_at":       str("When the campaign was created (UTC, RFC 3339)."),
		"started_at":       map[string]any{"type": []string{"string", "null"}, "description": "When the campaign first became 'running', or null if it never has."},
		"completed_at":     map[string]any{"type": []string{"string", "null"}, "description": "Approximate: this application has no dedicated completion timestamp column. Set to the campaign's last-updated time ONLY when status is a terminal one (completed/failed/cancelled); null otherwise. See missing_evidence if you need an exact time."},
		"total_recipients": integer("Total recipients across every bucket in counts."),
		"counts":           countsSchema,
		"delivery":         deliverySchema,
		"failures_by_reason": map[string]any{
			"type":                 "object",
			"description":          "Currently-failed recipients grouped by their latest attempt's structured error code (see campaign_recipients tool for the code vocabulary). \"unknown\" here means a failed recipient with no attempt record at all (legacy data predating this diagnostics feature), never a guess.",
			"additionalProperties": map[string]any{"type": "integer"},
		},
		"retry_summary": retrySchema,
		"connection": obj(map[string]any{
			"current": whatsappConnectionSummarySchema(),
			"observed_during_campaign": map[string]any{
				"type":        "array",
				"description": "Connection history events for the sending account whose timestamp falls within this campaign's own active window (started_at through completed_at, or now if still running). Empty if the channel is not WhatsApp (connection history is only tracked for WhatsApp/Simulator accounts) or none occurred.",
				"items":       connectionEventSchema(),
			},
		}, "current", "observed_during_campaign"),
		"recent_events": map[string]any{
			"type":        "array",
			"description": "The most recent entries of this campaign's event timeline (see the campaign_events tool for the full, paginated history).",
			"items":       campaignEventSchema(),
		},
		"facts": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Observed, certain facts derived directly from persisted counts/events — never a guess.",
		},
		"likely_causes": map[string]any{
			"type": "array",
			"items": obj(map[string]any{
				"cause":                str("Plain-language candidate explanation."),
				"confidence":           enumStr("How strong the supporting evidence is.", "likely", "possible"),
				"supporting_event_ids": strArray("campaign_events ids this inference is based on."),
			}, "cause", "confidence"),
			"description": "Candidate explanations the evidence supports — ALWAYS time-correlated with the events they cite (e.g. a disconnect event is only cited as a cause for failures whose own timestamps fall within that outage), never inferred from the account's CURRENT state alone.",
		},
		"missing_evidence": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Named gaps: data this tool could not observe (e.g. no connection history before this feature existed, or an unavailable WhatsApp connection check) — never silently treated as \"nothing happened\".",
		},
		"observed_at": str("When this diagnosis was computed (UTC, RFC 3339) — every count/state above is a snapshot as of this moment, not a live subscription."),
	}, "campaign_id", "name", "status", "channel", "account_ids", "created_at", "total_recipients", "counts", "delivery",
		"failures_by_reason", "retry_summary", "connection", "recent_events", "facts", "likely_causes", "missing_evidence", "observed_at")
}

func whatsappConnectionSummarySchema() map[string]any {
	return obj(map[string]any{
		"account_id":  str("The account this status describes."),
		"state":       enumStr("Current connection state.", "connected", "disconnected", "logged_out", "unknown"),
		"freshness":   enumStr("Whether state was checked live just now, or is the last value this application had stored.", "live", "stale", "unknown"),
		"observed_at": str("When this state was observed (UTC, RFC 3339)."),
	}, "account_id", "state", "freshness", "observed_at")
}

func connectionEventSchema() map[string]any {
	return obj(map[string]any{
		"event_id":  str("Stable event id."),
		"timestamp": str("UTC, RFC 3339."),
		"event":     str("connected | disconnected | logged_out | connect_failure | stream_replaced | client_outdated | temporary_ban (whatever whatsmeow itself reported)."),
		"reason":    map[string]any{"type": []string{"string", "null"}, "description": "Sanitized, human-readable detail, when the adapter provided one."},
	}, "event_id", "timestamp", "event")
}

func campaignEventSchema() map[string]any {
	return obj(map[string]any{
		"event_id":              str("Stable event id."),
		"timestamp":             str("UTC, RFC 3339."),
		"event_type":            str("e.g. started, paused, send_attempt_started, provider_accepted, provider_rejected, retry_scheduled, retry_exhausted, recipient_outcome_unknown, delivery_receipt_received, completed."),
		"campaign_recipient_id": map[string]any{"type": []string{"string", "null"}, "description": "Opaque recipient id, when this event is recipient-scoped."},
		"attempt_id":            map[string]any{"type": []string{"string", "null"}},
		"chat_id":               map[string]any{"type": []string{"string", "null"}, "description": "Opaque chat id, when resolved at the time of this event."},
		"message_id":            map[string]any{"type": []string{"string", "null"}},
		"error_code":            map[string]any{"type": []string{"string", "null"}, "description": "Structured error code, when this event names a failure."},
		"detail":                map[string]any{"type": "object", "description": "Small structured, sanitized detail (never a raw provider payload or a secret)."},
	}, "event_id", "timestamp", "event_type")
}

func (s *Server) handleDiagnoseCampaign(ctx context.Context, orgID, _ uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	if s.Deps.Store == nil {
		return toolError("diagnostics are not available on this server"), nil
	}
	id, err := uuid.Parse(stringField(args, "campaign_id"))
	if err != nil {
		return toolError(errCampaignNotFound), nil
	}
	camp, err := orgScopedCampaign(ctx, s.Deps.Store, orgID, id)
	if err != nil {
		return toolError(errCampaignNotFound), nil
	}

	summary, err := s.Deps.Store.CampaignOutcomeSummaryFor(ctx, camp.ID)
	if err != nil {
		return nil, err
	}
	events, _, err := s.Deps.Store.ListCampaignEvents(ctx, camp.ID, 10, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	windowEnd := now
	if purecampaign.IsTerminal(purecampaign.Status(camp.Status)) {
		windowEnd = camp.UpdatedAt
	}
	windowStart := camp.CreatedAt
	if camp.StartedAt != nil {
		windowStart = *camp.StartedAt
	}

	connCurrent := s.whatsappConnectionSummary(ctx, camp.AccountID, camp.Channel)
	var connHistory []map[string]any
	var missing []string
	if camp.Channel == "whatsapp" || camp.Channel == "simulator" {
		evs, err := s.Deps.Store.ConnectionEventsInWindow(ctx, camp.AccountID, windowStart, windowEnd, 20)
		if err == nil {
			for _, e := range evs {
				connHistory = append(connHistory, mapConnectionEvent(e))
			}
		}
	} else {
		missing = append(missing, "connection history is only tracked for WhatsApp/Simulator accounts; this campaign's channel is "+camp.Channel+".")
	}

	var completedAt any
	if purecampaign.IsTerminal(purecampaign.Status(camp.Status)) {
		completedAt = ts(camp.UpdatedAt)
	}

	facts := []string{
		fmt.Sprintf("%d of %d recipients have reached a terminal outcome (sent, failed, or skipped).", summary.Sent+summary.Failed+summary.Skipped, summary.Total),
	}
	if summary.UnknownOutcome > 0 {
		facts = append(facts, fmt.Sprintf("%d recipient(s) have an UNRESOLVED (unknown) sending outcome and were not automatically retried.", summary.UnknownOutcome))
	}
	if summary.RetryExhausted > 0 {
		facts = append(facts, fmt.Sprintf("%d recipient(s) exhausted the automatic retry ladder.", summary.RetryExhausted))
	}
	if summary.Delivered == 0 && summary.Sent > 0 {
		facts = append(facts, "No delivery receipts have been recorded for this campaign's accepted sends — this does NOT necessarily mean delivery failed; it may mean the channel/account has not reported any receipt yet.")
	}

	var likelyCauses []map[string]any
	if n := summary.FailuresByCode[string(campaigndiag.ErrorCodeWhatsAppDisconnected)]; n > 0 && len(connHistory) > 0 {
		var supportingIDs []string
		for _, e := range connHistory {
			if e["event"] == "disconnected" {
				supportingIDs = append(supportingIDs, e["event_id"].(string))
			}
		}
		if len(supportingIDs) > 0 {
			likelyCauses = append(likelyCauses, map[string]any{
				"cause":                fmt.Sprintf("%d send(s) failed with a disconnected-account error while a disconnect event was recorded during this campaign's own active window.", n),
				"confidence":           "likely",
				"supporting_event_ids": supportingIDs,
			})
		}
	}
	if len(likelyCauses) == 0 && len(summary.FailuresByCode) > 0 {
		missing = append(missing, "no failure reason accounts for a large enough share of failures, or no time-correlated supporting event was found, to name a likely cause with confidence — see failures_by_reason for the raw breakdown instead.")
	}

	out := map[string]any{
		"campaign_id":      camp.ID.String(),
		"name":             camp.Name,
		"status":           camp.Status,
		"channel":          camp.Channel,
		"account_ids":      []string{camp.AccountID.String()},
		"created_at":       ts(camp.CreatedAt),
		"started_at":       tsPtrOrNil(camp.StartedAt),
		"completed_at":     completedAt,
		"total_recipients": summary.Total,
		"counts": map[string]any{
			"pending": summary.Pending, "sending": summary.Sending, "sent": summary.Sent,
			"failed": summary.Failed, "skipped": summary.Skipped, "unknown_outcome": summary.UnknownOutcome,
		},
		"delivery":           map[string]any{"delivered": summary.Delivered, "read": summary.Read},
		"failures_by_reason": summary.FailuresByCode,
		"retry_summary": map[string]any{
			"recovered": summary.RetryRecovered, "scheduled": summary.RetryScheduled,
			"exhausted": summary.RetryExhausted, "unresolved": summary.RetryUnresolved,
		},
		"connection": map[string]any{
			"current":                  connCurrent,
			"observed_during_campaign": orEmptySlice(connHistory),
		},
		"recent_events":    mapCampaignEvents(events),
		"facts":            facts,
		"likely_causes":    orEmptySlice(likelyCauses),
		"missing_evidence": missing,
		"observed_at":      ts(now),
	}
	return s.diagResult(fmt.Sprintf("Campaign %q: %s, %d/%d recipients terminal.", camp.Name, camp.Status, summary.Sent+summary.Failed+summary.Skipped, summary.Total), out), nil
}

func orEmptySlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func mapConnectionEvent(e store.WAConnectionEvent) map[string]any {
	return map[string]any{
		"event_id": e.ID.String(), "timestamp": ts(e.CreatedAt), "event": e.Event, "reason": strOrNil(e.Reason),
	}
}

func mapCampaignEvents(events []store.CampaignEvent) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"event_id": e.ID.String(), "timestamp": ts(e.CreatedAt), "event_type": e.Event,
			"campaign_recipient_id": nullUUIDStr(e.CampaignRecipientID), "attempt_id": nullUUIDStr(e.AttemptID),
			"chat_id": nullUUIDStr(e.ChatID), "message_id": nullUUIDStr(e.MessageID),
			"error_code": strOrNil(e.ErrorCode), "detail": e.Detail,
		})
	}
	return out
}

// whatsappConnectionSummary is diagnose_campaign's compact inline version of
// whatsapp_connection_status — the same underlying data, shaped for
// embedding rather than as that tool's own full response.
func (s *Server) whatsappConnectionSummary(ctx context.Context, accountID uuid.UUID, channel string) map[string]any {
	now := ts(time.Now().UTC())
	if channel != "whatsapp" && channel != "simulator" {
		return map[string]any{"account_id": accountID.String(), "state": "unknown", "freshness": "unknown", "observed_at": now}
	}
	if s.Deps.WA != nil {
		if cs, err := s.Deps.WA.Status(ctx, accountID.String()); err == nil {
			return map[string]any{"account_id": accountID.String(), "state": cs.State, "freshness": "live", "observed_at": now}
		}
	}
	if acct, err := s.Deps.Store.AccountByID(ctx, accountID); err == nil {
		return map[string]any{"account_id": accountID.String(), "state": orUnknown(acct.ConnectionState), "freshness": "stale", "observed_at": now}
	}
	return map[string]any{"account_id": accountID.String(), "state": "unknown", "freshness": "unknown", "observed_at": now}
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// diagResult is toolResult's shape minus the KB widget-specific fields
// (xchats/widgetView, the review-handoff URL) which have no meaning for a
// diagnostic tool with no KB Manager resource to open.
func (s *Server) diagResult(summary string, structured any) map[string]any {
	return map[string]any{
		"content":           []map[string]any{{"type": "text", "text": summary}},
		"isError":           false,
		"structuredContent": structured,
	}
}

// --- campaign_recipients -------------------------------------------------

func campaignRecipientsTool() Tool {
	return Tool{
		Name: toolCampaignRecipients,
		Description: "List one campaign's recipients (masked identity, opaque ids only) with each one's sending " +
			"outcome, delivery/read status when available, latest attempt error, retry state, and linked " +
			"chat/message — distinguishing the FINAL outcome from the latest attempt (a recipient whose retry " +
			"succeeded shows outcome=sent, never failed). Read-only.",
		InputSchema: obj(map[string]any{
			"campaign_id": str("The campaign's id."),
			"status": enumStr("Filter by outcome. pending/sending/sent/failed/skipped match campaign_recipients' own persisted status exactly (sent = provider accepted, not confirmed delivery). \"unknown\" is a derived filter: recipients whose most recent attempt ended in an unresolved outcome (a subset of status=failed). Omit for every status.",
				"pending", "sending", "sent", "failed", "skipped", "unknown"),
			"cursor": str("Opaque pagination cursor from a previous call's next_cursor."),
			"limit":  integer("Max recipients to return. Default 50, maximum 100."),
		}, "campaign_id"),
		Annotations:  readOnlyAnnotations(),
		OutputSchema: campaignRecipientsOutputSchema(),
	}
}

func campaignRecipientsOutputSchema() map[string]any {
	item := obj(map[string]any{
		"recipient_id":         str("Opaque recipient id."),
		"masked_identity":      str("The recipient's normalized identity (phone number or channel-specific id) with all but the last 4 characters masked."),
		"outcome":              enumStr("The recipient's CURRENT, final-so-far status — authoritative; not the same as the latest attempt below.", "pending", "sending", "sent", "failed", "skipped"),
		"delivery_status":      map[string]any{"type": []string{"string", "null"}, "description": "The linked message's own delivery/read state (queued, sent, delivered, read, failed), or null if no message has been linked yet. Stronger evidence than outcome=sent, and never implied by it."},
		"provider_accepted_at": map[string]any{"type": []string{"string", "null"}, "description": "When the provider accepted a send for this recipient (the existing sent_at concept in this application), from the attempt that succeeded — null if never accepted."},
		"latest_attempt": obj(map[string]any{
			"attempt_number": integer("1-based."),
			"started_at":     str("UTC, RFC 3339."),
			"completed_at":   map[string]any{"type": []string{"string", "null"}, "description": "Null if this attempt is still in flight (should be transient)."},
			"outcome":        enumStr("This SPECIFIC attempt's own outcome — may differ from the recipient's overall `outcome` above (e.g. a failed attempt followed by a successful retry).", "sending", "provider_accepted", "failed", "unknown"),
			"error_code":     map[string]any{"type": []string{"string", "null"}},
			"error_detail":   str("Sanitized detail, empty string if none."),
		}, "attempt_number", "started_at", "outcome"),
		"attempt_count": integer("Total attempts made so far."),
		"retry_state": enumStr("none = no retry involved; scheduled = will be retried automatically; exhausted = the retry ladder was used up; not_applicable = terminal for a reason a retry could never fix (e.g. invalid recipient), or still pending its first attempt.",
			"none", "scheduled", "exhausted", "not_applicable"),
		"next_retry_at":             map[string]any{"type": []string{"string", "null"}},
		"chat_id":                   map[string]any{"type": []string{"string", "null"}, "description": "Opaque id of the linked conversation, when one has been resolved."},
		"message_id":                map[string]any{"type": []string{"string", "null"}},
		"conversation_pre_existing": map[string]any{"type": []string{"boolean", "null"}, "description": "Whether the conversation already existed before this recipient's campaign processing began. Null when no chat has been resolved yet (nothing to answer this about) or when this predates the feature that started recording it."},
	}, "recipient_id", "masked_identity", "outcome", "attempt_count", "retry_state")
	return obj(map[string]any{
		"items":       map[string]any{"type": "array", "items": item},
		"next_cursor": str("Pass to the next call to continue; absent when this is the last page."),
		"total":       integer("Total recipients matching the filter, across every page."),
	}, "items", "total")
}

func (s *Server) handleCampaignRecipients(ctx context.Context, orgID, _ uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	if s.Deps.Store == nil {
		return toolError("diagnostics are not available on this server"), nil
	}
	id, err := uuid.Parse(stringField(args, "campaign_id"))
	if err != nil {
		return toolError(errCampaignNotFound), nil
	}
	camp, err := orgScopedCampaign(ctx, s.Deps.Store, orgID, id)
	if err != nil {
		return toolError(errCampaignNotFound), nil
	}
	status := stringField(args, "status")
	switch status {
	case "", "pending", "sending", "sent", "failed", "skipped", "unknown":
	default:
		return toolError(fmt.Sprintf("unknown status %q — valid values: pending, sending, sent, failed, skipped, unknown", status)), nil
	}
	limit := clampLimit(intField(args, "limit"))
	offset := parseCursorOffset(stringField(args, "cursor"))

	var recipients []store.CampaignRecipient
	var total int
	if status == "unknown" {
		recipients, total, err = s.Deps.Store.ListCampaignRecipientsUnknownOutcome(ctx, camp.ID, camp.Channel, limit, offset)
	} else {
		recipients, total, err = s.Deps.Store.ListCampaignRecipients(ctx, camp.ID, camp.Channel, status, limit, offset)
	}
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, len(recipients))
	for i, r := range recipients {
		ids[i] = r.ID
	}
	latest, err := s.Deps.Store.LatestAttemptsForRecipients(ctx, ids)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(recipients))
	for _, r := range recipients {
		items = append(items, mapCampaignRecipient(r, latest[r.ID]))
	}
	page := map[string]any{"items": items, "total": total}
	if nc := nextCursor(offset, len(recipients), total); nc != "" {
		page["next_cursor"] = nc
	}
	return s.diagResult(fmt.Sprintf("%d recipient(s).", len(items)), page), nil
}

func mapCampaignRecipient(r store.CampaignRecipient, latest store.CampaignSendAttempt) map[string]any {
	out := map[string]any{
		"recipient_id":    r.ID.String(),
		"masked_identity": maskPhoneLikeIdentity(r.NormalizedIdentity),
		"outcome":         r.Status,
		"attempt_count":   r.Attempts,
		"chat_id":         nullUUIDStr(r.ChatID),
		"message_id":      nullUUIDStr(r.MessageID),
	}
	if r.MessageDeliveryState != "" {
		out["delivery_status"] = r.MessageDeliveryState
	} else {
		out["delivery_status"] = nil
	}
	out["conversation_pre_existing"] = nil // see this feature's own historical-data caveat: not tracked pre-existing/independently of chat resolution today.

	var providerAcceptedAt any
	if latest.ID != uuid.Nil {
		out["latest_attempt"] = map[string]any{
			"attempt_number": latest.AttemptNumber, "started_at": ts(latest.StartedAt),
			"completed_at": tsPtrOrNil(latest.CompletedAt), "outcome": latest.Outcome,
			"error_code": strOrNil(latest.ErrorCode), "error_detail": latest.ErrorDetail,
		}
		if latest.Outcome == "provider_accepted" && latest.CompletedAt != nil {
			providerAcceptedAt = ts(*latest.CompletedAt)
		}
		out["retry_state"] = retryStateFor(r, latest)
		out["next_retry_at"] = tsPtrOrNil(r.NextAttemptAt)
	} else {
		out["retry_state"] = "not_applicable"
		out["next_retry_at"] = nil
	}
	out["provider_accepted_at"] = providerAcceptedAt
	return out
}

func retryStateFor(r store.CampaignRecipient, latest store.CampaignSendAttempt) string {
	switch {
	case r.Status == "pending" && r.NextAttemptAt != nil:
		return "scheduled"
	case r.Status == "failed" && strings.HasPrefix(r.FailureReason, "max retries exceeded"):
		return "exhausted"
	case latest.Retryable != nil && *latest.Retryable && r.Status == "pending":
		return "scheduled"
	// A clean first-attempt success never involved a retry at all — distinct
	// from not_applicable's own two cases (a retry could never have helped,
	// or nothing has been attempted yet).
	case r.Status == "sent" && r.Attempts <= 1:
		return "none"
	default:
		return "not_applicable"
	}
}

// --- whatsapp_connection_status -------------------------------------------

func whatsappConnectionStatusTool() Tool {
	return Tool{
		Name: toolWhatsAppConnectionStatus,
		Description: "Report a WhatsApp/Simulator account's current connection state (live-checked when " +
			"possible, otherwise the last known value with that explicitly marked stale), when it was last " +
			"observed, its recent connect/disconnect history, and whether it looks ready to attempt campaign " +
			"sending right now. Connection readiness does not guarantee any specific recipient or message is " +
			"deliverable. Read-only.",
		InputSchema: obj(map[string]any{
			"account_id": str("The WhatsApp/Simulator account's id."),
		}, "account_id"),
		Annotations: readOnlyAnnotations(),
		OutputSchema: obj(map[string]any{
			"account_id":  str("Echoes the request."),
			"state":       enumStr("Current connection state.", "connected", "disconnected", "logged_out", "unknown"),
			"freshness":   enumStr("Whether state was checked live just now, or is the last stored value.", "live", "stale", "unknown"),
			"observed_at": str("UTC, RFC 3339."),
			"last_successful_activity": map[string]any{
				"type":        []string{"object", "null"},
				"description": "The most recent recorded connection-history event of ANY kind for this account (its type and timestamp) — the closest available proxy for \"last known good activity\"; null if no history has ever been recorded.",
				"properties":  map[string]any{"type": str("The event name, e.g. connected."), "at": str("UTC, RFC 3339.")},
			},
			"last_error": map[string]any{
				"type":        []string{"object", "null"},
				"description": "The most recent disconnect/failure-shaped event's sanitized reason, if any.",
				"properties":  map[string]any{"event": str("e.g. disconnected, connect_failure."), "reason": str("Sanitized detail."), "at": str("UTC, RFC 3339.")},
			},
			"history": map[string]any{
				"type": "array", "items": connectionEventSchema(),
				"description": "Recent connect/disconnect/logged-out events, newest first, capped at 20. Empty if this deployment predates connection history being recorded — see missing_evidence.",
			},
			"ready_for_campaign_sending": enumStr("yes/no/unknown — connection state plus known blockers, not a guarantee any specific send will succeed.", "yes", "no", "unknown"),
			"blockers":                   strArray("Known reasons sending would not proceed right now (e.g. \"account is disconnected\", \"account-wide sending is paused\"). Empty if none are known — absence of a listed blocker is not a guarantee."),
			"missing_evidence":           strArray("Named gaps in the evidence used to answer this."),
		}, "account_id", "state", "freshness", "observed_at", "history", "ready_for_campaign_sending", "blockers", "missing_evidence"),
	}
}

func (s *Server) handleWhatsAppConnectionStatus(ctx context.Context, orgID, _ uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	if s.Deps.Store == nil {
		return toolError("diagnostics are not available on this server"), nil
	}
	id, err := uuid.Parse(stringField(args, "account_id"))
	if err != nil {
		return toolError(errAccountNotFound), nil
	}
	acct, err := orgScopedAccount(ctx, s.Deps.Store, orgID, id)
	if err != nil {
		return toolError(errAccountNotFound), nil
	}
	if acct.Channel != "whatsapp" && acct.Channel != "simulator" {
		return toolError("this account's channel (" + acct.Channel + ") is not WhatsApp/Simulator; connection status is only tracked for those"), nil
	}

	now := time.Now().UTC()
	var state, freshness string
	if s.Deps.WA != nil {
		if cs, werr := s.Deps.WA.Status(ctx, acct.ID.String()); werr == nil {
			state, freshness = cs.State, "live"
		}
	}
	var missing []string
	if state == "" {
		state, freshness = orUnknown(acct.ConnectionState), "stale"
		missing = append(missing, "a live connection check was not available; state reflects the last value this application stored, which may be out of date.")
	}

	history, herr := s.Deps.Store.ListWAConnectionEventsForAccount(ctx, acct.ID, 20)
	if herr != nil {
		return nil, herr
	}
	if len(history) == 0 {
		missing = append(missing, "no connection history has been recorded for this account yet (either it has never transitioned since this feature started tracking history, or it predates that).")
	}
	historyOut := make([]map[string]any, 0, len(history))
	var lastActivity, lastError map[string]any
	for i, e := range history {
		historyOut = append(historyOut, mapConnectionEvent(e))
		if i == 0 {
			lastActivity = map[string]any{"type": e.Event, "at": ts(e.CreatedAt)}
		}
		if lastError == nil && (e.Event == "disconnected" || e.Event == "connect_failure" || e.Event == "logged_out" || e.Event == "stream_replaced" || e.Event == "client_outdated" || e.Event == "temporary_ban") {
			lastError = map[string]any{"event": e.Event, "reason": e.Reason, "at": ts(e.CreatedAt)}
		}
	}

	var blockers []string
	ready := "unknown"
	switch state {
	case "connected":
		ready = "yes"
	case "disconnected", "logged_out", "unknown":
		ready = "no"
		blockers = append(blockers, "account is "+state)
	}
	if settings, serr := s.Deps.Store.CampaignAccountSettingsFor(ctx, acct.ID, acct.Channel); serr == nil && settings.Paused {
		blockers = append(blockers, "account-wide campaign sending is manually paused")
		ready = "no"
	}

	out := map[string]any{
		"account_id": acct.ID.String(), "state": state, "freshness": freshness, "observed_at": ts(now),
		"last_successful_activity": lastActivity, "last_error": lastError, "history": historyOut,
		"ready_for_campaign_sending": ready, "blockers": orEmptySlice(blockers), "missing_evidence": orEmptySlice(missing),
	}
	return s.diagResult(fmt.Sprintf("Account %s: %s (%s).", acct.ID, state, freshness), out), nil
}

// --- campaign_events -------------------------------------------------------

func campaignEventsTool() Tool {
	return Tool{
		Name: toolCampaignEvents,
		Description: "Return one campaign's persisted diagnostic timeline (campaign lifecycle, per-recipient " +
			"send-attempt, retry, and delivery-receipt events), paginated and in deterministic order, plus a " +
			"separate, bounded list of the sending account's connection events relevant to this campaign's own " +
			"active window (never its unrelated history). Read-only.",
		InputSchema: obj(map[string]any{
			"campaign_id": str("The campaign's id."),
			"cursor":      str("Opaque pagination cursor from a previous call's next_cursor."),
			"limit":       integer("Max events to return. Default 50, maximum 100."),
		}, "campaign_id"),
		Annotations: readOnlyAnnotations(),
		OutputSchema: obj(map[string]any{
			"items":       map[string]any{"type": "array", "items": campaignEventSchema()},
			"next_cursor": str("Pass to the next call to continue; absent when this is the last page."),
			"total":       integer("Total events matching, across every page."),
			"relevant_connection_events": map[string]any{
				"type": "array", "items": connectionEventSchema(),
				"description": "NOT part of the paginated cursor above — a separately bounded (max 20) list of the sending account's connection events whose timestamp falls within this campaign's own active window. Empty for a non-WhatsApp/Simulator channel.",
			},
		}, "items", "total", "relevant_connection_events"),
	}
}

func (s *Server) handleCampaignEvents(ctx context.Context, orgID, _ uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	if s.Deps.Store == nil {
		return toolError("diagnostics are not available on this server"), nil
	}
	id, err := uuid.Parse(stringField(args, "campaign_id"))
	if err != nil {
		return toolError(errCampaignNotFound), nil
	}
	camp, err := orgScopedCampaign(ctx, s.Deps.Store, orgID, id)
	if err != nil {
		return toolError(errCampaignNotFound), nil
	}
	limit := clampLimit(intField(args, "limit"))
	offset := parseCursorOffset(stringField(args, "cursor"))

	events, total, err := s.Deps.Store.ListCampaignEvents(ctx, camp.ID, limit, offset)
	if err != nil {
		return nil, err
	}
	page := map[string]any{"items": mapCampaignEvents(events), "total": total}
	if nc := nextCursor(offset, len(events), total); nc != "" {
		page["next_cursor"] = nc
	}

	var connEvents []map[string]any
	if camp.Channel == "whatsapp" || camp.Channel == "simulator" {
		windowEnd := time.Now().UTC()
		if purecampaign.IsTerminal(purecampaign.Status(camp.Status)) {
			windowEnd = camp.UpdatedAt
		}
		windowStart := camp.CreatedAt
		if camp.StartedAt != nil {
			windowStart = *camp.StartedAt
		}
		if evs, cerr := s.Deps.Store.ConnectionEventsInWindow(ctx, camp.AccountID, windowStart, windowEnd, 20); cerr == nil {
			for _, e := range evs {
				connEvents = append(connEvents, mapConnectionEvent(e))
			}
		}
	}
	page["relevant_connection_events"] = orEmptySlice(connEvents)

	return s.diagResult(fmt.Sprintf("%d event(s).", len(events)), page), nil
}
