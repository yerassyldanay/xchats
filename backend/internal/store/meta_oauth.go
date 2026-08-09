// Package store (this file): the OAuth-redirect connect flow's own state —
// Instagram Direct and Facebook Messenger both connect via a top-level
// browser redirect to Meta's consent screen and back (WhatsApp Cloud does
// not; see whatsapp_cloud_accounts.go's manual WABA-id+token flow instead).
// meta_oauth_states is what lets the PUBLIC callback route (it cannot trust
// a session cookie — see internal/httpapi/meta_oauth.go's own doc comment)
// recover who initiated the connect and finish it.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// MetaOAuthState is one in-flight (or settled) OAuth connect attempt —
// mirrors one meta_oauth_states row.
type MetaOAuthState struct {
	State          string
	Channel        string
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	RedirectURI    string
	Status         string
	AccountID      uuid.NullUUID
	LastError      string
	ExpiresAt      time.Time
	SettledAt      *time.Time
}

// CreateMetaOAuthStateInput is CreateMetaOAuthState's request. State must
// already be a caller-generated, unguessable random token (32 bytes,
// base64url) — this layer never mints it, exactly like ClaimChannelAccount
// never derives its own id.
type CreateMetaOAuthStateInput struct {
	State          string
	Channel        string
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	RedirectURI    string
	TTL            time.Duration
}

// CreateMetaOAuthState records a fresh connect attempt, BEFORE the browser
// ever leaves this site — organization_id/user_id are captured here, while
// the caller is still an authenticated in-app request, precisely so the
// later public callback never has to trust a session cookie to know who is
// connecting.
func (s *Store) CreateMetaOAuthState(ctx context.Context, in CreateMetaOAuthStateInput) error {
	ttl := in.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO meta_oauth_states (state, channel, organization_id, user_id, redirect_uri, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		in.State, in.Channel, in.OrganizationID, in.UserID, in.RedirectURI, time.Now().Add(ttl).UTC())
	return err
}

// MetaOAuthStateByID resolves a still-pending, unexpired state — the
// callback handler's first and only identity check. A state that is
// unknown, already settled, or expired is uniformly ErrNotFound: a replayed
// or stale callback must not be able to distinguish "never existed" from
// "already used" from "expired".
func (s *Store) MetaOAuthStateByID(ctx context.Context, state string) (MetaOAuthState, error) {
	var st MetaOAuthState
	err := s.db.QueryRow(ctx, `
		SELECT state, channel, organization_id, user_id, redirect_uri, status, account_id, last_error, expires_at, settled_at
		FROM meta_oauth_states
		WHERE state = $1 AND status = 'pending' AND expires_at > strftime('%Y-%m-%d %H:%M:%f','now')`, state).
		Scan(&st.State, &st.Channel, &st.OrganizationID, &st.UserID, &st.RedirectURI, &st.Status,
			&st.AccountID, &st.LastError, &st.ExpiresAt, &st.SettledAt)
	if errors.Is(err, dbx.ErrNoRows) {
		return MetaOAuthState{}, ErrNotFound
	}
	return st, err
}

// SettleMetaOAuthState records the callback's outcome. This ALSO moves
// status off 'pending', so MetaOAuthStateByID's own WHERE clause rejects any
// later replay of the same state — the callback handler must call this on
// every exit path once a state row was found (success or failure alike), or
// a failed attempt would stay repeatable until its TTL expires.
func (s *Store) SettleMetaOAuthState(ctx context.Context, state, status string, accountID uuid.NullUUID, lastError string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE meta_oauth_states
		SET status = $2, account_id = $3, last_error = $4, settled_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE state = $1`, state, status, accountID, lastError)
	return err
}
