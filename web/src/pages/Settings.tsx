import { AskButton } from '../components/AskButton'
import React, { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useData } from '../lib/data'
import type { APIToken, AppSlug, AppState, ConnMode, Settings as SettingsT, User } from '../lib/types'
import { DATE_FORMATS, TIME_FORMATS, fmtBytes, fmtDateTime, type DateFormat, type TimeFormat } from '../lib/format'
import { Avatar, Confirm, CoreHint, Field, Loading, Modal, Segmented, SettingRow, Switch, Tabs, useAsync, useIsMobile, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { ShortcutsSheet } from '../components/ShortcutsSheet'
import { PocketModelSelect } from '../components/PocketModelSelect'
import { APP_INFO, STATE_WORDS } from '../lib/apps'

type TabId = 'general' | 'notifications' | 'apps' | 'ai' | 'profile' | 'people' | 'tokens' | 'data' | 'about'

const TABS: { id: TabId; label: string; icon: string; sub: string }[] = [
  { id: 'general', label: 'General', icon: 'settings', sub: 'How sums are worked out and shown, practice, dates and the app title.' },
  { id: 'notifications', label: 'Notifications', icon: 'bell', sub: 'What reaches your phone: practice that is not done yet.' },
  { id: 'apps', label: 'Connected apps', icon: 'link', sub: 'Rooms, Home, Garage, Health, Money, Wealth, Tables, Notes, Files, Photos, Chat, Learn, Slides, Habits, Family and Alerts.' },
  { id: 'ai', label: 'AI', icon: 'sparkles', sub: 'Who helps you: Pocket AI through Core, or your own OpenAI key.' },
  { id: 'profile', label: 'Profile', icon: 'user', sub: 'Your name, email and password.' },
  { id: 'people', label: 'People', icon: 'users', sub: 'Everyone who has an account on this box. Each person has their own notebooks.' },
  { id: 'tokens', label: 'API & shortcuts', icon: 'key', sub: 'Personal tokens for Siri Shortcuts, scripts and other tools.' },
  { id: 'data', label: 'Backup & data', icon: 'database', sub: 'Export everything, bring an export back, download the database.' },
  { id: 'about', label: 'About', icon: 'info', sub: 'Version, numbers and the server.' },
]

export default function Settings() {
  const { tab: raw } = useParams()
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const tab: TabId = TABS.find(t => t.id === raw)?.id || 'general'
  const current = TABS.find(t => t.id === tab)!
  const go = (id: TabId) => navigate(`/settings/${id}`, { replace: true })
  useEffect(() => { document.querySelector('.set-content')?.scrollTo({ top: 0 }) }, [tab])
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape' && !document.querySelector('.modal-backdrop, .menu, .sel-pop')) navigate('/') }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [navigate])
  return (
    <div className="set-page">
      <header className="set-header">
        <button type="button" className="btn ghost icon-only" aria-label="Back" title="Back (esc)" onClick={() => navigate('/')}><Icon name="chevron-left" size={18} /></button>
        <img src="/favicon.svg" alt="" width={24} height={24} className="set-logo" />
        <h1>Settings</h1>
        <span style={{ flex: 1 }} />
        <AskButton />
      </header>
      {mobile && <div className="set-tabs-mobile"><Tabs tabs={TABS.map(t => ({ id: t.id, label: t.label, icon: t.icon }))} value={tab} onChange={go} /></div>}
      <div className="set-body">
        {!mobile && (
          <nav className="set-nav" aria-label="Settings sections">
            {TABS.map(t => (
              <button key={t.id} type="button" className={`set-nav-item ${t.id === tab ? 'active' : ''}`} aria-current={t.id === tab ? 'page' : undefined} onClick={() => go(t.id)}>
                <Icon name={t.icon} size={16} />{t.label}
              </button>
            ))}
          </nav>
        )}
        <main className="set-content">
          <div className="set-inner" key={tab}>
            <div className="set-title"><h2>{current.label}</h2><p>{current.sub}</p></div>
            {tab === 'general' && <General />}
            {tab === 'notifications' && <Notifications />}
            {tab === 'apps' && <Integrations />}
            {tab === 'ai' && <AITab />}
            {tab === 'profile' && <Profile />}
            {tab === 'people' && <Users />}
            {tab === 'tokens' && <Tokens />}
            {tab === 'data' && <Data />}
            {tab === 'about' && <About />}
          </div>
        </main>
      </div>
    </div>
  )
}

function Section({ title, sub, icon, actions, children }: { title: React.ReactNode; sub?: React.ReactNode; icon?: string; actions?: React.ReactNode; children?: React.ReactNode }) {
  return (
    <section className="set-section">
      <div className="set-section-head">
        {icon && <span className="set-app-ico"><Icon name={icon} size={17} /></span>}
        <div className="grow"><h3>{title}</h3>{sub && <p>{sub}</p>}</div>
        {actions && <div className="row">{actions}</div>}
      </div>
      {children && <div className="set-section-body">{children}</div>}
    </section>
  )
}

function useSettingsForm() {
  const { settings, setSettings, user } = useApp()
  const { toast } = useToast()
  const [f, setF] = useState<SettingsT | null>(settings)
  const [busy, setBusy] = useState(false)
  useEffect(() => { if (!f && settings) setF(settings) }, [settings]) // eslint-disable-line react-hooks/exhaustive-deps
  const save = async (patch?: Partial<SettingsT>) => {
    if (!f) return
    setBusy(true)
    try { const s = await api.put<SettingsT>(`${V1}/settings`, { ...f, ...patch }); setSettings(s); setF(s); toast('Saved') } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return { f, setF, save, busy, isAdmin: user?.role === 'admin' }
}

function useTester() {
  const [test, setTest] = useState<Record<string, string>>({})
  const run = async (which: string, url: string, key: string, extra: Record<string, string> = {}) => {
    setTest(t => ({ ...t, [which]: 'testing…' }))
    try { const r = await api.post<{ ok: boolean; message?: string; error?: string }>(`${V1}/integrations/test`, { which, url, key, ...extra }); setTest(t => ({ ...t, [which]: r.ok ? `✓ ${r.message}` : `✕ ${r.error}` })) } catch (e) { setTest(t => ({ ...t, [which]: `✕ ${(e as Error).message}` })) }
  }
  const status = (which: string) => <span className={`small ${test[which]?.startsWith('✓') ? 'ok' : test[which]?.startsWith('✕') ? 'bad' : 'muted'}`}>{test[which]}</span>
  return { run, status }
}

const HOURS = Array.from({ length: 24 }, (_, h) => ({ value: String(h), label: `${String(h).padStart(2, '0')}:00` }))

function General() {
  const { f, setF, save, busy, isAdmin } = useSettingsForm()
  const { rates, ratesInfo } = useData()
  const { data: habits } = useAsync(() => api.get<{ habits: { id: number; name: string }[] }>(`${V1}/apps/habits`).then(r => r.habits).catch(() => null), [])
  if (!f) return <Loading />
  return (
    <div className="stack loose">
      <Section title="Working out" icon="calc" sub="How a new notebook starts; each notebook can change it (its ⋯ menu).">
        <SettingRow title="Angles" help="How plain numbers in sin, cos and tan are read. 30° always means degrees."><Segmented size="sm" value={f.angle} onChange={v => setF({ ...f, angle: v })} options={[{ value: 'deg', label: 'Degrees' }, { value: 'rad', label: 'Radians' }]} /></SettingRow>
        <SettingRow title="Read units and money" help="5 km + 300 m, 100 EUR in BAM. Off: letters are only names (x, y, a)."><Switch checked={f.units} onChange={v => setF({ ...f, units: v })} label="Units" /></SettingRow>
        <SettingRow title="Numbers look like"><Segmented size="sm" value={f.number_fmt} onChange={v => setF({ ...f, number_fmt: v })} options={[{ value: '1,234.5', label: '1,234.5' }, { value: '1.234,5', label: '1.234,5' }, { value: '1 234,5', label: '1 234,5' }, { value: '1234.5', label: '1234.5' }]} /></SettingRow>
        <SettingRow title="Digits shown" help="Answers are rounded to this many significant digits."><Segmented size="sm" value={f.digits} onChange={v => setF({ ...f, digits: v })} options={[{ value: 6, label: '6' }, { value: 8, label: '8' }, { value: 10, label: '10' }, { value: 12, label: '12' }, { value: 15, label: '15' }]} /></SettingRow>
        <SettingRow title="Fractions beside decimals" help="0.3333333333 = 1/3"><Switch checked={f.show_alt} onChange={v => setF({ ...f, show_alt: v })} label="Fractions" /></SettingRow>
      </Section>
      <Section title="Money" icon="coins" sub={ratesInfo?.source ? `Exchange rates of the ${ratesInfo.source}, ${ratesInfo.date} (${Object.keys(rates).length} currencies). The server fetches them twice a day.` : ratesInfo?.note}>
        <SettingRow title="Your money" help="First in the converter."><input className="set-short" value={f.currency} onChange={e => setF({ ...f, currency: e.target.value.toUpperCase().slice(0, 3) })} placeholder="EUR" /></SettingRow>
        <SettingRow title="Exchange rates" help="Off: only currencies tied to the euro (BAM, BGN, DKK…) convert."><Segmented size="sm" value={f.rates} onChange={v => setF({ ...f, rates: v })} options={[{ value: 'ecb', label: 'European Central Bank' }, { value: 'off', label: 'Off' }]} /></SettingRow>
      </Section>
      <Section title="Graphs" icon="graph">
        <SettingRow title="Grid lines"><Switch checked={f.graph_grid} onChange={v => setF({ ...f, graph_grid: v })} label="Grid" /></SettingRow>
        <SettingRow title="x in radians" help="sin(x) goes round every 2π (as in school maths). Off: every 360."><Switch checked={f.graph_rad} onChange={v => setF({ ...f, graph_rad: v })} label="Radians" /></SettingRow>
      </Section>
      <Section title="Practice" icon="abacus">
        <SettingRow title="Questions a day" help="The daily goal for each person practising; 0 for none."><Segmented size="sm" value={f.daily_goal} onChange={v => setF({ ...f, daily_goal: v })} options={[{ value: 0, label: 'None' }, { value: 10, label: '10' }, { value: 20, label: '20' }, { value: 30, label: '30' }, { value: 50, label: '50' }]} /></SettingRow>
        <SettingRow title="Questions in a round"><Segmented size="sm" value={f.round_size} onChange={v => setF({ ...f, round_size: v })} options={[{ value: 5, label: '5' }, { value: 10, label: '10' }, { value: 15, label: '15' }, { value: 20, label: '20' }]} /></SettingRow>
        <SettingRow title="Tick a habit in Pocket Habits" help="When your own goal for the day is reached.">
          {habits ? <Select ariaLabel="Habit" value={String(f.practice_habit || 0)} onChange={v => setF({ ...f, practice_habit: Number(v) })} options={[{ value: '0', label: 'None' }, ...habits.map(h => ({ value: String(h.id), label: h.name }))]} /> : <span className="muted small">Link Pocket Habits in Connected apps.</span>}
        </SettingRow>
      </Section>
      <Section title="Dates" icon="calendar">
        <SettingRow title="Week starts on"><Segmented size="sm" value={f.week_start} onChange={v => setF({ ...f, week_start: v })} options={[{ value: 1, label: 'Monday' }, { value: 6, label: 'Saturday' }, { value: 7, label: 'Sunday' }]} /></SettingRow>
        <SettingRow title="Date format"><Select ariaLabel="Date format" value={f.date_format} onChange={v => setF({ ...f, date_format: v as DateFormat })} options={DATE_FORMATS.map(d => ({ value: d.id, label: d.label }))} /></SettingRow>
        <SettingRow title="Time format"><Select ariaLabel="Time format" value={f.time_format} onChange={v => setF({ ...f, time_format: v as TimeFormat })} options={TIME_FORMATS.map(d => ({ value: d.id, label: d.label }))} /></SettingRow>
        {isAdmin && <SettingRow title="App title" help="Shown in the sidebar for everyone on this box."><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} /></SettingRow>}
      </Section>
      <div className="row"><button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button></div>
    </div>
  )
}

function Notifications() {
  const { f, setF, save, busy } = useSettingsForm()
  const { run, status } = useTester()
  if (!f) return <Loading />
  return (
    <div className="stack loose">
      <Section title="Push" icon="bell" actions={<span className={`badge ${f.notify_configured ? 'ok' : ''}`}>{f.notify_configured ? 'on' : 'not set'}</span>}
        sub={<>Where messages go: an Apprise gateway (<code>…/notify/&lt;key&gt;</code>), an ntfy topic URL, or any webhook that takes JSON.</>}>
        <SettingRow title="URL" stack><input value={f.notify_url} onChange={e => setF({ ...f, notify_url: e.target.value })} placeholder="http://apprise:8000/notify/math" /></SettingRow>
        <SettingRow title="Format"><Segmented value={f.notify_format} onChange={v => setF({ ...f, notify_format: v })} options={[{ value: 'apprise', label: 'Apprise' }, { value: 'ntfy', label: 'ntfy' }, { value: 'json', label: 'JSON' }]} size="sm" /></SettingRow>
      </Section>
      <Section title="What to push" icon="bell">
        <SettingRow title="Practice is not done yet" help="At the hour below, when someone practising here has not reached the day's goal."><Switch checked={f.notify_practice} onChange={v => setF({ ...f, notify_practice: v })} label="Practice reminder" /></SettingRow>
        <SettingRow title="Reminders go at" help="Server time."><Select ariaLabel="Hour" value={String(f.notify_hour)} onChange={v => setF({ ...f, notify_hour: Number(v) })} options={HOURS} /></SettingRow>
      </Section>
      <Section title="Pocket Alerts" icon="bell" sub={f.alerts_via ? 'Also in your Pocket Alerts inbox.' : 'Connect Pocket Alerts (Connected apps) to get these in your alerts inbox too.'}>
        <SettingRow title="Practice reminder in Pocket Alerts"><Switch checked={f.alerts_goal} onChange={v => setF({ ...f, alerts_goal: v })} label="Alerts" disabled={!f.alerts_via} /></SettingRow>
      </Section>
      <div className="row wrap">
        <button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button>
        <button type="button" className="btn" onClick={() => run('notify', f.notify_url, '', { format: f.notify_format })} disabled={!f.notify_url}><Icon name="mail" size={15} />Send a test push</button>
        {status('notify')}
      </div>
    </div>
  )
}

const APPS: { slug: AppSlug; title: string; sub: string; token: string; port: string }[] = [
  { slug: 'rooms', title: 'Pocket Rooms — room sizes', sub: 'Fill in floor, walls (without doors and windows), skirting and height for paint, flooring and wallpaper.', token: 'Pocket Rooms → Settings → API tokens', port: '8265' },
  { slug: 'home', title: 'Pocket Home — meters and prices', sub: 'Your meter readings by month into a table; the price of a kWh for running costs.', token: 'Pocket Home → Settings → API tokens', port: '8229' },
  { slug: 'garage', title: 'Pocket Garage — real fuel use', sub: "Your car's fuel use and last price for trip costs; fill-ups into a table.", token: 'Pocket Garage → Settings → API tokens', port: '8172' },
  { slug: 'health', title: 'Pocket Health — weight and height', sub: 'For BMI, calories and water; your weight over a year into a table.', token: 'Pocket Health → Settings → API tokens', port: '8196' },
  { slug: 'money', title: 'Pocket Money — spending by month', sub: 'Spending and income per month into a table for averages and trends; account balances.', token: 'Pocket Money → Settings → API tokens', port: '8178' },
  { slug: 'wealth', title: 'Pocket Wealth — savings', sub: 'Start savings sums from what you have; your net worth over time into a table.', token: 'Pocket Wealth → Settings → API tokens', port: '8231' },
  { slug: 'tables', title: 'Pocket Tables — tables in and out', sub: 'Work out averages and trends from a table; add answers to a table as rows.', token: 'Pocket Tables → Settings → API tokens', port: '8252' },
  { slug: 'notes', title: 'Pocket Notes — notebooks as notes', sub: 'Keep a notebook with its answers as a note.', token: 'Pocket Notes → Settings → API tokens', port: '8182' },
  { slug: 'files', title: 'Pocket Files — CSV in, files out', sub: 'Bring in a CSV; save tables, notebooks and pictures there.', token: 'Pocket Files → Settings → API tokens', port: '8187' },
  { slug: 'photos', title: 'Pocket Photos — homework in a photo', sub: 'Pick a photo of a sum or a homework sheet for Pocket AI to read.', token: 'Pocket Photos → Settings → API tokens', port: '8194' },
  { slug: 'chat', title: 'Pocket Chat — share an answer', sub: 'Send answers, or a notebook link, to a chat.', token: 'Pocket Chat → Settings → API tokens', port: '8247' },
  { slug: 'learn', title: 'Pocket Learn — flashcards', sub: 'Turn a formula into a flashcard.', token: 'Pocket Learn → Settings → API tokens', port: '8218' },
  { slug: 'slides', title: 'Pocket Slides — a chart slide', sub: 'Make a chart slide from a table of data.', token: 'Pocket Slides → Settings → API tokens', port: '8269' },
  { slug: 'habits', title: 'Pocket Habits — the practice habit', sub: 'Tick your habit when the day’s practice is done.', token: 'Pocket Habits → Settings → API tokens', port: '8175' },
  { slug: 'family', title: 'Pocket Family — the children', sub: 'Pick the children who practise here from your family.', token: 'Pocket Family → Settings → API tokens', port: '8257' },
  { slug: 'alerts', title: 'Pocket Alerts — reminders', sub: 'Practice that is not done yet, in your alerts inbox.', token: 'Pocket Alerts → Settings → API tokens', port: '8217' },
]
function Integrations() {
  const { f, setF, save, busy } = useSettingsForm()
  const { run, status } = useTester()
  const { data: states } = useAsync(() => api.get<{ apps: AppState[] }>(`${V1}/apps`).then(r => r.apps).catch(() => [] as AppState[]), [APPS.map(a => f?.[`${a.slug}_mode`]).join()])
  if (!f) return <Loading />
  const coreOn = f.core_configured && f.core_registered !== false
  const eff = (m: ConnMode): ConnMode => !coreOn && m === 'core' ? 'off' : m
  const set = (k: string, v: string) => setF({ ...f, [k]: v } as SettingsT)
  return (
    <div className="stack loose">
      <div className="callout info"><Icon name="info" size={15} /><div>These connections are yours. Math only reads or adds things in them when you press a button (and ticks your practice habit, if you choose one), and everything here works without them. Through <b>Pocket Core</b> Math acts as you in your own Pocket apps with nothing to paste (switch it off for yourself in Pocket Core → Settings → Apps acting for me); <b>Direct</b> uses a URL and an API token you enter.{!coreOn && <CoreHint />}</div></div>
      {APPS.map(a => {
        const mode = f[`${a.slug}_mode`]
        const via = f[`${a.slug}_via`]
        const name = APP_INFO[a.slug].name
        const st = states?.find(x => x.slug === a.slug)
        return (
          <Section key={a.slug} title={a.title} icon={APP_INFO[a.slug].icon} actions={st && st.state !== 'on' && st.state !== 'off' ? <span className={`badge ${st.state === 'partly' ? 'ok' : 'warn'}`}>{STATE_WORDS[st.state]}</span> : connBadge(eff(mode), via)} sub={a.sub}>
            <Connection app={name} mode={mode} via={via} detail={f[`${a.slug}_detail`]} core={coreOn} hasToken={!!f[`${a.slug}_token`]}
              onMode={m => save({ [`${a.slug}_mode`]: m } as Partial<SettingsT>)} onDisconnect={() => save({ [`${a.slug}_url`]: '', [`${a.slug}_token`]: '' } as Partial<SettingsT>)}
              test={<><button type="button" className="btn sm" onClick={() => run(a.slug, f[`${a.slug}_url`], f[`${a.slug}_token`])}>Test connection</button>{status(a.slug)}</>}
              direct={<div className="two-col tight">
                <Field label="URL" help="Inside Docker use http://host.docker.internal:PORT or the service name."><input value={f[`${a.slug}_url`]} onChange={e => set(`${a.slug}_url`, e.target.value)} placeholder={`http://host.docker.internal:${a.port}`} /></Field>
                <Field label="API token" help={a.token}><input value={f[`${a.slug}_token`]} onChange={e => set(`${a.slug}_token`, e.target.value)} autoComplete="off" /></Field>
              </div>} />
            {st?.fix && st.state !== 'on' && <p className="muted small"><Icon name="info" size={12} className="inline-ico" /> {st.fix}.</p>}
          </Section>
        )
      })}
      <Section title="Where things go" icon="layers">
        <SettingRow title="Notes folder" help="Where notebooks go in Pocket Notes."><input value={f.notes_folder} onChange={e => setF({ ...f, notes_folder: e.target.value })} /></SettingRow>
        <SettingRow title="Files folder" help="Where tables and notebooks go in Pocket Files."><input value={f.files_folder} onChange={e => setF({ ...f, files_folder: e.target.value })} /></SettingRow>
        <SettingRow title="Chat" help="The Pocket Chat conversation to send to first; empty: a note to yourself."><input value={f.chat_to} onChange={e => setF({ ...f, chat_to: e.target.value })} placeholder="me" /></SettingRow>
        <SettingRow title="Learn deck" help="The Pocket Learn deck flashcards go to; empty: Maths."><input value={f.learn_deck} onChange={e => setF({ ...f, learn_deck: e.target.value })} placeholder="Maths" /></SettingRow>
      </Section>
      <Section title="Other Pocket apps" icon="layers" sub="Math offers your maths to the rest of the box through Pocket Core: Pocket Day shows what you worked out and practised, Pocket Search finds your notebooks and calculations, Pocket AI can work sums out exactly (with units and money) and keep answers for you. Switched off until you allow them in Pocket Core → Links.">
        <dl className="set-dl">
          <div><dt>math:read</dt><dd className="muted">Your notebooks with their answers, your calculations, kept answers and practice — and exact sums and conversions.</dd></div>
          <div><dt>math:write</dt><dd className="muted">Start a notebook, add cells, save a calculation and keep an answer; that app may change only what it made, and can never share or delete your own notebooks.</dd></div>
          <div><dt>events:read</dt><dd className="muted"><code>math.notebook_created</code>, <code>answer_kept</code>, <code>formula_saved</code>, <code>practised</code>, <code>shared</code> — for Pocket Flows.</dd></div>
        </dl>
      </Section>
      <div className="row"><button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button></div>
    </div>
  )
}

function AITab() {
  const { f, setF, save, busy } = useSettingsForm()
  const { run, status } = useTester()
  const { data: ai } = useAsync(() => api.get<{ ready: boolean; provider: string; detail: string }>(`${V1}/ai/status`), [])
  if (!f) return <Loading />
  const coreOn = f.core_configured && f.core_registered !== false
  const shown = !coreOn && f.ai_provider === 'pocket' ? 'openai' : (f.ai_provider || 'auto')
  return (
    <div className="stack loose">
      <Section title="What the AI does" icon="sparkles" sub="Only when you press its button: it explains the working step by step (for you or a child), turns a problem in words or a photo into sums, drafts a calculation to reuse, and checks how a notebook is set up. The numbers always come from the calculator, never from the AI." >
        <SettingRow title="AI buttons" help="Off: the buttons are gone and nothing is sent to an AI."><Switch checked={f.ai_help} onChange={v => save({ ai_help: v })} label="AI buttons" /></SettingRow>
        {ai && (ai.ready ? <p className="muted small row" style={{ gap: 6 }}><Icon name="sparkles" size={13} />{ai.provider === 'pocket' ? 'Pocket AI' : 'Your OpenAI key'} is ready.</p>
          : <div className="callout info"><Icon name="info" size={15} /><div>No AI connected{ai.detail ? ` (${ai.detail})` : ''}.{!coreOn && <CoreHint />}</div></div>)}
      </Section>
      <Section title="Provider" icon="sparkles">
        <div className="grid" style={{ gap: 12 }}>
          <Field group label="Who writes" help="Automatic prefers your enabled Pocket AI connection; without one it uses your OpenAI settings.">
            <Select ariaLabel="AI provider" value={shown} onChange={v => setF({ ...f, ai_provider: v as SettingsT['ai_provider'] })} options={[{ value: 'auto', label: coreOn ? 'Automatic — prefer Pocket AI' : 'Automatic' }, { value: 'pocket', label: 'Pocket AI', disabled: !coreOn, hint: coreOn ? undefined : 'needs Pocket Core' }, { value: 'openai', label: 'OpenAI / compatible API' }]} />
          </Field>
          {!coreOn && <CoreHint />}
          {coreOn && shown !== 'openai' && <PocketModelSelect value={f.ai_pocket_model || ''} onChange={v => setF({ ...f, ai_pocket_model: v })} />}
          {(shown === 'openai' || (!coreOn && shown === 'auto')) && <>
            <Field label="OpenAI API key" help="platform.openai.com → API keys. Stored on the server; shown masked."><input value={f.openai_api_key} onChange={e => setF({ ...f, openai_api_key: e.target.value })} autoComplete="off" placeholder="sk-…" /></Field>
            <div className="two-col tight">
              <Field label="Model" help="Default gpt-4o-mini."><input value={f.openai_model} onChange={e => setF({ ...f, openai_model: e.target.value })} placeholder="gpt-4o-mini" /></Field>
              <Field label="Base URL (optional)" help="Ollama, LM Studio, OpenRouter…"><input value={f.openai_base_url} onChange={e => setF({ ...f, openai_base_url: e.target.value })} placeholder="https://api.openai.com/v1" /></Field>
            </div>
          </>}
        </div>
      </Section>
      <div className="row wrap">
        <button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button>
        {(shown === 'openai' || (!coreOn && shown === 'auto')) && <><button type="button" className="btn" onClick={() => run('openai', f.openai_base_url, f.openai_api_key)} disabled={!f.openai_api_key}>Test the OpenAI key</button>{status('openai')}</>}
      </div>
    </div>
  )
}

function Connection({ app, mode: saved, via, detail, core, onMode, onDisconnect, hasToken, test, direct }: {
  app: string; mode: ConnMode; via: string; detail: string; core: boolean; onMode: (m: ConnMode) => void; onDisconnect: () => void
  hasToken: boolean; test: React.ReactNode; direct: React.ReactNode
}) {
  const [advanced, setAdvanced] = useState(saved === 'direct')
  const mode: ConnMode = !core && saved === 'core' ? 'off' : saved
  const ok = via !== '' && mode !== 'off'
  const showDirect = !core || mode === 'direct' || advanced
  const line = mode === 'off' ? `Off: Math does not reach ${app}.`
    : mode === 'core' ? (ok ? `Connected through Pocket Core, as you${detail ? ` (${detail})` : ''}. Nothing to fill in; switch it off for yourself in Pocket Core → Settings → Apps acting for me.` : `Not connected: ${detail}.`)
    : (ok ? `Direct connection with your own ${app} URL and token; use Test connection to check it.` : `Not connected: ${detail || 'add the URL and a token'}.`)
  return (
    <div className="conn-block">
      <div className="conn-mode">
        <Segmented size="sm" value={mode} onChange={m => onMode(m as ConnMode)} options={[
          { value: 'core', label: 'Pocket Core', title: core ? 'Through Pocket Core: per person, nothing to paste' : 'Needs Pocket Core: there is none on this box', disabled: !core },
          ...(showDirect ? [{ value: 'direct', label: 'Direct', title: 'Your own URL and API token' }] : []),
          { value: 'off', label: 'Off' },
        ]} />
        {!showDirect && <button type="button" className="conn-advanced" onClick={() => setAdvanced(true)}>Advanced: connect directly with a URL and key</button>}
      </div>
      <div className={`conn-status ${mode === 'off' ? '' : ok ? 'ok' : 'bad'}`}><span className="dot" />{line}</div>
      {mode === 'direct' && <>
        {direct}
        <div className="row wrap">{test}{hasToken && <button type="button" className="btn sm ghost danger-text" onClick={onDisconnect} title="Remove the saved URL and token">Disconnect</button>}</div>
      </>}
      {mode === 'core' && <div className="row wrap">{test}</div>}
    </div>
  )
}

const connBadge = (mode: ConnMode, via: string) => mode === 'off' ? <span className="badge">off</span>
  : via ? <span className="badge ok">{via === 'core' ? 'Pocket Core' : 'direct'}</span>
  : <span className="badge warn">not connected</span>

function Users() {
  const { user, users, reloadMeta, status } = useApp()
  const { toast } = useToast()
  const [edit, setEdit] = useState<Partial<User> | null>(null)
  const [del, setDel] = useState<User | null>(null)
  const isAdmin = user?.role === 'admin'
  const run = async (fn: () => Promise<unknown>, msg: string) => { try { await fn(); await reloadMeta(); toast(msg) } catch (e) { toast((e as Error).message, 'err') } }
  return (
    <Section title="People on this box" actions={isAdmin && <button type="button" className="btn primary sm" onClick={() => setEdit({ role: 'member' })}><Icon name="plus" size={14} />Add person</button>}
      sub="Everyone signs in with their own account and has their own notebooks, calculations, practice, settings and connections. Admins manage accounts; they never see other people's notebooks.">
      <div className="set-people">
        {users.map(u => (
          <div key={u.id} className="set-person">
            <Avatar name={u.name} email={u.email} size={34} />
            <div className="grow">
              <div className="set-person-name">{u.name || <span className="muted">—</span>}{u.id === user?.id && <span className="badge">you</span>}{u.role === 'admin' && <span className="badge accent">admin</span>}</div>
              <div className="muted small ellipsis">{u.email}</div>
            </div>
            {isAdmin && <span className="row" style={{ justifyContent: 'flex-end' }}><button type="button" className="btn ghost sm" onClick={() => setEdit(u)}>Edit</button>{u.id !== user?.id && <button type="button" className="btn ghost sm danger-text" onClick={() => setDel(u)}>Remove</button>}</span>}
          </div>
        ))}
      </div>
      {status?.auth_mode !== 'local' && <p className="muted small mt">Auth mode is <code>{status?.auth_mode}</code>: users are created automatically on first visit; passwords are not used here.</p>}
      {edit && <UserModal user={edit} local={status?.auth_mode === 'local'} onClose={() => setEdit(null)} onSaved={() => { setEdit(null); reloadMeta() }} />}
      {del && <Confirm confirmLabel="Remove" title={`Remove ${del.name || del.email}?`} message="Their account goes, with their notebooks, calculations and practice." onCancel={() => setDel(null)} onConfirm={() => { const d = del; setDel(null); run(() => api.del(`${V1}/users/${d.id}`), 'User removed') }} />}
    </Section>
  )
}

function UserModal({ user, local, onClose, onSaved }: { user: Partial<User>; local: boolean; onClose: () => void; onSaved: () => void }) {
  const { toast } = useToast()
  const [f, setF] = useState({ name: user.name || '', email: user.email || '', password: '', role: user.role || 'member' })
  const [busy, setBusy] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    try { if (user.id) await api.put(`${V1}/users/${user.id}`, f); else await api.post(`${V1}/users`, f); toast(user.id ? 'User saved' : 'User added'); onSaved() } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={user.id ? 'Edit person' : 'Add a person'} onClose={onClose} size="sm">
      <form onSubmit={save} className="grid" style={{ gap: 14 }}>
        <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} autoFocus required maxLength={60} /></Field>
        <Field label="Email" help="Used to sign in."><input type="email" value={f.email} onChange={e => setF({ ...f, email: e.target.value })} required /></Field>
        {local && <Field label={user.id ? 'New password (leave empty to keep)' : 'Password'} help="At least 8 characters."><input type="password" value={f.password} onChange={e => setF({ ...f, password: e.target.value })} minLength={user.id ? 0 : 8} required={!user.id} autoComplete="new-password" /></Field>}
        <Field group label="Role"><Segmented value={f.role} onChange={v => setF({ ...f, role: v })} options={[{ value: 'member', label: 'Member' }, { value: 'admin', label: 'Admin' }]} /></Field>
        <div className="form-actions"><button type="button" className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={busy}>{user.id ? 'Save' : 'Add person'}</button></div>
      </form>
    </Modal>
  )
}

