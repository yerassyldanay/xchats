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

    // follow-ups (the Задачи board + the sidebar's next action). `followups`
    // is every OPEN one in scope, unbounded by due date — TODO.md's board
    // groups them into Overdue/Today/Tomorrow/Later sections client-side
    // (Followups.vue), rather than the server filtering to one bucket at a
    // time the way the old single-bucket list view did.
    followups: [] as Followup[],
    buckets: { today: 0, tomorrow: 0, this_week: 0, overdue: 0 } as FollowupBuckets,
    followupAssignee: 'me' as 'all' | 'me' | 'unassigned',
    loadingFollowups: false,
    // Completed/cancelled history (TODO.md "Completed" tab) — a separate
    // list from the open board above, sorted newest-completed-first.
    completedFollowups: [] as Followup[],
    completedTotal: 0,
    loadingCompletedFollowups: false,
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

    // loadFollowups fetches every OPEN follow-up in scope, unbounded by due
    // date — the board (Followups.vue) groups the result into time sections
    // itself. bucket= is deliberately never sent: the backend's own bucket
    // filter both windows by date AND implicitly restricts to state=open, so
    // asking for state=open with no bucket returns the complete open set in
    // one request instead of one per section.
    async loadFollowups() {
      this.loadingFollowups = true
      try {
        const params = new URLSearchParams(tzParam())
        params.set('state', 'open')
        params.set('page_size', '200')
        if (this.followupAssignee !== 'all') params.set('assignee', this.followupAssignee)
        const p = await api.get<Page<Followup>>('/followups?' + params.toString())
        this.followups = p.items
      } finally {
        this.loadingFollowups = false
      }
    },

    // loadCompletedFollowups is the "Completed" tab's own history view — a
    // separate list from the open board above, newest-completed-first (the
    // backend itself orders by due_at, which reads oddly for a history log).
    async loadCompletedFollowups() {
      this.loadingCompletedFollowups = true
      try {
        const params = new URLSearchParams()
        params.set('state', 'completed')
        params.set('page_size', '100')
        if (this.followupAssignee !== 'all') params.set('assignee', this.followupAssignee)
        const p = await api.get<Page<Followup>>('/followups?' + params.toString())
        this.completedFollowups = [...p.items].sort((a, b) => (b.completed_at ?? '').localeCompare(a.completed_at ?? ''))
        this.completedTotal = p.total
      } finally {
        this.loadingCompletedFollowups = false
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

    // reopenFollowup is the Completed tab's "put this back on the board"
    // (TODO.md "Allow reopening accidentally completed tasks"). There is no
    // dedicated reopen endpoint: RescheduleFollowup already resets a
    // completed/cancelled item back to state=open as a side effect (see its
    // own doc comment in crm_followups.go), so echoing the item's own
    // due/action/note/assignee back through PATCH does exactly that without
    // actually changing when it's due.
    async reopenFollowup(fu: Followup): Promise<Followup> {
      const reopened = await api.patch<Followup>(`/followups/${fu.id}`, {
        customer_id: fu.customer_id,
        conversation_id: fu.conversation_id,
        channel: fu.channel,
        due_at: fu.due_at,
        due_date: fu.due_date,
        due_minute: fu.due_minute,
        action: fu.action,
        note: fu.note,
        assignee_user_id: fu.assignee_user_id,
      })
      this.completedFollowups = this.completedFollowups.filter((f) => f.id !== fu.id)
      this.completedTotal = Math.max(0, this.completedTotal - 1)
      this.afterFollowupChange(reopened)
      return reopened
    },

    // afterFollowupChange keeps every view that can show this follow-up in
    // sync: the board it may have left (any state other than open), the
    // bucket counts, the Completed history if it just landed there, and the
    // sidebar's "next action" line.
    afterFollowupChange(fu: Followup) {
      this.followups = this.followups.filter((f) => f.id !== fu.id)
      if (fu.state === 'open') this.followups.push(fu)
      this.followups.sort((a, b) => a.due_at.localeCompare(b.due_at))
      this.completedFollowups = this.completedFollowups.filter((f) => f.id !== fu.id)
      if (fu.state === 'completed') this.completedFollowups.unshift(fu)
      void this.loadBuckets()
      if (this.profile?.customer.id === fu.customer_id) {
        void this.loadProfile(fu.customer_id)
      }
    },

    // --- realtime ----------------------------------------------------------

    // applyCustomerEvent handles the customer.updated SSE frame: refresh the
    // row wherever it is held, but never pull a customer the user is not
    // looking at into view.
    applyCustomerEvent(c: Customer) {
      this.applyCustomer(c)
    },

    // The board shows every open follow-up in scope regardless of due date
    // now, so — unlike the old single-bucket list — there is no "does this
    // even belong in the visible window" branch left to short-circuit on.
    applyFollowupEvent(fu: Followup) {
      this.afterFollowupChange(fu)
    },
  },
})
