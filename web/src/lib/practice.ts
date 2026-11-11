// Practice questions: made up fresh each round, by skill and level.
import { calculate } from './calc'

export interface Q { q: string; answer: string; value: number; hint?: string }

export const SKILLS: { id: string; name: string; about: string; icon: string }[] = [
  { id: 'add', name: 'Adding', about: '7 + 5, 38 + 47, 456 + 789', icon: 'plus' },
  { id: 'subtract', name: 'Taking away', about: '12 − 5, 83 − 37', icon: 'minus' },
  { id: 'tables', name: 'Times tables', about: 'One table at a time, or all of them', icon: 'grid' },
  { id: 'times', name: 'Times', about: '7 × 8, 23 × 4, 36 × 17', icon: 'x' },
  { id: 'divide', name: 'Dividing', about: '56 ÷ 7, 144 ÷ 12', icon: 'divide' },
  { id: 'fractions', name: 'Fractions', about: '½ + ¼, ¾ of 20, simplest form', icon: 'pi' },
  { id: 'decimals', name: 'Decimals', about: '0.4 + 0.75, 1.2 × 3', icon: 'dot' },
  { id: 'percent', name: 'Percentages', about: '20% of 80, 15% off 60', icon: 'percent' },
  { id: 'rounding', name: 'Rounding', about: 'to the nearest 10, 100, one decimal', icon: 'target' },
  { id: 'powers', name: 'Squares and roots', about: '7², √81, 2⁵', icon: 'sigma' },
  { id: 'equations', name: 'Equations', about: 'Find x: 3x + 4 = 19', icon: 'equals' },
  { id: 'mixed', name: 'A mix', about: 'A bit of everything at this level', icon: 'shuffle' },
]

export const LEVELS = [
  { n: 1, name: 'Starting', about: 'numbers up to 10' },
  { n: 2, name: 'Getting there', about: 'numbers up to 20 · tables to 5' },
  { n: 3, name: 'Confident', about: 'numbers up to 100 · all tables' },
  { n: 4, name: 'Strong', about: 'bigger numbers, two steps' },
  { n: 5, name: 'Expert', about: 'big numbers, harder steps' },
]

type Rnd = () => number
const r = (rnd: Rnd, a: number, b: number) => a + Math.floor(rnd() * (b - a + 1))
const pick = <T,>(rnd: Rnd, xs: T[]) => xs[Math.floor(rnd() * xs.length)]
const gcd = (a: number, b: number): number => (b ? gcd(b, a % b) : Math.abs(a))
const frac = (p: number, q: number) => { const g = gcd(p, q); p /= g; q /= g; return q === 1 ? String(p) : `${p}/${q}` }
const fmt = (v: number) => String(Number(v.toFixed(6)))

export function seeded(seed: number): Rnd {
  let s = seed >>> 0 || 1
  return () => { s = (s * 1664525 + 1013904223) >>> 0; return s / 4294967296 }
}

