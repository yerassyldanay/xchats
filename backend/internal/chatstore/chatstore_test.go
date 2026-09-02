package chatstore_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/chatstore"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// seedScope creates an organization with two users in it and returns a scope
// for each — the shape every isolation assertion below needs.
func seedScope(t *testing.T, st *store.Store) (chatstore.Scope, chatstore.Scope) {
	t.Helper()
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "acme")
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	alice, err := st.SeedUser(ctx, org.ID, "alice@acme.test", "hash", "Alice")
	if err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	bob, err := st.SeedUser(ctx, org.ID, "bob@acme.test", "hash", "Bob")
	if err != nil {
		t.Fatalf("seed bob: %v", err)
	}
	return chatstore.Scope{OrgID: org.ID, UserID: alice.ID}, chatstore.Scope{OrgID: org.ID, UserID: bob.ID}
}

func TestConversationLifecycle(t *testing.T) {
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	alice, _ := seedScope(t, st)

	conv, err := cs.CreateConversation(ctx, alice, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if conv.ID == uuid.Nil {
		t.Fatal("create returned the nil UUID")
	}
	if conv.Title != "" {
		t.Errorf("title = %q, want empty (the first message names the thread)", conv.Title)
	}

	renamed, err := cs.SetTitle(ctx, alice, conv.ID, "Vitamin D pricing")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Title != "Vitamin D pricing" {
		t.Errorf("title = %q, want %q", renamed.Title, "Vitamin D pricing")
	}

	list, err := cs.ListConversations(ctx, alice, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != conv.ID {
		t.Fatalf("list = %+v, want exactly the created conversation", list)
	}

	if err := cs.DeleteConversation(ctx, alice, conv.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := cs.Conversation(ctx, alice, conv.ID); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("after delete, load err = %v, want ErrNotFound", err)
	}
}

// A conversation is private to the operator who started it. Every read and
// write for somebody else's id must be indistinguishable from one that never
// existed — not a 403, which would confirm the id is real.
func TestConversationsAreScopedToTheirOwner(t *testing.T) {
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	alice, bob := seedScope(t, st)

	conv, err := cs.CreateConversation(ctx, alice, "Alice's chat")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := cs.Conversation(ctx, bob, conv.ID); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("bob loading alice's conversation: err = %v, want ErrNotFound", err)
	}
	if _, err := cs.Messages(ctx, bob, conv.ID); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("bob reading alice's messages: err = %v, want ErrNotFound", err)
	}
	if _, err := cs.SetTitle(ctx, bob, conv.ID, "hijacked"); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("bob renaming alice's conversation: err = %v, want ErrNotFound", err)
	}
	if err := cs.DeleteConversation(ctx, bob, conv.ID); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("bob deleting alice's conversation: err = %v, want ErrNotFound", err)
	}
	_, err = cs.AppendMessage(ctx, bob, conv.ID, chatstore.AppendInput{Role: chatstore.RoleUser, Content: "hi"})
	if !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("bob appending to alice's conversation: err = %v, want ErrNotFound", err)
	}

	bobList, err := cs.ListConversations(ctx, bob, 0)
	if err != nil {
		t.Fatalf("bob list: %v", err)
	}
	if len(bobList) != 0 {
		t.Errorf("bob's conversation list = %+v, want empty", bobList)
	}
}

