// Practice: short rounds of sums for a child (or yourself), made up fresh
// each time; how it goes day by day.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useAppState } from '../lib/data'
import { ago } from '../lib/format'
import { isRight, LEVELS, question, seeded, SKILLS, skillName, type Q } from '../lib/practice'
import type { LearnerStats, Practice as Round, PracticeQ, Settings } from '../lib/types'
import { LineChart, Ring } from '../components/Chart'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { errMsg, Loading, Modal, Segmented, useToast } from '../components/ui'

export default function Practice() {
  const { settings, setSettings } = useApp()
  const { toast } = useToast()
  const appState = useAppState()
  const [sp, setSp] = useSearchParams()
  const learner = sp.get('learner') || ''
  const [data, setData] = useState<{ rounds: Round[]; learners: LearnerStats[]; goal: number; round_size: number } | null>(null)
  const [skill, setSkill] = useState('tables')
  const [level, setLevel] = useState(2)
  const [table, setTable] = useState('0')
  const [drill, setDrill] = useState(false)
  const [addKid, setAddKid] = useState(false)
  const load = useCallback(() => { api.get<typeof data>(`${V1}/practice?learner=${encodeURIComponent(learner)}&limit=30`).then(setData).catch(e => toast(errMsg(e), 'err')) }, [learner, toast])
  useEffect(() => { load() }, [load])
  if (!data || !settings) return <Loading />
  const me = data.learners.find(l => l.learner === learner) || data.learners[0]
  const saveLearners = async (list: string[]) => {
    try { setSettings(await api.put<Settings>(`${V1}/settings`, { learners: list })); load() } catch (e) { toast(errMsg(e), 'err') }
  }
  const goal = data.goal
  return (
    <div className="stack loose practice">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Practice</h1><p className="muted">Ten quick questions at a time. Little and often works best.</p></div>
      </div>
      <div className="row wrap learner-row">
        <Segmented value={learner} onChange={v => { const n = new URLSearchParams(sp); if (v) n.set('learner', v); else n.delete('learner'); setSp(n) }} options={data.learners.map(l => ({ value: l.learner, label: l.learner || 'You' }))} label="Who" />
        <button type="button" className="btn sm ghost" onClick={() => setAddKid(true)}><Icon name="plus" size={14} />Someone else</button>
      </div>

      <div className="practice-cols">
        <section className="card pad-card start-round">
          <h3>A new round{learner ? ` for ${learner}` : ''}</h3>
          <div className="skill-grid">{SKILLS.map(s => <button key={s.id} type="button" className={`skill ${skill === s.id ? 'on' : ''}`} onClick={() => setSkill(s.id)} aria-pressed={skill === s.id}>
            <Icon name={s.icon} size={16} /><b>{s.name}</b><span className="muted small">{s.about}</span>
          </button>)}</div>
          <div className="field"><span className="field-label">Level</span><div className="scroll-x"><Segmented size="sm" value={level} onChange={setLevel} options={LEVELS.map(l => ({ value: l.n, label: l.name, title: l.about }))} label="Level" /></div><span className="field-help">{LEVELS[level - 1].about}</span></div>
          {skill === 'tables' && <div className="field"><span className="field-label">Which table</span><Select value={table} onChange={setTable} options={[{ value: '0', label: 'All of them, mixed' }, ...Array.from({ length: 11 }, (_, i) => ({ value: String(i + 2), label: `The ${i + 2} times table` }))]} /></div>}
          <button type="button" className="btn primary lg" onClick={() => setDrill(true)}><Icon name="play" size={16} />Start ({data.round_size} questions)</button>
        </section>

        <section className="card pad-card today-card">
          <div className="today-top">
            <Ring pct={goal ? Math.min(100, (me.today / goal) * 100) : 0} size={84} stroke={8}><div className="ring-txt"><b>{me.today}</b><span className="muted small">{goal ? `of ${goal}` : 'today'}</span></div></Ring>
            <div className="grow stack tight">
              <b>{goal && me.today >= goal ? 'Done for today' : goal ? `${goal - me.today} to go today` : 'Today'}</b>
              <span className="muted small">{me.streak > 0 ? `${me.streak} ${me.streak === 1 ? 'day' : 'days'} in a row` : 'No streak yet'}</span>
              <span className="muted small">{me.questions ? `${Math.round((me.correct / me.questions) * 100)}% right over ${me.rounds} rounds` : 'No rounds yet'}</span>
            </div>
          </div>
          {me.days && <LineChart points={me.days.map(d => ({ x: d.day, y: d.questions }))} kind="bar" height={130} goal={goal || undefined} goalLabel="goal" zero />}
          {me.skills && me.skills.length > 0 && <ul className="skill-stats">{me.skills.map(s => <li key={s.skill}><span className="grow">{skillName(s.skill)}</span><span className="bar-mini"><span style={{ width: `${s.pct}%` }} /></span><span className="small nowrap">{s.pct}% · {s.seconds_each.toFixed(1)} s</span></li>)}</ul>}
          {!learner && settings.practice_habit > 0 && <p className="muted small"><Icon name="habit" size={13} /> Reaching the goal ticks your habit in Pocket Habits.</p>}
        </section>
      </div>

      {data.rounds.length > 0 && <section className="card">
        <div className="card-h sec-pad"><h3>Rounds</h3></div>
        <ul className="list">{data.rounds.slice(0, 20).map(r => <RoundRow key={r.id} r={r} onDelete={async () => { await api.del(`${V1}/practice/${r.id}`); load(); announceChange() }} />)}</ul>
      </section>}

      {drill && <Drill skill={skill} level={level} table={+table} n={data.round_size} learner={learner} onClose={done => { setDrill(false); if (done) { load(); announceChange() } }} />}
      {addKid && <AddLearner current={settings.learners} familyOK={appState('family', 'family:read').ok} onClose={() => setAddKid(false)} onSave={list => { saveLearners(list); setAddKid(false) }} />}
    </div>
  )
}

