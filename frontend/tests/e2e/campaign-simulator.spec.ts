import { test, expect, type Page } from '@playwright/test'
import { login } from './helpers'

// campaign-simulator.spec.ts (CAM-18) drives an entire Simulator campaign
// exactly as an operator would, through the real 3-step wizard: Campaigns ->
// New campaign -> Audience (pick Simulator, paste two recipients) -> Message
// (compose with a {{variable}}) -> Schedule & Launch. It never touches
// WhatsApp, Telegram, or any external API — Simulator is a fully local,
// zero-cost channel, and this spec is the proof that a campaign sent
// through it runs the exact same runner/sender pipeline a live channel
// would (backend/internal/simulator + backend/internal/simreceipts): real
// recipient/message rows, real pacing and template rendering, real status
// transitions, and the sent message actually landing in Inbox.
//
// The audience deliberately includes ONE destination ending in the digit 0
// (a fixed, documented "always fails" convention — see
// backend/internal/simulator.Outcome) alongside a normal one, so this test
// asserts BOTH a successful send (through to delivered/read) and a genuine
// permanent failure without any randomness to make the run flaky.

const READ_NUMBER = '77011234567' // digit 7 -> Outcome.Read (sent, delivered, then read)
const FAIL_NUMBER = '77011234560' // digit 0 -> Outcome.Failed (Send() itself rejects)

function uniqueCampaignName(workerIndex: number): string {
  return `e2e-sim-${workerIndex}-${Date.now()}`
}

// pickSimulatorAccount opens the Step 1 account Select and chooses the
// entry badged "Тестовый режим" (Test Mode) — reka-ui's SelectItem, not a
// plain <select>, so this is a real click-open-click-option sequence.
async function pickSimulatorAccount(page: Page) {
  await page.getByRole('combobox').click()
  await page.getByRole('option', { name: /Simulator/ }).click()
}

// giveSimulatorAFastPace overrides the Simulator account's sending pace via
// PUT /accounts/:id/sending-limits — the same endpoint stores/campaigns.ts's
// saveSendingLimits calls (wired into the store for a future Sending-limits
// settings view, not yet mounted anywhere) — not a test-only backdoor.
// Simulator now defaults to the exact same whatsmeow-style pacing as a real
// WhatsApp account (90s +/-30s jitter between sends, 5/8/20/35/50
// rolling-window tiers — see backend/campaign.DefaultTiersFor/
// DefaultPacingFor), so that a campaign sent through it is throttled the
// way a live one would be. That realism is exactly what this spec's own
// two-recipient run doesn't need: without an override it would need
// minutes just to clear the second recipient's min-interval. An operator
// who wants a fast rehearsal run will have this same per-account override
// available once that settings view exists; this drives the API directly.
async function giveSimulatorAFastPace(page: Page) {
  const accountsRes = await page.request.get('/xchats/api/v1/accounts')
  const accountsBody = await accountsRes.json()
  const sim = accountsBody.payload.items.find((a: { channel: string }) => a.channel === 'simulator')
  if (!sim) throw new Error('expected a simulator account to always exist (GetOrCreateSimulatorAccount)')
  const res = await page.request.put(`/xchats/api/v1/accounts/${sim.id}/sending-limits`, {
    data: {
      limit_mode: 'custom',
      min_interval_seconds: 1,
      jitter_seconds: 0,
      paused: false,
      tiers: [{ window_seconds: 60, max_sends: 50 }],
      windows: [],
    },
  })
  if (!res.ok()) throw new Error(`sending-limits override failed: ${res.status()} ${await res.text()}`)
}

