// Pickers for the other Pocket apps: bring a table in, fill a box in, send
// something out. Each shows plainly what is there to pick.
import { useEffect, useState } from 'react'
import { api, V1 } from '../lib/api'
import type { Column } from '../lib/types'
import { Icon } from './icons'
import { Select } from './Select'
import { Empty, errMsg, Loading, Modal, Segmented, useToast } from './ui'

export type PickKind = 'tables' | 'files' | 'money' | 'home' | 'garage' | 'health' | 'wealth' | 'tables-out'

type Opt = { id: string; name: string; sub?: string }

function useList<T>(url: string | null, pick: (r: T) => Opt[]) {
  const [list, setList] = useState<Opt[] | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    if (!url) return
    setList(null); setErr('')
    api.get<T>(url).then(r => setList(pick(r))).catch(e => { setErr(errMsg(e)); setList([]) })
  }, [url]) // eslint-disable-line react-hooks/exhaustive-deps
  return { list, err }
}

const TITLES: Record<PickKind, string> = {
  tables: 'A table from Pocket Tables', files: 'A CSV file from Pocket Files', money: 'Spending by month from Pocket Money', home: 'A meter from Pocket Home',
  garage: 'Fill-ups from Pocket Garage', health: 'From Pocket Health', wealth: 'Net worth from Pocket Wealth', 'tables-out': 'Add the rows to Pocket Tables',
}

export function AppPicker({ kind, onClose, onColumns, columns }: { kind: PickKind; onClose: () => void; onColumns: (c: Column[], source: string) => void; columns?: Column[] }) {
  const { toast } = useToast()
  const [busy, setBusy] = useState(false)
  const [months, setMonths] = useState('12')
  const [metric, setMetric] = useState('weight')
  const [path, setPath] = useState('/')
  const load = async (url: string, source: string, post?: unknown) => {
    setBusy(true)
    try {
      const r = post ? await api.post<{ columns: Column[] }>(url, post) : await api.get<{ columns: Column[] }>(url)
      if (!r.columns?.length || !r.columns[0].values.length) { toast('There is nothing there yet', 'err'); setBusy(false); return }
      onColumns(r.columns, source)
    } catch (e) { toast(errMsg(e), 'err'); setBusy(false) }
  }
  const listUrl = { tables: `${V1}/apps/tables`, 'tables-out': `${V1}/apps/tables`, home: `${V1}/apps/home/meters`, garage: `${V1}/apps/garage`, files: `${V1}/apps/files?path=${encodeURIComponent(path)}` }[kind as string] || null
  const { list, err } = useList<Record<string, Record<string, unknown>[]>>(listUrl, r => {
    if (kind === 'tables' || kind === 'tables-out') return (r.tables || []).map(t => ({ id: String(t.id), name: String(t.name), sub: String(t.base || '') }))
    if (kind === 'home') return (r.meters || []).map(m => ({ id: String(m.id), name: String(m.name), sub: [m.kind, m.home].filter(Boolean).join(' · ') }))
    if (kind === 'garage') return (r.vehicles || []).map(v => ({ id: String(v.id), name: String(v.name), sub: v.economy ? `${Number(v.economy).toFixed(1)} L/100km` : '' }))
    if (kind === 'files') return ((r as unknown as { entries: Record<string, unknown>[] }).entries || []).map(e => ({ id: String(e.path), name: String(e.name), sub: String(e.kind) }))
    return []
  })

  let body: React.ReactNode
  if (kind === 'money') body = <div className="stack">
    <p className="muted">What you spent and what came in, month by month, from your transactions.</p>
    <Segmented value={months} onChange={setMonths} options={[{ value: '6', label: '6 months' }, { value: '12', label: 'A year' }, { value: '24', label: 'Two years' }]} label="How far back" />
    <button type="button" className="btn primary" disabled={busy} onClick={() => load(`${V1}/apps/money/months?months=${months}`, 'money')}>{busy ? 'Bringing it in…' : 'Bring it in'}</button>
  </div>
  else if (kind === 'health') body = <div className="stack">
    <Segmented value={metric} onChange={setMetric} options={[{ value: 'weight', label: 'Weight' }, { value: 'body_fat', label: 'Body fat' }, { value: 'bmi', label: 'BMI' }]} label="What" />
    <button type="button" className="btn primary" disabled={busy} onClick={() => load(`${V1}/apps/health/${metric}?range=y`, 'health')}>{busy ? 'Bringing it in…' : 'The last year'}</button>
  </div>
  else if (kind === 'wealth') body = <div className="stack">
    <p className="muted">Your net worth and savings, over the last two years.</p>
    <button type="button" className="btn primary" disabled={busy} onClick={() => load(`${V1}/apps/wealth`, 'wealth')}>{busy ? 'Bringing it in…' : 'Bring it in'}</button>
  </div>
  else if (kind === 'tables-out') body = <TablesOut list={list} err={err} columns={columns || []} onDone={onClose} />
  else body = <>
    {kind === 'files' && path !== '/' && <button type="button" className="btn sm ghost" onClick={() => setPath(path.replace(/\/[^/]+\/?$/, '') || '/')}><Icon name="arrow-left" size={14} />Up</button>}
    {err && <p className="bad small">{err}</p>}
    {!list ? <Loading /> : !list.length && !err ? <Empty title="Nothing to pick yet" icon="inbox" /> :
      <ul className="pick-list">{list.map(o => <li key={o.id}><button type="button" className="pick-row" disabled={busy} onClick={() => {
        if (kind === 'files' && o.sub === 'folder') { setPath(o.id); return }
        if (kind === 'tables') load(`${V1}/apps/tables/${o.id}`, 'tables')
        if (kind === 'files') load(`${V1}/apps/files/import`, 'files', { path: o.id })
        if (kind === 'home') load(`${V1}/apps/home/meters/${o.id}`, 'home')
        if (kind === 'garage') load(`${V1}/apps/garage/${o.id}`, 'garage')
      }}><Icon name={o.sub === 'folder' ? 'grid' : 'table'} size={16} /><span className="grow"><b>{o.name}</b>{o.sub && <span className="muted small"> · {o.sub}</span>}</span><Icon name="chevron-right" size={14} /></button></li>)}</ul>}
  </>
  return <Modal title={TITLES[kind]} onClose={onClose}>{body}</Modal>
}