export function question(skill: string, level: number, rnd: Rnd, table = 0): Q {
  const L = Math.max(1, Math.min(5, level))
  switch (skill) {
    case 'add': {
      const m = [10, 20, 100, 1000, 10000][L - 1]
      const a = r(rnd, 1, m), b = r(rnd, 1, m)
      return { q: `${a} + ${b}`, answer: String(a + b), value: a + b }
    }
    case 'subtract': {
      const m = [10, 20, 100, 1000, 10000][L - 1]
      let a = r(rnd, 1, m), b = r(rnd, 1, m)
      if (b > a) [a, b] = [b, a]
      return { q: `${a} − ${b}`, answer: String(a - b), value: a - b }
    }
    case 'tables': {
      const t = table || r(rnd, 2, [5, 5, 10, 12, 12][L - 1])
      const k = r(rnd, 1, [10, 10, 10, 12, 12][L - 1])
      if (L >= 4 && rnd() < 0.3) return { q: `${t * k} ÷ ${t}`, answer: String(k), value: k }
      return rnd() < 0.5 ? { q: `${t} × ${k}`, answer: String(t * k), value: t * k } : { q: `${k} × ${t}`, answer: String(t * k), value: t * k }
    }
    case 'times': {
      const [ma, mb] = [[5, 5], [10, 10], [12, 12], [99, 9], [99, 99]][L - 1]
      const a = r(rnd, 2, ma), b = r(rnd, 2, mb)
      return { q: `${a} × ${b}`, answer: String(a * b), value: a * b }
    }
    case 'divide': {
      const [mq, md] = [[5, 5], [10, 10], [12, 12], [50, 12], [99, 25]][L - 1]
      const d = r(rnd, 2, md), q = r(rnd, 1, mq)
      return { q: `${d * q} ÷ ${d}`, answer: String(q), value: q }
    }
    case 'fractions': {
      const kind = pick(rnd, L <= 2 ? ['of', 'simplify'] : ['of', 'simplify', 'add', 'sub', 'times'])
      const den = () => pick(rnd, L <= 2 ? [2, 4, 5, 10] : [2, 3, 4, 5, 6, 8, 10, 12])
      if (kind === 'of') {
        const q = den(), p = r(rnd, 1, q - 1), k = r(rnd, 1, [5, 10, 10, 20, 50][L - 1])
        return { q: `${frac(p, q)} of ${q * k}`, answer: String(p * k), value: p * k }
      }
      if (kind === 'simplify') {
        const q = den(), p = r(rnd, 1, q - 1), k = r(rnd, 2, 6)
        return { q: `Simplest form of ${p * k}/${q * k}`, answer: frac(p, q), value: p / q, hint: 'like 3/4' }
      }
      const q1 = den(), q2 = den(), p1 = r(rnd, 1, q1 - 1), p2 = r(rnd, 1, q2 - 1)
      if (kind === 'add') return { q: `${p1}/${q1} + ${p2}/${q2}`, answer: frac(p1 * q2 + p2 * q1, q1 * q2), value: p1 / q1 + p2 / q2, hint: 'a fraction, like 5/6' }
      if (kind === 'sub') { const [a, b] = p1 / q1 >= p2 / q2 ? [[p1, q1], [p2, q2]] : [[p2, q2], [p1, q1]]; return { q: `${a[0]}/${a[1]} − ${b[0]}/${b[1]}`, answer: frac(a[0] * b[1] - b[0] * a[1], a[1] * b[1]), value: a[0] / a[1] - b[0] / b[1], hint: 'a fraction' } }
      return { q: `${p1}/${q1} × ${p2}/${q2}`, answer: frac(p1 * p2, q1 * q2), value: (p1 * p2) / (q1 * q2), hint: 'a fraction' }
    }
    case 'decimals': {
      const d = L <= 2 ? 1 : 2
      const a = r(rnd, 1, [9, 30, 99, 999, 999][L - 1]) / 10 ** d, b = r(rnd, 1, [9, 30, 99, 999, 999][L - 1]) / 10 ** d
      const op = pick(rnd, L <= 2 ? ['+', '−'] : ['+', '−', '×'])
      if (op === '×') { const k = r(rnd, 2, 9); return { q: `${a} × ${k}`, answer: fmt(a * k), value: a * k } }
      const [x, y] = op === '−' && b > a ? [b, a] : [a, b]
      const v = op === '+' ? x + y : x - y
      return { q: `${x} ${op} ${y}`, answer: fmt(v), value: Number(fmt(v)) }
    }
    case 'percent': {
      const p = pick(rnd, L <= 2 ? [10, 25, 50, 100] : [5, 10, 15, 20, 25, 30, 40, 50, 75])
      const base = r(rnd, 1, [10, 20, 40, 100, 200][L - 1]) * (L <= 2 ? 4 : 20)
      const kind = pick(rnd, L <= 2 ? ['of'] : ['of', 'off', 'on'])
      if (kind === 'of') return { q: `${p}% of ${base}`, answer: fmt((p * base) / 100), value: (p * base) / 100 }
      if (kind === 'off') return { q: `${base} with ${p}% off`, answer: fmt(base * (1 - p / 100)), value: base * (1 - p / 100) }
      return { q: `${base} plus ${p}%`, answer: fmt(base * (1 + p / 100)), value: base * (1 + p / 100) }
    }
    case 'rounding': {
      const kind = pick(rnd, L <= 2 ? [10] : L === 3 ? [10, 100] : [10, 100, 0.1, 1000])
      const n = kind === 0.1 ? r(rnd, 100, 9999) / 100 : r(rnd, 11, [99, 999, 9999, 99999, 999999][L - 1])
      const v = kind === 0.1 ? Math.round(n * 10) / 10 : Math.round(n / kind) * kind
      return { q: `${n} to the nearest ${kind === 0.1 ? 'tenth' : kind}`, answer: fmt(v), value: v }
    }
    case 'powers': {
      const kind = pick(rnd, L <= 2 ? ['sq'] : L === 3 ? ['sq', 'root'] : ['sq', 'root', 'pow', 'cube'])
      if (kind === 'sq') { const a = r(rnd, 2, [5, 10, 12, 15, 25][L - 1]); return { q: `${a}²`, answer: String(a * a), value: a * a } }
      if (kind === 'root') { const a = r(rnd, 2, [5, 10, 12, 15, 25][L - 1]); return { q: `√${a * a}`, answer: String(a), value: a } }
      if (kind === 'cube') { const a = r(rnd, 2, 6); return { q: `${a}³`, answer: String(a ** 3), value: a ** 3 } }
      const a = pick(rnd, [2, 3, 5, 10]), k = r(rnd, 2, a === 2 ? 10 : 4)
      return { q: `${a}${'⁰¹²³⁴⁵⁶⁷⁸⁹'[k] ?? '^' + k}`, answer: String(a ** k), value: a ** k }
    }
    case 'equations': {
      const x = r(rnd, L <= 2 ? 1 : -10, [5, 10, 12, 20, 30][L - 1])
      const a = r(rnd, L <= 2 ? 1 : 2, [1, 5, 9, 12, 15][L - 1]), b = r(rnd, 1, [10, 20, 30, 50, 99][L - 1])
      const sign = L >= 3 && rnd() < 0.5 ? -1 : 1
      const c = a * x + sign * b
      const lhs = `${a === 1 ? '' : a}x ${sign > 0 ? '+' : '−'} ${b}`
      return { q: `Find x: ${lhs} = ${c}`, answer: String(x), value: x }
    }
  }
  // mixed
  const s = pick(rnd, ['add', 'subtract', 'tables', 'divide', ...(L >= 3 ? ['fractions', 'percent', 'equations', 'decimals'] : [])])
  return question(s, level, rnd)
}

/** Is what was typed right? Fractions and decimals are both fine. */
export function isRight(q: Q, given: string): boolean {
  const g = given.trim().replace(',', '.').replace(/[−–]/g, '-').replace('÷', '/')
  if (!g) return false
  if (g === q.answer) return true
  const ln = calculate(g, { units: false, deg: true })
  if (ln.kind !== 'value' || ln.number === undefined) return false
  return Math.abs(ln.number - q.value) < 1e-6 * Math.max(1, Math.abs(q.value))
}

export function skillName(id: string) { return SKILLS.find(s => s.id === id)?.name || id }
