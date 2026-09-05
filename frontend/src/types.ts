export interface User {
  id: string
  email: string
  name: string
  role: string
  must_change_password: boolean
}
export interface Organization {
  id: string
  name: string
  // timezone is an IANA zone name — purely a display default for the
  // campaign quiet-hours window picker (lib/schedule.ts does the actual
  // local<->UTC conversion); nothing server-side computes with it.
  timezone?: string
}

// Page mirrors internal/httpapi.page — the generic list-pagination envelope
// (backend/internal/httpapi/server.go).
export interface Page<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

// --- Settings (Track 2) ---

// LLMSettings mirrors internal/settings.LLMSettings.
export interface LLMSettings {
  default_provider: string
  default_model: string
  vision_model: string
  // stt_provider is "" (not configured), "openai", or "groq" — see
  // internal/settings.LLMSettings.STTProvider's own doc comment for why no
  // other provider is valid here.
  stt_provider: string
  stt_model: string
  // stt_language is "auto", "kk", "ru", or "en".
  stt_language: string
  stt_vocabulary: string
  max_tokens: number
  temperature: number
  timeout_seconds: number
  retry: boolean
}
// ProviderSettings mirrors internal/settings.ProviderSettings.
export interface ProviderSettings {
  base_url?: string
  default_model?: string
  disabled: boolean
  last_verified_at?: string
  last_error?: string
}
// NgrokSettings mirrors internal/settings.NgrokSettings.
export interface NgrokSettings {
  region?: string
  domain?: string
}
// Settings mirrors internal/settings.Settings — GET /settings's payload.
export interface Settings {
  version: number
  llm: LLMSettings
  providers: Record<string, ProviderSettings>
  ngrok: NgrokSettings
  credential_file_fallback_accepted: boolean
}

// IntegrationField mirrors internal/httpapi.fieldSummary.
export interface IntegrationField {
  key: string
  label: string
}
// IntegrationSummary mirrors internal/httpapi.integrationSummary — one entry
// of GET /settings/integrations' payload.providers.
export interface IntegrationSummary {
  id: string
  display_name: string
  credential_url: string
  docs_url: string
  has_base_url: boolean
  has_model: boolean
  fields: IntegrationField[]
  validatable: boolean
  configured: boolean
  source?: string // "env" | "keyring" | "file"
  base_url?: string
  default_model?: string
  disabled: boolean
  last_verified_at?: string
  last_error?: string
}

// TunnelStatus mirrors internal/tunnel.Status.
export interface TunnelStatus {
  running: boolean
  public_url?: string
  started_at?: string
  last_error?: string
}

// ProviderHealthStatus mirrors internal/providerhealth.Status — one
// provider's live, in-production health (GET /settings/provider-health,
// and the "integration.status_changed" realtime event carry the same shape).
export interface ProviderHealthStatus {
  provider: string
  healthy: boolean
  at: string
}

// UpdateCheckResult mirrors internal/updatecheck.Result — GET /settings/
// update-check's payload.
export interface UpdateCheckResult {
  current_version: string
  latest_version?: string
  update_available: boolean
  release_url?: string
  checked_at?: string
}
export interface Contact {
  id: string
  display_name: string
  phone_number: string
  phone_jid: string
  lid_jid: string
  push_name: string
  attributes?: Record<string, unknown>
}
// ChannelName is every channel the backend can report on an account/chat.
// whatsapp is the unofficial whatsmeow (QR-paired) leg; whatsapp_cloud is
// the official Meta WhatsApp Cloud API — the two run side by side and are
// deliberately distinct values, never merged.
export type ChannelName =
  | 'whatsapp'
  | 'simulator'
  | 'telegram'
  | 'instagram'
  | 'messenger'
  | 'whatsapp_cloud'

// ConnectableChannel is every channel AddAccountDialog's picker can start a
// NEW connect flow for — ChannelName minus 'simulator' (nothing user-facing
// ever connects that leg directly). Shared by AddAccountDialog's own
// startChannel prop and Accounts.vue's addStartChannel state so a guided
// Channel setup run (stores/channelSetup.ts) can hand back exactly which
// channel to resume.
export type ConnectableChannel = 'whatsapp' | 'telegram' | 'whatsapp_cloud' | 'instagram' | 'messenger'

export interface Chat {
  id: string
  channel: ChannelName
  account_id: string
  /** @deprecated alias for account_id, emitted only for wa_* channels. */
  wa_account_id?: string
  contact: Contact
  status: string
  assignee_user_id: string | null
  unread_count: number
  last_message_at: string | null
  last_message_preview: string
  /** The CRM customer this conversation belongs to; null for a chat on an
   *  unassigned account and for chats that predate the CRM layer. */
  customer_id: string | null
}
export interface Media {
  id: string
  url: string
  media_type: string
  mimetype: string
  file_name: string
  file_size: number
  // download_status is "pending" | "ready" | "failed".
  download_status: string
  // transcript is a "ready" audio attachment's speech-to-text result, "" if
  // not yet transcribed (or not audio, or STT is not configured).
  transcript: string
}
export interface Message {
  id: string
  chat_id: string
  direction: 'in' | 'out'
  sender_type: string
  external_message_id: string
  message_type: string
  content: string
  media: Media[]
  status: string
  source: string
  timestamp: string | null
}
export interface WhatsAppAccount {
  id: string
  display_name: string
  connection_status: string // connected | disconnected | logged_out
  assigned: boolean
  owner_jid: string
  phone_number: string
  last_live_event_at: string | null
  created_at: string | null
}

// AutomationMode mirrors automation.Mode (backend/automation) — the closed
// three-value channel automation mode.
export type AutomationMode = 'off' | 'suggestions' | 'scheduled_auto'

// ScheduleWindow mirrors dto.ScheduleWindow — one recurring UTC weekday/
// time-of-day range. weekday is 0=Sunday..6=Saturday (UTC), matching
// JavaScript's own Date.getDay()/getUTCDay() numbering, so no translation
// table is needed anywhere this is used. end_minute <= start_minute means
// the window wraps past UTC midnight into (weekday+1)%7 — see
// frontend/src/lib/schedule.ts, which is the only place that interprets
// (rather than just stores/transports) this shape.
export interface ScheduleWindow {
  weekday: number
  start_minute: number
  end_minute: number
}

