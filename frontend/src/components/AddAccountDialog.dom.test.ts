import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import AddAccountDialog from './AddAccountDialog.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

// AddAccountDialog renders entirely inside reka-ui's Dialog, which teleports
// its content to document.body — every button/text here lives outside
// @vue/test-utils' own wrapper subtree and must be queried through the real
// DOM. Mirrors AutomationSettingsDialog.dom.test.ts's body().
function body() {
  return new DOMWrapper(document.body)
}

function findButton(text: string) {
  return body()
    .findAll('button')
    .find((b) => b.text() === text)
}

let wrapper: VueWrapper<any> | undefined

async function mountDialog() {
  const pinia = testPinia()
  const auth = useAuth()
  auth.user = { id: '1', email: 'a@b.c', name: 'Admin', role: 'admin', must_change_password: false }
  wrapper = mountKb(AddAccountDialog, { pinia })
  await flushPromises()
  return wrapper
}

describe('AddAccountDialog — WhatsApp QR pairing', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('shows a pre-flight checklist before starting a pairing session (docs/ux/flows/02-connect-whatsapp-qr.md #1)', async () => {
    const { api } = await import('@/api/client')
    await mountDialog()

    await body()
      .findAll('button')
      .find((b) => b.text().includes('WhatsApp'))
      ?.trigger('click')
    await flushPromises()

    expect(body().text()).toContain('Перед началом')
    expect(body().text()).toContain('интернет')
    expect(body().text()).toContain('4 устройств')
    expect(api.post).not.toHaveBeenCalled()
  })

  it('starts pairing only once the operator continues past the checklist, and does not auto-close on success', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.post).mockResolvedValue({ session_id: 's1', status: 'qr_required' })
    vi.mocked(api.get).mockResolvedValue({ status: 'connected', account_id: 'acc-1' })

    await mountDialog()
    await body()
      .findAll('button')
      .find((b) => b.text().includes('WhatsApp'))
      ?.trigger('click')
    await flushPromises()

    await findButton('Показать QR-код')?.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/wa-accounts/pair', {})
    expect(api.get).toHaveBeenCalledWith('/wa-accounts/pair/s1')
    // Connected — but the dialog stays open until the operator confirms.
    expect(body().text()).toContain('Номер подключён!')
    expect(wrapper!.emitted('connected')).toBeTruthy()
    expect(wrapper!.emitted('close')).toBeFalsy()

    await findButton('Готово')?.trigger('click')
    expect(wrapper!.emitted('close')).toBeTruthy()
  })
})
