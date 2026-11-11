// One ready calculation: fill in the boxes, see the answers at once, keep
// the ones worth keeping. Your own can be changed; a built-in can be copied.
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useAppState, useData } from '../lib/data'
import { choiceLabels, runFormula } from '../lib/formula'
import { ago } from '../lib/format'
import type { FInput, FOutput, Formula, Run } from '../lib/types'
import { useCrumb } from '../components/Layout'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Confirm, copyText, Empty, errMsg, Loading, MenuButton, Modal, useToast, type MenuItem } from '../components/ui'
import { numberFormat } from '../components/cells/common'
import { ChatSend, FillPicker, AppPicker } from '../components/AppPickers'
import { CalcCell } from '../components/cells/CalcCell'
import { newEnv, runLine } from '../lib/calc'

const FROM_APP: Record<string, [string, string]> = {
  rooms: ['rooms', 'rooms:read'], health: ['health', 'health:read'], garage: ['garage', 'vehicles:read'], wealth: ['wealth', 'wealth:read'], home: ['home', 'home:read'], money: ['money', 'accounts:read'],
}
const APP_NAME: Record<string, string> = { rooms: 'Pocket Rooms', health: 'Pocket Health', garage: 'Pocket Garage', wealth: 'Pocket Wealth', home: 'Pocket Home', money: 'Pocket Money' }

