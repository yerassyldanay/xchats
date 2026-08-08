package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/domain"
)

// ---------------------------------------------------------------------------
// Customer management (CRM) — customers, channel identities, tags, statuses,
// custom-field definitions.
// ---------------------------------------------------------------------------
// This layer sits above channel transport. A customer is org-owned; the link
// to a channel is one crm_customer_identities row per transport contact
// (wa_contacts / tg_contacts), which is why merging two customers is a single
// UPDATE here rather than a rewrite of every channel's tables.
//
// crm_customer_identities.account_id/contact_id carry no foreign key: each can
// reference a wa_* or a tg_* row and a single FK cannot express an either/or
// reference — the same constraint ai_drafts.chat_id lives with (see
// 0003_ai_engine.up.sql's file header). Ownership is enforced here instead:
// every read and write below takes an organization id and puts it in the WHERE
// clause, so a cross-org id resolves to no row rather than to someone else's
// customer.

// CustomerStatus is one entry in the organization's configurable lifecycle.
type CustomerStatus struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	Color          string
	Position       int
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CustomerTag is one org-owned label. The product brief's examples (VIP,
// xpayment, «Не готов», …) are deliberately not seeded anywhere — an
// organization creates its own.
type CustomerTag struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	Color          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CustomerIdentity is one channel identity belonging to a customer. ExternalID
// is the provider's own identity for the person — the phone JID for
// WhatsApp/simulator, the numeric user id for Telegram — mirroring
// store.Contact.ExternalContactRef's naming.
type CustomerIdentity struct {
	ID          uuid.UUID
	CustomerID  uuid.UUID
	Channel     string
	AccountID   uuid.UUID
	ContactID   uuid.UUID
	ExternalID  string
	Username    string
	Phone       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Customer is the unified person. There is no Notes field: notes are
// crm_customer_notes rows (authored, timestamped, part of the timeline), and
// the profile renders the latest one — see LatestNote on CustomerProfile.
//
// MergedIntoID is a tombstone marker: a merged-away customer keeps its row so
// an id captured before the merge still resolves to its survivor, and so the
// merge stays auditable. Every list query filters it out.
type Customer struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	DisplayName    string
	Phone          string
	Email          string
	AvatarURL      string
	StatusID       uuid.NullUUID
	AssigneeUserID uuid.NullUUID
	CustomFields   []byte
	MergedIntoID   uuid.NullUUID
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Status/Tags/Identities are populated by the reads that join them
	// (CustomerByID, ListCustomers); a bare row read leaves them nil.
	Status     *CustomerStatus
	Tags       []CustomerTag
	Identities []CustomerIdentity
}

// CustomFieldDef is one org-defined extra profile field. Values live in
// crm_customers.custom_fields keyed by Key.
type CustomFieldDef struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Key            string
	Label          string
	FieldType      string
	Options        []byte
	Position       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// defaultStatuses is the lifecycle every organization starts with. It is the
// exact set 0011_crm.up.sql seeds for organizations that predate the
// migration; EnsureDefaultStatuses applies it to every organization created
// after. Both are idempotent via UNIQUE (organization_id, slug), so running
// one after the other is a no-op rather than a duplicate.
var defaultStatuses = []CustomerStatus{
	{Slug: "new", Name: "Новый", Color: "#64748b", Position: 1, IsDefault: true},
	{Slug: "interested", Name: "Интересуется", Color: "#0ea5e9", Position: 2},
	{Slug: "in_progress", Name: "В работе", Color: "#f59e0b", Position: 3},
	{Slug: "customer", Name: "Клиент", Color: "#10b981", Position: 4},
	{Slug: "lost", Name: "Потерян", Color: "#ef4444", Position: 5},
}

// EnsureDefaultStatuses seeds defaultStatuses for orgID if it has none. Called
// from the CRM handlers' org guard so an organization created after migration
// 0011 gets the same starting lifecycle, without a backfill job.
func (s *Store) EnsureDefaultStatuses(ctx context.Context, orgID uuid.UUID) error {
	var n int
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM crm_statuses WHERE organization_id = $1`, orgID).Scan(&n); err != nil {
		return wrap("count statuses", err)
	}
	if n > 0 {
		return nil
	}
	for _, d := range defaultStatuses {
		if _, err := s.db.Exec(ctx, `
			INSERT INTO crm_statuses (organization_id, slug, name, color, position, is_default)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (organization_id, slug) DO NOTHING`,
			orgID, d.Slug, d.Name, d.Color, d.Position, d.IsDefault); err != nil {
			return wrap("seed status", err)
		}
	}
	return nil
}

// --- statuses --------------------------------------------------------------

const statusCols = `id, organization_id, slug, name, color, position, is_default, created_at, updated_at`

func scanStatusDst(st *CustomerStatus) []any {
	return []any{&st.ID, &st.OrganizationID, &st.Slug, &st.Name, &st.Color, &st.Position,
		&st.IsDefault, &st.CreatedAt, &st.UpdatedAt}
}

// ListStatuses returns the org's lifecycle in display order.
func (s *Store) ListStatuses(ctx context.Context, orgID uuid.UUID) ([]CustomerStatus, error) {
	rows, err := s.db.Query(ctx, `SELECT `+statusCols+`
		FROM crm_statuses WHERE organization_id = $1 ORDER BY position, name`, orgID)
	if err != nil {
		return nil, wrap("list statuses", err)
	}
	defer func() { _ = rows.Close() }()
	out := []CustomerStatus{}
	for rows.Next() {
		var st CustomerStatus
		if err := rows.Scan(scanStatusDst(&st)...); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// CreateStatus adds one lifecycle entry. A duplicate slug is a conflict, not a
// silent upsert — the caller surfaces it as 409.
func (s *Store) CreateStatus(ctx context.Context, orgID uuid.UUID, in CustomerStatus) (CustomerStatus, error) {
	var st CustomerStatus
	err := s.db.QueryRow(ctx, `
		INSERT INTO crm_statuses (organization_id, slug, name, color, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+statusCols,
		orgID, in.Slug, in.Name, in.Color, in.Position).Scan(scanStatusDst(&st)...)
	return st, wrapDup("create status", err)
}

