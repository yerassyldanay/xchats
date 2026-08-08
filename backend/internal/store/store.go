// Package store is xchats' core data layer (SQLite via internal/dbx, plain
// SQL, no ORM): organizations, users, sessions, and the WhatsApp/Telegram
// transport tables' shared read/write surface. New opens (or attaches to
// an already-open, shared-by-path) database and migrates it — there is no
// separate migration step or DB handle for callers to manage.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/domain"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// ErrNotFound is returned when a lookup matches no row. It is the exact
// same sentinel as domain.ErrNotFound (not a new error) — kept as a
// store-local name so existing errors.Is(err, store.ErrNotFound) call
// sites keep working unchanged; new code should prefer domain.ErrNotFound
// directly.
var ErrNotFound = domain.ErrNotFound

// Store wraps the SQLite pool.
type Store struct {
	db *dbx.DB
	// creds protects provider credentials at rest (see UseCredentialsBox). nil
	// until the composition root installs one; every credential path then fails
	// with ErrNoCredentialsKey rather than storing plaintext.
	creds *secretbox.Box
}

// New opens dbPath (creating it, and its parent directory, if needed),
// applies every pending migration, and returns a ready Store. Safe to call
// more than once for the same path from within this process: every caller
// shares the one underlying connection (see internal/dbx.Open) — this is
// how kbstore.New, mcpauth.NewStore, and responsestore's DB-backed repos
// end up on the same pool as this Store without either side importing dbx
// types into an exported signature outside the persistence boundary.
func New(ctx context.Context, dbPath string) (*Store, error) {
	db, err := dbx.Open(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := dbx.RunMigrations(ctx, db, sqlitemigrations.FS); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Ping verifies the DB is reachable (readyz).
func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }

// Close releases this Store's reference to the pool.
func (s *Store) Close() { _ = s.db.Close() }

// ---------------------------------------------------------------------------
// Domain types (API-facing shapes are assembled in the httpapi layer)
// ---------------------------------------------------------------------------

type Organization struct {
	ID          uuid.UUID
	Name        string
	RespondMode string
	// Timezone is an IANA zone name (e.g. "Asia/Almaty") — purely a display
	// default for the frontend's campaign quiet-hours window picker
	// (frontend/src/lib/schedule.ts already does the local<->UTC conversion
	// for automation's own windows; campaigns reuse it). Every campaign
	// window is stored and evaluated in UTC regardless — see
	// backend/campaign.Window's own doc comment — so nothing server-side
	// ever reads this field for a computation.
	Timezone string
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
	// Role is the user's role ("admin" or "member") within a specific
	// organization — only populated by org-scoped queries (CreateUser,
	// ListUsersForOrg); left "" by lookups with no org in scope
	// (UserByEmail, UserForSession), which callers resolve separately via
	// MembershipRole once they have an organization id.
	Role string
	// MustChangePassword gates every route but /me, /auth/logout, and
	// /auth/password (internal/httpapi's requirePasswordChanged) until the
	// user sets a real password — true for the migration-seeded sentinel
	// admin from first boot until its first password change (see
	// SetUserPassword, BootstrapSentinelAdminPassword).
	MustChangePassword bool
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
	ExternalHandle  string
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
	// LastInboundAt is when the customer last messaged in, distinct from
	// LastMessageAt: only an inbound reopens a Meta channel's 24-hour
	// customer-service window, so an outbound reply must not advance it. Nil
	// on every wa_*/tg_* chat, which has no such window.
	LastInboundAt      *time.Time
	LastMessagePreview string
	UnreadCount        int
	Contact            Contact
	// CustomerID is the CRM customer this conversation belongs to, resolved
	// through the contact's channel identity (see read.go's chatCustomerJoin).
	// Invalid for a chat on an unassigned account, and for chats that predate
	// migration 0013.
	CustomerID uuid.NullUUID
}

type Message struct {
	ID           uuid.UUID
	ChatID       uuid.UUID
	Channel      string
	Direction    string
	SenderKind   string
	SenderUserID uuid.NullUUID
	// ExternalMessageID is the provider's id for this message: whatsmeow's
	// message id for WhatsApp, the numeric message_id for Telegram.
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
// account table, not just wa_accounts: a Telegram- or Meta-channel-only
// organization is just as real, and ignoring tg_accounts/channel_accounts
// would let a stray duplicate org shadow it.
const orgOwnsAccountExpr = `(EXISTS (SELECT 1 FROM wa_accounts a WHERE a.organization_id = o.id)
	 OR EXISTS (SELECT 1 FROM tg_accounts t WHERE t.organization_id = o.id)
	 OR EXISTS (SELECT 1 FROM channel_accounts ca WHERE ca.organization_id = o.id))`

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
	err := s.db.QueryRow(ctx, `
		SELECT o.id, o.name, o.respond_mode, o.timezone
		FROM organizations o
		WHERE o.name = $1
		ORDER BY `+orgOwnsAccountExpr+` DESC,
		         o.created_at ASC
		LIMIT 1`, name).Scan(&o.ID, &o.Name, &o.RespondMode, &o.Timezone)
	if errors.Is(err, dbx.ErrNoRows) {
		err = s.db.QueryRow(ctx, `
			INSERT INTO organizations (name) VALUES ($1)
			RETURNING id, name, respond_mode, timezone`, name).Scan(&o.ID, &o.Name, &o.RespondMode, &o.Timezone)
	}
	return o, err
}

// SeedUser upserts a user by (case-insensitive) email and joins them to the
// org as its admin. Every existing caller (test harnesses across the module,
// via internal/dbtest and its own package-local Stores) uses this to create
// the one operator for a freshly seeded org, mirroring migration 0006's own
// sentinel-admin seed — never a second, lesser-privileged user in the same
// org — so hardcoding "admin" here needs no role parameter threaded through
// every call site. Tests that specifically need a "member" for an RBAC
// boundary check create one with CreateUser or SetMembershipRole instead.
func (s *Store) SeedUser(ctx context.Context, orgID uuid.UUID, email, passwordHash, displayName string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING id, email, password_hash, display_name, created_at`,
		email, passwordHash, displayName).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO organization_users (organization_id, user_id, role)
		VALUES ($1, $2, 'admin')
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = 'admin'`, orgID, u.ID)
	u.Role = "admin"
	return u, err
}

