// A notebook seen through its share link: read-only, no account needed.
import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Cell, Notebook, Settings } from '../lib/types'
import { Icon } from '../components/icons'
import { Empty, errMsg, Loading } from '../components/ui'
import { KIND_INFO, numberFormat, type CellProps } from '../components/cells/common'
import { CalcCell } from '../components/cells/CalcCell'
import { TextCell } from '../components/cells/TextCell'
import { GraphCell } from '../components/cells/GraphCell'
import { GeometryCell } from '../components/cells/GeometryCell'
import { DataCell } from '../components/cells/DataCell'
import { SolveCell } from '../components/cells/SolveCell'
import { useWorked } from './NotebookPage'
import { newEnv } from '../lib/calc'

const CELLS: Record<string, (p: CellProps) => React.ReactElement> = { calc: CalcCell, text: TextCell, graph: GraphCell, geometry: GeometryCell, data: DataCell, solve: SolveCell }

interface SharedNB { notebook: Pick<Notebook, 'title' | 'about' | 'subject' | 'angle' | 'units' | 'updated_at' | 'color'>; cells: Cell[]; owner: string; number_fmt: string; digits: number; rates: Record<string, number> }

export default function Shared() {
  const { key } = useParams()
  const [d, setD] = useState<SharedNB | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => { api.get<SharedNB>(`${V1}/public/s/${key}`).then(x => { setD(x); document.title = x.notebook.title }).catch(e => setErr(errMsg(e))) }, [key])
  const format = useMemo(() => numberFormat({ number_fmt: d?.number_fmt, digits: d?.digits } as Settings), [d])
  const worked = useWorked(d?.cells || [], d?.notebook || null, d?.rates || {}, format)
  if (err) return <div className="shared-page"><Empty title="This link does not work any more" icon="link">{err}</Empty></div>
  if (!d) return <div className="shared-page"><Loading /></div>
  const deg = d.notebook.angle !== 'rad'
  return (
    <div className="shared-page">
      <header className="shared-head">
        <img src="/favicon.svg" alt="" width={28} height={28} />
        <div className="grow"><h1>{d.notebook.title}</h1><p className="muted small">{d.owner ? `Shared by ${d.owner}` : 'Shared'} · Pocket Math</p></div>
        <button type="button" className="btn sm" onClick={() => window.print()}><Icon name="printer" size={14} />Print</button>
      </header>
      {d.notebook.about && <p>{d.notebook.about}</p>}
      <div className="cells ro">
        {d.cells.filter(c => !c.collapsed).map(c => {
          const C = CELLS[c.kind] || CalcCell
          return <section key={c.id} className={`cell cell-${c.kind}`}>
            {c.title && <div className="cell-head"><span className="cell-kind"><Icon name={KIND_INFO[c.kind]?.icon || 'calc'} size={14} /></span><b className="cell-title-ro">{c.title}</b></div>}
            <div className="cell-body"><C cell={c} onChange={() => {}} env={worked.before[c.id] || newEnv({ units: d.notebook.units, deg })} lines={worked.lines[c.id]} format={format} settings={null} readOnly deg={deg} /></div>
          </section>
        })}
      </div>
    </div>
  )
}
