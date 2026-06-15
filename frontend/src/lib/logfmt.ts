// A tiny logfmt logger — the SAME key=value format as the Go backend, so a
// dev reading both consoles sees one shape. Build 0 keeps frontend logs in the
// browser console (shipping to a backend sink is a later add).
type Fields = Record<string, unknown>

function val(v: unknown): string {
  if (v === null || v === undefined) return '""'
  const s = String(v)
  return /[\s"=]/.test(s) ? JSON.stringify(s) : s
}

function line(level: string, msg: string, fields: Fields): string {
  let out = `ts=${new Date().toISOString()} level=${level} msg=${val(msg)}`
  for (const [k, v] of Object.entries(fields)) out += ` ${k}=${val(v)}`
  return out
}

export const log = {
  info: (msg: string, f: Fields = {}) => console.log(line('info', msg, f)),
  warn: (msg: string, f: Fields = {}) => console.warn(line('warn', msg, f)),
  error: (msg: string, f: Fields = {}) => console.error(line('error', msg, f)),
}
