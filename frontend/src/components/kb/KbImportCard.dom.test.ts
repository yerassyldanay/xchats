import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { useKbImport } from '@/stores/kbImport'
import { resetImportProvidersCache } from '@/composables/useImportProviders'
import { mountKb, testPinia } from '@/test/mount'
import KbImportCard from './KbImportCard.vue'
import type { KbImportProviderCapability, KbImportRun } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, getKbImportProviders: vi.fn() } }
})

// SelectContent/SelectItem render through reka-ui's own Portal (same
// Teleport-to-document.body gotcha as Dialog — see KnowledgeBase.dom.test.ts's
// own body() helper), so these tests never open the <Select> itself. They
// instead read the plain (non-Teleported) unavailable-reasons <ul> the card
// renders right below it, and the actual submitted payload — both are
// enough to prove availability/fallback without touching the dropdown.
function defaultProviders(overrides: Record<string, Partial<KbImportProviderCapability>> = {}): KbImportProviderCapability[] {
  const base: KbImportProviderCapability[] = [
    { id: 'native', display_name: 'native', families: ['url', 'docx', 'text', 'image'], requires_credential: false, configured: true },
    { id: 'firecrawl', display_name: 'Firecrawl', families: ['url'], requires_credential: true, configured: true },
    { id: 'llamaparse', display_name: 'LlamaParse', families: ['pdf', 'docx', 'text'], requires_credential: true, configured: true },
  ]
  return base.map((p) => ({ ...p, ...(overrides[p.id] ?? {}) }))
}

async function mountCard(kind: 'url' | 'file', providers = defaultProviders()) {
  const { api } = await import('@/api/client')
  vi.mocked(api.getKbImportProviders).mockResolvedValue(providers)
  const pinia = testPinia()
  const kbi = useKbImport()
  const wrapper = mountKb(KbImportCard, { pinia, props: { kind } })
  await flushPromises()
  return { wrapper, kbi }
}

function buttonLabelled(wrapper: ReturnType<typeof mountKb>, label: string) {
  const btn = wrapper.findAll('button').find((b) => b.text() === label)
  if (!btn) throw new Error(`no button labelled ${label}`)
  return btn
}
function urlInput(wrapper: ReturnType<typeof mountKb>) {
  return wrapper.find('input[type="url"]')
}
async function stageOneUrl(wrapper: ReturnType<typeof mountKb>, url = 'https://vendor.example/zt40h') {
  await urlInput(wrapper).setValue(url)
  await buttonLabelled(wrapper, 'Добавить').trigger('click')
}
async function stageOneFile(wrapper: ReturnType<typeof mountKb>, filename: string, mime: string) {
  const file = new File(['data'], filename, { type: mime })
  const input = wrapper.find('input[type="file"]')
  Object.defineProperty(input.element, 'files', { value: [file] })
  await input.trigger('change')
  await flushPromises()
}

beforeEach(() => {
  resetImportProvidersCache()
})

describe('KbImportCard(kind=url) — staging URLs, no file controls', () => {
  it('adding a valid URL shows a chip and clears the input', async () => {
    const { wrapper } = await mountCard('url')
    await stageOneUrl(wrapper)
    expect(wrapper.text()).toContain('https://vendor.example/zt40h')
    expect((urlInput(wrapper).element as HTMLInputElement).value).toBe('')
  })

  it('rejects a non-http(s) value with an inline error and adds no chip', async () => {
    const { wrapper } = await mountCard('url')
    await urlInput(wrapper).setValue('not a url')
    await buttonLabelled(wrapper, 'Добавить').trigger('click')
    expect(wrapper.text()).toContain('Введите корректную ссылку')
    expect(wrapper.findAll('.rounded-full').length).toBe(0)
  })

  it('does not add the same URL twice', async () => {
    const { wrapper } = await mountCard('url')
    await stageOneUrl(wrapper)
    await stageOneUrl(wrapper)
    expect(wrapper.findAll('.rounded-full').length).toBe(1)
  })

  it('shows no dropzone or file input', async () => {
    const { wrapper } = await mountCard('url')
    expect(wrapper.find('input[type="file"]').exists()).toBe(false)
  })
})