function RoundRow({ r, onDelete }: { r: Round; onDelete: () => void }) {
  const [open, setOpen] = useState(false)
  const [full, setFull] = useState<Round | null>(null)
  useEffect(() => { if (open && !full) api.get<{ round: Round }>(`${V1}/practice/${r.id}`).then(x => setFull(x.round)).catch(() => {}) }, [open, full, r.id])
  return <li>
    <div className="list-row">
      <button type="button" className="linkish grow left" onClick={() => setOpen(!open)}><b>{skillName(r.skill)}</b> <span className="muted small">level {r.level} · {ago(r.created_at)}{r.learner ? ` · ${r.learner}` : ''}</span></button>
      <span className={`pill ${r.correct === r.questions ? 'ok' : ''}`}>{r.correct}/{r.questions}</span>
      <span className="muted small nowrap">{Math.round(r.seconds / Math.max(1, r.questions))} s each</span>
      <button type="button" className="btn ghost icon-only sm" onClick={onDelete} aria-label="Remove this round" title="Remove"><Icon name="trash" size={13} /></button>
    </div>
    {open && full && <div className="round-qs">{(full.details as PracticeQ[]).map((q, i) => <span key={i} className={`rq ${q.right ? 'ok' : 'bad'}`} title={`${q.seconds} s`}>{q.q} = {q.given}{q.right ? '' : ` (${q.answer})`}</span>)}</div>}
  </li>
}

function AddLearner({ current, familyOK, onClose, onSave }: { current: string[]; familyOK: boolean; onClose: () => void; onSave: (l: string[]) => void }) {
  const [list, setList] = useState(current)
  const [name, setName] = useState('')
  const [kids, setKids] = useState<{ name: string; emoji: string }[] | null>(null)
  useEffect(() => { if (familyOK) api.get<{ children: { name: string; emoji: string }[] }>(`${V1}/apps/family`).then(r => setKids(r.children)).catch(() => setKids([])) }, [familyOK])
  const add = (n: string) => { n = n.trim(); if (n && !list.some(x => x.toLowerCase() === n.toLowerCase())) setList([...list, n]) }
  return <Modal title="Who practises here" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={() => onSave(list)}>Save</button></>}>
    <div className="stack">
      <p className="muted small">Each has their own rounds and progress, on your account.</p>
      <div className="chips">{list.map(l => <span key={l} className="chip">{l}<button type="button" className="chip-x" onClick={() => setList(list.filter(x => x !== l))} aria-label={`Remove ${l}`}><Icon name="x" size={11} /></button></span>)}</div>
      <div className="row"><input className="grow" value={name} onChange={e => setName(e.target.value)} placeholder="A name" onKeyDown={e => { if (e.key === 'Enter') { add(name); setName('') } }} /><button type="button" className="btn" onClick={() => { add(name); setName('') }}>Add</button></div>
      {familyOK ? (kids === null ? <Loading /> : kids.length > 0 && <><p className="muted small">From Pocket Family:</p><div className="chips">{kids.map(k => <button key={k.name} type="button" className="chip-btn" onClick={() => add(k.name)}>{k.emoji} {k.name}</button>)}</div></>)
        : <p className="muted small">Link Pocket Family (Settings → Connected apps) to pick your children from there.</p>}
    </div>
  </Modal>
}

