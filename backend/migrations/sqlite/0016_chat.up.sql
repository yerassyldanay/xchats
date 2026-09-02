-- xchats SQLite schema, part 16: the Knowledge Base chat assistant.
--
-- A ChatGPT-shaped conversation surface over the org's own Knowledge Base
-- (/chat). Everything this feature owns carries the chat_ prefix, so it is
-- unambiguously separate from the CUSTOMER-facing conversation tables
-- (wa_chats/tg_chats/channel_threads and their messages, 0002/0011): those
-- are inbound traffic from real people on a channel, these are an operator
-- talking to the assistant about their own KB. Nothing here is ever
-- delivered to a customer.
--
-- Follows 0001_core.up.sql's conventions verbatim (uuidv4 default
-- expression, TEXT timestamps in internal/dbx.TimeLayout format, json_valid
-- CHECKs).
--
-- Scoping is (organization_id, user_id): a conversation is private to the
-- operator who started it, inside the organization it was started in, so a
-- second operator in the same org never sees another's chat history. Both
-- FKs cascade — deleting an org or a user takes their chat history with it,
-- and chat_messages cascades from its conversation, which is what makes
-- "delete this conversation" a single DELETE.

CREATE TABLE chat_conversations (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

-- The sidebar's only query: this operator's conversations in this org, most
-- recently active first. Ordering lives in the index rather than being
-- sorted per request — the list is read on every /chat page load.
CREATE INDEX chat_conversations_owner_idx
    ON chat_conversations(organization_id, user_id, updated_at DESC);

CREATE TABLE chat_messages (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    -- seq is the turn's 1-based position in its conversation, and it — not
    -- created_at — is what every read orders by. created_at has millisecond
    -- resolution, which two appends can share; a transcript that rendered a
    -- question after its own answer because of a tie would be nonsense, and
    -- the last-N window sent to the model would be picking turns by coin
    -- flip. Assigned by the writer inside the same transaction as the insert
    -- (see chatstore.AppendMessage), which the single-writer connection
    -- makes race-free.
    seq             INTEGER NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('user','assistant','system')),
    content         TEXT NOT NULL DEFAULT '',
    -- metadata carries everything about an assistant turn that is not its
    -- prose: the structured KB components the UI renders as cards
    -- ({"components":[{"type":"kb_comparison","data":{...}}]}), token usage,
    -- and the provider/model the answer came from. A user turn stores '{}'.
    metadata        TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata)),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

-- Both message reads are the same shape — the full transcript for rendering
-- and the last-N window for the LLM context — so one ascending index serves
-- both (the window is the tail of this ordering). UNIQUE because a duplicate
-- position within a conversation is exactly the ambiguity seq exists to
-- remove, and because it is what makes the writer's "MAX(seq) + 1" safe
-- against a concurrent second writer rather than merely unlikely to collide.
CREATE UNIQUE INDEX chat_messages_conversation_seq_idx
    ON chat_messages(conversation_id, seq);
