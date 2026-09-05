import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mountKb, testPinia } from '@/test/mount'
import { useCrm } from '@/stores/crm'
import AssistantPanel from './AssistantPanel.vue'

// No active chat is selected in any of these — CustomerPanel's own
// customerId watcher then resolves to null and clears state locally without
// an API call (loadProfile(null) returns early), so the collapse toggle can
// be exercised without mocking @/api/client at all.
function mountPanel() {
  const pinia = testPinia()
  useCrm().catalogsLoaded = true // skip CustomerPanel's tag/status/field fetch
  return mountKb(AssistantPanel, { pinia })
}

// The "based on ..." draft source badge (Component 4 of the multimodal
// plan) is covered at the logic level in lib/draftSource.test.ts rather
// than here: driving reka-ui's Tabs from "Клиент" to "ИИ-помощник" via
// synthetic pointer events in jsdom does not reliably flip the underlying
// state in this test environment, for reasons unrelated to the badge logic
// itself — sourceOf() is a plain, directly-testable function specifically
// so its correctness does not depend on that interaction working here.

// INB-02: collapsing reclaims the fixed 340px this panel used to always
// take, on the 13"/14" laptop widths the flow doc calls out.
describe('AssistantPanel — collapse toggle (INB-02)', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('starts expanded, showing the Customer/ИИ-помощник tabs', () => {
    const wrapper = mountPanel()
    expect(wrapper.text()).toContain('ИИ-помощник')
    expect(wrapper.find('button[title="Свернуть панель"]').exists()).toBe(true)
  })

  it('collapses to a slim rail on toggle, hiding the tabs, and can expand back', async () => {
    const wrapper = mountPanel()

    await wrapper.find('button[title="Свернуть панель"]').trigger('click')
    expect(wrapper.text()).not.toContain('ИИ-помощник')
    const expandBtn = wrapper.find('button[title="Развернуть панель"]')
    expect(expandBtn.exists()).toBe(true)

    await expandBtn.trigger('click')
    expect(wrapper.text()).toContain('ИИ-помощник')
  })

  it('persists the collapsed choice across remounts (survives a refresh)', async () => {
    const first = mountPanel()
    await first.find('button[title="Свернуть панель"]').trigger('click')
    first.unmount()

    const second = mountPanel()
    expect(second.find('button[title="Развернуть панель"]').exists()).toBe(true)
    expect(second.text()).not.toContain('ИИ-помощник')
  })
})
