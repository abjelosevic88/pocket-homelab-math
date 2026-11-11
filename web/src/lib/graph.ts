// What a graph line means, and the numbers to draw it.
import { builtinSet, compileNumeric, Env, evaluate, floatOf, K, parseExpr, parseLine, type Node } from './calc'

export interface GraphExpr { id: string; text: string; color?: string; hidden?: boolean; slider?: { min: number; max: number; step: number }; range?: [number, number]; dashed?: boolean }
export interface GraphData { exprs: GraphExpr[]; view: { x: [number, number]; y: [number, number] }; grid?: boolean; axes?: boolean; height?: number; square?: boolean }

export type Kind = 'fx' | 'fy' | 'implicit' | 'region' | 'polar' | 'param' | 'point' | 'points' | 'slider' | 'const' | 'define' | 'blank' | 'error'

export interface Compiled {
  id: string; kind: Kind; color: string; error?: string; missing?: string[]
  f?: (v: number) => number          // y = f(x), x = f(y), r = f(θ)
  g?: (x: number, y: number) => number // implicit: g = 0; region: g op 0
  op?: string                          // region: lt gt le ge
  px?: (t: number) => number; py?: (t: number) => number
  pts?: [number, number][]
  value?: number; name?: string; range?: [number, number]; dashed?: boolean
}

export const PALETTE = ['#3554a5', '#c2410c', '#15803d', '#9333ea', '#0e7490', '#b91c1c', '#a16207', '#be185d']

const VARS = new Set(['x', 'y', 't', 'θ', 'theta', 'r'])

function freeVars(env: Env, n: Node | undefined, out: Set<string>) {
  if (!n) return
  if (n.op === 'id' && n.name && (VARS.has(n.name) || (!env.known(n.name) && !builtinSet.has(n.name) && !['pi', 'e', 'tau', 'phi', 'inf', 'infinity', '∞'].includes(n.name)))) out.add(n.name)
  if (n.op === 'call' && n.name && !env.fn(n.name) && !builtinSet.has(n.name) && n.kids?.length === 1) out.add(n.name) // a(x+1): a times
  for (const k of n.kids || []) freeVars(env, k, out)
}

const hasOnly = (s: Set<string>, allowed: string[]) => [...s].every(v => allowed.includes(v))

function isVarNode(n: Node, name: string): boolean { return n.op === 'id' && n.name === name || (n.op === 'paren' && isVarNode(n.kids![0], name)) }

/** compileGraph reads every line in order: sliders and functions first
 * become known to the lines after them (and to the lines before, on a second
 * pass), like a notebook. */
export function compileGraph(exprs: GraphExpr[], base: Env): { list: Compiled[]; env: Env } {
  const env = base.child()
  env.units = false
  // pass 1: constants and functions, so order does not matter
  for (let pass = 0; pass < 2; pass++) {
    for (const e of exprs) {
      const text = e.text.trim()
      if (!text) continue
      try {
        const st = parseLine(text, env)
        if (st.kind === 'assign' && st.name && !VARS.has(st.name)) {
          const fv = new Set<string>(); freeVars(env, st.expr, fv)
          if (fv.size === 0) { const v = evaluate(env, st.expr!); if (v.k === K.Num) env.vars.set(st.name, v) }
        } else if (st.kind === 'func' && st.name) {
          env.funcs.set(st.name, { params: st.params!, body: st.expr! })
        }
      } catch { /* shown in pass 2 */ }
    }
  }
  const list: Compiled[] = exprs.map((e, i) => compileOne(e, env, i))
  return { list, env }
}

