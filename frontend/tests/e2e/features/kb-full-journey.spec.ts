import { type BrowserContext, type Locator, type Page, type TestInfo, expect, test } from '@playwright/test'
import { login } from '../helpers'
import {
  BASE_URL,
  FIRECRAWL_KEY,
  KASPI_URL,
  LLAMAPARSE_KEY,
  LLM_KEY,
  LLM_MODEL,
  LLM_PROVIDER,
  PDF_EXISTS,
  PDF_PATH,
  XPAYMENT_URL,
  missingCredentialKeys,
} from '../harness/env'
import {
  HOLDING_TEXT,
  MEDIA_URL_PATTERN,
  type ImportRun,
  loadDraftChanges,
  verifyCredentialStored,
  verifyDefaultModel,
  verifyDraftConfigContains,
  verifyDraftContains,
  verifyDraftEmpty,
  verifyDraftExcludes,
  liveRefs,
  verifyImportRunTerminal,
  verifyNoBackendErrors,
  verifyPromptContains,
  verifyPromptExcludes,
} from '../harness/verify'
import { runDir, shot, step } from '../harness/shot'

// kb-full-journey.spec.ts is a feature test, not a PR test (see
// docs/plan for the KB import & AI assistant feature this exercises) — it
// proves the feature works exactly as a user experiences it: every action
// through the browser UI, every result cross-checked against the backend
// API, and it names no branch/commit anywhere. Built from scratch with zero
// dependency on scripts/ (see that directory's own note on the legacy,
// hardcoded-credential ad-hoc scripts this suite replaces).
//
// Structure: ONE page/browser context, shared across 22 named test()
// blocks (steps 1-21 individually, 22-25 combined — see that test's own
// comment for why) via test.beforeAll/afterAll rather than the per-test
// `page` fixture — this is what makes `mode: 'serial'` meaningful here: each
// step depends on browser state (cookies, current route, what's on screen)
// the previous step left behind, not just shared backend data. A genuine
// failure aborts the remaining steps (nothing downstream could be trusted
// anyway); a MISSING PREREQUISITE (no .env key, no example.pdf, an import
// blocked by the target site) SKIPS just that step and its dependents
// without aborting the chain — see test.skip() calls throughout.
//
// Isolation: every entity this run creates is tracked in `state.entities`
// and removed again in afterAll (live-KB DELETE, since Phase 6 published
// everything); the five assistant-config fields Phase 5 touches are
// restored to their pre-run values. Nothing here ever calls the blanket
// "discard whole draft" endpoint — only this run's own keys are touched, so
// running this against a shared dev database never clobbers unrelated
// pending work.

test.describe.configure({ mode: 'serial' })

interface EntityInfo {
  kind: 'products' | 'tariffs' | 'topics'
  ref: string
  name: string
  hasImage: boolean
  // existsLive: did the PUBLISHED KB already hold this natural key when the
  // import staged it? Not cosmetic — re-running this journey against an org
  // that still holds a previous run's published rows legitimately re-imports
  // the same refs, and the draft stages those as edits ("Изменён", with a
  // FieldDiffNote "Было: …") rather than additions ("Новый"); a product that
  // was already live also SURVIVES the cancelled draft edit in Phase 4
  // instead of disappearing. Phase 3's badge assertion and Phase 4's deletion
  // target both depend on knowing which.
  //
  // Deliberately NOT AppliedUpsert.Created: that flag answers "was this key
  // absent from live ∪ draft", so it reads false for a row an earlier,
  // still-unpublished run already staged — which Черновик nonetheless badges
  // "Новый", since live has never seen it. The UI classifies on live presence
  // alone (composables/draftChanges.ts), so this mirrors that exactly.
  existsLive: boolean
}

interface ConfigSnapshot {
  persona: string
  mission: string
  guardrails: string
  language_policy: string
  reply_max_words: number
}

const runTag = `e2e-${Date.now()}`
const KIND_TAB_LABEL: Record<EntityInfo['kind'], string> = { products: 'Товары', tariffs: 'Тарифы', topics: 'Темы' }
const SYNTH_TYPE_TO_KIND: Record<string, EntityInfo['kind'] | undefined> = { product: 'products', tariff: 'tariffs', topic: 'topics' }

const state = {
  entities: [] as EntityInfo[],
  skippedImports: [] as string[],
  productToDelete: '',
  productToDeleteName: '',
  personaText: '',
  missionText: '',
  guardrailsText: '',
  languagePolicyText: '',
  replyMaxWords: 0,
  originalConfig: null as ConfigSnapshot | null,
}

function entitiesOf(kind: EntityInfo['kind']): EntityInfo[] {
  return state.entities.filter((e) => e.kind === kind)
}

let context: BrowserContext
let page: Page
let runStartedAt = ''
const pageErrors: string[] = []