function Profile() {
  const { user, setUser, status } = useApp()
  const { toast } = useToast()
  const [f, setF] = useState({ name: user?.name || '', email: user?.email || '', current_password: '', new_password: '' })
  const [busy, setBusy] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    try {
      setUser(await api.put(`${V1}/auth/me`, f))
      setF(x => ({ ...x, current_password: '', new_password: '' })); toast('Profile saved')
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <form onSubmit={save} className="stack loose">
      <div className="set-profile">
        <Avatar name={user?.name} email={user?.email} size={56} />
        <div className="grow"><h3>{user?.name || user?.email}</h3><p className="muted small">{user?.email} · {user?.role}</p></div>
      </div>
      <Section title="You">
        <div className="stack">
          <Field label="Name" help="Shown when you share a notebook."><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} /></Field>
          <Field label="Email" help="The same email as in Pocket Core, so signing in finds your account."><input type="email" value={f.email} onChange={e => setF({ ...f, email: e.target.value })} /></Field>
        </div>
      </Section>
      {status?.auth_mode === 'local' && <Section title="Password" sub="Leave empty to keep the current one."><div className="stack">
        <Field label="Current password"><input type="password" value={f.current_password} onChange={e => setF({ ...f, current_password: e.target.value })} autoComplete="current-password" /></Field>
        <Field label="New password" help="At least 8 characters."><input type="password" value={f.new_password} onChange={e => setF({ ...f, new_password: e.target.value })} autoComplete="new-password" /></Field>
      </div></Section>}
      <div className="row"><button className="btn primary" disabled={busy}>Save changes</button></div>
    </form>
  )
}