// AccountAutomation mirrors dto.AccountAutomation — one channel's effective
// (resolved) automation settings, embedded in Account. wait_seconds is
// always the RESOLVED value (override ?? default); wait_seconds_override is
// the raw stored value, null when the channel uses the system default.
export interface AccountAutomation {
  mode: AutomationMode
  wait_seconds: number
  wait_seconds_override: number | null
  default_wait_seconds: number
  schedule: ScheduleWindow[]
}

// AccountAutomationPatch is PUT /accounts/:id/automation's request body —
// always a full replace (mode + override + the complete schedule), never a
// partial patch: the dialog always saves the whole settings set at once.
export interface AccountAutomationPatch {
  mode: AutomationMode
  wait_seconds_override: number | null
  schedule: ScheduleWindow[]
}

// Account is the channel-neutral shape GET /accounts returns — one list for
// every channel. external_handle is the phone number for WhatsApp and
// "@botname" for Telegram; the webhook_* fields are Telegram health and stay
// null everywhere else.
export interface Account {
  id: string
  channel: ChannelName
  display_name: string
  external_handle: string
  // whatsapp: connected | disconnected | logged_out
  // telegram: connecting | connected | webhook_error | token_error
  //         | disconnect_pending | disconnect_error | disconnected
  connection_state: string
  assigned: boolean
  last_live_event_at: string | null
  created_at: string | null
  webhook_url: string | null
  webhook_registered_at: string | null
  webhook_last_checked_at: string | null
  webhook_last_error: string | null
  automation: AccountAutomation
}

// TelegramAccountResponse is what the /telegram-accounts lifecycle routes
// return: the refreshed account plus the state the call just produced.
export interface TelegramAccountResponse {
  account: Account
  connection_state: string
  pending_update_count?: number
  expected_webhook_url?: string
}
// WhatsAppCloudPhoneNumber is one entry in POST /whatsapp-cloud-accounts/
// discover's response — the operator picks one before the connect step
// (which spends one of Meta's limited PIN attempts) ever runs.
export interface WhatsAppCloudPhoneNumber {
  id: string
  display_phone_number: string
  verified_name: string
  quality_rating: string
}
export interface WhatsAppCloudDiscoverResponse {
  phone_numbers: WhatsAppCloudPhoneNumber[]
}
// WhatsAppCloudAccountResponse is what POST /whatsapp-cloud-accounts
// returns — the same {account, connection_state} shape as
// TelegramAccountResponse below.
export interface WhatsAppCloudAccountResponse {
  account: Account
  connection_state: string
}

// InstagramOAuthStartResponse is POST /instagram-accounts/oauth/start's
// response — a URL to redirect the browser's TOP-LEVEL navigation to (never
// fetched), and the actual connect happens entirely server-side once Meta
// redirects back (see backend/internal/httpapi/meta_oauth.go).
export interface InstagramOAuthStartResponse {
  authorize_url: string
}

// MessengerOAuthStartResponse is POST /messenger-accounts/oauth/start's
// response — same shape and same redirect-then-server-side-connect flow as
// InstagramOAuthStartResponse above (see
// backend/internal/httpapi/meta_oauth_messenger.go).
export interface MessengerOAuthStartResponse {
  authorize_url: string
}

// SetupKey is one channel-setup prerequisite's id — see
// backend/internal/httpapi/channel_setup.go's SetupKey* constants.
// public_access/meta_app are one-time, install-wide prerequisites;
// instagram/messenger/whatsapp_cloud are the three Meta channels that
// depend on them.
export type SetupKey = 'public_access' | 'meta_app' | 'instagram' | 'messenger' | 'whatsapp_cloud'

// MetaDashboardField names the literal Dashboard box one copy-paste value
// belongs in (App Settings → Basic → App Domains, and so on).
export interface MetaDashboardField {
  field: string
  where: string
  value: string
}

// ChannelSetupEntry is one prerequisite's status — GET/PUT /channel-setup's
// per-entry shape. key/status go to every caller; every field below is
// present ONLY in an admin's response, and even then only on the three Meta
// CHANNEL entries (see backend/internal/httpapi/channel_setup.go's
// ChannelSetupEntry doc comment) — absent, not just empty, for a member.
export interface ChannelSetupEntry {
  key: SetupKey
  status: 'ready' | 'not_configured' | 'setup_required'
  webhook_callback?: string
  redirect_uri?: string
  scopes?: string[]
  subscribe_fields?: string
  dashboard_path?: string
  dashboard_fields?: MetaDashboardField[]
}

// MetaStaleAccount is one connected Instagram/Messenger account whose
// registered webhook origin no longer matches the current public base URL —
// WhatsApp Cloud self-heals and never appears here.
export interface MetaStaleAccount {
  account_id: string
  channel: string
  display_name: string
  detail: string
}

// AdminContact is one workspace administrator's name/email — present only
// for a member caller who just hit a missing prerequisite (see
// ChannelSetupInfo.admin_contacts), so they have someone to ask instead of
// a dead end.
export interface AdminContact {
  name: string
  email: string
}

// ChannelSetupInfo is GET (and both PUT save endpoints') /channel-setup
// response. can_configure/public_base_url/entries go to every authenticated
// caller; everything from verify_token down is admin-only and simply absent
// for a member — see ChannelSetupTab.vue. admin_contacts is the mirror
// image: present only for a member.
export interface ChannelSetupInfo {
  can_configure: boolean
  public_base_url: string
  entries: ChannelSetupEntry[]
  verify_token?: string
  graph_api_version?: string
  dashboard_url?: string
  stale_accounts?: MetaStaleAccount[]
  admin_contacts?: AdminContact[]
}

// WaPairSession is POST /wa-accounts/pair's response: a session id to poll
// via GET /wa-accounts/pair/:session_id.
export interface WaPairSession {
  session_id: string
  status: string // qr_required
}
// WaPairStatus is one poll's result. account_id is only set once status is
// "connected"; qr_code/qr_base64 refresh on every new code whatsmeow issues.
export interface WaPairStatus {
  status: string // qr_required | connected | timeout | error
  qr_code?: string | null
  qr_base64?: string | null
  account_id?: string
  message?: string
}
export interface AiDraft {
  id: string
  chat_id: string
  trigger_message_id: string | null
  ordinal: number
  draft_text: string
  context_status: string
  confidence: number | null
  escalate: boolean
  escalation_reason: string
  status: string
  created_at: string
}

