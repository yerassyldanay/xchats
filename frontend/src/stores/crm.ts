import { defineStore } from 'pinia'
import { api, ApiError, isAbortError } from '../api/client'
import { log } from '../lib/logfmt'
import { currentUtcOffsetMinutes } from '../lib/schedule'
import type {
  CustomFieldDef,
  Customer,
  CustomerNote,
  CustomerProfile,
  CustomerStatus,
  CustomerTag,
  Followup,
  FollowupBuckets,
  Page,
  TimelineEntry,
} from '../types'

// Customer-management store. Two shapes live here on purpose:
//
//  - the ACTIVE PROFILE (profile/notes/timeline), which the conversation
//    sidebar drives and which follows whichever chat is open, and
//  - the CUSTOMERS LIST plus follow-up views, which the standalone pages drive.
//
// They share the org's tag/status/custom-field catalogs, which is why those are
// loaded once here rather than by each consumer.

type FollowupFilter = 'all' | 'today' | 'overdue' | 'any' | 'none'

interface ListResponse<T> {
  items: T[]
}

// tzParam is the caller's UTC offset, which every day-bucketed endpoint needs:
// organizations have no stored timezone, so "today" and "overdue" are resolved
// against the browser's own offset (see the backend's clientDayStart).
function tzParam(): string {
  return `tz_offset_minutes=${currentUtcOffsetMinutes()}`
}

function errorMessage(e: unknown, fallback: string): string {
  return e instanceof ApiError ? e.message : fallback
}