function compileOne(e: GraphExpr, env: Env, i: number): Compiled {
  const color = e.color || PALETTE[i % PALETTE.length]
  const base: Compiled = { id: e.id, kind: 'blank', color, dashed: e.dashed }
  const text = e.text.trim()
  if (!text) return base
  try {
    const st = parseLine(text, env)
    if (st.kind === 'func') {
      const fn = env.fn(st.name!)!
      if (st.params!.length === 1) {
        const p = st.params![0]
        const fv = new Set<string>(); freeVars(env, fn.body, fv); fv.delete(p)
        if (fv.size) return { ...base, kind: 'error', error: missingText(fv), missing: [...fv] }
        const f = compileNumeric(fn.body, env, [p])
        if (p === 'θ' || p === 'theta') return { ...base, kind: 'polar', f, range: e.range || [0, 2 * Math.PI] }
        return { ...base, kind: 'fx', f, name: st.name }
      }
      return { ...base, kind: 'define', name: st.name }
    }
    if (st.kind === 'assign') {
      const name = st.name!
      const fv = new Set<string>(); freeVars(env, st.expr, fv)
      if (name === 'y') {
        if (fv.has('y')) return implicit(base, env, { op: '-', kids: [{ op: 'id', name: 'y', pos: 0 }, st.expr!], pos: 0 }, 'eq')
        fv.delete('x')
        if (fv.size) return { ...base, kind: 'error', error: missingText(fv), missing: [...fv] }
        return { ...base, kind: 'fx', f: compileNumeric(st.expr!, env, ['x']) }
      }
      if (name === 'x') {
        if (fv.has('x')) return implicit(base, env, { op: '-', kids: [{ op: 'id', name: 'x', pos: 0 }, st.expr!], pos: 0 }, 'eq')
        fv.delete('y')
        if (fv.size) return { ...base, kind: 'error', error: missingText(fv), missing: [...fv] }
        return { ...base, kind: 'fy', f: compileNumeric(st.expr!, env, ['y']) }
      }
      if (name === 'r') {
        fv.delete('θ'); fv.delete('theta')
        if (fv.size) return { ...base, kind: 'error', error: missingText(fv), missing: [...fv] }
        const usesTheta = new Set<string>(); freeVars(env, st.expr, usesTheta)
        const v = usesTheta.has('theta') ? 'theta' : 'θ'
        return { ...base, kind: 'polar', f: compileNumeric(st.expr!, env, [v]), range: e.range || [0, 2 * Math.PI] }
      }
      if (fv.size === 0) {
        const v = env.lookup(name)
        const value = v ? floatOf(v) : NaN
        return { ...base, kind: e.slider ? 'slider' : 'const', name, value }
      }
      return { ...base, kind: 'error', error: missingText(fv), missing: [...fv] }
    }
    if (st.kind !== 'expr' || !st.expr) return base
    const n = st.expr
    if (n.op === 'tuple' && n.kids!.length === 2) {
      const fv = new Set<string>(); freeVars(env, n, fv)
      if (fv.size === 0) {
        const p = evaluate(env, n)
        return { ...base, kind: 'point', pts: [[floatOf(p.list![0]), floatOf(p.list![1])]] }
      }
      if (hasOnly(fv, ['t'])) return { ...base, kind: 'param', px: compileNumeric(n.kids![0], env, ['t']), py: compileNumeric(n.kids![1], env, ['t']), range: e.range || [0, 2 * Math.PI] }
      fv.delete('t')
      return { ...base, kind: 'error', error: missingText(fv), missing: [...fv] }
    }
    if (n.op === 'list') {
      const v = evaluate(env, n)
      const pts: [number, number][] = []
      for (const p of v.list || []) if (p.k === K.Point && p.list?.length === 2) pts.push([floatOf(p.list[0]), floatOf(p.list[1])])
      return { ...base, kind: 'points', pts }
    }
    if (['eq', 'lt', 'gt', 'le', 'ge'].includes(n.op)) {
      const [l, r] = n.kids!
      const fv = new Set<string>(); freeVars(env, n, fv)
      const extra = [...fv].filter(v => v !== 'x' && v !== 'y')
      if (extra.length) return { ...base, kind: 'error', error: missingText(new Set(extra)), missing: extra }
      const diff: Node = { op: '-', kids: [l, r], pos: 0 }
      if (n.op === 'eq') {
        // y = f(x) written the other way round, or with y on the left
        const rf = new Set<string>(); freeVars(env, r, rf)
        if (isVarNode(l, 'y') && !rf.has('y')) return { ...base, kind: 'fx', f: compileNumeric(r, env, ['x']) }
        return implicit(base, env, diff, 'eq')
      }
      return implicit(base, env, diff, n.op)
    }
    const fv = new Set<string>(); freeVars(env, n, fv)
    if (fv.size === 0) {
      const v = evaluate(env, n)
      return { ...base, kind: 'const', value: floatOf(v) }
    }
    if (hasOnly(fv, ['x'])) return { ...base, kind: 'fx', f: compileNumeric(n, env, ['x']) }
    fv.delete('x')
    return { ...base, kind: 'error', error: missingText(fv), missing: [...fv] }
  } catch (err) {
    return { ...base, kind: 'error', error: (err as Error).message }
  }
}

function implicit(base: Compiled, env: Env, diff: Node, op: string): Compiled {
  const g = compileNumeric(diff, env, ['x', 'y'])
  return op === 'eq' ? { ...base, kind: 'implicit', g } : { ...base, kind: 'region', g, op }
}