// --- Knowledge Base draft + live views ---
// An entity is either LIVE (a row in a live ai_ table) or «Черновик» — present
// in the kbd_draft blob, flagged `draft: true` here. There is no more
// review_state/proposed-approved-rejected: a pending entity IS the "Правки" set.

export interface DraftConfig {
  organization_id: string
  persona: string
  mission: string
  guardrails: string
  language_policy: string
  reply_max_words: number
  draft: boolean
  base_version: number
  updated_at: string
}
export interface TopicRow {
  id: string // = slug (blob entries carry no DB row id)
  slug: string
  title: string
  body_md: string
  featured_image: string | null
  illustration_images: string[]
  explainer_videos: string[]
  reference_documents: string[]
  draft: boolean
  updated_at: string
}
// AdditionalFact mirrors aiprompt.AdditionalFact — one seller-authored
// "virtual fact column" on a product, tariff, or the tariff_info singleton:
// {ref, value, instruction}. value is the exact fact the customer prompt
// keeps hidden behind a {{table.ref.fact_ref}} token, substituted only
// alongside `instruction`'s safe-phrasing guidance (aiprompt/facts.go) —
// never rendered verbatim into the prompt itself. ref is lowercase
// snake_case, unique within the record, and must not collide with one of
// the record's own concrete field names (e.g. "price").
export interface AdditionalFact {
  ref: string
  value: number | boolean | string
  instruction: string
}
// Facts lane — typed entity rows. Every exact fact (price, limit, phone, …) is a
// CONCRETE COLUMN here, never a generic key→value; the brain quotes it in replies
// as a {{table.slug.field}} token. `id` = the row's natural key (ref, or the
// fixed contact/policy singleton slug) since blob entries carry no DB row id.
export interface TariffRow {
  id: string // = ref
  ref: string
  name: string
  price: string
  limit_text: string
  fee: string
  summary: string
  pricing_type: string // fixed | percentage | tiered
  advantages: string
  disadvantages: string
  best_for: string
  not_for: string
  additional_facts: AdditionalFact[]
  sales_status: string // active | inactive
  featured_image: string | null
  pricing_images: string[]
  explainer_videos: string[]
  terms_documents: string[]
  draft: boolean
  updated_at: string
}
export interface ProductRow {
  id: string // = ref
  ref: string
  name: string
  price: string
  description: string
  category: string
  brand: string
  advantages: string
  disadvantages: string
  best_for: string
  not_for: string
  availability_status: string // in_stock | preorder | on_demand | unavailable
  availability_note: string
  installation_terms: string
  warranty_terms: string
  additional_facts: AdditionalFact[]
  sales_status: string // active | inactive
  featured_image: string | null
  gallery_images: string[]
  demo_videos: string[]
  certificate_documents: string[]
  guarantee_documents: string[]
  draft: boolean
  updated_at: string
}
export interface ContactRow {
  id: string // = the 'support' singleton slug — one row per org
  slug: string
  whatsapp: string
  email: string
  address: string
  legal_information: string
  callback_time: string
  working_hours: string
  phone: string
  website: string
  instagram: string
  contact_card_image: string | null
  location_map_image: string | null
  company_legal_documents: string[]
  draft: boolean
  updated_at: string
}
// PolicyRow — org commerce-policy scalars (delivery/payment/returns terms), a
// structural clone of ContactRow (id = the 'main' singleton slug).
export interface PolicyRow {
  id: string
  slug: string
  delivery_cost: string
  delivery_in_days: string
  free_delivery_from: string
  min_order: string
  prepayment: string
  installment: string
  return_period_in_days: string
  warranty: string
  outside_zones_note: string
  commerce_policy_documents: string[]
  draft: boolean
  updated_at: string
}
// TariffInfoRow — the ai_tariff_info singleton (id/slug always "main"):
// organization-wide tariff facts not specific to any one tariff (e.g. a
// trial period shared by every plan). Carries no prose column at all, only
// additional_facts — a structural clone of ContactRow/PolicyRow otherwise.
export interface TariffInfoRow {
  id: string
  slug: string
  additional_facts: AdditionalFact[]
  draft: boolean
  updated_at: string
}
// DeliveryZoneRow — an ai_delivery_zones row («Зоны доставки»), live overlaid
// by a pending kbd_draft entry — the same live+blob merge as every other
// entity kind (see kbstore.mergedView's zones section).
export interface DeliveryZoneRow {
  id: string // = ref
  ref: string
  name: string
  zone_level: string // city | region | country
  parent_ref: string
  delivery_available: boolean
  delivery_cost: string
  delivery_in_days: string
  notes: string
  sales_status: string // active | inactive
  draft: boolean
  updated_at: string
}
export interface KbMaterial {
  id: string
  source_type: string
  source_ref: string
  blob_id: string
  extracted_text: string
  media_kind: string
  status: string // ready | pending | failed
  extraction: string
  created_at: string
  updated_at: string
  // The fields below belong to the upload-material lineage (MCP kb_media_upload
  // → PUT /mcp/uploads/:id) — empty/zero for a material still on the legacy
  // text/url path above (blob_id/extracted_text/etc.).
  filename: string
  mime_type: string
  size_bytes: number
  processing_status: string // uploaded | parsed (see kbstore.CompleteMaterialUpload)
  customer_visibility: string
  visual_summary: string
  transcript_text: string
  operator_note: string
  // has_content reports whether bytes are actually retrievable at
  // GET /kb/materials/:id/content — false while an upload is still in
  // flight, or for a legacy material that never had bytes at all.
  has_content: boolean
}
export interface KbRequest {
  id: string
  material_id: string | null
  req_type: string // confirm_fact | describe_file | comment
  prompt: string
  context: string // JSON string, e.g. {"suggested":"5000 ₸"}
  target: string // JSON string, e.g. {table,slug,field} | {material_id}
  state: string // pending | resolved | dismissed
  resolution: string | null
  created_at: string
  resolved_at: string | null
}
export interface DraftView {
  config: DraftConfig
  topics: TopicRow[]
  tariffs: TariffRow[]
  products: ProductRow[]
  contacts: ContactRow[]
  policies: PolicyRow[]
  tariff_info: TariffInfoRow[]
  zones: DeliveryZoneRow[]
  materials: KbMaterial[]
  requests: KbRequest[]
}

