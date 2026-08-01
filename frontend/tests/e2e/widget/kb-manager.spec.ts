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

// stagedUpload builds a kb_media_upload structuredContent response — the
// same shape media_upload.go's handleKBMediaUpload returns.
function stagedUpload(materialId: string, opts: { expiresInMs?: number; contentType?: string } = {}) {
  return toolResult({
    material_id: materialId,
    upload_url: UPLOAD_URL,
    upload_method: 'PUT',
    upload_headers: { 'Content-Type': opts.contentType ?? 'image/png' },
    expires_at: new Date(Date.now() + (opts.expiresInMs ?? 900_000)).toISOString(),
    max_size_bytes: 1024,
    processing_status: 'uploaded',
  })
}

// summaryWith builds a kb_summary response carrying one record per (type,
// key, title) triple, all state="published" unless overridden — the shape
// the widget's attach-target picker (loadAttachRecords) reads.
function summaryWith(items: Array<{ type: string; key: string; title: string; state?: string }>) {
  return toolResult({
    draft_version: 1,
    items: items.map((it) => ({
      type: it.type,
      key: it.key,
      title: it.title,
      exists_in_live: true,
      exists_in_draft: false,
      state: it.state ?? 'published',
    })),
  })
}

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

test('selecting a file immediately stages and uploads it, with real progress', async ({ page }) => {
  await mountWidget(page, { kb_media_upload: [stagedUpload('mat-1')] })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()

  // The PUT resolves slowly enough for the progress bar to have a moment to
  // render before completing.
  await page.route(UPLOAD_URL, async (route) => {
    await new Promise((r) => setTimeout(r, 100))
    return route.fulfill({ status: 200, body: 'ok' })
  })

  // Selecting a file auto-stages AND auto-uploads (handleFile) — there is no
  // separate "click to start" step, and no client-side hashing delay before
  // the kb_media_upload call fires.
  await widget.locator('#file-input').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from('fake png bytes') })

  await expect(widget.locator('.progress-bar')).toBeVisible()
  await expect(widget.getByText('Файл загружен')).toBeVisible()

  const uploadCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_media_upload')
  expect(uploadCalls.length).toBe(1)
  // No client-side SHA-256 preprocessing: the staging call carries no
  // sha256_checksum argument at all.
  expect((uploadCalls[0].arguments as any).sha256_checksum).toBeUndefined()
})

test('a failed PUT can be retried against the SAME signed target, without re-staging', async ({ page }) => {
  await mountWidget(page, { kb_media_upload: [stagedUpload('mat-1')] })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()

  let putCount = 0
  await page.route(UPLOAD_URL, (route) => {
    putCount += 1
    if (putCount === 1) return route.fulfill({ status: 500, body: 'fail' })
    return route.fulfill({ status: 200, body: 'ok' })
  })

  await widget.locator('#file-input').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from('fake png bytes') })
  await expect(widget.locator('.error-box')).toBeVisible()
  const retryButton = widget.locator('#retry-upload')
  await expect(retryButton).toBeVisible()

  await retryButton.click()
  await expect(widget.getByText('Файл загружен')).toBeVisible()
  const uploadCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_media_upload')
  expect(uploadCalls.length).toBe(1)
  expect(putCount).toBe(2)
})

test('retrying against an EXPIRED signed target re-stages instead of re-PUTting', async ({ page }) => {
  // expires_at already in the past — the widget must detect this itself
  // (it never inspects the PUT's failure body) and re-call kb_media_upload
  // rather than retry a target that can only ever fail again.
  await mountWidget(page, {
    kb_media_upload: [stagedUpload('mat-1', { expiresInMs: -1000 }), stagedUpload('mat-2')],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()

  let putCount = 0
  await page.route(UPLOAD_URL, (route) => {
    putCount += 1
    if (putCount === 1) return route.fulfill({ status: 403, body: 'expired' })
    return route.fulfill({ status: 200, body: 'ok' })
  })

  await widget.locator('#file-input').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from('fake png bytes') })
  await expect(widget.locator('.error-box')).toContainText('истёк')
  await widget.locator('#retry-upload').click()
  await expect(widget.getByText('Файл загружен')).toBeVisible()

  const uploadCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_media_upload')
  expect(uploadCalls.length).toBe(2)
  expect(putCount).toBe(2)
})

test('an unsupported MIME type is rejected before any tool call', async ({ page }) => {
  await mountWidget(page)
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()

  await widget.locator('#file-input').setInputFiles({ name: 'model.glb', mimeType: 'model/gltf-binary', buffer: Buffer.from('bytes') })

  await expect(widget.locator('.error-box')).toContainText('Неподдерживаемый тип файла')
  const uploadCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_media_upload')
  expect(uploadCalls.length).toBe(0)
})

