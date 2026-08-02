import { test, expect } from '@playwright/test'
import {
  mountWidget,
  setToolResponse,
  toolCalls,
  receivedUiInitialize,
  sendToWidget,
  postFromImpostorWindow,
  toolResult,
  recordRead,
  mediaMeta,
  routeMedia,
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

// ---------------------------------------------------------------------------
// Record view: delete confirmation, media previews, per-field uploads.
// ---------------------------------------------------------------------------

const PRODUCT_ROW = {
  ref: 'coffee-machine',
  name: 'Кофемашина',
  in_stock: true,
  featured_image: null as string | null,
  gallery_images: [] as string[],
  demo_videos: [] as string[],
  certificate_documents: [] as string[],
  guarantee_documents: [] as string[],
}

function productRecord(over: Partial<typeof PRODUCT_ROW> = {}) {
  return [{ type: 'product', source: 'draft', data: { ...PRODUCT_ROW, ...over } }]
}

// openProduct mounts the widget, seeds a summary containing the product, and
// clicks into its record view.
async function openProduct(
  page: import('@playwright/test').Page,
  read = recordRead(productRecord()),
) {
  const widget = await mountWidget(page, {
    kb_summary: [summaryWith([{ type: 'product', key: 'coffee-machine', title: 'Кофемашина' }])],
    kb_read: [read],
  })
  await widget.locator('[data-open-type="product"]').click()
  await expect(widget.locator('.source-block').first()).toBeVisible()
  return widget
}

// --- delete -----------------------------------------------------------------

test('clicking Удалить only arms a confirmation — it issues no kb_delete', async ({ page }) => {
  const widget = await openProduct(page)
  await setToolResponse(page, 'kb_delete', toolResult({ ok: true }))

  await widget.locator('[data-delete-type]').click()
  await expect(widget.locator('#confirm-delete')).toBeVisible()

  // The whole point of the bug: the first click must never delete anything.
  expect((await toolCalls(page)).filter((c) => c.name === 'kb_delete').length).toBe(0)
})

test('confirming the deletion issues exactly one kb_delete with type and key', async ({ page }) => {
  const widget = await openProduct(page)
  await setToolResponse(page, 'kb_delete', toolResult({ ok: true }))

  await widget.locator('[data-delete-type]').click()
  await widget.locator('[data-confirm-delete]').click()

  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_delete').length)
    .toBe(1)
  const call = (await toolCalls(page)).find((c) => c.name === 'kb_delete')!
  expect(call.arguments).toEqual({ type: 'product', key: 'coffee-machine' })
})

test('cancelling the deletion issues no kb_delete and hides the confirmation', async ({ page }) => {
  const widget = await openProduct(page)
  await setToolResponse(page, 'kb_delete', toolResult({ ok: true }))

  await widget.locator('[data-delete-type]').click()
  await widget.locator('[data-cancel-delete]').click()

  await expect(widget.locator('#confirm-delete')).toHaveCount(0)
  expect((await toolCalls(page)).filter((c) => c.name === 'kb_delete').length).toBe(0)
})

test('Escape cancels the deletion, and repeated cycles do not stack listeners', async ({ page }) => {
  const widget = await openProduct(page)
  await setToolResponse(page, 'kb_delete', toolResult({ ok: true }))

  // Arm and Escape several times. A keydown listener registered inside
  // wireEvents() (which runs after EVERY render) would accumulate one
  // duplicate per render; the single confirm below is what would then fire
  // more than once.
  for (let i = 0; i < 3; i++) {
    await widget.locator('[data-delete-type]').click()
    await expect(widget.locator('#confirm-delete')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(widget.locator('#confirm-delete')).toHaveCount(0)
  }
  expect((await toolCalls(page)).filter((c) => c.name === 'kb_delete').length).toBe(0)

  await widget.locator('[data-delete-type]').click()
  await widget.locator('[data-confirm-delete]').click()
  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_delete').length)
    .toBe(1)
})

test('navigating away clears an armed confirmation', async ({ page }) => {
  const widget = await openProduct(page)
  await widget.locator('[data-delete-type]').click()
  await expect(widget.locator('#confirm-delete')).toBeVisible()

  await widget.locator('[data-back]').click()
  await widget.locator('[data-open-type="product"]').click()
  await expect(widget.locator('.source-block').first()).toBeVisible()

  await expect(widget.locator('#confirm-delete')).toHaveCount(0)
})

test('the delete flow never opens a native dialog', async ({ page }) => {
  // The production bug exactly: window.confirm is suppressed in a sandboxed
  // iframe and returns false, so the button did nothing. The fix must not
  // depend on native dialogs at all.
  let dialogs = 0
  page.on('dialog', (d) => {
    dialogs += 1
    return d.dismiss()
  })

  const widget = await openProduct(page)
  await setToolResponse(page, 'kb_delete', toolResult({ ok: true }))
  await widget.locator('[data-delete-type]').click()
  await widget.locator('[data-confirm-delete]').click()
  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_delete').length)
    .toBe(1)

  expect(dialogs).toBe(0)
})

// --- previews ---------------------------------------------------------------

test('an attached image renders a thumbnail that actually loads', async ({ page }) => {
  await routeMedia(page)
  const widget = await openProduct(
    page,
    recordRead(productRecord({ gallery_images: ['mat-img'] }), {
      'mat-img': mediaMeta('mat-img', { filename: 'кофемашина.png' }),
    }),
  )

  const img = widget.locator('img.media-thumb')
  await expect(img).toHaveCount(1)
  await expect(img).toHaveAttribute('alt', 'кофемашина.png')
  // naturalWidth proves the URL resolved to real bytes, not merely that an
  // <img> tag was emitted.
  await expect.poll(async () => img.evaluate((el: HTMLImageElement) => el.naturalWidth)).toBeGreaterThan(0)
})

test('video and documents render as links, never as media players', async ({ page }) => {
  await routeMedia(page)
  const widget = await openProduct(
    page,
    recordRead(
      productRecord({
        demo_videos: ['mat-vid'],
        certificate_documents: ['mat-doc'],
      }),
      {
        'mat-vid': mediaMeta('mat-vid', { kind: 'video', mime_type: 'video/mp4', filename: 'обзор.mp4' }),
        'mat-doc': mediaMeta('mat-doc', { kind: 'document', mime_type: 'application/pdf', filename: 'манул.pdf' }),
      },
    ),
  )

  // blob.Store has no streaming read, so nothing but images may be fetched
  // without the user asking: no player elements at all.
  await expect(widget.locator('video')).toHaveCount(0)
  await expect(widget.locator('audio')).toHaveCount(0)
  await expect(widget.locator('.media-doc')).toHaveCount(2)
  await expect(widget.locator('.media-doc', { hasText: 'обзор.mp4' })).toBeVisible()
  await expect(widget.locator('.media-doc', { hasText: 'манул.pdf' })).toBeVisible()
})

test('an empty media column still renders, as «не задано»', async ({ page }) => {
  const widget = await openProduct(page)
  const field = widget.locator('.media-field', { hasText: 'Gallery images' })
  await expect(field).toBeVisible()
  await expect(field.locator('.empty-val').first()).toContainText('не задано')
})

test('featured_image gets a preview but no upload input', async ({ page }) => {
  await routeMedia(page)
  const widget = await openProduct(
    page,
    recordRead(productRecord({ featured_image: 'mat-feat' }), {
      'mat-feat': mediaMeta('mat-feat', { filename: 'главное.png' }),
    }),
  )

  const field = widget.locator('.media-field', { hasText: 'Featured image' })
  await expect(field).toBeVisible()
  await expect(field.locator('img.media-thumb')).toHaveCount(1)
  // Not an attachment target — the registry omits it, so no input may appear.
  await expect(field.locator('[data-upload-field]')).toHaveCount(0)
  await expect(widget.locator('[data-upload-field="featured_image"]')).toHaveCount(0)
})

test('a registry field absent from the record data renders nothing', async ({ page }) => {
  // gallery_images is in the registry, but this record simply does not carry
  // the column — a registry entry must never conjure a synthetic UI field.
  const widget = await openProduct(
    page,
    recordRead([{ type: 'product', source: 'draft', data: { ref: 'coffee-machine', name: 'Кофемашина' } }]),
  )
  await expect(widget.locator('.media-field')).toHaveCount(0)
  await expect(widget.locator('[data-upload-field]')).toHaveCount(0)
})

test('a record with no entries shows «не найдена» and no media section', async ({ page }) => {
  // Mounted directly rather than via openProduct: with zero entries there is
  // no .source-block to wait for, which is the whole point.
  const widget = await mountWidget(page, {
    kb_summary: [summaryWith([{ type: 'product', key: 'coffee-machine', title: 'Кофемашина' }])],
    kb_read: [recordRead([])],
  })
  await widget.locator('[data-open-type="product"]').click()

  await expect(widget.locator('.empty')).toContainText('не найдена')
  await expect(widget.locator('.media-field')).toHaveCount(0)
  await expect(widget.locator('[data-upload-field]')).toHaveCount(0)
})

test('without _meta the widget degrades to chips, with no image and no error', async ({ page }) => {
  // A host that strips _meta: previews are unavailable, but nothing breaks.
  const widget = await openProduct(page, recordRead(productRecord({ gallery_images: ['mat-img'] })))

  await expect(widget.locator('img.media-thumb')).toHaveCount(0)
  await expect(widget.locator('.error-box')).toHaveCount(0)
  await expect(widget.locator('.media-field .media-chip').first()).toContainText('предпросмотр недоступен')
})

// --- uploads ----------------------------------------------------------------

test('every attachable field gets its own upload input, with unique ids', async ({ page }) => {
  const widget = await openProduct(page)
  const inputs = widget.locator('[data-upload-field]')

  // Product has exactly four attachable fields; featured_image and the
  // intentionally removed fields are not upload targets.
  await expect(inputs).toHaveCount(4)
  const ids = await inputs.evaluateAll((els) => els.map((e) => e.id))
  expect(new Set(ids).size).toBe(ids.length)
  expect(ids).not.toContain('up-input-featured_image')
  await expect(inputs.evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.uploadField))).resolves.toEqual([
    'gallery_images',
    'demo_videos',
    'certificate_documents',
    'guarantee_documents',
  ])
})