export default function FormulaPage() {
  const { id = '' } = useParams()
  const [sp] = useSearchParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { settings } = useApp()
  const { rates } = useData()
  const appState = useAppState()
  const isNew = id === 'new'
  const builtin = !/^\d+$/.test(id) && !isNew
  const [f, setF] = useState<Formula | null>(null)
  const [runs, setRuns] = useState<Run[]>([])
  const [values, setValues] = useState<Record<string, string>>({})
  const [editing, setEditing] = useState(isNew)
  const [fill, setFill] = useState<FInput | null>(null)
  const [keep, setKeep] = useState(false)
  const [modal, setModal] = useState<'chat' | 'delete' | 'tables' | null>(null)
  const [err, setErr] = useState('')
  useCrumb(f?.name, '/formulas')

  useEffect(() => {
    setErr('')
    if (isNew) {
      const code = sp.get('code') || 'area = length * width'
      setF({ id: 0, name: sp.get('name') || 'My calculation', about: '', category: 'Mine', icon: 'fx', code, inputs: guessInputs(code), outputs: [{ label: 'Answer', expr: lastName(code) || 'area', unit: '', decimals: 2, main: true }], from_key: '', pinned: false, builtin: false, source: '', created_at: '', updated_at: '', runs: 0, last_run: null })
      return
    }
    const url = builtin ? `${V1}/library/${id}` : `${V1}/formulas/${id}`
    api.get<{ formula: Formula; runs: Run[] }>(url).then(r => { setF(r.formula); setRuns(r.runs); setValues({}) }).catch(e => setErr(errMsg(e)))
  }, [id, isNew, builtin, sp])

  const format = useMemo(() => numberFormat(settings), [settings])
  const res = useMemo(() => f ? runFormula(f.code, f.inputs, values, f.outputs, { units: true, deg: true, rates, format }) : null, [f, values, rates, format])

  if (err) return <Empty title="This calculation can't be opened" icon="alert">{err}</Empty>
  if (!f || !res) return <Loading />

  const save = async () => {
    try {
      const body = { name: f.name, about: f.about, category: f.category, icon: f.icon, code: f.code, inputs: f.inputs, outputs: f.outputs, pinned: f.pinned }
      const r = f.id ? await api.put<{ formula: Formula }>(`${V1}/formulas/${f.id}`, body) : await api.post<{ formula: Formula }>(`${V1}/formulas`, body)
      setF(r.formula); setEditing(false); announceChange(); toast('Saved', 'ok')
      if (!f.id) navigate(`/f/${r.formula.id}`, { replace: true })
    } catch (e) { toast(errMsg(e), 'err') }
  }
  const copyBuiltin = async () => {
    try { const r = await api.post<{ formula: Formula }>(`${V1}/library/${f.key}/copy`); announceChange(); navigate(`/f/${r.formula.id}`); setEditing(true) } catch (e) { toast(errMsg(e), 'err') }
  }
  const main = res.outputs.find(o => o.main && !o.error) || res.outputs.find(o => !o.error)
  const lines = [`${f.name}`, ...f.inputs.map(i => `${i.label || i.name}: ${values[i.name] || i.default}${i.unit && /^[\d.]+$/.test(values[i.name] || i.default) ? ' ' + i.unit : ''}`), ...res.outputs.filter(o => !o.error).map(o => `${o.label}: ${o.text}`)]
  const menu = (): MenuItem[] => [
    ...(builtin ? [{ label: 'Make my own copy to change', icon: 'copy', onSelect: copyBuiltin } as MenuItem] : [
      { label: 'Change it', icon: 'edit', onSelect: () => setEditing(true) } as MenuItem,
      { label: f.pinned ? 'Unpin' : 'Pin to the start page', icon: 'pin', onSelect: async () => { const r = await api.put<{ formula: Formula }>(`${V1}/formulas/${f.id}`, { pinned: !f.pinned }); setF(r.formula); announceChange() } } as MenuItem,
    ]),
    { label: 'Copy the answers', icon: 'copy', onSelect: async () => toast(await copyText(lines.join('\n')) ? 'Copied' : 'Could not copy', 'ok') },
    { label: 'Into a new notebook', icon: 'calc', onSelect: async () => {
      const body = [...f.inputs.map(i => { const raw = values[i.name] || i.default; const v = i.kind === 'percent' && !raw.endsWith('%') ? raw + '%' : i.kind === 'choice' ? choiceValueOf(i, raw) : i.unit && /^[\d.]+$/.test(raw) ? `${raw} ${i.unit}` : raw; return `${i.name} = ${v}` }), f.code, ...f.outputs.map(o => `${o.label}: ${o.unit ? `${o.expr} to ${o.unit}` : o.expr}`)].join('\n')
      try { const r = await api.post<{ notebook: { id: number } }>(`${V1}/notebooks`, { title: f.name, cells: [{ kind: 'calc', body }] }); navigate(`/n/${r.notebook.id}`) } catch (e) { toast(errMsg(e), 'err') }
    } },
    'sep',
    { label: 'Send to Pocket Chat…', icon: 'message', disabled: !appState('chat', 'chat:write').ok, hint: appState('chat', 'chat:write').ok ? undefined : 'not linked', onSelect: () => setModal('chat') },
    { label: 'Add as a row in Pocket Tables…', icon: 'table', disabled: !appState('tables', 'tables:write').ok, hint: appState('tables', 'tables:write').ok ? undefined : 'not linked', onSelect: () => setModal('tables') },
    { label: 'A flashcard in Pocket Learn', icon: 'book-open', disabled: !appState('learn', 'learn:write').ok, hint: appState('learn', 'learn:write').ok ? undefined : 'not linked', onSelect: async () => {
      const back = [f.about, '', ...f.code.split('\n')].join('\n')
      try { const r = await api.post<{ text: string }>(`${V1}/apps/learn`, { cards: [{ front: `How do you work out: ${f.name}?`, back, ref: `formula:${f.key || f.id}` }] }); toast(r.text, 'ok') } catch (e) { toast(errMsg(e), 'err') }
    } },
    ...(!builtin ? ['sep' as const, { label: 'Delete…', icon: 'trash', danger: true, onSelect: () => setModal('delete') } as MenuItem] : []),
  ]

  if (editing) return <FormulaEditor f={f} onChange={setF} onSave={save} onCancel={() => (f.id ? setEditing(false) : navigate('/formulas'))} />

  return (
    <div className="f-page">
      <div className="page-head">
        <span className="tile-ico lg"><Icon name={f.icon || 'fx'} size={18} /></span>
        <div className="grow"><h1 className="page-title">{f.name}</h1><p className="muted">{f.about}</p></div>
        <MenuButton label="More" className="btn icon-only" align="right" width={260} items={menu}><Icon name="more" size={18} /></MenuButton>
      </div>
      <div className="f-cols">
        <section className="card pad-card f-form">
          {f.inputs.map(i => {
            const [app, scope]: [string, string] | [] = i.from ? FROM_APP[i.from.split(':')[0]] || [] : []
            const st = app ? appState(app, scope) : null
            const appName = app ? APP_NAME[app] : ''
            const val = values[i.name] ?? ''
            return <div key={i.name} className="field">
              <span className="field-label">{i.label || i.name}</span>
              <div className="f-in-row">
                {i.kind === 'choice' ? <Select value={val || i.default} onChange={v => setValues({ ...values, [i.name]: v })} options={choiceLabels(i).map(l => ({ value: l, label: l }))} ariaLabel={i.label} />
                  : i.kind === 'date' ? <input type="date" value={val || (i.default === 'today' ? new Date().toISOString().slice(0, 10) : i.default)} onChange={e => setValues({ ...values, [i.name]: e.target.value })} />
                    : <div className="input-unit"><input value={val} placeholder={i.default} inputMode={/^[\d.]*$/.test(i.default) ? 'decimal' : undefined} onChange={e => setValues({ ...values, [i.name]: e.target.value })} spellCheck={false} />{(i.unit || i.kind === 'percent') && <span className="unit">{i.kind === 'percent' ? '%' : i.unit}</span>}</div>}
                {st && <button type="button" className="btn sm" disabled={!st.ok} onClick={() => setFill(i)} title={st.ok ? `Fill in from ${appName}` : `Link ${appName} in Settings → Connected apps`}><Icon name="download" size={13} />{appName.replace('Pocket ', '')}</button>}
              </div>
              {i.help && <span className="field-help">{i.help}</span>}
            </div>
          })}
          {f.inputs.length > 0 && Object.keys(values).length > 0 && <button type="button" className="linkish small" onClick={() => setValues({})}>Back to the examples</button>}
        </section>
        <section className="card pad-card f-answers">
          {res.error ? <p className="bad">{res.error}</p> : <>
            {main && <div className="f-main"><span className="muted small">{main.label}</span><span className="f-big">{main.text}</span></div>}
            <dl className="f-outs">{res.outputs.filter(o => o !== main).map((o, k) => <div key={k}><dt>{o.label}</dt><dd className={o.error ? 'bad small' : ''}>{o.error || o.text}</dd></div>)}</dl>
            <details className="f-working"><summary>How it is worked out</summary><pre>{f.code.split('\n').map((l, k) => `${l}${res.lines[k]?.text ? '   → ' + res.lines[k].text : ''}`).join('\n')}</pre></details>
            <button type="button" className="btn primary" onClick={() => setKeep(true)}><Icon name="check" size={15} />Keep this answer</button>
          </>}
        </section>
      </div>
      {runs.length > 0 && <section className="card">
        <div className="card-h sec-pad"><h3>Kept answers</h3></div>
        <ul className="list">{runs.map(r => <li key={r.id} className="list-row">
          <span className="grow"><b>{r.title || ago(r.created_at)}</b><span className="muted small block-line">{Object.entries(r.inputs).map(([k, v]) => `${f.inputs.find(i => i.name === k)?.label || k}: ${v}`).join(' · ')}</span></span>
          <span className="pill">{r.main}</span>
          <button type="button" className="btn sm ghost" onClick={() => setValues(r.inputs)} title="Fill the boxes in with these">Use again</button>
        </li>)}</ul>
      </section>}
      {fill && fill.from && <FillPicker source={fill.from} onClose={() => setFill(null)} onPick={v => setValues({ ...values, [fill.name]: v })} />}
      {keep && <KeepModal onClose={() => setKeep(false)} onKeep={async (title, note) => {
        try {
          const url = builtin ? `${V1}/library/${f.key}/run` : `${V1}/formulas/${f.id}/run`
          const r = await api.post<{ run: Run }>(url, { values: Object.fromEntries(f.inputs.map(i => [i.name, values[i.name] || i.default])), keep: true, title, note })
          setRuns([r.run, ...runs]); setKeep(false); announceChange(); toast('Kept', 'ok')
        } catch (e) { toast(errMsg(e), 'err') }
      }} />}
      {modal === 'chat' && <ChatSend title={f.name} lines={lines} onClose={() => setModal(null)} />}
      {modal === 'tables' && <AppPicker kind="tables-out" onClose={() => setModal(null)} onColumns={() => {}} columns={[...f.inputs.map(i => ({ name: i.label || i.name, values: [values[i.name] || i.default] })), ...res.outputs.map(o => ({ name: o.label, values: [o.text] }))]} />}
      {modal === 'delete' && <Confirm title="Delete this calculation?" message="Its kept answers stay (under Kept answers)." confirmLabel="Delete" onCancel={() => setModal(null)} onConfirm={async () => { await api.del(`${V1}/formulas/${f.id}`); announceChange(); navigate('/formulas') }} />}
    </div>
  )
}