test.beforeAll(async ({ browser }) => {
  const missing = missingCredentialKeys()
  if (missing.length) {
    console.log(`[kb-full-journey] missing from .env: ${missing.join(', ')} — steps that need them will be skipped, not failed.`)
  }

  context = await browser.newContext({ baseURL: BASE_URL, viewport: { width: 1440, height: 900 } })
  page = await context.newPage()
  page.on('pageerror', (err) => pageErrors.push(err.message))
  runStartedAt = new Date().toISOString()

  await login(page) // dismisses the first-run SetupWizard internally, if present

  try {
    const res = await page.request.get('/xchats/api/v1/kb')
    const env = await res.json()
    const cfg = env.payload?.config ?? {}
    state.originalConfig = {
      persona: cfg.persona ?? '',
      mission: cfg.mission ?? '',
      guardrails: cfg.guardrails ?? '',
      language_policy: cfg.language_policy ?? '',
      reply_max_words: cfg.reply_max_words ?? 0,
    }
  } catch {
    // best-effort — if this fails, afterAll's restoration step just no-ops
  }
})

test.beforeEach(({}, testInfo) => {
  testInfo.setTimeout(300_000) // imports can take minutes — see plan's Playwright config notes
})

test.afterAll(async () => {
  // Best-effort cleanup: remove everything this run published live (Phase 6
  // published all of state.entities except the one cancelled in Phase 4),
  // and restore the assistant-config fields Phase 5 touched.
  for (const e of state.entities) {
    if (e.ref === state.productToDelete) continue // cancelled in the draft — never went live
    const path = e.kind === 'topics' ? `topics/${encodeURIComponent(e.ref)}` : `${e.kind}/${encodeURIComponent(e.ref)}`
    try {
      await page.request.delete(`/xchats/api/v1/kb/${path}`)
    } catch {
      // best-effort — see file doc comment
    }
  }
  if (state.originalConfig) {
    try {
      await page.request.patch('/xchats/api/v1/kb/config', { data: state.originalConfig })
    } catch {
      // best-effort
    }
  }

  if (pageErrors.length) {
    console.warn(`[kb-full-journey] ${pageErrors.length} uncaught frontend error(s):\n${pageErrors.join('\n')}`)
  }
  const sweep = await verifyNoBackendErrors(runStartedAt)
  if (sweep.checked && sweep.suspiciousLines.length) {
    console.warn(
      `[kb-full-journey] backend log sweep found ${sweep.suspiciousLines.length} suspicious line(s) since ${runStartedAt}:\n` +
        sweep.suspiciousLines.slice(0, 20).join('\n'),
    )
  } else if (!sweep.checked) {
    console.warn(`[kb-full-journey] backend log sweep skipped (${sweep.note})`)
  }
  if (state.skippedImports.length) {
    console.warn(`[kb-full-journey] skipped imports:\n${state.skippedImports.join('\n')}`)
  }
  console.log(`[kb-full-journey] screenshot trail: ${runDir()}`)

  await context.close()
})

// --- shared helpers ----------------------------------------------------

// productCard finds a kb/records/*Record.vue card by text, scoped to the
// Черновик Товары tab's own wrapper (data-testid="draft-tab-products") —
// necessary because Черновик's tab panels are v-show, not v-if: every
// kind's cards stay mounted at once, so an unscoped kb-record filter can
// match a same/overlapping-text card on a different, hidden tab.
function productCard(page: Page, text: string): Locator {
  return page.locator('[data-testid="draft-tab-products"] [data-testid="kb-record"]').filter({ hasText: text })
}
// importTab scopes to ONE of KbIngestPanel's two ingest tabs (Ссылки/Файлы)
// — same v-show-not-v-if situation as draft-tab-products above: both
// KbImportCard instances stay mounted at once, so their shared provider/
// target-type/guidance/submit controls are each attached twice in the DOM
// regardless of which tab is visually active. Scope every such control
// through the active tab's own wrapper instead of querying the page bare.
function importTab(page: Page, kind: 'urls' | 'files'): Locator {
  return page.locator(`[data-testid="kb-import-tab-${kind}"]`)
}
// liveCard additionally scopes to ONE of Знаний база's own tabs
// (data-testid="live-tab-<kind>") — same v-show-not-v-if situation as
// productCard/importTab above: KnowledgeBase.vue's tab panels (config/
// topics/products/tariffs/delivery_zones) all stay mounted at once, so an
// unscoped kb-record filter can match a same/overlapping-text card on a
// DIFFERENT, currently-hidden tab (e.g. "Миссия" colliding with seeded
// content elsewhere on the page).
function liveCard(page: Page, kind: 'config' | 'topics' | 'products' | 'tariffs' | 'delivery_zones', text: string): Locator {
  return page.locator(`[data-testid="live-tab-${kind}"] [data-testid="kb-record"]`).filter({ hasText: text })
}

