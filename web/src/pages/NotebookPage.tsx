// A notebook: cells one under the other. Calculations are worked out in the
// browser as you type (what one sets, the cells below can use); every change
// is saved a moment later.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useAppState, useData } from '../lib/data'
import { newEnv, runLine, snapshot, type Env, type Format, type Line } from '../lib/calc'
import type { Cell, CellKind, Notebook, NotebookFull } from '../lib/types'
import { useCrumb } from '../components/Layout'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Confirm, copyText, CopyField, download, Empty, errMsg, Loading, MenuButton, Modal, Switch, useToast, type MenuItem } from '../components/ui'
import { KIND_INFO, numberFormat, type CellProps } from '../components/cells/common'
import { CalcCell } from '../components/cells/CalcCell'
import { TextCell } from '../components/cells/TextCell'
import { GraphCell } from '../components/cells/GraphCell'
import { GeometryCell } from '../components/cells/GeometryCell'
import { DataCell } from '../components/cells/DataCell'
import { SolveCell } from '../components/cells/SolveCell'
import { ChatSend } from '../components/AppPickers'
import { AIExplain, AIWrite } from '../components/AIHelp'

const CELLS: Record<CellKind, (p: CellProps) => React.ReactElement> = { calc: CalcCell, text: TextCell, graph: GraphCell, geometry: GeometryCell, data: DataCell, solve: SolveCell }

/** Works out all calculation cells in order; what each cell sees. */
export function useWorked(cells: Cell[], nb: Pick<Notebook, 'units' | 'angle'> | null, rates: Record<string, number>, format: Format = {}) {
  return useMemo(() => {
    const env = newEnv({ units: nb?.units ?? true, deg: nb?.angle !== 'rad', rates })
    const lines: Record<number, Line[]> = {}
    const before: Record<number, Env> = {}
    for (const c of cells) {
      before[c.id] = snapshot(env)
      if (c.kind !== 'calc') continue
      env.break()
      lines[c.id] = c.body.split('\n').map(l => runLine(env, l, format))
    }
    return { lines, before }
  }, [cells, nb?.units, nb?.angle, rates, format])
}

