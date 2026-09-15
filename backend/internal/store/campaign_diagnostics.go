package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// CampaignSendAttempt is one row of a recipient's non-pruned attempt
// history — see migrations/sqlite/0020_campaign_diagnostics.up.sql's own
// doc comment for how this differs from campaign_send_log.
type CampaignSendAttempt struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	CampaignID          uuid.UUID
	CampaignRecipientID uuid.UUID
	AccountID           string
	AttemptNumber       int
	ChatID              uuid.NullUUID
	MessageID           uuid.NullUUID
	ProviderMessageID   string
	StartedAt           time.Time
	CompletedAt         *time.Time
	Outcome             string
	ErrorCode           string
	ErrorDetail         string
	Retryable           *bool
	NextAttemptAt       *time.Time
	CreatedAt           time.Time
}

const campaignSendAttemptCols = `id, organization_id, campaign_id, campaign_recipient_id, account_id, attempt_number,
	chat_id, message_id, provider_message_id, started_at, completed_at, outcome, error_code, error_detail, retryable,
	next_attempt_at, created_at`

func scanCampaignSendAttempt(row dbx.Scanner) (CampaignSendAttempt, error) {
	var a CampaignSendAttempt
	var providerMsgID, errorCode *string
	if err := row.Scan(&a.ID, &a.OrganizationID, &a.CampaignID, &a.CampaignRecipientID, &a.AccountID, &a.AttemptNumber,
		&a.ChatID, &a.MessageID, &providerMsgID, &a.StartedAt, &a.CompletedAt, &a.Outcome, &errorCode, &a.ErrorDetail,
		&a.Retryable, &a.NextAttemptAt, &a.CreatedAt); err != nil {
		return a, err
	}
	if providerMsgID != nil {
		a.ProviderMessageID = *providerMsgID
	}
	if errorCode != nil {
		a.ErrorCode = *errorCode
	}
	return a, nil
}