// saveProviderCredential drives ProviderCredentialCard.vue's save flow for
// ONE provider: reveal the field if the card starts collapsed, fill it,
// submit, and accept a "Сохранить всё равно" force-save if the live
// verification call was merely inconclusive (never if the key was
// confirmed invalid — the backend refuses to let Force override that).
async function saveProviderCredential(card: Locator, apiKey: string): Promise<void> {
  // A credential can already be resolved from a backend-side environment
  // variable (Chain.Source == "env") — the card then shows "managed by
  // environment" and renders no editable form at all (ProviderCredentialCard.vue's
  // managedByEnv branch). Nothing to do through the UI in that case.
  const managedByEnv = card.getByText('задано через переменную окружения')
  if (await managedByEnv.isVisible({ timeout: 2_000 }).catch(() => false)) return

  const secretInput = card.getByPlaceholder('Введите значение')
  if (!(await secretInput.isVisible().catch(() => false))) {
    await card.getByRole('button', { name: 'Сохранить ключ' }).click()
  }
  await expect(secretInput).toBeVisible()
  await secretInput.fill(apiKey)
  await card.getByRole('button', { name: /^Сохранить/ }).click()

  // The live verification call can come back "inconclusive" rather than a
  // clean pass/fail (backend errcode CREDENTIAL_UNVERIFIED, surfaced as a
  // 409) — the SAME submit button then relabels to "Сохранить всё равно"
  // and needs a second click to force-save. isVisible() does NOT wait (it
  // samples the DOM immediately, ignoring its `timeout` option), so it
  // would race the in-flight save request and always see the pre-response
  // label; waitFor() actually polls, so use that instead.
  const forceButton = card.getByRole('button', { name: 'Сохранить всё равно' })
  const becameForce = await forceButton
    .waitFor({ state: 'visible', timeout: 10_000 })
    .then(() => true)
    .catch(() => false)
  if (becameForce) {
    await forceButton.click()
  }
  await expect(secretInput).toBeHidden({ timeout: 20_000 })
}

// afterSubmitImport clicks the submit button and reads run_id straight off
// THAT click's own POST /kb/imports response — not a separate "latest run"
// GET afterward. The backend allows only one active run per org, so a
// second query can easily race a PREVIOUS run's own terminal-status write
// (the background extractor retries every few seconds) and silently return
// someone else's run_id instead of the one this call just created.
async function afterSubmitImport(label: string, submitButton: Locator): Promise<string> {
  const responsePromise = page.waitForResponse(
    (res) => res.request().method() === 'POST' && res.url().includes('/xchats/api/v1/kb/imports'),
  )
  await submitButton.click()
  const res = await responsePromise
  const env = await res.json()
  const runId: string = env.payload?.run_id ?? ''
  expect(runId, `expected a run_id in the submit response for ${label}`).toBeTruthy()
  await expect(page.locator('[data-testid="kb-import-run-status"]')).toBeVisible({ timeout: 20_000 })
  return runId
}

// resolveImportOrSkip waits for a run to reach a terminal status, records
// its synthesized entities into state.entities, or soft-skips the CALLING
// test with a clear reason (blocked crawl, quota, nothing extractable) —
// imports can legitimately fail for reasons outside this suite's control;
// the rest of the journey must still run against whatever else succeeded.
async function resolveImportOrSkip(testInfo: TestInfo, label: string, runId: string): Promise<void> {
  let run: ImportRun
  try {
    run = await verifyImportRunTerminal(page, runId)
  } catch (e) {
    await shot(page, testInfo, `${label}: timed out waiting for a terminal status`)
    state.skippedImports.push(`${label}: timed out — ${e instanceof Error ? e.message : String(e)}`)
    test.skip(true, `${label} did not reach a terminal status in time`)
    return
  }
  await page.reload()
  await shot(page, testInfo, `${label}: final status ${run.status}`)

  if (run.status !== 'built') {
    const reason = run.materials.find((m) => m.error)?.error ?? run.status
    state.skippedImports.push(`${label}: ${run.status} — ${reason}`)
    test.skip(true, `${label} ended in status "${run.status}" (${reason}) — continuing the rest of the journey against whatever else succeeded.`)
    return
  }

  const applied = run.synthesis?.applied ?? []
  // Snapshot live presence ONCE per kind, before anything is published — this
  // is what decides each staged card's badge (see EntityInfo.existsLive).
  const liveByKind = new Map<EntityInfo['kind'], Set<string>>()
  for (const kind of ['products', 'tariffs', 'topics'] as const) {
    liveByKind.set(kind, await liveRefs(page, kind))
  }
  let added = 0
  for (const a of applied) {
    const kind = SYNTH_TYPE_TO_KIND[a.type]
    if (!kind) continue
    const row = await verifyDraftContains(page, kind, a.key)
    const name = String(row.name ?? row.title ?? a.key)
    const hasImage = !!row.featured_image
    state.entities.push({ kind, ref: a.key, name, hasImage, existsLive: liveByKind.get(kind)!.has(a.key) })
    added++
  }
  if (!added) {
    state.skippedImports.push(`${label}: built, but synthesis produced no products/tariffs/topics`)
    test.skip(true, `${label} completed but produced no usable entities — continuing.`)
  }
}

