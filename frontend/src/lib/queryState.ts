// Reading small bits of list state (page numbers, a filter value) back out
// of a route's query object. vue-router types a query value as
// `string | (string | null)[] | null` — repeated keys serialize as an
// array — so every read needs to unwrap that before use.
export function queryInt(v: unknown, fallback: number): number {
  const raw = Array.isArray(v) ? v[0] : v
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : fallback
}

export function queryString(v: unknown): string {
  const raw = Array.isArray(v) ? v[0] : v
  return typeof raw === 'string' ? raw : ''
}