function Tokens() {
  const { toast } = useToast()
  const { settings } = useApp()
  const { data, reload } = useAsync(() => api.get<APIToken[]>(`${V1}/tokens`), [])
  const [name, setName] = useState('')
  const [fresh, setFresh] = useState('')
  const [del, setDel] = useState<APIToken | null>(null)
  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    try { const r = await api.post<{ token: string }>(`${V1}/tokens`, { name: name || 'token' }); setFresh(r.token); setName(''); reload() } catch (e) { toast((e as Error).message, 'err') }
  }
  const base = settings?.base_url || window.location.origin
  return (
    <div className="stack loose">
      <Section title="Personal tokens" icon="key" sub={<>Use as <code>Authorization: Bearer &lt;token&gt;</code>. A token acts as you: it can read and change everything, feed a Homepage widget (<code>GET /api/v1/summary</code>) or send a recording from a Siri Shortcut. See <code>docs/api.md</code>.</>}>
        <form onSubmit={create} className="row mb"><input value={name} onChange={e => setName(e.target.value)} placeholder="Token name (e.g. iPhone Shortcut)" /><button className="btn primary"><Icon name="plus" size={15} />Create</button></form>
        {fresh && <div className="set-fresh mb"><div className="row"><Icon name="check-circle" size={16} /><b>Copy it now — it is shown once.</b></div><pre className="mono">{fresh}</pre></div>}
        {data && data.length > 0 ? (
          <table className="table compact">
            <thead><tr><th>Name</th><th>Prefix</th><th>Last used</th><th>Created</th><th /></tr></thead>
            <tbody>{data.map(t => <tr key={t.id}><td>{t.name}</td><td className="mono">{t.prefix}…</td><td className="muted">{t.last_used_at ? fmtDateTime(t.last_used_at) : 'never'}</td><td className="muted">{fmtDateTime(t.created_at)}</td><td className="right"><button type="button" className="btn ghost sm danger-text" onClick={() => setDel(t)}>Revoke</button></td></tr>)}</tbody>
          </table>
        ) : <p className="muted">No tokens yet.</p>}
        {del && <Confirm confirmLabel="Revoke" title={`Revoke “${del.name}”?`} message="Anything using this token stops working immediately." onCancel={() => setDel(null)} onConfirm={async () => { await api.del(`${V1}/tokens/${del.id}`); setDel(null); reload() }} />}
      </Section>
      <Section title="Endpoints" icon="code" sub={<>At <code>{base}/api/v1</code>.</>}>
        <dl className="set-dl">
          <div><dt><code>GET /calculate?q=…</code></dt><dd className="muted">Works a sum out: <code>?q=100 EUR in BAM</code> → <code>{'{"result": {"text": "195.58 BAM", "number": 195.58, "unit": "BAM"}}'}</code>. <code>POST /calculate</code> takes <code>{'{"expr": "…"}'}</code>, several lines too — for a Siri Shortcut.</dd></div>
          <div><dt><code>GET /convert?value=5&amp;from=km&amp;to=mi</code></dt><dd className="muted">A unit or money conversion.</dd></div>
          <div><dt><code>GET /summary</code></dt><dd className="muted">Notebooks, answers and today's practice — for Homepage.</dd></div>
          <div><dt><code>GET /notebooks</code></dt><dd className="muted">Your notebooks; <code>GET /notebooks/ID</code> with its cells and answers; <code>POST /notebooks</code> <code>{'{"title": "Trip", "cells": [{"kind": "calc", "body": "2 * 320 km"}]}'}</code>.</dd></div>
          <div><dt><code>POST /runs</code></dt><dd className="muted"><code>{'{"key": "paint", "values": {"wall_area": "32"}, "title": "Bedroom"}'}</code> works a ready calculation out and keeps the answer.</dd></div>
          <div><dt><code>GET /search?q=…</code></dt><dd className="muted">Find notebooks, calculations and kept answers.</dd></div>
          <div><dt><code>GET /day/YYYY-MM-DD</code></dt><dd className="muted">What you worked out and practised on a day.</dd></div>
        </dl>
      </Section>
    </div>
  )
}

