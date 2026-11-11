// Words: headings, lists, **bold**, *italic*, `code`, links and $maths$.
import React, { useState } from 'react'
import { PrettyLine } from '../../lib/pretty'
import { AutoText, type CellProps } from './common'

function inline(text: string, key = ''): React.ReactNode[] {
  const out: React.ReactNode[] = []
  const re = /(\$[^$\n]+\$|\*\*[^*]+\*\*|\*[^*\s][^*]*\*|`[^`]+`|\[[^\]]+\]\(https?:\/\/[^)\s]+\))/g
  let last = 0, m: RegExpExecArray | null, i = 0
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index))
    const t = m[0], k = `${key}-${i++}`
    if (t.startsWith('$')) out.push(<span key={k} className="md-math"><PrettyLine text={t.slice(1, -1)} /></span>)
    else if (t.startsWith('**')) out.push(<b key={k}>{t.slice(2, -2)}</b>)
    else if (t.startsWith('*')) out.push(<i key={k}>{t.slice(1, -1)}</i>)
    else if (t.startsWith('`')) out.push(<code key={k}>{t.slice(1, -1)}</code>)
    else { const mm = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(t)!; out.push(<a key={k} href={mm[2]} target="_blank" rel="noopener noreferrer">{mm[1]}</a>) }
    last = m.index + t.length
  }
  if (last < text.length) out.push(text.slice(last))
  return out
}

export function Markdown({ text }: { text: string }) {
  const blocks: React.ReactNode[] = []
  const lines = text.split('\n')
  let list: { ordered: boolean; items: string[] } | null = null
  let para: string[] = []
  const flushPara = () => { if (para.length) { blocks.push(<p key={'p' + blocks.length}>{inline(para.join(' '), 'p' + blocks.length)}</p>); para = [] } }
  const flushList = () => {
    if (!list) return
    const items = list.items.map((it, i) => <li key={i}>{inline(it, 'l' + i)}</li>)
    blocks.push(list.ordered ? <ol key={'o' + blocks.length}>{items}</ol> : <ul key={'u' + blocks.length}>{items}</ul>)
    list = null
  }
  for (const l of lines) {
    const h = /^(#{1,3})\s+(.*)$/.exec(l)
    const li = /^\s*([-*•]|\d+[.)])\s+(.*)$/.exec(l)
    if (h) { flushPara(); flushList(); const n = h[1].length; const C = (`h${n + 1}`) as 'h2' | 'h3' | 'h4'; blocks.push(<C key={'h' + blocks.length}>{inline(h[2], 'h' + blocks.length)}</C>); continue }
    if (li) {
      flushPara()
      const ordered = /\d/.test(li[1])
      if (!list || list.ordered !== ordered) { flushList(); list = { ordered, items: [] } }
      list.items.push(li[2]); continue
    }
    if (!l.trim()) { flushPara(); flushList(); continue }
    flushList()
    para.push(l)
  }
  flushPara(); flushList()
  return <div className="md">{blocks}</div>
}

export function TextCell({ cell, onChange, readOnly }: CellProps) {
  const [edit, setEdit] = useState(!cell.body)
  if (readOnly || (!edit && cell.body)) {
    return (
      <div className={`text-cell ${readOnly ? '' : 'editable'}`} onClick={() => !readOnly && setEdit(true)} title={readOnly ? undefined : 'Click to change'}>
        <Markdown text={cell.body} />
      </div>
    )
  }
  return (
    <div className="text-cell editing">
      <AutoText className="text-in" value={cell.body} autoFocus={!!cell.body} spellCheck placeholder="Write here. # for a heading, - for a list, **bold**, $x^2 + 1$ for maths." minRows={3}
        onChange={v => onChange({ body: v })} onBlur={() => { if (cell.body.trim()) setEdit(false) }} />
    </div>
  )
}
