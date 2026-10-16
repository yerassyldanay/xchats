import { computed, inject, onBeforeUnmount, provide, ref, type InjectionKey, type UnwrapNestedRefs } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, ApiError } from '@/api/client'
import { log } from '@/lib/logfmt'
import { CURATED_MODELS } from '@/lib/curatedModels'
import { useSettings } from '@/stores/settings'
import { useAccounts } from '@/stores/accounts'
import type { McpConnectionInfo, WaPairStatus } from '@/types'

// useSetupWizard drives the full-screen setup questionnaire (views/Setup.vue):
// welcome → AI key → channel → connect → KB (domain question) → address/ngrok
// → ChatGPT/Claude connector → done. It owns no endpoints of its own — every
// action is one the Settings and Channels pages already perform (store calls
// for the credential/LLM/tunnel/account writes, GET /mcp-connection for the
// connector address), so completing the wizard leaves exactly the state doing
// the same things by hand would.
export const SETUP_STEPS = ['welcome', 'ai', 'channel', 'connect', 'kb', 'address', 'mcp', 'done'] as const
export type SetupStep = (typeof SETUP_STEPS)[number]
export type AiProvider = 'openrouter' | 'openai' | 'gemini'
export type SetupChannel = 'whatsapp' | 'telegram'

export const AI_PROVIDERS: Record<AiProvider, { name: string; keyUrl: string; fallbackModel: string }> = {
  openrouter: { name: 'OpenRouter', keyUrl: 'https://openrouter.ai/keys', fallbackModel: 'google/gemini-2.5-flash' },
  openai: { name: 'OpenAI', keyUrl: 'https://platform.openai.com/api-keys', fallbackModel: 'gpt-4o' },
  gemini: { name: 'Gemini', keyUrl: 'https://aistudio.google.com/apikey', fallbackModel: 'gemini-2.5-flash' },
}

// Where each step sits on the three-segment progress bar: [segment it belongs
// to, how far through that segment]. welcome/done sit at the ends.
const PROGRESS: Record<SetupStep, [number, number]> = {
  welcome: [0, 0],
  ai: [1, 0.5],
  channel: [2, 0.3],
  connect: [2, 0.7],
  kb: [3, 0.25],
  address: [3, 0.55],
  mcp: [3, 0.85],
  done: [4, 0],
}

function initialIndex(q: unknown): number {
  const i = SETUP_STEPS.indexOf(String(q ?? '') as SetupStep)
  return i < 0 ? 0 : i
}

// normalizeAddress turns whatever the operator typed ("chat.mycompany.kz/")
// into an origin without a trailing slash, assuming https when no scheme is
// given. Empty in, empty out.
export function normalizeAddress(raw: string): string {
  const v = raw.trim().replace(/\/+$/, '')
  if (!v) return ''
  return /^https?:\/\//i.test(v) ? v : `https://${v}`
}