export default function NotebookPage() {
  const { id } = useParams()
  const nbId = Number(id)
  const navigate = useNavigate()
  const { toast } = useToast()
  const { settings } = useApp()
  const { rates } = useData()
  const appState = useAppState()
  const [nb, setNb] = useState<Notebook | null>(null)
  const [cells, setCells] = useState<Cell[]>([])
  const [shareUrl, setShareUrl] = useState('')
  const [err, setErr] = useState('')
  const [saving, setSaving] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const [modal, setModal] = useState<null | { kind: 'share' | 'chat' | 'delete' | 'check' | 'write' | 'explain'; cell?: Cell; after?: number }>(null)
  useCrumb(nb?.title, '/notebooks')

  const load = useCallback(async () => {
    try {
      const r = await api.get<NotebookFull>(`${V1}/notebooks/${nbId}`)
      setNb(r.notebook); setCells(r.cells); setShareUrl(r.share_url); setErr('')
    } catch (e) { setErr(errMsg(e)) }
  }, [nbId])
  useEffect(() => { setNb(null); load() }, [load])

  // ---- saving: each cell a moment after its last change ----
  const pending = useRef(new Map<number, Partial<Cell>>())
  const timer = useRef<number | undefined>(undefined)
  const flush = useCallback(async () => {
    window.clearTimeout(timer.current)
    const jobs = [...pending.current.entries()]
    pending.current.clear()
    if (!jobs.length) return
    setSaving('saving')
    try {
      for (const [cid, patch] of jobs) await api.put(`${V1}/cells/${cid}`, patch)
      setSaving('saved')
      announceChange()
    } catch (e) { setSaving('error'); toast('Not saved: ' + errMsg(e), 'err') }
  }, [toast])
  useEffect(() => {
    const h = () => { for (const [cid, patch] of pending.current) fetch(`${V1}/cells/${cid}`, { method: 'PUT', body: JSON.stringify(patch), headers: { 'Content-Type': 'application/json' }, keepalive: true, credentials: 'same-origin' }) }
    window.addEventListener('beforeunload', h)
    return () => { window.removeEventListener('beforeunload', h); flush() }
  }, [flush])
  const change = useCallback((cid: number, patch: Partial<Cell>) => {
    setCells(cs => cs.map(c => (c.id === cid ? { ...c, ...patch } : c)))
    const keep: Partial<Cell> = {}
    for (const k of ['body', 'data', 'summary', 'title', 'collapsed', 'kind'] as const) if (k in patch) (keep as Record<string, unknown>)[k] = patch[k]
    pending.current.set(cid, { ...(pending.current.get(cid) || {}), ...keep })
    setSaving('saving')
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(flush, 700)
  }, [flush])

  const saveNb = async (patch: Partial<Notebook>) => {
    if (!nb) return
    setNb({ ...nb, ...patch })
    try { const r = await api.put<NotebookFull>(`${V1}/notebooks/${nb.id}`, patch); setNb(r.notebook); announceChange() } catch (e) { toast(errMsg(e), 'err') }
  }

  const addCell = async (kind: CellKind, after?: number, init?: Partial<Cell>) => {
    await flush()
    try {
      const r = await api.post<{ cell: Cell }>(`${V1}/notebooks/${nbId}/cells`, { kind, after: after ?? (cells.length ? cells[cells.length - 1].id : undefined), ...init })
      setCells(cs => {
        const i = after !== undefined ? cs.findIndex(c => c.id === after) + 1 : cs.length
        const next = [...cs]; next.splice(i, 0, r.cell); return next
      })
      setTimeout(() => document.querySelector<HTMLElement>(`[data-cell="${r.cell.id}"] textarea, [data-cell="${r.cell.id}"] input`)?.focus(), 60)
    } catch (e) { toast(errMsg(e), 'err') }
  }
  const delCell = async (c: Cell) => {
    pending.current.delete(c.id)
    setCells(cs => cs.filter(x => x.id !== c.id))
    try { await api.del(`${V1}/cells/${c.id}`) } catch (e) { toast(errMsg(e), 'err'); load() }
  }
  const copyCell = async (c: Cell) => {
    await flush()
    try { const r = await api.post<{ cell: Cell }>(`${V1}/cells/${c.id}/copy`); setCells(cs => { const i = cs.findIndex(x => x.id === c.id) + 1; const n = [...cs]; n.splice(i, 0, r.cell); return n }) } catch (e) { toast(errMsg(e), 'err') }
  }
  const move = async (c: Cell, d: number) => {
    const i = cells.findIndex(x => x.id === c.id), j = i + d
    if (j < 0 || j >= cells.length) return
    const next = [...cells]; [next[i], next[j]] = [next[j], next[i]]
    setCells(next)
    try { await api.put(`${V1}/notebooks/${nbId}/order`, { ids: next.map(x => x.id) }) } catch (e) { toast(errMsg(e), 'err') }
  }

  const format = useMemo(() => numberFormat(settings), [settings])
  const worked = useWorked(cells, nb, rates, format)
  const deg = nb?.angle !== 'rad'

  if (err) return <Empty title="This notebook can't be opened" icon="alert">{err}</Empty>
  if (!nb) return <Loading />

  const answerLines = () => cells.filter(c => c.kind === 'calc').flatMap(c => c.body.split('\n').map((l, i) => { const r = worked.lines[c.id]?.[i]; return r && (r.kind === 'value' || r.kind === 'set') ? `${l.trim()} = ${r.text}` : '' }).filter(Boolean))
  const menu = (): MenuItem[] => [
    { heading: 'Working out' },
    { label: 'Angles in degrees', checked: nb.angle === 'deg', onSelect: () => saveNb({ angle: 'deg' }) },
    { label: 'Angles in radians', checked: nb.angle === 'rad', onSelect: () => saveNb({ angle: 'rad' }) },
    { label: 'Read units and money', checked: nb.units, onSelect: () => saveNb({ units: !nb.units }) },
    'sep',
    { label: nb.pinned ? 'Unpin' : 'Pin to the top', icon: 'pin', onSelect: () => saveNb({ pinned: !nb.pinned }) },
    { label: 'Share a read-only link…', icon: 'link', onSelect: () => setModal({ kind: 'share' }) },
    { label: 'Make a copy', icon: 'copy', onSelect: async () => { await flush(); const r = await api.post<NotebookFull>(`${V1}/notebooks/${nb.id}/copy`); announceChange(); navigate(`/n/${r.notebook.id}`) } },
    { label: 'Download as text (Markdown)', icon: 'download', onSelect: async () => { await flush(); download(`${V1}/notebooks/${nb.id}/export.md`) } },
    { label: 'Print or save as PDF', icon: 'printer', onSelect: () => window.print() },
    'sep',
    { heading: 'Other apps' },
    { label: 'Keep in Pocket Notes', icon: 'note', disabled: !appState('notes', 'notes:write').ok, hint: appState('notes', 'notes:write').ok ? undefined : 'not linked', onSelect: async () => {
      await flush()
      try { const r = await api.post<{ text: string; url?: string }>(`${V1}/apps/notes`, { notebook_id: nb.id }); toast(r.text, 'ok', r.url ? { label: 'Open', run: () => window.open(r.url, '_blank', 'noopener') } : undefined) } catch (e) { toast(errMsg(e), 'err') }
    } },
    { label: 'Send to Pocket Chat…', icon: 'message', disabled: !appState('chat', 'chat:write').ok, hint: appState('chat', 'chat:write').ok ? undefined : 'not linked', onSelect: () => setModal({ kind: 'chat' }) },
    { label: 'Save in Pocket Files', icon: 'upload', disabled: !appState('files', 'files:write').ok, hint: appState('files', 'files:write').ok ? undefined : 'not linked', onSelect: async () => {
      await flush()
      try { const md = await api.getText(`${V1}/notebooks/${nb.id}/export.md`); const r = await api.post<{ text: string }>(`${V1}/apps/files`, { name: nb.title, kind: 'md', text: md }); toast(r.text, 'ok') } catch (e) { toast(errMsg(e), 'err') }
    } },
    'sep',
    { label: 'Check my working (AI)', icon: 'sparkles', onSelect: async () => { await flush(); setModal({ kind: 'check' }) } },
    { label: nb.archived ? 'Bring back from the archive' : 'Archive', icon: 'archive', onSelect: () => saveNb({ archived: !nb.archived }) },
    { label: 'Delete…', icon: 'trash', danger: true, onSelect: () => setModal({ kind: 'delete' }) },
  ]

  return (
    <div className="nb-page">
      <div className="nb-head">
        <div className="grow">
          <input className="nb-title" value={nb.title} onChange={e => setNb({ ...nb, title: e.target.value })} onBlur={() => saveNb({ title: nb.title })} aria-label="Title" />
          <div className="nb-meta">
            <SubjectPicker value={nb.subject} onChange={v => saveNb({ subject: v })} />
            <span className="pill" title="How plain numbers in sin, cos and tan are read">{nb.angle === 'deg' ? 'degrees' : 'radians'}</span>
            {nb.shared && <button type="button" className="pill accent" onClick={() => setModal({ kind: 'share' })}><Icon name="link" size={12} />shared</button>}
            <span className={`save-state ${saving === 'error' ? 'error' : ''}`}>{saving === 'saving' ? 'Saving…' : saving === 'saved' ? 'Saved' : saving === 'error' ? 'Not saved' : ''}</span>
          </div>
        </div>
        <button type="button" className="btn hide-sm" onClick={() => setModal({ kind: 'write' })} title="Write a problem in words (or take a photo) and get the sums"><Icon name="sparkles" size={15} />From words</button>
        <MenuButton label="Notebook" className="btn icon-only" align="right" width={260} items={menu}><Icon name="more" size={18} /></MenuButton>
      </div>

      <div className="cells">
        {cells.map((c, i) => {
          const C = CELLS[c.kind] || CalcCell
          const info = KIND_INFO[c.kind]
          const cellMenu = (): MenuItem[] => [
            ...(c.kind === 'calc' ? [
              { label: 'Explain this (AI)', icon: 'sparkles', onSelect: () => setModal({ kind: 'explain', cell: c }) } as MenuItem,
              { label: 'Make a calculation to reuse…', icon: 'fx', onSelect: () => navigate(`/f/new?code=${encodeURIComponent(c.body)}&name=${encodeURIComponent(c.title || nb.title)}`) } as MenuItem,
              { label: 'Copy the answers', icon: 'copy', onSelect: async () => { const t = c.body.split('\n').map((l, k) => { const r = worked.lines[c.id]?.[k]; return r?.text ? `${l.trim()} = ${r.text}` : l }).join('\n'); toast(await copyText(t) ? 'Copied' : 'Could not copy', 'ok') } } as MenuItem,
              'sep' as const,
            ] : []),
            { label: 'Add below', icon: 'plus', submenu: (Object.keys(KIND_INFO) as CellKind[]).map(k => ({ label: KIND_INFO[k].name, icon: KIND_INFO[k].icon, onSelect: () => addCell(k, c.id) })) },
            { label: 'Move up', icon: 'arrow-up', disabled: i === 0, onSelect: () => move(c, -1) },
            { label: 'Move down', icon: 'arrow-down', disabled: i === cells.length - 1, onSelect: () => move(c, 1) },
            { label: 'Duplicate', icon: 'copy', onSelect: () => copyCell(c) },
            { label: c.collapsed ? 'Show' : 'Fold away', icon: c.collapsed ? 'eye' : 'eye-off', onSelect: () => change(c.id, { collapsed: !c.collapsed }) },
            'sep',
            { label: 'Delete', icon: 'trash', danger: true, onSelect: () => delCell(c) },
          ]
          return (
            <section key={c.id} className={`cell cell-${c.kind} ${c.collapsed ? 'collapsed' : ''}`} data-cell={c.id}>
              <div className="cell-head">
                <span className="cell-kind" title={info?.name}><Icon name={info?.icon || 'calc'} size={14} /></span>
                <input className="cell-title" value={c.title} placeholder={info?.name} onChange={e => change(c.id, { title: e.target.value })} aria-label="Cell title" />
                <MenuButton label="Cell" className="btn ghost icon-only sm" align="right" width={240} items={cellMenu}><Icon name="more" size={16} /></MenuButton>
              </div>
              {!c.collapsed && <div className="cell-body"><C cell={c} onChange={p => change(c.id, p)} env={worked.before[c.id] || newEnv({ units: nb.units, deg })} lines={worked.lines[c.id]} format={format} settings={settings} notebookId={nb.id} deg={deg} /></div>}
              <AddBar onAdd={k => addCell(k, c.id)} />
            </section>
          )
        })}
        {cells.length === 0 && <Empty title="An empty notebook" icon="calc">Add a calculation, some words, a graph or a table.</Empty>}
        <AddBar onAdd={k => addCell(k)} open />
      </div>

      {modal?.kind === 'share' && <ShareModal nb={nb} url={shareUrl} onClose={() => setModal(null)} onChange={(n, u) => { setNb(n); setShareUrl(u) }} />}
      {modal?.kind === 'chat' && <ChatSend notebookId={nb.id} title={nb.title} lines={answerLines()} onClose={() => { setModal(null); load() }} />}
      {modal?.kind === 'delete' && <Confirm title="Delete this notebook?" message={`“${nb.title}” and its ${cells.length} cells go for good.`} confirmLabel="Delete" onCancel={() => setModal(null)}
        onConfirm={async () => { await api.del(`${V1}/notebooks/${nb.id}`); announceChange(); navigate('/notebooks') }} />}
      {modal?.kind === 'check' && <CheckModal id={nb.id} onClose={() => setModal(null)} />}
      {modal?.kind === 'explain' && modal.cell && <AIExplain text={modal.cell.body} angle={nb.angle} onClose={() => setModal(null)} />}
      {modal?.kind === 'write' && <AIWrite onClose={() => setModal(null)} onLines={async (text, title) => { setModal(null); await addCell('calc', undefined, { body: text, title } as Partial<Cell>) }} />}
    </div>
  )
}

