// Maths written the way it is printed: fractions stacked, powers raised,
// roots with their sign. From the calculator's parse of a line.
import React from 'react'
import { Env, parseLine, unitText, type Node } from './calc'

const OPS: Record<string, string> = { '+': '+', '-': '−', '*': '×', eq: '=', lt: '<', gt: '>', le: '≤', ge: '≥', ne: '≠', mod: 'mod', of: 'of', off: 'off', on: 'on' }
const prec: Record<string, number> = { eq: 0, lt: 0, gt: 0, le: 0, ge: 0, ne: 0, conv: 1, '+': 2, '-': 2, '*': 3, '/': 3, imp: 3, mod: 3, of: 3, off: 3, on: 3, neg: 4, '^': 5 }
const GREEK: Record<string, string> = { alpha: 'α', beta: 'β', gamma: 'γ', delta: 'δ', theta: 'θ', lambda: 'λ', mu: 'μ', sigma: 'σ', omega: 'ω', pi: 'π', phi: 'φ', tau: 'τ' }

function num(n: number) { return String(n).replace('-', '−') }

function P({ children, on }: { children: React.ReactNode; on: boolean }) {
  return on ? <span className="m-par">(<span>{children}</span>)</span> : <>{children}</>
}

export function Pretty({ node }: { node: Node }): React.ReactElement {
  return <span className="m">{render(node, 0)}</span>
}

function render(n: Node, outer: number): React.ReactNode {
  const p = prec[n.op] ?? 9
  const wrap = (x: React.ReactNode) => <P on={p < outer}>{x}</P>
  const k = n.kids || []
  switch (n.op) {
    case 'num': return num(n.num!)
    case 'qty': return <>{num(n.num!)}{n.scale ? '' : ' '}<span className="m-unit">{n.scale ? ` ${unitText(n.units!.slice(0, -1), 2)}/${n.scale}${unitText([{ name: n.units![n.units!.length - 1].name, pow: 1 }], 1)}` : unitText(n.units!, n.num!)}</span></>
    case 'unit': return <span className="m-unit">{unitText([{ name: n.name!, pow: 1 }], 1)}</span>
    case 'id': return <i className="m-var">{GREEK[n.name!] || n.name}</i>
    case 'date': return <span className="m-unit">{n.name}</span>
    case 'paren': return <span className="m-par">(<span>{render(k[0], 0)}</span>)</span>
    case 'neg': return wrap(<>−{render(k[0], 4)}</>)
    case 'pct': return <>{render(k[0], 9)}%</>
    case '!': return <>{render(k[0], 9)}!</>
    case 'cur': return <>{render(k[0], 9)} <span className="m-unit">{n.name}</span></>
    case '/': return <span className="m-frac"><span className="m-num">{render(k[0], 0)}</span><span className="m-den">{render(k[1], 0)}</span></span>
    case '^': return <>{render(k[0], 6)}<sup className="m-sup">{render(k[1], 0)}</sup></>
    case 'imp': return wrap(<>{render(k[0], 3)}<span className="m-thin" />{render(k[1], 3)}</>)
    case 'conv': return wrap(<>{render(k[0], 1)} <span className="m-op">→</span> {n.name ? n.name : render(k[1], 2)}</>)
    case 'tuple': return <>({k.map((x, i) => <React.Fragment key={i}>{i ? ', ' : ''}{render(x, 0)}</React.Fragment>)})</>
    case 'list': return <>[{k.map((x, i) => <React.Fragment key={i}>{i ? ', ' : ''}{render(x, 0)}</React.Fragment>)}]</>
    case 'call': {
      const name = n.name!
      if (name === 'sqrt' && k.length === 1) return <span className="m-root"><span className="m-rad">√</span><span className="m-under">{render(k[0], 0)}</span></span>
      if (name === 'abs' && k.length === 1) return <>|{render(k[0], 0)}|</>
      const fname = <span className="m-fn">{GREEK[name] || name}{"'".repeat(n.num || 0)}</span>
      return <>{fname}<span className="m-par">(<span>{k.map((x, i) => <React.Fragment key={i}>{i ? ', ' : ''}{render(x, 0)}</React.Fragment>)}</span>)</span></>
    }
  }
  if (OPS[n.op] && k.length === 2) {
    const right = n.op === '-' ? p + 1 : p
    return wrap(<>{render(k[0], p)} <span className="m-op">{OPS[n.op]}</span> {render(k[1], right)}</>)
  }
  return '?'
}

/** A line of text as pretty maths, or null when it does not read. */
export function PrettyLine({ text, env }: { text: string; env?: Env }) {
  const e = env || new Env(false, false)
  try {
    const st = parseLine(text, e)
    if (st.kind === 'empty' || !st.expr) return null
    if (st.kind === 'assign') return <span className="m"><i className="m-var">{GREEK[st.name!] || st.name}</i> <span className="m-op">=</span> {render(st.expr, 0)}</span>
    if (st.kind === 'func') return <span className="m"><span className="m-fn">{st.name}</span>({st.params!.map(p => GREEK[p] || p).join(', ')}) <span className="m-op">=</span> {render(st.expr, 0)}</span>
    return <Pretty node={st.expr} />
  } catch { return null }
}
