import { test, expect, type Page } from '@playwright/test'
import { customAlphabet } from 'nanoid'
import { login } from './helpers'

// A topic/product/tariff/zone ref is a "natural ref" the prompt renderer
// addresses fact tokens by (backend/aiprompt/catalog.go's refPattern);
// nanoid's default alphabet includes uppercase letters, which that pattern
// rejects outright ("aiprompt: invalid natural ref") the moment the row
// goes live and something tries to render a prompt from it — lowercase
// digits+letters only, matching refPattern's [a-z0-9_-]*.
const lowerId = customAlphabet('abcdefghijklmnopqrstuvwxyz0123456789', 6)

// kb-draft.spec.ts exercises the Черновик/Знаний база redesign end to end:
// staging from /knowledge-base never touches live, publishing from
// /playground does, and cancelling a staged edit leaves live untouched.
//
// Isolation (kb-draft-review-boundaries's own standard, spec §3.3):
//   - every test owns a UNIQUE natural key, so tests never collide with each
//     other or with seed data;
//   - cleanup runs in try/finally and tolerates partial failure — each step
//     is independently best-effort, so a test that died halfway still
//     leaves the org clean for the next run;
//   - nothing here asserts the page is GLOBALLY empty; only that THIS
//     test's own key behaves as expected.

function uniqueSlug(workerIndex: number, tag: string): string {
  return `e2e-${tag}-${workerIndex}-${Date.now()}-${lowerId()}`
}

// recordCard finds a kb/records/*Record.vue card by a text it contains.
// Two reasons NOT to use a bare `page.getByText(text)` for "is this record
// present/absent" checks on /knowledge-base:
//   1. A plain `page.locator('div', { has: ... }).first()` resolves to the
//      OUTERMOST matching div (every ancestor up to the page wrapper also
//      "has" the text as a descendant), not the card itself.
//   2. Знаний база's tabs use v-show, so EVERY tab's content — including
//      PromptTab's own raw-template dump of the assembled prompt — stays
//      mounted in the DOM even while hidden. Once a topic is live, its
//      title legitimately also appears inside that dump (a `<table>`, not
//      a card), so `page.getByText(title)` can resolve to TWO real,
//      unrelated elements — a strict-mode violation, not a bug in the app.
// RecordShell.vue's root div carries data-testid="kb-record" for exactly
// this — it previously used a 4-class CSS selector
// (div.rounded-lg.border-border.bg-card.p-4) that happened to ALSO match
// KbImportRunStatus.vue's root (Черновик) and the Файлы/materials row
// (Знаний база), both unrelated to this component and both sharing that
// same incidental Tailwind class combination — a second strict-mode
// violation waiting for a search text that happened to appear in either.
function recordCard(page: Page, text: string) {
  return page.locator('[data-testid="kb-record"]').filter({ hasText: text })
}

// cleanupTopic best-effort undoes everything a test could have done to one
// topic key: cancel any pending add/edit, then stage-and-publish a deletion
// in case the test got as far as making it live. Each step is independently
// wrapped so one failing (e.g. nothing was ever staged) never blocks the rest.
async function cleanupTopic(page: Page, slug: string) {
  const attempt = async (fn: () => Promise<unknown>) => {
    try {
      await fn()
    } catch {
      // best effort — see this function's own doc comment
    }
  }
  await attempt(() => page.request.delete(`/xchats/api/v1/playground/draft/changes/topics/${slug}`))
  await attempt(() => page.request.delete(`/xchats/api/v1/playground/draft/topics/${slug}`))
  await attempt(() => page.request.post(`/xchats/api/v1/playground/draft/approve/topics/${slug}`))
}

// No describe.configure({mode:'serial'}) here on purpose: each test owns a
// fully unique natural key (never colliding with the other), so unlike
// smoke.spec.ts these two do not depend on execution order — only on
// playwright.config.ts's global fullyParallel:false, which keeps the whole
// suite from racing the SAME shared backend draft at once.