function Drill({ skill, level, table, n, learner, onClose }: { skill: string; level: number; table: number; n: number; learner: string; onClose: (done: boolean) => void }) {
  const { toast } = useToast()
  const qs = useMemo<Q[]>(() => { const rnd = seeded(Date.now()); return Array.from({ length: n }, () => question(skill, level, rnd, table)) }, [skill, level, table, n])
  const [i, setI] = useState(0)
  const [given, setGiven] = useState('')
  const [answers, setAnswers] = useState<PracticeQ[]>([])
  const [flash, setFlash] = useState<null | boolean>(null)
  const [saved, setSaved] = useState<{ goal_met?: boolean; habit?: string } | null>(null)
  const started = useRef(Date.now()), qStart = useRef(Date.now())
  const input = useRef<HTMLInputElement>(null)
  const done = answers.length === n
  useEffect(() => { input.current?.focus() }, [i])
  const submit = () => {
    if (flash !== null || !given.trim()) return
    const q = qs[i]
    const right = isRight(q, given)
    const a: PracticeQ = { q: q.q, answer: q.answer, given: given.trim(), right, seconds: Math.round((Date.now() - qStart.current) / 100) / 10 }
    const next = [...answers, a]
    setAnswers(next); setFlash(right)
    setTimeout(() => {
      setFlash(null); setGiven('')
      if (next.length < n) { setI(i + 1); qStart.current = Date.now() }
      else {
        const secs = Math.round((Date.now() - started.current) / 1000)
        api.post<{ goal_met?: boolean; habit?: string }>(`${V1}/practice`, { learner, skill, level, questions: n, correct: next.filter(x => x.right).length, seconds: secs, details: next })
          .then(setSaved).catch(e => toast(errMsg(e), 'err'))
      }
    }, right ? 450 : 1300)
  }
  const correct = answers.filter(a => a.right).length
  const secs = Math.round((Date.now() - started.current) / 1000)
  return (
    <div className="drill" role="dialog" aria-label="Practice round">
      <div className="drill-top">
        <button type="button" className="btn ghost" onClick={() => onClose(done)}><Icon name="x" size={16} />{done ? 'Close' : 'Stop'}</button>
        <span className="grow" />
        <span className="drill-progress">{Math.min(answers.length + (done ? 0 : 1), n)} / {n}</span>
      </div>
      <div className="drill-bar"><span style={{ width: `${(answers.length / n) * 100}%` }} /></div>
      {!done ? <div className={`drill-card ${flash === true ? 'right' : flash === false ? 'wrong' : ''}`}>
        <div className="drill-q">{qs[i].q}</div>
        <input ref={input} className="drill-in" value={given} onChange={e => setGiven(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') submit() }}
          inputMode={skill === 'fractions' || skill === 'equations' ? 'text' : 'decimal'} autoComplete="off" aria-label="Your answer" placeholder={qs[i].hint || '?'} disabled={flash !== null} />
        <div className="drill-flash" aria-live="polite">{flash === true ? 'Right!' : flash === false ? `It is ${qs[i].answer}` : ''}</div>
        <button type="button" className="btn primary lg" onClick={submit} disabled={!given.trim() || flash !== null}>Check</button>
      </div> : <div className="drill-card end">
        <div className="drill-score">{correct} / {n}</div>
        <p className="muted">{correct === n ? 'All right — brilliant!' : correct >= n * 0.8 ? 'Very good!' : correct >= n * 0.5 ? 'Good work. Again?' : 'Keep going — it gets easier.'} · {Math.floor(secs / 60)}:{String(secs % 60).padStart(2, '0')}</p>
        {saved?.goal_met && <p className="callout ok"><Icon name="trophy" size={16} /><span>Today's goal is done!{saved.habit ? ' ' + saved.habit + '.' : ''}</span></p>}
        {answers.some(a => !a.right) && <div className="round-qs">{answers.filter(a => !a.right).map((a, k) => <span key={k} className="rq bad">{a.q} = {a.answer} <span className="muted">(not {a.given})</span></span>)}</div>}
        <button type="button" className="btn primary lg" onClick={() => onClose(true)}>Done</button>
      </div>}
    </div>
  )
}
