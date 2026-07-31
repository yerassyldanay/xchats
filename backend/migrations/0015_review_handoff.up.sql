-- 0015_review_handoff — active-organization state for the browser session
-- (plan Task 15: a user in multiple organizations who authorizes the MCP
-- connector for org B must land on /playground scoped to B, not whichever
-- org OrgForUser's single-org default picks). active_organization_id is set
-- either by a verified review-handoff redirect (mcpauth's signed,
-- one-time-use token — see internal/mcpauth/review_handoff.go) or by an
-- explicit org switch the frontend now offers; ON DELETE SET NULL so a
-- deleted organization never leaves a session pointing at a dangling id, in
-- which case orgOf's existing OrgForUser deterministic-default takes over
-- unchanged.
SET search_path = xchats, public;

ALTER TABLE xchats.sessions
    ADD COLUMN active_organization_id uuid REFERENCES xchats.organizations(id) ON DELETE SET NULL;
