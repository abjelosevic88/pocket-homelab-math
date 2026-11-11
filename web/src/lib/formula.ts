// A saved calculation worked out in the browser: the same as calc.Run in Go.
import { evaluate, lineOf, newEnv, parseExpr, runLine, type Format, type Options } from './calc'
import type { FInput, FOutput, OutputResult, RunResult } from './types'

function choiceValue(i: FInput, raw: string): string {
  for (const o of (i.options || '').split('|')) {
    const [label, val] = o.includes('=') ? [o.slice(0, o.indexOf('=')), o.slice(o.indexOf('=') + 1)] : [o, o]
    if (label.trim().toLowerCase() === raw.toLowerCase() || val.trim() === raw) return val.trim()
  }
  return raw
}
const isBare = (s: string) => /^[+-]?[\d_]*\.?\d+([eE][+-]?\d+)?$/.test(s.trim().replace(/_/g, ''))

export function choiceLabels(i: FInput): string[] { return (i.options || '').split('|').map(o => (o.includes('=') ? o.slice(0, o.indexOf('=')) : o).trim()).filter(Boolean) }

export function runFormula(code: string, inputs: FInput[], values: Record<string, string>, outputs: FOutput[], o: Options & { format?: Format }): RunResult {
  const e = newEnv(o)
  const f = o.format || {}
  const res: RunResult = { outputs: [], lines: [] }
  for (const i of inputs) {
    let raw = (values[i.name] ?? '').trim()
    if (!raw) raw = i.default
    if (!raw) { res.error = 'fill in ' + (i.label || i.name); return res }
    let src = raw
    if (i.kind === 'choice') src = choiceValue(i, raw)
    else if (i.kind === 'percent' && !src.endsWith('%')) src += '%'
    else if (i.unit && isBare(raw)) src = raw + ' ' + i.unit
    try { e.vars.set(i.name, evaluate(e, parseExpr(src, e))) } catch (err) { res.error = `${i.label || i.name}: ${(err as Error).message}`; return res }
  }
  for (const l of code.split('\n')) res.lines.push(runLine(e, l, f))
  for (const out of outputs) {
    const r: OutputResult = { label: out.label, text: '', main: out.main }
    let src = out.expr
    if (out.unit.trim()) src = `(${src}) to ${out.unit}`
    try {
      let v = evaluate(e, parseExpr(src, e))
      if (out.decimals !== null && out.decimals !== undefined && v.k === 0) {
        const p = Math.pow(10, out.decimals)
        const sc = v.label && v.lscale ? v.lscale : 1
        const round = (x: number) => (x < 0 ? -Math.round(-x) : Math.round(x))
        v = v.pct ? { ...v, n: round(v.n * p) / p } : { ...v, n: (round((v.n / sc) * p) / p) * sc }
      }
      const ln = lineOf(e, v, f)
      r.text = ln.text || ''; r.number = ln.number; r.unit = ln.unit
    } catch (err) { r.error = (err as Error).message }
    res.outputs.push(r)
  }
  return res
}
