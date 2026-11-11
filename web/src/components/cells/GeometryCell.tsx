// Geometry: place points, build lines, circles and angles from them, and
// drag the points — everything built on them follows.
import { useMemo, useRef, useState } from 'react'
import { dist, measures, nextName, solve, toolNeeds, type GeoData, type GObj, type P, type Solved } from '../../lib/geometry'
import { niceStep, tickLabel } from '../../lib/graph'
import { Icon } from '../icons'
import { useWidth } from '../Plot'
import { useIsMobile } from '../ui'
import type { CellProps } from './common'

const TOOLS: { id: string; label: string; icon: string }[] = [
  { id: 'move', label: 'Move', icon: 'move' }, { id: 'point', label: 'Point', icon: 'point' }, { id: 'segment', label: 'Segment', icon: 'segment' },
  { id: 'line', label: 'Line', icon: 'line' }, { id: 'ray', label: 'Ray', icon: 'ray' }, { id: 'polygon', label: 'Shape', icon: 'polygon' },
  { id: 'circle', label: 'Circle', icon: 'circle' }, { id: 'midpoint', label: 'Middle', icon: 'midpoint' }, { id: 'perpendicular', label: 'Right angle', icon: 'perp' },
  { id: 'parallel', label: 'Parallel', icon: 'parallel' }, { id: 'bisector', label: 'Halfway line', icon: 'perp' }, { id: 'intersection', label: 'Crossing', icon: 'crosshair' },
  { id: 'angle', label: 'Angle', icon: 'angle' }, { id: 'circumcircle', label: 'Circle through 3', icon: 'circle' }, { id: 'incircle', label: 'Circle inside', icon: 'circle' },
  { id: 'delete', label: 'Remove', icon: 'trash' },
]

export function normGeo(d: Partial<GeoData> | undefined): GeoData {
  return { objects: d?.objects || [], view: d?.view || { x: [-8, 8], y: [-6, 6] }, grid: d?.grid ?? true, snap: d?.snap ?? true, height: d?.height }
}

