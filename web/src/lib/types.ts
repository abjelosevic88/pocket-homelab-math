// The shapes the API sends.
import type { Line } from './calc'

export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = 'core' | 'direct' | ''
export type AppSlug = 'notes' | 'files' | 'photos' | 'chat' | 'tables' | 'learn' | 'alerts' | 'rooms' | 'home' | 'garage' | 'health' | 'money' | 'wealth' | 'slides' | 'habits' | 'family'
type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  angle: 'deg' | 'rad'; units: boolean; digits: number; number_fmt: string; currency: string; rates: 'ecb' | 'off'
  graph_grid: boolean; graph_rad: boolean; show_alt: boolean
  daily_goal: number; round_size: number; practice_habit: number; learners: string[]
  notes_folder: string; files_folder: string; chat_to: string; learn_deck: string; alerts_goal: boolean; ai_help: boolean
  notify_practice: boolean; notify_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; demo: boolean
}

export type AppStateName = 'on' | 'partly' | 'off' | 'link_off' | 'missing' | 'old' | 'no_core' | 'not_set_up' | 'error'
export interface AppState { slug: AppSlug; name: string; reason: string; mode: ConnMode; via: ConnVia; state: AppStateName; detail: string; fix: string; on: string[]; open_url: string }

export type CellKind = 'calc' | 'text' | 'graph' | 'geometry' | 'data' | 'solve'

export interface Notebook {
  id: number; title: string; subject: string; about: string; color: string; angle: 'deg' | 'rad'; units: boolean; tags: string[]
  pinned: boolean; archived: boolean; shared: boolean; share_key: string; revision: number; source: string; source_ref: string
  opened_at: string | null; created_at: string; updated_at: string
  cells: number; kinds: CellKind[]; preview: string
}

export interface Cell {
  id: number; notebook_id: number; position: number; kind: CellKind; title: string; body: string
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  data: any; results: Line[]; summary: string; collapsed: boolean; source: string; created_at: string; updated_at: string
}

export interface NotebookFull { notebook: Notebook; cells: Cell[]; url: string; share_url: string }

export interface FInput { name: string; label: string; unit: string; default: string; help?: string; kind?: '' | 'number' | 'percent' | 'date' | 'choice'; options?: string; from?: string }
export interface FOutput { label: string; expr: string; unit: string; decimals: number | null; main: boolean }
export interface Formula {
  id: number; key?: string; name: string; about: string; category: string; icon: string; code: string
  inputs: FInput[]; outputs: FOutput[]; from_key: string; pinned: boolean; builtin: boolean; source: string
  created_at: string; updated_at: string; runs: number; last_run: string | null
}
export interface OutputResult { label: string; text: string; number?: number; unit?: string; error?: string; main?: boolean }
export interface RunResult { outputs: OutputResult[]; lines: Line[]; error?: string }
export interface Run {
  id: number; formula_id: number | null; formula_key: string; formula_name: string; title: string
  inputs: Record<string, string>; outputs: OutputResult[]; main: string; note: string; source: string; created_at: string
}

export interface Practice { id: number; learner: string; skill: string; level: number; questions: number; correct: number; seconds: number; details: PracticeQ[]; day: string; created_at: string }
export interface PracticeQ { q: string; answer: string; given: string; right: boolean; seconds: number }
export interface LearnerStats {
  learner: string; today: number; goal: number; streak: number; rounds: number; questions: number; correct: number
  days: { day: string; questions: number; correct: number }[] | null
  skills: { skill: string; questions: number; correct: number; pct: number; seconds_each: number }[] | null
  last: Practice | null
}

export interface Template { key: string; name: string; about: string; icon: string }
export interface Home {
  notebooks: Notebook[]; pinned: Formula[]; favourites: Formula[]; runs: Run[]; practice: LearnerStats[]
  counts: Record<string, number>; templates: Template[]
}

export interface Column { name: string; values: string[] }