// LatestAttemptsForRecipients returns, for every id in recipientIDs that has
// at least one attempt, its most-recent attempt (by attempt_number) —
// batched in one query so listing a page of recipients never triggers one
// query per row. A recipient absent from the result has never been claimed
// (still genuinely 'pending', attempts = 0).
func (s *Store) LatestAttemptsForRecipients(ctx context.Context, recipientIDs []uuid.UUID) (map[uuid.UUID]CampaignSendAttempt, error) {
	out := map[uuid.UUID]CampaignSendAttempt{}
	if len(recipientIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT `+campaignSendAttemptCols+` FROM campaign_send_attempts a
		WHERE a.campaign_recipient_id IN (SELECT value FROM json_each($1))
		  AND a.attempt_number = (
		    SELECT MAX(a2.attempt_number) FROM campaign_send_attempts a2
		    WHERE a2.campaign_recipient_id = a.campaign_recipient_id
		  )`, dbx.UUIDArray(recipientIDs))
	if err != nil {
		return nil, wrap("latest campaign send attempts", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		a, err := scanCampaignSendAttempt(rows)
		if err != nil {
			return nil, err
		}
		out[a.CampaignRecipientID] = a
	}
	return out, rows.Err()
}

// ListAttemptsForRecipient returns every attempt for one recipient, oldest
// first — the full per-attempt history behind campaign_recipients' own
// single coarse status, for a diagnostics view that needs to show every
// try, not just the latest.
func (s *Store) ListAttemptsForRecipient(ctx context.Context, recipientID uuid.UUID) ([]CampaignSendAttempt, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+campaignSendAttemptCols+` FROM campaign_send_attempts
		WHERE campaign_recipient_id = $1 ORDER BY attempt_number ASC`, recipientID)
	if err != nil {
		return nil, wrap("list campaign send attempts", err)
	}
	defer func() { _ = rows.Close() }()
	var out []CampaignSendAttempt
	for rows.Next() {
		a, err := scanCampaignSendAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CampaignOutcomeSummary is diagnose_campaign's counts section — every
// number here is a DISTINCT, documented, non-overlapping bucket over the
// campaign's recipients (see this struct's own field comments), so a caller
// summing them all recovers the campaign's total recipient count exactly
// once each.
type CampaignOutcomeSummary struct {
	Total   int
	Pending int
	Sending int
	// Sent is "provider accepted" — see this feature's own status-semantics
	// doc: proof the provider took the message, never proof of delivery.
	Sent    int
	Failed  int
	Skipped int
	// UnknownOutcome counts recipients whose MOST RECENT attempt ended
	// 'unknown' (an ambiguous timeout, or a crash-interrupted send) — these
	// recipients' coarse Status is 'failed' (existing vocabulary, counted
	// in Failed above too) specifically so this number is NEVER silently
	// folded into either a success or a confirmed-failure count elsewhere;
	// it is reported here as its own, separately-surfaced bucket.
	UnknownOutcome int
	// Delivered/Read are provider delivery/read receipts on the recipient's
	// linked message — evidence STRONGER than "sent" (provider accepted),
	// counted separately and never implied by it. Both are 0 for a channel
	// or account that has never reported a receipt, which is NOT the same
	// as "confirmed not delivered" — see this feature's documentation.
	Delivered int
	Read      int

	// FailuresByCode groups every recipient CURRENTLY 'failed' by their
	// latest attempt's structured error code ("unknown" for a failed
	// recipient with no attempt recorded at all — legacy data, or a
	// pre-InsertCampaignOutbound failure that predates this feature).
	FailuresByCode map[string]int

	// Retry summary — see each field's own meaning. Recovered/Exhausted are
	// both terminal; Scheduled is not (it will be claimed again once its
	// backoff passes); Unresolved always equals UnknownOutcome above,
	// repeated here under the retry-specific framing diagnose_campaign's
	// spec asks for.
	RetryRecovered  int // now 'sent', but only after at least one prior failed attempt
	RetryScheduled  int // 'pending' with a backoff floor set — will be retried automatically
	RetryExhausted  int // 'failed' because every retry in the bounded ladder was used
	RetryUnresolved int
}

// CampaignOutcomeSummaryFor computes every number in CampaignOutcomeSummary
// for campaignID in a small, fixed number of queries — always computed
// live, never cached, mirroring CampaignRecipientCounts' own reasoning (a
// campaign's recipient count is bounded).
func (s *Store) CampaignOutcomeSummaryFor(ctx context.Context, campaignID uuid.UUID) (CampaignOutcomeSummary, error) {
	var out CampaignOutcomeSummary
	counts, err := s.CampaignRecipientCounts(ctx, campaignID)
	if err != nil {
		return out, err
	}
	out.Pending, out.Sending, out.Sent, out.Failed, out.Skipped = counts["pending"], counts["sending"], counts["sent"], counts["failed"], counts["skipped"]
	for _, n := range counts {
		out.Total += n
	}

	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM campaign_recipients r
		WHERE r.campaign_id = $1 AND EXISTS (
			SELECT 1 FROM campaign_send_attempts a
			WHERE a.campaign_recipient_id = r.id AND a.outcome = 'unknown'
			  AND a.attempt_number = (SELECT MAX(a2.attempt_number) FROM campaign_send_attempts a2 WHERE a2.campaign_recipient_id = r.id)
		)`, campaignID).Scan(&out.UnknownOutcome); err != nil {
		return out, wrap("count unknown-outcome recipients", err)
	}
	out.RetryUnresolved = out.UnknownOutcome

	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND status = 'sent' AND attempts > 1`, campaignID).
		Scan(&out.RetryRecovered); err != nil {
		return out, wrap("count recovered recipients", err)
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND status = 'pending' AND next_attempt_at IS NOT NULL`, campaignID).
		Scan(&out.RetryScheduled); err != nil {
		return out, wrap("count scheduled retries", err)
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND status = 'failed' AND failure_reason LIKE 'max retries exceeded%'`, campaignID).
		Scan(&out.RetryExhausted); err != nil {
		return out, wrap("count exhausted retries", err)
	}

	out.FailuresByCode = map[string]int{}
	rows, err := s.db.Query(ctx, `
		SELECT COALESCE(a.error_code, 'unknown'), count(*) FROM campaign_recipients r
		LEFT JOIN campaign_send_attempts a ON a.campaign_recipient_id = r.id
			AND a.attempt_number = (SELECT MAX(a2.attempt_number) FROM campaign_send_attempts a2 WHERE a2.campaign_recipient_id = r.id)
		WHERE r.campaign_id = $1 AND r.status = 'failed'
		GROUP BY 1`, campaignID)
	if err != nil {
		return out, wrap("group failures by code", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return out, err
		}
		out.FailuresByCode[code] = n
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	channel, err := s.campaignChannel(ctx, campaignID)
	if err != nil {
		return out, err
	}
	msgTable, err := messagesTableFor(channel)
	if err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM campaign_recipients r JOIN `+msgTable+` m ON m.id = r.message_id
		WHERE r.campaign_id = $1 AND m.delivery_state IN ('delivered','read')`, campaignID).Scan(&out.Delivered); err != nil {
		return out, wrap("count delivered", err)
	}
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM campaign_recipients r JOIN `+msgTable+` m ON m.id = r.message_id
		WHERE r.campaign_id = $1 AND m.delivery_state = 'read'`, campaignID).Scan(&out.Read); err != nil {
		return out, wrap("count read", err)
	}
	return out, nil
}

// CampaignRef is the minimal identification of a campaign a chat/message is
// linked to for display purposes — never recipient/audience data.
type CampaignRef struct {
	ID   uuid.UUID
	Name string
}

// CampaignRefsForChats batches the Inbox's "which campaign(s) touched this
// chat" lookup across a whole page of chats in one query — the same
// membership signal campaignMembershipClause (internal/store/read.go) uses
// for View="campaign", here returning WHICH campaign(s) rather than a
// boolean. A chat absent from the result has no campaign participation at
// all. Ordered oldest-campaign-first per chat, so a chat touched by several
// campaigns shows them in a stable, meaningful order.
func (s *Store) CampaignRefsForChats(ctx context.Context, chatIDs []uuid.UUID) (map[uuid.UUID][]CampaignRef, error) {
	out := map[uuid.UUID][]CampaignRef{}
	if len(chatIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT cr.chat_id, camp.id, camp.name, camp.created_at FROM campaign_recipients cr
		JOIN campaigns camp ON camp.id = cr.campaign_id
		WHERE cr.chat_id IN (SELECT value FROM json_each($1))
		ORDER BY cr.chat_id, camp.created_at ASC`, dbx.UUIDArray(chatIDs))
	if err != nil {
		return nil, wrap("campaign refs for chats", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var chatID uuid.UUID
		var ref CampaignRef
		var createdAt time.Time
		if err := rows.Scan(&chatID, &ref.ID, &ref.Name, &createdAt); err != nil {
			return nil, err
		}
		out[chatID] = append(out[chatID], ref)
	}
	return out, rows.Err()
}

func (s *Store) campaignChannel(ctx context.Context, campaignID uuid.UUID) (string, error) {
	var channel string
	err := s.db.QueryRow(ctx, `SELECT channel FROM campaigns WHERE id = $1`, campaignID).Scan(&channel)
	if err != nil {
		return "", wrap("resolve campaign channel", err)
	}
	return channel, nil
}
