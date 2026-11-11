// The app shell: sidebar, top bar, search palette, shortcuts — the Pocket Calendar layout.
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, DataProvider } from '../lib/data'
import { Icon } from './icons'
import { Sidebar } from './Sidebar'
import { CommandPalette, type PaletteCommand } from './CommandPalette'
import { ShortcutsSheet } from './ShortcutsSheet'
import { Avatar, type MenuItem, MenuButton, modalOpen, useIsMobile, useMediaQuery } from './ui'
import { AskButton } from './AskButton'

export { announceChange, useOnChange } from '../lib/data'

const SIDEBAR_KEY = 'pmth-sidebar'

/** The page's own name for the top bar breadcrumb. */
const CrumbCtx = createContext<(t: string, to?: string) => void>(() => {})
export function useCrumb(title: string | undefined, parent?: string) {
  const set = useContext(CrumbCtx)
  useEffect(() => { set(title || '', parent); return () => set('') }, [title, parent, set])
}

/** Start a notebook from anywhere (a page's button), from a start. */
export const openCreate = (template?: string) => window.dispatchEvent(new CustomEvent('pmth:create', { detail: { template } }))

export default function Layout() {
  return <DataProvider><Shell /></DataProvider>
}

function Shell() {
  const navigate = useNavigate()
  const loc = useLocation()
  const mobile = useIsMobile()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const drawer = mobile || narrow
  const [sidebarOpen, setSidebarOpen] = useState(() => !drawer && localStorage.getItem(SIDEBAR_KEY) !== '0')
  useEffect(() => { setSidebarOpen(!drawer && localStorage.getItem(SIDEBAR_KEY) !== '0') }, [drawer])
  const drawerOpenedAt = useRef(0)
  const toggleSidebar = () => setSidebarOpen(o => { drawerOpenedAt.current = performance.now(); const n = !o; if (!drawer) localStorage.setItem(SIDEBAR_KEY, n ? '1' : '0'); return n })
  const closeDrawer = () => { if (drawer) setSidebarOpen(false) }
  useEffect(() => { document.querySelector('.content')?.scrollTo({ top: 0 }) }, [loc.pathname])

  const [crumb, setCrumbRaw] = useState<{ t: string; to?: string }>({ t: '' })
  const setCrumb = useCallback((t: string, to?: string) => setCrumbRaw({ t, to }), [])

  const [palette, setPalette] = useState(false)
  const [shortcuts, setShortcuts] = useState(false)

  const create = async (template?: string) => {
    closeDrawer()
    try {
      const r = await api.post<{ notebook: { id: number } }>(`${V1}/notebooks`, { template: template || 'blank' })
      announceChange()
      navigate(`/n/${r.notebook.id}`)
    } catch { navigate('/') }
  }
  const createRef = useRef(create)
  createRef.current = create

  useEffect(() => {
    const open = () => setShortcuts(true)
    const make = (e: Event) => createRef.current((e as CustomEvent<{ template?: string }>).detail.template)
    window.addEventListener('pmth:shortcuts', open)
    window.addEventListener('pmth:create', make)
    return () => { window.removeEventListener('pmth:shortcuts', open); window.removeEventListener('pmth:create', make) }
  }, [])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); setPalette(p => !p); return }
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (t.closest('input, textarea, select, [contenteditable="true"]') || modalOpen() || document.querySelector('.menu, .popover, .sel-pop')) return
      const go = (to: string) => { e.preventDefault(); navigate(to) }
      switch (e.key) {
        case '/': e.preventDefault(); setPalette(true); break
        case '?': e.preventDefault(); setShortcuts(true); break
        case 'n': e.preventDefault(); createRef.current(); break
        case 'h': go('/'); break
        case 'b': go('/notebooks'); break
        case 'c': go('/formulas'); break
        case 'v': go('/convert'); break
        case 'p': go('/practice'); break
        case ',': go('/settings'); break
        case '[': e.preventDefault(); toggleSidebar(); break
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const commands: PaletteCommand[] = [
    { id: 'new', label: 'New notebook', icon: 'plus', hint: 'N', keywords: 'start sums calculation', run: () => create() },
    { id: 'new-budget', label: 'New: monthly budget', icon: 'wallet', keywords: 'money spending income', run: () => create('budget') },
    { id: 'new-trip', label: 'New: trip costs', icon: 'car', keywords: 'fuel travel holiday', run: () => create('trip') },
    { id: 'new-room', label: 'New: doing up a room', icon: 'paint', keywords: 'paint floor renovation tiles', run: () => create('renovation') },
    { id: 'new-homework', label: 'New: homework', icon: 'school', keywords: 'school equation graph', run: () => create('homework') },
    { id: 'new-graph', label: 'New: graph', icon: 'graph', keywords: 'plot function desmos', run: () => create('graph') },
    { id: 'new-geo', label: 'New: geometry', icon: 'triangle', keywords: 'triangle circle construction geogebra', run: () => create('geometry') },
    { id: 'new-data', label: 'New: data and averages', icon: 'table', keywords: 'statistics mean trend regression chart', run: () => create('data') },
    { id: 'home', label: 'Start page', icon: 'home', hint: 'H', run: () => navigate('/') },
    { id: 'notebooks', label: 'Notebooks', icon: 'calc', hint: 'B', run: () => navigate('/notebooks') },
    { id: 'formulas', label: 'Ready calculations', icon: 'fx', hint: 'C', keywords: 'paint loan bmi fuel formula', run: () => navigate('/formulas') },
    { id: 'answers', label: 'Kept answers', icon: 'check-circle', run: () => navigate('/answers') },
    { id: 'convert', label: 'Converter', icon: 'convert', hint: 'V', keywords: 'units currency exchange money', run: () => navigate('/convert') },
    { id: 'practice', label: 'Practice', icon: 'abacus', hint: 'P', keywords: 'times tables kids children drill', run: () => navigate('/practice') },
    { id: 'reference', label: 'What can I write?', icon: 'help', keywords: 'help syntax functions units', run: () => navigate('/reference') },
    { id: 'archived', label: 'Archived notebooks', icon: 'archive', run: () => navigate('/notebooks?archived=1') },
    { id: 'settings', label: 'Settings', icon: 'settings', hint: ',', run: () => navigate('/settings') },
    { id: 'apps', label: 'Settings: connected apps', icon: 'link', keywords: 'rooms home garage health money wealth tables notes files photos chat learn slides habits family alerts core', run: () => navigate('/settings/apps') },
    { id: 'ai-set', label: 'Settings: AI', icon: 'sparkles', keywords: 'pocket ai openai', run: () => navigate('/settings/ai') },
    { id: 'export', label: 'Settings: export and import', icon: 'download', keywords: 'backup json csv', run: () => navigate('/settings/data') },
    { id: 'shortcuts', label: 'Keyboard shortcuts', icon: 'keyboard', hint: '?', run: () => setShortcuts(true) },
  ]

  return (
    <CrumbCtx.Provider value={setCrumb}>
      <div className={`app ${sidebarOpen ? 'with-sidebar' : 'no-sidebar'} ${mobile ? 'is-mobile' : ''} ${drawer ? 'drawer-mode' : ''}`}>
        {drawer && sidebarOpen && <div className="drawer-backdrop" onClick={e => { if (e.timeStamp - drawerOpenedAt.current > 400) setSidebarOpen(false) }} />}
        <div className={`sidebar-wrap ${sidebarOpen ? 'open' : ''}`}>
          <Sidebar onCreate={create} onNavigate={closeDrawer} />
        </div>
        <div className="main">
          <TopBar onMenu={toggleSidebar} onPalette={() => setPalette(true)} mobile={mobile} compact={narrow} sidebarOpen={sidebarOpen} crumb={crumb} />
          <main className="content"><div className="content-inner"><Outlet /></div></main>
        </div>
        {mobile && /^\/(|notebooks)$/.test(loc.pathname) && <button type="button" className="fab" aria-label="New notebook" title="New notebook" onClick={() => create()}><Icon name="plus" size={26} strokeWidth={2} /></button>}
        {palette && <CommandPalette onClose={() => setPalette(false)} commands={commands} />}
        {shortcuts && <ShortcutsSheet onClose={() => setShortcuts(false)} />}
      </div>
    </CrumbCtx.Provider>
  )
}

