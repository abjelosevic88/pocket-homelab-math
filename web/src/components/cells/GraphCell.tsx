// A graph: lines of maths on the left, drawn on the right; sliders for
// numbers you want to play with.
import { useMemo, useRef, useState } from 'react'
import { compileGraph, PALETTE, short, type GraphData, type GraphExpr } from '../../lib/graph'
import { PrettyLine } from '../../lib/pretty'
import { Icon } from '../icons'
import { Plot } from '../Plot'
import { useIsMobile } from '../ui'
import type { CellProps } from './common'

const newId = () => Math.random().toString(36).slice(2, 8)

export function normGraph(d: Partial<GraphData> | undefined): GraphData {
  return { exprs: d?.exprs?.length ? d.exprs : [{ id: newId(), text: 'y = x^2', color: PALETTE[0] }], view: d?.view || { x: [-10, 10], y: [-7, 7] }, grid: d?.grid ?? true, height: d?.height, square: d?.square }
}

export function GraphCell({ cell, onChange, env, readOnly, settings }: CellProps & { canvasRef?: React.MutableRefObject<HTMLCanvasElement | null> }) {
  const data = useMemo(() => normGraph(cell.data), [cell.data])
  const mobile = useIsMobile()
  const [focus, setFocus] = useState<string | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const { list } = useMemo(() => {
    const e = env.child()
    e.deg = !(settings?.graph_rad ?? true)
    return compileGraph(data.exprs, e)
  }, [data.exprs, env, settings?.graph_rad])
  const set = (d: Partial<GraphData>) => {
    const next = { ...data, ...d }
    onChange({ data: next, summary: next.exprs.map(x => x.text).filter(Boolean).join('\n') })
  }
  const setExpr = (id: string, p: Partial<GraphExpr>) => set({ exprs: data.exprs.map(x => (x.id === id ? { ...x, ...p } : x)) })
  const add = (text = '', after?: string) => {
    const used = new Set(data.exprs.map(x => x.color))
    const color = PALETTE.find(c => !used.has(c)) || PALETTE[data.exprs.length % PALETTE.length]
    const e: GraphExpr = { id: newId(), text, color }
    const i = after ? data.exprs.findIndex(x => x.id === after) + 1 : data.exprs.length
    const exprs = [...data.exprs]; exprs.splice(i, 0, e)
    set({ exprs })
    setTimeout(() => document.querySelector<HTMLInputElement>(`[data-gx="${e.id}"]`)?.focus(), 30)
  }
  const remove = (id: string) => set({ exprs: data.exprs.filter(x => x.id !== id) })
  const addSlider = (name: string) => {
    const e: GraphExpr = { id: newId(), text: `${name} = 1`, slider: { min: -10, max: 10, step: 0.1 } }
    set({ exprs: [...data.exprs, e] })
  }
  const visible = list.filter(c => !data.exprs.find(x => x.id === c.id)?.hidden)
  const height = data.height || (mobile ? 300 : 380)

  return (
    <div className={`graph-cell ${mobile ? 'stack' : ''}`}>
      <div className="gx-list">
        {data.exprs.map((x, i) => {
          const c = list[i]
          const plottable = c && !['slider', 'const', 'define', 'blank', 'error'].includes(c.kind)
          const sliderVal = c?.kind === 'slider' ? c.value ?? 0 : null
          return (
            <div key={x.id} className={`gx-row ${c?.kind === 'error' ? 'bad' : ''} ${focus === x.id ? 'focus' : ''}`}>
              <button type="button" className={`gx-dot ${x.hidden ? 'off' : ''} ${plottable ? '' : 'none'}`} style={{ background: plottable && !x.hidden ? c.color : undefined, borderColor: c?.color }}
                disabled={readOnly || !plottable} title={x.hidden ? 'Show' : 'Hide'} aria-label={x.hidden ? 'Show this line' : 'Hide this line'} onClick={() => setExpr(x.id, { hidden: !x.hidden })} />
              <div className="gx-main">
                {readOnly ? <div className="gx-pretty"><PrettyLine text={x.text} /></div> :
                  <input data-gx={x.id} className="gx-in" value={x.text} placeholder="y = 2x + 1" spellCheck={false} autoCapitalize="off" autoCorrect="off"
                    onFocus={() => setFocus(x.id)} onBlur={() => setFocus(null)}
                    onChange={e => setExpr(x.id, { text: e.target.value })}
                    onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); add('', x.id) } if (e.key === 'Backspace' && !x.text && data.exprs.length > 1) { e.preventDefault(); remove(x.id) } }} />}
                {c?.kind === 'error' && <div className="gx-err">{c.error}{c.missing && !readOnly && <span> {c.missing.map(m => <button key={m} type="button" className="chip-btn" onClick={() => addSlider(m)}>+ slider {m}</button>)}</span>}</div>}
                {sliderVal !== null && x.slider && <div className="gx-slider">
                  <span className="muted small">{short(x.slider.min)}</span>
                  <input type="range" min={x.slider.min} max={x.slider.max} step={x.slider.step} value={sliderVal} disabled={readOnly}
                    onChange={e => setExpr(x.id, { text: `${c!.name} = ${e.target.value}` })} aria-label={`${c!.name}`} />
                  <span className="muted small">{short(x.slider.max)}</span>
                </div>}
                {c?.kind === 'const' && c.value !== undefined && isFinite(c.value) && !x.slider && <div className="gx-val">= {short(c.value)}{!readOnly && c.name && <button type="button" className="chip-btn" onClick={() => setExpr(x.id, { slider: { min: Math.min(-10, c.value! * 2), max: Math.max(10, c.value! * 2), step: 0.1 } })}>slider</button>}</div>}
              </div>
              {!readOnly && <button type="button" className="btn ghost icon-only sm gx-x" onClick={() => remove(x.id)} aria-label="Remove this line" title="Remove"><Icon name="x" size={14} /></button>}
            </div>
          )
        })}
        {!readOnly && <div className="gx-add">
          <button type="button" className="btn sm ghost" onClick={() => add()}><Icon name="plus" size={14} />Add a line</button>
          <span className="muted small">y = …, x² + y² = 25, y &lt; x, (2, 3), r = 1 + cos(θ), (cos(t), sin(t))</span>
          <label className="check-row small"><input type="checkbox" checked={!!data.square} onChange={e => set({ square: e.target.checked })} />Same scale up and across (circles look round)</label>
        </div>}
      </div>
      <div className="gx-plot">
        <Plot curves={visible} range={data.view} onRange={readOnly ? undefined : v => set({ view: v })} height={height} grid={data.grid !== false && (settings?.graph_grid ?? true)} square={!!data.square}
          highlight={focus || undefined} readOnly={false} canvasRef={canvasRef} />
        <div className="gx-foot muted small">Drag to move · pinch, or ctrl + scroll, to zoom · tap a dot for its numbers</div>
      </div>
    </div>
  )
}
