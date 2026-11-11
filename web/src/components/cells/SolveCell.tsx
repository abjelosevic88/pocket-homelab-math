// Equations: one solved for its unknown, a few straight-line ones solved
// together, a table of values, the area under a curve.
import { useMemo } from 'react'
import { builtinSet, compileNumeric, Env, evaluate, integrate, K, parseExpr, show, unknowns, type Node } from '../../lib/calc'
import { short, type Compiled } from '../../lib/graph'
import { PrettyLine } from '../../lib/pretty'
import { solveLinear } from '../../lib/stats'
import { Icon } from '../icons'
import { Plot } from '../Plot'
import { Segmented, useIsMobile } from '../ui'
import type { CellProps } from './common'

export interface SolveData { mode: 'equation' | 'system' | 'table' | 'area'; equations: string[]; var?: string; expr?: string; from?: string; to?: string; step?: string }

export function normSolve(d: Partial<SolveData> | undefined): SolveData {
  return { mode: d?.mode || 'equation', equations: d?.equations?.length ? d.equations : ['2x + 3 = 11'], var: d?.var || '', expr: d?.expr ?? 'x^2', from: d?.from ?? '0', to: d?.to ?? '3', step: d?.step ?? '0.5' }
}

function plainEnv(env: Env) { const e = env.child(); e.units = false; return e }

function freeOf(env: Env, n: Node) {
  const s = new Set<string>()
  unknowns(env, n, s)
  return [...s].filter(v => !builtinSet.has(v)).sort()
}

export function SolveCell({ cell, onChange, env, readOnly, format, deg }: CellProps) {
  const data = useMemo(() => normSolve(cell.data), [cell.data])
  const mobile = useIsMobile()
  const set = (d: Partial<SolveData>) => {
    const next = { ...data, ...d }
    const sum = next.mode === 'equation' || next.mode === 'system' ? next.equations.join('\n') : next.mode === 'table' ? `Table of ${next.expr} from ${next.from} to ${next.to}` : `Area under ${next.expr} from ${next.from} to ${next.to}`
    onChange({ data: next, summary: sum })
  }
  const e = useMemo(() => { const x = plainEnv(env); x.deg = deg; return x }, [env, deg])
  const result = useMemo(() => work(data, e, format), [data, e, format])
  const setEq = (i: number, v: string) => set({ equations: data.equations.map((x, j) => (j === i ? v : x)) })

  return (
    <div className="solve-cell">
      <div className="solve-bar">
        <Segmented size="sm" value={data.mode} onChange={v => !readOnly && set({ mode: v, equations: v === 'system' && data.equations.length < 2 ? ['2x + y = 7', 'x - y = 2'] : data.equations })} label="What to do"
          options={[{ value: 'equation', label: 'Solve' }, { value: 'system', label: 'Solve together' }, { value: 'table', label: 'Table of values' }, { value: 'area', label: 'Area under' }]} />
      </div>
      <div className={`solve-body ${mobile ? 'stack' : ''}`}>
        <div className="solve-in">
          {(data.mode === 'equation' || data.mode === 'system') && <>
            {data.equations.slice(0, data.mode === 'equation' ? 1 : 6).map((q, i) => (
              <div key={i} className="solve-eq">
                {readOnly ? <PrettyLine text={q} /> : <input value={q} onChange={ev => setEq(i, ev.target.value)} spellCheck={false} autoCapitalize="off" placeholder={i ? 'x - y = 2' : '2x + 3 = 11'} aria-label={`Equation ${i + 1}`} />}
                {data.mode === 'system' && !readOnly && data.equations.length > 2 && <button type="button" className="btn ghost icon-only sm" onClick={() => set({ equations: data.equations.filter((_, j) => j !== i) })} aria-label="Remove"><Icon name="x" size={14} /></button>}
              </div>
            ))}
            {data.mode === 'system' && !readOnly && data.equations.length < 6 && <button type="button" className="btn sm ghost" onClick={() => set({ equations: [...data.equations, ''] })}><Icon name="plus" size={14} />Another equation</button>}
          </>}
          {(data.mode === 'table' || data.mode === 'area') && <div className="solve-grid">
            <label className="field"><span className="field-label">f(x) =</span><input value={data.expr} onChange={ev => set({ expr: ev.target.value })} readOnly={readOnly} spellCheck={false} /></label>
            <label className="field"><span className="field-label">From</span><input value={data.from} onChange={ev => set({ from: ev.target.value })} readOnly={readOnly} inputMode="decimal" /></label>
            <label className="field"><span className="field-label">To</span><input value={data.to} onChange={ev => set({ to: ev.target.value })} readOnly={readOnly} inputMode="decimal" /></label>
            {data.mode === 'table' && <label className="field"><span className="field-label">Step</span><input value={data.step} onChange={ev => set({ step: ev.target.value })} readOnly={readOnly} inputMode="decimal" /></label>}
          </div>}
          <div className={`solve-out ${result.error ? 'bad' : ''}`}>
            {result.error ? <span>{result.error}</span> : result.lines.map((l, i) => <div key={i} className="solve-ans">{l}</div>)}
            {result.note && <div className="muted small">{result.note}</div>}
          </div>
          {result.table && <div className="scroll-x"><table className="dtable small"><thead><tr><th>x</th><th>f(x)</th></tr></thead><tbody>{result.table.map(([x, y], i) => <tr key={i}><td className="num">{short(x)}</td><td className="num">{isFinite(y) ? short(y) : '—'}</td></tr>)}</tbody></table></div>}
        </div>
        {result.curves.length > 0 && <div className="solve-plot"><Plot curves={result.curves} range={result.range} height={mobile ? 240 : 280} square={data.mode === 'system'} /></div>}
      </div>
    </div>
  )
}

