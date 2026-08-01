import { test, expect } from '@playwright/test'
import {
  mountWidget,
  setToolResponse,
  toolCalls,
  receivedUiInitialize,
  sendToWidget,
  postFromImpostorWindow,
  toolResult,
  UPLOAD_URL,
} from './mockHost'

// kb-manager.spec.ts exercises the KB Manager MCP widget (the real,
// production backend/internal/mcpserver/widget/kb-manager.html) inside a
// MOCK MCP HOST — a parent page implementing just enough of the generic
// postMessage bridge to stand in for a real ChatGPT/Claude runtime (plan
// Task 18). This tests the widget's OWN behavior at the host boundary; it
// does not exercise the real Xchats backend at all (no DATABASE_URL/backend
// process needed) and is independent of the smoke.spec.ts suite.

test('sends ui/initialize during connect', async ({ page }) => {
  await mountWidget(page)
  await expect.poll(() => receivedUiInitialize(page)).toBe(true)
})

test('a tool-result notification for a mutating tool refreshes the view', async ({ page }) => {
  await mountWidget(page, {
    kb_summary: [toolResult({ draft_version: 1, items: [{ type: 'topic', key: 'first', title: 'First topic', exists_in_live: true, exists_in_draft: false, state: 'published' }] })],
  })
  const widget = await pageFrame(page)
  await expect(widget.getByText('First topic')).toBeVisible()

  // The model calls kb_topic_upsert directly (not through this widget); the
  // host relays the standard tool-result notification. The next kb_summary
  // response now reflects the change.
  await setToolResponse(page, 'kb_summary', toolResult({
    draft_version: 2,
    items: [{ type: 'topic', key: 'second', title: 'Second topic (after model write)', exists_in_live: false, exists_in_draft: true, state: 'new' }],
  }))
  await sendToWidget(page, { jsonrpc: '2.0', method: 'notifications/tools/result', params: { name: 'kb_topic_upsert', result: {} } })

  await expect(widget.getByText('Second topic (after model write)')).toBeVisible()
})

