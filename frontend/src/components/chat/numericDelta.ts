// numericDelta answers one narrow question for the comparison card: when a
// field changed, by how much?
//
// It is deliberately conservative. A knowledge base holds free text — "12 000
// ₸", "2-3 дня", "по договорённости" — and only some of it is arithmetic. A
// delta is computed ONLY when both sides are unambiguously the same kind of
// number in the same unit; everything else renders as a plain before/after
// pair. Guessing here would put a fabricated number on a card whose entire
// purpose is to be more trustworthy than the prose above it.

export interface Delta {
  /** The signed difference, formatted with its unit — e.g. "−1 200 ₸". */
  label: string
  /** True when the draft value is larger than the live one. */
  increased: boolean
}

// A parsed value: its magnitude, and the unit text around it (currency
// symbol, "дн.", "%", ...). Two values only compare if their units match.
interface Parsed {
  value: number
  unit: string
}

// One number, allowing the spaces and separators a price is grouped with
// ("12 000", "1,234.50") but always ending on a digit.
const NUMBER = /-?\d(?:[\d\s.,]*\d)?/g

/**
 * numericDelta returns the draft-minus-real difference, or null when the two
 * values are not comparable numbers (or are equal).
 */
export function numericDelta(real: string, draft: string): Delta | null {
  const a = parseValue(real)
  const b = parseValue(draft)
  if (!a || !b || a.unit !== b.unit) return null
  const diff = b.value - a.value
  if (diff === 0) return null

  const magnitude = formatNumber(Math.abs(diff))
  const sign = diff > 0 ? '+' : '−' // a real minus sign, not a hyphen
  return {
    label: a.unit ? `${sign}${magnitude} ${a.unit}` : `${sign}${magnitude}`,
    increased: diff > 0,
  }
}

// parseValue splits "12 000 ₸" into 12000 and "₸". Returns null unless the
// text holds exactly one standalone number: "2-3 дня" has two, and the "3" in
// "Vitamin D3" is part of a word, not a quantity to subtract.
function parseValue(raw: string): Parsed | null {
  const text = raw.trim()
  if (!text) return null

  const matches = [...text.matchAll(NUMBER)]
  if (matches.length !== 1) return null
  const [match] = matches
  const at = match.index ?? 0
  // A digit glued to the end of a word is part of that word.
  if (at > 0 && /[\p{L}\d]/u.test(text[at - 1])) return null

  const value = normalizeNumber(match[0].replace(/\s/g, ''))
  if (value === null) return null

  const unit = (text.slice(0, at) + text.slice(at + match[0].length)).replace(/\s+/g, ' ').trim()
  return { value, unit }
}

// normalizeNumber resolves "12000" / "12.5" / "12,5" / "12,000" into a JS
// number.
//
// The separator rules are the ambiguous part: a comma is a thousands
// separator in English and a decimal comma in Russian and Kazakh, and this
// knowledge base holds all three languages. So the shape decides — a
// separator followed by exactly three digits, with more digits before it, is
// grouping; anything else is a decimal point. A value that fits neither
// reading is rejected rather than guessed at.
function normalizeNumber(raw: string): number | null {
  let text = raw
  // Strip grouping separators, repeatedly (1,234,567 has two).
  for (;;) {
    const stripped = text.replace(/(\d)[.,](\d{3})(?=\D|$)/, '$1$2')
    if (stripped === text) break
    text = stripped
  }
  // Whatever separator is left is decimal.
  text = text.replace(',', '.')
  if (!/^-?\d+(\.\d+)?$/.test(text)) return null
  const value = Number(text)
  return Number.isFinite(value) ? value : null
}

// formatNumber groups with plain spaces, matching how prices are written in
// this product's own knowledge bases ("12 000"), and keeps at most two
// decimals. Intl's own group separator is a narrow no-break space, replaced
// here so the label copies and compares as ordinary text.
function formatNumber(value: number): string {
  return value.toLocaleString('ru-RU', { maximumFractionDigits: 2 }).replace(/\s/g, ' ')
}