export const useCrm = defineStore('crm', {
  state: () => ({
    // catalogs (org-wide, loaded once)
    tags: [] as CustomerTag[],
    statuses: [] as CustomerStatus[],
    customFields: [] as CustomFieldDef[],
    catalogsLoaded: false,

    // active customer (the conversation sidebar)
    profile: null as CustomerProfile | null,
    notes: [] as CustomerNote[],
    timeline: [] as TimelineEntry[],
    loadingProfile: false,
    savingProfile: false,
    profileError: '',
    // profileAbort/timelineAbort are INB-08's race guard for the sidebar's
    // customerId watcher: rapid chat switching fires loadProfile/loadTimeline
    // for each intermediate customer, and without this the slowest response
    // — not the latest request — can win and show the wrong customer.
    profileAbort: null as AbortController | null,
    timelineAbort: null as AbortController | null,

    // customers list (the Клиенты page)
    customers: [] as Customer[],
    customersTotal: 0,
    loadingCustomers: false,
    query: '',
    statusFilter: null as string | null,
    tagFilter: null as string | null,
    channelFilter: null as string | null,
    assigneeFilter: 'all' as 'all' | 'me' | 'unassigned' | string,
    followupFilter: 'all' as FollowupFilter,

    // follow-ups (the Задачи page + the sidebar's next action)
    followups: [] as Followup[],
    buckets: { today: 0, tomorrow: 0, this_week: 0, overdue: 0 } as FollowupBuckets,
    bucketFilter: 'today' as 'today' | 'tomorrow' | 'week' | 'overdue',
    followupAssignee: 'me' as 'all' | 'me' | 'unassigned',
    loadingFollowups: false,
  }),
  getters: {
    activeCustomer(state): Customer | null {
      return state.profile?.customer ?? null
    },
    // tagById/statusById back the chips the sidebar renders from ids alone.
    tagById(state) {
      return (id: string) => state.tags.find((t) => t.id === id) || null
    },
    statusById(state) {
      return (id: string) => state.statuses.find((s) => s.id === id) || null
    },
  },
  actions: {
    // --- catalogs ----------------------------------------------------------

    // loadCatalogs fetches the org's tags, statuses and custom-field
    // definitions. Idempotent: the sidebar and the customers page both call it
    // on mount, and only the first one does the work.
    async loadCatalogs(force = false) {
      if (this.catalogsLoaded && !force) return
      try {
        const [tags, statuses, fields] = await Promise.all([
          api.get<ListResponse<CustomerTag>>('/crm/tags'),
          api.get<ListResponse<CustomerStatus>>('/crm/statuses'),
          api.get<ListResponse<CustomFieldDef>>('/crm/custom-fields'),
        ])
        this.tags = tags.items
        this.statuses = statuses.items
        this.customFields = fields.items
        this.catalogsLoaded = true
      } catch (e) {
        log.warn('load crm catalogs failed', { error: String(e) })
      }
    },

    // --- active customer profile -------------------------------------------

    async loadProfile(customerId: string | null) {
      this.profileAbort?.abort()
      if (!customerId) {
        this.profileAbort = null
        this.profile = null
        this.notes = []
        this.timeline = []
        return
      }
      const ctrl = new AbortController()
      this.profileAbort = ctrl
      this.loadingProfile = true
      this.profileError = ''
      try {
        const p = await api.get<CustomerProfile>(`/customers/${customerId}`, undefined, ctrl.signal)
        if (this.profileAbort !== ctrl) return // superseded by a newer selection
        this.profile = p
      } catch (e) {
        if (isAbortError(e)) return
        if (this.profileAbort !== ctrl) return
        this.profile = null
        this.profileError = errorMessage(e, 'Не удалось загрузить карточку клиента.')
      } finally {
        if (this.profileAbort === ctrl) this.loadingProfile = false
      }
    },

    async loadNotes(customerId: string) {
      const p = await api.get<ListResponse<CustomerNote>>(`/customers/${customerId}/notes`)
      this.notes = p.items
    },

    async loadTimeline(customerId: string) {
      this.timelineAbort?.abort()
      const ctrl = new AbortController()
      this.timelineAbort = ctrl
      try {
        const p = await api.get<ListResponse<TimelineEntry>>(`/customers/${customerId}/timeline`, undefined, ctrl.signal)
        if (this.timelineAbort !== ctrl) return
        this.timeline = p.items
      } catch (e) {
        if (isAbortError(e)) return
        log.warn('load timeline failed', { customerId, error: String(e) })
      }
    },

    // patchCustomer applies one inline edit. Only the touched keys are sent —
    // an absent key means "leave alone" on the backend, so two managers editing
    // different fields do not overwrite each other.
    async patchCustomer(customerId: string, patch: Record<string, unknown>) {
      this.savingProfile = true
      this.profileError = ''
      try {
        const updated = await api.patch<Customer>(`/customers/${customerId}`, patch)
        this.applyCustomer(updated)
      } catch (e) {
        this.profileError = errorMessage(e, 'Не удалось сохранить изменения.')
      } finally {
        this.savingProfile = false
      }
    },

    async addTag(customerId: string, tagId: string) {
      try {
        const updated = await api.post<Customer>(`/customers/${customerId}/tags`, { tag_id: tagId })
        this.applyCustomer(updated)
      } catch (e) {
        this.profileError = errorMessage(e, 'Не удалось добавить тег.')
      }
    },

    async removeTag(customerId: string, tagId: string) {
      try {
        const updated = await api.del<Customer>(`/customers/${customerId}/tags/${tagId}`)
        this.applyCustomer(updated)
      } catch (e) {
        this.profileError = errorMessage(e, 'Не удалось убрать тег.')
      }
    },

    async createTag(name: string, color = ''): Promise<CustomerTag | null> {
      try {
        const tag = await api.post<CustomerTag>('/crm/tags', { name, color })
        this.tags.push(tag)
        this.tags.sort((a, b) => a.name.localeCompare(b.name))
        return tag
      } catch (e) {
        this.profileError = errorMessage(e, 'Не удалось создать тег.')
        return null
      }
    },

    async addNote(customerId: string, body: string) {
      const note = await api.post<CustomerNote>(`/customers/${customerId}/notes`, { body })
      this.notes.unshift(note)
      if (this.profile && this.profile.customer.id === customerId) {
        this.profile.latest_note = note
      }
      // The note is a timeline event too, so refresh it if it is on screen.
      if (this.timeline.length) await this.loadTimeline(customerId)
    },

    async deleteNote(customerId: string, noteId: string) {
      await api.del(`/customers/${customerId}/notes/${noteId}`)
      this.notes = this.notes.filter((n) => n.id !== noteId)
      if (this.profile?.latest_note?.id === noteId) {
        this.profile.latest_note = this.notes[0] ?? null
      }
    },

    // applyCustomer writes a fresh customer into whichever collections hold it,
    // so one PATCH updates the sidebar and the list without a refetch of either.
    applyCustomer(c: Customer) {
      if (this.profile && this.profile.customer.id === c.id) {
        this.profile.customer = c
      }
      const i = this.customers.findIndex((x) => x.id === c.id)
      if (i >= 0) this.customers[i] = c
    },

    // --- customers list ----------------------------------------------------

    async loadCustomers() {
      this.loadingCustomers = true
      try {
        const params = new URLSearchParams()
        if (this.query) params.set('q', this.query)
        if (this.statusFilter) params.set('status_id', this.statusFilter)
        if (this.tagFilter) params.set('tag_id', this.tagFilter)
        if (this.channelFilter) params.set('channel', this.channelFilter)
        if (this.assigneeFilter !== 'all') params.set('assignee', this.assigneeFilter)
        if (this.followupFilter !== 'all') params.set('followup', this.followupFilter)
        params.set('page_size', '100')
        params.set('tz_offset_minutes', String(currentUtcOffsetMinutes()))
        const p = await api.get<Page<Customer>>('/customers?' + params.toString())
        this.customers = p.items
        this.customersTotal = p.total
      } finally {
        this.loadingCustomers = false
      }
    },

    async createCustomer(patch: Record<string, unknown>): Promise<Customer> {
      const c = await api.post<Customer>('/customers', patch)
      this.customers.unshift(c)
      this.customersTotal += 1
      return c
    },

    // mergeCustomers folds source into target. Always an explicit operator
    // action — nothing merges automatically, by name or otherwise.
    async mergeCustomers(targetId: string, sourceId: string): Promise<Customer> {
      const merged = await api.post<Customer>(`/customers/${targetId}/merge`, {
        source_customer_id: sourceId,
      })
      this.customers = this.customers.filter((c) => c.id !== sourceId)
      this.applyCustomer(merged)
      if (this.profile?.customer.id === targetId) await this.loadProfile(targetId)
      return merged
    },

    // --- follow-ups --------------------------------------------------------

    async loadBuckets() {
      try {
        const params = new URLSearchParams(tzParam())
        if (this.followupAssignee !== 'all') params.set('assignee', this.followupAssignee)
        this.buckets = await api.get<FollowupBuckets>('/followups/buckets?' + params.toString())
      } catch (e) {
        log.warn('load followup buckets failed', { error: String(e) })
      }
    },

    async loadFollowups() {
      this.loadingFollowups = true
      try {
        const params = new URLSearchParams(tzParam())
        params.set('bucket', this.bucketFilter)
        params.set('page_size', '200')
        if (this.followupAssignee !== 'all') params.set('assignee', this.followupAssignee)
        const p = await api.get<Page<Followup>>('/followups?' + params.toString())
        this.followups = p.items
      } finally {
        this.loadingFollowups = false
      }
    },

    async createFollowup(body: Record<string, unknown>): Promise<Followup> {
      const fu = await api.post<Followup>('/followups', body)
      this.afterFollowupChange(fu)
      return fu
    },

    async rescheduleFollowup(id: string, body: Record<string, unknown>): Promise<Followup> {
      const fu = await api.patch<Followup>(`/followups/${id}`, body)
      this.afterFollowupChange(fu)
      return fu
    },

    async completeFollowup(id: string) {
      this.afterFollowupChange(await api.post<Followup>(`/followups/${id}/complete`))
    },

    async cancelFollowup(id: string) {
      this.afterFollowupChange(await api.post<Followup>(`/followups/${id}/cancel`))
    },

    // afterFollowupChange keeps every view that can show this follow-up in
    // sync: the list it may have left, the bucket counts, and the sidebar's
    // "next action" line.
    afterFollowupChange(fu: Followup) {
      this.followups = this.followups.filter((f) => f.id !== fu.id)
      if (fu.state === 'open' && this.matchesBucket(fu)) this.followups.push(fu)
      this.followups.sort((a, b) => a.due_at.localeCompare(b.due_at))
      void this.loadBuckets()
      if (this.profile?.customer.id === fu.customer_id) {
        void this.loadProfile(fu.customer_id)
      }
    },

    // matchesBucket decides locally whether a just-changed follow-up still
    // belongs in the visible bucket, so completing one row does not need a
    // full refetch. Boundaries mirror the backend's bucketWindow.
    matchesBucket(fu: Followup): boolean {
      const due = new Date(fu.due_at)
      const start = new Date()
      start.setHours(0, 0, 0, 0)
      const day = 24 * 60 * 60 * 1000
      switch (this.bucketFilter) {
        case 'today':
          return due >= start && due < new Date(start.getTime() + day)
        case 'tomorrow':
          return due >= new Date(start.getTime() + day) && due < new Date(start.getTime() + 2 * day)
        case 'week':
          return due >= start && due < new Date(start.getTime() + 7 * day)
        case 'overdue':
          return due < start
        default:
          return true
      }
    },

    // --- realtime ----------------------------------------------------------

    // applyCustomerEvent handles the customer.updated SSE frame: refresh the
    // row wherever it is held, but never pull a customer the user is not
    // looking at into view.
    applyCustomerEvent(c: Customer) {
      this.applyCustomer(c)
    },

    applyFollowupEvent(fu: Followup) {
      const known = this.followups.some((f) => f.id === fu.id)
      if (known || this.matchesBucket(fu)) this.afterFollowupChange(fu)
      else void this.loadBuckets()
    },
  },
})