// UpdateStatus renames/recolors/reorders one entry. The slug is immutable:
// it is the natural key customers were seeded against.
func (s *Store) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, in CustomerStatus) (CustomerStatus, error) {
	var st CustomerStatus
	err := s.db.QueryRow(ctx, `
		UPDATE crm_statuses SET name = $3, color = $4, position = $5,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2
		RETURNING `+statusCols,
		orgID, id, in.Name, in.Color, in.Position).Scan(scanStatusDst(&st)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return st, ErrNotFound
	}
	return st, wrapDup("update status", err)
}

// DeleteStatus removes one entry. Customers holding it fall back to NULL
// (the FK is ON DELETE SET NULL), which the UI renders as «Без статуса» —
// deleting a status must never delete customers.
func (s *Store) DeleteStatus(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM crm_statuses WHERE organization_id = $1 AND id = $2`, orgID, id)
	if err != nil {
		return wrap("delete status", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- tags ------------------------------------------------------------------

// tagCols is the bare column list for single-table tag reads; tagColsT is the
// `t.`-qualified form the customer-tags join needs — crm_customer_tags and
// crm_tags both have created_at, so an unqualified list is ambiguous there.
const tagCols = `id, organization_id, slug, name, color, created_at, updated_at`

const tagColsT = `t.id, t.organization_id, t.slug, t.name, t.color, t.created_at, t.updated_at`

func scanTagDst(t *CustomerTag) []any {
	return []any{&t.ID, &t.OrganizationID, &t.Slug, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt}
}

func (s *Store) ListTags(ctx context.Context, orgID uuid.UUID) ([]CustomerTag, error) {
	rows, err := s.db.Query(ctx, `SELECT `+tagCols+`
		FROM crm_tags WHERE organization_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, wrap("list tags", err)
	}
	defer func() { _ = rows.Close() }()
	out := []CustomerTag{}
	for rows.Next() {
		var t CustomerTag
		if err := rows.Scan(scanTagDst(&t)...); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTag(ctx context.Context, orgID uuid.UUID, in CustomerTag) (CustomerTag, error) {
	var t CustomerTag
	err := s.db.QueryRow(ctx, `
		INSERT INTO crm_tags (organization_id, slug, name, color)
		VALUES ($1, $2, $3, $4)
		RETURNING `+tagCols, orgID, in.Slug, in.Name, in.Color).Scan(scanTagDst(&t)...)
	return t, wrapDup("create tag", err)
}

func (s *Store) UpdateTag(ctx context.Context, orgID, id uuid.UUID, in CustomerTag) (CustomerTag, error) {
	var t CustomerTag
	err := s.db.QueryRow(ctx, `
		UPDATE crm_tags SET name = $3, color = $4,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2
		RETURNING `+tagCols, orgID, id, in.Name, in.Color).Scan(scanTagDst(&t)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, wrapDup("update tag", err)
}

// DeleteTag removes the tag and, via ON DELETE CASCADE on crm_customer_tags,
// its assignment to every customer. Customers themselves are untouched.
func (s *Store) DeleteTag(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM crm_tags WHERE organization_id = $1 AND id = $2`, orgID, id)
	if err != nil {
		return wrap("delete tag", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddCustomerTag links a tag to a customer. Both ids are org-checked in the
// same statement, so a tag from another organization silently matches nothing
// rather than crossing the tenant boundary.
func (s *Store) AddCustomerTag(ctx context.Context, orgID, customerID, tagID uuid.UUID, actor uuid.NullUUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var name string
	if err := tx.QueryRow(ctx, `
		SELECT t.name FROM crm_tags t
		JOIN crm_customers c ON c.id = $2 AND c.organization_id = t.organization_id
		WHERE t.id = $3 AND t.organization_id = $1`, orgID, customerID, tagID).Scan(&name); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return ErrNotFound
		}
		return wrap("resolve tag", err)
	}
	res, err := tx.Exec(ctx, `
		INSERT INTO crm_customer_tags (customer_id, tag_id) VALUES ($1, $2)
		ON CONFLICT (customer_id, tag_id) DO NOTHING`, customerID, tagID)
	if err != nil {
		return wrap("add customer tag", err)
	}
	// Only a genuine new link is timeline-worthy; re-adding an existing tag is
	// a no-op, not an event.
	if res.RowsAffected() > 0 {
		if err := appendTimeline(ctx, tx, orgID, customerID, timelineEvent{
			Kind: TimelineTagAdded, Actor: actor, Summary: name,
			Detail: jsonObject(map[string]any{"tag_id": tagID.String(), "name": name}),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) RemoveCustomerTag(ctx context.Context, orgID, customerID, tagID uuid.UUID, actor uuid.NullUUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var name string
	if err := tx.QueryRow(ctx, `
		SELECT t.name FROM crm_tags t
		JOIN crm_customers c ON c.id = $2 AND c.organization_id = t.organization_id
		WHERE t.id = $3 AND t.organization_id = $1`, orgID, customerID, tagID).Scan(&name); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return ErrNotFound
		}
		return wrap("resolve tag", err)
	}
	res, err := tx.Exec(ctx,
		`DELETE FROM crm_customer_tags WHERE customer_id = $1 AND tag_id = $2`, customerID, tagID)
	if err != nil {
		return wrap("remove customer tag", err)
	}
	if res.RowsAffected() > 0 {
		if err := appendTimeline(ctx, tx, orgID, customerID, timelineEvent{
			Kind: TimelineTagRemoved, Actor: actor, Summary: name,
			Detail: jsonObject(map[string]any{"tag_id": tagID.String(), "name": name}),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// --- custom field definitions ---------------------------------------------

const customFieldCols = `id, organization_id, key, label, field_type, options, position, created_at, updated_at`

func scanCustomFieldDst(f *CustomFieldDef) []any {
	return []any{&f.ID, &f.OrganizationID, &f.Key, &f.Label, &f.FieldType, &f.Options,
		&f.Position, &f.CreatedAt, &f.UpdatedAt}
}

func (s *Store) ListCustomFieldDefs(ctx context.Context, orgID uuid.UUID) ([]CustomFieldDef, error) {
	rows, err := s.db.Query(ctx, `SELECT `+customFieldCols+`
		FROM crm_custom_field_defs WHERE organization_id = $1 ORDER BY position, label`, orgID)
	if err != nil {
		return nil, wrap("list custom fields", err)
	}
	defer func() { _ = rows.Close() }()
	out := []CustomFieldDef{}
	for rows.Next() {
		var f CustomFieldDef
		if err := rows.Scan(scanCustomFieldDst(&f)...); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) CreateCustomFieldDef(ctx context.Context, orgID uuid.UUID, in CustomFieldDef) (CustomFieldDef, error) {
	var f CustomFieldDef
	err := s.db.QueryRow(ctx, `
		INSERT INTO crm_custom_field_defs (organization_id, key, label, field_type, options, position)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+customFieldCols,
		orgID, in.Key, in.Label, in.FieldType, jsonOrDefault(in.Options, "[]"), in.Position).
		Scan(scanCustomFieldDst(&f)...)
	return f, wrapDup("create custom field", err)
}

func (s *Store) UpdateCustomFieldDef(ctx context.Context, orgID, id uuid.UUID, in CustomFieldDef) (CustomFieldDef, error) {
	var f CustomFieldDef
	err := s.db.QueryRow(ctx, `
		UPDATE crm_custom_field_defs SET label = $3, field_type = $4, options = $5, position = $6,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2
		RETURNING `+customFieldCols,
		orgID, id, in.Label, in.FieldType, jsonOrDefault(in.Options, "[]"), in.Position).
		Scan(scanCustomFieldDst(&f)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, wrapDup("update custom field", err)
}

// DeleteCustomFieldDef removes the definition. Values already stored under its
// key in crm_customers.custom_fields are left alone — they simply stop being
// rendered, so re-creating the field brings the data back rather than losing it
// to a definition edit.
func (s *Store) DeleteCustomFieldDef(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM crm_custom_field_defs WHERE organization_id = $1 AND id = $2`, orgID, id)
	if err != nil {
		return wrap("delete custom field", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- customers -------------------------------------------------------------

// customerCols is the alias-qualified projection the list/lookup reads use;
// customerColsBare is the same list for statements whose only table is
// crm_customers (INSERT ... RETURNING, which cannot use an alias).
const customerCols = `c.id, c.organization_id, c.display_name, c.phone, c.email, c.avatar_url,
	c.status_id, c.assignee_user_id, c.custom_fields, c.merged_into_id, c.created_at, c.updated_at`

const customerColsBare = `id, organization_id, display_name, phone, email, avatar_url,
	status_id, assignee_user_id, custom_fields, merged_into_id, created_at, updated_at`

func scanCustomerDst(c *Customer) []any {
	return []any{&c.ID, &c.OrganizationID, &c.DisplayName, &c.Phone, &c.Email, &c.AvatarURL,
		&c.StatusID, &c.AssigneeUserID, &c.CustomFields, &c.MergedIntoID, &c.CreatedAt, &c.UpdatedAt}
}

// CustomerFilter is the customers list's filter set — the search/filter surface
// the product brief asks for, built the same way ChatFilter is (positional
// args accumulated alongside the WHERE fragments that reference them).
//
// Followup selects on the customer's open follow-ups: "today" and "overdue" are
// resolved against DayStartUTC/DayEndUTC, which the caller computes from the
// client's timezone offset — organizations have no stored timezone.
type CustomerFilter struct {
	OrgID       uuid.UUID
	Query       string
	StatusID    uuid.NullUUID
	TagID       uuid.NullUUID
	Channel     string
	Assignee    string // me|unassigned|<uuid>
	MeUserID    uuid.UUID
	Followup    string // today|overdue|any|none
	DayStartUTC time.Time
	DayEndUTC   time.Time
	Limit       int
	Offset      int
}

// ListCustomers returns one page of the org's live customers plus the total.
// Merged-away rows are excluded everywhere: they are tombstones, not people.
func (s *Store) ListCustomers(ctx context.Context, f CustomerFilter) ([]Customer, int, error) {
	var where []string
	var args []any
	args = append(args, f.OrgID)
	where = append(where, "c.organization_id = $1", "c.merged_into_id IS NULL")

	if f.Query != "" {
		args = append(args, "%"+strings.ToLower(f.Query)+"%")
		i := itoa(len(args))
		// unicode_lower, not SQLite's ASCII-only lower(): this product's names
		// are mostly Cyrillic, and lower() would leave «Алия» unchanged so a
		// search for "али" could never match it (see internal/dbx's
		// unicodelower.go).
		//
		// Identity handles are searchable too, so pasting a phone number or an
		// @username from another tool finds the customer even when the profile's
		// own phone/email fields were never filled in.
		where = append(where, `(unicode_lower(c.display_name) LIKE $`+i+
			` OR unicode_lower(c.phone) LIKE $`+i+
			` OR unicode_lower(c.email) LIKE $`+i+
			` OR EXISTS (SELECT 1 FROM crm_customer_identities ci WHERE ci.customer_id = c.id
				AND (unicode_lower(ci.username) LIKE $`+i+` OR unicode_lower(ci.phone) LIKE $`+i+
			` OR unicode_lower(ci.display_name) LIKE $`+i+` OR unicode_lower(ci.external_id) LIKE $`+i+`)))`)
	}
	if f.StatusID.Valid {
		args = append(args, f.StatusID.UUID)
		where = append(where, "c.status_id = $"+itoa(len(args)))
	}
	if f.TagID.Valid {
		args = append(args, f.TagID.UUID)
		where = append(where, "EXISTS (SELECT 1 FROM crm_customer_tags ct WHERE ct.customer_id = c.id AND ct.tag_id = $"+itoa(len(args))+")")
	}
	if f.Channel != "" {
		args = append(args, f.Channel)
		where = append(where, "EXISTS (SELECT 1 FROM crm_customer_identities ci WHERE ci.customer_id = c.id AND ci.channel = $"+itoa(len(args))+")")
	}
	switch {
	case f.Assignee == "me":
		args = append(args, f.MeUserID)
		where = append(where, "c.assignee_user_id = $"+itoa(len(args)))
	case f.Assignee == "unassigned":
		where = append(where, "c.assignee_user_id IS NULL")
	case f.Assignee != "":
		if id, err := uuid.Parse(f.Assignee); err == nil {
			args = append(args, id)
			where = append(where, "c.assignee_user_id = $"+itoa(len(args)))
		}
	}
	switch f.Followup {
	case "any":
		where = append(where, openFollowupExists(""))
	case "none":
		where = append(where, "NOT "+openFollowupExists(""))
	case "today":
		args = append(args, f.DayStartUTC, f.DayEndUTC)
		where = append(where, openFollowupExists("AND fu.due_at >= $"+itoa(len(args)-1)+" AND fu.due_at < $"+itoa(len(args))))
	case "overdue":
		args = append(args, f.DayStartUTC)
		where = append(where, openFollowupExists("AND fu.due_at < $"+itoa(len(args))))
	}

	clause := strings.Join(where, " AND ")
	args = append(args, f.Limit, f.Offset)
	q := `SELECT ` + customerCols + ` FROM crm_customers c
		WHERE ` + clause + `
		ORDER BY c.updated_at DESC
		LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, wrap("list customers", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(scanCustomerDst(&c)...); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM crm_customers c WHERE `+clause, args[:len(args)-2]...).Scan(&total); err != nil {
		return nil, 0, wrap("count customers", err)
	}
	if err := s.hydrateCustomers(ctx, f.OrgID, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// openFollowupExists builds the EXISTS fragment the follow-up filters share.
// extra is appended inside the subquery (already-bound placeholders).
func openFollowupExists(extra string) string {
	return `EXISTS (SELECT 1 FROM crm_followups fu
		WHERE fu.customer_id = c.id AND fu.state = 'open' ` + extra + `)`
}

// hydrateCustomers fills Status/Tags/Identities for a page of customers in
// three queries rather than three per row.
func (s *Store) hydrateCustomers(ctx context.Context, orgID uuid.UUID, cs []Customer) error {
	if len(cs) == 0 {
		return nil
	}
	byID := make(map[uuid.UUID]*Customer, len(cs))
	ids := make([]any, 0, len(cs))
	for i := range cs {
		byID[cs[i].ID] = &cs[i]
		ids = append(ids, cs[i].ID)
	}
	in := placeholders(1, len(ids))

	statuses, err := s.ListStatuses(ctx, orgID)
	if err != nil {
		return err
	}
	byStatus := make(map[uuid.UUID]CustomerStatus, len(statuses))
	for _, st := range statuses {
		byStatus[st.ID] = st
	}
	for _, c := range byID {
		if c.StatusID.Valid {
			if st, ok := byStatus[c.StatusID.UUID]; ok {
				stCopy := st
				c.Status = &stCopy
			}
		}
	}

	tagRows, err := s.db.Query(ctx, `
		SELECT ct.customer_id, `+tagColsT+`
		FROM crm_customer_tags ct JOIN crm_tags t ON t.id = ct.tag_id
		WHERE ct.customer_id IN (`+in+`) ORDER BY t.name`, ids...)
	if err != nil {
		return wrap("load customer tags", err)
	}
	defer func() { _ = tagRows.Close() }()
	for tagRows.Next() {
		var cid uuid.UUID
		var t CustomerTag
		if err := tagRows.Scan(append([]any{&cid}, scanTagDst(&t)...)...); err != nil {
			return err
		}
		if c, ok := byID[cid]; ok {
			c.Tags = append(c.Tags, t)
		}
	}
	if err := tagRows.Err(); err != nil {
		return err
	}

	idRows, err := s.db.Query(ctx, `SELECT `+identityCols+`
		FROM crm_customer_identities WHERE customer_id IN (`+in+`) ORDER BY channel, created_at`, ids...)
	if err != nil {
		return wrap("load customer identities", err)
	}
	defer func() { _ = idRows.Close() }()
	for idRows.Next() {
		var ci CustomerIdentity
		if err := idRows.Scan(scanIdentityDst(&ci)...); err != nil {
			return err
		}
		if c, ok := byID[ci.CustomerID]; ok {
			c.Identities = append(c.Identities, ci)
		}
	}
	return idRows.Err()
}

// CustomerByID returns one live customer with status, tags and identities.
// A merged-away id resolves to its survivor, so a link captured before a merge
// still opens the right profile instead of 404ing.
func (s *Store) CustomerByID(ctx context.Context, orgID, id uuid.UUID) (Customer, error) {
	var c Customer
	err := s.db.QueryRow(ctx, `SELECT `+customerCols+`
		FROM crm_customers c WHERE c.organization_id = $1 AND c.id = $2`, orgID, id).
		Scan(scanCustomerDst(&c)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, wrap("load customer", err)
	}
	if c.MergedIntoID.Valid {
		return s.CustomerByID(ctx, orgID, c.MergedIntoID.UUID)
	}
	// hydrateCustomers fills through the slice's own backing array, so read the
	// result back out of it rather than out of the local c.
	one := []Customer{c}
	if err := s.hydrateCustomers(ctx, orgID, one); err != nil {
		return c, err
	}
	return one[0], nil
}

// CustomerPatch is a partial profile edit: a nil field means "not touched",
// mirroring how kbstore's DraftConfigPatch distinguishes absent from empty.
type CustomerPatch struct {
	DisplayName    *string
	Phone          *string
	Email          *string
	AvatarURL      *string
	StatusID       *uuid.NullUUID
	AssigneeUserID *uuid.NullUUID
	CustomFields   []byte
}

// CreateCustomer inserts a manually-created customer (the "add customer"
// button — inbound messages go through ResolveCustomerForContact instead) and
// records the creation on its timeline.
func (s *Store) CreateCustomer(ctx context.Context, orgID uuid.UUID, in CustomerPatch, actor uuid.NullUUID) (Customer, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Customer{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	statusID, err := defaultStatusID(ctx, tx, orgID)
	if err != nil {
		return Customer{}, err
	}
	if in.StatusID != nil {
		statusID = *in.StatusID
	}
	var c Customer
	if err := tx.QueryRow(ctx, `
		INSERT INTO crm_customers (organization_id, display_name, phone, email, avatar_url,
			status_id, assignee_user_id, custom_fields)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+customerColsBare,
		orgID, derefString(in.DisplayName), derefString(in.Phone), derefString(in.Email),
		derefString(in.AvatarURL), statusID, derefNullUUID(in.AssigneeUserID),
		jsonOrDefault(in.CustomFields, "{}")).Scan(scanCustomerDst(&c)...); err != nil {
		return Customer{}, wrapDup("create customer", err)
	}
	if err := appendTimeline(ctx, tx, orgID, c.ID, timelineEvent{
		Kind: TimelineCustomerCreated, Actor: actor, Summary: c.DisplayName,
	}); err != nil {
		return Customer{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Customer{}, err
	}
	return s.CustomerByID(ctx, orgID, c.ID)
}

// UpdateCustomer applies a partial edit and writes a timeline event for each
// field the product treats as an event (status, assignee) — plain profile
// edits (name/phone/email) are not events, they are corrections.
func (s *Store) UpdateCustomer(ctx context.Context, orgID, id uuid.UUID, p CustomerPatch, actor uuid.NullUUID) (Customer, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Customer{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before Customer
	if err := tx.QueryRow(ctx, `SELECT `+customerCols+`
		FROM crm_customers c WHERE c.organization_id = $1 AND c.id = $2`, orgID, id).
		Scan(scanCustomerDst(&before)...); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return Customer{}, ErrNotFound
		}
		return Customer{}, wrap("load customer", err)
	}

	sets := []string{"updated_at = strftime('%Y-%m-%d %H:%M:%f','now')"}
	args := []any{orgID, id}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if p.DisplayName != nil {
		add("display_name", *p.DisplayName)
	}
	if p.Phone != nil {
		add("phone", *p.Phone)
	}
	if p.Email != nil {
		add("email", *p.Email)
	}
	if p.AvatarURL != nil {
		add("avatar_url", *p.AvatarURL)
	}
	if p.StatusID != nil {
		add("status_id", *p.StatusID)
	}
	if p.AssigneeUserID != nil {
		add("assignee_user_id", *p.AssigneeUserID)
	}
	if p.CustomFields != nil {
		add("custom_fields", jsonOrDefault(p.CustomFields, "{}"))
	}
	if _, err := tx.Exec(ctx, `UPDATE crm_customers SET `+strings.Join(sets, ", ")+`
		WHERE organization_id = $1 AND id = $2`, args...); err != nil {
		return Customer{}, wrap("update customer", err)
	}

	if p.StatusID != nil && *p.StatusID != before.StatusID {
		from, to, err := statusNames(ctx, tx, before.StatusID, *p.StatusID)
		if err != nil {
			return Customer{}, err
		}
		if err := appendTimeline(ctx, tx, orgID, id, timelineEvent{
			Kind: TimelineStatusChanged, Actor: actor, Summary: from + " → " + to,
			Detail: jsonObject(map[string]any{"from": from, "to": to}),
		}); err != nil {
			return Customer{}, err
		}
	}
	if p.AssigneeUserID != nil && *p.AssigneeUserID != before.AssigneeUserID {
		to := "—"
		if p.AssigneeUserID.Valid {
			if n, err := userDisplayName(ctx, tx, p.AssigneeUserID.UUID); err == nil {
				to = n
			}
		}
		if err := appendTimeline(ctx, tx, orgID, id, timelineEvent{
			Kind: TimelineAssigneeChanged, Actor: actor, Summary: to,
			Detail: jsonObject(map[string]any{"assignee": to}),
		}); err != nil {
			return Customer{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Customer{}, err
	}
	return s.CustomerByID(ctx, orgID, id)
}

// --- identities and inbound resolution -------------------------------------

const identityCols = `id, customer_id, channel, account_id, contact_id, external_id,
	username, phone, display_name, created_at, updated_at`

func scanIdentityDst(ci *CustomerIdentity) []any {
	return []any{&ci.ID, &ci.CustomerID, &ci.Channel, &ci.AccountID, &ci.ContactID,
		&ci.ExternalID, &ci.Username, &ci.Phone, &ci.DisplayName, &ci.CreatedAt, &ci.UpdatedAt}
}

// IdentityInput is one channel identity as observed on an inbound message.
type IdentityInput struct {
	Channel     string
	AccountID   uuid.UUID
	ContactID   uuid.UUID
	ExternalID  string
	Username    string
	Phone       string
	DisplayName string
}

// ResolveCustomerForContact is the find-or-create the product brief specifies:
// look up the external identity; use its customer if it exists; otherwise
// create a customer and the identity, then attach the conversation to it.
//
// It runs inside the caller's inbound transaction (UpsertInbound,
// IngestTelegramInbound) so a message and its customer link commit together,
// and it is idempotent under provider redelivery — the UNIQUE key does the
// deduplication, not a prior read.
//
// Resolution is by provider identity ONLY. Names and phone numbers are never
// compared, so two channels never collapse into one customer without an
// explicit manual merge (MergeCustomers).
//
// orgID may be uuid.Nil when the owning account is not assigned to an
// organization; that is not an error, it is a no-op — such chats are already
// invisible to every org's inbox, so there is no customer to own them.
func ResolveCustomerForContact(ctx context.Context, tx *dbx.Tx, orgID uuid.UUID, in IdentityInput) (uuid.UUID, error) {
	if orgID == uuid.Nil {
		return uuid.Nil, nil
	}
	var customerID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT customer_id FROM crm_customer_identities
		WHERE organization_id = $1 AND channel = $2 AND account_id = $3 AND external_id = $4`,
		orgID, in.Channel, in.AccountID, in.ExternalID).Scan(&customerID)
	switch {
	case err == nil:
		// Keep the identity's provider-side display data fresh (a WhatsApp push
		// name or a Telegram @username can change), but never touch the
		// customer's own edited profile fields from an inbound message.
		if _, err := tx.Exec(ctx, `
			UPDATE crm_customer_identities SET
				username = CASE WHEN $1 <> '' THEN $1 ELSE username END,
				phone = CASE WHEN $2 <> '' THEN $2 ELSE phone END,
				display_name = CASE WHEN $3 <> '' THEN $3 ELSE display_name END,
				contact_id = $4,
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE organization_id = $5 AND channel = $6 AND account_id = $7 AND external_id = $8`,
			in.Username, in.Phone, in.DisplayName, in.ContactID,
			orgID, in.Channel, in.AccountID, in.ExternalID); err != nil {
			return uuid.Nil, wrap("refresh identity", err)
		}
		return customerID, nil
	case !errors.Is(err, dbx.ErrNoRows):
		return uuid.Nil, wrap("lookup identity", err)
	}

	statusID, err := defaultStatusID(ctx, tx, orgID)
	if err != nil {
		return uuid.Nil, err
	}
	name := in.DisplayName
	if name == "" {
		name = in.Username
	}
	if name == "" {
		name = in.Phone
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO crm_customers (organization_id, display_name, phone, status_id)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		orgID, name, in.Phone, statusID).Scan(&customerID); err != nil {
		return uuid.Nil, wrap("create customer", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO crm_customer_identities (organization_id, customer_id, channel, account_id,
			contact_id, external_id, username, phone, display_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (channel, contact_id) DO NOTHING`,
		orgID, customerID, in.Channel, in.AccountID, in.ContactID, in.ExternalID,
		in.Username, in.Phone, in.DisplayName); err != nil {
		return uuid.Nil, wrap("create identity", err)
	}
	if err := appendTimeline(ctx, tx, orgID, customerID, timelineEvent{
		Kind: TimelineCustomerCreated, Summary: name,
		Detail: jsonObject(map[string]any{"channel": in.Channel, "source": "inbound"}),
	}); err != nil {
		return uuid.Nil, err
	}
	if err := appendTimeline(ctx, tx, orgID, customerID, timelineEvent{
		Kind: TimelineIdentityLinked, Summary: in.Channel,
		Detail: jsonObject(map[string]any{"channel": in.Channel, "external_id": in.ExternalID}),
	}); err != nil {
		return uuid.Nil, err
	}
	return customerID, nil
}

// accountOwner reads the owning organization (and, for the wa_* leg, the
// channel) of an account inside an inbound transaction. table is a package
// constant, never caller input.
//
// A NULL organization_id — an account nobody has assigned yet — comes back as
// uuid.Nil, which ResolveCustomerForContact treats as "no customer to create".
func accountOwner(ctx context.Context, tx *dbx.Tx, table string, accountID uuid.UUID) (uuid.UUID, string, error) {
	var org uuid.NullUUID
	var channel string
	var err error
	if table == "wa_accounts" {
		err = tx.QueryRow(ctx,
			`SELECT organization_id, channel FROM wa_accounts WHERE id = $1`, accountID).Scan(&org, &channel)
	} else {
		channel = string(chanTelegram)
		err = tx.QueryRow(ctx,
			`SELECT organization_id FROM tg_accounts WHERE id = $1`, accountID).Scan(&org)
	}
	if err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return uuid.Nil, "", nil
		}
		return uuid.Nil, "", wrap("account owner", err)
	}
	if !org.Valid {
		return uuid.Nil, channel, nil
	}
	return org.UUID, channel, nil
}

// CustomerConversations returns every chat belonging to a customer, across all
// its channel identities, newest first. Channel context is preserved — these
// are the same store.Chat rows the inbox renders, not a flattened merge.
func (s *Store) CustomerConversations(ctx context.Context, orgID, customerID uuid.UUID) ([]Chat, error) {
	rows, err := s.db.Query(ctx, `SELECT `+chatCols+`
		FROM inbox_chats_v c`+chatCustomerJoin+`
		WHERE c.organization_id = $1 AND ci.customer_id = $2 AND c.account_deleted_at IS NULL
		ORDER BY c.last_message_at DESC NULLS LAST`, orgID, customerID)
	if err != nil {
		return nil, wrap("customer conversations", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Chat{}
	for rows.Next() {
		var c Chat
		if err := rows.Scan(scanChatDst(&c)...); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CustomerSummary is the read-only projection the response engine is allowed
// to see: what a manager would want the assistant to know, and nothing more.
// It carries no ids, no custom fields and no free-text profile columns —
// whatever reaches a model prompt must be deliberately chosen, not "whatever
// the row happened to have" (see plan/DECISIONS.md's core principles).
type CustomerSummary struct {
	Name       string
	Status     string
	Tags       []string
	LatestNote string
	NextAction string
	NextDueOn  string
}

// CustomerContextForChat resolves the customer behind a conversation and
// returns their summary for the prompt builder. ok is false when the chat has
// no customer yet — the caller then renders no customer block at all, which is
// what keeps the prompt byte-identical to the eval harness's fixtures.
func (s *Store) CustomerContextForChat(ctx context.Context, chatID uuid.UUID) (CustomerSummary, bool, error) {
	var orgID, customerID uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT c.organization_id, ci.customer_id
		FROM inbox_chats_v c`+chatCustomerJoin+`
		WHERE c.id = $1 AND ci.customer_id IS NOT NULL`, chatID).Scan(&orgID, &customerID)
	if errors.Is(err, dbx.ErrNoRows) {
		return CustomerSummary{}, false, nil
	}
	if err != nil {
		return CustomerSummary{}, false, wrap("customer for chat", err)
	}

	cust, err := s.CustomerByID(ctx, orgID, customerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return CustomerSummary{}, false, nil
		}
		return CustomerSummary{}, false, err
	}
	out := CustomerSummary{Name: cust.DisplayName}
	if cust.Status != nil {
		out.Status = cust.Status.Name
	}
	for _, t := range cust.Tags {
		out.Tags = append(out.Tags, t.Name)
	}
	if note, err := s.LatestCustomerNote(ctx, orgID, customerID); err == nil {
		out.LatestNote = note.Body
	} else if !errors.Is(err, ErrNotFound) {
		return CustomerSummary{}, false, err
	}
	if fu, err := s.NextOpenFollowup(ctx, orgID, customerID); err == nil {
		out.NextAction = fu.Action
		out.NextDueOn = fu.DueDate
	} else if !errors.Is(err, ErrNotFound) {
		return CustomerSummary{}, false, err
	}
	return out, true, nil
}

// --- shared helpers --------------------------------------------------------

// defaultStatusID returns the org's is_default status, seeding the default
// lifecycle first if the organization has none (an org created after migration
// 0011 ran). uuid.NullUUID{} when the org has statuses but none is marked
// default — a customer with no status is a valid state.
func defaultStatusID(ctx context.Context, tx *dbx.Tx, orgID uuid.UUID) (uuid.NullUUID, error) {
	var id uuid.NullUUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM crm_statuses WHERE organization_id = $1 AND is_default = TRUE`, orgID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, dbx.ErrNoRows) {
		return id, wrap("default status", err)
	}
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM crm_statuses WHERE organization_id = $1`, orgID).Scan(&n); err != nil {
		return id, wrap("count statuses", err)
	}
	if n > 0 {
		return uuid.NullUUID{}, nil
	}
	for _, d := range defaultStatuses {
		if _, err := tx.Exec(ctx, `
			INSERT INTO crm_statuses (organization_id, slug, name, color, position, is_default)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (organization_id, slug) DO NOTHING`,
			orgID, d.Slug, d.Name, d.Color, d.Position, d.IsDefault); err != nil {
			return id, wrap("seed status", err)
		}
	}
	if err := tx.QueryRow(ctx,
		`SELECT id FROM crm_statuses WHERE organization_id = $1 AND is_default = TRUE`, orgID).Scan(&id); err != nil {
		return uuid.NullUUID{}, nil
	}
	return id, nil
}

func statusNames(ctx context.Context, tx *dbx.Tx, from, to uuid.NullUUID) (string, string, error) {
	name := func(u uuid.NullUUID) (string, error) {
		if !u.Valid {
			return "—", nil
		}
		var n string
		if err := tx.QueryRow(ctx, `SELECT name FROM crm_statuses WHERE id = $1`, u.UUID).Scan(&n); err != nil {
			if errors.Is(err, dbx.ErrNoRows) {
				return "—", nil
			}
			return "", wrap("status name", err)
		}
		return n, nil
	}
	f, err := name(from)
	if err != nil {
		return "", "", err
	}
	t, err := name(to)
	return f, t, err
}

func userDisplayName(ctx context.Context, tx *dbx.Tx, id uuid.UUID) (string, error) {
	var name, email string
	if err := tx.QueryRow(ctx, `SELECT display_name, email FROM users WHERE id = $1`, id).Scan(&name, &email); err != nil {
		return "", err
	}
	if name == "" {
		return email, nil
	}
	return name, nil
}

// placeholders renders "$n, $n+1, …" for an IN list starting at start.
func placeholders(start, n int) string {
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, "$"+itoa(start+i))
	}
	return strings.Join(parts, ", ")
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefNullUUID(p *uuid.NullUUID) uuid.NullUUID {
	if p == nil {
		return uuid.NullUUID{}
	}
	return *p
}

// jsonOrDefault keeps a JSON column non-NULL and json_valid: an absent or
// blank value becomes the column's documented empty shape rather than NULL,
// which the CHECK constraint would reject anyway.
func jsonOrDefault(b []byte, def string) string {
	if len(b) == 0 {
		return def
	}
	return string(b)
}

// wrapDup is wrap plus the exported-boundary duplicate translation every
// create/update path here needs: a UNIQUE collision leaves this package as
// domain.ErrDuplicate, never a driver error, so internal/httpapi can answer 409
// with errors.Is and no knowledge of which engine is underneath (see
// store.CreateUser and internal/domain's package doc).
func wrapDup(op string, err error) error {
	if err == nil {
		return nil
	}
	if dbx.IsUniqueViolation(err) {
		return domain.ErrDuplicate
	}
	return wrap(op, err)
}

// jsonObject renders a timeline event's Detail. A marshal failure degrades to
// the empty object rather than failing the write: Detail is display sugar, and
// losing the whole status change because one label would not encode is worse
// than losing the label.
func jsonObject(m map[string]any) []byte {
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// ErrMergeSameCustomer is a store-level constraint (not a lookup miss), so it
// needs its own sentinel to reach the handler as a 400 rather than a 404.
var ErrMergeSameCustomer = errors.New("store: cannot merge a customer into itself")