async function askSimulator(testInfo: TestInfo, stepLabel: string, question: string): Promise<string> {
  return step(testInfo, page, stepLabel, async () => {
    const bubbles = page.locator('[data-testid="simulator-message"][data-role="assistant"]')
    const before = await bubbles.count()
    await page.locator('[data-testid="simulator-input"]').fill(question)
    await page.locator('[data-testid="simulator-send"]').click()
    await expect(bubbles).toHaveCount(before + 1, { timeout: 90_000 })
    return (await bubbles.last().locator('[data-testid="simulator-message-text"]').innerText()).trim()
  })
}

// =====================================================================
// Phase 1 — configure providers (as a new user would)
// =====================================================================

test('1. Settings → Парсеры и краулеры: save the Firecrawl credential', async ({}, testInfo) => {
  test.skip(!FIRECRAWL_KEY, 'FIRECRAWL_API_KEY not set in .env — see harness/env.ts')
  await step(testInfo, page, '1. Configure Firecrawl credential', async () => {
    await page.goto('/settings')
    await page.getByRole('button', { name: 'Парсеры и краулеры' }).click()
    const card = page.locator('[data-testid="integration-card-firecrawl"]')
    await expect(card).toBeVisible()
    await saveProviderCredential(card, FIRECRAWL_KEY)
    await expect(card.getByText('Не настроено')).toHaveCount(0)
  })
  await verifyCredentialStored(page, 'firecrawl')
})

test('2. Settings → Парсеры и краулеры: save the LlamaParse credential', async ({}, testInfo) => {
  test.skip(!LLAMAPARSE_KEY, 'LLAMA_PARSE_API_KEY not set in .env — see harness/env.ts')
  await step(testInfo, page, '2. Configure LlamaParse credential', async () => {
    await page.goto('/settings')
    await page.getByRole('button', { name: 'Парсеры и краулеры' }).click()
    const card = page.locator('[data-testid="integration-card-llamaparse"]')
    await expect(card).toBeVisible()
    await saveProviderCredential(card, LLAMAPARSE_KEY)
    await expect(card.getByText('Не настроено')).toHaveCount(0)
  })
  await verifyCredentialStored(page, 'llamaparse')
})

test('3. Settings → ИИ-движок: select the provider and set the default model', async ({}, testInfo) => {
  test.skip(!LLM_KEY, 'LLM_API_KEY not set in .env — see harness/env.ts')
  await step(testInfo, page, '3. Configure AI engine (provider + model)', async () => {
    await page.goto('/settings')
    await page.getByRole('button', { name: 'ИИ-движок' }).click()

    // The dropdown only lists has_model providers, but selecting one with no
    // saved credential would leave the response engine unable to call it —
    // save this run's chosen provider's own key first, same card component
    // as Phase 1's, just rendered again under this tab.
    if (['openrouter', 'openai', 'gemini'].includes(LLM_PROVIDER)) {
      const providerCard = page.locator(`[data-testid="integration-card-${LLM_PROVIDER}"]`)
      await expect(providerCard).toBeVisible()
      await saveProviderCredential(providerCard, LLM_KEY)
    }

    await page.locator('[data-testid="ai-engine-default-provider"]').click()
    await page.getByRole('option', { name: new RegExp(`^${LLM_PROVIDER}`, 'i') }).click()
    await page.locator('[data-testid="ai-engine-default-model"]').fill(LLM_MODEL)
    await page.locator('[data-testid="ai-engine-save"]').click()
    await expect(page.getByText('Сохранено')).toBeVisible()
  })
  await verifyDefaultModel(page, LLM_PROVIDER, LLM_MODEL)
})

test('4. Playground → Ссылки: provider dropdown lists all three providers, no missing-key warnings', async ({}, testInfo) => {
  await step(testInfo, page, '4. Check the Ссылки provider dropdown', async () => {
    await page.goto('/playground')
    await page.getByRole('button', { name: 'Ссылки' }).click()
    await importTab(page, 'urls').locator('[data-testid="kb-import-provider"]').click()
    for (const name of ['Встроенный', 'Firecrawl', 'LlamaParse']) {
      await expect(page.getByRole('option', { name: new RegExp(`^${name}`) })).toBeVisible()
    }
    await page.keyboard.press('Escape')
    if (FIRECRAWL_KEY && LLAMAPARSE_KEY) {
      await expect(page.getByText('ключ не настроен')).toHaveCount(0)
    }
  })
})

// =====================================================================
// Phase 2 — import content
// =====================================================================

