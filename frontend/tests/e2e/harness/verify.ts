import { execFile } from 'node:child_process'
import path from 'node:path'
import { promisify } from 'node:util'
import { type Page, expect } from '@playwright/test'
import { REPO_ROOT } from './env'

// verify.ts — backend/API-level ground truth for every UI action the journey
// spec takes. Every function here calls the API through `page.request`,
// which shares the browser context's session cookie (xchats_session) — never
// a second, separately-authenticated client. These functions VERIFY state a
// UI step already changed; they never themselves drive the feature (see the
// suite's "always act through the UI" principle).

const API = '/xchats/api/v1'

// HoldingText verbatim, backend/response/service.go:15 — the canned fallback
// substituted ONLY when Generate itself hard-fails (missing/invalid LLM
// credential, KB-load error, contract-validation failure), never in place of
// the model's own escalation text. Its presence in a simulator reply means
// the KB/LLM isn't actually grounding the answer.
export const HOLDING_TEXT = 'Сейчас у меня нет этой информации — передаю ваш вопрос менеджеру и вернусь с точным ответом.'

// MEDIA_URL_PATTERN mirrors dto.go's mediaURL(id) — /xchats/api/v1/media/<uuid>.
export const MEDIA_URL_PATTERN = /\/xchats\/api\/v1\/media\/[0-9a-fA-F-]{20,36}/

interface Envelope<T> {
  payload: T
  errcode: string
  message: string
}

async function apiGet<T>(page: Page, urlPath: string): Promise<T> {
  const res = await page.request.get(API + urlPath)
  const env = (await res.json()) as Envelope<T>
  expect(env.errcode, `GET ${urlPath} failed: ${env.errcode} ${env.message}`).toBe('OK')
  return env.payload
}

// --- credentials (Phase 1) --------------------------------------------------

export interface IntegrationSummary {
  id: string
  display_name: string
  configured: boolean
  source?: string
  last_verified_at?: string
  last_error?: string
}

// verifyCredentialStored confirms GET /settings/integrations reports the
// given provider id as configured — the ground truth behind whatever badge
// (Настроено/Подтверждено) the card happens to show, which depends on
// whether the live validation call against the real provider API succeeded.
export async function verifyCredentialStored(page: Page, provider: string): Promise<IntegrationSummary> {
  const { providers } = await apiGet<{ providers: IntegrationSummary[] }>(page, '/settings/integrations')
  const p = providers.find((x) => x.id === provider)
  expect(p, `provider "${provider}" missing from GET /settings/integrations`).toBeTruthy()
  expect(p!.configured, `provider "${provider}" is not configured after saving its credential (last_error: ${p!.last_error ?? 'none'})`).toBe(true)
  return p!
}

export interface LLMSettings {
  default_provider: string
  default_model: string
}

export async function verifyDefaultModel(page: Page, provider: string, model: string): Promise<void> {
  const settings = await apiGet<{ llm: LLMSettings }>(page, '/settings')
  expect(settings.llm.default_provider, 'llm.default_provider').toBe(provider)
  expect(settings.llm.default_model, 'llm.default_model').toBe(model)
}

// --- draft (Phases 2-5) ------------------------------------------------

export interface DraftChangeSet {
  base_version: number
  config: Record<string, unknown> | null
  topics: Array<Record<string, unknown>>
  tariffs: Array<Record<string, unknown>>
  products: Array<Record<string, unknown>>
  contacts: Array<Record<string, unknown>>
  policies: Array<Record<string, unknown>>
  zones: Array<Record<string, unknown>>
  deletes: Array<{ kind: string; key: string }>
}

// entityType accepts either the UI's plural content kind (delivery_zones) or
// the wire field it actually lands on in DraftChangeSet (zones) — kbEntities.
// ts's own payloadField table documents the one deliberate mismatch.
const DRAFT_FIELD: Record<string, keyof DraftChangeSet> = {
  topics: 'topics',
  tariffs: 'tariffs',
  products: 'products',
  delivery_zones: 'zones',
  zones: 'zones',
  contacts: 'contacts',
  policies: 'policies',
}