test('all remaining product file fields upload and attach matching files', async ({ page }) => {
  const widget = await openProduct(page)
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))

  const cases = [
    { field: 'gallery_images', name: 'photo.png', mimeType: 'image/png' },
    { field: 'demo_videos', name: 'demo.mp4', mimeType: 'video/mp4' },
    { field: 'certificate_documents', name: 'certificate.pdf', mimeType: 'application/pdf' },
    { field: 'guarantee_documents', name: 'guarantee.pdf', mimeType: 'application/pdf' },
  ]
  await setToolResponse(
    page,
    'kb_media_upload',
    ...cases.map((c, i) => stagedUpload(`mat-${i + 1}`, { contentType: c.mimeType })),
  )
  await setToolResponse(page, 'kb_media_attach', toolResult({ material_id: 'attached', draft_version: 2 }))

  for (const [i, c] of cases.entries()) {
    await widget.locator(`[data-upload-field="${c.field}"]`).setInputFiles({
      name: c.name,
      mimeType: c.mimeType,
      buffer: Buffer.from(c.name),
    })
    await expect
      .poll(async () => (await toolCalls(page)).filter((call) => call.name === 'kb_media_attach').length)
      .toBe(i + 1)
  }

  const calls = (await toolCalls(page)).filter((c) => c.name === 'kb_media_attach')
  expect(calls.map((c) => (c.arguments as any).field)).toEqual(cases.map((c) => c.field))
})

