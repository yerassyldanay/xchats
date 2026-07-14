export interface User {
  id: string
  email: string
  name: string
}
export interface Organization {
  id: string
  name: string
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
export interface Chat {
  id: string
  wa_account_id: string
  contact: Contact
  status: string
  assignee_user_id: string | null
  unread_count: number
  last_message_at: string | null
  last_message_preview: string
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
  evolution_message_id: string
  message_type: string
  content: string
  media: Media[]
  status: string
  source: string
  timestamp: string | null
}
export interface WhatsAppAccount {
  id: string
  instance_name: string
  display_name: string
  connection_status: string // connecting | qr_required | connected | disconnected | error
  assigned: boolean
  owner_jid: string
  phone_number: string
  last_live_event_at: string | null
  created_at: string | null
}
export interface QrResponse {
  status: string // qr_required | connecting | connected
  qr_code?: string | null
  qr_base64?: string | null
  pairing_code?: string | null
  wa_account_id?: string
  instance_name?: string
}
export interface EvolutionInstance {
  name: string
  connection_status: string
  owner_jid: string
  phone_number: string
  created_at: string | null
  managed: boolean
  stale: boolean
}
export interface DraftMedia {
  asset_id: string
  media_kind: string
  url: string
}
export interface AiDraft {
  id: string
  chat_id: string
  trigger_message_id: string | null
  ordinal: number
  draft_text: string
  media: DraftMedia[]
  context_status: string
  confidence: number | null
  escalate: boolean
  escalation_reason: string
  status: string
  created_at: string
}

// --- Knowledge Base (Playground) — see plan/7.1-endpoints.md, plan/15 ---
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
  lang: string
  title: string
  keywords: string
  body_md: string
  draft: boolean
  provenance?: string
  updated_at: string
}
export interface AssetRow {
  id: string // = ref
  ref: string
  kind: string
  owner_kind: string // 'topic' | 'product' | 'tariff' | '' (unattached)
  owner_ref: string
  title: string
  description: string
  url: string
  lang: string
  draft: boolean
  provenance?: string
  updated_at: string
}
// Facts lane — typed entity rows. Every exact fact (price, limit, phone, …) is a
// CONCRETE COLUMN here, never a generic key→value; the brain quotes it in replies
// as a {{table.slug.field}} token. `id` = the row's natural key (ref, or the
// contact singleton lang) since blob entries carry no DB row id.
export interface TariffRow {
  id: string // = ref
  ref: string
  lang: string
  name: string
  price: string
  limit_text: string
  fee: string
  summary: string
  pricing_type: string // fixed | percentage | tiered
  advantages: string
  disadvantages: string
  draft: boolean
  provenance?: string
  updated_at: string
}
export interface ProductRow {
  id: string // = ref
  ref: string
  lang: string
  name: string
  price: string
  description: string
  category: string
  availability: string
  draft: boolean
  provenance?: string
  updated_at: string
}
export interface ContactRow {
  id: string // = lang (the 'support' singleton, one row per language)
  slug: string
  lang: string
  whatsapp: string
  email: string
  address: string
  legal: string
  callback_time: string
  working_hours: string
  phone: string
  website: string
  instagram: string
  draft: boolean
  provenance?: string
  updated_at: string
}
// PolicyRow — org commerce-policy scalars (delivery/payment/returns terms), a
// structural clone of ContactRow (id = lang, the 'main' singleton).
export interface PolicyRow {
  id: string // = lang
  slug: string
  lang: string
  delivery_cost: string
  delivery_time: string
  free_delivery_from: string
  min_order: string
  prepayment: string
  installment: string
  return_period: string
  warranty: string
  draft: boolean
  provenance?: string
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
}
export interface KbRequest {
  id: string
  material_id: string | null
  req_type: string // confirm_fact | describe_media | comment
  prompt: string
  context: string // JSON string, e.g. {"suggested":"5000 ₸"}
  target: string // JSON string, e.g. {table,slug,field,lang} | {asset_ref} | {material_id}
  state: string // pending | resolved | dismissed
  resolution: string | null
  created_at: string
  resolved_at: string | null
}
export interface DraftView {
  config: DraftConfig
  topics: TopicRow[]
  assets: AssetRow[]
  tariffs: TariffRow[]
  products: ProductRow[]
  contacts: ContactRow[]
  policies: PolicyRow[]
  materials: KbMaterial[]
  requests: KbRequest[]
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
  scores: VScore[]
  rollups: VRollup[]
  cost: VCost
  latency_ms?: number
  scenario?: VScenarioDetails
  extract?: VExtractDetails
}
export interface ExecutionsFile {
  schema_version: number
  run_id: string
  launch_id?: string
  generated_at: string
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