// The attach-target picker is server-driven (kb_info.media_attachment_fields),
// not a client-side matrix — an image file must offer only types/fields whose
// kind is "image", and featured_image (never an attachment target) must never
// appear regardless of file kind.
test('the attach form offers only types and fields matching the file kind, never featured_image', async ({ page }) => {
  await mountWidget(page, {
    kb_media_upload: [stagedUpload('mat-1')],
    kb_summary: [summaryWith([{ type: 'product', key: 'coffee-machine', title: 'Кофемашина' }])],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))
  await widget.locator('#file-input').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from('bytes') })
  await expect(widget.getByText('Файл загружен')).toBeVisible()

  const typeOptions = await widget.locator('#attach-type option').allTextContents()
  // Every type with an image field (topic, product, tariff, contacts) is
  // offered; delivery_zone/assistant (no media columns at all) are not.
  expect(typeOptions).toEqual(expect.arrayContaining(['Тема', 'Товар', 'Тариф', 'Контакты']))
  expect(typeOptions).not.toContain('Зона доставки')
  expect(typeOptions).not.toContain('Ассистент')

  await widget.locator('#attach-type').selectOption('product')
  const fieldOptions = await widget.locator('#attach-field option').evaluateAll((els) => els.map((el) => (el as HTMLOptionElement).value))
  expect(fieldOptions).toEqual(['gallery_images'])
  expect(fieldOptions).not.toContain('featured_image')
  expect(fieldOptions).not.toContain('demo_videos') // a video-kind field must not appear for an image file
})

test('a video file offers only video-kind fields', async ({ page }) => {
  await mountWidget(page, {
    kb_media_upload: [stagedUpload('mat-1', { contentType: 'video/mp4' })],
    kb_summary: [summaryWith([{ type: 'product', key: 'coffee-machine', title: 'Кофемашина' }])],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))
  await widget.locator('#file-input').setInputFiles({ name: 'demo.mp4', mimeType: 'video/mp4', buffer: Buffer.from('bytes') })
  await expect(widget.getByText('Файл загружен')).toBeVisible()

  await widget.locator('#attach-type').selectOption('product')
  const fieldOptions = await widget.locator('#attach-field option').evaluateAll((els) => els.map((el) => (el as HTMLOptionElement).value))
  expect(fieldOptions).toEqual(['demo_videos'])
})

// Records staged for deletion must never be offered as an attach target —
// the only way this widget can enforce "excluded" client-side.
test('a record staged for deletion (state=to_delete) is excluded from the picker', async ({ page }) => {
  await mountWidget(page, {
    kb_media_upload: [stagedUpload('mat-1')],
    kb_summary: [summaryWith([
      { type: 'product', key: 'live-one', title: 'Живой товар' },
      { type: 'product', key: 'doomed', title: 'Удаляемый товар', state: 'to_delete' },
    ])],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))
  await widget.locator('#file-input').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from('bytes') })
  await expect(widget.getByText('Файл загружен')).toBeVisible()

  await widget.locator('#attach-type').selectOption('product')
  const recordOptions = await widget.locator('#attach-key option').allTextContents()
  expect(recordOptions.join(' ')).toContain('Живой товар')
  expect(recordOptions.join(' ')).not.toContain('Удаляемый товар')
})

// The full happy path: stage, PUT, pick a record, attach — exactly one
// kb_media_attach call with the expected arguments, no read-modify-upsert.
test('attach issues exactly one kb_media_attach call with the selected target', async ({ page }) => {
  await mountWidget(page, {
    kb_media_upload: [stagedUpload('mat-1')],
    kb_summary: [summaryWith([{ type: 'product', key: 'coffee-machine', title: 'Кофемашина' }])],
    kb_media_attach: [toolResult({
      type: 'product', key: 'coffee-machine', field: 'gallery_images',
      material_id: 'mat-1', draft_version: 2, already_present: false,
    })],
  })
  const widget = await pageFrame(page)
  await widget.locator('nav.tabs button[data-tab="media"]').click()
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))
  await widget.locator('#file-input').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from('bytes') })
  await expect(widget.getByText('Файл загружен')).toBeVisible()

  await widget.locator('#attach-type').selectOption('product')
  await widget.locator('#attach-key').selectOption('coffee-machine')
  await widget.locator('#attach-field').selectOption('gallery_images')
  await widget.locator('#do-attach').click()

  await expect(widget.getByText('Материал прикреплён к записи.')).toBeVisible()
  const attachCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_media_attach')
  expect(attachCalls.length).toBe(1)
  expect(attachCalls[0].arguments).toEqual({
    material_id: 'mat-1', type: 'product', key: 'coffee-machine', field: 'gallery_images',
  })
  // No read-modify-upsert: the old flow's kb_read/kb_*_upsert calls must not
  // happen as part of attaching.
  const upsertCalls = (await toolCalls(page)).filter((c) => c.name === 'kb_product_upsert')
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
