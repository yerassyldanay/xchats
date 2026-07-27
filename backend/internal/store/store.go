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
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// Store wraps the pgx pool.
type Store struct {
	pool *pgxpool.Pool
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
	ID           uuid.UUID
	Name         string
	RespondMode  string
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
}

type Account struct {
	ID              uuid.UUID
	OrganizationID  uuid.NullUUID
	DisplayName     string
	OwnerJID        string
	PhoneNumber     string
	InstanceName    string
	InstanceID      string
	ConnectionState string
	LastLiveEventAt *time.Time
	CreatedAt       time.Time
	DeletedAt       *time.Time
}

type Contact struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	PhoneNumber string
	PhoneJID    string
	LidJID      string
	PushName    string
	DisplayName string
	Attributes  []byte
}

type Chat struct {
	ID                 uuid.UUID
	AccountID          uuid.UUID
	ContactID          uuid.UUID
	RemoteJID          string
	ChatState          string
	AssigneeUserID     uuid.NullUUID
	LastMessageAt      *time.Time
	LastMessagePreview string
	UnreadCount        int
	Contact            Contact
}

type Message struct {
	ID                 uuid.UUID
	ChatID             uuid.UUID
	Direction          string
	SenderKind         string
	SenderUserID       uuid.NullUUID
	EvolutionMessageID string
	MessageKind        string
	Body               string
	DeliveryState      string
	Source             string
	MessageTS          *time.Time
	Media              []MediaRef
}

type MediaRef struct {
	ID        uuid.UUID
	MediaType string
	Mimetype  string
	FileName  string
	FileSize  int
}

type Draft struct {
	ID                uuid.UUID
	ChatID            uuid.UUID
	TriggerMessageID  uuid.NullUUID
	OptionOrdinal     int
	DraftText         string
	ContextState      string
	Confidence        *float64
	Escalate          bool
	EscalationReason  string
	DraftState        string
	CreatedAt         time.Time
	Assets            []DraftAsset
}

type DraftAsset struct {
	ID        uuid.UUID
	AssetRef  string
	MediaKind string
	MediaURL  string
	Ordinal   int
}

// ---------------------------------------------------------------------------
// Seeding (idempotent, on boot)
// ---------------------------------------------------------------------------

// SeedOrganization upserts the single default organization by name and returns it.
func (s *Store) SeedOrganization(ctx context.Context, name string) (Organization, error) {
	var o Organization
	// Idempotent without a UNIQUE(name) constraint: reuse the existing org for this
	// name (preferring the one that owns a WhatsApp account, else the oldest) and
	// only insert when none exists. The old code used `ON CONFLICT DO NOTHING` with
	// no conflict target, so every boot inserted a fresh "xchats" — this stops that
	// at the source without deleting the historical duplicates.
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.respond_mode
		FROM xchats.organizations o
		WHERE o.name = $1
		ORDER BY (EXISTS (SELECT 1 FROM xchats.wa_accounts a WHERE a.organization_id = o.id)) DESC,
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
		RETURNING `+accountCols,
		a.ID, a.OrganizationID, a.DisplayName, a.OwnerJID, a.PhoneNumber, a.InstanceName, a.ConnectionState).
		Scan(scanAccountDst(&a)...)
	return a, err
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

// accountCols is the canonical wa_accounts projection; scanAccountDst pairs with it.
const accountCols = `id, organization_id, display_name, owner_jid, phone_number,
	evolution_instance_name, evolution_instance_id, connection_state,
	last_live_event_at, created_at, deleted_at`

func scanAccountDst(a *Account) []any {
	return []any{
		&a.ID, &a.OrganizationID, &a.DisplayName, &a.OwnerJID, &a.PhoneNumber,
		&a.InstanceName, &a.InstanceID, &a.ConnectionState,
		&a.LastLiveEventAt, &a.CreatedAt, &a.DeletedAt,
	}
}

// AccountByID returns a live (non-deleted) account by its derived id.
func (s *Store) AccountByID(ctx context.Context, id uuid.UUID) (Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `SELECT `+accountCols+`
		FROM xchats.wa_accounts WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(scanAccountDst(&a)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ListAccountsForOrg returns the org's live (non-deleted) accounts, oldest first.
func (s *Store) ListAccountsForOrg(ctx context.Context, orgID uuid.UUID) ([]Account, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accountCols+`
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
		if err := rows.Scan(scanAccountDst(&a)...); err != nil {
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
		RETURNING `+accountCols,
		a.ID, a.OrganizationID, a.DisplayName, a.OwnerJID, a.PhoneNumber,
		a.InstanceName, a.InstanceID, a.ConnectionState).
		Scan(scanAccountDst(&a)...)
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

func (s *Store) ListUsers(ctx context.Context, limit, offset int) ([]User, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, display_name, created_at FROM xchats.users
		ORDER BY created_at LIMIT $1 OFFSET $2`, limit, offset)
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
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM xchats.users`).Scan(&total)
	return out, total, rows.Err()
}

func (s *Store) OrgForUser(ctx context.Context, userID uuid.UUID) (Organization, error) {
	var o Organization
	// Deterministic pick: prefer an org that actually owns a WhatsApp account
	// (where the chats live), then the oldest. Guards against a stray duplicate
	// org silently shadowing the real one (see migration 0003).
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.respond_mode
		FROM xchats.organizations o
		JOIN xchats.organization_users ou ON ou.organization_id = o.id
		WHERE ou.user_id = $1
		ORDER BY (EXISTS (SELECT 1 FROM xchats.wa_accounts a WHERE a.organization_id = o.id)) DESC,
		         o.created_at ASC
		LIMIT 1`, userID).Scan(&o.ID, &o.Name, &o.RespondMode)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	return o, err
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
