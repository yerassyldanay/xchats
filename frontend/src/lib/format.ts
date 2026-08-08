// UI formatting helpers. Local time is computed only here, at the UI edge
// (the backend stores/serves UTC instants).
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

export function shortTime(iso: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  return sameDay
    ? d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' })
}

// connStatus maps an account connection state to a Russian label + a UI-agnostic
// tone discriminant; the component maps the tone to badge/dot classes. It covers
// both lifecycles: the WhatsApp QR states and the Telegram webhook states.
export type ConnTone = 'connected' | 'qr' | 'connecting' | 'disconnected' | 'error'
export function connStatus(status: string): { label: string; tone: ConnTone } {
  switch (status) {
    case 'connected':
      return { label: 'Подключён', tone: 'connected' }
    case 'qr_required':
      return { label: 'Нужен QR', tone: 'qr' }
    case 'connecting':
      return { label: 'Подключение…', tone: 'connecting' }
    case 'disconnected':
      return { label: 'Отключён', tone: 'disconnected' }
    // --- Telegram ---
    case 'webhook_error':
      return { label: 'Ошибка вебхука', tone: 'error' }
    case 'token_error':
      return { label: 'Токен отклонён', tone: 'error' }
    case 'disconnect_pending':
      return { label: 'Отключение…', tone: 'connecting' }
    case 'disconnect_error':
      return { label: 'Не отключился', tone: 'error' }
    default:
      return { label: status || 'Ошибка', tone: 'error' }
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
