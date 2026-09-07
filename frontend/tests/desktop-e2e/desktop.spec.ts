// The one scenario `make desktop-test-ui` runs: log in to the REAL packaged
// executable (built by wails build, or DESKTOP_E2E_BINARY), create a
// product, attach an uploaded image as both its featured image and gallery
// media, and prove the whole thing survives a reload AND a full stop/
// restart of the process against the same data root — the exact path that
// used to silently drop media on the live KB lane (see
// backend/internal/httpapi/kb_live.go's fix). Playwright drives the
// executable's XCHATS_DESKTOP_E2E_HTTP=1 loopback listener; the native
// Wails window, cookie jar, and realtime transport are covered by focused
// Go/frontend tests instead — see docs/desktop.md.
import { existsSync, readdirSync, statSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { test, expect } from '@playwright/test'
import { login } from '../e2e/helpers'
import { launchDesktopApp, type DesktopApp } from './harness/desktop-app'

// realOSDefaultDataDir mirrors backend/internal/appdirs.dataDirFor's
// unset-override branch, purely so step 10 can assert --data-dir actually
// redirected storage away from it — never to read or write anything there.
function realOSDefaultDataDir(): string | null {
  const home = os.homedir()
  if (!home) return null
  switch (process.platform) {
    case 'win32':
      return process.env.LOCALAPPDATA ? path.join(process.env.LOCALAPPDATA, 'xchats') : null
    case 'darwin':
      return path.join(home, 'Library', 'Application Support', 'xchats')
    default:
      return path.join(process.env.XDG_DATA_HOME || path.join(home, '.local', 'share'), 'xchats')
  }
}

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(__dirname, '..', '..', '..')
const FIXTURE_IMAGE = path.join(__dirname, 'fixtures', 'test-image.png')

// A distinctive, run-unique ref/name so the card is unambiguous to find and
// never collides with anything a previous (crashed) run might have left —
// this suite always starts from a brand-new, isolated data root anyway, but
// the uniqueness costs nothing and rules out any doubt when reading a
// failure's screenshot/trace.
const PRODUCT_REF = 'e2e-desktop-product'
const PRODUCT_NAME = `E2E Desktop Product ${Date.now()}`

test.describe.configure({ mode: 'serial' })

let app: DesktopApp

// Recorded before launch so step 10 can tell "already existed for an
// unrelated reason on this dev machine" apart from "this run created it" —
// best-effort, like kb-full-journey.spec.ts's own Docker-logs check: it
// only asserts the negative when it can be sure what "before" looked like.
let osDefaultDataDirPreexisted = true

test.beforeAll(async () => {
  const osDefault = realOSDefaultDataDir()
  osDefaultDataDirPreexisted = osDefault === null || existsSync(osDefault)

  app = await launchDesktopApp(REPO_ROOT, { logDir: path.join(REPO_ROOT, 'frontend', 'test-results', 'desktop-e2e-logs') })
  // Step 1: wait for executable and Wails readiness (both the backend
  // listener and the native window's OnStartup — see
  // internal/desktop/e2e_http.go's Readiness).
  await app.waitForReady()
})

test.afterAll(async () => {
  await app?.stop()
})

test('product media survives save, reload, and a full app restart', async ({ browser }) => {
  const context = await browser.newContext({ baseURL: app.baseURL })
  const page = await context.newPage()

  // Step 2: log in (the migration-seeded sentinel admin — see
  // tests/e2e/helpers.ts) and open the Knowledge Base.
  await login(page)
  await page.goto('/knowledge-base', { waitUntil: 'domcontentloaded' })
  await page.getByRole('button', { name: 'Товары', exact: true }).click()

  // Step 3: create a product and upload the checked-in image fixture.
  await page.getByRole('button', { name: 'Добавить товар' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  const textInputs = dialog.locator('input:not([type=file]):not([type=checkbox])')
  await textInputs.nth(0).fill(PRODUCT_REF) // Артикул (ref)
  await textInputs.nth(1).fill(PRODUCT_NAME) // Название (name)

  const fileInputs = dialog.locator('input[type=file]')
  await fileInputs.nth(0).setInputFiles(FIXTURE_IMAGE) // featured_image picker
  // uploadMaterial is a real network round trip; wait for the uploaded
  // thumbnail to actually render before treating the image as "attached".
  const featuredThumb = dialog.locator('img[src*="/kb/materials/"]')
  await expect(featuredThumb).toBeVisible({ timeout: 15_000 })

  // Step 4: attach the SAME uploaded image as gallery media too, via
  // "attach existing" rather than uploading a second copy. dialog has
  // TWO <select> elements at this point — availability_status (a plain
  // form field, always present) and the gallery MediaFieldPicker's
  // "attach existing" dropdown — so filter by option content rather than
  // assume this is the only one: only the gallery picker offers an option
  // naming the uploaded file.
  await dialog
    .locator('select')
    .filter({ hasText: 'test-image.png' })
    .selectOption({ label: 'test-image.png' })

  // Step 5: save and verify the card renders the image.
  await dialog.getByRole('button', { name: 'Сохранить' }).click()
  await expect(dialog).toBeHidden()
  const card = page.locator('[data-testid="kb-record"]').filter({ hasText: PRODUCT_REF })
  await expect(card).toBeVisible()
  // Two MediaStrip rows (featured_image, gallery_images) each render one
  // thumbnail for the same attached image.
  const cardImages = card.locator('img[src*="/kb/materials/"]')
  await expect(cardImages).toHaveCount(2)
  const mediaSrc = await cardImages.first().getAttribute('src')
  expect(mediaSrc).toBeTruthy()

  // Step 6: reload and verify it remains rendered.
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.getByRole('button', { name: 'Товары', exact: true }).click()
  const cardAfterReload = page.locator('[data-testid="kb-record"]').filter({ hasText: PRODUCT_REF })
  await expect(cardAfterReload).toBeVisible()
  await expect(cardAfterReload.locator('img[src*="/kb/materials/"]')).toHaveCount(2)

  await context.close()

  // Step 7: stop and restart the executable with the same temporary data
  // root — a fresh loopback port is chosen, so every subsequent request
  // uses app.baseURL again (it has already been reassigned by restart()).
  await app.restart()

  // Step 8: log in again and verify the product and image persisted.
  const context2 = await browser.newContext({ baseURL: app.baseURL })
  const page2 = await context2.newPage()
  await login(page2)
  await page2.goto('/knowledge-base', { waitUntil: 'domcontentloaded' })
  await page2.getByRole('button', { name: 'Товары', exact: true }).click()
  const cardAfterRestart = page2.locator('[data-testid="kb-record"]').filter({ hasText: PRODUCT_REF })
  await expect(cardAfterRestart).toBeVisible()
  const persistedImages = cardAfterRestart.locator('img[src*="/kb/materials/"]')
  await expect(persistedImages).toHaveCount(2)
  const persistedSrc = await persistedImages.first().getAttribute('src')
  expect(persistedSrc).toBeTruthy()

  // Step 9: verify the media endpoint returns the expected content type and
  // nonzero bytes — page2.request shares this context's session cookie, the
  // same way the <img> tag itself authenticated.
  const mediaResponse = await page2.request.get(persistedSrc as string)
  expect(mediaResponse.ok()).toBeTruthy()
  expect(mediaResponse.headers()['content-type']).toMatch(/^image\//)
  const body = await mediaResponse.body()
  expect(body.byteLength).toBeGreaterThan(0)

  await context2.close()

  // Step 10: verify database and blob files were created only under the
  // configured test data root.
  const dbPath = path.join(app.dataDir, 'data', 'xchats.db')
  expect(existsSync(dbPath)).toBe(true)
  expect(statSync(dbPath).size).toBeGreaterThan(0)
  const blobDir = path.join(app.dataDir, 'blobdata')
  expect(existsSync(blobDir)).toBe(true)
  expect(walkFiles(blobDir).length).toBeGreaterThan(0)
  // The negative half: --data-dir must have kept the OS default data
  // directory untouched. Only asserted when it's known not to have existed
  // before this run for some unrelated reason.
  const osDefault = realOSDefaultDataDir()
  if (osDefault && !osDefaultDataDirPreexisted) {
    expect(existsSync(osDefault)).toBe(false)
  }
})

function walkFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) out.push(...walkFiles(full))
    else out.push(full)
  }
  return out
}
