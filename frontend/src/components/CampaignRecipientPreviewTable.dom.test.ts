import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import CampaignRecipientPreviewTable from './CampaignRecipientPreviewTable.vue'
import type { CampaignRecipientPreviewResult, CampaignRecipientPreviewRow } from '@/types'

function row(over: Partial<CampaignRecipientPreviewRow> = {}): CampaignRecipientPreviewRow {
  return { raw: '77011234567,Aigul,SUMMER2026', status: 'valid', ...over }
}
function result(rows: CampaignRecipientPreviewRow[]): CampaignRecipientPreviewResult {
  return { rows, total: rows.length, valid: rows.filter((r) => r.status === 'valid').length, invalid: 0, duplicate: 0 }
}

// CAM-06: an Attribute's key IS the {{variable}} name it feeds (the parser
// keys Attributes by the CSV header text verbatim) — these chips read that
// straight off the already-parsed rows, and the unmatched-variable warning
// catches a message that references a column the uploaded data never had.
describe('CampaignRecipientPreviewTable — column mapping (CAM-06)', () => {
  it('shows no column chips when no row carries a name or attributes', async () => {
    const wrapper = mountKb(CampaignRecipientPreviewTable, { props: { result: result([row()]) } })
    expect(wrapper.text()).not.toContain('Найденные колонки')
  })

  it('lists name plus every distinct attribute key across all rows as detected columns', async () => {
    const rows = [
      row({ name: 'Aigul', attributes: { promo_code: 'SUMMER2026' } }),
      row({ raw: '77022222222,Bota,WINTER2026,VIP', name: 'Bota', attributes: { promo_code: 'WINTER2026', tier: 'VIP' } }),
    ]
    const wrapper = mountKb(CampaignRecipientPreviewTable, { props: { result: result(rows) } })
    const text = wrapper.text()
    expect(text).toContain('Найденные колонки')
    expect(text).toContain('name → {{name}}')
    expect(text).toContain('promo_code → {{promo_code}}')
    expect(text).toContain('tier → {{tier}}')
  })

  it('warns when the message uses a variable no uploaded column provides', async () => {
    const wrapper = mountKb(CampaignRecipientPreviewTable, {
      props: { result: result([row({ name: 'Aigul', attributes: { promo_code: 'SUMMER2026' } })]), messageVariables: ['name', 'promo_code', 'discount'] },
    })
    expect(wrapper.find('[data-testid="unmatched-variables-warning"]').text()).toContain('discount')
    expect(wrapper.find('[data-testid="unmatched-variables-warning"]').text()).not.toContain('promo_code')
  })

  it('never treats phone as unmatched — it is the identity column, not a named Attribute', async () => {
    const wrapper = mountKb(CampaignRecipientPreviewTable, {
      props: { result: result([row({ name: 'Aigul' })]), messageVariables: ['name', 'phone'] },
    })
    expect(wrapper.find('[data-testid="unmatched-variables-warning"]').exists()).toBe(false)
  })
})