function missingText(fv: Set<string>): string {
  const names = [...fv]
  return names.length === 1 ? `What is ${names[0]}? Add a slider or set it.` : `What are ${names.join(', ')}? Add sliders or set them.`
}

// ---------- sampling ----------

export interface View { x0: number; x1: number; y0: number; y1: number; w: number; h: number }
export const toPx = (v: View, x: number, y: number): [number, number] => [((x - v.x0) / (v.x1 - v.x0)) * v.w, v.h - ((y - v.y0) / (v.y1 - v.y0)) * v.h]
export const fromPx = (v: View, px: number, py: number): [number, number] => [v.x0 + (px / v.w) * (v.x1 - v.x0), v.y0 + ((v.h - py) / v.h) * (v.y1 - v.y0)]

/** Points of y = f(x) across the view, split where the curve jumps or stops. */
export function sampleFx(f: (x: number) => number, v: View, steps = Math.max(200, v.w * 1.5)): [number, number][][] {
  const out: [number, number][][] = []
  let cur: [number, number][] = []
  const span = v.y1 - v.y0
  let prev: number | null = null
  for (let i = 0; i <= steps; i++) {
    const x = v.x0 + ((v.x1 - v.x0) * i) / steps
    const y = f(x)
    if (!isFinite(y)) { if (cur.length) out.push(cur); cur = []; prev = null; continue }
    if (prev !== null && Math.abs(y - prev) > span * 3 && Math.sign(y) !== Math.sign(prev)) { if (cur.length) out.push(cur); cur = [] }
    cur.push([x, y])
    prev = y
  }
  if (cur.length) out.push(cur)
  return out
}

export function sampleCurve(px: (t: number) => number, py: (t: number) => number, range: [number, number], steps = 800): [number, number][][] {
  const out: [number, number][][] = []
  let cur: [number, number][] = []
  for (let i = 0; i <= steps; i++) {
    const t = range[0] + ((range[1] - range[0]) * i) / steps
    const x = px(t), y = py(t)
    if (!isFinite(x) || !isFinite(y)) { if (cur.length) out.push(cur); cur = []; continue }
    cur.push([x, y])
  }
  if (cur.length) out.push(cur)
  return out
}

/** Marching squares: the line segments where g = 0. */
export function contour(g: (x: number, y: number) => number, v: View, cell = 5): [number, number, number, number][] {
  const nx = Math.ceil(v.w / cell), ny = Math.ceil(v.h / cell)
  const vals = new Float64Array((nx + 1) * (ny + 1))
  const X = (i: number) => v.x0 + ((v.x1 - v.x0) * i) / nx, Y = (j: number) => v.y0 + ((v.y1 - v.y0) * j) / ny
  for (let j = 0; j <= ny; j++) for (let i = 0; i <= nx; i++) vals[j * (nx + 1) + i] = g(X(i), Y(j))
  const segs: [number, number, number, number][] = []
  const at = (i: number, j: number) => vals[j * (nx + 1) + i]
  const lerp = (a: number, b: number, fa: number, fb: number) => a + (b - a) * (fa / (fa - fb))
  for (let j = 0; j < ny; j++) for (let i = 0; i < nx; i++) {
    const a = at(i, j), b = at(i + 1, j), c = at(i + 1, j + 1), d = at(i, j + 1)
    if (![a, b, c, d].every(isFinite)) continue
    const x0 = X(i), x1 = X(i + 1), y0 = Y(j), y1 = Y(j + 1)
    const pts: [number, number][] = []
    if ((a > 0) !== (b > 0)) pts.push([lerp(x0, x1, a, b), y0])
    if ((b > 0) !== (c > 0)) pts.push([x1, lerp(y0, y1, b, c)])
    if ((c > 0) !== (d > 0)) pts.push([lerp(x1, x0, c, d), y1])
    if ((d > 0) !== (a > 0)) pts.push([x0, lerp(y1, y0, d, a)])
    // a jump across a pole is not a line: both sides must be small
    const m = Math.max(Math.abs(a), Math.abs(b), Math.abs(c), Math.abs(d))
    const scale = Math.max(Math.abs(v.x1 - v.x0), Math.abs(v.y1 - v.y0))
    if (m > scale * 50) continue
    if (pts.length === 2) segs.push([pts[0][0], pts[0][1], pts[1][0], pts[1][1]])
    if (pts.length === 4) { segs.push([pts[0][0], pts[0][1], pts[1][0], pts[1][1]]); segs.push([pts[2][0], pts[2][1], pts[3][0], pts[3][1]]) }
  }
  return segs
}

