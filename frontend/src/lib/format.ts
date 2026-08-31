// UI formatting helpers. Local time is computed only here, at the UI edge
// (the backend stores/serves UTC instants).
//
// Nothing here returns a translated string: this module is imported by
// stores and components alike, and a label baked in at import time would be
// frozen to whichever locale was active then. Functions that used to return
// Russian now return an i18n key (see connStatus) and the caller translates.

// intlLocale maps a vue-i18n locale code onto the BCP-47 tag Intl wants.
// Single source of truth so a fourth locale is one line, not a ternary
// duplicated across every component that formats a date or a number.
export function intlLocale(locale: string): string {
  switch (locale) {
    case 'en':
      return 'en-US'
    case 'kk':
      return 'kk-KZ'
    default:
      return 'ru-RU'
  }
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[1][0]).toUpperCase()
}

const palette = ['#4F46E5', '#0EA5E9', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6', '#EC4899']
export function colorFor(id: string): string {
  let h = 0
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0
  return palette[h % palette.length]
}

// formatDateTime renders an absolute instant (day, month, year, hour:minute)
// — unlike shortTime's today-vs-not split, entries spanning many days (e.g.
// KB-14's import history list) need the date spelled out every time, not
// just when it isn't today.
export function formatDateTime(iso: string, locale = 'ru'): string {
  if (!iso) return ''
  return new Date(iso).toLocaleString(intlLocale(locale), {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function shortTime(iso: string | null, locale = 'ru'): string {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const tag = intlLocale(locale)
  return sameDay
    ? d.toLocaleTimeString(tag, { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString(tag, { day: '2-digit', month: '2-digit' })
}

// connStatus maps an account connection state to an i18n key under
// channels.connStatus.* plus a UI-agnostic tone discriminant; the component
// translates the key and maps the tone to badge/dot classes. It covers both
// lifecycles: the WhatsApp QR states and the Telegram webhook states. An
// unrecognised status has no key of its own — `key` is null and the caller
// shows the raw status (or channels.connStatus.error when it is empty).
export type ConnTone = 'connected' | 'qr' | 'connecting' | 'disconnected' | 'error'
export type ConnStatusKey =
  | 'connected'
  | 'qr_required'
  | 'connecting'
  | 'disconnected'
  | 'webhook_error'
  | 'token_error'
  | 'disconnect_pending'
  | 'disconnect_error'
  | 'error'
export function connStatus(status: string): { key: ConnStatusKey | null; tone: ConnTone } {
  switch (status) {
    case 'connected':
      return { key: 'connected', tone: 'connected' }
    case 'qr_required':
      return { key: 'qr_required', tone: 'qr' }
    case 'connecting':
      return { key: 'connecting', tone: 'connecting' }
    case 'disconnected':
      return { key: 'disconnected', tone: 'disconnected' }
    // --- Telegram ---
    case 'webhook_error':
      return { key: 'webhook_error', tone: 'error' }
    case 'token_error':
      return { key: 'token_error', tone: 'error' }
    case 'disconnect_pending':
      return { key: 'disconnect_pending', tone: 'connecting' }
    case 'disconnect_error':
      return { key: 'disconnect_error', tone: 'error' }
    default:
      return { key: status ? null : 'error', tone: 'error' }
  }
}

// formatBytes renders a byte count the way file managers do (1000-based
// decimal units, IEC abbreviations left untranslated like every OS does) —
// used by the KB materials tab and MediaStrip's document rows.
export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1000)), units.length - 1)
  const value = bytes / 1000 ** i
  return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`
}

// formatElapsed renders a duration the way a live progress counter should
// (KB-04's own "elapsed time counter") — coarsest-unit-first, no
// zero-padding (this is a running total, not a clock face). ms is always a
// wall-clock delta the caller computes fresh (Date.now() - startedAt), so
// this function itself stays a pure formatter with no timer of its own.
export function formatElapsed(ms: number, t: (key: string, named?: Record<string, unknown>) => string): string {
  const totalSeconds = Math.max(0, Math.round(ms / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  if (hours > 0) return t('common.elapsed.hm', { h: hours, m: minutes })
  if (minutes > 0) return t('common.elapsed.ms', { m: minutes, s: seconds })
  return t('common.elapsed.s', { s: seconds })
}

// tick maps a message delivery status to a UI-agnostic discriminant. The
// component maps the discriminant to an icon + class, keeping this module pure.
export type TickStatus = 'queued' | 'sent' | 'delivered' | 'read' | 'failed'
export function tick(status: string): TickStatus {
  switch (status) {
    case 'sent':
      return 'sent'
    case 'delivered':
      return 'delivered'
    case 'read':
      return 'read'
    case 'failed':
      return 'failed'
    default:
      return 'queued'
  }
}
