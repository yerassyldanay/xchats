package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// KBGapEvent mirrors one ai_kb_gap_events row with its missing_fields
// children attached (0018_kb_gap_telemetry).
type KBGapEvent struct {
	ID               string
	OrganizationID   string
	DraftID          uuid.NullUUID
	Channel          string
	ChatID           uuid.UUID
	TriggerMessageID uuid.NullUUID
	ReasonCode       string
	TargetEntityType string
	TargetEntityRef  string
	EscalationReason string
	Source           string
	CreatedAt        time.Time
	MissingFields    []string
}

const kbGapEventCols = `id, organization_id, draft_id, channel, chat_id, trigger_message_id,
	reason_code, target_entity_type, target_entity_ref, escalation_reason, source, created_at`

func scanKBGapEventDst(e *KBGapEvent) []any {
	return []any{
		&e.ID, &e.OrganizationID, &e.DraftID, &e.Channel, &e.ChatID, &e.TriggerMessageID,
		&e.ReasonCode, &e.TargetEntityType, &e.TargetEntityRef, &e.EscalationReason, &e.Source, &e.CreatedAt,
	}
}

// GapEventsForChat returns a chat's kb-gap telemetry events, newest first,
// each with its missing_fields children attached — a per-conversation
// drill-down mirroring PendingDrafts' existing chat-scoped convention.
func (s *Store) GapEventsForChat(ctx context.Context, chatID uuid.UUID) ([]KBGapEvent, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+kbGapEventCols+`
		FROM ai_kb_gap_events WHERE chat_id = $1 ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []KBGapEvent
	for rows.Next() {
		var e KBGapEvent
		if err := rows.Scan(scanKBGapEventDst(&e)...); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachMissingFields(ctx, s.db, out); err != nil {
		return nil, err
	}
	return out, nil
}

// attachMissingFields fills each event's MissingFields in one query rather
// than one per event.
func attachMissingFields(ctx context.Context, db *dbx.DB, events []KBGapEvent) error {
	if len(events) == 0 {
		return nil
	}
	ids := make([]string, len(events))
	idx := make(map[string]int, len(events))
	for i := range events {
		ids[i] = events[i].ID
		idx[events[i].ID] = i
	}
	rows, err := db.Query(ctx, `
		SELECT event_id, field_name FROM ai_kb_gap_missing_fields
		WHERE event_id IN (SELECT value FROM json_each($1))
		ORDER BY created_at`, dbx.StringArray(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var eventID, field string
		if err := rows.Scan(&eventID, &field); err != nil {
			return err
		}
		if i, ok := idx[eventID]; ok {
			events[i].MissingFields = append(events[i].MissingFields, field)
		}
	}
	return rows.Err()
}

// defaultKBGapRecentLimit bounds GET /kb/gaps' "recent representative
// events" page when the caller does not specify one.
const defaultKBGapRecentLimit = 50

// KBGapFilter narrows a GET /kb/gaps query. OrgID is always required —
// every gap report is organization-scoped; every other field is optional
// (its zero value means "no filter on this dimension").
type KBGapFilter struct {
	OrgID            uuid.UUID
	From             *time.Time
	To               *time.Time
	ReasonCode       string
	TargetEntityType string
	TargetEntityRef  string
	// Limit bounds the "recent representative events" page; <= 0 defaults
	// to defaultKBGapRecentLimit, and anything above maxKBGapRecentLimit is
	// clamped to it — GET /kb/gaps describes this as a bounded page, so an
	// oversized caller value (e.g. ?limit=2147483647) must not be able to
	// materialize every matching event instead.
	Limit int
}

// KBGapReasonCount is one reason code's event count within a filter.
type KBGapReasonCount struct {
	ReasonCode string
	Count      int
}

// KBGapTargetEntityCount is one target entity's escalation count within a
// filter, most-frequent first — the per-product/tariff/etc. rollup a reason-
// code count alone cannot answer ("which product causes the most
// escalations"), and Recent alone cannot either once matching events exceed
// its own bounded page.
type KBGapTargetEntityCount struct {
	TargetEntityType string
	TargetEntityRef  string
	Count            int
}

// KBGapMissingFieldCount is one (entity type, field name) pair's count
// within a filter, most-frequent first. Scoped by entity type because the
// same field name can mean different things on different entities (e.g.
// "price" on a product vs. a tariff) — see kbGapFieldAllowlist.
type KBGapMissingFieldCount struct {
	TargetEntityType string
	FieldName        string
	Count            int
}

// KBGapReport is GET /kb/gaps' payload: aggregated counts plus a bounded
// page of recent representative events — never the customer-facing
// draft/message text, only the diagnostic metadata KBGapEvent carries.
type KBGapReport struct {
	// Counts is the DEFAULT report: aiprompt.DefaultReportReasonCodes'
	// counts only, zero-filled (a code with zero matching events still
	// appears, at 0 — a missing row would be misread as "no data" rather
	// than "none of these happened"), in vocabulary order. Genuine
	// knowledge-base content gaps only.
	Counts []KBGapReasonCount
	// OperationalCounts covers the rest of the vocabulary
	// (unsupported_request, human_requested, engine_error, other) —
	// tracked and returned, but kept out of Counts so an operational error
	// or a customer's own request for a human is never miscounted as a
	// knowledge-base gap.
	OperationalCounts []KBGapReasonCount
	// TopTargetEntities and TopMissingFields are rollups over the SAME
	// filter as Counts/Recent, bounded at maxKBGapRollupRows — see their
	// types' own doc comments for why a reason-code count and a bounded
	// Recent sample cannot answer "which entity/field" on their own.
	TopTargetEntities []KBGapTargetEntityCount
	TopMissingFields  []KBGapMissingFieldCount
	Recent            []KBGapEvent
}

// defaultKBGapRecentLimit's ceiling — see KBGapFilter.Limit's doc comment.
const maxKBGapRecentLimit = 500

// maxKBGapRollupRows bounds TopTargetEntities/TopMissingFields: both are
// "which ones matter most" rollups an operator scans by eye, not a paged
// listing, so a fixed, generous cap (rather than a caller-controlled one)
// keeps the query answerable without inventing a second pagination scheme.
const maxKBGapRollupRows = 20

// operationalReasonCodes is the closed vocabulary's remainder after
// aiprompt.DefaultReportReasonCodes — see KBGapReport.OperationalCounts.
var operationalReasonCodes = []string{
	aiprompt.KBGapReasonUnsupportedRequest, aiprompt.KBGapReasonHumanRequested,
	aiprompt.KBGapReasonEngineError, aiprompt.KBGapReasonOther,
}

// KBGapReportFor answers GET /kb/gaps: aggregated counts (split into the
// default content-gap report and the operational/human-requested bucket —
// see KBGapReport) plus a bounded page of recent events, all under the same
// filter.
func (s *Store) KBGapReportFor(ctx context.Context, f KBGapFilter) (KBGapReport, error) {
	where, args := kbGapFilterClause(f, "")
	clause := strings.Join(where, " AND ")

	countRows, err := s.db.Query(ctx, `
		SELECT reason_code, count(*) FROM ai_kb_gap_events
		WHERE `+clause+`
		GROUP BY reason_code`, args...)
	if err != nil {
		return KBGapReport{}, err
	}
	counts := map[string]int{}
	for countRows.Next() {
		var code string
		var n int
		if err := countRows.Scan(&code, &n); err != nil {
			_ = countRows.Close()
			return KBGapReport{}, err
		}
		counts[code] = n
	}
	if err := countRows.Err(); err != nil {
		_ = countRows.Close()
		return KBGapReport{}, err
	}
	if err := countRows.Close(); err != nil {
		return KBGapReport{}, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = defaultKBGapRecentLimit
	} else if limit > maxKBGapRecentLimit {
		limit = maxKBGapRecentLimit
	}
	recentArgs := append(append([]any{}, args...), limit)
	rows, err := s.db.Query(ctx, `
		SELECT `+kbGapEventCols+` FROM ai_kb_gap_events
		WHERE `+clause+`
		ORDER BY created_at DESC LIMIT $`+itoa(len(recentArgs)), recentArgs...)
	if err != nil {
		return KBGapReport{}, err
	}
	var recent []KBGapEvent
	for rows.Next() {
		var e KBGapEvent
		if err := rows.Scan(scanKBGapEventDst(&e)...); err != nil {
			_ = rows.Close()
			return KBGapReport{}, err
		}
		recent = append(recent, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return KBGapReport{}, err
	}
	if err := rows.Close(); err != nil {
		return KBGapReport{}, err
	}
	if err := attachMissingFields(ctx, s.db, recent); err != nil {
		return KBGapReport{}, err
	}

	topEntities, err := topTargetEntitiesFor(ctx, s.db, clause, args)
	if err != nil {
		return KBGapReport{}, err
	}
	topFields, err := topMissingFieldsFor(ctx, s.db, f, args)
	if err != nil {
		return KBGapReport{}, err
	}

	return KBGapReport{
		Counts:            reasonCountsFor(aiprompt.DefaultReportReasonCodes(), counts),
		OperationalCounts: reasonCountsFor(operationalReasonCodes, counts),
		TopTargetEntities: topEntities,
		TopMissingFields:  topFields,
		Recent:            recent,
	}, nil
}

// topTargetEntitiesFor ranks target entities by escalation count under the
// same filter clause KBGapReportFor already built (target_entity_ref = ”
// rows — no target at all — are meaningless here and excluded, unlike the
// reason-code counts above which count every matching event regardless of
// whether it named a target).
func topTargetEntitiesFor(ctx context.Context, db *dbx.DB, clause string, args []any) ([]KBGapTargetEntityCount, error) {
	rows, err := db.Query(ctx, `
		SELECT target_entity_type, target_entity_ref, count(*) AS n FROM ai_kb_gap_events
		WHERE `+clause+` AND target_entity_ref != ''
		GROUP BY target_entity_type, target_entity_ref
		ORDER BY n DESC, target_entity_type, target_entity_ref
		LIMIT `+itoa(maxKBGapRollupRows), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []KBGapTargetEntityCount
	for rows.Next() {
		var c KBGapTargetEntityCount
		if err := rows.Scan(&c.TargetEntityType, &c.TargetEntityRef, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// topMissingFieldsFor ranks (entity type, field name) pairs by how often
// each was cited as missing, under the SAME filter as everything else in
// KBGapReportFor — field names live in ai_kb_gap_missing_fields, so this is
// the one query here that joins back to ai_kb_gap_events for the filter and
// for target_entity_type (a field's own row has no entity-type column of
// its own; see 0018_kb_gap_telemetry.up.sql). kbGapFilterClause(f, "e.")
// yields the same filter values as the caller's own args, qualified for the
// "e" alias — a fresh call rather than string-editing the caller's clause.
func topMissingFieldsFor(ctx context.Context, db *dbx.DB, f KBGapFilter, args []any) ([]KBGapMissingFieldCount, error) {
	whereJoined, _ := kbGapFilterClause(f, "e.")
	clauseJoined := strings.Join(whereJoined, " AND ")
	rows, err := db.Query(ctx, `
		SELECT e.target_entity_type, mf.field_name, count(*) AS n
		FROM ai_kb_gap_missing_fields mf JOIN ai_kb_gap_events e ON mf.event_id = e.id
		WHERE `+clauseJoined+`
		GROUP BY e.target_entity_type, mf.field_name
		ORDER BY n DESC, e.target_entity_type, mf.field_name
		LIMIT `+itoa(maxKBGapRollupRows), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []KBGapMissingFieldCount
	for rows.Next() {
		var c KBGapMissingFieldCount
		if err := rows.Scan(&c.TargetEntityType, &c.FieldName, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func reasonCountsFor(codes []string, counts map[string]int) []KBGapReasonCount {
	out := make([]KBGapReasonCount, len(codes))
	for i, code := range codes {
		out[i] = KBGapReasonCount{ReasonCode: code, Count: counts[code]}
	}
	return out
}

// kbGapFilterClause builds KBGapFilter's WHERE clause and positional args —
// organization_id is always present (never optional), matching every other
// filter in this codebase's own dynamic-query convention (see ListChats).
// prefix qualifies every column (e.g. "e." for a query that joins
// ai_kb_gap_events under an "e" alias, "" for a query against that table
// alone) — every column referenced here belongs to ai_kb_gap_events itself,
// never to a joined table, so one prefix covers the whole clause.
func kbGapFilterClause(f KBGapFilter, prefix string) ([]string, []any) {
	args := []any{f.OrgID}
	// The simulator (GetOrCreateSimulatorAccount) drives real drafts through
	// the SAME response.Service pipeline as a genuine customer, so prompt
	// testing produces genuine ai_kb_gap_events rows too — deliberately
	// excluded from this operator-facing report so testing a prompt never
	// inflates the "real conversations" counts it exists to track. A per-chat
	// drill-down (GapEventsForChat) is unaffected: it never calls this
	// clause, so a chat that happens to be a simulator conversation still
	// shows its own gap history when opened directly.
	where := []string{prefix + "organization_id = $1", prefix + "channel != 'simulator'"}
	if f.From != nil {
		args = append(args, f.From.UTC().Format("2006-01-02 15:04:05.000"))
		where = append(where, prefix+"created_at >= $"+itoa(len(args)))
	}
	if f.To != nil {
		args = append(args, f.To.UTC().Format("2006-01-02 15:04:05.000"))
		where = append(where, prefix+"created_at <= $"+itoa(len(args)))
	}
	if f.ReasonCode != "" {
		args = append(args, f.ReasonCode)
		where = append(where, prefix+"reason_code = $"+itoa(len(args)))
	}
	if f.TargetEntityType != "" {
		args = append(args, f.TargetEntityType)
		where = append(where, prefix+"target_entity_type = $"+itoa(len(args)))
	}
	if f.TargetEntityRef != "" {
		args = append(args, f.TargetEntityRef)
		where = append(where, prefix+"target_entity_ref = $"+itoa(len(args)))
	}
	return where, args
}

// writeDraftOptionsTx is WriteDraftSet and WriteDraftSetIfVersionCurrent's
// shared body, run inside a transaction the caller already opened (and will
// commit): supersede the chat's prior suggested options, insert the new
// ones, and — for any option that escalated — insert exactly one
// ai_kb_gap_events row (plus its missing_fields children) referencing the
// just-inserted draft, all in the SAME transaction
// (0018_kb_gap_telemetry). Sharing this one body between both callers is
// what makes "a stale automation generation produces neither a draft nor a
// telemetry event" true for free: WriteDraftSetIfVersionCurrent's version
// check and rollback already guard everything this function does, exactly
// as they already guarded the draft insert alone before this existed.
func writeDraftOptionsTx(ctx context.Context, tx *dbx.Tx, channel string, chatID uuid.UUID, trigger uuid.NullUUID, opts []DraftOption) ([]Draft, error) {
	if channel == "" {
		channel = "whatsapp"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ai_drafts SET draft_state='superseded', updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE chat_id = $1 AND draft_state='suggested'`, chatID); err != nil {
		return nil, err
	}

	var orgID string
	var orgResolved bool
	out := make([]Draft, 0, len(opts))
	for _, o := range opts {
		var d Draft
		if err := tx.QueryRow(ctx, `
			INSERT INTO ai_drafts (chat_id, channel, trigger_message_id, option_ordinal, draft_text, reply_language, confidence, escalate, escalation_reason, draft_state)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'suggested')
			RETURNING `+draftCols,
			chatID, channel, trigger, o.Ordinal, o.Text, o.ReplyLanguage, o.Confidence, o.Escalate, o.EscalationReason).
			Scan(scanDraftDst(&d)...); err != nil {
			return nil, err
		}
		out = append(out, d)

		if !o.Escalate {
			continue // "Record an event only for escalate=true" (0018_kb_gap_telemetry)
		}
		if !orgResolved {
			var err error
			orgID, err = chatOrganizationIDTx(ctx, tx, chatID)
			if err != nil {
				return nil, fmt.Errorf("store: resolve organization for kb-gap event: %w", err)
			}
			orgResolved = true
		}
		if err := insertKBGapEventTx(ctx, tx, orgID, channel, chatID, trigger, d.ID, o); err != nil {
			return nil, fmt.Errorf("store: insert kb-gap event: %w", err)
		}
	}
	return out, nil
}

