// Package chatstore is the Knowledge Base chat assistant's data layer: the
// chat_conversations / chat_messages tables from migration 0012 (SQLite via
// internal/dbx, plain SQL, no ORM).
//
// It owns persistence ONLY — the conversation list, the full transcript, and
// the last-N window a request sends to the model. Prompt assembly, KB
// retrieval, and the LLM call live above it in internal/chat, which is what
// keeps the chat feature's three responsibilities separable (see that
// package's doc comment).
//
// Every read and write is scoped by (organizationID, userID): a conversation
// belongs to the operator who started it, inside the organization it was
// started in. That scope is a parameter on every method rather than
// something a caller can forget — there is deliberately no "by id alone"
// lookup, so an id guessed or leaked from another org resolves to
// ErrNotFound rather than to somebody else's chat.
package chatstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/domain"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// ErrNotFound is returned when a conversation lookup matches no row within
// the caller's (organization, user) scope — the same sentinel as
// domain.ErrNotFound, re-exported here for symmetry with the other
// repository packages.
var ErrNotFound = domain.ErrNotFound

// Roles a chat_messages row may carry, matching the table's own CHECK.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Store wraps the SQLite pool with the chat tables' operations.
type Store struct {
	db *dbx.DB
}

// New opens (or attaches to an already shared-by-path) dbPath and returns a
// ready Store. Safe to call more than once for the same path within this
// process — internal/dbx.Open refcounts one connection per path, so this
// shares the connection internal/store and internal/kbstore already hold
// rather than racing a second one against the same file.
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

// Close releases this Store's reference to the pool.
func (s *Store) Close() { _ = s.db.Close() }

// Scope is the (organization, user) pair every operation is keyed by — see
// the package doc comment for why it is never optional.
type Scope struct {
	OrgID  uuid.UUID
	UserID uuid.UUID
}

// Conversation is one chat thread's header — the sidebar row.
type Conversation struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message is one persisted turn. Metadata is the raw JSON object from the
// column — structured KB components, token usage, and the provider/model
// the answer came from for an assistant turn; an empty object for a user
// turn. It is carried as json.RawMessage rather than a typed struct so this
// package stays free of the component vocabulary internal/chatkb owns.
type Message struct {
	ID uuid.UUID `json:"id"`
	// Seq is the turn's position within its conversation — the transcript's
	// canonical order (see migration 0012's own comment on the column).
	Seq       int64           `json:"seq"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

// emptyMetadata is what a row with no metadata stores and returns — the
// column is NOT NULL with a json_valid CHECK, so "nothing" is an empty JSON
// object, never NULL and never "".
var emptyMetadata = json.RawMessage(`{}`)

// CreateConversation starts a new thread and returns its header. An empty
// title is normal: the first message names the thread (see SetTitle), and
// the UI renders an untitled conversation as "New chat" until then.
func (s *Store) CreateConversation(ctx context.Context, scope Scope, title string) (Conversation, error) {
	var c Conversation
	err := s.db.QueryRow(ctx, `
		INSERT INTO chat_conversations (organization_id, user_id, title)
		VALUES ($1, $2, $3)
		RETURNING id, title, created_at, updated_at`,
		scope.OrgID, scope.UserID, title).
		Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return Conversation{}, fmt.Errorf("chatstore: create conversation: %w", err)
	}
	return c, nil
}

// ListConversations returns the scope's threads, most recently active
// first. limit <= 0 means no limit.
func (s *Store) ListConversations(ctx context.Context, scope Scope, limit int) ([]Conversation, error) {
	query := `
		SELECT id, title, created_at, updated_at
		FROM chat_conversations
		WHERE organization_id = $1 AND user_id = $2
		ORDER BY updated_at DESC`
	args := []any{scope.OrgID, scope.UserID}
	if limit > 0 {
		query += ` LIMIT $3`
		args = append(args, limit)
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chatstore: list conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []Conversation{}
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("chatstore: scan conversation: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatstore: list conversations: %w", err)
	}
	return out, nil
}

// Conversation returns one thread's header, or ErrNotFound when no such
// conversation exists WITHIN scope — an id belonging to another user or
// another organization is indistinguishable from one that never existed.
func (s *Store) Conversation(ctx context.Context, scope Scope, id uuid.UUID) (Conversation, error) {
	var c Conversation
	err := s.db.QueryRow(ctx, `
		SELECT id, title, created_at, updated_at
		FROM chat_conversations
		WHERE id = $1 AND organization_id = $2 AND user_id = $3`,
		id, scope.OrgID, scope.UserID).
		Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, dbx.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("chatstore: load conversation: %w", err)
	}
	return c, nil
}

// SetTitle renames a thread (the auto-title after the first message, or an
// explicit rename from the UI).
func (s *Store) SetTitle(ctx context.Context, scope Scope, id uuid.UUID, title string) (Conversation, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE chat_conversations
		SET title = $1, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $2 AND organization_id = $3 AND user_id = $4`,
		title, id, scope.OrgID, scope.UserID)
	if err != nil {
		return Conversation{}, fmt.Errorf("chatstore: rename conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Conversation{}, ErrNotFound
	}
	return s.Conversation(ctx, scope, id)
}