test('staging a topic from Знаний база never touches live; publishing from Черновик does', async ({ page }, testInfo) => {
  const slug = uniqueSlug(testInfo.workerIndex, 'stage')
  const title = `Тестовая тема ${slug}`

  try {
    await login(page)

    // 1. Stage the topic from /knowledge-base.
    await page.goto('/knowledge-base')
    await page.getByRole('button', { name: 'Темы' }).click()
    await page.getByRole('button', { name: 'Добавить тему' }).click()
    await page.getByPlaceholder('напр. tariffs').fill(slug)
    await page.getByRole('dialog').getByRole('textbox').nth(1).fill(title)
    await page.getByRole('button', { name: 'Сохранить' }).click()

    // 2. The banner confirms the stage, and the published list still lacks it.
    await expect(page.getByText('Изменение добавлено в Черновик')).toBeVisible()
    await expect(recordCard(page, title)).toHaveCount(0)

    // 3. Черновик shows it exactly once, as Новый, under a Темы tab.
    await page.goto('/playground')
    await expect(page.getByRole('button', { name: /^Темы/ })).toBeVisible()
    await page.getByRole('button', { name: /^Темы/ }).click()
    const card = recordCard(page, title)
    await expect(card.getByText('Новый')).toBeVisible()

    // 4. Publish it — it appears in /kb, its Черновик card is gone.
    await card.getByRole('button', { name: 'Опубликовать', exact: true }).click()
    await expect(recordCard(page, title)).toHaveCount(0)

    await page.goto('/knowledge-base')
    await page.getByRole('button', { name: 'Темы' }).click()
    await expect(recordCard(page, title)).toBeVisible()
  } finally {
    await cleanupTopic(page, slug)
  }
})

test('cancelling a staged edit leaves live untouched and clears the Черновик card', async ({ page }, testInfo) => {
  const slug = uniqueSlug(testInfo.workerIndex, 'cancel')
  const originalTitle = `Оригинал ${slug}`
  const editedTitle = `Отредактировано ${slug}`

  try {
    await login(page)

    // Setup via API: a published topic this test owns.
    await page.request.post('/xchats/api/v1/playground/draft/topics', {
      data: { slug, title: originalTitle, body_md: 'Текст.' },
    })
    await page.request.post(`/xchats/api/v1/playground/draft/approve/topics/${slug}`)

    // Edit it from /knowledge-base.
    await page.goto('/knowledge-base')
    await page.getByRole('button', { name: 'Темы' }).click()
    const liveCard = recordCard(page, originalTitle)
    await liveCard.getByRole('button', { name: 'Изменить' }).click()
    const titleInput = page.getByRole('dialog').getByRole('textbox').nth(1)
    await titleInput.fill(editedTitle)
    await page.getByRole('button', { name: 'Сохранить' }).click()
    await expect(page.getByText('Изменение добавлено в Черновик')).toBeVisible()

    // Черновик shows the pending edit with its "Было: …" note.
    await page.goto('/playground')
    await page.getByRole('button', { name: /^Темы/ }).click()
    const draftCard = recordCard(page, editedTitle)
    await expect(draftCard.getByText('Изменён')).toBeVisible()
    await expect(draftCard.getByText(originalTitle)).toBeVisible() // FieldDiffNote's "Было: …"

    // Cancel it — live stays exactly as it was, the card disappears.
    await draftCard.getByRole('button', { name: 'Отменить изменение' }).click()
    await expect(recordCard(page, editedTitle)).toHaveCount(0)

    await page.goto('/knowledge-base')
    await page.getByRole('button', { name: 'Темы' }).click()
    await expect(recordCard(page, originalTitle)).toBeVisible()
    await expect(recordCard(page, editedTitle)).toHaveCount(0)
  } finally {
    await cleanupTopic(page, slug)
  }
})