test('two files into one field upload and attach both, then refresh the record', async ({ page }) => {
  const widget = await openProduct(page)
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))
  await setToolResponse(page, 'kb_media_upload', stagedUpload('mat-1'), stagedUpload('mat-2'))
  await setToolResponse(page, 'kb_media_attach', toolResult({ material_id: 'mat-1', draft_version: 2 }))

  await widget.locator('[data-upload-field="gallery_images"]').setInputFiles([
    { name: 'a.png', mimeType: 'image/png', buffer: Buffer.from('a') },
    { name: 'b.png', mimeType: 'image/png', buffer: Buffer.from('b') },
  ])

  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_media_attach').length)
    .toBe(2)

  const calls = await toolCalls(page)
  const uploads = calls.filter((c) => c.name === 'kb_media_upload')
  expect(uploads.length).toBe(2)
  // target is sent so the server marks visibility and validates the pairing at
  // staging time rather than at attach time.
  expect((uploads[0].arguments as any).target).toEqual({ type: 'product', field: 'gallery_images' })
  for (const a of calls.filter((c) => c.name === 'kb_media_attach')) {
    expect((a.arguments as any).field).toBe('gallery_images')
    expect((a.arguments as any).key).toBe('coffee-machine')
  }
  // The record is re-read once the batch drains, for fresh ids and URLs.
  await expect.poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_read').length).toBeGreaterThan(1)
})