// DraftConfigPatch mirrors kbstore.DraftConfigPatch — an omitted field has no
// pending edit at all (as opposed to DraftConfig above, which always carries
// a full, possibly-live-only value). Every field is optional for exactly
// that reason: `omitempty` on the wire means an untouched field is simply
// absent from the JSON, not present-and-null.
export interface DraftConfigPatch {
  persona?: string
  mission?: string
  guardrails?: string
  language_policy?: string
  reply_max_words?: number
}

// DraftChangeSet mirrors kbstore.DraftChangeSet — the Черновик review
// payload behind GET /playground/draft. Unlike DraftView it carries ONLY
// what kbd_draft has staged, plus explicit deletion entries: an unchanged
// published row can never appear here. The published counterpart a reviewer
// diffs a pending row against comes from GET /kb (DraftView), never from
// this payload.
export interface DraftChangeSet {
  base_version: number
  updated_at: string
  config: DraftConfigPatch | null // null = no pending config edit at all
  topics: TopicRow[]
  tariffs: TariffRow[]
  products: ProductRow[]
  contacts: ContactRow[]
  policies: PolicyRow[]
  tariff_info: TariffInfoRow[]
  zones: DeliveryZoneRow[]
  deletes: DraftChangeDelete[]
}

// DraftChangeDelete is one staged removal, addressed in the SAME plural
// vocabulary POST /playground/draft/approve/:kind/:id and DELETE
// /playground/draft/changes/:kind/:key use.
export interface DraftChangeDelete {
  kind: string // topics|tariffs|products|contacts|policies|tariff_info|delivery_zones
  key: string
}

// CancelChangeResponse mirrors handlePlaygroundCancelChange's response body.
export interface CancelChangeResponse {
  changed: boolean
  changes: DraftChangeSet
}

// KbGateReason mirrors kbstore.GateReason — one deterministic approve-gate
// violation, carried in a 422's payload.reasons alongside the existing
// flat message (KB-09). kind matches ChangeKind (draftChanges.ts) when the
// violation names a single entity; both kind and key are "" for one that
// doesn't (an unresolved-request count spans the whole org, not one row).
export interface KbGateReason {
  kind: string
  key: string
  message: string
}

// --- Structured KB import pipeline (internal/kbimport) — submit a URL/file,
// pass 1 extracts it, pass 2 synthesizes the accumulated evidence into typed
// KB draft records (kbd_draft only — see DraftChangeSet above; an import
// never touches DraftView/live directly, same rule as every other authoring
// path). POST/GET /kb/imports.

// KbImportRunStatus mirrors kbimport.deriveRunStatus's closed vocabulary —
// branched on by KbImportRunStatus.vue (badge colour) and the store (when to
// stop polling), so this is a real union rather than string+comment.
export type KbImportRunStatus = 'extracting' | 'synthesizing' | 'built' | 'failed' | 'needs_human' | 'cancelled'

// KbImportMaterialStatus mirrors kbimport.MaterialStatus — one submitted
// URL/file's pass-1 progress within a run. processing_status mirrors
// kbd_materials' own lifecycle (kbstore/import.go's doc comment):
// queued -> extracting -> parsed | needs_human | failed | cancelled.
export interface KbImportMaterialStatus {
  id: string
  kind: 'url' | 'file'
  label: string
  handle: string
  processing_status: 'queued' | 'extracting' | 'parsed' | 'needs_human' | 'failed' | 'cancelled'
  error?: string
}

// KbImportAppliedUpsert / KbImportDroppedUpsert mirror kbstore.AppliedUpsert /
// kbstore.DroppedUpsert — pass 2's per-call synthesis outcome (one MCP
// typed-upsert call each; dropped ones never reached kbd_draft at all).
export interface KbImportAppliedUpsert {
  tool: string
  type: string
  key: string
  created: boolean
}
export interface KbImportDroppedUpsert {
  tool: string
  reason: string
}
// KbImportTokenUsage mirrors kbstore.TokenUsage.
export interface KbImportTokenUsage {
  prompt_tokens: number
  completion_tokens: number
}
// KbImportSynthesisSummary mirrors kbimport.SynthesisSummary — present once
// pass 2 has started (RunSummary.synthesis is omitted until then).
export interface KbImportSynthesisSummary {
  status: string // running | built | failed | needs_human
  notes?: string
  applied?: KbImportAppliedUpsert[]
  dropped?: KbImportDroppedUpsert[]
  usage: KbImportTokenUsage
}
// KbImportRun mirrors kbimport.RunSummary — POST/GET /kb/imports' payload.
export interface KbImportRun {
  run_id: string
  status: KbImportRunStatus
  // started_by is a raw user id — the org's own already-loaded user list
  // (useInbox().users, id -> name) resolves it to a display name; the API
  // does not (see RunSummary.StartedBy's own doc comment).
  started_by: string
  started_at: string
  // cancelable is true only while pass 1 is still running — see
  // kbstore.CancelImportRun's own doc comment for why cancelling is
  // refused once synthesis has been claimed.
  cancelable: boolean
  // finished_at is set once the run reaches a terminal status (built,
  // failed, needs_human, cancelled) — absent while still active, mirroring
  // synthesis' own omitted-until-started shape rather than a zero instant.
  finished_at?: string
  materials: KbImportMaterialStatus[]
  synthesis?: KbImportSynthesisSummary
}

// KbImportRunPage mirrors GET /kb/imports' response shape — KB-14's history
// list, ?limit=&offset= paginated. total is the org's full run count (not
// just runs.length), so a page beyond the last still reports how many
// pages exist.
export interface KbImportRunPage {
  runs: KbImportRun[]
  total: number
}

// KbImportProviderFamily mirrors extractor.Family — the source kinds
// Capabilities() probes each provider with (backend/internal/extractor/
// capabilities.go). A real union: useImportProviders branches on these
// values to map a staged File.type to a family.
export type KbImportProviderFamily = 'url' | 'text' | 'docx' | 'pdf' | 'image'