// KB-11: kb_import.go enforces at most 10 files and 50 MiB each, but used to
// surface that only after a real upload attempt — these mirror the SAME
// limits client-side, rejecting an invalid selection before it ever reaches
// the network.
describe('KbImportCard(kind=file) — client-side upload limits (KB-11)', () => {
  function stageFiles(wrapper: ReturnType<typeof mountKb>, files: File[]) {
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: files, configurable: true })
    return input.trigger('change')
  }
  function fileOfSize(name: string, bytes: number): File {
    const f = new File(['x'], name, { type: 'application/pdf' })
    Object.defineProperty(f, 'size', { value: bytes, configurable: true })
    return f
  }

  it('shows the file-count and per-file size limits beside the dropzone', async () => {
    const { wrapper } = await mountCard('file')
    expect(wrapper.find('[data-testid="kb-import-file-limits"]').text()).toContain('10')
  })

  it('rejects an oversized file with an inline notice, and never stages it', async () => {
    const { wrapper } = await mountCard('file')
    await stageFiles(wrapper, [fileOfSize('huge.pdf', 60 * 1024 * 1024)])
    await flushPromises()

    expect(wrapper.findAll('.rounded-full').length).toBe(0) // never staged as a chip
    expect(wrapper.find('[data-testid="kb-import-file-error"]').text()).toContain('huge.pdf')
  })

  it('caps at 10 files and reports how many were left out', async () => {
    const { wrapper } = await mountCard('file')
    const dozen = Array.from({ length: 12 }, (_, i) => fileOfSize(`f${i}.txt`, 1024))
    await stageFiles(wrapper, dozen)
    await flushPromises()

    expect(wrapper.findAll('.rounded-full').length).toBe(10)
    expect(wrapper.find('[data-testid="kb-import-file-error"]').text()).toContain('2')
  })

  it('a valid file within limits stages normally with no error', async () => {
    const { wrapper } = await mountCard('file')
    await stageFiles(wrapper, [fileOfSize('normal.pdf', 1024)])
    await flushPromises()

    expect(wrapper.text()).toContain('normal.pdf')
    expect(wrapper.find('[data-testid="kb-import-file-error"]').exists()).toBe(false)
  })
})

describe('KbImportCard(kind=file) — staging files, no URL input', () => {
  it('picking a file through the hidden input shows a chip, removable via its X', async () => {
    const { wrapper } = await mountCard('file')
    await stageOneFile(wrapper, 'price-list.docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document')
    expect(wrapper.text()).toContain('price-list.docx')

    await wrapper.findAll('.rounded-full button')[0].trigger('click')
    expect(wrapper.text()).not.toContain('price-list.docx')
  })

  it('shows no URL input', async () => {
    const { wrapper } = await mountCard('file')
    expect(urlInput(wrapper).exists()).toBe(false)
  })
})

describe('KbImportCard — context-aware provider availability (AC2/AC3)', () => {
  it('in url mode, llamaparse is listed unavailable with "не читает ссылки"', async () => {
    const { wrapper } = await mountCard('url')
    expect(wrapper.text()).toContain('LlamaParse')
    expect(wrapper.text()).toContain('не читает ссылки')
  })

  it('in file mode, firecrawl is unavailable with "не читает файлы" even before anything is staged', async () => {
    const { wrapper } = await mountCard('file')
    expect(wrapper.text()).toContain('Firecrawl')
    expect(wrapper.text()).toContain('не читает файлы')
  })

  it('AC2: staging a .pdf additionally disables native with the PDF-specific reason', async () => {
    const { wrapper } = await mountCard('file')
    await stageOneFile(wrapper, 'price-list.pdf', 'application/pdf')
    expect(wrapper.text()).toContain('не читает PDF — нужен LlamaParse')
  })

  it('AC3: an unconfigured provider is disabled with a reason and a settings link', async () => {
    const { wrapper } = await mountCard('file', defaultProviders({ llamaparse: { configured: false } }))
    await stageOneFile(wrapper, 'price-list.pdf', 'application/pdf')
    expect(wrapper.text()).toContain('ключ не настроен')
    const settingsLink = wrapper.findAll('a').find((a) => a.text() === 'Настроить в настройках')
    expect(settingsLink).toBeTruthy()
  })

  it('shows the vision-gap notice once an image is staged, not before', async () => {
    const { wrapper } = await mountCard('file')
    expect(wrapper.text()).not.toContain('текст с него не считывается')
    await stageOneFile(wrapper, 'photo.png', 'image/png')
    expect(wrapper.text()).toContain('текст с него не считывается')
  })

  it('falls back to the first still-available provider once the current selection is invalidated', async () => {
    const { wrapper, kbi } = await mountCard('file') // default provider is 'native'
    const submitSpy = vi.spyOn(kbi, 'submit').mockResolvedValue(true)
    await stageOneFile(wrapper, 'price-list.pdf', 'application/pdf') // native can't read PDF -> auto-falls back to llamaparse
    await buttonLabelled(wrapper, 'Начать импорт').trigger('click')
    await flushPromises()
    expect(submitSpy).toHaveBeenCalledWith(expect.objectContaining({ provider: 'llamaparse' }))
  })
})