test('one failing file does not discard the rest of the batch', async ({ page }) => {
  const widget = await openProduct(page)
  let puts = 0
  await page.route(UPLOAD_URL, (route) => {
    puts += 1
    return route.fulfill(puts === 2 ? { status: 500, body: 'boom' } : { status: 200, body: 'ok' })
  })
  await setToolResponse(page, 'kb_media_upload', stagedUpload('mat-1'), stagedUpload('mat-2'))
  await setToolResponse(page, 'kb_media_attach', toolResult({ material_id: 'mat-1', draft_version: 2 }))

  await widget.locator('[data-upload-field="gallery_images"]').setInputFiles([
    { name: 'good.png', mimeType: 'image/png', buffer: Buffer.from('a') },
    { name: 'bad.png', mimeType: 'image/png', buffer: Buffer.from('b') },
  ])

  // The first file still attached...
  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_media_attach').length)
    .toBe(1)
  // ...and the second is shown as a per-item error with a retry, not a global one.
  await expect(widget.locator('.upload-item', { hasText: 'bad.png' }).locator('.error-box')).toBeVisible()
  await expect(widget.locator('[data-retry-upload]')).toHaveCount(1)
})

test('a stale ChatGPT manifest falls back from kb_media_attach to the existing typed upsert', async ({ page }) => {
  const widget = await openProduct(page)
  await page.route(UPLOAD_URL, (route) => route.fulfill({ status: 200, body: 'ok' }))
  await setToolResponse(page, 'kb_media_upload', stagedUpload('mat-1'), stagedUpload('mat-2'))
  await setToolResponse(page, 'kb_media_attach', {
    content: [{ type: 'text', text: 'MCP Resource not found' }],
    isError: true,
  })
  await setToolResponse(page, 'kb_product_upsert', toolResult({ draft_version: 2 }))

  await widget.locator('[data-upload-field="gallery_images"]').setInputFiles([
    { name: 'a.png', mimeType: 'image/png', buffer: Buffer.from('a') },
    { name: 'b.png', mimeType: 'image/png', buffer: Buffer.from('b') },
  ])

  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_product_upsert').length)
    .toBe(2)

  const calls = await toolCalls(page)
  const fallbacks = calls.filter((c) => c.name === 'kb_product_upsert')
  expect(fallbacks[0].arguments).toEqual({
    ref: 'coffee-machine',
    changes: { gallery_images: ['mat-1'] },
  })
  // The second fallback must retain the first material even though the batch
  // does not refresh kb_read until both files finish.
  expect(fallbacks[1].arguments).toEqual({
    ref: 'coffee-machine',
    changes: { gallery_images: ['mat-1', 'mat-2'] },
  })
  await expect(widget.locator('[data-retry-upload]')).toHaveCount(0)
})

test('a file of the wrong kind for the field is rejected before any tool call', async ({ page }) => {
  const widget = await openProduct(page)
  await widget.locator('[data-upload-field="gallery_images"]').setInputFiles({
    name: 'clip.mp4',
    mimeType: 'video/mp4',
    buffer: Buffer.from('mp4'),
  })

  await expect(widget.locator('.upload-item', { hasText: 'clip.mp4' }).locator('.error-box')).toBeVisible()
  expect((await toolCalls(page)).filter((c) => c.name === 'kb_media_upload').length).toBe(0)
})

test('a singular field offers no multiple attribute', async ({ page }) => {
  const widget = await mountWidget(page, {
    kb_summary: [summaryWith([{ type: 'contacts', key: 'main', title: 'Контакты' }])],
    kb_read: [
      recordRead([
        {
          type: 'contacts',
          source: 'draft',
          data: {
            contact_card_image: null,
            location_map_image: null,
            company_legal_documents: [],
          },
        },
      ]),
    ],
  })
  await widget.locator('[data-open-type="contacts"]').click()
  await expect(widget.locator('.source-block').first()).toBeVisible()

  const single = widget.locator('[data-upload-field="contact_card_image"]')
  await expect(single).toHaveCount(1)
  await expect(single).not.toHaveAttribute('multiple', /.*/)
  // A plural field on the same record still allows multi-select.
  await expect(widget.locator('[data-upload-field="company_legal_documents"]')).toHaveAttribute('multiple', /.*/)
})