// KbImportProviderCapability mirrors httpapi.kbImportProviderSummary — one
// entry of GET /kb/import/providers' payload.providers: what a provider
// reads (families, data-derived from its real Supports() method) and
// whether it is currently usable (requires_credential/configured), so the
// parser dropdown can never disagree with Submit's own precheck.
export interface KbImportProviderCapability {
  id: string
  display_name: string
  families: KbImportProviderFamily[]
  requires_credential: boolean
  configured: boolean
}

// McpConnectionInfo mirrors GET /mcp-connection (backend/internal/httpapi/mcp_info.go)
// — the session-level (non-admin) view of what URL to paste into a ChatGPT or
// Claude MCP connector. public_url/tunnel_running are only meaningful when
// tunnel_available is true.
export interface McpConnectionInfo {
  mcp_url: string
  auth_enabled: boolean
  tunnel_available: boolean
  tunnel_running: boolean
  public_url?: string
  scopes: string[]
}

// PromptView mirrors GET /kb/prompt (backend/internal/httpapi/kb_prompt.go) —
// the rendered prompt the response engine would send right now, plus enough
// metadata to power the Промпт tab's «О промпте» sidebar without a second
// round trip.
export interface PromptSectionCounts {
  topics: number
  products: number
  tariffs: number
  zones: number
  contacts: number
  policies: number
}
export interface PromptView {
  prompt_ref: string
  rendered_text: string
  frame_text: string
  char_count: number
  approx_tokens: number
  built_at: string
  status: 'ok' | 'error'
  error?: string
  section_counts: PromptSectionCounts
}

// KbGapReasonCode/KbGapEntityType mirror aiprompt's closed vocabularies
// (backend/aiprompt/kbgap.go) — see KbGapReport below. There is deliberately
// no "unavailable_entity": see the reason-code const block's doc comment in
// kbgap.go for why no reachable, non-conflicting trigger for it exists.
export type KbGapReasonCode =
  | 'missing_entity'
  | 'missing_field'
  | 'ambiguous_entity'
  | 'conflicting_kb_data'
  | 'unsupported_request'
  | 'human_requested'
  | 'engine_error'
  | 'other'
export type KbGapEntityType = 'product' | 'tariff' | 'tariff_info' | 'contact' | 'policy' | 'delivery_zone' | 'topic'
export type KbGapSource = 'model' | 'engine'

// KbGapEvent mirrors one httpapi.kbGapEventView row from GET /kb/gaps
// (backend/internal/httpapi/kb_gaps.go) — deliberately never the customer-
// facing draft/message text, only diagnostic metadata.
export interface KbGapEvent {
  id: string
  channel: string
  chat_id: string
  draft_id?: string
  reason_code: KbGapReasonCode
  target_entity_type?: KbGapEntityType
  target_entity_ref?: string
  missing_fields?: string[]
  escalation_reason?: string
  source: KbGapSource
  created_at: string
}
export interface KbGapReasonCount {
  reason_code: KbGapReasonCode
  count: number
}
// KbGapEntityCount/KbGapFieldCount rank which specific entity/field drives
// the counts above — most-frequent first, bounded server-side — answering
// "which product/field causes the most escalations" that counts/recent
// alone cannot once matching events exceed recent's own bounded page.
export interface KbGapEntityCount {
  target_entity_type: KbGapEntityType
  target_entity_ref: string
  count: number
}
export interface KbGapFieldCount {
  target_entity_type: KbGapEntityType
  field_name: string
  count: number
}
// KbGapReport mirrors GET /kb/gaps' payload: counts is the default report
// (genuine content gaps only, zero-filled); operational_counts keeps
// unsupported_request/human_requested/engine_error/other distinguishable
// rather than blended into counts; recent is a bounded, newest-first page.
export interface KbGapReport {
  counts: KbGapReasonCount[]
  operational_counts: KbGapReasonCount[]
  top_target_entities: KbGapEntityCount[]
  top_missing_fields: KbGapFieldCount[]
  recent: KbGapEvent[]
}
// KbGapFilter is GET /kb/gaps' query-param shape (all optional).
export interface KbGapFilter {
  reason?: string
  entity_type?: string
  entity_ref?: string
  from?: string
  to?: string
  limit?: number
}

// --- Eval comparison UI — mirrors evals/harness/viewmodel.go + export.go +
// internal/provenance/launch.go's JSON shapes EXACTLY (field-for-field, same json
// tags). Fetched as plain static files from /evals-data/ (frontend/nginx.conf),
// never through the backend's envelope-wrapped API — see api/evals.ts.

export type ScoreStatus = 'pass' | 'fail' | 'not_run' | 'error'

export interface VScore {
  name: string
  status: ScoreStatus
  detail?: string
}
export interface VRollup {
  key: string
  label: string
  pass: boolean
}
// The Requirements-panel projection of scores/rollups above — same pass/fail, never a
// re-grade, just Expected/Actual display strings. pass is null (not omitted) when a row
// is not applicable to this test (e.g. no escalate: expectation) rather than a real fail.
export interface VContractRow {
  key: string
  label: string
  kind: 'requirement' | 'safety'
  expected?: string
  actual?: string
  pass?: boolean | null
}
export interface HistoryTurn {
  role: 'client' | 'assistant'
  text: string
}
export interface VSubject {
  scenario?: string
  test_id?: string
  message?: string
  history?: HistoryTurn[]
  case_id?: string
  input_ref?: string
}
export interface PromptRef {
  name: string
  version: number
  sha256?: string
}
export interface VVariant {
  model: string
  setup?: string
  experiment?: string
  prompt?: PromptRef
  preprocessor?: string
}
export interface VOutput {
  raw?: string
  parse_ok: boolean
  reply_text?: string
  parse_error?: string
  error?: string
  raw_has_reasoning_markers?: boolean
  reasoning?: string
}
export interface VCost {
  tokens_in: number
  tokens_out: number
  estimate_usd: number
  basis: string // "measured_split" | "cached_replay_borrowed" | "cached_replay_unpriceable" | "unknown_pricing"
}
export interface VScenarioDetails {
  injected_text?: string
  unknown_tokens?: string[]
  unknown_media?: string[]
  invented_digits?: string[]
  unit_issues?: string[]
  forbidden_phrase?: string
  blocked: boolean
  leftover_braces: boolean
  finish_reason?: string
  truncated: boolean
  reasoning_leak: boolean
  // media_count_evaluated is a CODE-VERSION marker, not derived from parse success — a
  // verdict judged before this check existed omits it entirely (undefined), and must be
  // rendered as "not checked", never a fabricated pass just because too_many_media's
  // default is false. See judge.go's Verdict.MediaCountEvaluated doc comment.
  media_count?: number
  too_many_media?: boolean
  media_count_evaluated?: boolean
}
export interface VExtractDetails {
  content_kind?: string
  summary?: string
  extracted_text?: string
  language?: string
  visibility_suggestion?: string
  media_role_hint?: string
  relates_to_hint?: string
}
export interface VExecution {
  family: 'scenario' | 'extract'
  subject: VSubject
  variant: VVariant
  output: VOutput
  scores: VScore[] | null
  rollups: VRollup[]
  contract?: VContractRow[]
  cost: VCost
  latency_ms?: number
  // retries mirrors the harness's Verdict.Retries (evals/harness/judge.go) — how many
  // times the retry mechanism (evals/harness/retry.go) retried this row. 0/undefined
  // for every execution the retry path never touched.
  retries?: number
  scenario?: VScenarioDetails
  extract?: VExtractDetails
}
export interface ExecutionsFile {
  schema_version: number
  run_id: string
  launch_id?: string
  generated_at: string
  git_sha?: string
  executions: VExecution[]
}