interface Result { lines: string[]; note?: string; error?: string; curves: Compiled[]; range: { x: [number, number]; y: [number, number] }; table?: [number, number][] }

function rangeAround(xs: number[], f?: (x: number) => number): { x: [number, number]; y: [number, number] } {
  const lo = Math.min(-5, ...xs.map(x => x - 3)), hi = Math.max(5, ...xs.map(x => x + 3))
  let ys: number[] = []
  if (f) for (let i = 0; i <= 60; i++) { const y = f(lo + ((hi - lo) * i) / 60); if (isFinite(y)) ys.push(y) }
  ys = ys.sort((a, b) => a - b)
  const ylo = ys.length ? ys[Math.floor(ys.length * 0.05)] : -5, yhi = ys.length ? ys[Math.floor(ys.length * 0.95)] : 5
  const pad = (yhi - ylo) * 0.15 || 2
  return { x: [lo, hi], y: [Math.min(ylo - pad, -1), Math.max(yhi + pad, 1)] }
}

function work(d: SolveData, env: Env, format: CellProps['format']): Result {
  const empty: Result = { lines: [], curves: [], range: { x: [-10, 10], y: [-7, 7] } }
  try {
    if (d.mode === 'equation') {
      const src = (d.equations[0] || '').trim()
      if (!src) return { ...empty, note: 'Write an equation, like 2x + 3 = 11 or x² − 5x + 6 = 0.' }
      const n = parseExpr(src, env)
      const eq: Node = n.op === 'eq' ? n : { op: 'eq', kids: [n, { op: 'num', num: 0, pos: 0 }], pos: 0 }
      const vars = freeOf(env, eq)
      if (!vars.length) { const v = evaluate(env, eq); return { ...empty, lines: [v.n ? 'This is true.' : 'This is not true.'] } }
      if (vars.length > 1) return { ...empty, error: `There are ${vars.length} unknowns (${vars.join(', ')}): use "Solve together" with as many equations.` }
      const name = vars[0]
      const v = evaluate(env, { op: 'call', name: 'solve', kids: [eq, { op: 'id', name, pos: 0 }], num: 0, pos: 0 })
      const roots = v.k === K.Solved ? v.list!.map(x => x.n) : []
      const f = compileNumeric({ op: '-', kids: [eq.kids![0], eq.kids![1]], pos: 0 }, env, [name])
      const r = rangeAround(roots, f)
      const lines = roots.map(x => `${name} = ${show(env, { k: K.Num, n: x, u: [] }, format)[0]}`)
      return { lines, note: roots.length > 1 ? `${roots.length} answers` : undefined, curves: [{ id: 'f', kind: 'fx', color: '#3554a5', f }, { id: 'r', kind: 'points', color: '#c2410c', pts: roots.map(x => [x, 0]) }], range: r }
    }
    if (d.mode === 'system') {
      const eqs = d.equations.map(s => s.trim()).filter(Boolean).map(s => parseExpr(s, env))
      if (eqs.length < 2) return { ...empty, note: 'Write two or more equations.' }
      const diffs = eqs.map(n => (n.op === 'eq' ? { op: '-', kids: [n.kids![0], n.kids![1]], pos: 0 } : n) as Node)
      const vars = [...new Set(diffs.flatMap(n => freeOf(env, n)))].sort()
      if (vars.length !== diffs.length) return { ...empty, error: `${diffs.length} equations need ${diffs.length} unknowns; these have ${vars.length} (${vars.join(', ') || 'none'}).` }
      const fs = diffs.map(n => compileNumeric(n, env, vars))
      const zero = vars.map(() => 0)
      const A = fs.map(f => vars.map((_, j) => { const p = [...zero]; p[j] = 1; return f(...p) - f(...zero) }))
      const b = fs.map(f => -f(...zero))
      // straight-line check at another point
      const probe = vars.map((_, j) => 1.7 + j)
      const linear = fs.every((f, i) => Math.abs(f(...probe) - (A[i].reduce((s, a, j) => s + a * probe[j], 0) - b[i])) < 1e-6 * (1 + Math.abs(f(...probe))))
      if (!linear) return { ...empty, error: 'Only straight-line equations (no x², no xy) can be solved together here. Solve one at a time.' }
      const sol = solveLinear(A, b)
      if (!sol) return { ...empty, error: 'There is no single answer: the equations are parallel (never meet) or the same.' }
      const lines = vars.map((v, i) => `${v} = ${show(env, { k: K.Num, n: Math.abs(sol[i]) < 1e-12 ? 0 : sol[i], u: [] }, format)[0]}`)
      let curves: Compiled[] = []
      let range = empty.range
      if (vars.length === 2) {
        curves = diffs.map((n, i) => ({ id: 'e' + i, kind: 'implicit' as const, color: ['#3554a5', '#c2410c', '#15803d', '#9333ea'][i % 4], g: compileNumeric(n, env, vars) }))
        curves.push({ id: 'p', kind: 'point', color: '#e5484d', pts: [[sol[0], sol[1]]] })
        range = { x: [sol[0] - 6, sol[0] + 6], y: [sol[1] - 5, sol[1] + 5] }
      }
      return { lines, curves, range, note: vars.length === 2 ? 'Where the lines cross.' : undefined }
    }
    const expr = parseExpr(d.expr || '', env)
    const fv = freeOf(env, expr)
    const name = fv[0] || 'x'
    if (fv.length > 1) return { ...empty, error: `Use one unknown (${fv.join(', ')})` }
    const f = compileNumeric(expr, env, [name])
    const num = (s?: string) => { const v = evaluate(env, parseExpr(s || '0', env)); return v.k === K.Num ? v.n : NaN }
    const a = num(d.from), b = num(d.to)
    if (!isFinite(a) || !isFinite(b)) return { ...empty, error: 'From and to are numbers.' }
    const span = Math.abs(b - a) || 1
    const range = { x: [Math.min(a, b) - span * 0.25, Math.max(a, b) + span * 0.25] as [number, number], y: rangeAround([], f).y }
    if (d.mode === 'table') {
      const st = Math.abs(num(d.step)) || 1
      const rows: [number, number][] = []
      for (let x = Math.min(a, b), k = 0; x <= Math.max(a, b) + st / 1e6 && k < 500; x += st, k++) rows.push([Number(x.toPrecision(12)), f(x)])
      return { lines: [], table: rows, curves: [{ id: 'f', kind: 'fx', color: '#3554a5', f }, { id: 'p', kind: 'points', color: '#c2410c', pts: rows.filter(r => isFinite(r[1])) }], range }
    }
    const area = integrate(f, a, b)
    if (!isFinite(area)) return { ...empty, error: "This area can't be worked out (the curve has a gap)." }
    return {
      lines: [`Area = ${show(env, { k: K.Num, n: Number(area.toPrecision(10)), u: [] }, format)[0]}`], note: 'Below the x axis counts as minus.',
      curves: [{ id: 'f', kind: 'fx', color: '#3554a5', f }, { id: 'reg', kind: 'region', color: '#3554a5', op: 'lt', g: (x, y) => (x < Math.min(a, b) || x > Math.max(a, b) ? 1 : (f(x) >= 0 ? (y < 0 || y > f(x) ? 1 : -1) : (y > 0 || y < f(x) ? 1 : -1))) }], range,
    }
  } catch (err) {
    return { ...empty, error: (err as Error).message }
  }
}
