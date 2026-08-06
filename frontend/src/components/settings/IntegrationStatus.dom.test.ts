import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import IntegrationStatus from './IntegrationStatus.vue'

describe('IntegrationStatus', () => {
  it('shows "not configured" when configured is false', () => {
    const wrapper = mountKb(IntegrationStatus, { props: { configured: false } })
    expect(wrapper.text()).toContain('Не настроено')
  })

  it('shows an error state when configured but last_error is set', () => {
    const wrapper = mountKb(IntegrationStatus, { props: { configured: true, lastError: 'boom' } })
    expect(wrapper.text()).toContain('Ошибка')
  })

  it('shows verified when configured, no error, and last_verified_at is set', () => {
    const wrapper = mountKb(IntegrationStatus, {
      props: { configured: true, lastVerifiedAt: '2026-01-01T00:00:00Z' },
    })
    expect(wrapper.text()).toContain('Подтверждено')
  })

  it('shows configured (unverified) when configured but never checked', () => {
    const wrapper = mountKb(IntegrationStatus, { props: { configured: true } })
    expect(wrapper.text()).toContain('Настроено')
  })
})