// DeleteConversation removes a thread and, by the chat_messages foreign
// key's ON DELETE CASCADE, its whole transcript.
func (s *Store) DeleteConversation(ctx context.Context, scope Scope, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM chat_conversations
		WHERE id = $1 AND organization_id = $2 AND user_id = $3`,
		id, scope.OrgID, scope.UserID)
	if err != nil {
		return fmt.Errorf("chatstore: delete conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AppendInput is one turn to persist. ID may be uuid.Nil to have one
// generated; the chat service instead pre-generates the assistant turn's id
// so it can name the message in the stream's opening event, before a single
// token has been produced.
type AppendInput struct {
	ID       uuid.UUID
	Role     string
	Content  string
	Metadata json.RawMessage
}

// AppendMessage persists one turn and bumps its conversation's updated_at
// (which is what orders the sidebar). Both writes run in one transaction:
// a message whose thread did not move to the top of the list would be
// invisible to the operator who just sent it.
//
// The conversation is re-checked against scope inside that transaction, so
// appending to another user's thread fails with ErrNotFound rather than
// writing a row.
func (s *Store) AppendMessage(ctx context.Context, scope Scope, conversationID uuid.UUID, in AppendInput) (Message, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Message{}, fmt.Errorf("chatstore: begin append: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE chat_conversations
		SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND organization_id = $2 AND user_id = $3`,
		conversationID, scope.OrgID, scope.UserID)
	if err != nil {
		return Message{}, fmt.Errorf("chatstore: touch conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Message{}, ErrNotFound
	}

	metadata := in.Metadata
	if len(metadata) == 0 {
		metadata = emptyMetadata
	}
	// The next position in this conversation, read inside the same
	// transaction as the insert — safe against a concurrent second writer
	// because dbx serializes every write transaction through one connection
	// (see internal/dbx's package doc), and guarded by the UNIQUE index
	// regardless.
	var seq int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM chat_messages WHERE conversation_id = $1`,
		conversationID).Scan(&seq); err != nil {
		return Message{}, fmt.Errorf("chatstore: next message seq: %w", err)
	}

	id := in.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	var m Message
	// metadata scans through a string: json.RawMessage is a []byte alias the
	// driver will not convert a TEXT column into on its own.
	var storedMetadata string
	err = tx.QueryRow(ctx, `
		INSERT INTO chat_messages (id, conversation_id, seq, role, content, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, seq, role, content, metadata, created_at`,
		id, conversationID, seq, in.Role, in.Content, string(metadata)).
		Scan(&m.ID, &m.Seq, &m.Role, &m.Content, &storedMetadata, &m.CreatedAt)
	if err != nil {
		return Message{}, fmt.Errorf("chatstore: append message: %w", err)
	}
	m.Metadata = json.RawMessage(storedMetadata)
	if len(m.Metadata) == 0 {
		m.Metadata = emptyMetadata
	}
	if err := tx.Commit(ctx); err != nil {
		return Message{}, fmt.Errorf("chatstore: commit append: %w", err)
	}
	return m, nil
}

// Messages returns a conversation's WHOLE transcript, oldest first — what
// the UI renders when a thread is opened. The model never sees this: it
// gets RecentMessages' bounded window instead.
func (s *Store) Messages(ctx context.Context, scope Scope, conversationID uuid.UUID) ([]Message, error) {
	if _, err := s.Conversation(ctx, scope, conversationID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, seq, role, content, metadata, created_at
		FROM chat_messages
		WHERE conversation_id = $1
		ORDER BY seq`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("chatstore: list messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanMessages(rows)
}

// RecentMessages returns the last n turns of a conversation, oldest first —
// the ONLY history the model is ever shown (spec §3: the complete history is
// persisted, a bounded window is sent). n <= 0 returns nothing rather than
// everything: a misconfigured window must degrade to "no history," never
// silently to "the entire transcript."
func (s *Store) RecentMessages(ctx context.Context, scope Scope, conversationID uuid.UUID, n int) ([]Message, error) {
	if _, err := s.Conversation(ctx, scope, conversationID); err != nil {
		return nil, err
	}
	if n <= 0 {
		return []Message{}, nil
	}
	// Take the newest n by descending order, then re-sort ascending in Go —
	// "the last n, oldest first" is not expressible as one ORDER BY.
	rows, err := s.db.Query(ctx, `
		SELECT id, seq, role, content, metadata, created_at
		FROM chat_messages
		WHERE conversation_id = $1
		ORDER BY seq DESC
		LIMIT $2`, conversationID, n)
	if err != nil {
		return nil, fmt.Errorf("chatstore: recent messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// CountMessages reports how many turns a conversation holds — what the chat
// service checks to decide whether an incoming message is the first one and
// should therefore name the thread.
func (s *Store) CountMessages(ctx context.Context, scope Scope, conversationID uuid.UUID) (int, error) {
	if _, err := s.Conversation(ctx, scope, conversationID); err != nil {
		return 0, err
	}
	var n int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM chat_messages WHERE conversation_id = $1`, conversationID).Scan(&n); err != nil {
		return 0, fmt.Errorf("chatstore: count messages: %w", err)
	}
	return n, nil
}

func scanMessages(rows *dbx.Rows) ([]Message, error) {
	out := []Message{}
	for rows.Next() {
		var m Message
		var metadata string
		if err := rows.Scan(&m.ID, &m.Seq, &m.Role, &m.Content, &metadata, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("chatstore: scan message: %w", err)
		}
		m.Metadata = json.RawMessage(metadata)
		if len(m.Metadata) == 0 {
			m.Metadata = emptyMetadata
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatstore: read messages: %w", err)
	}
	return out, nil
}
