// README screenshots and a browser smoke test against a demo instance:
//
//   make build && DATA_DIR=/tmp/math-demo DEMO_DATA=true AUTH_MODE=none HOST=127.0.0.1 PORT=18272 ./pocket-math &
//   CDP=http://127.0.0.1:9340 BASE=http://127.0.0.1:18272 node scripts/screenshots.mjs   # any Chrome with remote debugging
//   SHOTS=0 … node scripts/screenshots.mjs                                                # checks only
// playwright-core comes from web/node_modules, or set PW_FROM to a package.json next to an install of it.
import { createRequire } from 'node:module'
import assert from 'node:assert/strict'

const require = createRequire(process.env.PW_FROM || new URL('../web/package.json', import.meta.url))
let pw
try { pw = require('playwright') } catch { pw = require('playwright-core') }
const { chromium } = pw
const base = process.env.BASE || 'http://127.0.0.1:18272'
const shots = process.env.SHOTS !== '0'
const only = process.env.ONLY || ''
const dir = process.env.OUT || new URL('../docs/screenshots/', import.meta.url).pathname
const browser = process.env.CDP ? await chromium.connectOverCDP(process.env.CDP) : await chromium.launch()
const errors = []

async function session({ phone = false, dark = false } = {}) {
  const ctx = await browser.newContext({
    viewport: phone ? { width: 390, height: 844 } : { width: 1440, height: 900 },
    deviceScaleFactor: 2, isMobile: phone, hasTouch: phone, colorScheme: dark ? 'dark' : 'light',
  })
  const page = await ctx.newPage()
  page.on('pageerror', e => errors.push(`pageerror: ${e}`))
  page.on('console', m => { if (m.type() === 'error' && !/Failed to load resource/.test(m.text())) errors.push(m.text()) })
  return { ctx, page }
}
async function go(page, path, wait = 1500) {
  await page.goto(base + path, { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(wait)
}
async function shot(page, name) {
  if (!shots || (only && !only.split(',').includes(name))) return
  for (let i = 0; ; i++) {
    try { await page.bringToFront(); await page.screenshot({ path: dir + name + '.png', animations: 'disabled' }); return } catch (e) { if (i >= 3) throw e; await page.waitForTimeout(700) }
  }
}
const api = (page, path, method = 'GET', body) => page.evaluate(([p, m, b]) => fetch('/api/v1' + p, { method: m, headers: { 'Content-Type': 'application/json' }, body: b ? JSON.stringify(b) : undefined }).then(r => r.status === 204 ? {} : r.json()), [path, method, body])
const scrollTo = (page, sel) => page.evaluate(s => document.querySelector(s)?.scrollIntoView({ block: 'start' }), sel)

const { page } = await session()
await go(page, '/', 2000)
const nbs = (await api(page, '/notebooks')).notebooks
const nb = t => nbs.find(n => n.title.startsWith(t)).id
assert.ok((await page.content()).includes('Monthly budget'), 'the demo notebooks show on the start page')

// the quick sum answers as you type
await page.fill('.quick-in', '5 km + 300 m')
await page.waitForTimeout(200)
assert.equal((await page.textContent('.quick-out')).trim().slice(0, 6), '5.3 km', 'the quick sum')
await page.fill('.quick-in', '')
await shot(page, 'desktop-home')

await go(page, `/n/${nb('Monthly budget')}`)
assert.ok((await page.content()).includes('3,050.00 BAM'), 'the budget adds up')
await shot(page, 'desktop-notebook')

// typing into a calculation: the answer appears, then it is saved
await page.click('.cell-calc .calc-in >> nth=2')
await page.keyboard.press('End')
await page.keyboard.press('Control+End')
await page.keyboard.type('\nleft in EUR')
await page.waitForTimeout(1600)
assert.ok((await page.content()).includes('331.83 EUR'), 'a new line is worked out at once')
const saved = await api(page, `/notebooks/${nb('Monthly budget')}`)
assert.ok(saved.cells.some(c => c.body.includes('left in EUR')), 'the change is saved')

await go(page, `/n/${nb('Physics')}`)
await scrollTo(page, '.cell-graph')
await shot(page, 'desktop-graph')
await go(page, `/n/${nb('Triangle')}`)
await scrollTo(page, '.cell-geometry')
await shot(page, 'desktop-geometry')
await go(page, `/n/${nb('Electricity')}`)
await scrollTo(page, '.data-stats')
await shot(page, 'desktop-data')
await go(page, '/f/paint')
await page.fill('.f-form input >> nth=0', '45')
await page.waitForTimeout(300)
assert.equal((await page.textContent('.f-big')).trim(), '9 L', 'a ready calculation answers at once')
await shot(page, 'desktop-formula')
await go(page, '/practice?learner=Mila')
await shot(page, 'desktop-practice')
await go(page, '/convert')
await page.fill('.conv-in', '12')
await page.waitForTimeout(200)
assert.equal((await page.textContent('.conv-out')).trim(), '0.012 m', 'the converter')

// phone
const ph = (await session({ phone: true })).page
await go(ph, '/', 2000)
await ph.fill('.quick-in', '100 EUR in BAM')
await ph.waitForTimeout(200)
await shot(ph, 'phone-home')
await go(ph, `/n/${nb('Kitchen')}`)
await shot(ph, 'phone-notebook')
await go(ph, '/practice?learner=Luka')
await ph.click('text=Times tables')
await ph.click('.start-round .btn.primary')
await ph.waitForTimeout(500)
await shot(ph, 'phone-drill')
const q = (await ph.textContent('.drill-q')).trim()
const [a, b] = q.split('×').map(s => parseInt(s, 10))
await ph.fill('.drill-in', String(a * b))
await ph.keyboard.press('Enter')
await ph.waitForTimeout(700)
assert.ok((await ph.textContent('.drill-progress')).startsWith('2 /'), 'a right answer moves on')

assert.deepEqual(errors, [], 'no errors in the browser')
console.log('screenshots and checks done')
process.exit(0)
