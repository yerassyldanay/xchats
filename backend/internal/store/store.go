// Package store is the PostgreSQL data layer (pgx v5, plain SQL, no ORM).
// All tables live in the xchats schema; the pool sets search_path=xchats,public.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// Store wraps the pgx pool.
type Store struct {
	pool *pgxpool.Pool
	// creds protects provider credentials at rest (see UseCredentialsBox). nil
	// until the composition root installs one; every credential path then fails
	// with ErrNoCredentialsKey rather than storing plaintext.
	creds *secretbox.Box
}

// New opens a pool against dsn and pins search_path to the xchats schema.
func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "xchats,public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Pool exposes the underlying pool (migrations, health checks).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Ping verifies the DB is reachable (readyz).
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// ---------------------------------------------------------------------------
// Domain types (API-facing shapes are assembled in the httpapi layer)
// ---------------------------------------------------------------------------

type Organization struct {
	ID          uuid.UUID
	Name        string
	RespondMode string
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
}

// Account is the channel-neutral account shape the unified read layer returns
// (inbox_accounts_v). Provider vocabulary lives in the two External* fields
// rather than in column names, so a WhatsApp number and a Telegram bot are the
// same type to everything above the store.
type Account struct {
	ID             uuid.UUID
	OrganizationID uuid.NullUUID
	DisplayName    string
	Channel        string // "whatsapp" | "simulator" | "telegram"
	// ExternalAccountRef is the provider's own identity for this account: the
	// owner JID for WhatsApp/simulator, "telegram:bot:<bot_id>" for Telegram.
	ExternalAccountRef string
	// ExternalHandle is what an operator recognizes the account by: the phone
	// number for WhatsApp, "@botusername" for Telegram.
	ExternalHandle string
	// InstanceName is the Evolution instance (wa_* gateway only; "" elsewhere).
	InstanceName string
	// InstanceID is Evolution's own instance id. Write-side only: it is not
	// carried by inbox_accounts_v, so view-backed reads leave it empty.
	InstanceID      string
	ConnectionState string
	LastLiveEventAt *time.Time
	CreatedAt       time.Time
	DeletedAt       *time.Time

	// Telegram webhook health. Empty/nil for every other channel — the view
	// projects NULLs on the WhatsApp leg.
	WebhookURL           string
	WebhookRegisteredAt  *time.Time
	WebhookLastCheckedAt *time.Time
	WebhookLastError     string
}

type Contact struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	PhoneNumber string
	// ExternalContactRef is the provider's identity for the person: the phone
	// JID for WhatsApp/simulator, the numeric user id for Telegram.
	ExternalContactRef string
	LidJID             string
	PushName           string
	DisplayName        string
	Attributes         []byte
}

type Chat struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	ContactID uuid.UUID
	Channel   string // the owning account's channel — set by every view-backed read
	// ExternalConversationRef is the provider's identity for the conversation
	// (and the outbound destination): the remote JID for WhatsApp/simulator,
	// the numeric chat id for Telegram.
	ExternalConversationRef string
	ChatState               string
	AssigneeUserID          uuid.NullUUID
	LastMessageAt           *time.Time
	LastMessagePreview      string
	UnreadCount             int
	Contact                 Contact
}

type Message struct {
	ID           uuid.UUID
	ChatID       uuid.UUID
	Channel      string
	Direction    string
	SenderKind   string
	SenderUserID uuid.NullUUID
	// ExternalMessageID is the provider's id for this message: Evolution's
	// key.id for WhatsApp, the numeric message_id for Telegram.
	ExternalMessageID string
	MessageKind       string
	Body              string
	DeliveryState     string
	Source            string
	MessageTS         *time.Time
	Media             []MediaRef
}

type MediaRef struct {
	ID        uuid.UUID
	MediaType string
	Mimetype  string
	FileName  string
	FileSize  int
}

type Draft struct {
	ID               uuid.UUID
	ChatID           uuid.UUID
	Channel          string
	TriggerMessageID uuid.NullUUID
	OptionOrdinal    int
	DraftText        string
	ReplyLanguage    string
	ContextState     string
	Confidence       *float64
	Escalate         bool
	EscalationReason string
	DraftState       string
	CreatedAt        time.Time
}

// orgOwnsAccountExpr ranks an organization by whether it actually owns a
// messaging account — where the chats live. It must cover every channel's
// account table, not just wa_accounts: a Telegram-only organization is just as
// real, and ignoring tg_accounts would let a stray duplicate org shadow it.
const orgOwnsAccountExpr = `(EXISTS (SELECT 1 FROM xchats.wa_accounts a WHERE a.organization_id = o.id)
	 OR EXISTS (SELECT 1 FROM xchats.tg_accounts t WHERE t.organization_id = o.id))`

