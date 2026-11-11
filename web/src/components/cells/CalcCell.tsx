// Sums written line by line; each line's answer beside it.
import { useRef, useState } from 'react'
import { copyText, useToast } from '../ui'
import { AutoText, type CellProps } from './common'

export function CalcCell({ cell, onChange, lines = [], readOnly }: CellProps) {
  const { toast } = useToast()
  const ta = useRef<HTMLTextAreaElement | null>(null)
  const [focus, setFocus] = useState(-1)
  const rows = cell.body.split('\n')
  const caret = () => {
    const t = ta.current
    if (!t) return -1
    return t.value.slice(0, t.selectionStart).split('\n').length - 1
  }
  return (
    <div className={`calc ${readOnly ? 'ro' : ''}`}>
      <AutoText className="calc-in" value={cell.body} readOnly={readOnly} placeholder={'Write a sum on each line, like\n5 km + 300 m\n100 EUR in BAM\n20% off 80'} minRows={3}
        onChange={v => onChange({ body: v })} taRef={ta}
        onFocus={() => setFocus(caret())} onBlur={() => setFocus(-1)}
        onKeyDown={() => requestAnimationFrame(() => setFocus(caret()))} />
      <div className="calc-out" aria-label="Answers">
        {rows.map((_, i) => {
          const ln = lines[i]
          if (!ln || ln.kind === 'empty' || ln.kind === 'text') return <div key={i} className="calc-r" />
          if (ln.kind === 'func') return <div key={i} className="calc-r fn" title="A function: use it on the lines below">{ln.name}(…)</div>
          if (ln.kind === 'error') return <div key={i} className={`calc-r err ${focus === i ? 'show' : ''}`} title={ln.error}>{focus === i ? ln.error : '?'}</div>
          return (
            <button key={i} type="button" className={`calc-r val ${ln.kind === 'set' ? 'set' : ''}`} title={(ln.alt ? ln.alt + ' · ' : '') + 'Copy'}
              onMouseDown={e => e.preventDefault()}
              onClick={async () => toast(await copyText(ln.text || '') ? `Copied ${ln.text}` : 'Could not copy', 'ok')}>
              {ln.name && <span className="calc-name">{ln.name} =</span>}
              <span className="calc-v">{ln.text}</span>
              {ln.alt && <span className="calc-alt">{ln.alt}</span>}
            </button>
          )
        })}
      </div>
    </div>
  )
}
