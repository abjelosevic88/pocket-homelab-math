// Geometry you can drag: free points, and everything built from them
// (lines, circles, midpoints, crossings…) worked out again on every move.

export type GType = 'point' | 'segment' | 'line' | 'ray' | 'circle' | 'polygon' | 'midpoint' | 'intersection' | 'perpendicular' | 'parallel' | 'bisector' | 'angle' | 'circumcircle' | 'incircle'
export interface GObj { id: string; type: GType; x?: number; y?: number; of?: string[]; which?: number; color?: string; hidden?: boolean; label?: string }
export interface GeoData { objects: GObj[]; view: { x: [number, number]; y: [number, number] }; grid?: boolean; snap?: boolean; height?: number }

export type P = [number, number]
export interface Solved {
  obj: GObj
  kind: 'point' | 'line' | 'circle' | 'polygon' | 'angle'
  p?: P                     // point
  a?: P; b?: P; ext?: 'segment' | 'line' | 'ray' // line through a and b
  c?: P; r?: number         // circle
  pts?: P[]                 // polygon
  v?: P; deg?: number       // angle at v from a to b
  ok: boolean
}

const sub = (a: P, b: P): P => [a[0] - b[0], a[1] - b[1]]
const add = (a: P, b: P): P => [a[0] + b[0], a[1] + b[1]]
const mul = (a: P, k: number): P => [a[0] * k, a[1] * k]
const dot = (a: P, b: P) => a[0] * b[0] + a[1] * b[1]
const cross = (a: P, b: P) => a[0] * b[1] - a[1] * b[0]
export const dist = (a: P, b: P) => Math.hypot(a[0] - b[0], a[1] - b[1])

function lineLine(a1: P, a2: P, b1: P, b2: P): P | null {
  const d1 = sub(a2, a1), d2 = sub(b2, b1)
  const den = cross(d1, d2)
  if (Math.abs(den) < 1e-12) return null
  const t = cross(sub(b1, a1), d2) / den
  return add(a1, mul(d1, t))
}

function lineCircle(a: P, b: P, c: P, r: number): P[] {
  const d = sub(b, a), f = sub(a, c)
  const A = dot(d, d), B = 2 * dot(f, d), C = dot(f, f) - r * r
  const disc = B * B - 4 * A * C
  if (disc < 0 || A === 0) return []
  const s = Math.sqrt(disc)
  return [add(a, mul(d, (-B - s) / (2 * A))), add(a, mul(d, (-B + s) / (2 * A)))]
}

function circleCircle(c1: P, r1: number, c2: P, r2: number): P[] {
  const d = dist(c1, c2)
  if (d > r1 + r2 || d < Math.abs(r1 - r2) || d === 0) return []
  const a = (r1 * r1 - r2 * r2 + d * d) / (2 * d)
  const h = Math.sqrt(Math.max(0, r1 * r1 - a * a))
  const m = add(c1, mul(sub(c2, c1), a / d))
  const off = mul([-(c2[1] - c1[1]) / d, (c2[0] - c1[0]) / d], h)
  return [add(m, off), sub(m, off)]
}

function circumcircle(a: P, b: P, c: P): { c: P; r: number } | null {
  const d = 2 * (a[0] * (b[1] - c[1]) + b[0] * (c[1] - a[1]) + c[0] * (a[1] - b[1]))
  if (Math.abs(d) < 1e-12) return null
  const sq = (p: P) => p[0] * p[0] + p[1] * p[1]
  const ux = (sq(a) * (b[1] - c[1]) + sq(b) * (c[1] - a[1]) + sq(c) * (a[1] - b[1])) / d
  const uy = (sq(a) * (c[0] - b[0]) + sq(b) * (a[0] - c[0]) + sq(c) * (b[0] - a[0])) / d
  return { c: [ux, uy], r: dist([ux, uy], a) }
}

function incircle(a: P, b: P, c: P): { c: P; r: number } | null {
  const la = dist(b, c), lb = dist(a, c), lc = dist(a, b)
  const p = la + lb + lc
  if (p === 0) return null
  const ctr: P = [(la * a[0] + lb * b[0] + lc * c[0]) / p, (la * a[1] + lb * b[1] + lc * c[1]) / p]
  const s = p / 2
  const area = Math.sqrt(Math.max(0, s * (s - la) * (s - lb) * (s - lc)))
  return { c: ctr, r: area / s }
}