// ---------------------------------------------------------------------------
// Seeding (idempotent, on boot)
// ---------------------------------------------------------------------------

// SeedOrganization upserts the single default organization by name and returns it.
func (s *Store) SeedOrganization(ctx context.Context, name string) (Organization, error) {
	var o Organization
	// Idempotent without a UNIQUE(name) constraint: reuse the existing org for this
	// name (preferring the one that owns a messaging account on ANY channel, else
	// the oldest) and only insert when none exists. The old code used `ON CONFLICT
	// DO NOTHING` with no conflict target, so every boot inserted a fresh "xchats"
	// — this stops that at the source without deleting the historical duplicates.
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.respond_mode
		FROM xchats.organizations o
		WHERE o.name = $1
		ORDER BY `+orgOwnsAccountExpr+` DESC,
		         o.created_at ASC
		LIMIT 1`, name).Scan(&o.ID, &o.Name, &o.RespondMode)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			INSERT INTO xchats.organizations (name) VALUES ($1)
			RETURNING id, name, respond_mode`, name).Scan(&o.ID, &o.Name, &o.RespondMode)
	}
	return o, err
}

// SeedUser upserts a user by (case-insensitive) email and joins them to the org.
func (s *Store) SeedUser(ctx context.Context, orgID uuid.UUID, email, passwordHash, displayName string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xchats.users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = now()
		RETURNING id, email, password_hash, display_name, created_at`,
		email, passwordHash, displayName).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO xchats.organization_users (organization_id, user_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, orgID, u.ID)
	return u, err
}

// SeedAccount upserts the pre-connected xpayment account by its derived id. Kept
// for the Build 0 seed; the manager (B1) connects further accounts via QR.
func (s *Store) SeedAccount(ctx context.Context, a Account) (Account, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xchats.wa_accounts
			(id, organization_id, display_name, owner_jid, phone_number, evolution_instance_name, connection_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			display_name = EXCLUDED.display_name,
			evolution_instance_name = EXCLUDED.evolution_instance_name,
			connection_state = EXCLUDED.connection_state,
			deleted_at = NULL,
			updated_at = now()
		RETURNING `+waAccountCols,
		a.ID, a.OrganizationID, a.DisplayName, a.ExternalAccountRef, a.ExternalHandle, a.InstanceName, a.ConnectionState).
		Scan(scanWaAccountDst(&a)...)
	return a, err
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

// waAccountCols is the wa_accounts table projection, used by the WhatsApp
// write paths that RETURN the row they just wrote. Reads go through
// accountViewCols below instead, so they see every channel.
const waAccountCols = `id, organization_id, display_name, owner_jid, phone_number,
	evolution_instance_name, evolution_instance_id, connection_state, channel,
	last_live_event_at, created_at, deleted_at`

func scanWaAccountDst(a *Account) []any {
	return []any{
		&a.ID, &a.OrganizationID, &a.DisplayName, &a.ExternalAccountRef, &a.ExternalHandle,
		&a.InstanceName, &a.InstanceID, &a.ConnectionState, &a.Channel,
		&a.LastLiveEventAt, &a.CreatedAt, &a.DeletedAt,
	}
}

// accountViewCols is the canonical inbox_accounts_v projection — every channel's
// accounts under one neutral shape; scanAccountDst pairs with it.
const accountViewCols = `id, organization_id, display_name, channel,
	external_account_ref, external_handle, instance_name, connection_state,
	last_live_event_at, created_at, deleted_at,
	webhook_url, webhook_registered_at, webhook_last_checked_at, webhook_last_error`

// scanAccountView reads one inbox_accounts_v row. webhook_url and
// webhook_last_error are NULL on the WhatsApp leg, so they land in pointers
// first and fold to "" — the struct keeps plain strings for its callers.
func scanAccountView(row pgx.Row) (Account, error) {
	var a Account
	var url, lastErr *string
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.DisplayName, &a.Channel,
		&a.ExternalAccountRef, &a.ExternalHandle, &a.InstanceName, &a.ConnectionState,
		&a.LastLiveEventAt, &a.CreatedAt, &a.DeletedAt,
		&url, &a.WebhookRegisteredAt, &a.WebhookLastCheckedAt, &lastErr)
	if err != nil {
		return a, err
	}
	a.WebhookURL, a.WebhookLastError = deref(url), deref(lastErr)
	return a, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// AccountByID returns a live (non-deleted) account of ANY channel by its
