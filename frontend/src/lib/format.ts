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

// delivery ticks: queued ·, sent ✓, delivered ✓✓, read ✓✓ (blue), failed ✕
export function tick(status: string): { glyph: string; cls: string } {
  switch (status) {
    case 'sent':
      return { glyph: '✓', cls: 'text-slate-400' }
    case 'delivered':
      return { glyph: '✓✓', cls: 'text-slate-400' }
    case 'read':
      return { glyph: '✓✓', cls: 'text-sky-500' }
    case 'failed':
      return { glyph: '✕', cls: 'text-red-500' }
    default:
      return { glyph: '·', cls: 'text-slate-300' }
  }
}