export function polygonArea(pts: P[]): number {
  let s = 0
  for (let i = 0; i < pts.length; i++) s += cross(pts[i], pts[(i + 1) % pts.length])
  return Math.abs(s) / 2
}

export function angleDeg(a: P, v: P, b: P): number {
  const u = sub(a, v), w = sub(b, v)
  let t = Math.atan2(cross(u, w), dot(u, w)) * 180 / Math.PI
  if (t < 0) t += 360
  return t > 180 ? 360 - t : t
}

/** Works out every object from the free points. */
export function solve(objs: GObj[]): Map<string, Solved> {
  const out = new Map<string, Solved>()
  const pt = (id?: string): P | undefined => { const s = id ? out.get(id) : undefined; return s?.ok && s.kind === 'point' ? s.p : undefined }
  const ln = (id?: string) => { const s = id ? out.get(id) : undefined; return s?.ok && s.kind === 'line' ? s : undefined }
  const cr = (id?: string) => { const s = id ? out.get(id) : undefined; return s?.ok && s.kind === 'circle' ? s : undefined }
  for (const o of objs) {
    const of = o.of || []
    let s: Solved = { obj: o, kind: 'point', ok: false }
    switch (o.type) {
      case 'point': s = { obj: o, kind: 'point', p: [o.x ?? 0, o.y ?? 0], ok: true }; break
      case 'midpoint': { const a = pt(of[0]), b = pt(of[1]); if (a && b) s = { obj: o, kind: 'point', p: mul(add(a, b), 0.5), ok: true }; break }
      case 'segment': case 'line': case 'ray': { const a = pt(of[0]), b = pt(of[1]); s = { obj: o, kind: 'line', a, b, ext: o.type, ok: !!a && !!b && dist(a, b) > 0 }; break }
      case 'perpendicular': case 'parallel': {
        const l = ln(of[0]), p = pt(of[1])
        if (l && p) { const d = sub(l.b!, l.a!); const dir: P = o.type === 'parallel' ? d : [-d[1], d[0]]; s = { obj: o, kind: 'line', a: p, b: add(p, dir), ext: 'line', ok: true } }
        else s = { obj: o, kind: 'line', ok: false }
        break
      }
      case 'bisector': {
        const a = pt(of[0]), b = pt(of[1])
        if (a && b) { const m = mul(add(a, b), 0.5), d = sub(b, a); s = { obj: o, kind: 'line', a: m, b: add(m, [-d[1], d[0]]), ext: 'line', ok: true } }
        else s = { obj: o, kind: 'line', ok: false }
        break
      }
      case 'circle': { const c = pt(of[0]), p = pt(of[1]); s = { obj: o, kind: 'circle', c, r: c && p ? dist(c, p) : 0, ok: !!c && !!p }; break }
      case 'circumcircle': case 'incircle': {
        const a = pt(of[0]), b = pt(of[1]), c = pt(of[2])
        const cc = a && b && c ? (o.type === 'circumcircle' ? circumcircle(a, b, c) : incircle(a, b, c)) : null
        s = { obj: o, kind: 'circle', c: cc?.c, r: cc?.r, ok: !!cc }
        break
      }
      case 'polygon': { const pts = of.map(pt); s = { obj: o, kind: 'polygon', pts: pts.filter(Boolean) as P[], ok: pts.every(Boolean) && pts.length >= 3 }; break }
      case 'angle': { const a = pt(of[0]), v = pt(of[1]), b = pt(of[2]); s = { obj: o, kind: 'angle', a, v, b, deg: a && v && b ? angleDeg(a, v, b) : 0, ok: !!a && !!v && !!b }; break }
      case 'intersection': {
        const [x, y] = [out.get(of[0]), out.get(of[1])]
        let pts: P[] = []
        if (x?.ok && y?.ok) {
          if (x.kind === 'line' && y.kind === 'line') { const p = lineLine(x.a!, x.b!, y.a!, y.b!); if (p) pts = [p] }
          else if (x.kind === 'line' && y.kind === 'circle') pts = lineCircle(x.a!, x.b!, y.c!, y.r!)
          else if (x.kind === 'circle' && y.kind === 'line') pts = lineCircle(y.a!, y.b!, x.c!, x.r!)
          else if (x.kind === 'circle' && y.kind === 'circle') pts = circleCircle(x.c!, x.r!, y.c!, y.r!)
        }
        const p = pts[Math.min(o.which || 0, pts.length - 1)]
        s = { obj: o, kind: 'point', p, ok: !!p }
        break
      }
    }
    out.set(o.id, s)
    void cr
  }
  return out
}