function AddBar({ onAdd, open = false }: { onAdd: (k: CellKind) => void; open?: boolean }) {
  return (
    <div className={`add-bar ${open ? 'open' : ''}`}>
      <span className="add-line" />
      <div className="add-kinds">
        {(Object.keys(KIND_INFO) as CellKind[]).map(k => <button key={k} type="button" className="add-kind" onClick={() => onAdd(k)} title={KIND_INFO[k].about}><Icon name={KIND_INFO[k].icon} size={14} />{KIND_INFO[k].name}</button>)}
      </div>
      <span className="add-line" />
    </div>
  )
}

function SubjectPicker({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [list, setList] = useState<string[]>([])
  useEffect(() => { api.get<{ subjects: { name: string }[] }>(`${V1}/notebooks?limit=1`).then(r => setList(r.subjects.map(s => s.name))).catch(() => {}) }, [])
  const opts = [...new Set([...['Home', 'Money', 'School', 'Work', 'Travel', 'Car', 'Kitchen'], ...list, ...(value ? [value] : [])])].map(s => ({ value: s, label: s }))
  return <div className="subject-pick"><Select value={value} onChange={onChange} options={[{ value: '', label: 'No folder' }, ...opts]} placeholder="Folder" onCreate={l => l.trim()} createLabel="New folder" ariaLabel="Folder" /></div>
}

function ShareModal({ nb, url, onClose, onChange }: { nb: Notebook; url: string; onClose: () => void; onChange: (n: Notebook, url: string) => void }) {
  const { toast } = useToast()
  const set = async (shared: boolean, renew = false) => {
    try { const r = await api.put<{ notebook: Notebook; share_url: string }>(`${V1}/notebooks/${nb.id}/share`, { shared, new: renew }); onChange(r.notebook, r.share_url); announceChange() } catch (e) { toast(errMsg(e), 'err') }
  }
  const full = url && !/^https?:/.test(url) ? window.location.origin + url : url
  return <Modal title="Share a read-only link" onClose={onClose} size="sm">
    <div className="stack">
      <label className="check-row"><Switch checked={nb.shared} onChange={v => set(v)} label="Share" /><span>Anyone with the link can see this notebook, with its answers and graphs — without an account. They can't change it.</span></label>
      {nb.shared && full && <><CopyField value={full} label="Link" /><button type="button" className="linkish small" onClick={() => set(true, true)}>Make a new link (the old one stops working)</button></>}
    </div>
  </Modal>
}

function CheckModal({ id, onClose }: { id: number; onClose: () => void }) {
  const [res, setRes] = useState<{ ok: boolean; points: { line: string; text: string }[]; summary: string } | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => { api.post<typeof res>(`${V1}/notebooks/${id}/ai/check`).then(setRes).catch(e => setErr(errMsg(e))) }, [id])
  return <Modal title="Check my working" onClose={onClose}>
    {err ? <p className="bad">{err}</p> : !res ? <Loading label="Pocket AI is reading it through…" /> : <div className="stack">
      <p><b>{res.summary}</b></p>
      {res.points?.length ? <ul className="check-list">{res.points.map((p, i) => <li key={i}>{p.line && <code>{p.line}</code>}<span>{p.text}</span></li>)}</ul> : <p className="muted">Nothing to look at.</p>}
      <p className="muted small">The answers come from the calculator; the AI only looks at how the sums are set up.</p>
    </div>}
  </Modal>
}