// derived id, read through inbox_accounts_v.
func (s *Store) AccountByID(ctx context.Context, id uuid.UUID) (Account, error) {
	a, err := scanAccountView(s.pool.QueryRow(ctx, `SELECT `+accountViewCols+`
		FROM xchats.inbox_accounts_v WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ListAccountsForOrg returns the org's live (non-deleted) accounts across every
// channel, oldest first — the neutral GET /accounts listing.
func (s *Store) ListAccountsForOrg(ctx context.Context, orgID uuid.UUID) ([]Account, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accountViewCols+`
		FROM xchats.inbox_accounts_v
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, err := scanAccountView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListWaAccountsForOrg returns the org's live accounts that live on the wa_*
// gateway — WhatsApp numbers plus the simulator account. It is what the
// WhatsApp-only surfaces (the /whatsapp-accounts manager, the compose "from
// number" picker) read: a Telegram bot has no QR lifecycle and cannot start a
// conversation, so it must never appear there.
func (s *Store) ListWaAccountsForOrg(ctx context.Context, orgID uuid.UUID) ([]Account, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+waAccountCols+`
		FROM xchats.wa_accounts
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(scanWaAccountDst(&a)...); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpsertConnectedAccount writes (or revives) the row for a freshly connected
// number. Identity is uuidv5(owner_jid), so a re-added number lands on the SAME
// row — its chats/messages stay attached and deleted_at is cleared. A blank
// display_name keeps the existing one (so reconnect doesn't clobber the label).
func (s *Store) UpsertConnectedAccount(ctx context.Context, a Account) (Account, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xchats.wa_accounts
			(id, organization_id, display_name, owner_jid, phone_number,
			 evolution_instance_name, evolution_instance_id, connection_state, last_live_event_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name ELSE xchats.wa_accounts.display_name END,
			phone_number = EXCLUDED.phone_number,
			evolution_instance_name = EXCLUDED.evolution_instance_name,
			evolution_instance_id = EXCLUDED.evolution_instance_id,
			connection_state = EXCLUDED.connection_state,
			last_live_event_at = now(),
			deleted_at = NULL,
			updated_at = now()
		RETURNING `+waAccountCols,
		a.ID, a.OrganizationID, a.DisplayName, a.ExternalAccountRef, a.ExternalHandle,
		a.InstanceName, a.InstanceID, a.ConnectionState).
		Scan(scanWaAccountDst(&a)...)
	return a, err
}

// SetAccountState updates a live account's connection_state (and stamps activity).
func (s *Store) SetAccountState(ctx context.Context, id uuid.UUID, state string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.wa_accounts SET connection_state = $2, last_live_event_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, id, state)
	return err
}

// SetAccountStateByInstance maps a connection.update (which carries the instance
// name, not the id) to its account and updates the state. Returns the account id,
// or ErrNotFound for an unknown/pre-connect/deleted instance.
func (s *Store) SetAccountStateByInstance(ctx context.Context, instanceName, state string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		UPDATE xchats.wa_accounts SET connection_state = $2, last_live_event_at = now(), updated_at = now()
		WHERE evolution_instance_name = $1 AND deleted_at IS NULL
		RETURNING id`, instanceName, state).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, ErrNotFound
	}
	return id, err
}

// SoftDeleteAccount hides an account (a "clean"): its chats drop out of the inbox
// but the rows stay, so re-adding the number revives everything.
func (s *Store) SoftDeleteAccount(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.wa_accounts SET deleted_at = now(), connection_state = 'disconnected', updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// ManagedInstanceNames returns the instance names of all live (non-deleted)
// accounts — the maintenance view flags everything else as a stray instance.
func (s *Store) ManagedInstanceNames(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT evolution_instance_name FROM xchats.wa_accounts
		WHERE deleted_at IS NULL AND evolution_instance_name <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Auth & users
// ---------------------------------------------------------------------------

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, display_name, created_at
		FROM xchats.users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateUser(ctx context.Context, orgID uuid.UUID, email, passwordHash, displayName string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xchats.users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, display_name, created_at`,
		email, passwordHash, displayName).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO xchats.organization_users (organization_id, user_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, orgID, u.ID)
	return u, err
}

func (s *Store) ListUsersForOrg(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]User, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.created_at
		FROM xchats.users u
		JOIN xchats.organization_users ou ON ou.user_id = u.id
		WHERE ou.organization_id = $1
		ORDER BY u.created_at LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	var total int
	_ = s.pool.QueryRow(ctx, `
		SELECT count(*) FROM xchats.organization_users WHERE organization_id = $1`, orgID).Scan(&total)
	return out, total, rows.Err()
}

