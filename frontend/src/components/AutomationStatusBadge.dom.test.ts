import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import AutomationStatusBadge from './AutomationStatusBadge.vue'

describe('AutomationStatusBadge', () => {
  it('shows the off label', () => {
    const wrapper = mountKb(AutomationStatusBadge, { props: { mode: 'off' } })
    expect(wrapper.text()).toContain('Автоматизация выключена')
  })
  it('shows the suggestions label', () => {
    const wrapper = mountKb(AutomationStatusBadge, { props: { mode: 'suggestions' } })
    expect(wrapper.text()).toContain('Подсказки')
  })
  it('shows the scheduled auto-reply label', () => {
    const wrapper = mountKb(AutomationStatusBadge, { props: { mode: 'scheduled_auto' } })
    expect(wrapper.text()).toContain('Автоответ по расписанию')
  })
})
