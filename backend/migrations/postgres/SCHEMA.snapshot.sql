--
-- PostgreSQL database dump
--

\restrict W7lmeAOYnNM832HvwqU6660CXVEqi7zHhf2QqpIvg7O8963nP8T6VkhgZwoFq7f

-- Dumped from database version 16.13 (Ubuntu 16.13-0ubuntu0.24.04.1)
-- Dumped by pg_dump version 16.13 (Ubuntu 16.13-0ubuntu0.24.04.1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: xchats; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA xchats;


--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: EXTENSION citext; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: xchats_schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.xchats_schema_migrations (
    version text NOT NULL,
    applied_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ai_assistants; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_assistants (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    persona text DEFAULT ''::text NOT NULL,
    mission text DEFAULT ''::text NOT NULL,
    guardrails text DEFAULT ''::text NOT NULL,
    language_policy text DEFAULT ''::text NOT NULL,
    reply_max_words integer DEFAULT 120 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ai_audit_log; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_audit_log (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    action text NOT NULL,
    actor_user_id uuid,
    note text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ai_contacts; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_contacts (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    whatsapp text DEFAULT ''::text NOT NULL,
    email text DEFAULT ''::text NOT NULL,
    address text DEFAULT ''::text NOT NULL,
    callback_time text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    working_hours text DEFAULT ''::text NOT NULL,
    phone text DEFAULT ''::text NOT NULL,
    website text DEFAULT ''::text NOT NULL,
    instagram text DEFAULT ''::text NOT NULL,
    legal_information text,
    contact_card_image uuid,
    location_map_image uuid,
    company_legal_documents uuid[] DEFAULT '{}'::uuid[] NOT NULL
);


--
-- Name: ai_delivery_zones; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_delivery_zones (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    ref text NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    zone_level text NOT NULL,
    parent_ref text DEFAULT ''::text NOT NULL,
    delivery_available boolean NOT NULL,
    delivery_cost text DEFAULT ''::text NOT NULL,
    delivery_in_days text DEFAULT ''::text NOT NULL,
    notes text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    sales_status text DEFAULT 'active'::text NOT NULL,
    CONSTRAINT ai_delivery_zones_zone_level_check CHECK ((zone_level = ANY (ARRAY['city'::text, 'region'::text, 'country'::text])))
);


--
-- Name: ai_drafts; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_drafts (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    chat_id uuid NOT NULL,
    trigger_message_id uuid,
    option_ordinal integer NOT NULL,
    draft_text text DEFAULT ''::text NOT NULL,
    sent_message_id uuid,
    context_state text DEFAULT 'full'::text NOT NULL,
    confidence numeric,
    escalate boolean DEFAULT false NOT NULL,
    escalation_reason text DEFAULT ''::text NOT NULL,
    draft_state text DEFAULT 'suggested'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    reply_language text DEFAULT ''::text NOT NULL,
    channel text DEFAULT 'whatsapp'::text NOT NULL
);


--
-- Name: ai_policies; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_policies (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    delivery_cost text DEFAULT ''::text NOT NULL,
    free_delivery_from text DEFAULT ''::text NOT NULL,
    min_order text DEFAULT ''::text NOT NULL,
    prepayment text DEFAULT ''::text NOT NULL,
    installment text DEFAULT ''::text NOT NULL,
    warranty text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    outside_zones_note text DEFAULT ''::text NOT NULL,
    delivery_in_days text,
    return_period_in_days text,
    commerce_policy_documents uuid[] DEFAULT '{}'::uuid[] NOT NULL
);


--
-- Name: ai_products; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_products (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    ref text NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    price text DEFAULT ''::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    category text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    in_stock boolean NOT NULL,
    sales_status text DEFAULT 'active'::text NOT NULL,
    featured_image uuid,
    gallery_images uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    demo_videos uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    certificate_documents uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    guarantee_documents uuid[] DEFAULT '{}'::uuid[] NOT NULL
);


--
-- Name: ai_tariffs; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_tariffs (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    ref text NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    price text DEFAULT ''::text NOT NULL,
    limit_text text DEFAULT ''::text NOT NULL,
    fee text DEFAULT ''::text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    pricing_type text DEFAULT 'fixed'::text NOT NULL,
    advantages text DEFAULT ''::text NOT NULL,
    disadvantages text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    sales_status text DEFAULT 'active'::text NOT NULL,
    featured_image uuid,
    pricing_images uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    explainer_videos uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    terms_documents uuid[] DEFAULT '{}'::uuid[] NOT NULL
);


--
-- Name: ai_topics; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.ai_topics (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    slug text NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    body_md text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    featured_image uuid,
    illustration_images uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    explainer_videos uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    reference_documents uuid[] DEFAULT '{}'::uuid[] NOT NULL
);


--
-- Name: tg_accounts; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.tg_accounts (
    id uuid NOT NULL,
    organization_id uuid,
    display_name text DEFAULT ''::text NOT NULL,
    bot_id bigint NOT NULL,
    bot_username text DEFAULT ''::text NOT NULL,
    connection_state text DEFAULT 'connecting'::text NOT NULL,
    webhook_url text DEFAULT ''::text NOT NULL,
    webhook_registered_at timestamp with time zone,
    webhook_last_checked_at timestamp with time zone,
    webhook_last_error text DEFAULT ''::text NOT NULL,
    last_live_event_at timestamp with time zone,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: wa_accounts; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.wa_accounts (
    id uuid NOT NULL,
    organization_id uuid,
    display_name text DEFAULT ''::text NOT NULL,
    owner_jid text NOT NULL,
    phone_number text DEFAULT ''::text NOT NULL,
    evolution_instance_name text DEFAULT ''::text NOT NULL,
    evolution_instance_id text DEFAULT ''::text NOT NULL,
    connection_state text DEFAULT 'connected'::text NOT NULL,
    last_live_event_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    channel text DEFAULT 'whatsapp'::text NOT NULL,
    CONSTRAINT wa_accounts_channel_check CHECK ((channel = ANY (ARRAY['whatsapp'::text, 'simulator'::text])))
);


--
-- Name: inbox_accounts_v; Type: VIEW; Schema: xchats; Owner: -
--

CREATE VIEW xchats.inbox_accounts_v AS
 SELECT a.id,
    a.organization_id,
    a.channel,
    a.display_name,
    a.phone_number AS external_handle,
    a.owner_jid AS external_account_ref,
    a.evolution_instance_name AS instance_name,
    a.connection_state,
    a.last_live_event_at,
    a.deleted_at,
    a.created_at,
    NULL::text AS webhook_url,
    NULL::timestamp with time zone AS webhook_registered_at,
    NULL::timestamp with time zone AS webhook_last_checked_at,
    NULL::text AS webhook_last_error
   FROM xchats.wa_accounts a
UNION ALL
 SELECT t.id,
    t.organization_id,
    'telegram'::text AS channel,
    t.display_name,
    ('@'::text || t.bot_username) AS external_handle,
    ('telegram:bot:'::text || (t.bot_id)::text) AS external_account_ref,
    ''::text AS instance_name,
    t.connection_state,
    t.last_live_event_at,
    t.deleted_at,
    t.created_at,
    t.webhook_url,
    t.webhook_registered_at,
    t.webhook_last_checked_at,
    t.webhook_last_error
   FROM xchats.tg_accounts t;


--
-- Name: tg_chats; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.tg_chats (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    account_id uuid NOT NULL,
    contact_id uuid NOT NULL,
    telegram_chat_id bigint NOT NULL,
    chat_type text DEFAULT 'private'::text NOT NULL,
    chat_state text DEFAULT 'open'::text NOT NULL,
    assignee_user_id uuid,
    last_message_at timestamp with time zone,
    last_message_preview text DEFAULT ''::text NOT NULL,
    unread_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tg_contacts; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.tg_contacts (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    account_id uuid NOT NULL,
    telegram_user_id bigint NOT NULL,
    username text DEFAULT ''::text NOT NULL,
    first_name text DEFAULT ''::text NOT NULL,
    last_name text DEFAULT ''::text NOT NULL,
    display_name text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: wa_chats; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.wa_chats (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    account_id uuid NOT NULL,
    contact_id uuid NOT NULL,
    remote_jid text NOT NULL,
    chat_state text DEFAULT 'open'::text NOT NULL,
    assignee_user_id uuid,
    stage text DEFAULT ''::text NOT NULL,
    ai_summary text DEFAULT ''::text NOT NULL,
    last_message_at timestamp with time zone,
    last_message_preview text DEFAULT ''::text NOT NULL,
    unread_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: wa_contacts; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.wa_contacts (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    account_id uuid NOT NULL,
    phone_number text DEFAULT ''::text NOT NULL,
    phone_jid text NOT NULL,
    lid_jid text,
    push_name text DEFAULT ''::text NOT NULL,
    display_name text DEFAULT ''::text NOT NULL,
    avatar_url text,
    attributes jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: inbox_chats_v; Type: VIEW; Schema: xchats; Owner: -
--

CREATE VIEW xchats.inbox_chats_v AS
 SELECT c.id,
    c.account_id,
    a.organization_id,
    a.channel,
    a.deleted_at AS account_deleted_at,
    c.remote_jid AS external_conversation_ref,
    c.chat_state,
    c.assignee_user_id,
    c.last_message_at,
    c.last_message_preview,
    c.unread_count,
    c.created_at,
    c.updated_at,
    ct.id AS contact_id,
    ct.phone_number AS contact_phone_number,
    ct.phone_jid AS external_contact_ref,
    COALESCE(ct.lid_jid, ''::text) AS contact_lid_jid,
    ct.push_name AS contact_push_name,
    ct.display_name AS contact_display_name,
    ct.attributes AS contact_attributes
   FROM ((xchats.wa_chats c
     JOIN xchats.wa_contacts ct ON ((ct.id = c.contact_id)))
     JOIN xchats.wa_accounts a ON ((a.id = c.account_id)))
UNION ALL
 SELECT c.id,
    c.account_id,
    t.organization_id,
    'telegram'::text AS channel,
    t.deleted_at AS account_deleted_at,
    (c.telegram_chat_id)::text AS external_conversation_ref,
    c.chat_state,
    c.assignee_user_id,
    c.last_message_at,
    c.last_message_preview,
    c.unread_count,
    c.created_at,
    c.updated_at,
    ct.id AS contact_id,
    ''::text AS contact_phone_number,
    (ct.telegram_user_id)::text AS external_contact_ref,
    ''::text AS contact_lid_jid,
    COALESCE(NULLIF(TRIM(BOTH FROM ((ct.first_name || ' '::text) || ct.last_name)), ''::text), ct.username) AS contact_push_name,
    ct.display_name AS contact_display_name,
    '{}'::jsonb AS contact_attributes
   FROM ((xchats.tg_chats c
     JOIN xchats.tg_contacts ct ON ((ct.id = c.contact_id)))
     JOIN xchats.tg_accounts t ON ((t.id = c.account_id)));


--
-- Name: message_media; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.message_media (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    message_id uuid NOT NULL,
    media_type text NOT NULL,
    mimetype text DEFAULT ''::text NOT NULL,
    file_name text DEFAULT ''::text NOT NULL,
    file_size integer DEFAULT 0 NOT NULL,
    storage_url text DEFAULT ''::text NOT NULL,
    download_status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tg_message_media; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.tg_message_media (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    message_id uuid NOT NULL,
    file_id text NOT NULL,
    file_unique_id text DEFAULT ''::text NOT NULL,
    media_type text NOT NULL,
    mimetype text DEFAULT ''::text NOT NULL,
    filename text DEFAULT ''::text NOT NULL,
    size integer DEFAULT 0 NOT NULL,
    storage_key text DEFAULT ''::text NOT NULL,
    download_status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: wa_messages; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.wa_messages (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    account_id uuid NOT NULL,
    chat_id uuid NOT NULL,
    direction text NOT NULL,
    sender_kind text NOT NULL,
    sender_user_id uuid,
    evolution_message_id text,
    participant_jid text DEFAULT ''::text NOT NULL,
    message_kind text DEFAULT ''::text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    delivery_state text DEFAULT 'queued'::text NOT NULL,
    source text DEFAULT 'live_webhook'::text NOT NULL,
    raw jsonb,
    message_ts timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: inbox_message_media_v; Type: VIEW; Schema: xchats; Owner: -
--

CREATE VIEW xchats.inbox_message_media_v AS
 SELECT mm.id,
    mm.message_id,
    a.channel,
    mm.media_type,
    mm.mimetype,
    mm.file_name AS filename,
    mm.file_size AS size,
    mm.storage_url AS storage_key,
    mm.download_status,
    mm.created_at
   FROM ((xchats.message_media mm
     JOIN xchats.wa_messages m ON ((m.id = mm.message_id)))
     JOIN xchats.wa_accounts a ON ((a.id = m.account_id)))
UNION ALL
 SELECT tm.id,
    tm.message_id,
    'telegram'::text AS channel,
    tm.media_type,
    tm.mimetype,
    tm.filename,
    tm.size,
    tm.storage_key,
    tm.download_status,
    tm.created_at
   FROM xchats.tg_message_media tm;


--
-- Name: tg_messages; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.tg_messages (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    account_id uuid NOT NULL,
    chat_id uuid NOT NULL,
    direction text NOT NULL,
    sender_kind text NOT NULL,
    sender_user_id uuid,
    telegram_update_id bigint,
    telegram_message_id bigint,
    message_kind text DEFAULT ''::text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    delivery_state text DEFAULT 'queued'::text NOT NULL,
    source text DEFAULT 'live_webhook'::text NOT NULL,
    raw jsonb,
    message_ts timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: inbox_messages_v; Type: VIEW; Schema: xchats; Owner: -
--

CREATE VIEW xchats.inbox_messages_v AS
 SELECT m.id,
    m.chat_id,
    m.account_id,
    a.channel,
    m.direction,
    m.sender_kind,
    m.sender_user_id,
    COALESCE(m.evolution_message_id, ''::text) AS external_message_id,
    m.message_kind,
    m.body,
    m.delivery_state,
    m.source,
    m.message_ts,
    m.created_at
   FROM (xchats.wa_messages m
     JOIN xchats.wa_accounts a ON ((a.id = m.account_id)))
UNION ALL
 SELECT m.id,
    m.chat_id,
    m.account_id,
    'telegram'::text AS channel,
    m.direction,
    m.sender_kind,
    m.sender_user_id,
    COALESCE((m.telegram_message_id)::text, ''::text) AS external_message_id,
    m.message_kind,
    m.body,
    m.delivery_state,
    m.source,
    m.message_ts,
    m.created_at
   FROM xchats.tg_messages m;


--
-- Name: kbd_draft; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.kbd_draft (
    organization_id uuid NOT NULL,
    draft jsonb DEFAULT '{}'::jsonb NOT NULL,
    base_version bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid
);


--
-- Name: kbd_materials; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.kbd_materials (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    source_type text NOT NULL,
    source_ref text DEFAULT ''::text NOT NULL,
    blob_id text DEFAULT ''::text NOT NULL,
    extracted_text text DEFAULT ''::text NOT NULL,
    media_kind text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    extraction jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    source_text text,
    operator_note text,
    storage_backend text,
    storage_key text,
    filename text,
    mime_type text,
    size_bytes bigint,
    sha256_checksum text,
    visual_summary text,
    transcript_text text,
    extraction_metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    processing_status text DEFAULT 'uploaded'::text NOT NULL,
    customer_visibility text
);


--
-- Name: kbd_requests; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.kbd_requests (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    organization_id uuid NOT NULL,
    material_id uuid,
    req_type text NOT NULL,
    prompt text DEFAULT ''::text NOT NULL,
    context jsonb DEFAULT '{}'::jsonb NOT NULL,
    target jsonb DEFAULT '{}'::jsonb NOT NULL,
    state text DEFAULT 'pending'::text NOT NULL,
    resolution jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    resolved_at timestamp with time zone
);


--
-- Name: mcp_access_token_denylist; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.mcp_access_token_denylist (
    jti text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: mcp_authorization_codes; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.mcp_authorization_codes (
    code_hash text NOT NULL,
    client_id text NOT NULL,
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL,
    code_challenge_method text DEFAULT 'S256'::text NOT NULL,
    user_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    scope text DEFAULT ''::text NOT NULL,
    resource text DEFAULT ''::text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT mcp_authorization_codes_code_challenge_method_check CHECK ((code_challenge_method = 'S256'::text))
);


--
-- Name: mcp_oauth_clients; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.mcp_oauth_clients (
    client_id text NOT NULL,
    client_name text DEFAULT ''::text NOT NULL,
    redirect_uris text[] DEFAULT '{}'::text[] NOT NULL,
    registration_source text DEFAULT 'dcr'::text NOT NULL,
    token_endpoint_auth_method text DEFAULT 'none'::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT mcp_oauth_clients_registration_source_check CHECK ((registration_source = ANY (ARRAY['dcr'::text, 'cimd'::text])))
);


--
-- Name: mcp_refresh_tokens; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.mcp_refresh_tokens (
    token_hash text NOT NULL,
    client_id text NOT NULL,
    user_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    scope text DEFAULT ''::text NOT NULL,
    resource text DEFAULT ''::text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    replaced_by text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: organization_users; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.organization_users (
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    joined_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: organizations; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.organizations (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name text NOT NULL,
    respond_mode text DEFAULT 'NEVER'::text NOT NULL,
    respond_window_start time without time zone,
    respond_window_end time without time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: sessions; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.sessions (
    id text NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    active_organization_id uuid
);


--
-- Name: tg_credentials; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.tg_credentials (
    account_id uuid NOT NULL,
    bot_token_enc bytea NOT NULL,
    encryption_key_version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: xchats; Owner: -
--

CREATE TABLE xchats.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email public.citext NOT NULL,
    password_hash text NOT NULL,
    display_name text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: xchats_schema_migrations xchats_schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.xchats_schema_migrations
    ADD CONSTRAINT xchats_schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: ai_assistants ai_assistants_organization_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_assistants
    ADD CONSTRAINT ai_assistants_organization_id_key UNIQUE (organization_id);


--
-- Name: ai_assistants ai_assistants_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_assistants
    ADD CONSTRAINT ai_assistants_pkey PRIMARY KEY (id);


--
-- Name: ai_audit_log ai_audit_log_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_audit_log
    ADD CONSTRAINT ai_audit_log_pkey PRIMARY KEY (id);


--
-- Name: ai_contacts ai_contacts_organization_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_contacts
    ADD CONSTRAINT ai_contacts_organization_id_key UNIQUE (organization_id);


--
-- Name: ai_contacts ai_contacts_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_contacts
    ADD CONSTRAINT ai_contacts_pkey PRIMARY KEY (id);


--
-- Name: ai_delivery_zones ai_delivery_zones_organization_id_ref_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_delivery_zones
    ADD CONSTRAINT ai_delivery_zones_organization_id_ref_key UNIQUE (organization_id, ref);


--
-- Name: ai_delivery_zones ai_delivery_zones_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_delivery_zones
    ADD CONSTRAINT ai_delivery_zones_pkey PRIMARY KEY (id);


--
-- Name: ai_drafts ai_drafts_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_drafts
    ADD CONSTRAINT ai_drafts_pkey PRIMARY KEY (id);


--
-- Name: ai_policies ai_policies_organization_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_policies
    ADD CONSTRAINT ai_policies_organization_id_key UNIQUE (organization_id);


--
-- Name: ai_policies ai_policies_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_policies
    ADD CONSTRAINT ai_policies_pkey PRIMARY KEY (id);


--
-- Name: ai_products ai_products_organization_id_ref_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_products
    ADD CONSTRAINT ai_products_organization_id_ref_key UNIQUE (organization_id, ref);


--
-- Name: ai_products ai_products_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_products
    ADD CONSTRAINT ai_products_pkey PRIMARY KEY (id);


--
-- Name: ai_tariffs ai_tariffs_organization_id_ref_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_tariffs
    ADD CONSTRAINT ai_tariffs_organization_id_ref_key UNIQUE (organization_id, ref);


--
-- Name: ai_tariffs ai_tariffs_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_tariffs
    ADD CONSTRAINT ai_tariffs_pkey PRIMARY KEY (id);


--
-- Name: ai_topics ai_topics_organization_id_slug_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_topics
    ADD CONSTRAINT ai_topics_organization_id_slug_key UNIQUE (organization_id, slug);


--
-- Name: ai_topics ai_topics_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_topics
    ADD CONSTRAINT ai_topics_pkey PRIMARY KEY (id);


--
-- Name: kbd_draft kbd_draft_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_draft
    ADD CONSTRAINT kbd_draft_pkey PRIMARY KEY (organization_id);


--
-- Name: kbd_materials kbd_materials_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_materials
    ADD CONSTRAINT kbd_materials_pkey PRIMARY KEY (id);


--
-- Name: kbd_requests kbd_requests_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_requests
    ADD CONSTRAINT kbd_requests_pkey PRIMARY KEY (id);


--
-- Name: mcp_access_token_denylist mcp_access_token_denylist_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_access_token_denylist
    ADD CONSTRAINT mcp_access_token_denylist_pkey PRIMARY KEY (jti);


--
-- Name: mcp_authorization_codes mcp_authorization_codes_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_authorization_codes
    ADD CONSTRAINT mcp_authorization_codes_pkey PRIMARY KEY (code_hash);


--
-- Name: mcp_oauth_clients mcp_oauth_clients_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_oauth_clients
    ADD CONSTRAINT mcp_oauth_clients_pkey PRIMARY KEY (client_id);


--
-- Name: mcp_refresh_tokens mcp_refresh_tokens_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_refresh_tokens
    ADD CONSTRAINT mcp_refresh_tokens_pkey PRIMARY KEY (token_hash);


--
-- Name: message_media message_media_message_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.message_media
    ADD CONSTRAINT message_media_message_id_key UNIQUE (message_id);


--
-- Name: message_media message_media_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.message_media
    ADD CONSTRAINT message_media_pkey PRIMARY KEY (id);


--
-- Name: organization_users organization_users_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.organization_users
    ADD CONSTRAINT organization_users_pkey PRIMARY KEY (organization_id, user_id);


--
-- Name: organizations organizations_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: tg_accounts tg_accounts_bot_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_accounts
    ADD CONSTRAINT tg_accounts_bot_id_key UNIQUE (bot_id);


--
-- Name: tg_accounts tg_accounts_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_accounts
    ADD CONSTRAINT tg_accounts_pkey PRIMARY KEY (id);


--
-- Name: tg_chats tg_chats_account_id_telegram_chat_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_chats
    ADD CONSTRAINT tg_chats_account_id_telegram_chat_id_key UNIQUE (account_id, telegram_chat_id);


--
-- Name: tg_chats tg_chats_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_chats
    ADD CONSTRAINT tg_chats_pkey PRIMARY KEY (id);


--
-- Name: tg_contacts tg_contacts_account_id_telegram_user_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_contacts
    ADD CONSTRAINT tg_contacts_account_id_telegram_user_id_key UNIQUE (account_id, telegram_user_id);


--
-- Name: tg_contacts tg_contacts_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_contacts
    ADD CONSTRAINT tg_contacts_pkey PRIMARY KEY (id);


--
-- Name: tg_credentials tg_credentials_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_credentials
    ADD CONSTRAINT tg_credentials_pkey PRIMARY KEY (account_id);


--
-- Name: tg_message_media tg_message_media_message_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_message_media
    ADD CONSTRAINT tg_message_media_message_id_key UNIQUE (message_id);


--
-- Name: tg_message_media tg_message_media_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_message_media
    ADD CONSTRAINT tg_message_media_pkey PRIMARY KEY (id);


--
-- Name: tg_messages tg_messages_account_id_telegram_update_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_messages
    ADD CONSTRAINT tg_messages_account_id_telegram_update_id_key UNIQUE (account_id, telegram_update_id);


--
-- Name: tg_messages tg_messages_chat_id_telegram_message_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_messages
    ADD CONSTRAINT tg_messages_chat_id_telegram_message_id_key UNIQUE (chat_id, telegram_message_id);


--
-- Name: tg_messages tg_messages_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_messages
    ADD CONSTRAINT tg_messages_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: wa_accounts wa_accounts_owner_jid_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_accounts
    ADD CONSTRAINT wa_accounts_owner_jid_key UNIQUE (owner_jid);


--
-- Name: wa_accounts wa_accounts_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_accounts
    ADD CONSTRAINT wa_accounts_pkey PRIMARY KEY (id);


--
-- Name: wa_chats wa_chats_account_id_remote_jid_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_chats
    ADD CONSTRAINT wa_chats_account_id_remote_jid_key UNIQUE (account_id, remote_jid);


--
-- Name: wa_chats wa_chats_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_chats
    ADD CONSTRAINT wa_chats_pkey PRIMARY KEY (id);


--
-- Name: wa_contacts wa_contacts_account_id_phone_jid_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_contacts
    ADD CONSTRAINT wa_contacts_account_id_phone_jid_key UNIQUE (account_id, phone_jid);


--
-- Name: wa_contacts wa_contacts_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_contacts
    ADD CONSTRAINT wa_contacts_pkey PRIMARY KEY (id);


--
-- Name: wa_messages wa_messages_account_id_evolution_message_id_key; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_messages
    ADD CONSTRAINT wa_messages_account_id_evolution_message_id_key UNIQUE (account_id, evolution_message_id);


--
-- Name: wa_messages wa_messages_pkey; Type: CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_messages
    ADD CONSTRAINT wa_messages_pkey PRIMARY KEY (id);


--
-- Name: ai_contacts_contact_card_image_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX ai_contacts_contact_card_image_idx ON xchats.ai_contacts USING btree (contact_card_image);


--
-- Name: ai_contacts_location_map_image_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX ai_contacts_location_map_image_idx ON xchats.ai_contacts USING btree (location_map_image);


--
-- Name: ai_drafts_pending_uq; Type: INDEX; Schema: xchats; Owner: -
--

CREATE UNIQUE INDEX ai_drafts_pending_uq ON xchats.ai_drafts USING btree (channel, chat_id, option_ordinal) WHERE (draft_state = 'suggested'::text);


--
-- Name: ai_products_featured_image_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX ai_products_featured_image_idx ON xchats.ai_products USING btree (featured_image);


--
-- Name: ai_tariffs_featured_image_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX ai_tariffs_featured_image_idx ON xchats.ai_tariffs USING btree (featured_image);


--
-- Name: ai_topics_featured_image_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX ai_topics_featured_image_idx ON xchats.ai_topics USING btree (featured_image);


--
-- Name: kbd_materials_org_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX kbd_materials_org_idx ON xchats.kbd_materials USING btree (organization_id);


--
-- Name: kbd_materials_org_status_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX kbd_materials_org_status_idx ON xchats.kbd_materials USING btree (organization_id, processing_status);


--
-- Name: kbd_requests_org_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX kbd_requests_org_idx ON xchats.kbd_requests USING btree (organization_id, state);


--
-- Name: mcp_access_token_denylist_expires_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX mcp_access_token_denylist_expires_idx ON xchats.mcp_access_token_denylist USING btree (expires_at);


--
-- Name: mcp_authorization_codes_expires_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX mcp_authorization_codes_expires_idx ON xchats.mcp_authorization_codes USING btree (expires_at);


--
-- Name: mcp_refresh_tokens_client_user_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX mcp_refresh_tokens_client_user_idx ON xchats.mcp_refresh_tokens USING btree (client_id, user_id);


--
-- Name: sessions_user_id_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX sessions_user_id_idx ON xchats.sessions USING btree (user_id);


--
-- Name: tg_accounts_org_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX tg_accounts_org_idx ON xchats.tg_accounts USING btree (organization_id) WHERE (deleted_at IS NULL);


--
-- Name: tg_chats_last_message_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX tg_chats_last_message_idx ON xchats.tg_chats USING btree (last_message_at DESC NULLS LAST);


--
-- Name: tg_message_media_pending_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX tg_message_media_pending_idx ON xchats.tg_message_media USING btree (updated_at) WHERE (download_status <> 'ready'::text);


--
-- Name: tg_messages_chat_ts_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX tg_messages_chat_ts_idx ON xchats.tg_messages USING btree (chat_id, message_ts);


--
-- Name: wa_accounts_org_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX wa_accounts_org_idx ON xchats.wa_accounts USING btree (organization_id) WHERE (deleted_at IS NULL);


--
-- Name: wa_contacts_lid_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX wa_contacts_lid_idx ON xchats.wa_contacts USING btree (account_id, lid_jid);


--
-- Name: wa_messages_chat_ts_idx; Type: INDEX; Schema: xchats; Owner: -
--

CREATE INDEX wa_messages_chat_ts_idx ON xchats.wa_messages USING btree (chat_id, message_ts);


--
-- Name: ai_assistants ai_assistants_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_assistants
    ADD CONSTRAINT ai_assistants_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: ai_audit_log ai_audit_log_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_audit_log
    ADD CONSTRAINT ai_audit_log_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES xchats.users(id) ON DELETE SET NULL;


--
-- Name: ai_audit_log ai_audit_log_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_audit_log
    ADD CONSTRAINT ai_audit_log_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: ai_contacts ai_contacts_contact_card_image_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_contacts
    ADD CONSTRAINT ai_contacts_contact_card_image_fkey FOREIGN KEY (contact_card_image) REFERENCES xchats.kbd_materials(id);


--
-- Name: ai_contacts ai_contacts_location_map_image_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_contacts
    ADD CONSTRAINT ai_contacts_location_map_image_fkey FOREIGN KEY (location_map_image) REFERENCES xchats.kbd_materials(id);


--
-- Name: ai_contacts ai_contacts_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_contacts
    ADD CONSTRAINT ai_contacts_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: ai_delivery_zones ai_delivery_zones_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_delivery_zones
    ADD CONSTRAINT ai_delivery_zones_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: ai_policies ai_policies_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_policies
    ADD CONSTRAINT ai_policies_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: ai_products ai_products_featured_image_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_products
    ADD CONSTRAINT ai_products_featured_image_fkey FOREIGN KEY (featured_image) REFERENCES xchats.kbd_materials(id);


--
-- Name: ai_products ai_products_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_products
    ADD CONSTRAINT ai_products_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: ai_tariffs ai_tariffs_featured_image_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_tariffs
    ADD CONSTRAINT ai_tariffs_featured_image_fkey FOREIGN KEY (featured_image) REFERENCES xchats.kbd_materials(id);


--
-- Name: ai_tariffs ai_tariffs_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_tariffs
    ADD CONSTRAINT ai_tariffs_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: ai_topics ai_topics_featured_image_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_topics
    ADD CONSTRAINT ai_topics_featured_image_fkey FOREIGN KEY (featured_image) REFERENCES xchats.kbd_materials(id);


--
-- Name: ai_topics ai_topics_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.ai_topics
    ADD CONSTRAINT ai_topics_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: kbd_draft kbd_draft_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_draft
    ADD CONSTRAINT kbd_draft_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: kbd_draft kbd_draft_updated_by_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_draft
    ADD CONSTRAINT kbd_draft_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES xchats.users(id) ON DELETE SET NULL;


--
-- Name: kbd_materials kbd_materials_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_materials
    ADD CONSTRAINT kbd_materials_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: kbd_requests kbd_requests_material_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_requests
    ADD CONSTRAINT kbd_requests_material_id_fkey FOREIGN KEY (material_id) REFERENCES xchats.kbd_materials(id) ON DELETE SET NULL;


--
-- Name: kbd_requests kbd_requests_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.kbd_requests
    ADD CONSTRAINT kbd_requests_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: mcp_authorization_codes mcp_authorization_codes_client_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_authorization_codes
    ADD CONSTRAINT mcp_authorization_codes_client_id_fkey FOREIGN KEY (client_id) REFERENCES xchats.mcp_oauth_clients(client_id) ON DELETE CASCADE;


--
-- Name: mcp_authorization_codes mcp_authorization_codes_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_authorization_codes
    ADD CONSTRAINT mcp_authorization_codes_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: mcp_authorization_codes mcp_authorization_codes_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_authorization_codes
    ADD CONSTRAINT mcp_authorization_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES xchats.users(id) ON DELETE CASCADE;


--
-- Name: mcp_refresh_tokens mcp_refresh_tokens_client_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_refresh_tokens
    ADD CONSTRAINT mcp_refresh_tokens_client_id_fkey FOREIGN KEY (client_id) REFERENCES xchats.mcp_oauth_clients(client_id) ON DELETE CASCADE;


--
-- Name: mcp_refresh_tokens mcp_refresh_tokens_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_refresh_tokens
    ADD CONSTRAINT mcp_refresh_tokens_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: mcp_refresh_tokens mcp_refresh_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.mcp_refresh_tokens
    ADD CONSTRAINT mcp_refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES xchats.users(id) ON DELETE CASCADE;


--
-- Name: message_media message_media_message_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.message_media
    ADD CONSTRAINT message_media_message_id_fkey FOREIGN KEY (message_id) REFERENCES xchats.wa_messages(id) ON DELETE CASCADE;


--
-- Name: organization_users organization_users_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.organization_users
    ADD CONSTRAINT organization_users_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE CASCADE;


--
-- Name: organization_users organization_users_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.organization_users
    ADD CONSTRAINT organization_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES xchats.users(id) ON DELETE CASCADE;


--
-- Name: sessions sessions_active_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.sessions
    ADD CONSTRAINT sessions_active_organization_id_fkey FOREIGN KEY (active_organization_id) REFERENCES xchats.organizations(id) ON DELETE SET NULL;


--
-- Name: sessions sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES xchats.users(id) ON DELETE CASCADE;


--
-- Name: tg_accounts tg_accounts_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_accounts
    ADD CONSTRAINT tg_accounts_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE SET NULL;


--
-- Name: tg_chats tg_chats_account_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_chats
    ADD CONSTRAINT tg_chats_account_id_fkey FOREIGN KEY (account_id) REFERENCES xchats.tg_accounts(id) ON DELETE RESTRICT;


--
-- Name: tg_chats tg_chats_assignee_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_chats
    ADD CONSTRAINT tg_chats_assignee_user_id_fkey FOREIGN KEY (assignee_user_id) REFERENCES xchats.users(id) ON DELETE SET NULL;


--
-- Name: tg_chats tg_chats_contact_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_chats
    ADD CONSTRAINT tg_chats_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES xchats.tg_contacts(id) ON DELETE RESTRICT;


--
-- Name: tg_contacts tg_contacts_account_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_contacts
    ADD CONSTRAINT tg_contacts_account_id_fkey FOREIGN KEY (account_id) REFERENCES xchats.tg_accounts(id) ON DELETE RESTRICT;


--
-- Name: tg_credentials tg_credentials_account_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_credentials
    ADD CONSTRAINT tg_credentials_account_id_fkey FOREIGN KEY (account_id) REFERENCES xchats.tg_accounts(id) ON DELETE CASCADE;


--
-- Name: tg_message_media tg_message_media_message_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_message_media
    ADD CONSTRAINT tg_message_media_message_id_fkey FOREIGN KEY (message_id) REFERENCES xchats.tg_messages(id) ON DELETE CASCADE;


--
-- Name: tg_messages tg_messages_account_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_messages
    ADD CONSTRAINT tg_messages_account_id_fkey FOREIGN KEY (account_id) REFERENCES xchats.tg_accounts(id) ON DELETE RESTRICT;


--
-- Name: tg_messages tg_messages_chat_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_messages
    ADD CONSTRAINT tg_messages_chat_id_fkey FOREIGN KEY (chat_id) REFERENCES xchats.tg_chats(id) ON DELETE CASCADE;


--
-- Name: tg_messages tg_messages_sender_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.tg_messages
    ADD CONSTRAINT tg_messages_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES xchats.users(id) ON DELETE SET NULL;


--
-- Name: wa_accounts wa_accounts_organization_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_accounts
    ADD CONSTRAINT wa_accounts_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES xchats.organizations(id) ON DELETE SET NULL;


--
-- Name: wa_chats wa_chats_account_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_chats
    ADD CONSTRAINT wa_chats_account_id_fkey FOREIGN KEY (account_id) REFERENCES xchats.wa_accounts(id) ON DELETE RESTRICT;


--
-- Name: wa_chats wa_chats_assignee_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_chats
    ADD CONSTRAINT wa_chats_assignee_user_id_fkey FOREIGN KEY (assignee_user_id) REFERENCES xchats.users(id) ON DELETE SET NULL;


--
-- Name: wa_chats wa_chats_contact_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_chats
    ADD CONSTRAINT wa_chats_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES xchats.wa_contacts(id) ON DELETE RESTRICT;


--
-- Name: wa_contacts wa_contacts_account_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_contacts
    ADD CONSTRAINT wa_contacts_account_id_fkey FOREIGN KEY (account_id) REFERENCES xchats.wa_accounts(id) ON DELETE RESTRICT;


--
-- Name: wa_messages wa_messages_account_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_messages
    ADD CONSTRAINT wa_messages_account_id_fkey FOREIGN KEY (account_id) REFERENCES xchats.wa_accounts(id) ON DELETE RESTRICT;


--
-- Name: wa_messages wa_messages_chat_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_messages
    ADD CONSTRAINT wa_messages_chat_id_fkey FOREIGN KEY (chat_id) REFERENCES xchats.wa_chats(id) ON DELETE CASCADE;


--
-- Name: wa_messages wa_messages_sender_user_id_fkey; Type: FK CONSTRAINT; Schema: xchats; Owner: -
--

ALTER TABLE ONLY xchats.wa_messages
    ADD CONSTRAINT wa_messages_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES xchats.users(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--

\unrestrict W7lmeAOYnNM832HvwqU6660CXVEqi7zHhf2QqpIvg7O8963nP8T6VkhgZwoFq7f