export async function loadDraftChanges(page: Page): Promise<DraftChangeSet> {
  return apiGet<DraftChangeSet>(page, '/playground/draft')
}

// Topics are keyed by `slug`, every other kind by `ref` — both are the same
// "natural key" the import synthesizer reports back as AppliedUpsert.Key.
function refOf(row: Record<string, unknown>): string {
  return String(row.ref ?? row.slug ?? row.id ?? '')
}

// --- live KB (published ai_* rows) ---------------------------------------

// LiveKb is GET /kb's payload — kbstore.LiveView, the same field names
// DraftChangeSet uses (note `zones`, not `delivery_zones`), so DRAFT_FIELD
// maps both.
export type LiveKb = Record<string, unknown> & {
  config: Record<string, unknown>
}

export async function loadLiveKb(page: Page): Promise<LiveKb> {
  return apiGet<LiveKb>(page, '/kb')
}

// liveRefs returns every natural key the PUBLISHED KB already holds for one
// kind. This — not AppliedUpsert.Created — is what Черновик derives a staged
// card's badge from: composables/draftChanges.ts classifies an entry as
// 'added' (→ "Новый") when it has no live counterpart and 'updated' (→
// "Изменён") when it does. Created answers a different question, "was this
// key absent from live ∪ DRAFT", so it reports false for a row a previous,
// still-unpublished run already staged — which the UI nonetheless shows as
// "Новый" because live has never seen it.
export async function liveRefs(page: Page, entityType: string): Promise<Set<string>> {
  const live = await loadLiveKb(page)
  const field = DRAFT_FIELD[entityType] ?? entityType
  const rows = (live[field] as Array<Record<string, unknown>>) ?? []
  return new Set(rows.map(refOf).filter(Boolean))
}

export async function verifyDraftContains(page: Page, entityType: string, ref: string): Promise<Record<string, unknown>> {
  const changes = await loadDraftChanges(page)
  const field = DRAFT_FIELD[entityType] ?? (entityType as keyof DraftChangeSet)
  const rows = (changes[field] as Array<Record<string, unknown>>) ?? []
  const row = rows.find((r) => refOf(r) === ref)
  expect(row, `expected draft.${String(field)} to contain "${ref}"; got: ${rows.map(refOf).join(', ') || '(empty)'}`).toBeTruthy()
  return row!
}

export async function verifyDraftExcludes(page: Page, entityType: string, ref: string): Promise<void> {
  const changes = await loadDraftChanges(page)
  const field = DRAFT_FIELD[entityType] ?? (entityType as keyof DraftChangeSet)
  const rows = (changes[field] as Array<Record<string, unknown>>) ?? []
  const found = rows.some((r) => refOf(r) === ref)
  expect(found, `expected draft.${String(field)} to NOT contain "${ref}", but it does`).toBe(false)
}

export async function verifyDraftEmpty(page: Page): Promise<void> {
  const c = await loadDraftChanges(page)
  const pending = {
    config: c.config ? 1 : 0,
    topics: c.topics.length,
    tariffs: c.tariffs.length,
    products: c.products.length,
    contacts: c.contacts.length,
    policies: c.policies.length,
    zones: c.zones.length,
    deletes: c.deletes.length,
  }
  const total = Object.values(pending).reduce((a, b) => a + b, 0)
  expect(total, `expected an empty draft, got pending counts: ${JSON.stringify(pending)}`).toBe(0)
}

// verifyDraftConfigContains confirms a config/persona-shaped patch is
// staged: `field` is one of persona|mission|guardrails|language_policy|
// reply_max_words, and its pending value must equal `value`.
export async function verifyDraftConfigContains(page: Page, field: string, value: string | number): Promise<void> {
  const c = await loadDraftChanges(page)
  expect(c.config, 'expected a pending config patch in the draft, got none').toBeTruthy()
  expect((c.config as Record<string, unknown>)[field], `draft.config.${field}`).toBe(value)
}

// --- KB import runs (Phase 2) ------------------------------------------

export interface ImportMaterialStatus {
  id: string
  kind: 'url' | 'file'
  label: string
  processing_status: string
  error?: string
}