func (s *Store) OrgForUser(ctx context.Context, userID uuid.UUID) (Organization, error) {
	var o Organization
	// Deterministic pick: prefer an org that actually owns a messaging account on
	// any channel (where the chats live), then the oldest. Guards against a stray
	// duplicate org silently shadowing the real one (see migration 0003).
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.respond_mode
		FROM xchats.organizations o
		JOIN xchats.organization_users ou ON ou.organization_id = o.id
		WHERE ou.user_id = $1
		ORDER BY `+orgOwnsAccountExpr+` DESC,
		         o.created_at ASC
		LIMIT 1`, userID).Scan(&o.ID, &o.Name, &o.RespondMode)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	return o, err
}

// OrgsForUser returns every organization a user belongs to
// (organization_users), oldest first. OrgForUser resolves a single
// deterministic default for the rest of the app; this is for the few
// surfaces that need the full membership set — the MCP OAuth consent
// page's organization picker (plan/mcp.md §3: "the user selects an
// organization").
func (s *Store) OrgsForUser(ctx context.Context, userID uuid.UUID) ([]Organization, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.name, o.respond_mode
		FROM xchats.organizations o
		JOIN xchats.organization_users ou ON ou.organization_id = o.id
		WHERE ou.user_id = $1
		ORDER BY o.created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.RespondMode); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// OrgByID resolves one organization by id, with no membership check of its
// own — callers that reach this from user input (the active-organization
// endpoint, the review-handoff redirect) must check UserInOrg themselves
// first.
func (s *Store) OrgByID(ctx context.Context, orgID uuid.UUID) (Organization, error) {
	var o Organization
	err := s.pool.QueryRow(ctx, `SELECT id, name, respond_mode FROM xchats.organizations WHERE id = $1`, orgID).
		Scan(&o.ID, &o.Name, &o.RespondMode)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	return o, err
}

// UserInOrg reports whether userID is a live member of orgID — the MCP
// access-token verification's per-request tenant re-check (plan/mcp.md §3:
// "The backend must still verify that the user is active and belongs to
// the bound organization on every request"), re-derived from the live
// organization_users/users tables rather than trusted from the token's own
// claims alone. There is no separate "active" flag on xchats.users today
// (no soft-delete concept for users yet); membership plus the row existing
// is the whole of "active" this schema can currently express.
func (s *Store) UserInOrg(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM xchats.organization_users ou
			JOIN xchats.users u ON u.id = ou.user_id
			WHERE ou.user_id = $1 AND ou.organization_id = $2
		)`, userID, orgID).Scan(&exists)
	return exists, err
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func (s *Store) CreateSession(ctx context.Context, id string, userID uuid.UUID, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xchats.sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		id, userID, time.Now().Add(ttl).UTC())
	return err
}

// UserForSession returns the user for a non-expired session id.
func (s *Store) UserForSession(ctx context.Context, sessionID string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.display_name, u.created_at
		FROM xchats.sessions s JOIN xchats.users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > now()`, sessionID).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM xchats.sessions WHERE id = $1`, sessionID)
	return err
}

// ActiveOrganizationForSession returns the session's explicitly-selected
// organization (set by SetActiveOrganization, or by a verified MCP review
// handoff — plan Task 15), if any. ok is false when the session carries no
// active organization at all (the common case for a single-org user, or one
// who has never switched/landed via a handoff) — the caller falls back to
// OrgForUser's deterministic default.
func (s *Store) ActiveOrganizationForSession(ctx context.Context, sessionID string) (orgID uuid.UUID, ok bool, err error) {
	var id *uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT active_organization_id FROM xchats.sessions
		WHERE id = $1 AND expires_at > now()`, sessionID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, false, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, false, err
	}
	if id == nil {
		return uuid.UUID{}, false, nil
	}
	return *id, true, nil
}

// SetActiveOrganization records which organization THIS session is scoped
// to — orgOf (internal/httpapi) re-validates current membership on every
// read, so this alone never grants access to an org the caller was removed
// from after the fact.
func (s *Store) SetActiveOrganization(ctx context.Context, sessionID string, orgID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.sessions SET active_organization_id = $2 WHERE id = $1`, sessionID, orgID)
	return err
}

// inserted reports whether a row returned by an upsert was freshly inserted
// (xmax = 0) rather than updated.
func scanInserted(row pgx.Row, id *uuid.UUID, inserted *bool) error {
	return row.Scan(id, inserted)
}

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