export function GeometryCell({ cell, onChange, readOnly }: CellProps) {
  const data = useMemo(() => normGeo(cell.data), [cell.data])
  const mobile = useIsMobile()
  const [ref, W] = useWidth(600)
  const H = data.height || (mobile ? 320 : 400)
  const [tool, setTool] = useState('move')
  const [picks, setPicks] = useState<string[]>([])
  const [live, setLive] = useState<GObj[] | null>(null) // while dragging
  const objs = live || data.objects
  const solved = useMemo(() => solve(objs), [objs])
  const svg = useRef<SVGSVGElement | null>(null)

  // square units, centred on the saved view
  const xs = data.view.x[1] - data.view.x[0]
  const ppu = W / xs
  const cy = (data.view.y[0] + data.view.y[1]) / 2
  const y0 = cy - H / ppu / 2, y1 = cy + H / ppu / 2
  const X = (x: number) => (x - data.view.x[0]) * ppu, Y = (y: number) => H - (y - y0) * ppu
  const toW = (px: number, py: number): P => [data.view.x[0] + px / ppu, y0 + (H - py) / ppu]

  const save = (objects: GObj[]) => onChange({ data: { ...data, objects }, summary: summaryOf(objects) })
  const snap = (p: P): P => data.snap ? [Math.round(p[0] * 2) / 2, Math.round(p[1] * 2) / 2] : p

  const local = (e: React.PointerEvent): [number, number] => { const r = svg.current!.getBoundingClientRect(); return [e.clientX - r.left, e.clientY - r.top] }
  const nearPoint = (px: number, py: number) => {
    let best: { id: string; d: number } | null = null
    for (const [id, s] of solved) if (s.ok && s.kind === 'point' && !s.obj.hidden) { const d = Math.hypot(X(s.p![0]) - px, Y(s.p![1]) - py); if (d < 16 && (!best || d < best.d)) best = { id, d } }
    return best?.id
  }
  const nearCurve = (px: number, py: number, want: 'line' | 'curve') => {
    const p = toW(px, py)
    let best: { id: string; d: number } | null = null
    for (const [id, s] of solved) {
      if (!s.ok || s.obj.hidden) continue
      let d = Infinity
      if (s.kind === 'line') { const a = s.a!, b = s.b!; const dx = b[0] - a[0], dy = b[1] - a[1]; d = Math.abs(dy * p[0] - dx * p[1] + b[0] * a[1] - b[1] * a[0]) / Math.hypot(dx, dy) * ppu }
      else if (s.kind === 'circle' && want === 'curve') d = Math.abs(dist(p, s.c!) - s.r!) * ppu
      if (d < 14 && (!best || d < best.d)) best = { id, d }
    }
    return best?.id
  }

  const drag = useRef<{ id: string; moved: boolean } | null>(null)
  const pan = useRef<{ x: number; y: number; view: GeoData['view'] } | null>(null)
  const onDown = (e: React.PointerEvent) => {
    if (readOnly) return
    const [px, py] = local(e)
    const hit = nearPoint(px, py)
    if (tool === 'move') {
      const o = hit ? objs.find(x => x.id === hit) : undefined
      if (o && o.type === 'point') { drag.current = { id: o.id, moved: false }; svg.current!.setPointerCapture(e.pointerId); return }
      pan.current = { x: e.clientX, y: e.clientY, view: data.view }; svg.current!.setPointerCapture(e.pointerId)
      return
    }
    if (tool === 'delete') {
      const id = hit || nearCurve(px, py, 'curve')
      if (id) save(removeWith(objs, id))
      return
    }
    if (tool === 'point') {
      const p = snap(toW(px, py))
      const curve = nearCurve(px, py, 'curve')
      void curve
      save([...objs, { id: nextName(objs, true), type: 'point', x: p[0], y: p[1] }])
      return
    }
    if (tool === 'polygon') {
      let id = hit
      const extra: GObj[] = []
      if (!id) { const p = snap(toW(px, py)); id = nextName(objs, true); extra.push({ id, type: 'point', x: p[0], y: p[1] }) }
      if (picks.length >= 3 && id === picks[0]) { save([...objs, { id: nextName(objs, false), type: 'polygon', of: picks }]); setPicks([]); return }
      if (extra.length) save([...objs, ...extra])
      setPicks([...picks, id])
      return
    }
    const need = toolNeeds[tool]
    if (!need) return
    const want = need.picks[picks.length]
    let id: string | undefined
    const extra: GObj[] = []
    if (want === 'point') {
      id = hit
      if (!id) { const p = snap(toW(px, py)); id = nextName([...objs], true); extra.push({ id, type: 'point', x: p[0], y: p[1] }) }
    } else id = nearCurve(px, py, want === 'line' ? 'line' : 'curve')
    if (!id) return
    const next = [...picks, id]
    let all = [...objs, ...extra]
    if (next.length === need.picks.length) {
      const o: GObj = { id: nextName(all, need.type === 'midpoint' || need.type === 'intersection'), type: need.type, of: next }
      all = [...all, o]
      if (need.type === 'intersection') {
        // a line and a circle (or two circles) cross twice: add both
        const s = solve([...all, { ...o, id: o.id + '_2', which: 1 }]).get(o.id + '_2')
        const s1 = solve(all).get(o.id)
        if (s?.ok && s1?.ok && dist(s.p!, s1.p!) > 1e-6) all = [...all, { id: nextName(all, true), type: 'intersection', of: next, which: 1 }]
      }
      setPicks([])
    } else setPicks(next)
    save(all)
  }
  const onMove = (e: React.PointerEvent) => {
    if (drag.current) {
      const [px, py] = local(e)
      const p = snap(toW(px, py))
      drag.current.moved = true
      setLive(objs.map(o => (o.id === drag.current!.id ? { ...o, x: p[0], y: p[1] } : o)))
    } else if (pan.current) {
      const dx = (e.clientX - pan.current.x) / ppu, dy = (e.clientY - pan.current.y) / ppu
      const v = pan.current.view
      onChange({ data: { ...data, view: { x: [v.x[0] - dx, v.x[1] - dx], y: [v.y[0] + dy, v.y[1] + dy] } } })
    }
  }
  const onUp = () => {
    if (drag.current && live) save(live)
    drag.current = null; pan.current = null; setLive(null)
  }
  const zoom = (k: number) => {
    const cx = (data.view.x[0] + data.view.x[1]) / 2, hw = (xs / 2) * k
    const hh = ((data.view.y[1] - data.view.y[0]) / 2) * k
    onChange({ data: { ...data, view: { x: [cx - hw, cx + hw], y: [cy - hh, cy + hh] } } })
  }

  const step = niceStep(xs, Math.max(4, W / 70))
  const gridLines: React.ReactNode[] = []
  if (data.grid) {
    for (let x = Math.ceil(data.view.x[0] / step) * step; x <= data.view.x[1]; x += step) gridLines.push(<line key={'x' + x} className="geo-grid" x1={X(x)} x2={X(x)} y1={0} y2={H} />)
    for (let y = Math.ceil(y0 / step) * step; y <= y1; y += step) gridLines.push(<line key={'y' + y} className="geo-grid" x1={0} x2={W} y1={Y(y)} y2={Y(y)} />)
  }
  const ext = (s: Solved): [P, P] => {
    const a = s.a!, b = s.b!
    if (s.ext === 'segment') return [a, b]
    const d: P = [b[0] - a[0], b[1] - a[1]], k = (xs + (y1 - y0)) * 4 / Math.hypot(d[0], d[1])
    const far: P = [a[0] + d[0] * k, a[1] + d[1] * k]
    return s.ext === 'ray' ? [a, far] : [[a[0] - d[0] * k, a[1] - d[1] * k], far]
  }
  const shapes: React.ReactNode[] = [], points: React.ReactNode[] = [], labels: React.ReactNode[] = []
  for (const [id, s] of solved) {
    if (!s.ok || s.obj.hidden) continue
    const col = s.obj.color || (s.kind === 'point' ? (s.obj.type === 'point' ? 'var(--accent)' : '#64748b') : 'var(--text-2)')
    const picked = picks.includes(id)
    if (s.kind === 'line') { const [a, b] = ext(s); shapes.push(<line key={id} className={`geo-line ${picked ? 'picked' : ''}`} x1={X(a[0])} y1={Y(a[1])} x2={X(b[0])} y2={Y(b[1])} stroke={col} />) }
    if (s.kind === 'circle') shapes.push(<circle key={id} className={`geo-line ${picked ? 'picked' : ''}`} cx={X(s.c![0])} cy={Y(s.c![1])} r={s.r! * ppu} stroke={col} fill="none" />)
    if (s.kind === 'polygon') shapes.unshift(<polygon key={id} className="geo-poly" points={s.pts!.map(p => `${X(p[0])},${Y(p[1])}`).join(' ')} />)
    if (s.kind === 'angle') {
      const v = s.v!, a = s.a!, b = s.b!
      const r = 26
      const ua = Math.atan2(-(a[1] - v[1]), a[0] - v[0]), ub = Math.atan2(-(b[1] - v[1]), b[0] - v[0])
      let d = ub - ua
      while (d > Math.PI) d -= 2 * Math.PI
      while (d < -Math.PI) d += 2 * Math.PI
      const sx = X(v[0]) + r * Math.cos(ua), sy = Y(v[1]) + r * Math.sin(ua), ex = X(v[0]) + r * Math.cos(ua + d), ey = Y(v[1]) + r * Math.sin(ua + d)
      shapes.push(<path key={id} className="geo-angle" d={`M${X(v[0])},${Y(v[1])} L${sx},${sy} A${r},${r} 0 0 ${d > 0 ? 1 : 0} ${ex},${ey} Z`} />)
      const mid = ua + d / 2
      labels.push(<text key={id + 'l'} className="geo-label" x={X(v[0]) + (r + 14) * Math.cos(mid)} y={Y(v[1]) + (r + 14) * Math.sin(mid) + 4} textAnchor="middle">{s.deg!.toFixed(1)}°</text>)
    }
    if (s.kind === 'point') {
      points.push(<circle key={id} className={`geo-pt ${s.obj.type === 'point' ? 'free' : ''} ${picked ? 'picked' : ''}`} cx={X(s.p![0])} cy={Y(s.p![1])} r={s.obj.type === 'point' ? 6 : 4.5} fill={col} />)
      labels.push(<text key={id + 'l'} className="geo-name" x={X(s.p![0]) + 9} y={Y(s.p![1]) - 9}>{id.replace(/_\d+$/, '')}</text>)
    }
  }
  const ms = measures(objs, solved)
  const need = toolNeeds[tool]
  const hint = tool === 'move' ? 'Drag a point; drag the paper to move around.' : tool === 'point' ? 'Tap to place a point.' : tool === 'delete' ? 'Tap what to remove (what is built on it goes too).' : tool === 'polygon' ? (picks.length >= 3 ? 'Tap the first point again to close the shape.' : 'Tap the corners one by one.') : need?.about
  return (
    <div className="geo-cell">
      {!readOnly && <div className="geo-tools" role="toolbar" aria-label="Tools">
        {TOOLS.map(t => <button key={t.id} type="button" className={`geo-tool ${tool === t.id ? 'on' : ''}`} onClick={() => { setTool(t.id); setPicks([]) }} title={t.label} aria-pressed={tool === t.id}><Icon name={t.icon} size={16} /><span>{t.label}</span></button>)}
      </div>}
      {!readOnly && <div className="geo-hint muted small">{hint}{picks.length > 0 && <> · picked {picks.join(', ')} <button type="button" className="linkish" onClick={() => setPicks([])}>start again</button></>}</div>}
      <div className="geo-canvas" ref={ref} style={{ height: H }}>
        <svg ref={svg} width={W} height={H} viewBox={`0 0 ${W} ${H}`} className={`geo-svg tool-${tool}`} onPointerDown={onDown} onPointerMove={onMove} onPointerUp={onUp} onPointerCancel={onUp}>
          {gridLines}
          {data.view.x[0] < 0 && data.view.x[1] > 0 && <line className="geo-axis" x1={X(0)} x2={X(0)} y1={0} y2={H} />}
          {y0 < 0 && y1 > 0 && <line className="geo-axis" x1={0} x2={W} y1={Y(0)} y2={Y(0)} />}
          {data.grid && y0 < 0 && y1 > 0 && Array.from({ length: Math.floor(xs / step) + 1 }, (_, i) => Math.ceil(data.view.x[0] / step) * step + i * step).filter(x => Math.abs(x) > step / 1e6 && X(x) > 12 && X(x) < W - 12).map(x => <text key={'t' + x} className="geo-tick" x={X(x)} y={Math.min(H - 4, Y(0) + 14)} textAnchor="middle">{tickLabel(x, step)}</text>)}
          {shapes}{points}{labels}
        </svg>
        {!readOnly && <div className="plot-tools">
          <button type="button" className="btn sm icon-only" onClick={() => zoom(0.75)} aria-label="Zoom in" title="Zoom in"><Icon name="zoom-in" size={15} /></button>
          <button type="button" className="btn sm icon-only" onClick={() => zoom(1 / 0.75)} aria-label="Zoom out" title="Zoom out"><Icon name="zoom-out" size={15} /></button>
          <button type="button" className={`btn sm icon-only ${data.snap ? 'on' : ''}`} onClick={() => onChange({ data: { ...data, snap: !data.snap } })} aria-label="Snap to the grid" title={data.snap ? 'Snapping to the grid (click to place freely)' : 'Placing freely (click to snap to the grid)'}><Icon name="magnet" size={15} /></button>
        </div>}
      </div>
      {ms.length > 0 && <ul className="geo-measures">{ms.map(m => <li key={m.id}>{m.text}</li>)}</ul>}
    </div>
  )
}

/** Removing an object also removes what is built on it. */
function removeWith(objs: GObj[], id: string): GObj[] {
  const gone = new Set([id])
  let changed = true
  while (changed) {
    changed = false
    for (const o of objs) if (!gone.has(o.id) && (o.of || []).some(x => gone.has(x))) { gone.add(o.id); changed = true }
  }
  return objs.filter(o => !gone.has(o.id))
}

function summaryOf(objs: GObj[]): string {
  const n = (t: string) => objs.filter(o => o.type === t).length
  const parts = [[n('point'), 'point'], [n('segment') + n('line') + n('ray'), 'line'], [n('circle') + n('circumcircle') + n('incircle'), 'circle'], [n('polygon'), 'shape'], [n('angle'), 'angle']]
    .filter(([k]) => (k as number) > 0).map(([k, w]) => `${k} ${w}${k === 1 ? '' : 's'}`)
  return 'Geometry: ' + (parts.join(', ') || 'empty')
}