/** The next free name: A, B, … Z, A1… for points; a, b… for the rest. */
export function nextName(objs: GObj[], point: boolean): string {
  const used = new Set(objs.map(o => o.id))
  const letters = point ? 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' : 'abcdefghijklmnopqrstuvwxyz'
  for (let n = 0; n < 50; n++) for (const l of letters) { const id = n ? l + n : l; if (!used.has(id)) return id }
  return 'o' + objs.length
}

/** The measures worth showing for each object. */
export function measures(objs: GObj[], solved: Map<string, Solved>): { id: string; text: string }[] {
  const out: { id: string; text: string }[] = []
  const f = (v: number) => String(Number(v.toPrecision(4)))
  const names = (o: GObj) => (o.of || []).join('')
  for (const o of objs) {
    const s = solved.get(o.id)
    if (!s?.ok || o.hidden) continue
    switch (s.kind) {
      case 'point': out.push({ id: o.id, text: `${o.id} = (${f(s.p![0])}, ${f(s.p![1])})` }); break
      case 'line': if (o.type === 'segment') out.push({ id: o.id, text: `${names(o)} = ${f(dist(s.a!, s.b!))}` }); break
      case 'circle': out.push({ id: o.id, text: `${o.type === 'circle' ? 'circle' : o.type} r = ${f(s.r!)} · area ${f(Math.PI * s.r! ** 2)} · round ${f(2 * Math.PI * s.r!)}` }); break
      case 'polygon': {
        const per = s.pts!.reduce((a, p, i) => a + dist(p, s.pts![(i + 1) % s.pts!.length]), 0)
        const sides = s.pts!.map((p, i) => `${o.of![i]}${o.of![(i + 1) % o.of!.length]} = ${f(dist(p, s.pts![(i + 1) % s.pts!.length]))}`)
        out.push({ id: o.id, text: `${names(o)}: area ${f(polygonArea(s.pts!))} · perimeter ${f(per)} · ${sides.join(', ')}` })
        break
      }
      case 'angle': out.push({ id: o.id, text: `∠${o.of![1]} = ${f(s.deg!)}°` }); break
    }
  }
  return out
}

export const toolNeeds: Record<string, { picks: ('point' | 'line' | 'curve')[]; type: GType; about: string }> = {
  segment: { picks: ['point', 'point'], type: 'segment', about: 'Pick two points' },
  line: { picks: ['point', 'point'], type: 'line', about: 'Pick two points' },
  ray: { picks: ['point', 'point'], type: 'ray', about: 'Pick where it starts, then a point it goes through' },
  circle: { picks: ['point', 'point'], type: 'circle', about: 'Pick the centre, then a point on the circle' },
  midpoint: { picks: ['point', 'point'], type: 'midpoint', about: 'Pick two points' },
  bisector: { picks: ['point', 'point'], type: 'bisector', about: 'Pick two points: the line halfway, at a right angle' },
  perpendicular: { picks: ['line', 'point'], type: 'perpendicular', about: 'Pick a line, then the point it goes through' },
  parallel: { picks: ['line', 'point'], type: 'parallel', about: 'Pick a line, then the point it goes through' },
  angle: { picks: ['point', 'point', 'point'], type: 'angle', about: 'Pick three points: the corner is the middle one' },
  circumcircle: { picks: ['point', 'point', 'point'], type: 'circumcircle', about: 'Pick three points: the circle through all of them' },
  incircle: { picks: ['point', 'point', 'point'], type: 'incircle', about: 'Pick three points: the circle inside the triangle' },
  intersection: { picks: ['curve', 'curve'], type: 'intersection', about: 'Pick two lines or circles' },
}
