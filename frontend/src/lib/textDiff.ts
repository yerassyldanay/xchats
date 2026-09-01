// lineDiff computes a line-level diff via the classic LCS (longest common
// subsequence) dynamic-programming algorithm — O(n*m) in line count, fine
// for the KB fields this backs (a topic body, assistant guardrails, a
// policy note are at most a few dozen lines). Line granularity rather than
// word granularity on purpose: a rewritten sentence reads as noise at word
// level (nearly every word "changed" even when the meaning didn't), while
// line level reads as intent — this paragraph was replaced, that bullet was
// added.
export interface DiffLine {
  type: 'same' | 'added' | 'removed'
  text: string
}

export function lineDiff(before: string, after: string): DiffLine[] {
  // '' means NO lines, not one empty line — String.split disagrees
  // (''.split('\n') === ['']), which would otherwise render a brand-new
  // field's empty "before" as a spurious removed blank line.
  const a = before === '' ? [] : before.split('\n')
  const b = after === '' ? [] : after.split('\n')
  const n = a.length
  const m = b.length

  // dp[i][j] = LCS length of a[i:] and b[j:].
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1])
    }
  }

  const out: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ type: 'same', text: a[i] })
      i++
      j++
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      out.push({ type: 'removed', text: a[i] })
      i++
    } else {
      out.push({ type: 'added', text: b[j] })
      j++
    }
  }
  while (i < n) {
    out.push({ type: 'removed', text: a[i] })
    i++
  }
  while (j < m) {
    out.push({ type: 'added', text: b[j] })
    j++
  }
  return out
}
