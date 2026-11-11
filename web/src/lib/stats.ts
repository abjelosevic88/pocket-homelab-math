// Numbers in a table: averages, spread and trend lines.

/** A cell as a number ("1,234.5", "12,5", "€30", "45 %", a date → days). */
export function cellNumber(s: string): number | null {
  const t = (s || '').trim()
  if (!t) return null
  if (/^\d{4}-\d{2}(-\d{2})?$/.test(t)) {
    const [y, m, d] = t.split('-').map(Number)
    return Date.UTC(y, m - 1, d || 1) / 86400000
  }
  let c = t.replace(/[^\d.,\-eE+]/g, '')
  if (!c || !/\d/.test(c)) return null
  if (/^-?\d{1,3}(\.\d{3})+(,\d+)?$/.test(c)) c = c.replace(/\./g, '').replace(',', '.') // 1.234,5
  else if (/^-?\d+,\d+$/.test(c)) c = c.replace(',', '.') // 12,5
  else c = c.replace(/,/g, '') // 1,234.5
  const n = Number(c)
  return isFinite(n) ? n : null
}

export const isDateColumn = (vals: string[]) => vals.filter(Boolean).length > 0 && vals.filter(Boolean).every(v => /^\d{4}-\d{2}(-\d{2})?$/.test(v.trim()))

export interface Summary { n: number; sum: number; mean: number; median: number; min: number; max: number; sd: number; q1: number; q3: number; mode: number | null }

export function summary(xs: number[]): Summary | null {
  if (!xs.length) return null
  const s = [...xs].sort((a, b) => a - b)
  const n = s.length
  const sum = s.reduce((a, b) => a + b, 0)
  const mean = sum / n
  const q = (p: number) => { const i = (n - 1) * p, lo = Math.floor(i), hi = Math.ceil(i); return s[lo] + (s[hi] - s[lo]) * (i - lo) }
  const sd = n > 1 ? Math.sqrt(s.reduce((a, b) => a + (b - mean) ** 2, 0) / (n - 1)) : 0
  const counts = new Map<number, number>()
  for (const x of s) counts.set(x, (counts.get(x) || 0) + 1)
  let mode: number | null = null, best = 1
  for (const [k, c] of counts) if (c > best) { best = c; mode = k }
  return { n, sum, mean, median: q(0.5), min: s[0], max: s[n - 1], sd, q1: q(0.25), q3: q(0.75), mode }
}

export type FitKind = 'linear' | 'quadratic' | 'exponential' | 'none'
export interface Fit { kind: FitKind; coef: number[]; r2: number; f: (x: number) => number; text: string }

function solveLinear(A: number[][], b: number[]): number[] | null {
  const n = b.length
  const M = A.map((r, i) => [...r, b[i]])
  for (let c = 0; c < n; c++) {
    let p = c
    for (let r = c + 1; r < n; r++) if (Math.abs(M[r][c]) > Math.abs(M[p][c])) p = r
    if (Math.abs(M[p][c]) < 1e-12) return null
    ;[M[c], M[p]] = [M[p], M[c]]
    for (let r = 0; r < n; r++) {
      if (r === c) continue
      const k = M[r][c] / M[c][c]
      for (let j = c; j <= n; j++) M[r][j] -= k * M[c][j]
    }
  }
  return M.map((r, i) => r[n] / M[i][i])
}

/** Solves a system of linear equations (Gaussian elimination). */
export { solveLinear }

function r2(xs: number[], ys: number[], f: (x: number) => number) {
  const m = ys.reduce((a, b) => a + b, 0) / ys.length
  const tot = ys.reduce((a, y) => a + (y - m) ** 2, 0)
  const res = ys.reduce((a, y, i) => a + (y - f(xs[i])) ** 2, 0)
  return tot === 0 ? 1 : 1 - res / tot
}

const fmt = (v: number) => { const s = Number(v.toPrecision(4)); return String(s) }
const sgn = (v: number) => (v < 0 ? ' − ' : ' + ') + fmt(Math.abs(v))

export function fit(xs: number[], ys: number[], kind: FitKind): Fit | null {
  if (xs.length < 2 || kind === 'none') return null
  if (kind === 'linear') {
    const c = solveLinear([[xs.length, xs.reduce((a, b) => a + b, 0)], [xs.reduce((a, b) => a + b, 0), xs.reduce((a, b) => a + b * b, 0)]], [ys.reduce((a, b) => a + b, 0), xs.reduce((a, x, i) => a + x * ys[i], 0)])
    if (!c) return null
    const f = (x: number) => c[0] + c[1] * x
    return { kind, coef: c, r2: r2(xs, ys, f), f, text: `y = ${fmt(c[1])}x${sgn(c[0])}` }
  }
  if (kind === 'quadratic') {
    if (xs.length < 3) return null
    const S = (p: number) => xs.reduce((a, x) => a + x ** p, 0)
    const T = (p: number) => xs.reduce((a, x, i) => a + x ** p * ys[i], 0)
    const c = solveLinear([[xs.length, S(1), S(2)], [S(1), S(2), S(3)], [S(2), S(3), S(4)]], [T(0), T(1), T(2)])
    if (!c) return null
    const f = (x: number) => c[0] + c[1] * x + c[2] * x * x
    return { kind, coef: c, r2: r2(xs, ys, f), f, text: `y = ${fmt(c[2])}x²${sgn(c[1])}x${sgn(c[0])}` }
  }
  if (ys.some(y => y <= 0)) return null
  const ly = ys.map(Math.log)
  const lin = fit(xs, ly, 'linear')
  if (!lin) return null
  const a = Math.exp(lin.coef[0]), b = lin.coef[1]
  const f = (x: number) => a * Math.exp(b * x)
  return { kind, coef: [a, b], r2: r2(xs, ys, f), f, text: `y = ${fmt(a)} · e^(${fmt(b)}x)` }
}

export function correlation(xs: number[], ys: number[]): number {
  const n = xs.length
  if (n < 2) return NaN
  const mx = xs.reduce((a, b) => a + b, 0) / n, my = ys.reduce((a, b) => a + b, 0) / n
  let sxy = 0, sxx = 0, syy = 0
  for (let i = 0; i < n; i++) { sxy += (xs[i] - mx) * (ys[i] - my); sxx += (xs[i] - mx) ** 2; syy += (ys[i] - my) ** 2 }
  return sxy / Math.sqrt(sxx * syy)
}

/** Buckets for a histogram. */
export function histogram(xs: number[], bins = 0): { from: number; to: number; count: number }[] {
  if (!xs.length) return []
  const s = summary(xs)!
  const k = bins || Math.min(12, Math.max(4, Math.ceil(Math.log2(xs.length) + 1)))
  const w = (s.max - s.min) / k || 1
  const out = Array.from({ length: k }, (_, i) => ({ from: s.min + i * w, to: s.min + (i + 1) * w, count: 0 }))
  for (const x of xs) out[Math.min(k - 1, Math.floor((x - s.min) / w))].count++
  return out
}
