import fs from 'node:fs'
import path from 'node:path'
import { type Page, type TestInfo, test } from '@playwright/test'
import { REPO_ROOT } from './env'

const SCREENS_ROOT = path.resolve(REPO_ROOT, 'frontend/test-screens')

// One timestamped directory per test run (module-level: shared by every
// test() in the journey file, since Playwright runs them in one worker
// process — see playwright.config.ts's workers:1 comment) — this is what
// "the screenshot trail" means: a single, numbered, chronological set of
// PNGs a human can click through top to bottom.
const RUN_STAMP = new Date().toISOString().replace(/[:.]/g, '-')
const RUN_DIR = path.join(SCREENS_ROOT, RUN_STAMP)
fs.mkdirSync(RUN_DIR, { recursive: true })

interface ShotRecord {
  seq: number
  file: string
  name: string
  testTitle: string
}
const manifest: ShotRecord[] = []
let seq = 0

function writeIndex(): void {
  const rows = manifest
    .map(
      (r) => `
      <figure>
        <figcaption><strong>${r.seq.toString().padStart(3, '0')}</strong> — ${escapeHtml(r.testTitle)}<br /><span>${escapeHtml(r.name)}</span></figcaption>
        <a href="${encodeURIComponent(r.file)}" target="_blank"><img src="${encodeURIComponent(r.file)}" loading="lazy" /></a>
      </figure>`,
    )
    .join('\n')
  const html = `<!doctype html>
<html><head><meta charset="utf-8" /><title>KB import journey — ${RUN_STAMP}</title>
<style>
  body { font-family: -apple-system, system-ui, sans-serif; background: #0b0d12; color: #e6e8ec; margin: 0; padding: 24px; }
  h1 { font-size: 16px; font-weight: 600; opacity: .8; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 20px; margin-top: 20px; }
  figure { margin: 0; background: #14171d; border: 1px solid #262b35; border-radius: 10px; overflow: hidden; }
  figcaption { padding: 10px 12px; font-size: 12.5px; line-height: 1.5; border-bottom: 1px solid #262b35; }
  figcaption span { color: #9aa3b2; }
  img { width: 100%; display: block; }
</style></head>
<body>
  <h1>KB import &amp; AI assistant — full user journey — ${RUN_STAMP} (${manifest.length} screenshots)</h1>
  <div class="grid">${rows}</div>
</body></html>`
  fs.writeFileSync(path.join(RUN_DIR, 'index.html'), html)

  // Best-effort "latest" pointer — a plain symlink so `test-screens/latest/
  // index.html` always resolves to the newest run without a copy. Not
  // load-bearing: if the platform refuses symlinks, the timestamped
  // directory printed to the console at the end is still the source of truth.
  const latest = path.join(SCREENS_ROOT, 'latest')
  try {
    fs.rmSync(latest, { force: true })
    fs.symlinkSync(RUN_DIR, latest, 'dir')
  } catch {
    // best-effort — see comment above
  }
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!)
}

// shot captures a full-page screenshot, saves it under this run's numbered
// sequence, attaches it to the Playwright HTML report, and appends it to the
// run's own test-screens/<run>/index.html gallery.
export async function shot(page: Page, testInfo: TestInfo, name: string): Promise<void> {
  seq += 1
  const safeName = name
    .replace(/[^a-zA-Z0-9._ -]+/g, '')
    .trim()
    .replace(/\s+/g, '-')
    .slice(0, 90)
  const file = `${String(seq).padStart(3, '0')}-${safeName || 'step'}.png`
  const buffer = await page.screenshot({ path: path.join(RUN_DIR, file), fullPage: true })
  await testInfo.attach(name, { body: buffer, contentType: 'image/png' })
  manifest.push({ seq, file, name, testTitle: testInfo.title })
  writeIndex()
}

// step wraps one UI action in test.step() (so it renders as its own node in
// the Playwright report/trace) and screenshots the result immediately after
// — every state-changing action in the journey goes through this.
export async function step<T>(testInfo: TestInfo, page: Page, name: string, fn: () => Promise<T>): Promise<T> {
  return test.step(name, async () => {
    const result = await fn()
    await shot(page, testInfo, name)
    return result
  })
}

export function runDir(): string {
  return RUN_DIR
}
