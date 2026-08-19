import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { ChannelSetupEntry, ChannelSetupInfo, MetaStaleAccount, SetupKey } from '../types'

// GuidedChannel is the subset of ChannelName a guided setup run can target —
// telegram/whatsapp never consult channel setup at all (AddAccountDialog's
// pickChannel), so they are deliberately excluded here.
export type GuidedChannel = 'instagram' | 'messenger' | 'whatsapp_cloud'

// useChannelSetup is the Channel setup tab's data layer (Accounts.vue) AND
// the guided "Add channel" flow's routing brain (AddAccountDialog.vue's
// pickChannel) — see backend/internal/httpapi/channel_setup.go for the
// five-entry status model this mirrors.
export const useChannelSetup = defineStore('channelSetup', {
  state: () => ({
    entries: [] as ChannelSetupEntry[],
    canConfigure: false,
    publicBaseURL: '',
    loading: false,
    loaded: false,
    // Admin-only payload — see ChannelSetupInfo's own doc comment. Empty/""
    // for a member response, exactly as the backend leaves them absent.
    verifyToken: '',
    graphAPIVersion: '',
    dashboardURL: '',
    staleAccounts: [] as MetaStaleAccount[],

    // --- guided "Add channel" run state (in-memory only) ------------------
    // pendingChannel is consumed the moment a guided run completes (see
    // Accounts.vue's watcher) and must never survive a reload into a loop —
    // deliberately not persisted anywhere.
    pendingChannel: null as GuidedChannel | null,
    focusedEntry: null as SetupKey | null,
    // messenger/whatsapp_cloud own no installation credential of their own —
    // xchats has no signal that their Meta Dashboard checklist is actually
    // done, so nextRequiredSetup treats reaching that card as "not done"
    // until the admin explicitly confirms it once per session (see
    // confirmChecklist). instagram/meta_app/public_access all have a real
    // save action that IS the completion signal, so they never consult this.
    confirmedChecklists: {} as Partial<Record<GuidedChannel, boolean>>,
  }),
  getters: {
    statusOf: (s) => (key: SetupKey) => s.entries.find((e) => e.key === key)?.status ?? 'not_configured',
    entry: (s) => (key: SetupKey) => s.entries.find((e) => e.key === key),
  },
  actions: {
    applyInfo(info: ChannelSetupInfo) {
      this.entries = info.entries
      this.canConfigure = info.can_configure
      this.publicBaseURL = info.public_base_url
      this.verifyToken = info.verify_token ?? ''
      this.graphAPIVersion = info.graph_api_version ?? ''
      this.dashboardURL = info.dashboard_url ?? ''
      this.staleAccounts = info.stale_accounts ?? []
    },
    async load() {
      this.loading = true
      try {
        this.applyInfo(await api.get<ChannelSetupInfo>('/channel-setup'))
      } finally {
        this.loading = false
        this.loaded = true
      }
    },
    async saveMetaApp(appId: string, appSecret: string, force = false) {
      const info = await api.put<ChannelSetupInfo>('/channel-setup/meta-app', { app_id: appId, app_secret: appSecret, force })
      this.applyInfo(info)
      this.advanceGuidedSetup()
      return info
    },
    async saveInstagramApp(appId: string, appSecret: string) {
      const info = await api.put<ChannelSetupInfo>('/channel-setup/instagram-app', { app_id: appId, app_secret: appSecret })
      this.applyInfo(info)
      this.advanceGuidedSetup()
      return info
    },
    // isEntryDone applies the messenger/whatsapp_cloud confirmation gate
    // (see confirmedChecklists' own doc comment) on top of the entry's raw
    // backend status — everything else is just the backend status.
    isEntryDone(key: SetupKey, channel: GuidedChannel): boolean {
      if (this.statusOf(key) !== 'ready') return false
      if (key === channel && (channel === 'messenger' || channel === 'whatsapp_cloud')) {
        return !!this.confirmedChecklists[channel]
      }
      return true
    },
    // nextRequiredSetup returns the first non-ready prerequisite for channel
    // in dependency order — public_access, meta_app, then the channel's own
    // entry — or null once every one of them is done. Used both to decide
    // whether "Add channel" can proceed straight to connect, and to focus
    // the next card during a guided run (same function, same order, per
    // this feature's own design: "an admin picking Instagram on a fresh
    // install is focused on Public access — not the Instagram card, which
    // they cannot yet act on").
    nextRequiredSetup(channel: GuidedChannel): SetupKey | null {
      const order: SetupKey[] = ['public_access', 'meta_app', channel]
      for (const key of order) {
        if (!this.isEntryDone(key, channel)) return key
      }
      return null
    },
    // startGuidedSetup begins (or restarts) a guided run for channel —
    // AddAccountDialog's pickChannel calls this when an admin picks a Meta
    // channel with something missing.
    startGuidedSetup(channel: GuidedChannel) {
      this.pendingChannel = channel
      this.focusedEntry = this.nextRequiredSetup(channel)
    },
    // advanceGuidedSetup re-focuses the next missing prerequisite after a
    // save/confirm. Returns true once the run is complete (focusedEntry is
    // now null) — Accounts.vue's watcher reads that to reopen
    // AddAccountDialog on pendingChannel and clear the run. A no-op outside
    // an active run.
    advanceGuidedSetup(): boolean {
      if (!this.pendingChannel) return false
      this.focusedEntry = this.nextRequiredSetup(this.pendingChannel)
      return this.focusedEntry === null
    },
    confirmChecklist(channel: GuidedChannel) {
      this.confirmedChecklists = { ...this.confirmedChecklists, [channel]: true }
      this.advanceGuidedSetup()
    },
    clearGuidedSetup() {
      this.pendingChannel = null
      this.focusedEntry = null
    },
  },
})