// SeedAccount upserts the pre-connected xpayment account by its derived id. Kept
// for the Build 0 seed; the manager (B1) connects further accounts via QR.
func (s *Store) SeedAccount(ctx context.Context, a Account) (Account, error) {
	err := s.db.QueryRow(ctx, `
		INSERT INTO wa_accounts
			(id, organization_id, display_name, owner_jid, phone_number, connection_state)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			display_name = EXCLUDED.display_name,
			connection_state = EXCLUDED.connection_state,
			deleted_at = NULL,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING `+waAccountCols,
		a.ID, a.OrganizationID, a.DisplayName, a.ExternalAccountRef, a.ExternalHandle, a.ConnectionState).
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
	connection_state, channel,
	last_live_event_at, created_at, deleted_at`

func scanWaAccountDst(a *Account) []any {
	return []any{
		&a.ID, &a.OrganizationID, &a.DisplayName, &a.ExternalAccountRef, &a.ExternalHandle,
		&a.ConnectionState, &a.Channel,
		&a.LastLiveEventAt, &a.CreatedAt, &a.DeletedAt,
	}
}

// accountViewCols is the canonical inbox_accounts_v projection — every channel's
// accounts under one neutral shape; scanAccountDst pairs with it.
const accountViewCols = `id, organization_id, display_name, channel,
	external_account_ref, external_handle, connection_state,
	last_live_event_at, created_at, deleted_at,
	webhook_url, webhook_registered_at, webhook_last_checked_at, webhook_last_error`

// scanAccountView reads one inbox_accounts_v row. webhook_url and
// webhook_last_error are NULL on the WhatsApp leg, so they land in pointers
// first and fold to "" — the struct keeps plain strings for its callers.
func scanAccountView(row dbx.Scanner) (Account, error) {
	var a Account
	var url, lastErr *string
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.DisplayName, &a.Channel,
		&a.ExternalAccountRef, &a.ExternalHandle, &a.ConnectionState,
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
	a, err := scanAccountView(s.db.QueryRow(ctx, `SELECT `+accountViewCols+`
		FROM inbox_accounts_v WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, dbx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ListAccountsForOrg returns the org's live (non-deleted) accounts across every
// channel, oldest first — the neutral GET /accounts listing.
func (s *Store) ListAccountsForOrg(ctx context.Context, orgID uuid.UUID) ([]Account, error) {
	rows, err := s.db.Query(ctx, `SELECT `+accountViewCols+`
		FROM inbox_accounts_v
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
	rows, err := s.db.Query(ctx, `SELECT `+waAccountCols+`
		FROM wa_accounts
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
	err := s.db.QueryRow(ctx, `
		INSERT INTO wa_accounts
			(id, organization_id, display_name, owner_jid, phone_number,
			 connection_state, last_live_event_at)
		VALUES ($1, $2, $3, $4, $5, $6, strftime('%Y-%m-%d %H:%M:%f','now'))
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name ELSE wa_accounts.display_name END,
			phone_number = EXCLUDED.phone_number,
			connection_state = EXCLUDED.connection_state,
			last_live_event_at = strftime('%Y-%m-%d %H:%M:%f','now'),
			deleted_at = NULL,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING `+waAccountCols,
		a.ID, a.OrganizationID, a.DisplayName, a.ExternalAccountRef, a.ExternalHandle,
		a.ConnectionState).
		Scan(scanWaAccountDst(&a)...)
	return a, err
}

// SetAccountState updates a live account's connection_state (and stamps activity).
func (s *Store) SetAccountState(ctx context.Context, id uuid.UUID, state string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE wa_accounts SET connection_state = $2, last_live_event_at = strftime('%Y-%m-%d %H:%M:%f','now'), updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND deleted_at IS NULL`, id, state)
	return err
}

// SoftDeleteAccount hides an account (a "clean"): its chats drop out of the inbox
// but the rows stay, so re-adding the number revives everything.
func (s *Store) SoftDeleteAccount(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE wa_accounts SET deleted_at = strftime('%Y-%m-%d %H:%M:%f','now'), connection_state = 'disconnected', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// ---------------------------------------------------------------------------
// WhatsApp (whatsmeow) device credentials
// ---------------------------------------------------------------------------
// wa_credentials maps an account to whatsmeow's own device JID, so the manager
// can find which saved accounts to reconnect after a restart. The actual
// session (keys, identity) lives in whatsmeow's own device database, never here.

// WaCredential is one account's whatsmeow device mapping.
type WaCredential struct {
	AccountID uuid.UUID
	DeviceJID string
}

// SaveWaCredentials records (or replaces) the whatsmeow device JID for an
// account — called once pairing succeeds.
func (s *Store) SaveWaCredentials(ctx context.Context, accountID uuid.UUID, deviceJID string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO wa_credentials (account_id, device_jid)
		VALUES ($1, $2)
		ON CONFLICT (account_id) DO UPDATE SET
			device_jid = EXCLUDED.device_jid,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`, accountID, deviceJID)
	return err
}

// DeleteWaCredentials drops an account's device mapping — called on logout, so
// a subsequent boot does not try to reconnect a session that no longer exists.
func (s *Store) DeleteWaCredentials(ctx context.Context, accountID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM wa_credentials WHERE account_id = $1`, accountID)
	return err
}

// ListWaCredentials returns every saved account->device mapping, for the
// manager's boot-time reconnect-all pass.
func (s *Store) ListWaCredentials(ctx context.Context) ([]WaCredential, error) {
	rows, err := s.db.Query(ctx, `SELECT account_id, device_jid FROM wa_credentials`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WaCredential
	for rows.Next() {
		var c WaCredential
		if err := rows.Scan(&c.AccountID, &c.DeviceJID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Auth & users
// ---------------------------------------------------------------------------

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, display_name, created_at, must_change_password
		FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt, &u.MustChangePassword)
	if errors.Is(err, dbx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// SetUserPassword sets id's password hash and clears must_change_password —
// the user (or an admin acting for them, once that surface exists) has just
// set a real password, so the forced-change gate no longer applies. This is
// the general path; BootstrapSentinelAdminPassword below is the distinct
// first-boot path that deliberately leaves must_change_password set.
func (s *Store) SetUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE users SET password_hash = $2, must_change_password = 0,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, id, passwordHash)
	return err
}

// DeleteOtherSessions removes every session for userID except keepSessionID
// — called right after a successful password change so a stolen session
// elsewhere is evicted immediately, not just on its own natural expiry.
func (s *Store) DeleteOtherSessions(ctx context.Context, userID uuid.UUID, keepSessionID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id != $2`, userID, keepSessionID)
	return err
}