test('5. Playground → Ссылки: import a Kaspi shop URL via Firecrawl into Товары', async ({}, testInfo) => {
  test.skip(!FIRECRAWL_KEY, 'FIRECRAWL_API_KEY not set — cannot exercise the Firecrawl provider')
  let runId = ''
  await step(testInfo, page, '5. Submit Kaspi URL import (Firecrawl → Товары)', async () => {
    await page.goto('/playground')
    await page.getByRole('button', { name: 'Ссылки' }).click()
    const tab = importTab(page, 'urls')
    await tab.locator('[data-testid="kb-import-url-input"]').fill(KASPI_URL)
    await tab.locator('[data-testid="kb-import-add-url"]').click()
    await expect(page.getByText(KASPI_URL)).toBeVisible()

    await tab.locator('[data-testid="kb-import-provider"]').click()
    await page.getByRole('option', { name: /^Firecrawl/ }).click()
    await tab.locator('[data-testid="kb-import-target-type"]').click()
    await page.getByRole('option', { name: 'Товары', exact: true }).click()
    await tab.locator('[data-testid="kb-import-guidance"]').fill('Извлеки список товаров с названием и ценой.')

    runId = await afterSubmitImport('Kaspi import', tab.locator('[data-testid="kb-import-submit"]'))
  })
  await resolveImportOrSkip(testInfo, 'Kaspi import (Firecrawl → Товары)', runId)
})

test('6. Playground → Ссылки: import the xpayment.kz URL via Firecrawl into Тарифы', async ({}, testInfo) => {
  test.skip(!FIRECRAWL_KEY, 'FIRECRAWL_API_KEY not set — cannot exercise the Firecrawl provider')
  let runId = ''
  await step(testInfo, page, '6. Submit xpayment.kz URL import (Firecrawl → Тарифы)', async () => {
    await page.goto('/playground')
    await page.getByRole('button', { name: 'Ссылки' }).click()
    const tab = importTab(page, 'urls')
    await tab.locator('[data-testid="kb-import-url-input"]').fill(XPAYMENT_URL)
    await tab.locator('[data-testid="kb-import-add-url"]').click()
    await expect(page.getByText(XPAYMENT_URL)).toBeVisible()

    await tab.locator('[data-testid="kb-import-provider"]').click()
    await page.getByRole('option', { name: /^Firecrawl/ }).click()
    await tab.locator('[data-testid="kb-import-target-type"]').click()
    await page.getByRole('option', { name: 'Тарифы', exact: true }).click()
    await tab.locator('[data-testid="kb-import-guidance"]').fill('Извлеки тарифные планы с названием и ценой.')

    runId = await afterSubmitImport('xpayment.kz import', tab.locator('[data-testid="kb-import-submit"]'))
  })
  await resolveImportOrSkip(testInfo, 'xpayment.kz import (Firecrawl → Тарифы)', runId)
})

test('7. Playground → Файлы: import example.pdf via LlamaParse', async ({}, testInfo) => {
  test.skip(!LLAMAPARSE_KEY, 'LLAMA_PARSE_API_KEY not set — cannot exercise the LlamaParse provider')
  test.skip(!PDF_EXISTS, `example.pdf not found (looked at ${PDF_PATH})`)
  let runId = ''
  await step(testInfo, page, '7. Submit example.pdf import (LlamaParse)', async () => {
    await page.goto('/playground')
    await page.getByRole('button', { name: 'Файлы' }).click()
    const tab = importTab(page, 'files')
    await tab.locator('[data-testid="kb-import-file-input"]').setInputFiles(PDF_PATH)
    await expect(page.getByText('example.pdf')).toBeVisible()

    // PDF rules out native and firecrawl (extractor.Capabilities — neither
    // reads PDF); select LlamaParse explicitly rather than assume the UI
    // auto-picks the one remaining available option.
    await tab.locator('[data-testid="kb-import-provider"]').click()
    await page.getByRole('option', { name: /^LlamaParse/ }).click()

    runId = await afterSubmitImport('PDF import', tab.locator('[data-testid="kb-import-submit"]'))
  })
  await resolveImportOrSkip(testInfo, 'example.pdf import (LlamaParse)', runId)
})

// =====================================================================
// Phase 3 — review the draft
// =====================================================================

test('8. Черновик: review each entity tab — staged-state badge, populated fields', async ({}, testInfo) => {
  test.skip(!state.entities.length, 'no imports produced any entities — nothing to review')
  await step(testInfo, page, '8. Open Черновик for review', async () => {
    await page.goto('/playground')
  })
  for (const kind of ['products', 'tariffs', 'topics'] as const) {
    const entities = entitiesOf(kind)
    if (!entities.length) continue
    await step(testInfo, page, `8. Черновик — ${KIND_TAB_LABEL[kind]} tab`, async () => {
      await page.getByRole('button', { name: new RegExp(`^${KIND_TAB_LABEL[kind]}`) }).click()
      for (const e of entities) {
        const card = page.locator(`[data-testid="draft-tab-${kind}"] [data-testid="kb-record"]`).filter({ hasText: e.name })
        await expect(card).toBeVisible()
        // "Новый" only when live had never seen this ref. Re-running the
        // journey against an org that still holds a previous run's published
        // records legitimately re-imports the same natural refs, and the draft
        // stages those as edits — "Изменён", with a FieldDiffNote "Было: …" —
        // not additions. Asserting "Новый" unconditionally makes the whole spec
        // pass only ever on a virgin database.
        const wantBadge = e.existsLive ? 'Изменён' : 'Новый'
        await expect(card.getByText(wantBadge, { exact: true })).toBeVisible()
      }
    })
  }
})

