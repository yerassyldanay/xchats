package kbstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// Editable prompt templates (ai_prompt_templates, migration
// 20261008000000_ai_prompt_templates).
//
// Four text-ID rows exist per organization. An edit UPDATEs its own row in
// place — there are no default/custom tables and no revision history. The
// selected profile lives on the existing ai_assistants row
// (prompt_template_id); choosing one is optional and General is the default.
//
// Setup status is deliberately separate from profile selection: selecting a
// profile on an organization with no assistant settings inserts a STUB row
// with configured = FALSE, so it can never make an unconfigured assistant look
// set up (responsestore.ErrKBNotConfigured and the MCP existence check both
// require configured). The first real settings write (upsertConfigRow) flips
// configured to TRUE and leaves the selection alone.

// ErrUnknownTemplate means an ID outside the four shipped template IDs.
var ErrUnknownTemplate = errors.New("kbstore: unknown prompt template")

// PromptTemplateRow is one template as the editor sees it.
type PromptTemplateRow struct {
	ID           string
	Instructions string
	UpdatedAt    time.Time
	// IsDefaultText is true when no database row exists yet and Instructions is
	// the shipped default served from memory (an organization created outside
	// SeedOrganization). The first save creates the row.
	IsDefaultText bool
}

// PromptTemplatesView is everything the General Template tab needs.
type PromptTemplatesView struct {
	ActiveTemplateID string
	// KBConfigured reports whether assistant settings were ever saved —
	// independent of which profile is active.
	KBConfigured bool
	Templates    []PromptTemplateRow // PromptTemplateIDs() order
}

// ListPromptTemplates returns the four templates in display order plus the
// active ID and setup status. Rows missing from the database are served from
// the shipped defaults (in memory only).
func (s *Store) ListPromptTemplates(ctx context.Context, orgID uuid.UUID) (*PromptTemplatesView, error) {
	return listPromptTemplates(ctx, s.db, orgID)
}

func listPromptTemplates(ctx context.Context, db dbtx, orgID uuid.UUID) (*PromptTemplatesView, error) {
	view := &PromptTemplatesView{ActiveTemplateID: aiprompt.DefaultTemplateID}
	err := db.QueryRow(ctx, `SELECT prompt_template_id, configured FROM ai_assistants WHERE organization_id = $1`, orgID).
		Scan(&view.ActiveTemplateID, &view.KBConfigured)
	if err != nil && !errors.Is(err, dbx.ErrNoRows) {
		return nil, err
	}
	if !aiprompt.IsPromptTemplateID(view.ActiveTemplateID) {
		view.ActiveTemplateID = aiprompt.DefaultTemplateID
	}

	rows, err := db.Query(ctx, `SELECT id, instructions, updated_at FROM ai_prompt_templates WHERE organization_id = $1`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stored := map[string]PromptTemplateRow{}
	for rows.Next() {
		var r PromptTemplateRow
		if err := rows.Scan(&r.ID, &r.Instructions, &r.UpdatedAt); err != nil {
			return nil, err
		}
		stored[r.ID] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range aiprompt.PromptTemplateIDs() {
		if r, ok := stored[id]; ok {
			view.Templates = append(view.Templates, r)
			continue
		}
		text, _ := aiprompt.DefaultTemplateInstructions(id)
		view.Templates = append(view.Templates, PromptTemplateRow{ID: id, Instructions: text, IsDefaultText: true})
	}
	return view, nil
}

// ActivePromptTemplate returns the template the assistant currently runs on.
func (s *Store) ActivePromptTemplate(ctx context.Context, orgID uuid.UUID) (aiprompt.PromptTemplate, error) {
	view, err := s.ListPromptTemplates(ctx, orgID)
	if err != nil {
		return aiprompt.PromptTemplate{}, err
	}
	for _, r := range view.Templates {
		if r.ID == view.ActiveTemplateID {
			return aiprompt.PromptTemplate{ID: r.ID, Instructions: r.Instructions, UpdatedAt: r.UpdatedAt}, nil
		}
	}
	return aiprompt.PromptTemplate{}, fmt.Errorf("kbstore: active prompt template %q not found", view.ActiveTemplateID)
}

// SavePromptTemplate stores edited instructions for template id and, when
// activate is set, makes id the organization's active profile — all in one
// transaction. Unchanged text is not rewritten (updated_at, which is part of
// the prompt ref, only moves on a real edit). Other templates' rows are never
// touched, so switching profiles always preserves every other edit. Invalid
// text is rejected with the aiprompt.ErrTemplate* sentinels.
func (s *Store) SavePromptTemplate(ctx context.Context, orgID, actor uuid.UUID, id, instructions string, activate bool) error {
	if !aiprompt.IsPromptTemplateID(id) {
		return ErrUnknownTemplate
	}
	if err := aiprompt.ValidateTemplateInstructions(instructions); err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current string
	exists := true
	if err := tx.QueryRow(ctx, `SELECT instructions FROM ai_prompt_templates WHERE organization_id = $1 AND id = $2`, orgID, id).
		Scan(&current); errors.Is(err, dbx.ErrNoRows) {
		exists = false // first save for this template
	} else if err != nil {
		return err
	}
	if !exists || current != instructions {
		now := time.Now()
		if _, err := tx.Exec(ctx, `INSERT INTO ai_prompt_templates (organization_id, id, instructions, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			ON CONFLICT (organization_id, id) DO UPDATE SET instructions = EXCLUDED.instructions, updated_at = EXCLUDED.updated_at`,
			orgID, id, instructions, now); err != nil {
			return err
		}
		if err := auditRow(ctx, tx, orgID, actor, "edit", "prompt_template:"+id); err != nil {
			return err
		}
	}
	if activate {
		changed, err := selectTemplateRow(ctx, tx, orgID, id)
		if err != nil {
			return err
		}
		if changed {
			if err := auditRow(ctx, tx, orgID, actor, "edit", "prompt_template_active:"+id); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// selectTemplateRow records id as the active profile and reports whether the
// effective selection changed. It writes ONLY the selection: on an existing row
// `configured` is untouched; with no row yet it inserts a stub with
// configured = FALSE (see this file's header).
func selectTemplateRow(ctx context.Context, tx execerQuerier, orgID uuid.UUID, id string) (bool, error) {
	var current string
	err := tx.QueryRow(ctx, `SELECT prompt_template_id FROM ai_assistants WHERE organization_id = $1`, orgID).Scan(&current)
	if errors.Is(err, dbx.ErrNoRows) {
		current = aiprompt.DefaultTemplateID // no row: General is what runs
	} else if err != nil {
		return false, err
	}
	now := time.Now()
	if _, err := tx.Exec(ctx, `INSERT INTO ai_assistants (organization_id, prompt_template_id, configured, id, created_at, updated_at)
		VALUES ($1, $2, FALSE, $3, $4, $4)
		ON CONFLICT (organization_id) DO UPDATE SET prompt_template_id = EXCLUDED.prompt_template_id, updated_at = EXCLUDED.updated_at`,
		orgID, id, uuid.New(), now); err != nil {
		return false, err
	}
	return current != id, nil
}

// execerQuerier is the slice of a transaction selectTemplateRow needs.
type execerQuerier = dbx.DBTX
