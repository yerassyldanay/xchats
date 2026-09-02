import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountKb } from '@/test/mount'
import KbImportRunStatus from './KbImportRunStatus.vue'
import type { KbImportRun } from '@/types'

function run(over: Partial<KbImportRun> = {}): KbImportRun {
  return {
    run_id: 'run-1',
    status: 'extracting',
    started_by: 'user-1',
    started_at: '2026-08-31T10:00:00Z',
    cancelable: true,
    materials: [
      { id: 'm-1', kind: 'url', label: 'https://vendor.example/zt40h', handle: 'evidence.1', processing_status: 'parsed' },
    ],
    ...over,
  }
}

describe('KbImportRunStatus — run-level badge', () => {
  it('shows a spinning badge for a non-terminal status', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'extracting' }) } })
    expect(wrapper.text()).toContain('Извлечение…')
    expect(wrapper.find('.animate-spin').exists()).toBe(true)
  })

  it('shows a non-spinning badge once built', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'built' }) } })
    expect(wrapper.text()).toContain('Готово')
  })

  it('surfaces a failed material\'s own error message', () => {
    const wrapper = mountKb(KbImportRunStatus, {
      props: {
        run: run({
          status: 'extracting',
          materials: [{ id: 'm-1', kind: 'file', label: 'price-list.pdf', handle: 'upload.1', processing_status: 'failed', error: 'provider timed out' }],
        }),
      },
    })
    expect(wrapper.text()).toContain('Ошибка')
    expect(wrapper.text()).toContain('provider timed out')
  })
})

describe('KbImportRunStatus — synthesis block', () => {
  it('is absent until pass 2 has started', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'extracting' }) } })
    expect(wrapper.text()).not.toContain('Результат синтеза')
  })

  it('lists applied upserts with a created/updated badge and dropped ones with their reason', () => {
    const wrapper = mountKb(KbImportRunStatus, {
      props: {
        run: run({
          status: 'built',
          synthesis: {
            status: 'built',
            notes: 'готово',
            applied: [{ tool: 'kb_product_upsert', type: 'products', key: 'zt40h', created: true }],
            dropped: [{ tool: 'kb_topic_upsert', reason: 'media field referenced an unknown handle' }],
            usage: { prompt_tokens: 120, completion_tokens: 40 },
          },
        }),
      },
    })
    expect(wrapper.text()).toContain('products:zt40h')
    expect(wrapper.text()).toContain('новое')
    expect(wrapper.text()).toContain('media field referenced an unknown handle')
    expect(wrapper.text()).toContain('готово')
  })
})

// KB-04: an elapsed counter and a step summary, both derived from data
// RunSummary already carries — no separate endpoint needed.
describe('KbImportRunStatus — elapsed time and step indicator (KB-04)', () => {
  afterEach(() => vi.useRealTimers())

  it('shows the elapsed time computed from started_at', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-31T10:02:30Z'))
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ started_at: '2026-08-31T10:00:00Z' }) } })
    expect(wrapper.find('[data-testid="kb-import-elapsed"]').text()).toContain('2 мин 30 сек')
  })

  it('shows "Step 1/2" with a parsed/total count while extracting', () => {
    const wrapper = mountKb(KbImportRunStatus, {
      props: {
        run: run({
          status: 'extracting',
          materials: [
            { id: 'm-1', kind: 'url', label: 'a', handle: 'evidence.1', processing_status: 'parsed' },
            { id: 'm-2', kind: 'url', label: 'b', handle: 'evidence.2', processing_status: 'extracting' },
            { id: 'm-3', kind: 'url', label: 'c', handle: 'evidence.3', processing_status: 'queued' },
          ],
        }),
      },
    })
    expect(wrapper.text()).toContain('Шаг 1/2: обработано 1/3 файлов')
  })

  it('shows "Step 2/2" once pass 2 has started', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'synthesizing' }) } })
    expect(wrapper.text()).toContain('Шаг 2/2: синтез базы знаний')
  })
})

// KB-05: started_by is a raw user id on the wire (see KbImportRun's own
// doc comment) — this component only ever renders whatever label the
// caller already resolved, never the id itself.
describe('KbImportRunStatus — ownership (KB-05)', () => {
  it('shows who started the run when the caller provides a resolved label', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run(), startedByLabel: 'Aigul' } })
    expect(wrapper.text()).toContain('Начал(а): Aigul')
  })

  it('omits the ownership line when no label was resolved', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run() } })
    expect(wrapper.text()).not.toContain('Начал(а):')
  })
})

// KB-04: Cancel Import — offered only while it would actually do
// something (still running) and the caller allows it at all (cancellable
// defaults true, but KB-14's future history view would pass false for a
// past run).
describe('KbImportRunStatus — cancel (KB-04)', () => {
  it('shows Cancel while extracting and cancelable', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'extracting', cancelable: true }) } })
    expect(wrapper.find('[data-testid="kb-import-cancel"]').exists()).toBe(true)
  })

  it('hides Cancel once synthesis has started (cancelable turns false)', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'synthesizing', cancelable: false }) } })
    expect(wrapper.find('[data-testid="kb-import-cancel"]').exists()).toBe(false)
  })

  it('hides Cancel on a terminal run regardless of a stale cancelable flag', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'built', cancelable: true }) } })
    expect(wrapper.find('[data-testid="kb-import-cancel"]').exists()).toBe(false)
  })

  it('hides Cancel when the caller marks the view non-cancellable', () => {
    const wrapper = mountKb(KbImportRunStatus, {
      props: { run: run({ status: 'extracting', cancelable: true }), cancellable: false },
    })
    expect(wrapper.find('[data-testid="kb-import-cancel"]').exists()).toBe(false)
  })

  it('emits cancel when clicked', async () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'extracting', cancelable: true }) } })
    await wrapper.find('[data-testid="kb-import-cancel"]').trigger('click')
    expect(wrapper.emitted('cancel')).toBeTruthy()
  })

  it('shows a cancelled badge once the run itself is cancelled', () => {
    const wrapper = mountKb(KbImportRunStatus, { props: { run: run({ status: 'cancelled', cancelable: false }) } })
    expect(wrapper.text()).toContain('Отменено')
    expect(wrapper.find('[data-testid="kb-import-cancel"]').exists()).toBe(false)
  })
})