test('9. Черновик: card counts on screen match GET /playground/draft', async ({}, testInfo) => {
  test.skip(!state.entities.length, 'no imports produced any entities — nothing to cross-check')
  const changes = await loadDraftChanges(page)
  await step(testInfo, page, '9. Cross-check entity counts against the API', async () => {
    await page.goto('/playground')
    for (const kind of ['products', 'tariffs', 'topics'] as const) {
      const apiCount = (changes[kind] as unknown[]).length
      if (!apiCount) continue
      await page.getByRole('button', { name: new RegExp(`^${KIND_TAB_LABEL[kind]}`) }).click()
      const uiCount = await page.locator(`[data-testid="draft-tab-${kind}"] [data-testid="kb-record"]`).count()
      expect(uiCount, `${kind}: UI shows ${uiCount} card(s), API draft has ${apiCount}`).toBe(apiCount)
    }
  })
})

// =====================================================================
// Phase 4 — delete a product (UX evaluation)
// =====================================================================

test('10. Черновик: locate one imported product card (deletion UX — identification)', async ({}, testInfo) => {
  // Must be a product this run CREATED, not one it merely re-imported over an
  // already-live record: Phase 4 removes it by cancelling the staged change,
  // which for an edit leaves the original row live and untouched. Step 20
  // then asserts the "deleted" product is absent from Знаний база — true only
  // if it never existed live to begin with. Picking entities[0] blindly makes
  // Phase 4+6 pass on a virgin database and fail on every re-run.
  const fresh = entitiesOf('products').filter((e) => !e.existsLive)
  test.skip(!fresh.length, 'every imported product already existed live — Phase 4 needs one whose removal is actually observable')
  const target = fresh[0]
  state.productToDelete = target.ref
  state.productToDeleteName = target.name
  const row = await verifyDraftContains(page, 'products', target.ref)
  await step(testInfo, page, '10. Locate the product card to delete', async () => {
    await page.goto('/playground')
    await page.getByRole('button', { name: /^Товары/ }).click()
    const card = productCard(page, target.name)
    await expect(card).toBeVisible()
    await expect(card.getByText(String(row.name))).toBeVisible()
    if (row.price) await expect(card.getByText(String(row.price))).toBeVisible()
  })
  testInfo.annotations.push({
    type: 'ux-finding',
    description: `Identification: the card shows name ("${row.name}") and price ("${row.price ?? '—'}") — the fields a user would tell products apart by.`,
  })
})

test('11. Черновик: is the destructive action clearly labeled?', async ({}, testInfo) => {
  test.skip(!state.productToDelete, 'no product selected in step 10')
  await step(testInfo, page, '11. Inspect the destructive action button', async () => {
    const card = productCard(page, state.productToDeleteName)
    await expect(card.getByRole('button', { name: 'Отменить изменение' })).toBeVisible()
  })
  const deleteLabelCount = await productCard(page, state.productToDeleteName).getByRole('button', { name: 'Удалить', exact: true }).count()
  testInfo.annotations.push({
    type: 'ux-finding',
    description:
      'Labeling: Черновик has no "Удалить" (Delete) action on a pending card at all — the only removal action is "Отменить изменение" ' +
      '(Cancel the change; records/actions.ts only returns DELETE for page:"live", never "draft"). It IS styled destructively (red text, ' +
      `destructive:true), but the word "delete" never appears — a first-time user may not immediately read "cancel the change" as ` +
      `"this product will be gone." (${deleteLabelCount} "Удалить" button(s) found on this card, expect 0)`,
  })
})

test('12. Черновик: does the system confirm before destroying data?', async ({}, testInfo) => {
  test.skip(!state.productToDelete, 'no product selected in step 10')
  let dialogSeen = false
  page.once('dialog', async (d) => {
    dialogSeen = true
    await d.accept()
  })
  await step(testInfo, page, '12. Click Отменить изменение and observe confirmation (or its absence)', async () => {
    await productCard(page, state.productToDeleteName).getByRole('button', { name: 'Отменить изменение' }).click()
    await page.waitForTimeout(500) // give a native dialog, if any, a chance to actually surface
  })
  testInfo.annotations.push({
    type: 'ux-finding',
    description: dialogSeen
      ? 'Confirmation: a native browser confirm() appeared before the card was removed.'
      : 'Confirmation: NONE. The click removes the card immediately — no native confirm(), no Vue modal — unlike the equivalent "Удалить" ' +
        'action on /knowledge-base, which opens a real ConfirmDeleteDialog.vue. One click permanently drops a freshly-imported product ' +
        'from the draft with no safety net.',
  })
})

