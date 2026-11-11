// Ready calculations: the built-in ones by subject, and your own.
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useOnChange } from '../lib/data'
import type { Formula } from '../lib/types'
import { Icon } from '../components/icons'
import { errMsg, Loading, useDebounce, useToast } from '../components/ui'

export default function Formulas() {
  const navigate = useNavigate()
  const { toast } = useToast()
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 200)
  const [data, setData] = useState<{ mine: Formula[]; library: Formula[]; categories: string[] } | null>(null)
  const load = useCallback(() => { api.get<typeof data>(`${V1}/formulas?q=${encodeURIComponent(dq)}`).then(setData).catch(e => toast(errMsg(e), 'err')) }, [dq, toast])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  if (!data) return <Loading />
  const card = (f: Formula) => (
    <Link key={f.builtin ? f.key : f.id} to={`/f/${f.builtin ? f.key : f.id}`} className="tile f-tile">
      <div className="tile-head"><span className="tile-ico"><Icon name={f.icon || 'fx'} size={15} /></span><span className="tile-name">{f.name}</span>{f.pinned && <Icon name="pin" size={12} className="muted" />}</div>
      <span className="muted small">{f.about}</span>
      {f.runs > 0 && <span className="muted small">{f.runs} kept {f.runs === 1 ? 'answer' : 'answers'}</span>}
    </Link>
  )
  return (
    <div className="stack loose">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Ready calculations</h1><p className="muted">Fill in the boxes, get the answer. Some fill themselves in from your other Pocket apps.</p></div>
        <button type="button" className="btn primary" onClick={() => navigate('/f/new')}><Icon name="plus" size={15} />My own</button>
      </div>
      <div className="search-box wide"><Icon name="search" size={15} /><input value={q} onChange={e => setQ(e.target.value)} placeholder="Paint, loan, fuel, BMI, circle…" aria-label="Search calculations" /></div>
      {data.mine.length > 0 && <section><h3 className="sec-title">Mine</h3><div className="tiles">{data.mine.map(card)}</div></section>}
      {data.categories.map(cat => {
        const fs = data.library.filter(f => f.category === cat)
        if (!fs.length) return null
        return <section key={cat}><h3 className="sec-title">{cat}</h3><div className="tiles">{fs.map(card)}</div></section>
      })}
      {!data.mine.length && !data.library.length && <p className="muted">Nothing matches “{dq}”.</p>}
    </div>
  )
}