test('a campaign sent through Simulator runs the real pipeline: audience, message, launch, delivery, and Inbox', async ({ page }, testInfo) => {
  const name = uniqueCampaignName(testInfo.workerIndex)

  await login(page)
  await giveSimulatorAFastPace(page)
  await page.goto('/campaigns')
  await page.getByRole('button', { name: 'Новая рассылка' }).click()
  await expect(page).toHaveURL('/campaigns/new')

  // --- Step 1: Audience (Who) --------------------------------------------
  await expect(page.getByTestId('wizard-step-audience')).toHaveClass(/bg-primary/)
  await page.getByPlaceholder('Например, Летняя акция').fill(name)
  await pickSimulatorAccount(page)
  await expect(page.getByTestId('simulator-notice')).toBeVisible()

  await page.getByTestId('paste-recipients').fill(`phone,name\n${READ_NUMBER},Aigul\n${FAIL_NUMBER},Bota`)
  // CAM-04's own debounced auto-check — wait for the real reachability
  // preview (a real POST /campaigns/:id/preview round trip) rather than a
  // fixed sleep.
  await expect(page.getByText('корректных: 2')).toBeVisible({ timeout: 10_000 })

  await page.getByRole('button', { name: 'Продолжить к сообщению →' }).click()
  await expect(page.getByTestId('wizard-step-message')).toHaveClass(/bg-primary/)

  // --- Step 2: Message (What) --------------------------------------------
  const messageBox = page.getByTestId('message-textarea')
  await messageBox.fill('Привет, ')
  await page.getByTestId('insert-var-name').click()
  await expect(messageBox).toHaveValue('Привет, {{name}}')
  await messageBox.type('! Ваш промокод: SUMMER2026')

  // The inline {{ autocomplete menu.
  await messageBox.type(' {{ph')
  await expect(page.getByTestId('var-autocomplete-menu')).toBeVisible()
  await page.getByTestId('var-autocomplete-item').filter({ hasText: '{{phone}}' }).click()
  await expect(messageBox).toHaveValue('Привет, {{name}}! Ваш промокод: SUMMER2026 {{phone}}')

  // Sample-value preview (CAM-03) — never the raw template.
  await page.getByTestId('toggle-message-preview').click()
  await expect(page.getByTestId('message-preview-bubble')).toHaveText('Привет, Aigul! Ваш промокод: SUMMER2026 77011234567')

  await page.getByRole('button', { name: 'Продолжить к расписанию →' }).click()
  await expect(page.getByTestId('wizard-step-schedule')).toHaveClass(/bg-primary/)

  // --- Step 3: Schedule & Launch (When) -----------------------------------
  await expect(page.getByTestId('summary-reachable')).toContainText('2')
  await page.getByTestId('launch-campaign').click()

  // --- Campaign detail: launched through the real runner -----------------
  await expect(page).toHaveURL(/\/campaigns\/[^/]+$/)
  await expect(page.getByRole('heading', { name })).toBeVisible()
  await expect(page.getByText('Завершена')).toBeVisible({ timeout: 15_000 }) // status: completed

  await page.getByRole('tab', { name: 'Получатели' }).click()
  await expect(page.getByText(READ_NUMBER)).toBeVisible()
  await expect(page.getByText(FAIL_NUMBER)).toBeVisible()

  // The read-path recipient: sent, then (async, via ReceiptSimulator)
  // delivered, then read — the exact store.AdvanceDeliveryState path a
  // real channel's own delivery-receipt webhook drives.
  await expect(page.getByTestId('recipient-delivery-state')).toHaveText('Прочитано', { timeout: 10_000 })

  // The fail-path recipient: permanently failed, with the deterministic
  // reason messaging.ErrRecipientUnreachable carries, never retried.
  await expect(page.getByText('messaging: recipient is permanently unreachable')).toBeVisible()
  await expect(page.getByText('Ошибка').first()).toBeVisible()

  // --- Inbox/CRM: the simulated message actually landed there -----------
  // A cold-sent campaign chat is deliberately chat_state='campaign' —
  // hidden from the default Inbox list until the recipient replies (so a
  // one-way broadcast never floods the list of real conversations), so
  // "does it appear in Inbox/CRM" is checked the same way the Inbox itself
  // would once that chat IS opened: through the exact GET .../chats/:id/
  // messages endpoint ChatThread.vue reads from — not by browsing the
  // default list, which this specific chat is intentionally excluded from.
  const campaignIdMatch = page.url().match(/\/campaigns\/([^/]+)/)
  if (!campaignIdMatch) throw new Error('expected to still be on the campaign detail URL')
  const campaignId = campaignIdMatch[1]
  const recipientsRes = await page.request.get(`/xchats/api/v1/campaigns/${campaignId}/recipients?page=1&page_size=50`)
  const recipientsBody = await recipientsRes.json()
  const readRecipient = recipientsBody.payload.items.find((r: { normalized_identity: string }) => r.normalized_identity === READ_NUMBER)
  expect(readRecipient?.chat_id).toBeTruthy()

  const messagesRes = await page.request.get(`/xchats/api/v1/chats/${readRecipient.chat_id}/messages?limit=80`)
  const messagesBody = await messagesRes.json()
  const sentMessage = messagesBody.payload.items.find((m: { direction: string }) => m.direction === 'out')
  expect(sentMessage?.content).toBe('Привет, Aigul! Ваш промокод: SUMMER2026 77011234567')
  expect(sentMessage?.status).toBe('read')

  // Best-effort cleanup: a campaign that already delivered is refused by
  // DELETE (its send-ledger rows back the account rate limiter) — this is
  // expected to fail here, tolerated like every other best-effort cleanup
  // in this suite (see kb-draft.spec.ts's own cleanupTopic).
  await page.request.delete(`/xchats/api/v1/campaigns/${campaignId}`).catch(() => {})
})
