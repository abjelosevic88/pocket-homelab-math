// All notebooks, by folder, with search.
import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useOnChange } from '../lib/data'
import { ago } from '../lib/format'
import type { Notebook } from '../lib/types'
import { Icon } from '../components/icons'
import { Empty, errMsg, Loading, Segmented, useDebounce, useToast } from '../components/ui'
import { openCreate } from '../components/Layout'
import { KIND_ICON } from './Home'

export default function Notebooks() {
  const [sp, setSp] = useSearchParams()
  const { toast } = useToast()
  const subject = sp.get('subject') || ''
  const archived = sp.get('archived') === '1'
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 250)
  const [list, setList] = useState<Notebook[] | null>(null)
  const [subjects, setSubjects] = useState<{ name: string; count: number }[]>([])
  const load = useCallback(() => {
    const p = new URLSearchParams()
    if (dq) p.set('q', dq)
    if (subject) p.set('subject', subject)
    if (archived) p.set('archived', '1')
    api.get<{ notebooks: Notebook[]; subjects: { name: string; count: number }[] }>(`${V1}/notebooks?${p}`).then(r => { setList(r.notebooks); setSubjects(r.subjects) }).catch(e => toast(errMsg(e), 'err'))
  }, [dq, subject, archived, toast])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  const go = (k: string, v: string) => { const n = new URLSearchParams(sp); if (v) n.set(k, v); else n.delete(k); setSp(n) }
  return (
    <div className="stack">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">{archived ? 'Archived notebooks' : subject || 'Notebooks'}</h1></div>
        <button type="button" className="btn primary" onClick={() => openCreate('blank')}><Icon name="plus" size={15} />New notebook</button>
      </div>
      <div className="toolbar wrap">
        <div className="search-box"><Icon name="search" size={15} /><input value={q} onChange={e => setQ(e.target.value)} placeholder="Find in titles and sums" aria-label="Search notebooks" /></div>
        <div className="scroll-x"><Segmented size="sm" value={archived ? '__archived' : subject} onChange={v => { const n = new URLSearchParams(); if (v === '__archived') n.set('archived', '1'); else if (v) n.set('subject', v); setSp(n) }}
          options={[{ value: '', label: 'All' }, ...subjects.map(s => ({ value: s.name, label: `${s.name} ${s.count}` })), { value: '__archived', label: 'Archived' }]} label="Folder" /></div>
      </div>
      {!list ? <Loading /> : !list.length ? <Empty title={dq ? 'Nothing matches' : 'No notebooks here'} icon="calc">{!dq && <button type="button" className="btn" onClick={() => openCreate('blank')}>Start one</button>}</Empty> :
        <div className="nb-grid">
          {list.map(n => <Link key={n.id} to={`/n/${n.id}`} className="nb-card" style={n.color ? { ['--nb' as string]: n.color } : undefined}>
            <div className="nb-card-top">
              {n.pinned && <Icon name="pin" size={13} className="muted" />}
              <span className="nb-kinds">{n.kinds.slice(0, 4).map(k => <Icon key={k} name={KIND_ICON[k] || 'calc'} size={13} />)}</span>
              {n.shared && <Icon name="link" size={13} className="muted" />}
            </div>
            <b className="nb-card-title">{n.title}</b>
            <span className="nb-card-prev">{n.preview || 'Empty'}</span>
            <span className="nb-card-foot muted small">{n.subject ? <button type="button" className="linkish small" onClick={e => { e.preventDefault(); go('subject', n.subject) }}>{n.subject}</button> : null}{n.subject ? ' · ' : ''}{ago(n.updated_at)} · {n.cells} {n.cells === 1 ? 'cell' : 'cells'}</span>
          </Link>)}
        </div>}
    </div>
  )
}
