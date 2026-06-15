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

// connStatus maps a WhatsApp account connection_status to a label + badge class.
export function connStatus(status: string): { label: string; cls: string; dot: string } {
  switch (status) {
    case 'connected':
      return { label: 'Подключён', cls: 'bg-green-50 text-green-700', dot: 'bg-wa' }
    case 'qr_required':
      return { label: 'Нужен QR', cls: 'bg-amber-50 text-amber-700', dot: 'bg-amber-400' }
    case 'connecting':
      return { label: 'Подключение…', cls: 'bg-sky-50 text-sky-700', dot: 'bg-sky-400' }
    case 'disconnected':
      return { label: 'Отключён', cls: 'bg-slate-100 text-slate-500', dot: 'bg-slate-400' }
    default:
      return { label: status || 'Ошибка', cls: 'bg-red-50 text-red-600', dot: 'bg-red-500' }
  }
}

// delivery ticks (FontAwesome) — colored for the green outgoing bubble:
// queued clock, sent check, delivered double-check, read double-check (blue), failed warning.
export function tick(status: string): { icon: string; cls: string } {
  switch (status) {
    case 'sent':
      return { icon: 'fa-check', cls: 'text-white/70' }
    case 'delivered':
      return { icon: 'fa-check-double', cls: 'text-white/70' }
    case 'read':
      return { icon: 'fa-check-double', cls: 'text-sky-200' }
    case 'failed':
      return { icon: 'fa-triangle-exclamation', cls: 'text-rose-200' }
    default:
      return { icon: 'fa-clock', cls: 'text-white/50' }
  }
}