/** Cells of the view where an inequality holds (for shading). */
export function regionMask(g: (x: number, y: number) => number, op: string, v: View, cell = 6): { nx: number; ny: number; cell: number; on: Uint8Array } {
  const nx = Math.ceil(v.w / cell), ny = Math.ceil(v.h / cell)
  const on = new Uint8Array(nx * ny)
  for (let j = 0; j < ny; j++) for (let i = 0; i < nx; i++) {
    const [x, y] = fromPx(v, (i + 0.5) * cell, (j + 0.5) * cell)
    const d = g(x, y)
    if (!isFinite(d)) continue
    on[j * nx + i] = (op === 'lt' || op === 'le' ? d < 0 : d > 0) ? 1 : 0
  }
  return { nx, ny, cell, on }
}

/** Interesting points of y = f(x) in view: where it crosses zero, tops and
 * bottoms, and where it meets the y axis. */
export function features(f: (x: number) => number, v: View): { x: number; y: number; what: string }[] {
  const out: { x: number; y: number; what: string }[] = []
  const n = 600
  const xs = Array.from({ length: n + 1 }, (_, i) => v.x0 + ((v.x1 - v.x0) * i) / n)
  const ys = xs.map(f)
  const span = v.y1 - v.y0
  for (let i = 1; i <= n; i++) {
    const a = ys[i - 1], b = ys[i]
    if (!isFinite(a) || !isFinite(b)) continue
    if ((a > 0) !== (b > 0) && Math.abs(a - b) < span) {
      let lo = xs[i - 1], hi = xs[i], flo = a
      for (let k = 0; k < 60; k++) { const m = (lo + hi) / 2, fm = f(m); if ((fm > 0) === (flo > 0)) { lo = m; flo = fm } else hi = m }
      out.push({ x: (lo + hi) / 2, y: 0, what: 'crosses zero' })
    }
    if (i < n) {
      const c = ys[i + 1]
      if (isFinite(c) && ((b > a && b > c) || (b < a && b < c))) {
        // golden section to refine
        let lo = xs[i - 1], hi = xs[i + 1]
        const top = b > a
        for (let k = 0; k < 60; k++) {
          const m1 = lo + (hi - lo) / 3, m2 = hi - (hi - lo) / 3
          if ((f(m1) < f(m2)) === top) lo = m1; else hi = m2
        }
        const x = (lo + hi) / 2
        out.push({ x, y: f(x), what: top ? 'highest point' : 'lowest point' })
      }
    }
  }
  if (v.x0 < 0 && v.x1 > 0) { const y0 = f(0); if (isFinite(y0)) out.push({ x: 0, y: y0, what: 'meets the y axis' }) }
  return out.slice(0, 40)
}

/** Where two functions meet, in view. */
export function intersections(f: (x: number) => number, g: (x: number) => number, v: View): { x: number; y: number }[] {
  const out: { x: number; y: number }[] = []
  const h = (x: number) => f(x) - g(x)
  const n = 500
  let px = v.x0, py = h(px)
  const span = v.y1 - v.y0
  for (let i = 1; i <= n; i++) {
    const x = v.x0 + ((v.x1 - v.x0) * i) / n
    const y = h(x)
    if (isFinite(y) && isFinite(py) && (y > 0) !== (py > 0) && Math.abs(y - py) < span) {
      let lo = px, hi = x, flo = py
      for (let k = 0; k < 60; k++) { const m = (lo + hi) / 2, fm = h(m); if ((fm > 0) === (flo > 0)) { lo = m; flo = fm } else hi = m }
      const xm = (lo + hi) / 2
      out.push({ x: xm, y: f(xm) })
    }
    px = x; py = y
  }
  return out.slice(0, 20)
}

/** Nice grid steps (1, 2, 5 × 10^n) for about `target` lines. */
export function niceStep(span: number, target: number): number {
  const raw = span / Math.max(1, target)
  const p = Math.pow(10, Math.floor(Math.log10(raw)))
  const m = raw / p
  return (m < 1.5 ? 1 : m < 3.5 ? 2 : m < 7.5 ? 5 : 10) * p
}

export function tickLabel(v: number, step: number): string {
  if (Math.abs(v) < step / 1e6) return '0'
  const dec = Math.max(0, -Math.floor(Math.log10(step) + 1e-9))
  if (Math.abs(v) >= 1e6 || (Math.abs(v) < 1e-4)) return v.toExponential(1)
  return v.toFixed(dec).replace('-', '−')
}

export function short(v: number): string {
  if (!isFinite(v)) return '—'
  const r = Math.abs(v) < 1e-10 ? 0 : v
  return String(Number(r.toPrecision(5))).replace('-', '−')
}

export { parseExpr }