export interface RunSummary {
  run_id: string
  launch_id?: string
  has_manifest: boolean
  family: string // "scenario" | "extract" | "mixed" | "unknown"
  models: string[]
  prompts: string[]
  started_at?: string
  finished_at?: string // empty = interrupted/still running, never guessed
  scenario_total: number
  scenario_behavior_pass: number
  scenario_contract_pass: number
  extract_total: number
  extract_checks_pass: number
  has_index_html: boolean
  load_error?: string
}
export interface RunsFile {
  schema_version: number
  generated_at: string
  runs: RunSummary[]
}

export type LaunchMemberStatus = 'pending' | 'running' | 'complete' | 'failed'
export type LaunchStatus = 'running' | 'complete' | 'failed' | 'partial'
export interface LaunchCallCount {
  scenario: number
  extract: number
  total: number
}
export interface LaunchMember {
  family: string // "scenario" | "extract"
  run_id?: string
  status: LaunchMemberStatus
  error?: string
}
export interface LaunchManifest {
  schema_version: number
  launch_id: string
  status: LaunchStatus
  planned_families: string[]
  expected_calls: LaunchCallCount
  members: LaunchMember[]
  started_at: string
  finished_at?: string
}

// --- Requirements catalog (runs/catalog.json, evals/harness/catalog.go) ---
// The ONE repository-state (not run-snapshot) export: every scenario's and extraction
// case's requirements, resolved to real current values, for review BEFORE any billed
// run. See evalCatalog.ts for the display logic built on these types.
export interface CatalogFact {
  token: string
  value: string
}
export interface CatalogMedia {
  name: string
  kind: string
  description: string
}
export interface CatalogMediaExpect {
  any_of?: string[]
  all_of?: string[]
  // forbid means this test's reply must attach NO media at all — the opposite
  // expectation from any_of/all_of (mutually exclusive, enforced at render
  // time). Omitted (undefined) when false.
  forbid?: boolean
  // exclusive narrows any_of/all_of from "attach at least one/all of these" to
  // "attach these, and nothing else" — a modifier on the SAME declaration, not a
  // separate allowed-list field. Requires a non-empty any_of and is mutually exclusive
  // with forbid (both enforced at render time). Omitted when false.
  exclusive?: boolean
}
// One alternative acceptable-behavior block from a test's `outcomes:` list — the same
// per-test knobs a CatalogTestCase itself has (label is the block's display name; the
// test passes the outcomes gate when ANY one block's declared checks all hold).
export interface CatalogOutcomeCase {
  label: string
  requires?: string[][]
  media?: CatalogMediaExpect
  escalate?: boolean
  language?: string
  must_not_contain?: string[]
  must_contain_any?: string[]
}
// One binary semantic claim judged by a pinned cheap model — an OPTIONAL dimension,
// only evaluated when the separate `judge-llm` command runs (never the plain `judge`).
export interface CatalogLLMCheck {
  claim: string
  expect: boolean
}
// escalate is `boolean | undefined` on the wire (omitempty on a Go *bool omits only
// when nil) — undefined means "not checked by this test", false is an ACTIVE
// "must not escalate" requirement. Never collapse the two.
export interface CatalogTestCase {
  id: string
  message: string
  history?: HistoryTurn[]
  requires?: string[][]
  language?: string
  escalate?: boolean
  must_not_contain?: string[]
  must_contain_any?: string[]
  forbid_tokens?: string[]
  media?: CatalogMediaExpect
  outcomes?: CatalogOutcomeCase[]
  // stock_check names a product ref; judge-llm-only (see CatalogLLMCheck).
  stock_check?: string
  llm_checks?: CatalogLLMCheck[]
  source: string
}
export interface CatalogScenario {
  name: string
  description?: string
  setup?: string
  prompt_ref?: string
  experiment?: string
  // archived/archived_reason/pipeline/tests_path are optional: a stale v2 catalog.json
  // (schema_version < 3, or generated before the 2026-07 archival mechanism) simply
  // omits them — never a parse error, just a display fallback (see scenarioNavLabel).
  archived?: boolean
  archived_reason?: string
  pipeline?: string
  tests_path?: string
  facts_source: string
  facts: CatalogFact[]
  media_tokens?: string[]
  media?: CatalogMedia[]
  tests: CatalogTestCase[]
}
export interface CatalogExtractCase {
  id: string
  image: string
  fields?: Record<string, string>
  text_contains_all?: string[]
  identify_contains_all?: string[]
  identify_contains_any?: string[]
  allowed_numbers?: string[]
  required_numbers?: string[]
  forbid_currency?: boolean
  source: string
}
export interface CatalogFile {
  schema_version: number
  generated_at: string
  scenarios: CatalogScenario[]
  extract_cases: CatalogExtractCase[]
}

// --- Campaigns (backend/campaign, internal/campaign, internal/httpapi/campaigns.go) ---
// Bulk outbound messaging against a pasted/uploaded recipient list,
// rate-limited per sending account. A campaign's own windows and an
// account's shared quiet-hours windows both use the SAME ScheduleWindow
// shape as channel automation above (weekday 0=Sunday..6=Saturday UTC) —
// lib/schedule.ts's utcToLocal/localToUtc apply unchanged to either.