func TestAppendAndReadMessages(t *testing.T) {
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	alice, _ := seedScope(t, st)

	conv, err := cs.CreateConversation(ctx, alice, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A pre-generated id is how the chat service names an assistant turn in
	// the stream's opening event, before any text exists.
	assistantID := uuid.New()
	meta := json.RawMessage(`{"components":[{"type":"kb_item","data":{}}]}`)
	for _, in := range []chatstore.AppendInput{
		{Role: chatstore.RoleUser, Content: "what is the price of Vitamin D?"},
		{ID: assistantID, Role: chatstore.RoleAssistant, Content: "12 000 KZT", Metadata: meta},
	} {
		if _, err := cs.AppendMessage(ctx, alice, conv.ID, in); err != nil {
			t.Fatalf("append %s: %v", in.Role, err)
		}
	}

	msgs, err := cs.Messages(ctx, alice, conv.ID)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != chatstore.RoleUser || msgs[1].Role != chatstore.RoleAssistant {
		t.Errorf("roles = %q, %q — want the transcript oldest first", msgs[0].Role, msgs[1].Role)
	}
	if msgs[1].ID != assistantID {
		t.Errorf("assistant id = %s, want the pre-generated %s", msgs[1].ID, assistantID)
	}
	if string(msgs[1].Metadata) != string(meta) {
		t.Errorf("metadata = %s, want %s", msgs[1].Metadata, meta)
	}
	// A user turn stores an empty JSON object, never NULL or "".
	if string(msgs[0].Metadata) != "{}" {
		t.Errorf("user metadata = %q, want %q", msgs[0].Metadata, "{}")
	}

	n, err := cs.CountMessages(ctx, alice, conv.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

// RecentMessages is the only history the model ever sees, so both halves of
// its contract matter: the newest n turns, and in oldest-first order.
func TestRecentMessagesReturnsTheLastNOldestFirst(t *testing.T) {
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	alice, _ := seedScope(t, st)

	conv, err := cs.CreateConversation(ctx, alice, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, body := range []string{"m1", "m2", "m3", "m4", "m5"} {
		if _, err := cs.AppendMessage(ctx, alice, conv.ID, chatstore.AppendInput{Role: chatstore.RoleUser, Content: body}); err != nil {
			t.Fatalf("append %s: %v", body, err)
		}
	}

	recent, err := cs.RecentMessages(ctx, alice, conv.ID, 3)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	got := make([]string, len(recent))
	for i, m := range recent {
		got[i] = m.Content
	}
	want := []string{"m3", "m4", "m5"}
	if len(got) != len(want) {
		t.Fatalf("recent = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recent = %v, want %v", got, want)
		}
	}

	// A misconfigured window degrades to "no history", never to "everything".
	none, err := cs.RecentMessages(ctx, alice, conv.ID, 0)
	if err != nil {
		t.Fatalf("recent(0): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("recent(0) returned %d messages, want none", len(none))
	}
}

// The sidebar orders by activity, so appending must move a thread to the top
// — the whole reason AppendMessage touches the conversation in the same
// transaction.
func TestAppendMovesConversationToTopOfList(t *testing.T) {
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	alice, _ := seedScope(t, st)

	first, err := cs.CreateConversation(ctx, alice, "first")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := cs.CreateConversation(ctx, alice, "second")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if _, err := cs.AppendMessage(ctx, alice, first.ID, chatstore.AppendInput{Role: chatstore.RoleUser, Content: "hello"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	list, err := cs.ListConversations(ctx, alice, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	if list[0].ID != first.ID {
		t.Errorf("list[0] = %s, want the just-appended-to conversation %s (second is %s)", list[0].ID, first.ID, second.ID)
	}
}

// Deleting a conversation must take its transcript with it — the cascade is
// what makes "delete this chat" one statement instead of a cleanup job.
func TestDeleteCascadesToMessages(t *testing.T) {
	cs, st, db := dbtest.NewChat(t)
	ctx := context.Background()
	alice, _ := seedScope(t, st)

	conv, err := cs.CreateConversation(ctx, alice, "doomed")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := cs.AppendMessage(ctx, alice, conv.ID, chatstore.AppendInput{Role: chatstore.RoleUser, Content: "hi"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := cs.DeleteConversation(ctx, alice, conv.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var remaining int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM chat_messages WHERE conversation_id = $1`, conv.ID).Scan(&remaining); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d messages survived their deleted conversation", remaining)
	}
}

// The role CHECK is the schema's own guard against a turn the prompt builder
// would not know what to do with.
func TestUnknownRoleIsRejected(t *testing.T) {
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	alice, _ := seedScope(t, st)

	conv, err := cs.CreateConversation(ctx, alice, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := cs.AppendMessage(ctx, alice, conv.ID, chatstore.AppendInput{Role: "tool", Content: "{}"}); err == nil {
		t.Error("appending a 'tool' role succeeded, want the role CHECK to reject it")
	}
}

// Two turns appended within the same millisecond must still read back in the
// order they were written. created_at alone cannot express that (it has
// millisecond resolution), which is why chat_messages carries an explicit
// seq — a transcript that rendered an answer before its own question, or a
// context window that picked turns by coin flip, would both be silent
// corruption rather than a visible failure.
func TestMessageOrderIsStableWithinOneMillisecond(t *testing.T) {
	cs, st, db := dbtest.NewChat(t)
	ctx := context.Background()
	alice, _ := seedScope(t, st)

	conv, err := cs.CreateConversation(ctx, alice, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	want := []string{"first", "second", "third", "fourth"}
	for _, body := range want {
		if _, err := cs.AppendMessage(ctx, alice, conv.ID, chatstore.AppendInput{Role: chatstore.RoleUser, Content: body}); err != nil {
			t.Fatalf("append %s: %v", body, err)
		}
	}
	// Collapse every timestamp to one value: whatever ordering survives this
	// is ordering that does not depend on the clock at all.
	if _, err := db.Exec(ctx,
		`UPDATE chat_messages SET created_at = '2026-01-01 00:00:00.000' WHERE conversation_id = $1`, conv.ID); err != nil {
		t.Fatalf("flatten timestamps: %v", err)
	}

	msgs, err := cs.Messages(ctx, alice, conv.ID)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(msgs) != len(want) {
		t.Fatalf("len(messages) = %d, want %d", len(msgs), len(want))
	}
	for i, w := range want {
		if msgs[i].Content != w {
			t.Fatalf("transcript order = %v, want %v", contents(msgs), want)
		}
		if msgs[i].Seq != int64(i+1) {
			t.Errorf("messages[%d].Seq = %d, want %d", i, msgs[i].Seq, i+1)
		}
	}

	recent, err := cs.RecentMessages(ctx, alice, conv.ID, 2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if got := contents(recent); len(got) != 2 || got[0] != "third" || got[1] != "fourth" {
		t.Errorf("recent(2) = %v, want [third fourth]", got)
	}
}

func contents(msgs []chatstore.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Content
	}
	return out
}
