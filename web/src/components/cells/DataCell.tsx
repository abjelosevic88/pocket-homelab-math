// A table of numbers: averages and spread per column, a chart, and a trend
// line with how well it fits. Filled by hand, by pasting, or from another
// Pocket app (Tables, Files, Money, Home, Garage, Health, Wealth).
import { useMemo, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { useAppState } from '../../lib/data'
import { cellNumber, fit, histogram, isDateColumn, summary, type FitKind } from '../../lib/stats'
import type { Column } from '../../lib/types'
import { Bars, LineChart } from '../Chart'
import { Icon } from '../icons'
import { Plot } from '../Plot'
import { Select } from '../Select'
import { copyText, errMsg, Modal, MenuButton, Segmented, useIsMobile, useToast, type MenuItem } from '../ui'
import type { CellProps } from './common'
import { AppPicker, type PickKind } from '../AppPickers'

export interface DataData { columns: Column[]; chart?: 'line' | 'bar' | 'scatter' | 'histogram' | 'none'; x?: number; y?: number; fit?: FitKind; source?: string }

export function normData(d: Partial<DataData> | undefined): DataData {
  const columns = d?.columns?.length ? d.columns : [{ name: 'x', values: ['1', '2', '3', '4', '5'] }, { name: 'y', values: ['2.1', '3.9', '6.2', '7.8', '10.1'] }]
  return { columns, chart: d?.chart || 'line', x: d?.x ?? 0, y: d?.y ?? Math.min(1, columns.length - 1), fit: d?.fit || 'none', source: d?.source }
}

const fmt = (v: number) => (isFinite(v) ? String(Number(v.toPrecision(6))) : '—')

export function toCSV(cols: Column[]): string {
  const n = Math.max(0, ...cols.map(c => c.values.length))
  const q = (s: string) => (/[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s)
  const rows = [cols.map(c => q(c.name)).join(',')]
  for (let i = 0; i < n; i++) rows.push(cols.map(c => q(c.values[i] || '')).join(','))
  return rows.join('\n')
}

export function parseCSV(text: string): Column[] {
  const lines = text.replace(/\r/g, '').split('\n').filter(l => l.trim())
  if (!lines.length) return []
  const sep = lines[0].includes('\t') ? '\t' : (lines[0].split(';').length > lines[0].split(',').length ? ';' : ',')
  const split = (l: string) => {
    const out: string[] = []; let cur = '', q = false
    for (let i = 0; i < l.length; i++) {
      const c = l[i]
      if (q) { if (c === '"' && l[i + 1] === '"') { cur += '"'; i++ } else if (c === '"') q = false; else cur += c }
      else if (c === '"') q = true
      else if (c === sep) { out.push(cur); cur = '' }
      else cur += c
    }
    out.push(cur)
    return out.map(s => s.trim())
  }
  const grid = lines.map(split)
  const w = Math.max(...grid.map(r => r.length))
  const header = grid[0]
  const numericHeader = header.every(h => cellNumber(h) !== null)
  const body = numericHeader ? grid : grid.slice(1)
  return Array.from({ length: w }, (_, i) => ({ name: numericHeader ? `Column ${i + 1}` : header[i] || `Column ${i + 1}`, values: body.map(r => r[i] || '') }))
}

export function DataCell({ cell, onChange, readOnly }: CellProps) {
  const data = useMemo(() => normData(cell.data), [cell.data])
  const { toast } = useToast()
  const mobile = useIsMobile()
  const appState = useAppState()
  const [paste, setPaste] = useState<string | null>(null)
  const [picker, setPicker] = useState<PickKind | null>(null)
  const [showAll, setShowAll] = useState(false)
  const cols = data.columns
  const n = Math.max(0, ...cols.map(c => c.values.length))
  const set = (d: Partial<DataData>) => {
    const next = { ...data, ...d }
    onChange({ data: next, summary: next.columns.map(c => c.name).join(', ') })
  }
  const setCell = (ci: number, ri: number, v: string) => set({ columns: cols.map((c, i) => (i === ci ? { ...c, values: Object.assign([...c.values], { [ri]: v }) } : c)) })
  const setName = (ci: number, v: string) => set({ columns: cols.map((c, i) => (i === ci ? { ...c, name: v } : c)) })
  const addRow = () => set({ columns: cols.map(c => ({ ...c, values: [...c.values, ''] })) })
  const addCol = () => set({ columns: [...cols, { name: `Column ${cols.length + 1}`, values: Array(n).fill('') }] })
  const delCol = (ci: number) => set({ columns: cols.filter((_, i) => i !== ci), x: 0, y: Math.min(1, cols.length - 2) })
  const delRow = (ri: number) => set({ columns: cols.map(c => ({ ...c, values: c.values.filter((_, i) => i !== ri) })) })
  const replace = (columns: Column[], source?: string) => {
    if (!columns.length) return
    set({ columns, x: 0, y: Math.min(1, columns.length - 1), source })
  }

  const nums = cols.map(c => c.values.map(cellNumber))
  const numeric = cols.map((c, i) => nums[i].filter(v => v !== null).length >= Math.max(1, c.values.filter(Boolean).length * 0.8) && !isDateColumn(c.values))
  const stats = cols.map((_, i) => numeric[i] ? summary(nums[i].filter((v): v is number => v !== null)) : null)
  const xi = Math.min(data.x ?? 0, cols.length - 1), yi = Math.min(data.y ?? 1, cols.length - 1)
  const xNumeric = numeric[xi] || isDateColumn(cols[xi]?.values || [])
  const pairs: [number, number][] = []
  for (let r = 0; r < n; r++) { const x = nums[xi]?.[r], y = nums[yi]?.[r]; if (x !== null && x !== undefined && y !== null && y !== undefined) pairs.push([x, y]) }
  const fitted = xNumeric && data.fit && data.fit !== 'none' ? fit(pairs.map(p => p[0]), pairs.map(p => p[1]), data.fit) : null
  const chart = data.chart || 'line'

  const colOptions = cols.map((c, i) => ({ value: String(i), label: c.name }))
  const exportItems = (): MenuItem[] => [
    { label: 'Copy as CSV', icon: 'copy', onSelect: async () => toast(await copyText(toCSV(cols)) ? 'Copied the table' : 'Could not copy', 'ok') },
    { label: 'Save in Pocket Files (CSV)', icon: 'upload', disabled: !appState('files', 'files:write').ok, hint: appState('files', 'files:write').ok ? undefined : 'not linked', onSelect: async () => {
      try { const r = await api.post<{ text: string }>(`${V1}/apps/files`, { name: cols.map(c => c.name).slice(0, 2).join(' and '), kind: 'csv', text: toCSV(cols) }); toast(r.text, 'ok') } catch (e) { toast(errMsg(e), 'err') }
    } },
    { label: 'Add the rows to a Pocket Tables table…', icon: 'table', disabled: !appState('tables', 'tables:write').ok, hint: appState('tables', 'tables:write').ok ? undefined : 'not linked', onSelect: () => setPicker('tables-out') },
    { label: 'Make a chart slide in Pocket Slides', icon: 'presentation', disabled: !appState('slides', 'slides:write').ok || !numeric[yi], hint: appState('slides', 'slides:write').ok ? undefined : 'not linked', onSelect: async () => {
      const labels: string[] = [], values: number[] = []
      for (let r = 0; r < n; r++) { const y = nums[yi][r]; if (y !== null) { labels.push(cols[xi].values[r] || String(r + 1)); values.push(y) } }
      try { const r = await api.post<{ text: string; url?: string }>(`${V1}/apps/slides`, { title: cols[yi].name, kind: chart === 'bar' || chart === 'histogram' ? 'bar' : 'line', labels, values }); toast(r.text, 'ok', r.url ? { label: 'Open', run: () => window.open(r.url, '_blank', 'noopener') } : undefined) } catch (e) { toast(errMsg(e), 'err') }
    } },
  ]
  const importItems = (): MenuItem[] => [
    { label: 'Paste a table (from a spreadsheet)', icon: 'copy', onSelect: () => setPaste('') },
    'sep',
    { heading: 'From your Pocket apps' },
    ...([
      ['tables', 'tables:read', 'A table in Pocket Tables', 'table', 'tables'],
      ['files', 'files:read', 'A CSV file in Pocket Files', 'file', 'files'],
      ['money', 'transactions:read', 'Spending by month (Pocket Money)', 'wallet', 'money'],
      ['home', 'home:read', 'A meter by month (Pocket Home)', 'zap', 'home'],
      ['garage', 'vehicles:read', 'Fill-ups of a car (Pocket Garage)', 'fuel', 'garage'],
      ['health', 'health:read', 'Your weight (Pocket Health)', 'heart-pulse', 'health'],
      ['wealth', 'wealth:read', 'Net worth over time (Pocket Wealth)', 'chart', 'wealth'],
    ] as const).map(([slug, scope, label, icon, kind]) => {
      const st = appState(slug, scope)
      return { label, icon, disabled: !st.ok, hint: st.ok ? undefined : st.loading ? '…' : 'not linked', onSelect: () => setPicker(kind as PickKind) }
    }),
  ]

  const rowsShown = showAll ? n : Math.min(n, 12)
  return (
    <div className="data-cell">
      {!readOnly && <div className="data-bar">
        <MenuButton label="Bring in" className="btn sm" items={importItems}><Icon name="download" size={14} />Bring in</MenuButton>
        <MenuButton label="Send" className="btn sm" items={exportItems}><Icon name="share" size={14} />Send</MenuButton>
        <span className="spacer" />
        {data.source && <span className="pill">{data.source}</span>}
      </div>}
      <div className="data-grid scroll-x">
        <table className="dtable">
          <thead><tr>
            <th className="rn" />
            {cols.map((c, ci) => <th key={ci}>
              {readOnly ? c.name : <div className="th-in"><input value={c.name} onChange={e => setName(ci, e.target.value)} aria-label="Column name" />{cols.length > 1 && <button type="button" className="btn ghost icon-only xs" onClick={() => delCol(ci)} aria-label={`Remove ${c.name}`} title="Remove the column"><Icon name="x" size={12} /></button>}</div>}
            </th>)}
            {!readOnly && <th className="add"><button type="button" className="btn ghost icon-only xs" onClick={addCol} aria-label="Add a column" title="Add a column"><Icon name="plus" size={13} /></button></th>}
          </tr></thead>
          <tbody>
            {Array.from({ length: rowsShown }, (_, ri) => <tr key={ri}>
              <td className="rn">{readOnly ? ri + 1 : <button type="button" className="rn-btn" onClick={() => delRow(ri)} title="Remove this row" aria-label={`Remove row ${ri + 1}`}><span>{ri + 1}</span><Icon name="x" size={11} /></button>}</td>
              {cols.map((c, ci) => <td key={ci} className={numeric[ci] ? 'num' : ''}>{readOnly ? c.values[ri] : <input value={c.values[ri] || ''} onChange={e => setCell(ci, ri, e.target.value)} aria-label={`${c.name} row ${ri + 1}`} />}</td>)}
              {!readOnly && <td />}
            </tr>)}
          </tbody>
        </table>
      </div>
      <div className="data-more">
        {n > 12 && <button type="button" className="linkish small" onClick={() => setShowAll(!showAll)}>{showAll ? 'Show fewer rows' : `Show all ${n} rows`}</button>}
        {!readOnly && <button type="button" className="linkish small" onClick={addRow}>+ Add a row</button>}
      </div>

      <div className="data-stats">
        {cols.map((c, i) => stats[i] && <div key={i} className="dstat">
          <div className="dstat-h">{c.name}</div>
          <dl>
            <dt>Average</dt><dd>{fmt(stats[i]!.mean)}</dd>
            <dt>Middle</dt><dd>{fmt(stats[i]!.median)}</dd>
            <dt>Lowest</dt><dd>{fmt(stats[i]!.min)}</dd>
            <dt>Highest</dt><dd>{fmt(stats[i]!.max)}</dd>
            <dt>Spread (sd)</dt><dd>{fmt(stats[i]!.sd)}</dd>
            <dt>Total</dt><dd>{fmt(stats[i]!.sum)}</dd>
            <dt>Count</dt><dd>{stats[i]!.n}</dd>
          </dl>
        </div>)}
      </div>

      {cols.length > 0 && <div className="data-chart">
        <div className="data-chart-bar">
          <Segmented size="sm" value={chart} onChange={v => set({ chart: v })} options={[{ value: 'line', label: 'Line' }, { value: 'bar', label: 'Bars' }, { value: 'scatter', label: 'Dots' }, { value: 'histogram', label: 'How often' }, { value: 'none', label: 'None' }]} label="Chart" />
          {chart !== 'none' && chart !== 'histogram' && <label className="mini-sel">Across<Select value={String(xi)} onChange={v => set({ x: +v })} options={colOptions} ariaLabel="Across (x)" disabled={readOnly} /></label>}
          {chart !== 'none' && <label className="mini-sel">{chart === 'histogram' ? 'Of' : 'Up'}<Select value={String(yi)} onChange={v => set({ y: +v })} options={colOptions} ariaLabel="Up (y)" disabled={readOnly} /></label>}
          {(chart === 'scatter' || chart === 'line') && xNumeric && <label className="mini-sel">Trend<Select value={data.fit || 'none'} onChange={v => set({ fit: v as FitKind })} disabled={readOnly} ariaLabel="Trend line"
            options={[{ value: 'none', label: 'None' }, { value: 'linear', label: 'Straight line' }, { value: 'quadratic', label: 'Curve (x²)' }, { value: 'exponential', label: 'Growing (eˣ)' }]} /></label>}
        </div>
        {chart !== 'none' && !numeric[yi] && <p className="muted small">Pick a column of numbers to draw.</p>}
        {chart !== 'none' && numeric[yi] && <ChartView kind={chart} cols={cols} nums={nums} xi={xi} yi={yi} pairs={pairs} fitted={fitted} xNumeric={xNumeric} mobile={mobile} />}
        {fitted && <p className="data-fit"><b>{fitted.text}</b> <span className="muted">· fits {Math.round(fitted.r2 * 100)}% of the ups and downs (R² = {fmt(fitted.r2)})</span></p>}
      </div>}

      {paste !== null && <Modal title="Paste a table" onClose={() => setPaste(null)} footer={<>
        <button type="button" className="btn" onClick={() => setPaste(null)}>Cancel</button>
        <button type="button" className="btn primary" disabled={!paste.trim()} onClick={() => { replace(parseCSV(paste)); setPaste(null) }}>Use it</button>
      </>}>
        <p className="muted small">Copy cells from a spreadsheet (or CSV text) and paste them here. The first row is the column names.</p>
        <textarea className="paste-in" autoFocus value={paste} onChange={e => setPaste(e.target.value)} rows={10} placeholder={'Month, Spent\nJan, 820\nFeb, 760'} />
      </Modal>}
      {picker && <AppPicker kind={picker} columns={cols} onClose={() => setPicker(null)} onColumns={(c, src) => { replace(c, src); setPicker(null) }} />}
    </div>
  )
}

function ChartView({ kind, cols, nums, xi, yi, pairs, fitted, xNumeric, mobile }: {
  kind: string; cols: Column[]; nums: (number | null)[][]; xi: number; yi: number; pairs: [number, number][]; fitted: ReturnType<typeof fit>; xNumeric: boolean; mobile: boolean
}) {
  const h = mobile ? 220 : 260
  if (kind === 'histogram') {
    const hs = histogram(nums[yi].filter((v): v is number => v !== null))
    return <Bars height={h - 40} items={hs.map(b => ({ label: `${fmt(b.from)}–${fmt(b.to)}`, value: b.count, hint: `${b.count} values` }))} />
  }
  if (kind === 'scatter' || (fitted && xNumeric)) {
    if (!pairs.length) return null
    const xs = pairs.map(p => p[0]), ys = pairs.map(p => p[1])
    const pad = (a: number, b: number): [number, number] => { const s = b - a || Math.abs(a) || 1; return [a - s * 0.08, b + s * 0.12] }
    const range = { x: pad(Math.min(...xs), Math.max(...xs)), y: pad(Math.min(...ys), Math.max(...ys)) }
    const curves = [
      { id: 'pts', kind: 'points' as const, color: '#3554a5', pts: pairs },
      ...(kind === 'line' ? [] : []),
      ...(fitted ? [{ id: 'fit', kind: 'fx' as const, color: '#c2410c', f: fitted.f, dashed: true }] : []),
    ]
    return <Plot curves={curves} range={range} height={h} />
  }
  const points = cols[xi].values.map((x, r) => ({ x: x || String(r + 1), y: nums[yi][r] }))
  if (kind === 'bar') return <LineChart points={points} kind="bar" height={h} zero />
  return <LineChart points={points} height={h} />
}
