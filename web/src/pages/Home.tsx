// The start: a quick sum, new notebooks from a start, what was worked on
// last, the calculations used most and today's practice.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useData, useOnChange } from '../lib/data'
import { calculate } from '../lib/calc'
import { ago } from '../lib/format'
import type { Home as HomeT, NotebookFull } from '../lib/types'
import { Ring } from '../components/Chart'
import { Icon } from '../components/icons'
import { numberFormat } from '../components/cells/common'
import { copyText, errMsg, Loading, useToast } from '../components/ui'
import { openCreate } from '../components/Layout'
import { AIWrite } from '../components/AIHelp'

const QUICK_KEY = 'pmth-quick'
const EXAMPLES = ['5 km + 300 m', '100 EUR in BAM', '20% off 80', '2026-12-25 - today', '60 mph to km/h', 'x^2 - 5x + 6 = 0', 'sqrt(2) to fraction', '3 kW * 2 h * 0.19 BAM/kWh']

export const KIND_ICON: Record<string, string> = { calc: 'calc', text: 'text', graph: 'graph', geometry: 'triangle', data: 'table', solve: 'equals' }

export default function Home() {
  const { settings, user } = useApp()
  const { rates } = useData()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [home, setHome] = useState<HomeT | null>(null)
  const [q, setQ] = useState(() => new URLSearchParams(window.location.search).get('q') || '')
  const [hist, setHist] = useState<string[]>(() => { try { return JSON.parse(localStorage.getItem(QUICK_KEY) || '[]') } catch { return [] } })
  const [ai, setAi] = useState(false)
  const load = useCallback(() => { api.get<HomeT>(`${V1}/home`).then(setHome).catch(e => toast(errMsg(e), 'err')) }, [toast])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  const format = useMemo(() => numberFormat(settings), [settings])
  const opts = { units: settings?.units ?? true, deg: settings?.angle !== 'rad', rates, format }
  const res = q.trim() ? calculate(q, opts) : null
  const keep = (expr: string) => {
    const next = [expr, ...hist.filter(h => h !== expr)].slice(0, 8)
    setHist(next); localStorage.setItem(QUICK_KEY, JSON.stringify(next))
  }
  const toNotebook = async (body: string, title = 'Quick sums') => {
    try {
      const r = await api.post<NotebookFull>(`${V1}/notebooks`, { title, cells: [{ kind: 'calc', body }] })
      navigate(`/n/${r.notebook.id}`)
    } catch (e) { toast(errMsg(e), 'err') }
  }

  if (!home) return <Loading />
  const first = (user?.name || '').split(' ')[0]
  const popular = ['paint', 'floor', 'loan', 'savings', 'trip_fuel', 'bmi', 'vat', 'unit_price']
  return (
    <div className="stack loose home">
      <section className="quick card">
        <div className="quick-head"><h2>{first ? `Hello, ${first}` : 'Work something out'}</h2><span className="muted small">Units, money, percentages, dates — write it the way you think it.</span></div>
        <div className="quick-row">
          <input className="quick-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Try 5 km + 300 m" autoFocus={window.innerWidth > 767} spellCheck={false} autoCapitalize="off" autoCorrect="off"
            onKeyDown={e => { if (e.key === 'Enter' && res && res.kind !== 'error') keep(q.trim()) }} aria-label="A quick sum" />
          <div className={`quick-out ${res?.kind === 'error' ? 'bad' : ''}`} aria-live="polite">
            {res && res.kind !== 'error' && res.text ? <button type="button" className="quick-val" onClick={async () => { keep(q.trim()); toast(await copyText(res.text!) ? `Copied ${res.text}` : 'Could not copy', 'ok') }} title="Copy">{res.text}{res.alt && <span className="muted small"> {res.alt}</span>}</button>
              : res?.kind === 'error' ? <span className="small">{res.error}</span> : <span className="muted">=</span>}
          </div>
        </div>
        <div className="chips">
          {(hist.length ? hist : EXAMPLES).slice(0, 8).map(h => <button key={h} type="button" className="chip-btn" onClick={() => setQ(h)}>{h}</button>)}
          {q.trim() && <button type="button" className="chip-btn accent" onClick={() => toNotebook(q)}><Icon name="plus" size={12} />Into a notebook</button>}
        </div>
      </section>

      <section>
        <div className="sec-head"><h3>Start</h3><Link to="/reference" className="small">What can I write?</Link></div>
        <div className="tiles start-tiles">
          {home.templates.map(t => <button key={t.key} type="button" className="tile" onClick={() => openCreate(t.key)}>
            <div className="tile-head"><span className="tile-ico lg"><Icon name={t.icon === 'wallet' ? 'wallet' : t.icon === 'car' ? 'car' : t.icon === 'table' ? 'table' : t.icon} size={17} /></span><span className="tile-name">{t.name}</span></div>
            <span className="muted small">{t.about}</span>
          </button>)}
          <button type="button" className="tile ai" onClick={() => setAi(true)}>
            <div className="tile-head"><span className="tile-ico lg"><Icon name="sparkles" size={17} /></span><span className="tile-name">From words or a photo</span></div>
            <span className="muted small">Write the problem, or snap the homework: Pocket AI writes the sums.</span>
          </button>
        </div>
      </section>

      <div className="home-cols">
        <section className="card">
          <div className="card-h sec-pad"><h3>Notebooks</h3><Link to="/notebooks" className="small">All</Link></div>
          {home.notebooks.length === 0 ? <p className="muted pad">Nothing yet. Start one above.</p> :
            <ul className="list">{home.notebooks.slice(0, 7).map(n => <li key={n.id}><Link to={`/n/${n.id}`} className="list-row row-link">
              <span className="list-ico"><Icon name={KIND_ICON[n.kinds[0]] || 'calc'} size={16} /></span>
              <span className="grow ellipsis"><b>{n.title}</b><span className="muted small ellipsis block-line">{n.preview || n.subject || 'Empty'}</span></span>
              <span className="muted small nowrap">{ago(n.updated_at)}</span>
            </Link></li>)}</ul>}
        </section>

        <section className="card">
          <div className="card-h sec-pad"><h3>Ready calculations</h3><Link to="/formulas" className="small">All</Link></div>
          <ul className="list">
            {[...home.pinned.map(f => ({ to: `/f/${f.id}`, name: f.name, sub: f.about, icon: f.icon || 'fx' })),
              ...home.favourites.map(f => ({ to: `/f/${f.key}`, name: f.name, sub: `used ${f.runs}×`, icon: f.icon })),
            ].slice(0, 6).map(f => <li key={f.to}><Link to={f.to} className="list-row row-link"><span className="list-ico"><Icon name={f.icon} size={16} /></span><span className="grow ellipsis"><b>{f.name}</b><span className="muted small ellipsis block-line">{f.sub}</span></span><Icon name="chevron-right" size={14} className="muted" /></Link></li>)}
          </ul>
          <div className="chips pad">{popular.filter(k => !home.favourites.some(f => f.key === k)).slice(0, 6).map(k => <Link key={k} to={`/f/${k}`} className="chip-btn">{k.replace('_', ' ')}</Link>)}</div>
        </section>

        <section className="card">
          <div className="card-h sec-pad"><h3>Practice today</h3><Link to="/practice" className="small">Practise</Link></div>
          <div className="practice-today">
            {home.practice.map(p => {
              const pct = p.goal ? Math.min(100, (p.today / p.goal) * 100) : 0
              return <Link key={p.learner} to={`/practice?learner=${encodeURIComponent(p.learner)}`} className="pt-row">
                <Ring pct={pct} size={44} stroke={5}><span className="small"><b>{p.today}</b></span></Ring>
                <span className="grow"><b>{p.learner || 'You'}</b><span className="muted small block-line">{p.goal ? `${p.today} of ${p.goal} today` : `${p.today} today`}{p.streak > 1 ? ` · ${p.streak} days in a row` : ''}</span></span>
              </Link>
            })}
          </div>
        </section>

        {home.runs.length > 0 && <section className="card">
          <div className="card-h sec-pad"><h3>Kept answers</h3><Link to="/answers" className="small">All</Link></div>
          <ul className="list">{home.runs.map(r => <li key={r.id}><Link to={r.formula_key ? `/f/${r.formula_key}` : r.formula_id ? `/f/${r.formula_id}` : '/answers'} className="list-row row-link">
            <span className="grow ellipsis"><b>{r.title || r.formula_name}</b><span className="muted small ellipsis block-line">{r.title ? r.formula_name + ' · ' : ''}{ago(r.created_at)}</span></span>
            <span className="pill">{r.main}</span>
          </Link></li>)}</ul>
        </section>}
      </div>
      {ai && <AIWrite onClose={() => setAi(false)} onLines={async (text, title) => { setAi(false); await toNotebook(text, title || 'Worked out') }} />}
    </div>
  )
}