function TablesOut({ list, err, columns, onDone }: { list: Opt[] | null; err: string; columns: Column[]; onDone: () => void }) {
  const { toast } = useToast()
  const [table, setTable] = useState('')
  const [fields, setFields] = useState<string[] | null>(null)
  const [map, setMap] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    if (!table) return
    setFields(null)
    api.get<{ fields: string[] }>(`${V1}/apps/tables/${table}/fields`).then(r => {
      setFields(r.fields)
      const m: Record<string, string> = {}
      for (const c of columns) { const f = r.fields.find(x => x.toLowerCase() === c.name.toLowerCase()); if (f) m[c.name] = f }
      setMap(m)
    }).catch(e => toast(errMsg(e), 'err'))
  }, [table]) // eslint-disable-line react-hooks/exhaustive-deps
  const send = async () => {
    const n = Math.max(0, ...columns.map(c => c.values.length))
    const rows: Record<string, string>[] = []
    for (let i = 0; i < n; i++) { const r: Record<string, string> = {}; for (const c of columns) if (map[c.name]) r[map[c.name]] = c.values[i] || ''; if (Object.values(r).some(Boolean)) rows.push(r) }
    setBusy(true)
    try { const r = await api.post<{ text: string; url?: string }>(`${V1}/apps/tables/${table}/rows`, { rows }); toast(r.text, 'ok', r.url ? { label: 'Open', run: () => window.open(r.url, '_blank', 'noopener') } : undefined); onDone() } catch (e) { toast(errMsg(e), 'err') }
    setBusy(false)
  }
  if (err) return <p className="bad">{err}</p>
  if (!list) return <Loading />
  return <div className="stack">
    <label className="field"><span className="field-label">Table</span><Select value={table} onChange={setTable} options={list.map(o => ({ value: o.id, label: o.name, hint: o.sub }))} placeholder="Pick a table" /></label>
    {table && !fields && <Loading />}
    {fields && <>
      <p className="muted small">Which column goes where:</p>
      {columns.map(c => <label key={c.name} className="field row-field"><span className="field-label">{c.name}</span><Select value={map[c.name] || ''} onChange={v => setMap({ ...map, [c.name]: v })} options={[{ value: '', label: '— leave out —' }, ...fields.map(f => ({ value: f, label: f }))]} /></label>)}
      <button type="button" className="btn primary" disabled={busy || !Object.values(map).some(Boolean)} onClick={send}>{busy ? 'Adding…' : 'Add the rows'}</button>
    </>}
  </div>
}

