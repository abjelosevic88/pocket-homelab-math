// What every cell gets, and small shared pieces.
import { useEffect, useRef } from 'react'
import type { Env, Format, Line } from '../../lib/calc'
import type { Cell, Settings } from '../../lib/types'

export interface CellProps {
  cell: Cell
  /** a change to the cell: saved a moment later */
  onChange: (patch: Partial<Cell>) => void
  /** what the calculations above this cell set (variables and functions) */
  env: Env
  /** this cell's answers, worked out in the browser (calc cells) */
  lines?: Line[]
  format: Format
  settings: Settings | null
  readOnly?: boolean
  notebookId?: number
  deg: boolean
}

export function numberFormat(s: Settings | null): Format {
  const f: Format = { digits: s?.digits || 10, group: ',', point: '.' }
  switch (s?.number_fmt) {
    case '1.234,5': f.group = '.'; f.point = ','; break
    case '1 234,5': f.group = ' '; f.point = ','; break
    case '1234.5': f.group = ''; break
  }
  return f
}

/** A textarea that grows with its text. */
export function AutoText({ value, onChange, className = '', placeholder, onKeyDown, autoFocus, readOnly, spellCheck = false, minRows = 1, onFocus, onBlur, taRef }: {
  value: string; onChange: (v: string) => void; className?: string; placeholder?: string; onKeyDown?: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void
  autoFocus?: boolean; readOnly?: boolean; spellCheck?: boolean; minRows?: number; onFocus?: () => void; onBlur?: () => void; taRef?: React.MutableRefObject<HTMLTextAreaElement | null>
}) {
  const ref = useRef<HTMLTextAreaElement | null>(null)
  useEffect(() => {
    const t = ref.current
    if (!t) return
    t.style.height = 'auto'
    t.style.height = t.scrollHeight + 'px'
  }, [value])
  return <textarea ref={el => { ref.current = el; if (taRef) taRef.current = el }} className={className} value={value} rows={minRows} placeholder={placeholder} spellCheck={spellCheck} autoFocus={autoFocus} readOnly={readOnly}
    onChange={e => onChange(e.target.value)} onKeyDown={onKeyDown} onFocus={onFocus} onBlur={onBlur} autoCapitalize="off" autoCorrect="off" />
}

export const KIND_INFO: Record<string, { name: string; icon: string; about: string }> = {
  calc: { name: 'Calculation', icon: 'calc', about: 'Sums line by line, with units, money and percentages' },
  text: { name: 'Text', icon: 'text', about: 'Words, headings and $maths$' },
  graph: { name: 'Graph', icon: 'graph', about: 'Functions, points and sliders' },
  geometry: { name: 'Geometry', icon: 'triangle', about: 'Points, lines and circles you can drag' },
  data: { name: 'Table of data', icon: 'table', about: 'Numbers with averages, a trend line and a chart' },
  solve: { name: 'Equations', icon: 'equals', about: 'Solve an equation or a few at once; a table of values; an area' },
}