describe('KbImportCard — submitting', () => {
  it('«Начать импорт» is disabled with nothing staged', async () => {
    const { wrapper } = await mountCard('url')
    expect((buttonLabelled(wrapper, 'Начать импорт').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('submit forwards provider/target/guidance plus the staged batch, then clears staged content and guidance', async () => {
    const { wrapper, kbi } = await mountCard('url')
    const submitSpy = vi.spyOn(kbi, 'submit').mockResolvedValue(true)
    await stageOneUrl(wrapper)
    await wrapper.find('textarea').setValue('обрати внимание на цену')

    await buttonLabelled(wrapper, 'Начать импорт').trigger('click')
    await flushPromises()

    expect(submitSpy).toHaveBeenCalledWith({
      provider: 'native',
      targetType: 'auto',
      guidance: 'обрати внимание на цену',
      urls: ['https://vendor.example/zt40h'],
      files: [],
    })
    expect(wrapper.text()).not.toContain('vendor.example/zt40h')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('')
  })

  it('keeps the staged batch and shows the store error when submit fails', async () => {
    const { wrapper, kbi } = await mountCard('url')
    vi.spyOn(kbi, 'submit').mockImplementation(async () => {
      kbi.error = 'kbimport: provider is not configured'
      return false
    })
    await stageOneUrl(wrapper)
    await buttonLabelled(wrapper, 'Начать импорт').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('kbimport: provider is not configured')
    expect(wrapper.text()).toContain('vendor.example/zt40h')
  })

  it('disables submission and shows a notice while a run is non-terminal', async () => {
    const { wrapper, kbi } = await mountCard('url')
    kbi.current = { run_id: 'run-1', status: 'synthesizing', started_by: 'user-1', started_at: '2026-08-31T10:00:00Z', cancelable: false, materials: [] } as KbImportRun
    await stageOneUrl(wrapper)

    expect((buttonLabelled(wrapper, 'Начать импорт').element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.text()).toContain('Импорт уже выполняется')
  })

  // KB-15: a FAILED status load must disable submission just as firmly as a
  // known-active run — enabling it would risk a 409 against a run this
  // client just can't currently see.
  it('disables submission and shows a retry notice when the last status load failed', async () => {
    const { wrapper, kbi } = await mountCard('url')
    kbi.loadError = 'network error'
    await stageOneUrl(wrapper)

    expect((buttonLabelled(wrapper, 'Начать импорт').element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.find('[data-testid="kb-import-status-unknown"]').exists()).toBe(true)
    // The active-run notice must not ALSO show — statusUnknown is a distinct
    // condition (unknown, not known-active) with its own message.
    expect(wrapper.text()).not.toContain('Импорт уже выполняется')
  })

  it('retry re-calls loadLatest and, on success, re-enables submission', async () => {
    const { wrapper, kbi } = await mountCard('url')
    const loadSpy = vi.spyOn(kbi, 'loadLatest').mockImplementation(async () => {
      kbi.loadError = ''
    })
    kbi.loadError = 'network error'
    await stageOneUrl(wrapper)

    await wrapper.find('[data-testid="kb-import-retry-status"]').trigger('click')
    await flushPromises()

    expect(loadSpy).toHaveBeenCalledOnce()
    expect((buttonLabelled(wrapper, 'Начать импорт').element as HTMLButtonElement).disabled).toBe(false)
  })
})

describe('KbImportCard — guidance formatting tips (KB-06)', () => {
  it('shows a collapsed tips disclosure under the guidance textarea', async () => {
    const { wrapper } = await mountCard('url')
    expect(wrapper.find('[data-testid="kb-import-guidance-help"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Советы по написанию подсказки')
    expect(wrapper.text()).toContain('Одного-двух предложений достаточно')
  })
})
