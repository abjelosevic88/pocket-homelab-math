// A graph drawn on a canvas at its real size (device pixels), with pan,
// zoom, pinch, tracing and the points worth knowing (zeros, tops, crossings).
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { contour, features, fromPx, intersections, niceStep, regionMask, sampleCurve, sampleFx, short, tickLabel, toPx, type Compiled, type View } from '../lib/graph'
import { Icon } from './icons'

export function useWidth(fallback: number) {
  const [w, setW] = useState(fallback)
  const ro = useRef<ResizeObserver | null>(null)
  const ref = useCallback((el: Element | null) => {
    ro.current?.disconnect(); ro.current = null
    if (!el) return
    const measure = () => { const cw = Math.round(el.getBoundingClientRect().width); if (cw > 0) setW(cw) }
    measure(); ro.current = new ResizeObserver(measure); ro.current.observe(el)
  }, [])
  return [ref, w] as const
}

type Range = { x: [number, number]; y: [number, number] }

interface Mark { x: number; y: number; what: string; color: string }

function cssVar(name: string, fallback: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

/** Keep the units square: stretch the y range to the box's shape. */
export function fitView(r: Range, w: number, h: number, square: boolean): Range {
  if (!square || w <= 0 || h <= 0) return r
  const xs = r.x[1] - r.x[0], ys = r.y[1] - r.y[0]
  const want = (xs * h) / w
  const cy = (r.y[0] + r.y[1]) / 2
  if (Math.abs(want - ys) / ys < 0.02) return r
  return { x: r.x, y: [cy - want / 2, cy + want / 2] }
}

export function Plot({ curves, range, onRange, height = 360, grid = true, square = false, highlight, onPick, readOnly = false, canvasRef }: {
  curves: Compiled[]; range: Range; onRange?: (r: Range) => void; height?: number; grid?: boolean; square?: boolean
  highlight?: string; onPick?: (m: Mark | null) => void; readOnly?: boolean; canvasRef?: React.MutableRefObject<HTMLCanvasElement | null>
}) {
  const [wrapRef, W] = useWidth(600)
  const H = height
  const canvas = useRef<HTMLCanvasElement | null>(null)
  const [view, setView] = useState<Range>(range)
  const viewKey = JSON.stringify(range)
  useEffect(() => { setView(range) }, [viewKey]) // eslint-disable-line react-hooks/exhaustive-deps
  const shown = useMemo(() => fitView(view, W, H, square), [view, W, H, square])
  const v: View = { x0: shown.x[0], x1: shown.x[1], y0: shown.y[0], y1: shown.y[1], w: W, h: H }
  const [trace, setTrace] = useState<{ px: number; py: number; x: number; y: number; color: string } | null>(null)
  const [mark, setMark] = useState<Mark | null>(null)

  // marks: zeros, tops and bottoms, crossings of the function curves in view
  const marks = useMemo(() => {
    const out: Mark[] = []
    const fx = curves.filter(c => c.kind === 'fx' && c.f)
    for (const c of fx) for (const p of features(c.f!, v)) out.push({ ...p, color: c.color })
    for (let i = 0; i < fx.length; i++) for (let j = i + 1; j < fx.length; j++) for (const p of intersections(fx[i].f!, fx[j].f!, v)) out.push({ ...p, what: 'the two meet', color: '#64748b' })
    return out
  }, [curves, v.x0, v.x1, v.y0, v.y1, W, H]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const cv = canvas.current
    if (!cv) return
    if (canvasRef) canvasRef.current = cv
    const dpr = window.devicePixelRatio || 1
    cv.width = Math.round(W * dpr); cv.height = Math.round(H * dpr)
    cv.style.width = W + 'px'; cv.style.height = H + 'px'
    const ctx = cv.getContext('2d')!
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    const bg = cssVar('--surface', '#fff'), text = cssVar('--muted', '#6b7280'), line = cssVar('--border', '#e5e7eb'), axis = cssVar('--text-2', '#374151')
    ctx.fillStyle = bg; ctx.fillRect(0, 0, W, H)
    // grid
    const sx = niceStep(v.x1 - v.x0, Math.max(4, W / 80)), sy = niceStep(v.y1 - v.y0, Math.max(3, H / 60))
    ctx.font = '11px system-ui, -apple-system, sans-serif'
    if (grid) {
      ctx.strokeStyle = line; ctx.lineWidth = 1
      ctx.beginPath()
      for (let x = Math.ceil(v.x0 / sx) * sx; x <= v.x1; x += sx) { const [px] = toPx(v, x, 0); ctx.moveTo(Math.round(px) + 0.5, 0); ctx.lineTo(Math.round(px) + 0.5, H) }
      for (let y = Math.ceil(v.y0 / sy) * sy; y <= v.y1; y += sy) { const [, py] = toPx(v, 0, y); ctx.moveTo(0, Math.round(py) + 0.5); ctx.lineTo(W, Math.round(py) + 0.5) }
      ctx.stroke()
    }
    // axes
    const [ax, ay] = toPx(v, 0, 0)
    ctx.strokeStyle = axis; ctx.lineWidth = 1.2
    ctx.beginPath()
    if (ay >= 0 && ay <= H) { ctx.moveTo(0, Math.round(ay) + 0.5); ctx.lineTo(W, Math.round(ay) + 0.5) }
    if (ax >= 0 && ax <= W) { ctx.moveTo(Math.round(ax) + 0.5, 0); ctx.lineTo(Math.round(ax) + 0.5, H) }
    ctx.stroke()
    // tick labels: along the axes, or at the edges when an axis is out of view
    ctx.fillStyle = text
    const ly = Math.min(H - 4, Math.max(14, ay + 14))
    ctx.textAlign = 'center'
    for (let x = Math.ceil(v.x0 / sx) * sx; x <= v.x1; x += sx) {
      if (Math.abs(x) < sx / 1e6) continue
      const [px] = toPx(v, x, 0)
      if (px < 12 || px > W - 12) continue
      ctx.fillText(tickLabel(x, sx), px, ly)
    }
    const lx = Math.min(W - 6, Math.max(6, ax - 6))
    ctx.textAlign = ax - 6 < 30 ? 'left' : 'right'
    for (let y = Math.ceil(v.y0 / sy) * sy; y <= v.y1; y += sy) {
      if (Math.abs(y) < sy / 1e6) continue
      const [, py] = toPx(v, 0, y)
      if (py < 10 || py > H - 10) continue
      ctx.fillText(tickLabel(y, sy), ax - 6 < 30 ? Math.max(4, ax + 6) : lx, py + 4)
    }
    // regions
    for (const c of curves) {
      if (c.kind !== 'region' || !c.g) continue
      const m = regionMask(c.g, c.op!, v)
      ctx.fillStyle = c.color + '26'
      for (let j = 0; j < m.ny; j++) for (let i = 0; i < m.nx; i++) if (m.on[j * m.nx + i]) ctx.fillRect(i * m.cell, j * m.cell, m.cell + 0.5, m.cell + 0.5)
      ctx.strokeStyle = c.color; ctx.lineWidth = 2
      ctx.setLineDash(c.op === 'lt' || c.op === 'gt' ? [6, 5] : [])
      ctx.beginPath()
      for (const s of contour(c.g, v)) { const [a, b] = toPx(v, s[0], s[1]), [d, e] = toPx(v, s[2], s[3]); ctx.moveTo(a, b); ctx.lineTo(d, e) }
      ctx.stroke(); ctx.setLineDash([])
    }
    const width = (c: Compiled) => (highlight === c.id ? 3.4 : 2.4)
    for (const c of curves) {
      ctx.strokeStyle = c.color; ctx.lineWidth = width(c); ctx.lineJoin = 'round'; ctx.lineCap = 'round'
      ctx.setLineDash(c.dashed ? [7, 6] : [])
      if (c.kind === 'implicit' && c.g) {
        ctx.beginPath()
        for (const s of contour(c.g, v, 4)) { const [a, b] = toPx(v, s[0], s[1]), [d, e] = toPx(v, s[2], s[3]); ctx.moveTo(a, b); ctx.lineTo(d, e) }
        ctx.stroke()
      }
      let paths: [number, number][][] = []
      if (c.kind === 'fx' && c.f) paths = sampleFx(c.f, v)
      if (c.kind === 'fy' && c.f) paths = sampleFx(c.f, { x0: v.y0, x1: v.y1, y0: v.x0, y1: v.x1, w: H, h: W }).map(p => p.map(([y, x]) => [x, y] as [number, number]))
      if (c.kind === 'polar' && c.f) paths = sampleCurve(t => c.f!(t) * Math.cos(t), t => c.f!(t) * Math.sin(t), c.range || [0, 2 * Math.PI], 1200)
      if (c.kind === 'param' && c.px && c.py) paths = sampleCurve(c.px, c.py, c.range || [0, 2 * Math.PI], 1200)
      for (const p of paths) {
        ctx.beginPath()
        p.forEach(([x, y], i) => {
          const [px, py] = toPx(v, x, y)
          const cy = Math.max(-1e4, Math.min(1e4, py))
          if (i === 0) ctx.moveTo(px, cy); else ctx.lineTo(px, cy)
        })
        ctx.stroke()
      }
      ctx.setLineDash([])
      if ((c.kind === 'point' || c.kind === 'points') && c.pts) {
        ctx.fillStyle = c.color
        for (const [x, y] of c.pts) {
          const [px, py] = toPx(v, x, y)
          ctx.beginPath(); ctx.arc(px, py, 5, 0, Math.PI * 2); ctx.fill()
          if (c.kind === 'point') { ctx.fillStyle = axis; ctx.textAlign = 'left'; ctx.fillText(`(${short(x)}, ${short(y)})`, px + 8, py - 8); ctx.fillStyle = c.color }
        }
      }
    }
    // marks
    for (const m of marks) {
      const [px, py] = toPx(v, m.x, m.y)
      if (px < 0 || px > W || py < 0 || py > H) continue
      ctx.fillStyle = bg; ctx.strokeStyle = '#94a3b8'; ctx.lineWidth = 1.6
      ctx.beginPath(); ctx.arc(px, py, 3.6, 0, Math.PI * 2); ctx.fill(); ctx.stroke()
    }
  }, [curves, W, H, v.x0, v.x1, v.y0, v.y1, grid, highlight, marks, canvasRef]) // eslint-disable-line react-hooks/exhaustive-deps

  // ---- moving around ----
  const pointers = useRef(new Map<number, { x: number; y: number }>())
  const start = useRef<{ view: Range; dist?: number; mid?: [number, number] } | null>(null)
  const moved = useRef(false)
  const commit = (r: Range) => { setView(r); onRange?.(r) }
  const local = (e: React.PointerEvent | React.WheelEvent) => { const r = canvas.current!.getBoundingClientRect(); return [e.clientX - r.left, e.clientY - r.top] as [number, number] }

  const zoomAt = (px: number, py: number, k: number, from = shown) => {
    const fv: View = { x0: from.x[0], x1: from.x[1], y0: from.y[0], y1: from.y[1], w: W, h: H }
    const [cx, cy] = fromPx(fv, px, py)
    return { x: [cx - (cx - from.x[0]) * k, cx + (from.x[1] - cx) * k] as [number, number], y: [cy - (cy - from.y[0]) * k, cy + (from.y[1] - cy) * k] as [number, number] }
  }

  const traceAt = (px: number, py: number) => {
    const [x] = fromPx(v, px, py)
    let best: { px: number; py: number; x: number; y: number; color: string; d: number } | null = null
    for (const c of curves) {
      if (c.kind !== 'fx' || !c.f) continue
      const y = c.f(x)
      if (!isFinite(y)) continue
      const [, cpy] = toPx(v, x, y)
      const d = Math.abs(cpy - py)
      if (d < 40 && (!best || d < best.d)) best = { px, py: cpy, x, y, color: c.color, d }
    }
    setTrace(best)
  }

  const onDown = (e: React.PointerEvent) => {
    if (readOnly) return
    canvas.current!.setPointerCapture(e.pointerId)
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    moved.current = false
    if (pointers.current.size === 2) {
      const [a, b] = [...pointers.current.values()]
      start.current = { view: shown, dist: Math.hypot(a.x - b.x, a.y - b.y), mid: [(a.x + b.x) / 2, (a.y + b.y) / 2] }
    } else start.current = { view: shown }
  }
  const onMove = (e: React.PointerEvent) => {
    const [px, py] = local(e)
    if (!pointers.current.has(e.pointerId)) { if (e.pointerType === 'mouse') traceAt(px, py); return }
    const prev = pointers.current.get(e.pointerId)!
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    if (!start.current) return
    if (pointers.current.size === 2 && start.current.dist) {
      const [a, b] = [...pointers.current.values()]
      const d = Math.hypot(a.x - b.x, a.y - b.y)
      const rect = canvas.current!.getBoundingClientRect()
      const k = start.current.dist / Math.max(10, d)
      setView(zoomAt(start.current.mid![0] - rect.left, start.current.mid![1] - rect.top, k, start.current.view))
      moved.current = true
      return
    }
    const dx = e.clientX - prev.x, dy = e.clientY - prev.y
    if (Math.abs(dx) + Math.abs(dy) > 0) moved.current = true
    setView(cur => {
      const c = fitView(cur, W, H, square)
      const ux = ((c.x[1] - c.x[0]) / W) * dx, uy = ((c.y[1] - c.y[0]) / H) * dy
      return { x: [c.x[0] - ux, c.x[1] - ux], y: [c.y[0] + uy, c.y[1] + uy] }
    })
    setTrace(null)
  }
  const onUp = (e: React.PointerEvent) => {
    pointers.current.delete(e.pointerId)
    if (pointers.current.size === 0) {
      if (moved.current) onRange?.(view)
      else {
        // a tap: the nearest mark, or the curve under the finger
        const [px, py] = local(e)
        const near = marks.map(m => ({ m, d: Math.hypot(toPx(v, m.x, m.y)[0] - px, toPx(v, m.x, m.y)[1] - py) })).sort((a, b) => a.d - b.d)[0]
        if (near && near.d < 16) { setMark(near.m); onPick?.(near.m); setTrace(null) }
        else { setMark(null); onPick?.(null); traceAt(px, py) }
      }
      start.current = null
    }
  }
  const viewRef = useRef(view); viewRef.current = view
  useEffect(() => {
    const cv = canvas.current
    if (!cv || readOnly) return
    const h = (e: WheelEvent) => {
      if (!e.ctrlKey && !e.metaKey && !e.altKey) return
      e.preventDefault()
      const r = cv.getBoundingClientRect()
      const k = Math.exp(e.deltaY * 0.0015)
      const nv = zoomAt(e.clientX - r.left, e.clientY - r.top, k, fitView(viewRef.current, W, H, square))
      setView(nv)
      window.clearTimeout((h as unknown as { t?: number }).t)
      ;(h as unknown as { t?: number }).t = window.setTimeout(() => onRange?.(nv), 250)
    }
    cv.addEventListener('wheel', h, { passive: false })
    return () => cv.removeEventListener('wheel', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const button = (k: number) => commit(zoomAt(W / 2, H / 2, k))
  const home = () => commit(range)

  const [mx, my] = mark ? toPx(v, mark.x, mark.y) : [0, 0]
  return (
    <div className="plot" ref={wrapRef} style={{ height: H }}>
      <canvas ref={canvas} className={`plot-canvas ${readOnly ? '' : 'can-move'}`} onPointerDown={onDown} onPointerMove={onMove} onPointerUp={onUp} onPointerCancel={onUp} onPointerLeave={e => { if (e.pointerType === 'mouse' && !pointers.current.size) setTrace(null) }} aria-label="Graph" role="img" />
      {trace && <div className="plot-tip" style={{ left: Math.min(W - 120, trace.px + 10), top: Math.max(4, trace.py - 30), borderColor: trace.color }}>({short(trace.x)}, {short(trace.y)})</div>}
      {trace && <span className="plot-dot" style={{ left: trace.px - 5, top: trace.py - 5, background: trace.color }} />}
      {mark && <div className="plot-tip strong" style={{ left: Math.min(W - 170, mx + 10), top: Math.max(4, my - 34) }}>({short(mark.x)}, {short(mark.y)}) <span className="muted">{mark.what}</span></div>}
      {!readOnly && <div className="plot-tools">
        <button type="button" className="btn sm icon-only" onClick={() => button(0.7)} aria-label="Zoom in" title="Zoom in"><Icon name="zoom-in" size={15} /></button>
        <button type="button" className="btn sm icon-only" onClick={() => button(1 / 0.7)} aria-label="Zoom out" title="Zoom out"><Icon name="zoom-out" size={15} /></button>
        <button type="button" className="btn sm icon-only" onClick={home} aria-label="Back to the start" title="Back to where it started"><Icon name="home" size={15} /></button>
      </div>}
    </div>
  )
}