// sentinelAdminID is migration 0006_init_admin.up.sql's fixed seeded-admin
// user id — the one row BootstrapSentinelAdminPassword and
// ResetSentinelAdminPassword ever touch.
var sentinelAdminID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

// IsSentinelAdmin reports whether id belongs to the migration-created
// bootstrap administrator. The HTTP password-change path uses this narrow
// identity check to remove that account's one-time credential file without
// coupling every ordinary user password change to bootstrap storage.
func IsSentinelAdmin(id uuid.UUID) bool { return id == sentinelAdminID }

// BootstrapSentinelAdminPassword mints the sentinel admin's first real
// password on boot (cmd/xchats' first-boot bootstrap). It writes
// passwordHash ONLY if the row still carries 0008_bootstrap_admin's blanked
// "" sentinel — the same WHERE-guarded idempotency pattern that migration
// uses — which a fresh install never has, since
// 0011_restore_default_admin_password already restores the static default
// password. It clears must_change_password so a mint here (the "xchats
// reset-admin-password" recovery path, or an operator-supplied
// XCHATS_BOOTSTRAP_ADMIN_PASSWORD) lands on the same no-forced-change state
// as a fresh install. minted reports whether this call actually wrote it
// (false on every boot after the first, or if an operator has since changed
// it some other way) — cmd/xchats uses that to decide whether to
// print/persist the bootstrap credential file at all.
func (s *Store) BootstrapSentinelAdminPassword(ctx context.Context, passwordHash string) (minted bool, err error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE users SET password_hash = $2, must_change_password = 0,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND password_hash = ''`, sentinelAdminID, passwordHash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ResetSentinelAdminPassword re-blanks the sentinel admin's password hash
// and re-sets must_change_password — the "xchats reset-admin-password"
// recovery path for a lost/never-read one-time credential. The next boot's
// BootstrapSentinelAdminPassword call re-mints a fresh one.
func (s *Store) ResetSentinelAdminPassword(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		UPDATE users SET password_hash = '', must_change_password = 1,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, sentinelAdminID)
	return err
}

// CreateUser inserts a new user and joins them to orgID with the given role
// ("admin" or "member"; "" defaults to "member" — the safe default for a
// freshly created team member, promoted later via SetMembershipRole). A
// duplicate email (users.email is UNIQUE COLLATE NOCASE, citext's SQLite
// equivalent) comes back as domain.ErrDuplicate — the exported-boundary
// translation that replaces the old pgx "23505" string match, which
// internal/httpapi/auth.go's isUniqueViolation now compares against with
// errors.Is.
func (s *Store) CreateUser(ctx context.Context, orgID uuid.UUID, email, passwordHash, displayName, role string) (User, error) {
	if role == "" {
		role = "member"
	}
	var u User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, display_name, created_at`,
		email, passwordHash, displayName).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if err != nil {
		if dbx.IsUniqueViolation(err) {
			return u, domain.ErrDuplicate
		}
		return u, err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO organization_users (organization_id, user_id, role)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, orgID, u.ID, role)
	u.Role = role
	return u, err
}