// chatOrganizationIDTx resolves chat_id's owning organization through
// inbox_chats_v — the same channel-neutral view every chat read already
// goes through (WhatsApp, Telegram, simulator, and generic channel_* rows
// alike). ai_kb_gap_events.organization_id is a real, mandatory foreign
// key, unlike ai_drafts.chat_id itself, which deliberately carries none
// (migration 0013's channel-neutral chat_id) — this is the one place kb-gap
// telemetry needs the join ai_drafts otherwise avoids entirely. A chat_id
// that does not resolve here (every real caller's chat_id comes from an
// already-loaded conversation, so this should never happen in production)
// fails the whole write closed rather than silently persisting a draft
// with no telemetry for an escalation that was supposed to get one.
func chatOrganizationIDTx(ctx context.Context, tx *dbx.Tx, chatID uuid.UUID) (string, error) {
	var orgID string
	if err := tx.QueryRow(ctx, `SELECT organization_id FROM inbox_chats_v WHERE id = $1`, chatID).Scan(&orgID); err != nil {
		return "", err
	}
	return orgID, nil
}

// allKBGapReasonCodes/allKBGapEntityTypes are this package's own copies of
// aiprompt's closed vocabularies (built once, not per insert), checked
// again in insertKBGapEventTx — the final gate before a row exists in the
// DB at all (migration 0018 deliberately carries no CHECK constraint of its
// own; see 0018_kb_gap_telemetry.up.sql). DraftOption is exported, so
// aiprompt.sanitizeKBGap having already validated a MODEL-authored
// diagnostic is not a guarantee every caller went through it — a value
// outside the closed set here is normalized to the same default an absent
// one already gets, never rejected outright: an optional diagnostic must
// never fail the draft write it rides in on.
var (
	allKBGapReasonCodes = stringSet(aiprompt.AllKBGapReasonCodes())
	allKBGapEntityTypes = stringSet(aiprompt.AllKBGapEntityTypes())
)

func stringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, s := range items {
		out[s] = true
	}
	return out
}

// insertKBGapEventTx records one ai_kb_gap_events row (plus its
// missing_fields children) for a draft option that escalated.
//
// reason_code defaults to "other" — both when the option carries no
// structured diagnostic at all (a v6-and-earlier response, or a v7 response
// whose kb_gap aiprompt.ValidateResponseV7 sanitized away entirely) and
// when it names a value outside the closed vocabulary (see
// allKBGapReasonCodes above): every escalation gets exactly one event,
// never zero, or the "KB gaps" report would silently undercount however
// many escalations went unclassified.
//
// source defaults to "model" the same way — the only caller that sets
// KBGapSource to "engine" explicitly is response/service.go's holdingDraft,
// a hard engine failure that never reached the model contract at all.
//
// target_entity_type/target_entity_ref are cleared together (never one
// without the other) unless the type is one of the closed
// AllKBGapEntityTypes — the same "both valid or neither" invariant
// aiprompt.sanitizeKBGap enforces for a model-authored diagnostic.
func insertKBGapEventTx(ctx context.Context, tx *dbx.Tx, orgID, channel string, chatID uuid.UUID, trigger uuid.NullUUID, draftID uuid.UUID, o DraftOption) error {
	reasonCode := o.KBGapReasonCode
	if !allKBGapReasonCodes[reasonCode] {
		reasonCode = aiprompt.KBGapReasonOther
	}
	source := o.KBGapSource
	if source != aiprompt.KBGapSourceModel && source != aiprompt.KBGapSourceEngine {
		source = aiprompt.KBGapSourceModel
	}
	targetEntityType, targetEntityRef := o.KBGapTargetEntityType, o.KBGapTargetEntityRef
	if targetEntityType == "" || targetEntityRef == "" || !allKBGapEntityTypes[targetEntityType] {
		targetEntityType, targetEntityRef = "", ""
	}
	var eventID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO ai_kb_gap_events
			(organization_id, draft_id, channel, chat_id, trigger_message_id, reason_code, target_entity_type, target_entity_ref, escalation_reason, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		orgID, draftID, channel, chatID, trigger, reasonCode, targetEntityType, targetEntityRef, o.EscalationReason, source).
		Scan(&eventID); err != nil {
		return err
	}
	for _, field := range o.KBGapMissingFields {
		if field == "" {
			continue
		}
		// ON CONFLICT DO NOTHING makes this insert idempotent against a
		// repeated field name: aiprompt.sanitizeKBGap already dedupes a
		// model-authored diagnostic, but DraftOption is exported, so a
		// caller that builds one directly must not be able to roll back
		// the whole draft write over UNIQUE(event_id, field_name) — an
		// optional diagnostic must never be able to fail a customer draft.
		if _, err := tx.Exec(ctx,
			`INSERT INTO ai_kb_gap_missing_fields (event_id, field_name) VALUES ($1, $2)
				ON CONFLICT (event_id, field_name) DO NOTHING`,
			eventID, field); err != nil {
			return err
		}
	}
	return nil
}
