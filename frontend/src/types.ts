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
  setup_completed: boolean
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
export type ChannelName = 'whatsapp' | 'simulator' | 'telegram'

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
  in_stock: boolean
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
  zones: DeliveryZoneRow[]
  deletes: DraftChangeDelete[]
}

// DraftChangeDelete is one staged removal, addressed in the SAME plural
// vocabulary POST /playground/draft/approve/:kind/:id and DELETE
// /playground/draft/changes/:kind/:key use.
export interface DraftChangeDelete {
  kind: string // topics|tariffs|products|contacts|policies|delivery_zones
  key: string
}

// CancelChangeResponse mirrors handlePlaygroundCancelChange's response body.
export interface CancelChangeResponse {
  changed: boolean
  changes: DraftChangeSet
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

// --- Customer management (CRM) — mirrors internal/dto/crm.go -----------------

// CustomerStatus mirrors dto.CustomerStatus — one entry of the organization's
// configurable lifecycle. These are rows, not a fixed enum: an organization
// edits its own list, and the five seeded by migration 0011 are a starting
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