export type CampaignStatus = 'draft' | 'scheduled' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
export type CampaignRecipientStatus = 'pending' | 'sending' | 'sent' | 'failed' | 'skipped'

// Campaign mirrors dto.Campaign. min_interval_seconds/jitter_seconds are
// null when the campaign has no pace override of its own (inherits the
// sending account's shared pace); windows narrow the account's own
// quiet-hours windows further, never widen them.
export interface Campaign {
  id: string
  name: string
  account_id: string
  channel: ChannelName
  status: CampaignStatus
  message_body: string
  variables: string[]
  min_interval_seconds: number | null
  jitter_seconds: number | null
  windows: ScheduleWindow[]
  schedule_at: string | null
  started_at: string | null
  created_by: string
  created_at: string
  updated_at: string
  // recipient_counts keys by CampaignRecipientStatus — never null (an empty
  // object for a brand new draft), the list/detail views' progress bars.
  recipient_counts: Record<string, number>
}

// CampaignPatch is PATCH /campaigns/:id's body — an OMITTED key leaves that
// field untouched; min_interval_seconds/jitter_seconds must be given
// together (both a number, or both explicit null to clear the override
// back to inheriting the account's pace) or the request is rejected.
// JSON.stringify already gives exactly this wire shape: an `undefined`
// value is dropped from the object, an explicit `null` is kept.
export interface CampaignPatch {
  name?: string
  message_body?: string
  account_id?: string
  min_interval_seconds?: number | null
  jitter_seconds?: number | null
  schedule_at?: string | null
  windows?: ScheduleWindow[]
}

// CampaignRecipient mirrors dto.CampaignRecipient — one persisted recipient
// row.
export interface CampaignRecipient {
  id: string
  campaign_id: string
  normalized_identity: string
  raw_input: string
  name: string
  attributes?: Record<string, string>
  status: CampaignRecipientStatus
  failure_reason: string
  attempts: number
  next_attempt_at: string | null
  chat_id: string | null
  message_id: string | null
  created_at: string
  updated_at: string
  // message_delivery_state is the linked message's own delivery_state
  // (queued/sent/delivered/read/failed) — finer-grained than status above,
  // which never moves past 'sent' once ANY successful delivery happens.
  // Empty when message_id is null.
  message_delivery_state: string
}

// CampaignEvent mirrors dto.CampaignEvent — one campaign timeline entry
// (started, paused, auto_paused, recipients_replaced, retried_failed, ...).
export interface CampaignEvent {
  id: string
  campaign_id: string
  event: string
  actor_user_id: string | null
  detail?: Record<string, unknown>
  created_at: string
}

// CampaignTemplate mirrors dto.CampaignTemplate — one reusable,
// organization-wide message template (CAM-14). Pure content: no account,
// channel, status, pace, or schedule of its own.
export interface CampaignTemplate {
  id: string
  name: string
  message_body: string
  variables: string[]
  is_archived: boolean
  created_by: string
  created_at: string
  updated_at: string
}

// CampaignTemplatePatch is PATCH /campaign-templates/:id's body — an
// omitted key leaves that field untouched.
export interface CampaignTemplatePatch {
  name?: string
  message_body?: string
}

// CampaignTier mirrors dto.CampaignTier — one simultaneous rolling-window
// cap. used is present only in a SendingBudget response, absent in a plain
// CampaignAccountSettings read (configuration only, no live usage).
export interface CampaignTier {
  window_seconds: number
  max_sends: number
  used?: number
}

// CampaignAccountSettings mirrors dto.CampaignAccountSettings — GET/PUT
// /accounts/:id/sending-limits' shape: an account's pace, manual pause
// switch, full tier set, and full quiet-hours window set, always saved as
// one complete replace (never a partial patch).
export interface CampaignAccountSettings {
  account_id: string
  limit_mode: 'default' | 'custom'
  min_interval_seconds: number
  jitter_seconds: number
  paused: boolean
  tiers: CampaignTier[]
  windows: ScheduleWindow[]
}

// SendingBudget mirrors dto.SendingBudget — GET /accounts/:id/sending-budget's
// LIVE read: the shared widget the Accounts page, the campaign wizard, and a
// campaign's own detail page all render from.
export interface SendingBudget {
  account_id: string
  min_interval_seconds: number
  jitter_seconds: number
  paused: boolean
  allowed: boolean
  throttled_by: number
  next_send_at: string
  tiers: CampaignTier[]
}

// CampaignRecipientPreviewRow mirrors dto.CampaignRecipientPreview — one
// row of a preview (or a post-save PUT .../recipients) response.
// normalized_identity is set only for 'valid'; reason is set for
// 'invalid'/'duplicate'.
export interface CampaignRecipientPreviewRow {
  raw: string
  name?: string
  attributes?: Record<string, string>
  normalized_identity?: string
  status: 'valid' | 'invalid' | 'duplicate'
  reason?: string
}

// CampaignRecipientPreviewResult mirrors dto.CampaignRecipientPreviewResult
// — POST /campaigns/:id/preview and PUT /campaigns/:id/recipients' shared
// response shape (the former never persists; the latter has just persisted
// every 'valid' row).
export interface CampaignRecipientPreviewResult {
  rows: CampaignRecipientPreviewRow[]
  total: number
  valid: number
  invalid: number
  duplicate: number
}

// CampaignStatusEvent/CampaignRecipientEvent mirror dto.CampaignStatusEvent/
// dto.CampaignRecipientEvent — the campaign.status_changed/
// campaign.recipient_updated SSE payloads. Deliberately ids + an enum
// status only, never a recipient's identity — see internal/campaign.
// Broadcaster's own doc comment (backend/internal/campaign/runner.go).
export interface CampaignStatusEvent {
  campaign_id: string
  status: CampaignStatus
}
export interface CampaignRecipientEvent {
  campaign_id: string
  recipient_id: string
  status: CampaignRecipientStatus
}

// --- Customer management (CRM) — mirrors internal/dto/crm.go -----------------

