// Units and money, converted both ways as you type.
import { useMemo, useState } from 'react'
import { useApp } from '../lib/app-context'
import { useData } from '../lib/data'
import { calculate, converterKinds } from '../lib/calc'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { numberFormat } from '../components/cells/common'
import { copyText, Segmented, useToast } from '../components/ui'

const KIND_NAMES: Record<string, string> = { length: 'Length', mass: 'Weight', temperature: 'Temperature', area: 'Area', volume: 'Volume', speed: 'Speed', time: 'Time', energy: 'Energy', power: 'Power', pressure: 'Pressure', data: 'Data', angle: 'Angle', fuel: 'Fuel use', money: 'Money' }

export default function Convert() {
  const { settings } = useApp()
  const { rates, ratesInfo } = useData()
  const { toast } = useToast()
  const [kind, setKind] = useState('length')
  const kinds = [...converterKinds, { kind: 'money', units: Object.keys(rates).sort() }]
  const units = kinds.find(k => k.kind === kind)!.units
  const [pick, setPick] = useState<Record<string, [string, string]>>({ money: [settings?.currency || 'EUR', settings?.currency === 'EUR' ? 'USD' : 'EUR'] })
  const [from, to] = pick[kind] || [units[0], units[Math.min(2, units.length - 1)]]
  const [val, setVal] = useState('1')
  const format = useMemo(() => numberFormat(settings), [settings])
  const opts = { units: true, deg: true, rates, format }
  const conv = (v: string, a: string, b: string) => v.trim() ? calculate(`${v} (${a === 'L/100km' ? 'L/(100 km)' : a}) to ${b}`, opts) : null
  const res = conv(val, from, to)
  const all = units.filter(u => u !== from).map(u => ({ u, r: conv(val, from, u) }))
  const setPair = (a: string, b: string) => setPick({ ...pick, [kind]: [a, b] })
  return (
    <div className="stack loose conv">
      <div className="page-head"><div className="grow"><h1 className="page-title">Converter</h1><p className="muted">Or write it in any notebook: <code>60 mph to km/h</code>, <code>100 EUR in BAM</code>.</p></div></div>
      <div className="scroll-x"><Segmented value={kind} onChange={setKind} options={kinds.map(k => ({ value: k.kind, label: KIND_NAMES[k.kind] || k.kind }))} label="What" /></div>
      <section className="card pad-card conv-box">
        <div className="conv-row">
          <input className="conv-in" value={val} onChange={e => setVal(e.target.value)} inputMode="decimal" aria-label="Amount" />
          <Select value={from} onChange={v => setPair(v, to)} options={units.map(u => ({ value: u, label: u }))} ariaLabel="From" />
          <button type="button" className="btn icon-only" onClick={() => setPair(to, from)} aria-label="Swap" title="Swap"><Icon name="convert" size={16} /></button>
          <Select value={to} onChange={v => setPair(from, v)} options={units.map(u => ({ value: u, label: u }))} ariaLabel="To" />
        </div>
        <button type="button" className={`conv-out ${res?.kind === 'error' ? 'bad' : ''}`} onClick={async () => res?.text && toast(await copyText(res.text) ? `Copied ${res.text}` : 'Could not copy', 'ok')}>{res?.kind === 'error' ? res.error : res?.text || '—'}</button>
        {kind === 'money' && <p className="muted small">{ratesInfo?.source ? `Rates of the ${ratesInfo.source}, ${ratesInfo.date}.` : ratesInfo?.note}</p>}
      </section>
      <section className="card"><ul className="list">{all.map(({ u, r }) => <li key={u}><button type="button" className="list-row row-link wide" onClick={() => setPair(from, u)}><span className="grow">{u}</span><span className={r?.kind === 'error' ? 'muted small' : 'conv-all'}>{r?.kind === 'error' ? '—' : r?.text}</span></button></li>)}</ul></section>
    </div>
  )
}