test('clicking the Live tab issues a kb_read call with source=live', async ({ page }) => {
  await mountWidget(page, {
    kb_read: [toolResult({ items: [] })],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="live"]').click()

  await expect.poll(async () => {
    const calls = await toolCalls(page)
    return calls.some((c) => c.name === 'kb_read' && (c.arguments as any).source === 'live')
  }).toBe(true)
})

test('clicking a type filter chip includes that type in the next call', async ({ page }) => {
  await mountWidget(page)
  const widget = await pageFrame(page)
  await widget.locator('[data-type-filter="product"]').click()

  await expect.poll(async () => {
    const calls = await toolCalls(page)
    const last = calls[calls.length - 1]
    return last && last.name === 'kb_summary' && JSON.stringify((last.arguments as any).types) === JSON.stringify(['product'])
  }).toBe(true)
})

test('pagination follows next_cursor instead of truncating at 100', async ({ page }) => {
  const page1Items = Array.from({ length: 100 }, (_, i) => ({
    type: 'topic', key: `topic-${i}`, title: `Topic ${i}`, exists_in_live: true, exists_in_draft: false, state: 'published',
  }))
  const page2Items = Array.from({ length: 20 }, (_, i) => ({
    type: 'topic', key: `topic-${100 + i}`, title: `Topic ${100 + i}`, exists_in_live: true, exists_in_draft: false, state: 'published',
  }))
  await mountWidget(page, {
    kb_summary: [
      toolResult({ draft_version: 1, items: page1Items, next_cursor: '100' }),
      toolResult({ draft_version: 1, items: page2Items }),
    ],
  })
  const widget = await pageFrame(page)
  await expect(widget.getByText('Topic 0')).toBeVisible()
  await expect(widget.getByText('Topic 119')).toBeVisible()

  const calls = (await toolCalls(page)).filter((c) => c.name === 'kb_summary')
  expect(calls.length).toBe(2)
  expect((calls[0].arguments as any).cursor).toBeUndefined()
  expect((calls[1].arguments as any).cursor).toBe('100')
})

test('upload flow: progress, success, error, and retry', async ({ page }) => {
  await mountWidget(page, {
    kb_media_upload: [toolResult({
      material_id: 'mat-1', upload_url: UPLOAD_URL, upload_method: 'PUT',
      upload_headers: { 'Content-Type': 'text/plain' }, expires_at: new Date(Date.now() + 900_000).toISOString(),
      max_size_bytes: 1024, processing_status: 'uploaded',
    })],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()

  // First PUT fails.
  let putCount = 0
  await page.route(UPLOAD_URL, (route) => {
    putCount += 1
    if (putCount === 1) return route.fulfill({ status: 500, body: 'fail' })
    return route.fulfill({ status: 200, body: 'ok' })
  })

  // Selecting a file auto-stages AND auto-uploads (handleFile) — there is no
  // separate "click to start" step on this path (#do-upload is only shown
  // when a host seeds an already-staged upload via window.openai.toolOutput).
  await widget.locator('#file-input').setInputFiles({ name: 'note.txt', mimeType: 'text/plain', buffer: Buffer.from('hello world') })

  await expect(widget.locator('.error-box')).toBeVisible()
  const retryButton = widget.locator('#retry-upload')
  await expect(retryButton).toBeVisible()

  // Retry succeeds without re-staging (no second kb_media_upload call).
  await retryButton.click()
  await expect(widget.getByText('Файл загружен')).toBeVisible()
  const uploadCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_media_upload')
  expect(uploadCalls.length).toBe(1)
  expect(putCount).toBe(2)
})

// Reproduces a real failure: a user uploaded a file, then in the attach form
// picked type "Тариф" while the key they entered was actually a TOPIC slug.
// The typed upserts are upserts, so a key matching nothing fell through to
// "create" and the backend answered `pricing_type is required to create this
// record` — an error about an unrelated required field, for what the user
// experienced as "attach this image". Attaching must never create a record.
test('attaching to a key that does not exist for the chosen type is refused clearly', async ({ page }) => {
  await mountWidget(page, {
    kb_media_upload: [toolResult({
      material_id: 'mat-1', upload_url: UPLOAD_URL, upload_method: 'PUT',
      upload_headers: { 'Content-Type': 'text/plain' }, expires_at: new Date(Date.now() + 900_000).toISOString(),
      max_size_bytes: 1024, processing_status: 'uploaded',
    })],
    // No tariff has this key — kb_read returns an empty page, exactly as the
    // real backend does for a topic slug queried as a tariff.
    kb_read: [toolResult({ items: [] })],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))
  await widget.locator('#file-input').setInputFiles({ name: 'note.txt', mimeType: 'text/plain', buffer: Buffer.from('hi') })
  await expect(widget.getByText('Файл загружен')).toBeVisible()

  await widget.locator('#attach-type').selectOption('tariff')
  await widget.locator('#attach-key').fill('kak-dobavit-kassira-v-kaspi-pay')
  await widget.locator('#do-attach').click()

  await expect(widget.locator('.error-box')).toContainText('Запись не найдена')
  // The upsert must NOT have been attempted — that is what produced the
  // confusing "pricing_type is required" before.
  const upsertCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_tariff_upsert')
  expect(upsertCalls.length).toBe(0)
})

test('the review link falls back to the plain frontend URL when no reviewUrl is supplied', async ({ page }) => {
  await mountWidget(page)
  const widget = await pageFrame(page)
  const link = widget.getByRole('link', { name: 'Просмотреть и опубликовать в Xchats' })
  await expect(link).toHaveAttribute('href', 'https://xchats.test/playground')
})

test('the review link prefers a per-call reviewUrl over the plain frontend URL', async ({ page }) => {
  await mountWidget(page, {
    kb_summary: [toolResult({ draft_version: 1, items: [] }, { 'xchats/reviewUrl': 'https://xchats.test/playground/review-handoff?token=abc' })],
  })
  const widget = await pageFrame(page)
  const link = widget.getByRole('link', { name: 'Просмотреть и опубликовать в Xchats' })
  await expect(link).toHaveAttribute('href', 'https://xchats.test/playground/review-handoff?token=abc')
})

test('messages from a window that is not the real host are ignored', async ({ page }) => {
  await mountWidget(page, {
    kb_summary: [toolResult({ draft_version: 1, items: [{ type: 'topic', key: 'a', title: 'Original', exists_in_live: true, exists_in_draft: false, state: 'published' }] })],
  })
  const widget = await pageFrame(page)
  await expect(widget.getByText('Original')).toBeVisible()
  const callsBefore = (await toolCalls(page)).length

  await setToolResponse(page, 'kb_summary', toolResult({ draft_version: 2, items: [{ type: 'topic', key: 'b', title: 'Should not appear', exists_in_live: false, exists_in_draft: true, state: 'new' }] }))
  await postFromImpostorWindow(page, { jsonrpc: '2.0', method: 'notifications/tools/result', params: { name: 'kb_topic_upsert', result: {} } })

  // Give a real (trusted) message a moment to have propagated if the check
  // were broken, then assert nothing changed.
  await page.waitForTimeout(300)
  await expect(widget.getByText('Original')).toBeVisible()
  await expect(widget.getByText('Should not appear')).toHaveCount(0)
  expect((await toolCalls(page)).length).toBe(callsBefore)
})

async function pageFrame(page: import('@playwright/test').Page) {
  const widget = page.frameLocator('#widget')
  await expect(widget.locator('nav.tabs')).toBeVisible()
  return widget
}