const SECTION: Record<string, [string, string]> = {
  '': ['Start', '/'], notebooks: ['Notebooks', '/notebooks'], n: ['Notebooks', '/notebooks'], formulas: ['Ready calculations', '/formulas'], f: ['Ready calculations', '/formulas'],
  answers: ['Kept answers', '/answers'], convert: ['Converter', '/convert'], practice: ['Practice', '/practice'], reference: ['What can I write?', '/reference'],
}

function TopBar({ onMenu, onPalette, mobile, compact, sidebarOpen, crumb }: { onMenu: () => void; onPalette: () => void; mobile: boolean; compact: boolean; sidebarOpen: boolean; crumb: { t: string; to?: string } }) {
  const { user, themeMode, setThemeMode, status, setUser } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [section, sectionTo] = SECTION[loc.pathname.split('/')[1] || ''] || ['', '/']
  const signOut = async () => { await api.post(`${V1}/auth/logout`).catch(() => {}); setUser(null); navigate('/login') }
  const avatarMenu = (): MenuItem[] => [
    { heading: user?.name || user?.email || '' },
    { label: 'Profile', icon: 'user', onSelect: () => navigate('/settings/profile') },
    { label: 'Settings', icon: 'settings', onSelect: () => navigate('/settings'), hint: ',' },
    { label: 'Keyboard shortcuts', icon: 'keyboard', onSelect: () => window.dispatchEvent(new CustomEvent('pmth:shortcuts')), hint: '?' },
    'sep',
    { heading: 'Appearance' },
    { label: 'System', checked: themeMode === 'system', onSelect: () => setThemeMode('system') },
    { label: 'Light', checked: themeMode === 'light', onSelect: () => setThemeMode('light') },
    { label: 'Dark', checked: themeMode === 'dark', onSelect: () => setThemeMode('dark') },
    ...(status?.auth_mode === 'local' ? ['sep' as const, { label: 'Sign out', icon: 'log-out', onSelect: signOut }] : []),
  ]
  return (
    <header className="topbar">
      <button type="button" className="btn ghost icon-only" onClick={onMenu} aria-label={sidebarOpen ? 'Hide the sidebar' : 'Show the sidebar'} title="Menu ([)"><Icon name={mobile || compact ? 'menu' : 'sidebar'} size={18} /></button>
      <h1 className="range-title">
        {crumb.t ? <><button type="button" className="crumb" onClick={() => navigate(crumb.to || sectionTo)}>{section}</button><Icon name="chevron-right" size={16} className="crumb-sep" /><span className="ellipsis">{crumb.t}</span></> : section}
      </h1>
      <span className="spacer" />
      <button type="button" className="btn ghost icon-only" onClick={onPalette} aria-label="Search" title="Search (⌘K or /)"><Icon name="search" size={18} /></button>
      <AskButton />
      <MenuButton label="Account" className="avatar-btn" align="right" width={220} items={avatarMenu}>
        <Avatar name={user?.name} email={user?.email} size={30} />
      </MenuButton>
    </header>
  )
}
