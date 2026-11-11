// Every kept answer, newest first: what it was for, what was filled in.
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import { fmtDateTime } from '../lib/format'
import type { Run } from '../lib/types'
import { Icon } from '../components/icons'
import { download, Empty, errMsg, Loading, useToast } from '../components/ui'

export default function Answers() {
  const { toast } = useToast()
  const [runs, setRuns] = useState<Run[] | null>(null)
  const load = useCallback(() => { api.get<{ runs: Run[] }>(`${V1}/runs?limit=500`).then(r => setRuns(r.runs)).catch(e => toast(errMsg(e), 'err')) }, [toast])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  useEffect(() => { const id = window.location.hash.slice(1); if (id && runs) document.getElementById('run-' + id)?.scrollIntoView({ block: 'center' }) }, [runs])
  if (!runs) return <Loading />
  return (
    <div className="stack">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Kept answers</h1><p className="muted">Answers you kept from the ready calculations, with what you filled in.</p></div>
        {runs.length > 0 && <button type="button" className="btn" onClick={() => download(`${V1}/export.csv`)}><Icon name="download" size={15} />Spreadsheet (CSV)</button>}
      </div>
      {!runs.length ? <Empty title="Nothing kept yet" icon="check-circle">Open a <Link to="/formulas">ready calculation</Link> and press “Keep this answer”.</Empty> :
        <div className="card"><ul className="list">{runs.map(r => <li key={r.id} id={`run-${r.id}`} className={`list-row ${window.location.hash === '#' + r.id ? 'hl' : ''}`}>
          <span className="grow">
            <b>{r.title || r.formula_name}</b>
            <span className="muted small block-line">{r.title ? r.formula_name + ' · ' : ''}{fmtDateTime(r.created_at)}{r.source ? ` · from ${r.source}` : ''}</span>
            <span className="small block-line">{(r.outputs || []).filter(o => o.text).map(o => `${o.label}: ${o.text}`).join(' · ')}</span>
            {r.note && <span className="muted small block-line">{r.note}</span>}
          </span>
          <span className="pill">{r.main}</span>
          <Link className="btn sm ghost" to={r.formula_key ? `/f/${r.formula_key}` : r.formula_id ? `/f/${r.formula_id}` : '/formulas'}>Open</Link>
          <button type="button" className="btn ghost icon-only sm" aria-label="Forget this answer" title="Forget" onClick={async () => { await api.del(`${V1}/runs/${r.id}`); setRuns(runs.filter(x => x.id !== r.id)); announceChange() }}><Icon name="trash" size={14} /></button>
        </li>)}</ul></div>}
    </div>
  )
}