/** A box filled from another app: pick one of the values it knows. */
export function FillPicker({ source, onPick, onClose }: { source: string; onPick: (value: string) => void; onClose: () => void }) {
  const [opts, setOpts] = useState<{ label: string; value: string; sub: string }[] | null>(null)
  const [app, setApp] = useState('')
  const [err, setErr] = useState('')
  useEffect(() => {
    api.get<{ options: { label: string; value: string; sub: string }[]; app: string }>(`${V1}/apps/fill?source=${encodeURIComponent(source)}`).then(r => { setOpts(r.options); setApp(r.app) }).catch(e => { setErr(errMsg(e)); setOpts([]) })
  }, [source])
  return <Modal title={app ? `From ${app}` : 'Fill in from another app'} onClose={onClose} size="sm">
    {err && <p className="bad small">{err}</p>}
    {!opts ? <Loading /> : !opts.length && !err ? <Empty title="Nothing there yet" icon="inbox">Add it in the app first.</Empty> :
      <ul className="pick-list">{opts.map((o, i) => <li key={i}><button type="button" className="pick-row" onClick={() => { onPick(o.value); onClose() }}><span className="grow"><b>{o.label}</b>{o.sub && <span className="muted small"> · {o.sub}</span>}</span><span className="pill">{o.value}</span></button></li>)}</ul>}
  </Modal>
}

/** Send lines (or a notebook) to a chat in Pocket Chat. */
export function ChatSend({ notebookId, title, lines, onClose }: { notebookId?: number; title?: string; lines: string[]; onClose: () => void }) {
  const { toast } = useToast()
  const [chats, setChats] = useState<Opt[] | null>(null)
  const [to, setTo] = useState('')
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => { api.get<{ conversations: { id: string; name: string }[] }>(`${V1}/apps/chats`).then(r => setChats(r.conversations.map(c => ({ id: c.id, name: c.name || 'Chat' })))).catch(() => setChats([])) }, [])
  const send = async () => {
    setBusy(true)
    try { const r = await api.post<{ text: string; url?: string }>(`${V1}/apps/chat`, { to, text, notebook_id: notebookId, title, lines }); toast(r.text, 'ok', r.url ? { label: 'Open', run: () => window.open(r.url, '_blank', 'noopener') } : undefined); onClose() } catch (e) { toast(errMsg(e), 'err') }
    setBusy(false)
  }
  return <Modal title="Send to Pocket Chat" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={busy} onClick={send}>{busy ? 'Sending…' : 'Send'}</button></>}>
    <div className="stack">
      <label className="field"><span className="field-label">To</span>{chats ? <Select value={to} onChange={setTo} options={[{ value: '', label: 'Your own notes chat' }, ...chats.map(c => ({ value: c.id, label: c.name }))]} /> : <Loading />}</label>
      <label className="field"><span className="field-label">With a few words</span><input value={text} onChange={e => setText(e.target.value)} placeholder="Here is what it comes to" /></label>
      {notebookId ? <p className="muted small">The notebook goes as a read-only link: anyone in the chat can open it (switch the link off in the notebook's menu).</p> : null}
      {lines.length > 0 && <pre className="set-pre small">{lines.slice(0, 6).join('\n')}</pre>}
    </div>
  </Modal>
}
