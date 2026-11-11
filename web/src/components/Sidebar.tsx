// Left column: a new notebook, the sections, pinned and recent notebooks,
// and today's practice.
import { useCallback, useEffect, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useOnChange } from '../lib/data'
import type { Home } from '../lib/types'
import { Icon } from './icons'

export function Sidebar({ onCreate, onNavigate }: { onCreate: () => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [home, setHome] = useState<Home | null>(null)
  const load = useCallback(() => { api.get<Home>(`${V1}/home`).then(setHome).catch(() => {}) }, [])
  useEffect(() => { load() }, [load])
  useOnChange(load)

  const title = settings?.title || status?.title || 'Pocket Math'
  const path = loc.pathname
  const me = home?.practice.find(p => p.learner === '')
  const left = home?.practice.filter(p => p.goal && p.today < p.goal).length || 0
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number | string; hint?: string }[] = [
    { to: '/', label: 'Start', icon: 'home', active: path === '/' },
    { to: '/notebooks', label: 'Notebooks', icon: 'calc', active: path.startsWith('/notebooks') || path.startsWith('/n/') },
    { to: '/formulas', label: 'Ready calculations', icon: 'fx', active: path.startsWith('/formulas') || path.startsWith('/f/'), hint: 'Paint, loans, fuel, BMI… and your own' },
    { to: '/answers', label: 'Kept answers', icon: 'check-circle', active: path.startsWith('/answers') },
    { to: '/convert', label: 'Converter', icon: 'convert', active: path.startsWith('/convert'), hint: 'Units and money' },
    { to: '/practice', label: 'Practice', icon: 'abacus', active: path.startsWith('/practice'), count: left || undefined, hint: left ? `${left} still to practise today` : 'Sums for children (and you)' },
  ]
  const pinned = (home?.notebooks || []).filter(n => n.pinned)
  const recent = (home?.notebooks || []).filter(n => !n.pinned).slice(0, 6)
  const row = (n: Home['notebooks'][number]) => (
    <button key={n.id} type="button" className={`sb-row sb-list ${path === `/n/${n.id}` ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/n/${n.id}`) }} title={n.title}>
      <Icon name={n.pinned ? 'pin' : 'calc'} size={15} className="sb-list-ico" />
      <span className="grow ellipsis">{n.title}</span>
    </button>
  )
  return (
    <aside className="sidebar" aria-label="Pocket Math">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Math' ? title : <>Pocket <b>Math</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <button type="button" className="btn primary create-btn" onClick={onCreate} title="New notebook (N)">
        <Icon name="plus" size={15} strokeWidth={2.2} />New notebook
      </button>

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} onClick={onNavigate} title={n.hint} className={() => `sb-nav-item ${n.active ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className="sb-count" title={n.hint}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      {pinned.length > 0 && <section className="sb-sec"><h4 className="sb-h">Pinned</h4>{pinned.map(row)}</section>}
      <section className="sb-sec">
        <h4 className="sb-h"><Link to="/notebooks" onClick={onNavigate} className="sb-h-link">Recent</Link></h4>
        {recent.map(row)}
        {home && !home.notebooks.length && <button type="button" className="sb-row sb-list" onClick={() => { onNavigate(); onCreate() }}><Icon name="plus" size={15} className="sb-list-ico" /><span className="grow">Start your first notebook</span></button>}
      </section>
      {me && me.goal > 0 && <section className="sb-sec">
        <h4 className="sb-h">Practice today</h4>
        {home!.practice.map(p => <button key={p.learner} type="button" className="sb-row sb-list" onClick={() => { onNavigate(); navigate(`/practice?learner=${encodeURIComponent(p.learner)}`) }}>
          <Icon name={p.today >= p.goal ? 'check-circle' : 'circle'} size={15} className="sb-list-ico" />
          <span className="grow ellipsis">{p.learner || 'You'}</span>
          <span className="muted small nowrap">{p.today}/{p.goal}</span>
        </button>)}
      </section>}

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('pmth:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}