function Data() {
  const { user } = useApp()
  const { toast } = useToast()
  const file = React.useRef<HTMLInputElement>(null)
  const bring = async (fl: File) => {
    try {
      const res = await fetch(`${V1}/import`, { method: 'POST', body: await fl.text(), headers: { 'Content-Type': 'application/json' }, credentials: 'same-origin' })
      const j = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(j.error || res.statusText)
      const a = j.added || {}; toast(`Brought in: ${a.notebooks || 0} notebooks, ${a.formulas || 0} calculations, ${a.answers || 0} answers, ${a.practice || 0} rounds`)
    } catch (e) { toast((e as Error).message, 'err') }
  }
  return (
    <div className="stack loose">
      <Section title="Export" icon="archive" sub="Everything you made as one JSON file — notebooks with their cells, your calculations, kept answers and practice — or your kept answers as a spreadsheet (CSV).">
        <div className="row wrap"><a className="btn" href={`${V1}/export.json`}><Icon name="download" size={15} />Export JSON</a><a className="btn" href={`${V1}/export.csv`}><Icon name="download" size={15} />Kept answers as CSV</a></div>
      </Section>
      <Section title="Bring a file in" icon="upload" sub="A Pocket Math export (also from another box): everything in it comes back as yours. The same file twice adds nothing twice.">
        <button type="button" className="btn" onClick={() => file.current?.click()}><Icon name="upload" size={15} />Bring a file…</button>
        <input ref={file} type="file" hidden accept=".json,application/json" onChange={e => { const fl = e.target.files?.[0]; e.target.value = ''; if (fl) bring(fl) }} />
      </Section>
      {user?.role === 'admin' && <Section title="Database backup" icon="database" sub="The whole box, every account — admins only.">
        <p className="muted">A consistent copy of the SQLite database (<code>VACUUM INTO</code>). Restore by stopping the app and replacing <code>pocket-math.db</code> in the data folder.</p>
        <a className="btn" href={`${V1}/backup`}><Icon name="database" size={15} />Download database</a>
      </Section>}
    </div>
  )
}

