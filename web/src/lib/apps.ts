// The other Pocket apps Math works with.
import type { AppSlug, AppStateName } from './types'

export const APP_INFO: Record<AppSlug, { name: string; icon: string }> = {
  notes: { name: 'Pocket Notes', icon: 'note' }, files: { name: 'Pocket Files', icon: 'archive' }, photos: { name: 'Pocket Photos', icon: 'image' },
  chat: { name: 'Pocket Chat', icon: 'message' }, tables: { name: 'Pocket Tables', icon: 'table' }, learn: { name: 'Pocket Learn', icon: 'brain' },
  alerts: { name: 'Pocket Alerts', icon: 'bell' }, rooms: { name: 'Pocket Rooms', icon: 'door' }, home: { name: 'Pocket Home', icon: 'home' },
  garage: { name: 'Pocket Garage', icon: 'car' }, health: { name: 'Pocket Health', icon: 'heart-pulse' }, money: { name: 'Pocket Money', icon: 'wallet' },
  wealth: { name: 'Pocket Wealth', icon: 'chart' }, slides: { name: 'Pocket Slides', icon: 'presentation' }, habits: { name: 'Pocket Habits', icon: 'habit' },
  family: { name: 'Pocket Family', icon: 'users' },
}

export const STATE_WORDS: Record<AppStateName, string> = {
  on: 'Connected', partly: 'Partly connected', off: 'Off', link_off: 'Link is off in Pocket Core', missing: 'Not on this box', old: 'Needs an update',
  no_core: 'No Pocket Core', not_set_up: 'Not set up', error: 'Not working',
}