export interface ImportRun {
  run_id: string
  status: 'extracting' | 'synthesizing' | 'built' | 'failed' | 'needs_human'
  materials: ImportMaterialStatus[]
  synthesis?: {
    status: string
    notes?: string
    applied?: Array<{ tool: string; type: string; key: string; created: boolean }>
    dropped?: Array<{ tool: string; reason: string }>
  }
}

const TERMINAL_STATUSES: ImportRun['status'][] = ['built', 'failed', 'needs_human']

export async function getImportRun(page: Page, runId: string): Promise<ImportRun> {
  return apiGet<ImportRun>(page, `/kb/imports/${runId}`)
}

// verifyImportRunTerminal polls GET /kb/imports/:id until the run reaches a
// terminal status (built|failed|needs_human) or the timeout elapses — the
// backend allows only one active run per org, so every caller MUST await
// this before starting the next import.
export async function verifyImportRunTerminal(page: Page, runId: string, timeoutMs = 240_000): Promise<ImportRun> {
  const deadline = Date.now() + timeoutMs
  let last: ImportRun | undefined
  while (Date.now() < deadline) {
    last = await getImportRun(page, runId)
    if (TERMINAL_STATUSES.includes(last.status)) return last
    await new Promise((r) => setTimeout(r, 2500))
  }
  throw new Error(`import run ${runId} did not reach a terminal status within ${timeoutMs}ms (last status: ${last?.status})`)
}

// --- assembled prompt (Phase 6) -----------------------------------------

export interface PromptView {
  prompt_ref: string
  rendered_text: string
  status: 'ok' | 'error'
  error?: string
  section_counts: Record<string, number>
}

export async function loadPrompt(page: Page): Promise<PromptView> {
  return apiGet<PromptView>(page, '/kb/prompt')
}

export async function verifyPromptContains(page: Page, ...texts: string[]): Promise<PromptView> {
  const view = await loadPrompt(page)
  expect(view.status, `prompt assembly is not ok: ${view.error ?? '(no error message)'}`).toBe('ok')
  for (const text of texts) {
    expect(view.rendered_text, `expected the assembled prompt to contain "${text}"`).toContain(text)
  }
  return view
}

export async function verifyPromptExcludes(page: Page, ...texts: string[]): Promise<PromptView> {
  const view = await loadPrompt(page)
  for (const text of texts) {
    expect(view.rendered_text, `expected the assembled prompt to NOT contain "${text}"`).not.toContain(text)
  }
  return view
}

// --- container logs (final sweep) ---------------------------------------

const execFileAsync = promisify(execFile)
const COMPOSE_FILE = path.resolve(REPO_ROOT, 'deploy/docker-compose.yaml')
const ERROR_LINE = /\bpanic:|level=ERROR|level=error/

export interface LogSweepResult {
  checked: boolean
  suspiciousLines: string[]
  note?: string
}

// verifyNoBackendErrors sweeps `docker compose logs backend` for panics/
// error-level log lines, optionally scoped to everything logged since
// `sinceISO` (this test run's own start time — so a pre-existing, unrelated
// error from before the run started never fails it). Best-effort: if the
// Docker CLI or the `xchats` compose project isn't reachable from wherever
// the suite is running (e.g. the backend was started with `go run` instead
// of `make up`), this returns checked:false with a note instead of failing
// the run — the journey itself is still fully verified through the API
// assertions above.
export async function verifyNoBackendErrors(sinceISO?: string): Promise<LogSweepResult> {
  try {
    const args = ['compose', '-p', 'xchats', '-f', COMPOSE_FILE, 'logs', 'backend', '--no-color', '--no-log-prefix']
    if (sinceISO) args.push('--since', sinceISO)
    const { stdout } = await execFileAsync('docker', args, { maxBuffer: 32 * 1024 * 1024 })
    const suspiciousLines = stdout.split('\n').filter((l) => ERROR_LINE.test(l))
    return { checked: true, suspiciousLines }
  } catch (e) {
    return { checked: false, suspiciousLines: [], note: e instanceof Error ? e.message : String(e) }
  }
}
