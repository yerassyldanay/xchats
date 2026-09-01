-- xchats SQLite schema, part 15: Campaign message templates (CAM-14) — a
-- small, organization-wide library of reusable {{variable}}-templated
-- message bodies, so an operator building a new campaign can start from a
-- proven message instead of retyping (or copy-pasting from an old
-- campaign, which drags its whole audience/pace/schedule along with it).
--
-- Deliberately its own table, not a "campaign with no recipients": a
-- template has no account, channel, status, pace, or schedule at all — it
-- is pure content, applied to whichever campaign an operator is building
-- right now. is_archived is a soft hide (never a delete) so a template
-- already used by past campaigns.variables/message_body snapshots is never
-- lost — those columns copy the text at creation time and carry no FK back
-- here, so archiving (or one day deleting) a template can never affect a
-- campaign that already used it.
CREATE TABLE campaign_templates (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    message_body          TEXT NOT NULL DEFAULT '',
    -- variables mirrors campaigns.variables exactly (same backend/campaign.
    -- ExtractVariables cache, same "informational, Render never reads it"
    -- rule) — the wizard's own variable-vs-CSV-column check reads this
    -- rather than re-parsing message_body client-side.
    variables             TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(variables)),
    is_archived           INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0,1)),
    created_by            TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX campaign_templates_org_idx ON campaign_templates(organization_id);
-- Backs the Templates tab's Active/Archived filter — always org-scoped
-- first, so this covers ListCampaignTemplatesForOrg's WHERE clause
-- regardless of which side of that filter it's on.
CREATE INDEX campaign_templates_org_archived_idx ON campaign_templates(organization_id, is_archived);