test('retrying an EXPIRED signed target re-stages instead of re-PUTting', async ({ page }) => {
  const widget = await openProduct(page)
  let puts = 0
  await page.route(UPLOAD_URL, (route) => {
    puts += 1
    return route.fulfill(puts === 1 ? { status: 403, body: 'expired' } : { status: 200, body: 'ok' })
  })
  await setToolResponse(page, 'kb_media_upload', stagedUpload('mat-1', { expiresInMs: -1000 }), stagedUpload('mat-2'))
  await setToolResponse(page, 'kb_media_attach', toolResult({ material_id: 'mat-2', draft_version: 2 }))

  await widget.locator('[data-upload-field="gallery_images"]').setInputFiles({
    name: 'photo.png',
    mimeType: 'image/png',
    buffer: Buffer.from('png'),
  })
  await expect(widget.locator('[data-retry-upload]')).toBeVisible()
  await expect(widget.locator('.error-box')).toContainText('истёк')

  await widget.locator('[data-retry-upload]').click()
  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_media_attach').length)
    .toBe(1)
  // Expired → a SECOND kb_media_upload, i.e. a fresh material.
  expect((await toolCalls(page)).filter((c) => c.name === 'kb_media_upload').length).toBe(2)
  expect(puts).toBe(2)
})

test('upload progress does not rebuild the DOM', async ({ page }) => {
  const widget = await openProduct(page)
  await page.route(UPLOAD_URL, async (route) => {
    await new Promise((r) => setTimeout(r, 300))
    return route.fulfill({ status: 200, body: 'ok' })
  })
  await setToolResponse(page, 'kb_media_upload', stagedUpload('mat-1'))
  await setToolResponse(page, 'kb_media_attach', toolResult({ material_id: 'mat-1', draft_version: 2 }))

  await widget.locator('[data-upload-field="gallery_images"]').setInputFiles({
    name: 'slow.png',
    mimeType: 'image/png',
    buffer: Buffer.from('x'.repeat(1024 * 1024)),
  })

  // Mark only AFTER the status→uploading render has already happened. Status
  // transitions legitimately re-render; what must not re-render is a progress
  // tick, and this is the window where those fire.
  await expect(widget.locator('.progress-bar')).toBeVisible()
  await widget
    .locator('[data-upload-field="certificate_documents"]')
    .evaluate((el) => ((el as HTMLElement).dataset.marker = 'still-here'))

  await page.waitForTimeout(250)
  // Still uploading, so no status transition has intervened...
  await expect(widget.locator('.progress-bar')).toBeVisible()
  // ...and the untouched input in another field survived. render() rebuilds
  // the entire document via innerHTML, so a per-tick render would have
  // destroyed this node — along with any in-flight file input mid-batch.
  expect(
    await widget
      .locator('[data-upload-field="certificate_documents"]')
      .evaluate((el) => (el as HTMLElement).dataset.marker),
  ).toBe('still-here')
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

test('retry after a successful PUT but a failed attach re-attaches, never re-PUTs', async ({ page }) => {
  // A transient attach failure happens AFTER the bytes have already landed.
  // A signed upload target is one-shot server-side, so a retry that re-PUTs
  // gets a permanent 409 and the file can never be attached without picking
  // it again.
  const widget = await openProduct(page)
  let puts = 0
  await page.route(UPLOAD_URL, (route) => {
    puts += 1
    return route.fulfill({ status: 200, body: 'ok' })
  })
  await setToolResponse(page, 'kb_media_upload', stagedUpload('mat-1'))
  await setToolResponse(
    page,
    'kb_media_attach',
    { content: [{ type: 'text', text: 'temporary attach failure' }], isError: true },
    toolResult({ material_id: 'mat-1', draft_version: 2 }),
  )

  await widget.locator('[data-upload-field="gallery_images"]').setInputFiles({
    name: 'photo.png',
    mimeType: 'image/png',
    buffer: Buffer.from('png'),
  })

  await expect(widget.locator('[data-retry-upload]')).toBeVisible()
  await expect(widget.locator('.error-box')).toContainText('не прикреплён')
  expect(puts).toBe(1)

  await widget.locator('[data-retry-upload]').click()

  await expect
    .poll(async () => (await toolCalls(page)).filter((c) => c.name === 'kb_media_attach').length)
    .toBe(2)
  // Neither the staging nor the byte transfer may be repeated.
  expect((await toolCalls(page)).filter((c) => c.name === 'kb_media_upload').length).toBe(1)
  expect(puts).toBe(1)
})