function choiceValueOf(i: FInput, raw: string) {
  for (const o of (i.options || '').split('|')) { const [l, v] = o.split('='); if (l?.trim().toLowerCase() === raw.toLowerCase()) return (v || l).trim() }
  return raw
}

function KeepModal({ onClose, onKeep }: { onClose: () => void; onKeep: (title: string, note: string) => void }) {
  const [title, setTitle] = useState('')
  const [note, setNote] = useState('')
  return <Modal title="Keep this answer" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={() => onKeep(title, note)}>Keep</button></>}>
    <div className="stack">
      <label className="field"><span className="field-label">What is it for?</span><input autoFocus value={title} onChange={e => setTitle(e.target.value)} placeholder="Bedroom, the trip to Split, Bank B…" onKeyDown={e => { if (e.key === 'Enter') onKeep(title, note) }} /></label>
      <label className="field"><span className="field-label">A note</span><input value={note} onChange={e => setNote(e.target.value)} placeholder="Optional" /></label>
    </div>
  </Modal>
}

/** Names used in the working that are not set in it: they become boxes. */
function guessInputs(code: string): FInput[] {
  const env = newEnv({ units: true, deg: true })
  const set = new Set<string>(), used: string[] = []
  for (const l of code.split('\n')) {
    const m = /^\s*([A-Za-z_]\w*)\s*=/.exec(l)
    for (const w of l.replace(/^\s*[A-Za-z_]\w*\s*=/, '').match(/[A-Za-z_]\w*/g) || []) if (!set.has(w) && !used.includes(w)) used.push(w)
    if (m) set.add(m[1])
    runLine(env, l)
  }
  return used.filter(w => !set.has(w) && runLine(newEnv({ units: true, deg: true }), w).kind === 'error').slice(0, 12).map(w => ({ name: w, label: w.replace(/_/g, ' '), unit: '', default: '1' }))
}
function lastName(code: string) { const ls = code.split('\n').reverse(); for (const l of ls) { const m = /^\s*([A-Za-z_]\w*)\s*=/.exec(l); if (m) return m[1] } return '' }

