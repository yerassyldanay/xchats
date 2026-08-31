package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// CampaignTemplate is one reusable, organization-wide message template
// (CAM-14) — see migrations/sqlite/0015_campaign_templates.up.sql's own
// doc comment for why it is a standalone entity rather than a campaign
// with no recipients.
type CampaignTemplate struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	MessageBody    string
	Variables      []string // detected {{variable}} names — see backend/campaign.ExtractVariables
	IsArchived     bool
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CampaignTemplatePatch is PATCH /campaign-templates/:id's write shape —
// unlike CampaignPatch, templates carry no lifecycle lock (backend/
// campaign.CanEditContent has no equivalent here: nothing has ever "sent"
// a template), so every field is a plain optional pointer with no
// three-state clear-to-NULL case to support.
type CampaignTemplatePatch struct {
	Name        *string
	MessageBody *string
}

const campaignTemplateCols = `id, organization_id, name, message_body, variables, is_archived, created_by, created_at, updated_at`

func scanCampaignTemplate(row dbx.Scanner) (CampaignTemplate, error) {
	var t CampaignTemplate
	var variablesRaw string
	var archived int
	err := row.Scan(&t.ID, &t.OrganizationID, &t.Name, &t.MessageBody, &variablesRaw, &archived, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	t.Variables = decodeStringSlice(variablesRaw)
	t.IsArchived = archived != 0
	return t, nil
}

// CreateCampaignTemplate inserts a new active template. t.Variables is
// derived from t.MessageBody (backend/campaign.ExtractVariables), never
// taken from the input struct — same rule CreateCampaign follows.
func (s *Store) CreateCampaignTemplate(ctx context.Context, t CampaignTemplate) (CampaignTemplate, error) {
	variablesJSON, err := json.Marshal(purecampaign.ExtractVariables(t.MessageBody))
	if err != nil {
		return CampaignTemplate{}, wrap("marshal template variables", err)
	}
	out, err := scanCampaignTemplate(s.db.QueryRow(ctx, `
		INSERT INTO campaign_templates (organization_id, name, message_body, variables, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+campaignTemplateCols,
		t.OrganizationID, t.Name, t.MessageBody, string(variablesJSON), t.CreatedBy))
	return out, wrap("create campaign template", err)
}

// CampaignTemplateByIDForOrg returns a template only if it belongs to
// orgID — ErrNotFound for a cross-org id, indistinguishable from a missing
// one (mirrors CampaignByIDForOrg).
func (s *Store) CampaignTemplateByIDForOrg(ctx context.Context, id, orgID uuid.UUID) (CampaignTemplate, error) {
	out, err := scanCampaignTemplate(s.db.QueryRow(ctx,
		`SELECT `+campaignTemplateCols+` FROM campaign_templates WHERE id = $1 AND organization_id = $2`, id, orgID))
	if errors.Is(err, dbx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

// ListCampaignTemplatesForOrg returns one page of the org's templates on
// one side of the active/archived split, newest-edited first, plus the
// total — the Templates tab's Active/Archived toggle is a hard filter, not
// an "all" view, so archived is a required bool rather than optional.
// query, once trimmed, substring-matches the name case- and script-
// insensitively (unicode_lower — see ListCustomers' own doc comment on why
// SQLite's built-in ASCII-only lower() would silently miss every Cyrillic
// template name); empty query matches everything.
func (s *Store) ListCampaignTemplatesForOrg(ctx context.Context, orgID uuid.UUID, archived bool, query string, limit, offset int) ([]CampaignTemplate, int, error) {
	search := ""
	if q := strings.TrimSpace(query); q != "" {
		search = "%" + strings.ToLower(q) + "%"
	}
	rows, err := s.db.Query(ctx, `SELECT `+campaignTemplateCols+`
		FROM campaign_templates
		WHERE organization_id = $1 AND is_archived = $2 AND ($3 = '' OR unicode_lower(name) LIKE $3)
		ORDER BY updated_at DESC LIMIT $4 OFFSET $5`,
		orgID, archived, search, limit, offset)
	if err != nil {
		return nil, 0, wrap("list campaign templates", err)
	}
	defer func() { _ = rows.Close() }()
	var out []CampaignTemplate
	for rows.Next() {
		t, err := scanCampaignTemplate(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM campaign_templates
		WHERE organization_id = $1 AND is_archived = $2 AND ($3 = '' OR unicode_lower(name) LIKE $3)`,
		orgID, archived, search).Scan(&total)
	return out, total, nil
}

// UpdateCampaignTemplate applies p to id unconditionally — org-scoping and
// "is there anything to apply" are the caller's job (mirrors
// UpdateCampaign's own split with internal/httpapi).
func (s *Store) UpdateCampaignTemplate(ctx context.Context, id uuid.UUID, p CampaignTemplatePatch) (CampaignTemplate, error) {
	var sets []string
	var args []any
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if p.Name != nil {
		set("name", *p.Name)
	}
	if p.MessageBody != nil {
		set("message_body", *p.MessageBody)
		vj, err := json.Marshal(purecampaign.ExtractVariables(*p.MessageBody))
		if err != nil {
			return CampaignTemplate{}, wrap("marshal template variables", err)
		}
		set("variables", string(vj))
	}
	sets = append(sets, "updated_at = strftime('%Y-%m-%d %H:%M:%f','now')")

	args = append(args, id)
	q := `UPDATE campaign_templates SET ` + joinComma(sets) + ` WHERE id = $` + itoa(len(args)) + ` RETURNING ` + campaignTemplateCols
	out, err := scanCampaignTemplate(s.db.QueryRow(ctx, q, args...))
	if errors.Is(err, dbx.ErrNoRows) {
		return CampaignTemplate{}, ErrNotFound
	}
	return out, wrap("update campaign template", err)
}

// SetCampaignTemplateArchived flips is_archived — the soft archive/restore
// action (never a delete, see the migration's own doc comment). Idempotent:
// archiving an already-archived template (or restoring an already-active
// one) is a no-op that still returns the current row, not an error.
func (s *Store) SetCampaignTemplateArchived(ctx context.Context, id uuid.UUID, archived bool) (CampaignTemplate, error) {
	out, err := scanCampaignTemplate(s.db.QueryRow(ctx, `
		UPDATE campaign_templates SET is_archived = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1
		RETURNING `+campaignTemplateCols, id, archived))
	if errors.Is(err, dbx.ErrNoRows) {
		return CampaignTemplate{}, ErrNotFound
	}
	return out, wrap("set campaign template archived", err)
}
