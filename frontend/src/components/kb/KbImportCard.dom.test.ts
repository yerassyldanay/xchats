import { describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { useKbImport } from '@/stores/kbImport'
import { mountKb, testPinia } from '@/test/mount'
import KbImportCard from './KbImportCard.vue'
import type { KbImportRun } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))

// KbImportDialog renders through reka-ui's DialogPortal (Teleport to
// document.body) once open — same gotcha KnowledgeBase.dom.test.ts's own
// body() helper documents: queries for anything inside it must go through
// the real DOM, not `wrapper` (whose root is never attached to document.body).
function body() {
  return new DOMWrapper(document.body)
}

async function mountCard() {
  const pinia = testPinia()
  const kbi = useKbImport()
  vi.spyOn(kbi, 'loadLatest').mockResolvedValue(undefined)
  const wrapper = mountKb(KbImportCard, { pinia })
  await flushPromises()
  return { wrapper, kbi }
}

function urlInput(wrapper: ReturnType<typeof mountKb>) {
  return wrapper.find('input[type="url"]')
}
function buttonLabelled(scope: ReturnType<typeof mountKb> | DOMWrapper<Element>, label: string) {
  const btn = scope.findAll('button').find((b) => b.text() === label)
  if (!btn) throw new Error(`no button labelled ${label}`)
  return btn
}
async function stageOneUrl(wrapper: ReturnType<typeof mountKb>) {
  await urlInput(wrapper).setValue('https://vendor.example/zt40h')
  await buttonLabelled(wrapper, 'Добавить').trigger('click')
}

describe('KbImportCard — staging URLs and files locally', () => {
  it('adding a valid URL shows a chip and clears the input', async () => {
    const { wrapper } = await mountCard()
    await stageOneUrl(wrapper)
    expect(wrapper.text()).toContain('https://vendor.example/zt40h')
    expect((urlInput(wrapper).element as HTMLInputElement).value).toBe('')
  })

  it('rejects a non-http(s) value with an inline error and adds no chip', async () => {
    const { wrapper } = await mountCard()
    await urlInput(wrapper).setValue('not a url')
    await buttonLabelled(wrapper, 'Добавить').trigger('click')
    expect(wrapper.text()).toContain('Введите корректную ссылку')
    expect(wrapper.findAll('.rounded-full').length).toBe(0)
  })

  it('does not add the same URL twice', async () => {
    const { wrapper } = await mountCard()
    await stageOneUrl(wrapper)
    await stageOneUrl(wrapper)
    expect(wrapper.findAll('.rounded-full').length).toBe(1)
  })

  it('picking a file through the hidden input shows a chip, removable via its X', async () => {
    const { wrapper } = await mountCard()
    const file = new File(['data'], 'price-list.pdf', { type: 'application/pdf' })
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    expect(wrapper.text()).toContain('price-list.pdf')

    await wrapper.findAll('.rounded-full button')[0].trigger('click')
    expect(wrapper.text()).not.toContain('price-list.pdf')
  })

  it('«Начать импорт» is disabled with nothing staged', async () => {
    const { wrapper } = await mountCard()
    expect((buttonLabelled(wrapper, 'Начать импорт').element as HTMLButtonElement).disabled).toBe(true)
  })
})

describe('KbImportCard — submitting the staged batch', () => {
  it('opens the dialog with the right pending counts, and submit forwards provider/target/guidance plus the staged batch', async () => {
    const { wrapper, kbi } = await mountCard()
    const submitSpy = vi.spyOn(kbi, 'submit').mockResolvedValue(true)
    await stageOneUrl(wrapper)

    await buttonLabelled(wrapper, 'Начать импорт').trigger('click')
    expect(body().text()).toContain('файлов — 0, ссылок — 1')

    await body().find('textarea').setValue('обрати внимание на цену')
    await buttonLabelled(body(), 'Начать импорт').trigger('click')
    await flushPromises()

    expect(submitSpy).toHaveBeenCalledWith({
      provider: 'native',
      targetType: 'auto',
      guidance: 'обрати внимание на цену',
      urls: ['https://vendor.example/zt40h'],
      files: [],
    })
  })

  it('clears the staged batch, closes the dialog, and shows the run status once submit succeeds', async () => {
    const { wrapper, kbi } = await mountCard()
    vi.spyOn(kbi, 'submit').mockImplementation(async () => {
      kbi.current = { run_id: 'run-1', status: 'extracting', materials: [] } as KbImportRun
      return true
    })
    await stageOneUrl(wrapper)
    await buttonLabelled(wrapper, 'Начать импорт').trigger('click')
    await buttonLabelled(body(), 'Начать импорт').trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('vendor.example/zt40h')
    expect(body().find('textarea').exists()).toBe(false) // the dialog itself is gone
    expect(wrapper.text()).toContain('Извлечение…')
  })

  it('keeps the dialog open and shows the store error when submit fails', async () => {
    const { wrapper, kbi } = await mountCard()
    vi.spyOn(kbi, 'submit').mockImplementation(async () => {
      kbi.error = 'kbimport: provider is not configured'
      return false
    })
    await stageOneUrl(wrapper)
    await buttonLabelled(wrapper, 'Начать импорт').trigger('click')
    await buttonLabelled(body(), 'Начать импорт').trigger('click')
    await flushPromises()

    expect(body().text()).toContain('kbimport: provider is not configured')
    expect(wrapper.text()).toContain('vendor.example/zt40h') // still staged, nothing cleared
  })
})

describe('KbImportCard — one active run per org', () => {
  it('disables submission and shows a notice while a run is non-terminal', async () => {
    const { wrapper, kbi } = await mountCard()
    kbi.current = { run_id: 'run-1', status: 'synthesizing', materials: [] } as KbImportRun
    await wrapper.vm.$nextTick()
    await stageOneUrl(wrapper)

    expect((buttonLabelled(wrapper, 'Начать импорт').element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.text()).toContain('Импорт уже выполняется')
  })
})