export function useSetupWizard(initialStep?: unknown) {
  const { t } = useI18n()
  const settings = useSettings()
  const accounts = useAccounts()

  const index = ref(initialIndex(initialStep))
  const step = computed<SetupStep>(() => SETUP_STEPS[index.value]!)

  // --- step 1: AI provider ---------------------------------------------------
  const provider = ref<AiProvider>('openrouter')
  const showOtherProviders = ref(false)
  const apiKey = ref('')
  const aiBusy = ref(false)
  const aiError = ref('')
  const aiVerified = ref(false)
  const aiModel = ref('')

  function pickProvider(p: AiProvider) {
    provider.value = p
    aiVerified.value = false
    aiError.value = ''
  }

  async function verifyKey() {
    aiError.value = ''
    if (!apiKey.value.trim()) {
      aiError.value = t('setup.ai.keyRequired')
      return
    }
    aiBusy.value = true
    try {
      if (!settings.integrations.length) await settings.loadIntegrations()
      const integ = settings.integrations.find((p) => p.id === provider.value)
      const field = integ?.fields[0]?.key
      if (!integ || !field) {
        aiError.value = t('setup.errors.generic')
        return
      }
      const res = await settings.saveCredential(provider.value, { [field]: apiKey.value.trim() })
      if (!res.verified) {
        aiError.value = t('setup.ai.notVerified')
        return
      }
      // A key for a provider that isn't the default would sit unused, so make
      // the one just verified the default — with a model that provider serves.
      if (!settings.settings) await settings.load()
      const llm = settings.settings?.llm
      const model = integ.default_model || CURATED_MODELS[provider.value]?.[0] || AI_PROVIDERS[provider.value].fallbackModel
      if (llm && llm.default_provider !== provider.value) {
        await settings.updateLLM({ ...llm, default_provider: provider.value, default_model: model })
        aiModel.value = model
      } else {
        aiModel.value = llm?.default_model || model
      }
      aiVerified.value = true
      apiKey.value = '' // the key is stored server-side; never keep it in the page
    } catch (e) {
      aiError.value = e instanceof ApiError ? e.message : t('setup.errors.generic')
    } finally {
      aiBusy.value = false
    }
  }

  // --- step 2/3: channel -----------------------------------------------------
  const channel = ref<SetupChannel>('whatsapp')
  const showMoreChannels = ref(false)
  const connected = ref(false)
  const connectBusy = ref(false)
  const connectError = ref('')

  function pickChannel(c: SetupChannel) {
    if (channel.value !== c) stopPolling()
    channel.value = c
    connected.value = false
    connectError.value = ''
  }

  // WhatsApp: QR pairing (POST /wa-accounts/pair, then poll the session).
  const qr = ref<WaPairStatus | null>(null)
  let sessionId = ''
  let timer: number | undefined

  function stopPolling() {
    if (timer) window.clearInterval(timer)
    timer = undefined
  }
  async function pollPairing(): Promise<void> {
    if (!sessionId) return
    try {
      const r = await accounts.pairStatus(sessionId)
      if (r.status === 'connected') {
        stopPolling()
        connected.value = true
        connectError.value = ''
        await accounts.load().catch(() => {})
        return
      }
      if (r.status === 'timeout' || r.status === 'error') {
        stopPolling()
        qr.value = null
        sessionId = ''
        connectError.value = r.message || t('setup.connect.pairFailed')
        return
      }
      if (r.qr_code || r.qr_base64) qr.value = r
    } catch (e) {
      // A session the backend no longer knows about is terminal; anything else
      // is a transient blip the next tick retries.
      if (e instanceof ApiError && e.status === 404) {
        stopPolling()
        qr.value = null
        sessionId = ''
        connectError.value = t('setup.connect.sessionExpired')
        return
      }
      log.warn('setup qr poll failed', { err: String(e) })
    }
  }
  async function startPairing() {
    if (timer || connected.value) return
    connectError.value = ''
    qr.value = null
    connectBusy.value = true
    try {
      const started = await accounts.pair()
      sessionId = started.session_id
      void pollPairing()
      timer = window.setInterval(pollPairing, 2500)
    } catch (e) {
      connectError.value = e instanceof ApiError ? e.message : t('setup.connect.pairFailed')
    } finally {
      connectBusy.value = false
    }
  }
  // "Я отсканировал": check once, right now, instead of waiting for the tick.
  async function checkPairing() {
    connectBusy.value = true
    try {
      await pollPairing()
      if (!connected.value && !connectError.value) connectError.value = t('setup.connect.notYet')
    } finally {
      connectBusy.value = false
    }
  }

  // Telegram: bot token (POST /telegram-accounts).
  const botToken = ref('')
  const botName = ref('')
  async function connectTelegram() {
    connectError.value = ''
    const token = botToken.value.trim()
    if (!token) {
      connectError.value = t('setup.connect.tokenRequired')
      return
    }
    connectBusy.value = true
    try {
      await accounts.createTelegram(token, botName.value.trim() || 'Telegram', false)
      botToken.value = ''
      connected.value = true
    } catch (e) {
      connectError.value = e instanceof ApiError ? e.message : t('setup.connect.telegramFailed')
    } finally {
      connectBusy.value = false
    }
  }

  // --- step 3: knowledge base via ChatGPT/Claude (MCP) -----------------------
  const answer = ref<'yes' | 'no' | null>(null)
  const domain = ref('')
  const ngrokToken = ref('')
  const tunnelBusy = ref(false)
  const tunnelError = ref('')
  const mcpInfo = ref<McpConnectionInfo | null>(null)
  const copied = ref(false)

  const tunnelRunning = computed(() => !!settings.tunnel?.running)
  const tunnelUrl = computed(() => settings.tunnel?.public_url ?? '')

  async function startTunnel() {
    tunnelError.value = ''
    tunnelBusy.value = true
    try {
      if (!settings.integrations.length) await settings.loadIntegrations()
      const ngrok = settings.integrations.find((p) => p.id === 'ngrok')
      const token = ngrokToken.value.trim()
      if (!token && !ngrok?.configured) {
        tunnelError.value = t('setup.address.tokenRequired')
        return
      }
      if (token) {
        const field = ngrok?.fields[0]?.key
        if (!field) {
          tunnelError.value = t('setup.errors.generic')
          return
        }
        await settings.saveCredential('ngrok', { [field]: token })
        ngrokToken.value = ''
      }
      await settings.startTunnel()
      if (!settings.tunnel?.running) tunnelError.value = settings.tunnel?.last_error || t('setup.address.tunnelFailed')
    } catch (e) {
      tunnelError.value = e instanceof ApiError ? e.message : t('setup.errors.generic')
    } finally {
      tunnelBusy.value = false
    }
  }

  async function loadMcpInfo() {
    try {
      mcpInfo.value = await api.get<McpConnectionInfo>('/mcp-connection')
    } catch (e) {
      log.warn('setup mcp-connection failed', { err: String(e) })
    }
  }

  // The origin ChatGPT/Claude will reach: the address the operator typed, or
  // the running ngrok tunnel's. The path (/mcp) is the server's own.
  const baseUrl = computed(() => (answer.value === 'yes' ? normalizeAddress(domain.value) : tunnelUrl.value || mcpInfo.value?.public_url || ''))
  const mcpUrl = computed(() => (baseUrl.value ? `${baseUrl.value.replace(/\/+$/, '')}/mcp` : (mcpInfo.value?.mcp_url ?? '')))

  async function copyMcpUrl() {
    try {
      await navigator.clipboard.writeText(mcpUrl.value)
    } catch {
      // Clipboard can be unavailable (insecure context); the URL stays visible to copy by hand.
    }
    copied.value = true
    window.setTimeout(() => (copied.value = false), 2000)
  }

  // --- navigation ------------------------------------------------------------
  function go(i: number) {
    index.value = Math.max(0, Math.min(SETUP_STEPS.length - 1, i))
    if (step.value === 'connect') {
      if (channel.value === 'whatsapp') void startPairing()
    } else {
      stopPolling()
    }
    if (step.value === 'mcp') void loadMcpInfo()
    try {
      window.scrollTo(0, 0)
    } catch {
      // jsdom / non-browser — nothing to scroll.
    }
  }
  // An entry straight into a later step (?step=connect) still has to start its work.
  go(index.value)

  async function next() {
    switch (step.value) {
      case 'ai':
        if (!aiVerified.value) return verifyKey()
        return go(index.value + 1)
      case 'connect':
        if (!connected.value) return channel.value === 'whatsapp' ? checkPairing() : connectTelegram()
        return go(index.value + 1)
      case 'kb':
        if (!answer.value) return
        return go(index.value + 1)
      case 'address':
        if (answer.value === 'yes') {
          if (!normalizeAddress(domain.value)) return
          return go(index.value + 1)
        }
        if (!tunnelRunning.value) return startTunnel()
        return go(index.value + 1)
      default:
        return go(index.value + 1)
    }
  }
  function back() {
    go(index.value - 1)
  }
  function skip() {
    const id = step.value
    go(SETUP_STEPS.indexOf(id === 'ai' ? 'channel' : id === 'channel' || id === 'connect' ? 'kb' : 'done'))
  }

  const canSkip = computed(() => ['ai', 'channel', 'connect', 'kb', 'address', 'mcp'].includes(step.value))
  const busy = computed(() => aiBusy.value || connectBusy.value || tunnelBusy.value)
  // Steps whose primary action needs an input before it can fire.
  const canNext = computed(() => {
    if (busy.value) return false
    if (step.value === 'kb') return answer.value !== null
    if (step.value === 'address' && answer.value === 'yes') return !!normalizeAddress(domain.value)
    return true
  })
  const nextLabel = computed(() => {
    const id = step.value
    if (id === 'welcome') return t('setup.nav.start')
    if (id === 'ai' && !aiVerified.value) return t('setup.nav.verifyKey')
    if (id === 'connect' && !connected.value) return channel.value === 'whatsapp' ? t('setup.nav.scanned') : t('setup.nav.connect')
    if (id === 'address' && answer.value !== 'yes' && !tunnelRunning.value) return t('setup.nav.startTunnel')
    if (id === 'mcp') return t('setup.nav.finish')
    return t('setup.nav.next')
  })
  // Three-segment bar: each value is that segment's fill, 0..100.
  const progress = computed(() => {
    const [stage, within] = PROGRESS[step.value]
    return [1, 2, 3].map((n) => (stage > n ? 100 : stage === n ? Math.round(within * 100) : 0))
  })

  onBeforeUnmount(stopPolling)

  return {
    step, index, go, next, back, skip, canSkip, canNext, nextLabel, busy, progress,
    provider, pickProvider, showOtherProviders, apiKey, aiBusy, aiError, aiVerified, aiModel, verifyKey,
    channel, pickChannel, showMoreChannels, connected, connectBusy, connectError,
    qr, startPairing, checkPairing, botToken, botName, connectTelegram,
    answer, domain, ngrokToken, tunnelBusy, tunnelError, tunnelRunning, tunnelUrl, startTunnel,
    mcpInfo, mcpUrl, copied, copyMcpUrl,
  }
}

// The wizard is created once by views/Setup.vue (wrapped in reactive() so the
// step components see plain values, not refs) and handed down by injection —
// the steps write straight into it (v-model on its fields), which a prop would
// forbid.
export type SetupWizard = UnwrapNestedRefs<ReturnType<typeof useSetupWizard>>
const WIZARD_KEY: InjectionKey<SetupWizard> = Symbol('setupWizard')
export function provideSetupWizard(w: SetupWizard) {
  provide(WIZARD_KEY, w)
}
export function useSetupWizardContext(): SetupWizard {
  const w = inject(WIZARD_KEY)
  if (!w) throw new Error('useSetupWizardContext: no setup wizard provided')
  return w
}