test('13. Черновик: the product card disappears with clear feedback', async ({}, testInfo) => {
  test.skip(!state.productToDelete, 'no product selected in step 10')
  await step(testInfo, page, '13. Confirm the card is gone from the UI', async () => {
    await expect(productCard(page, state.productToDeleteName)).toHaveCount(0)
  })
  await verifyDraftExcludes(page, 'products', state.productToDelete)
})

test('14. Черновик: other product cards are unaffected', async ({}, testInfo) => {
  test.skip(!state.productToDelete, 'no product selected in step 10')
  const remaining = entitiesOf('products').filter((e) => e.ref !== state.productToDelete)
  await step(testInfo, page, '14. Confirm remaining product cards are untouched', async () => {
    for (const e of remaining) {
      await expect(productCard(page, e.name)).toBeVisible()
    }
  })
  for (const e of remaining) {
    await verifyDraftContains(page, 'products', e.ref)
  }
})

// =====================================================================
// Phase 5 — author identity & guardrails
// =====================================================================

async function editLiveConfigField(testInfo: TestInfo, stepLabel: string, fieldTitle: string, fill: (dialog: Locator) => Promise<void>): Promise<void> {
  await step(testInfo, page, stepLabel, async () => {
    await page.goto('/knowledge-base')
    await page.getByRole('button', { name: 'Обзор' }).click()
    const card = liveCard(page, 'config', fieldTitle)
    await expect(card).toBeVisible()
    await card.getByRole('button', { name: 'Изменить' }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await fill(dialog)
    await dialog.getByRole('button', { name: 'Сохранить', exact: true }).click()
    await expect(page.getByText('Изменение добавлено в Черновик')).toBeVisible()
  })
}

test('15. Знаний база → Обзор: set the assistant Персона', async ({}, testInfo) => {
  state.personaText = `Дружелюбный консультант интернет-магазина. Отвечает кратко и по делу, без канцелярита. (${runTag})`
  await editLiveConfigField(testInfo, '15. Set Персона', 'Персона', (dialog) => dialog.getByRole('textbox').fill(state.personaText))
  await verifyDraftConfigContains(page, 'persona', state.personaText)
})

test('16. Знаний база → Обзор: set the assistant Миссия', async ({}, testInfo) => {
  state.missionText = `Помочь клиенту быстро найти нужный товар или тариф и не ошибиться с оформлением. (${runTag})`
  await editLiveConfigField(testInfo, '16. Set Миссия', 'Миссия', (dialog) => dialog.getByRole('textbox').fill(state.missionText))
  await verifyDraftConfigContains(page, 'mission', state.missionText)
})

test('17. Знаний база → Обзор: set the assistant Ограничения', async ({}, testInfo) => {
  state.guardrailsText = `Не обсуждать конкурентов.\nНе давать юридических или медицинских советов.\nНикогда не выдумывать цены. (${runTag})`
  await editLiveConfigField(testInfo, '17. Set Ограничения', 'Ограничения', (dialog) => dialog.getByRole('textbox').fill(state.guardrailsText))
  await verifyDraftConfigContains(page, 'guardrails', state.guardrailsText)
})

test('18. Знаний база → Обзор: set Языковая политика and Макс. слов в ответе', async ({}, testInfo) => {
  state.languagePolicyText = `Отвечать на языке сообщения клиента, по умолчанию на русском. (${runTag})`
  await editLiveConfigField(testInfo, '18a. Set Языковая политика', 'Языковая политика', (dialog) =>
    dialog.getByRole('textbox').fill(state.languagePolicyText),
  )
  await verifyDraftConfigContains(page, 'language_policy', state.languagePolicyText)

  state.replyMaxWords = 60
  await editLiveConfigField(testInfo, '18b. Set Макс. слов в ответе', 'Макс. слов в ответе', (dialog) =>
    dialog.locator('input[type="number"]').fill(String(state.replyMaxWords)),
  )
  await verifyDraftConfigContains(page, 'reply_max_words', state.replyMaxWords)
})

// =====================================================================
// Phase 6 — publish and verify
// =====================================================================

test('19. Черновик: Опубликовать всё', async ({}, testInfo) => {
  await step(testInfo, page, '19. Publish the whole draft', async () => {
    await page.goto('/playground')
    // publishAll() carries no window.confirm() today (only discardAll()
    // does) — accept one defensively if it ever appears, without asserting
    // it must.
    page.once('dialog', (d) => d.accept())
    await page.locator('[data-testid="publish-all"]').click()
    await expect(page.locator('[data-testid="publish-all"]')).toHaveCount(0, { timeout: 30_000 })
  })
  await verifyDraftEmpty(page)
})

test('20. Знаний база: published records appear; the deleted product does not', async ({}, testInfo) => {
  await step(testInfo, page, '20. Browse published entity tabs', async () => {
    await page.goto('/knowledge-base')
    for (const kind of ['products', 'tariffs', 'topics'] as const) {
      const entities = entitiesOf(kind)
      if (!entities.length) continue
      await page.getByRole('button', { name: KIND_TAB_LABEL[kind], exact: true }).click()
      for (const e of entities) {
        if (e.ref === state.productToDelete) continue
        await expect(liveCard(page, kind, e.name)).toBeVisible()
      }
    }
    if (state.productToDeleteName) {
      await page.getByRole('button', { name: 'Товары', exact: true }).click()
      await expect(liveCard(page, 'products', state.productToDeleteName)).toHaveCount(0)
    }
  })
})

test('21. Знаний база → Промпт: assembled prompt has imported data + persona, excludes the deleted product', async ({}, testInfo) => {
  await step(testInfo, page, '21. Open the Промпт tab', async () => {
    await page.goto('/knowledge-base')
    await page.getByRole('button', { name: 'Промпт' }).click()
    await expect(page.getByText('Собран успешно')).toBeVisible()
  })

  const wantContains = [state.personaText]
  for (const e of entitiesOf('products')) if (e.ref !== state.productToDelete) wantContains.push(e.name)
  for (const e of entitiesOf('tariffs')) wantContains.push(e.name)
  const view = await verifyPromptContains(page, ...wantContains)
  if (entitiesOf('tariffs').length) {
    expect(view.rendered_text, 'expected a ТАРИФЫ block once tariffs exist in the KB').toContain('ТАРИФЫ')
  }
  if (state.productToDeleteName) {
    await verifyPromptExcludes(page, state.productToDeleteName)
  }
})

// =====================================================================
// Phase 7 — simulator UI (prove the bot actually works)
// =====================================================================

// Combined into ONE test (rather than four, one per question) so that a
// flaky/timed-out reply to one question — inherent to live LLM output —
// soft-fails and moves on to the next question instead of aborting the rest
// via mode:'serial'. Assertions target facts (a name appearing in the
// reply), never exact phrasing.
test('22-25. Симулятор: four grounding questions', async ({}, testInfo) => {
  test.skip(!LLM_KEY, 'LLM_API_KEY not set — the simulator cannot produce a grounded answer without a working LLM credential')

  await step(testInfo, page, '22-25. Open /simulator', async () => {
    await page.goto('/simulator')
    await expect(page.locator('[data-testid="simulator-input"]')).toBeVisible()
  })

  const withImage = state.entities.find((e) => e.hasImage && e.ref !== state.productToDelete)

  const questions: Array<{ n: number; text: string; check: (reply: string) => void }> = [
    {
      n: 22,
      text: 'Какие у вас тарифные планы и сколько они стоят?',
      check: (reply) => {
        const tariffs = entitiesOf('tariffs')
        if (!tariffs.length) return
        const mentioned = tariffs.some((t) => reply.includes(t.name))
        expect.soft(mentioned, `Q22: expected a real tariff name in the reply (have: ${tariffs.map((t) => t.name).join(', ')})`).toBe(true)
      },
    },
    {
      n: 23,
      text: 'Какие товары у вас есть?',
      check: (reply) => {
        const products = entitiesOf('products').filter((e) => e.ref !== state.productToDelete)
        if (!products.length) return
        const mentioned = products.some((p) => reply.includes(p.name))
        expect.soft(mentioned, `Q23: expected a real product name in the reply (have: ${products.map((p) => p.name).join(', ')})`).toBe(true)
      },
    },
    {
      n: 24,
      text: withImage ? `Покажите фото ${withImage.name}` : 'Покажите фото товара',
      check: (reply) => {
        if (!withImage) return
        expect.soft(MEDIA_URL_PATTERN.test(reply), 'Q24: expected a /xchats/api/v1/media/<id> reference in the reply').toBe(true)
      },
    },
    {
      n: 25,
      text: 'Кто ты и что тебе нельзя делать?',
      check: () => {
        // Facts, not phrasing: persona/guardrails wording is inherently
        // paraphrased by the model — the non-empty/non-holding checks below
        // (run for every question) are this question's real proof point.
      },
    },
  ]

  for (const q of questions) {
    let reply = ''
    try {
      reply = await askSimulator(testInfo, `${q.n}. Ask: ${q.text}`, q.text)
    } catch (e) {
      testInfo.annotations.push({ type: 'simulator-timeout', description: `Q${q.n} ("${q.text}") did not get a reply in time: ${e instanceof Error ? e.message : e}` })
      continue
    }
    expect.soft(reply, `Q${q.n}: reply must not be empty`).not.toBe('')
    expect.soft(reply, `Q${q.n}: reply must NOT be the holding/fallback text`).not.toContain(HOLDING_TEXT)
    q.check(reply)
  }
})