// CustomerStatus mirrors dto.CustomerStatus — one entry of the organization's
// configurable lifecycle. These are rows, not a fixed enum: an organization
// edits its own list, and the five seeded by migration 0013 are a starting
// point rather than a closed vocabulary.
export interface CustomerStatus {
  id: string
  slug: string
  name: string
  color: string
  position: number
  is_default: boolean
}

// CustomerTag mirrors dto.CustomerTag. Nothing is seeded — every tag an
// organization has, it created.
export interface CustomerTag {
  id: string
  slug: string
  name: string
  color: string
}

// CustomerIdentity mirrors dto.CustomerIdentity — one channel identity.
// external_id is the provider's own identity for the person (the phone JID for
// WhatsApp, the numeric user id for Telegram); the accounts themselves are
// never merged, only linked to one xchats customer.
export interface CustomerIdentity {
  id: string
  channel: ChannelName
  account_id: string
  contact_id: string
  external_id: string
  username: string
  phone: string
  display_name: string
}

// Customer mirrors dto.Customer. There is deliberately no notes field: the
// profile's note is the latest CustomerNote, carried on CustomerProfile.
export interface Customer {
  id: string
  display_name: string
  phone: string
  email: string
  avatar_url: string
  status_id: string | null
  status: CustomerStatus | null
  assignee_user_id: string | null
  tags: CustomerTag[]
  identities: CustomerIdentity[]
  custom_fields: Record<string, unknown>
  created_at: string
  updated_at: string
}

// CustomerNote mirrors dto.CustomerNote — an internal note. Never sent to the
// customer; no channel adapter reads them.
export interface CustomerNote {
  id: string
  customer_id: string
  author_user_id: string | null
  author_name: string
  body: string
  created_at: string
  updated_at: string
}

export type FollowupAction = 'call' | 'message' | 'meeting' | 'other'
export type FollowupState = 'open' | 'completed' | 'cancelled'

// Followup mirrors dto.Followup. due_at is the UTC instant the backend orders
// and buckets by; due_date/due_minute are the wall clock the manager typed, so
// the edit form round-trips without a timezone drift. due_minute is null for
// an all-day follow-up (the time is optional).
export interface Followup {
  id: string
  customer_id: string
  customer_name: string
  conversation_id: string | null
  channel: string
  due_at: string
  due_date: string
  due_minute: number | null
  action: FollowupAction
  note: string
  assignee_user_id: string | null
  assignee_name: string
  state: FollowupState
  completed_at: string | null
  created_at: string
}

// FollowupBuckets mirrors store.FollowupBuckets — the reminder view's header.
// Overdue keeps counting an item until it is completed or rescheduled.
export interface FollowupBuckets {
  today: number
  tomorrow: number
  this_week: number
  overdue: number
}

// CustomerProfile mirrors dto.CustomerProfile — the sidebar's single hydrate
// call: the customer plus what is always rendered next to them.
export interface CustomerProfile {
  customer: Customer
  latest_note: CustomerNote | null
  next_followup: Followup | null
  conversations: Chat[]
}

// TimelineEntry mirrors dto.TimelineEntry. `source` is "crm" for a recorded
// CRM event and "message" for conversation activity read live from the
// messages tables — the two populate different fields, so render by source
// rather than by sniffing for nulls.
export interface TimelineEntry {
  id: string
  source: 'crm' | 'message'
  kind: string
  actor_user_id: string | null
  summary: string
  detail?: Record<string, unknown>
  occurred_at: string
  channel?: string
  chat_id?: string
  direction?: 'in' | 'out'
  sender_kind?: string
  body?: string
}

// CustomFieldDef mirrors dto.CustomFieldDef — an org-defined extra profile
// field. Values live in Customer.custom_fields, keyed by `key`.
export interface CustomFieldDef {
  id: string
  key: string
  label: string
  field_type: 'text' | 'number' | 'date' | 'select'
  options: string[]
  position: number
}

// --- Knowledge Base chat assistant (/chat) ---
// Mirrors internal/chatkb and internal/chat. The backend computes every
// structured element below from the knowledge base itself and hands it over
// ready to render — nothing here is parsed out of the model's prose (see
// internal/chatkb/components.go).

// KbSource is the state a record was read from. The two are never merged.
export type KbSource = 'REAL_KB' | 'DRAFT_KB'

// KbRecordKind is the same plural entity vocabulary the KB editor pages use
// (see components/kb/kbEntities.ts's ENTITY_META), so a chat card can borrow
// their icon and localized entity name instead of inventing its own.
export type KbRecordKind = 'topics' | 'products' | 'tariffs' | 'delivery_zones' | 'contacts' | 'policies' | 'tariff_info' | 'config'

export interface KbRecordField {
  key: string
  label: string
  value: string
}

export interface KbRecord {
  kind: KbRecordKind
  key: string
  title: string
  source: KbSource
  fields: KbRecordField[]
}

// KbChangeType mirrors chatkb.ChangeType.
export type KbChangeType = 'added' | 'removed' | 'updated'

export interface KbFieldDiff {
  key: string
  label: string
  real: string
  draft: string
}

export interface KbItemData {
  record: KbRecord
}
export interface KbListData {
  kind: KbRecordKind | ''
  source: KbSource
  records: KbRecord[]
}
export interface KbComparisonData {
  kind: KbRecordKind
  key: string
  title: string
  change: KbChangeType
  real: KbRecord | null
  draft: KbRecord | null
  fields: KbFieldDiff[]
}

// ChatComponent is one structured element attached to an assistant turn. The
// union is discriminated by `type` — see components/chat/KbComponent.vue.
export type ChatComponent =
  | { type: 'kb_item'; data: KbItemData }
  | { type: 'kb_list'; data: KbListData }
  | { type: 'kb_comparison'; data: KbComparisonData }

export type ChatRole = 'user' | 'assistant' | 'system'

// ChatMessage mirrors internal/chat.Message.
export interface ChatMessage {
  id: string
  role: ChatRole
  content: string
  components: ChatComponent[]
  metadata: Record<string, unknown>
  created_at: string
}

// ChatConversation mirrors chatstore.Conversation — one sidebar row.
export interface ChatConversation {
  id: string
  title: string
  created_at: string
  updated_at: string
}

// ChatConversationDetail is GET /chat/conversations/:id — a thread plus its
// whole transcript.
export interface ChatConversationDetail {
  conversation: ChatConversation
  messages: ChatMessage[]
}