function FormulaEditor({ f, onChange, onSave, onCancel }: { f: Formula; onChange: (f: Formula) => void; onSave: () => void; onCancel: () => void }) {
  const { toast } = useToast()
  const [ai, setAi] = useState('')
  const [busy, setBusy] = useState(false)
  const set = (p: Partial<Formula>) => onChange({ ...f, ...p })
  const setIn = (k: number, p: Partial<FInput>) => set({ inputs: f.inputs.map((x, j) => (j === k ? { ...x, ...p } : x)) })
  const setOut = (k: number, p: Partial<FOutput>) => set({ outputs: f.outputs.map((x, j) => (j === k ? { ...x, ...p } : x)) })
  const env = useMemo(() => {
    const e = newEnv({ units: true, deg: true })
    for (const i of f.inputs) runLine(e, `${i.name} = ${i.kind === 'percent' ? (i.default || '0') + '%' : i.unit && /^[\d.]+$/.test(i.default) ? `${i.default} ${i.unit}` : i.default || '1'}`)
    return e
  }, [f.inputs])
  const lines = useMemo(() => { const e = newEnv({ units: true, deg: true }); e.vars = new Map(env.vars); return f.code.split('\n').map(l => runLine(e, l)) }, [env, f.code])
  const draft = async () => {
    setBusy(true)
    try {
      const r = await api.post<{ formula: Partial<Formula> }>(`${V1}/ai/formula`, { text: ai })
      onChange({ ...f, ...r.formula, inputs: r.formula.inputs || [], outputs: (r.formula.outputs || []).map(o => ({ ...o, decimals: o.decimals ?? 2 })) })
    } catch (e) { toast(errMsg(e), 'err') }
    setBusy(false)
  }
  return (
    <div className="stack loose f-edit">
      <div className="page-head"><div className="grow"><h1 className="page-title">{f.id ? 'Change the calculation' : 'A calculation of my own'}</h1></div>
        <button type="button" className="btn" onClick={onCancel}>Cancel</button><button type="button" className="btn primary" onClick={onSave}>Save</button></div>
      {!f.id && <section className="card pad-card"><div className="row wrap"><input className="grow" value={ai} onChange={e => setAi(e.target.value)} placeholder="Or say what it should work out: “tiles for a bathroom with spare, in boxes”" />
        <button type="button" className="btn" disabled={!ai.trim() || busy} onClick={draft}><Icon name="sparkles" size={15} />{busy ? 'Drafting…' : 'Draft it with AI'}</button></div></section>}
      <section className="card pad-card stack">
        <div className="grid two"><label className="field"><span className="field-label">Name</span><input value={f.name} onChange={e => set({ name: e.target.value })} /></label>
          <label className="field"><span className="field-label">Group</span><input value={f.category} onChange={e => set({ category: e.target.value })} placeholder="Home, Money, School…" /></label></div>
        <label className="field"><span className="field-label">What it works out</span><input value={f.about} onChange={e => set({ about: e.target.value })} /></label>
      </section>
      <section className="card pad-card stack">
        <h3>Boxes to fill in</h3>
        <div className="scroll-x"><table className="dtable edit">
          <thead><tr><th>Name in the working</th><th>Label</th><th>Unit</th><th>Example</th><th>Kind</th><th>Choices (Label=value|…)</th><th /></tr></thead>
          <tbody>{f.inputs.map((i, k) => <tr key={k}>
            <td><input value={i.name} onChange={e => setIn(k, { name: e.target.value.replace(/\W/g, '_') })} /></td>
            <td><input value={i.label} onChange={e => setIn(k, { label: e.target.value })} /></td>
            <td><input value={i.unit} onChange={e => setIn(k, { unit: e.target.value })} placeholder="m², EUR" /></td>
            <td><input value={i.default} onChange={e => setIn(k, { default: e.target.value })} /></td>
            <td><Select value={i.kind || ''} onChange={v => setIn(k, { kind: v as FInput['kind'] })} options={[{ value: '', label: 'Number' }, { value: 'percent', label: 'Percent' }, { value: 'date', label: 'Date' }, { value: 'choice', label: 'Choice' }]} /></td>
            <td><input value={i.options || ''} onChange={e => setIn(k, { options: e.target.value })} disabled={i.kind !== 'choice'} /></td>
            <td><button type="button" className="btn ghost icon-only sm" onClick={() => set({ inputs: f.inputs.filter((_, j) => j !== k) })} aria-label="Remove"><Icon name="x" size={14} /></button></td>
          </tr>)}</tbody>
        </table></div>
        <button type="button" className="btn sm ghost" onClick={() => set({ inputs: [...f.inputs, { name: `value${f.inputs.length + 1}`, label: '', unit: '', default: '1' }] })}><Icon name="plus" size={14} />A box</button>
      </section>
      <section className="card pad-card stack">
        <h3>The working</h3>
        <p className="muted small">Calculator lines using the names above. The answers on the right use the examples.</p>
        <CalcCell cell={{ id: 0, notebook_id: 0, position: 0, kind: 'calc', title: '', body: f.code, data: {}, results: [], summary: '', collapsed: false, source: '', created_at: '', updated_at: '' }}
          onChange={p => set({ code: p.body ?? f.code })} env={env} lines={lines} format={{}} settings={null} deg />
      </section>
      <section className="card pad-card stack">
        <h3>Answers</h3>
        <div className="scroll-x"><table className="dtable edit">
          <thead><tr><th>Label</th><th>From</th><th>Shown in</th><th>Decimals</th><th>Big</th><th /></tr></thead>
          <tbody>{f.outputs.map((o, k) => <tr key={k}>
            <td><input value={o.label} onChange={e => setOut(k, { label: e.target.value })} /></td>
            <td><input value={o.expr} onChange={e => setOut(k, { expr: e.target.value })} placeholder="area" /></td>
            <td><input value={o.unit} onChange={e => setOut(k, { unit: e.target.value })} placeholder="m²" /></td>
            <td><input value={o.decimals ?? ''} onChange={e => setOut(k, { decimals: e.target.value === '' ? null : Math.max(0, Math.min(12, +e.target.value)) })} inputMode="numeric" /></td>
            <td><input type="radio" checked={o.main} onChange={() => set({ outputs: f.outputs.map((x, j) => ({ ...x, main: j === k })) })} aria-label="The big answer" /></td>
            <td><button type="button" className="btn ghost icon-only sm" onClick={() => set({ outputs: f.outputs.filter((_, j) => j !== k) })} aria-label="Remove"><Icon name="x" size={14} /></button></td>
          </tr>)}</tbody>
        </table></div>
        <button type="button" className="btn sm ghost" onClick={() => set({ outputs: [...f.outputs, { label: 'Answer', expr: '', unit: '', decimals: 2, main: !f.outputs.length }] })}><Icon name="plus" size={14} />An answer</button>
      </section>
    </div>
  )
}