func (s *Store) ListUsersForOrg(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]User, int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.created_at, ou.role
		FROM users u
		JOIN organization_users ou ON ou.user_id = u.id
		WHERE ou.organization_id = $1
		ORDER BY u.created_at LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &u.Role); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	var total int
	_ = s.db.QueryRow(ctx, `
		SELECT count(*) FROM organization_users WHERE organization_id = $1`, orgID).Scan(&total)
	return out, total, rows.Err()
}

// MembershipRole returns userID's role ("admin" or "member") within orgID.
// It is the live source RequireAdmin gates on — re-queried on every request,
// never cached on the session, so a demotion or removal takes effect
// immediately mid-session.
func (s *Store) MembershipRole(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	var role string
	err := s.db.QueryRow(ctx, `
		SELECT role FROM organization_users WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID).Scan(&role)
	if errors.Is(err, dbx.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

// SetMembershipRole changes userID's role within orgID. It refuses to demote
// (or otherwise change away) an organization's LAST remaining admin —
// domain.ErrLastAdmin — so the Team management UI's role toggle can never
// leave an organization with nobody able to manage it. The admin-count check
// and the update happen in the same transaction to stay correct under
// concurrent role changes.
func (s *Store) SetMembershipRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current string
	err = tx.QueryRow(ctx, `
		SELECT role FROM organization_users WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID).Scan(&current)
	if errors.Is(err, dbx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if current == role {
		return tx.Commit(ctx)
	}
	if current == "admin" {
		var adminCount int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM organization_users WHERE organization_id = $1 AND role = 'admin'`,
			orgID).Scan(&adminCount); err != nil {
			return err
		}
		if adminCount <= 1 {
			return domain.ErrLastAdmin
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE organization_users SET role = $3 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID, role); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RenameOrganization updates an organization's display name (the Team
// management UI's "org rename" action; auto_response_mode has no editor
// yet). See SetOrganizationTimezone for the one other user-editable field.
func (s *Store) RenameOrganization(ctx context.Context, orgID uuid.UUID, name string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE organizations SET name = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, orgID, name)
	return err
}

// SetOrganizationTimezone updates an organization's display-default IANA
// timezone — see Organization.Timezone's own doc comment for why this is
// purely a frontend default, never read for a backend computation. A
// separate call from RenameOrganization (rather than one combined update)
// since the API accepts it as an independently optional field.
func (s *Store) SetOrganizationTimezone(ctx context.Context, orgID uuid.UUID, timezone string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE organizations SET timezone = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, orgID, timezone)
	return err
}

func (s *Store) OrgForUser(ctx context.Context, userID uuid.UUID) (Organization, error) {
	var o Organization
	// Deterministic pick: prefer an org that actually owns a messaging account on
	// any channel (where the chats live), then the oldest. Guards against a stray
	// duplicate org silently shadowing the real one (see migration 0003).
	err := s.db.QueryRow(ctx, `
		SELECT o.id, o.name, o.respond_mode, o.timezone
		FROM organizations o
		JOIN organization_users ou ON ou.organization_id = o.id
		WHERE ou.user_id = $1
		ORDER BY `+orgOwnsAccountExpr+` DESC,
		         o.created_at ASC
		LIMIT 1`, userID).Scan(&o.ID, &o.Name, &o.RespondMode, &o.Timezone)
	if errors.Is(err, dbx.ErrNoRows) {
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
	rows, err := s.db.Query(ctx, `
		SELECT o.id, o.name, o.respond_mode, o.timezone
		FROM organizations o
		JOIN organization_users ou ON ou.organization_id = o.id
		WHERE ou.user_id = $1
		ORDER BY o.created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.RespondMode, &o.Timezone); err != nil {
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
	err := s.db.QueryRow(ctx, `SELECT id, name, respond_mode, timezone FROM organizations WHERE id = $1`, orgID).
		Scan(&o.ID, &o.Name, &o.RespondMode, &o.Timezone)
	if errors.Is(err, dbx.ErrNoRows) {
		return o, ErrNotFound
	}
	return o, err
}

// UserInOrg reports whether userID is a live member of orgID — the MCP
// access-token verification's per-request tenant re-check (plan/mcp.md §3:
// "The backend must still verify that the user is active and belongs to
// the bound organization on every request"), re-derived from the live
// organization_users/users tables rather than trusted from the token's own
// claims alone. There is no separate "active" flag on users today
// (no soft-delete concept for users yet); membership plus the row existing
// is the whole of "active" this schema can currently express.
func (s *Store) UserInOrg(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM organization_users ou
			JOIN users u ON u.id = ou.user_id
			WHERE ou.user_id = $1 AND ou.organization_id = $2
		)`, userID, orgID).Scan(&exists)
	return exists, err
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func (s *Store) CreateSession(ctx context.Context, id string, userID uuid.UUID, ttl time.Duration) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		id, userID, time.Now().Add(ttl).UTC())
	return err
}

// UserForSession returns the user for a non-expired session id.
func (s *Store) UserForSession(ctx context.Context, sessionID string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.display_name, u.created_at, u.must_change_password
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > strftime('%Y-%m-%d %H:%M:%f','now')`, sessionID).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &u.MustChangePassword)
	if errors.Is(err, dbx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
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
	err = s.db.QueryRow(ctx, `
		SELECT active_organization_id FROM sessions
		WHERE id = $1 AND expires_at > strftime('%Y-%m-%d %H:%M:%f','now')`, sessionID).Scan(&id)
	if errors.Is(err, dbx.ErrNoRows) {
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
	_, err := s.db.Exec(ctx, `
		UPDATE sessions SET active_organization_id = $2 WHERE id = $1`, sessionID, orgID)
	return err
}

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