function About() {
  const { data, error } = useAsync(() => api.get<Record<string, unknown>>(`${V1}/settings/system`), [])
  const [keys, setKeys] = useState(false)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} /><span>{error}</span></div>
  if (!data) return <Loading />
  const counts = (data.counts || {}) as Record<string, number>
  const core = (data.core || {}) as { configured?: boolean; registered?: boolean; url?: string; error?: string }
  const tiles: [string, number | undefined, string][] = [['Notebooks', counts.notebooks, 'calc'], ['Cells', counts.cells, 'layers'], ['Your calculations', counts.formulas, 'fx'], ['Kept answers', counts.runs, 'check-circle'], ['Practice rounds', counts.practice, 'abacus']]
  const rows: [string, React.ReactNode][] = [
    ['Version', <code key="v">{String(data.version)}</code>],
    ['Uptime', String(data.uptime)],
    ['Runtime', `${String(data.go)} · ${String(data.os)}/${String(data.arch)}`],
    ['Time zone', String(data.timezone)],
    ['Sign-in', String(data.auth_mode)],
    ['Pocket Core', core.configured ? (core.registered ? <span key="c" className="ok">connected · {core.url}</span> : <span key="c" className="bad">{core.error || 'not registered yet'}</span>) : <span key="c" className="muted">not configured</span>],
    ['Database', <span key="db"><code>{String(data.db_path)}</code> · {fmtBytes(Number(data.db_bytes))}</span>],
    ['People', `${counts.users ?? 0} accounts on this box`],
  ]
  return (
    <div className="stack loose">
      <div className="set-about-hero">
        <img src="/favicon.svg" alt="" width={48} height={48} />
        <div>
          <h3>Pocket Math</h3>
          <p className="muted small">Maths for everyday life and school: sums with units, money, percentages and dates; graphs, geometry, data and equations; ready calculations filled in from your other apps; practice for children. One Go binary, one SQLite file, one volume.</p>
        </div>
      </div>
      <div className="set-stats">
        {tiles.map(([label, n, icon]) => (
          <div key={label} className="set-stat"><Icon name={icon} size={16} /><span className="set-stat-n tnum">{(n ?? 0).toLocaleString()}</span><span className="set-stat-l">{label}</span></div>
        ))}
      </div>
      <Section title="This server">
        <dl className="set-dl">
          {rows.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}
        </dl>
      </Section>
      <div className="row wrap">
        <button type="button" className="btn" onClick={() => setKeys(true)}><Icon name="keyboard" size={15} />Keyboard shortcuts</button>
        <a className="btn" href="https://github.com/abjelosevic88/pocket-homelab-math" target="_blank" rel="noreferrer"><Icon name="external" size={15} />Project on GitHub</a>
      </div>
      <p className="muted small">Open source (MIT). Part of the Pocket apps: Pocket Core connects them — Calendar, Contacts, Family, Email, Chat, Photos, Notes, Files, Voice, Tasks, Alerts, Blog, Weather, Bookshelf, Music, Movement, Health, Wealth, Pocket AI and more.</p>
      {keys && <ShortcutsSheet onClose={() => setKeys(false)} />}
    </div>
  )
}
