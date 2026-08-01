import { test, expect, type Page } from '@playwright/test'
import { login } from './helpers'

// Every test below runs against the ONE shared backend draft/state
// (playwright.config.ts: fullyParallel = false) — so no test may assume the
// draft or live KB starts empty, and every record this file creates uses its
// own unique natural key (never a fixed/seeded one) and is cleaned up before
// the test ends, so re-running this file never accumulates leftovers.
function uniqueSlug(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
}

// Locates a *Record.vue card (Черновик's ChangeList or База знаний's
// RecordList — both wrap RecordShell.vue) by its natural-key <code> chip,
// which is always rendered with the exact slug/ref text — the one substring
// on the page guaranteed unique to this test's own record.
function cardFor(page: Page, naturalKey: string) {
  return page.locator('code', { hasText: naturalKey }).locator('xpath=ancestor::div[contains(@class, "rounded-lg")][1]')
}

// Switches page via the persistent NavRail link (a real client-side Vue
// Router navigation) rather than page.goto (a hard reload) — besides being
// how an actual user moves between Черновик and База знаний, repeated hard
// reloads round-trip the whole SPA bundle again and were slow enough in this
// sandbox to blow past the test timeout on their own.
async function goNav(page: Page, href: '/playground' | '/knowledge-base') {
  await page.locator(`nav a[href="${href}"]`).click()
}

test('login → chatboard (nav rail + chat list)', async ({ page }) => {
  await login(page)
  await expect(page.getByText('xchats')).toBeVisible()
})

test('NavRail: Черновик and База знаний destinations, with the redesigned labels', async ({ page }) => {
  await login(page)
  // The rail link is icon-only (no accessible name of its own) — target it
  // by href, then read the label off reka-ui's TooltipContent (plain text,
  // no tooltip role) once hover triggers it.
  await page.locator('nav a[href="/playground"]').hover()
  await expect(page.getByText('Черновик', { exact: true })).toBeVisible()

  await goNav(page, '/playground')
  await expect(page.getByRole('heading', { name: 'Черновик', exact: true })).toBeVisible()

  await goNav(page, '/knowledge-base')
  await expect(page.getByRole('heading', { name: 'База знаний' })).toBeVisible()
})

// Черновик (§ redesign) is review-only: no Add button and no create form
// anywhere on the page, regardless of what happens to be pending right now —
// every write lives on База знаний instead.
test('Черновик is review-only: no Add buttons or create forms anywhere on the page', async ({ page }) => {
  await login(page)
  await page.goto('/playground')
  await expect(page.getByRole('heading', { name: 'Черновик', exact: true })).toBeVisible()

  await expect(page.getByRole('button', { name: /Добавить/ })).toHaveCount(0)
  await expect(page.locator('input')).toHaveCount(0)
  await expect(page.locator('textarea')).toHaveCount(0)
})

// The core of the redesign: База знаний is the one creation surface — every
// write there STAGES into the draft, never the live tables directly — so the
// published list must stay visibly unchanged, and the change must surface as
// its own pending card back on Черновик.
test('База знаний stages a new topic into Черновик without touching the published list', async ({ page }) => {
  await login(page)
  const slug = uniqueSlug('e2e-cancel')
  const title = 'E2E cancel-flow topic'

  await page.goto('/knowledge-base')
  await page.getByRole('button', { name: 'Темы' }).click()
  await page.getByRole('button', { name: 'Добавить тему' }).click()
  await expect(page.getByText('Новая тема')).toBeVisible()
  await page.getByPlaceholder('slug (например, tariffs)').fill(slug)
  await page.getByPlaceholder('Название').fill(title)
  await page.getByRole('button', { name: 'Сохранить' }).click()

  // Staged, not published: the banner names Темы, and the published list on
  // this same tab never shows the new row.
  await expect(page.getByText('Изменение добавлено в Черновик: Темы', { exact: false })).toBeVisible()
  await expect(page.getByText(title)).toHaveCount(0)

  try {
    await page.getByRole('link', { name: 'Перейти в Черновик' }).click()
    await expect(page.getByRole('heading', { name: 'Черновик', exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Темы' }).click()

    const card = cardFor(page, slug)
    await expect(card).toContainText(title)
    await expect(card.getByRole('button', { name: 'Изменить' })).toBeVisible()
    await expect(card.getByRole('button', { name: 'Опубликовать', exact: true })).toBeVisible()
  } finally {
    // Cancel (never publish) — the cleanest possible cleanup: the live KB
    // was never touched, so cancelling the pending add leaves zero residue.
    await cardFor(page, slug).getByRole('button', { name: 'Отменить изменение' }).click()
  }
  await expect(page.locator('code', { hasText: slug })).toHaveCount(0)
})

// Publishing a staged change is the other half of the loop this redesign is
// built around: Опубликовать on a single card must actually move the row
// into the live table the rest of база знаний reads.
test('publishing a staged topic moves it into the live knowledge base', async ({ page }) => {
  // This test's own full create → publish → verify-live → delete → publish-
  // delete lifecycle does strictly more round trips than any other test here
  // (which already run close to the default 30s budget) — give it more room
  // rather than raising the shared config for every other test.
  test.setTimeout(60_000)
  await login(page)
  const slug = uniqueSlug('e2e-publish')
  const title = 'E2E publish-flow topic'

  await page.goto('/knowledge-base')
  await page.getByRole('button', { name: 'Темы' }).click()
  await page.getByRole('button', { name: 'Добавить тему' }).click()
  await page.getByPlaceholder('slug (например, tariffs)').fill(slug)
  await page.getByPlaceholder('Название').fill(title)
  await page.getByRole('button', { name: 'Сохранить' }).click()
  await expect(page.getByText('Изменение добавлено в Черновик: Темы', { exact: false })).toBeVisible()

  try {
    await goNav(page, '/playground')
    await page.getByRole('button', { name: 'Темы' }).click()
    await cardFor(page, slug).getByRole('button', { name: 'Опубликовать', exact: true }).click()
    await expect(page.locator('code', { hasText: slug })).toHaveCount(0)

    await goNav(page, '/knowledge-base')
    await page.getByRole('button', { name: 'Темы' }).click()
    await expect(cardFor(page, slug)).toContainText(title)
  } finally {
    // The row is now genuinely live — removing it goes through the same
    // stage-then-publish path as any other БЗ write (§ redesign: БЗ never
    // deletes a live row directly).
    await goNav(page, '/knowledge-base')
    await page.getByRole('button', { name: 'Темы' }).click()
    const liveCard = cardFor(page, slug)
    // .count() resolves instantly against whatever's rendered right now — no
    // retrying — so right after a nav/fetch it can read 0 before the list
    // finishes loading and silently skip cleanup. waitFor actually polls.
    const stillLive = await liveCard
      .waitFor({ state: 'visible', timeout: 5_000 })
      .then(() => true)
      .catch(() => false)
    if (stillLive) {
      await liveCard.getByRole('button', { name: 'Удалить' }).click()
      await page.getByRole('button', { name: 'Удалить', exact: true }).click() // ConfirmDeleteDialog
      await goNav(page, '/playground')
      await page.getByRole('button', { name: 'Темы' }).click()
      await cardFor(page, slug).getByRole('button', { name: 'Опубликовать', exact: true }).click()
    }
  }
  // Verify cleanup actually landed rather than assuming the finally block's
  // best-effort clicks succeeded.
  await goNav(page, '/knowledge-base')
  await page.getByRole('button', { name: 'Темы' }).click()
  await expect(page.locator('code', { hasText: slug })).toHaveCount(0)
})
