import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { CampaignTemplate, CampaignTemplatePatch, Page } from '../types'

// useCampaignTemplates backs CAM-14's reusable message template library —
// the Templates tab on /campaigns and the campaign wizard's own template
// picker/save-as-template action. Mirrors stores/campaigns.ts's own shape:
// every call goes through api.get/post/patch directly in the action, no
// separate per-feature client module (templates carry no file upload, so
// there is not even a multipart exception here).
export const useCampaignTemplates = defineStore('campaignTemplates', {
  state: () => ({
    templates: [] as CampaignTemplate[],
    total: 0,
    loading: false,
  }),
  actions: {
    async list(opts: { archived?: boolean; q?: string; page?: number; pageSize?: number } = {}) {
      this.loading = true
      try {
        const params = new URLSearchParams({
          archived: String(opts.archived ?? false),
          page: String(opts.page ?? 1),
          page_size: String(opts.pageSize ?? 50),
        })
        if (opts.q) params.set('q', opts.q)
        const res = await api.get<Page<CampaignTemplate>>(`/campaign-templates?${params}`)
        this.templates = res.items
        this.total = res.total
        return res
      } finally {
        this.loading = false
      }
    },
    async create(input: { name: string; message_body: string }) {
      const t = await api.post<CampaignTemplate>('/campaign-templates', input)
      this.templates.unshift(t)
      this.total += 1
      return t
    },
    async update(id: string, patch: CampaignTemplatePatch) {
      const updated = await api.patch<CampaignTemplate>(`/campaign-templates/${id}`, patch)
      const idx = this.templates.findIndex((x) => x.id === id)
      if (idx !== -1) this.templates[idx] = updated
      return updated
    },
    // archive/restore both drop the affected row out of whichever filtered
    // list (Active or Archived) is currently loaded — it no longer belongs
    // on EITHER side of that toggle once its own state has flipped, so a
    // full re-list is never needed just to keep the visible list honest.
    async archive(id: string) {
      const updated = await api.post<CampaignTemplate>(`/campaign-templates/${id}/archive`)
      this.templates = this.templates.filter((x) => x.id !== id)
      this.total = Math.max(0, this.total - 1)
      return updated
    },
    async restore(id: string) {
      const updated = await api.post<CampaignTemplate>(`/campaign-templates/${id}/restore`)
      this.templates = this.templates.filter((x) => x.id !== id)
      this.total = Math.max(0, this.total - 1)
      return updated
    },
  },
})
