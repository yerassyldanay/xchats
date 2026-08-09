import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import KbImportRunStatus from './KbImportRunStatus.vue'
import type { KbImportRun } from '@/types'

function run(over: Partial<KbImportRun> = {}): KbImportRun {
  return {
    run_id: 'run-1',
    status: 'extracting',
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
