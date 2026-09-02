import { afterEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import CampaignTemplatesPanel from './CampaignTemplatesPanel.vue'
import type { CampaignTemplate } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn() } }
})

// The template form dialog teleports into document.body — unmount at the
// end of every test (real Vue teardown) so the next test's document.body
// queries never see a previous test's leftover nodes.
let mounted: ReturnType<typeof mountKb> | null = null
afterEach(() => {
  mounted?.unmount()
  mounted = null
})
function body() {
  return new DOMWrapper(document.body)
}

function template(over: Partial<CampaignTemplate> = {}): CampaignTemplate {
  return {
    id: 'tmpl-1', name: 'Summer promo', message_body: 'Hi {{name}}, {{discount}}% off!',
    variables: ['name', 'discount'], is_archived: false, created_by: 'user-1',
    created_at: '2026-08-01T00:00:00Z', updated_at: '2026-08-01T00:00:00Z',
    ...over,
  }
}

async function mountPanel(items: CampaignTemplate[] = [template()]) {
  const { api } = await import('@/api/client')
  // Call history (and any queued mockResolvedValueOnce) is NOT reset
  // between tests by default in this project — every mountPanel() call
  // starts clean so one test's leftover history/queue never leaks into
  // the next.
  vi.mocked(api.get).mockReset()
  vi.mocked(api.post).mockReset()
  vi.mocked(api.patch).mockReset()
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path.startsWith('/campaign-templates?')) return { items, total: items.length } as any
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const wrapper = mountKb(CampaignTemplatesPanel, { pinia })
  mounted = wrapper
  await flushPromises()
  return wrapper
}

describe('CampaignTemplatesPanel — list (CAM-14)', () => {
  it('loads the active list on mount and renders name, body, and variable tags', async () => {
    const wrapper = await mountPanel()
    const { api } = await import('@/api/client')
    expect(api.get).toHaveBeenCalledWith('/campaign-templates?archived=false&page=1&page_size=20')

    expect(wrapper.find('[data-testid="template-card"]').text()).toContain('Summer promo')
    expect(wrapper.text()).toContain('Hi {{name}}, {{discount}}% off!')
    expect(wrapper.text()).toContain('{{name}}')
    expect(wrapper.text()).toContain('{{discount}}')
  })

  it('shows the active-empty state when there are no templates', async () => {
    const wrapper = await mountPanel([])
    expect(wrapper.find('[data-testid="template-card"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Активных шаблонов пока нет')
  })

  it('switching to Archived re-fetches with archived=true', async () => {
    const wrapper = await mountPanel()
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockClear()

    await wrapper.find('[data-testid="template-filter-archived"]').trigger('click')
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/campaign-templates?archived=true&page=1&page_size=20')
  })

  it('typing a search query debounces then re-fetches with q=', async () => {
    vi.useFakeTimers()
    try {
      const wrapper = await mountPanel()
      const { api } = await import('@/api/client')
      vi.mocked(api.get).mockClear()

      await wrapper.find('[data-testid="template-search"]').setValue('лет')
      // Not yet — still inside the debounce window.
      expect(api.get).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(350)
      expect(api.get).toHaveBeenCalledWith('/campaign-templates?archived=false&page=1&page_size=20&q=%D0%BB%D0%B5%D1%82')
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('CampaignTemplatesPanel — create (CAM-14)', () => {
  it('opening "New template", filling the form, and saving posts and prepends the new template', async () => {
    const wrapper = await mountPanel([])
    const { api } = await import('@/api/client')
    const created = template({ id: 'tmpl-new', name: 'Winter sale', message_body: 'Snowy deals for {{name}}', variables: ['name'] })
    vi.mocked(api.post).mockResolvedValueOnce(created)

    await wrapper.find('[data-testid="template-new"]').trigger('click')
    await flushPromises()
    await body().find('[data-testid="template-name-input"]').setValue('Winter sale')
    await body().find('[data-testid="template-body-input"]').setValue('Snowy deals for {{name}}')
    await body().find('[data-testid="template-form-save"]').trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/campaign-templates', { name: 'Winter sale', message_body: 'Snowy deals for {{name}}' })
    expect(wrapper.text()).toContain('Winter sale')
  })

  it('rejects an empty name without calling the API', async () => {
    const wrapper = await mountPanel([])
    const { api } = await import('@/api/client')

    await wrapper.find('[data-testid="template-new"]').trigger('click')
    await flushPromises()
    await body().find('[data-testid="template-body-input"]').setValue('Some body')
    await body().find('[data-testid="template-form-save"]').trigger('click')
    await flushPromises()

    expect(api.post).not.toHaveBeenCalled()
    expect(body().find('[data-testid="template-form-error"]').exists()).toBe(true)
  })
})

describe('CampaignTemplatesPanel — edit (CAM-14)', () => {
  it('Edit opens pre-filled, and saving PATCHes the template', async () => {
    const wrapper = await mountPanel([template()])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(template({ name: 'Summer promo v2' }))

    await wrapper.find('[data-testid="template-edit"]').trigger('click')
    await flushPromises()
    const nameInput = body().find('[data-testid="template-name-input"]').element as HTMLInputElement
    expect(nameInput.value).toBe('Summer promo')

    await body().find('[data-testid="template-name-input"]').setValue('Summer promo v2')
    await body().find('[data-testid="template-form-save"]').trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/campaign-templates/tmpl-1', { name: 'Summer promo v2', message_body: 'Hi {{name}}, {{discount}}% off!' })
    expect(wrapper.text()).toContain('Summer promo v2')
  })
})

describe('CampaignTemplatesPanel — archive/restore (CAM-14)', () => {
  it('Archive posts to .../archive and removes the row from the active list, with no confirmation dialog', async () => {
    const wrapper = await mountPanel([template()])
    const { api } = await import('@/api/client')
    vi.mocked(api.post).mockResolvedValueOnce(template({ is_archived: true }))
    const nativeConfirm = vi.spyOn(window, 'confirm')

    await wrapper.find('[data-testid="template-toggle-archive"]').trigger('click')
    await flushPromises()

    expect(nativeConfirm).not.toHaveBeenCalled()
    expect(api.post).toHaveBeenCalledWith('/campaign-templates/tmpl-1/archive')
    expect(wrapper.find('[data-testid="template-card"]').exists()).toBe(false)
    nativeConfirm.mockRestore()
  })

  it('on the Archived filter, Restore posts to .../restore and removes the row', async () => {
    const wrapper = await mountPanel([template({ is_archived: true })])
    const { api } = await import('@/api/client')
    await wrapper.find('[data-testid="template-filter-archived"]').trigger('click')
    await flushPromises()
    vi.mocked(api.post).mockResolvedValueOnce(template({ is_archived: false }))

    await wrapper.find('[data-testid="template-toggle-archive"]').trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/campaign-templates/tmpl-1/restore')
    expect(wrapper.find('[data-testid="template-card"]').exists()).toBe(false)
  })
})
