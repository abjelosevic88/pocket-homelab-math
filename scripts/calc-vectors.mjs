// Runs the calculator test vectors (internal/calc/testdata/vectors.json)
// against the browser engine (web/src/lib/calc.ts), so Go and the web app
// answer the same. Usage: node scripts/calc-vectors.mjs
import { build } from '../web/node_modules/esbuild/lib/main.js'
import { readFileSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const out = join(mkdtempSync(join(tmpdir(), 'calc-')), 'calc.mjs')
await build({ entryPoints: [join(root, 'web/src/lib/calc.ts')], bundle: true, format: 'esm', platform: 'node', outfile: out, logLevel: 'error' })
const calc = await import(out)
const vectors = JSON.parse(readFileSync(join(root, 'internal/calc/testdata/vectors.json'), 'utf8'))
const rates = { EUR: 1, BAM: 1.95583, USD: 1.1, GBP: 0.85 }
const today = new Date(2026, 10, 11, 12)
let bad = 0
for (const v of vectors) {
  const [lines] = calc.evalSheet(v.lines, { units: true, deg: !!v.deg, rates, today, format: { group: ',' } })
  lines.forEach((ln, i) => {
    const fail = (msg) => { bad++; console.log(`✗ ${JSON.stringify(v.lines)} line ${i}: ${msg}`) }
    if (v.error && v.error[i]) { if (ln.kind !== 'error') fail(`want an error, got ${ln.text}`); return }
    if (ln.kind === 'error') return fail(ln.error)
    if (v.want && i < v.want.length && (ln.text || '') !== v.want[i]) fail(`got ${JSON.stringify(ln.text)} want ${JSON.stringify(v.want[i])}`)
    if (v.alt && i < v.alt.length && (ln.alt || '') !== v.alt[i]) fail(`alt ${JSON.stringify(ln.alt)} want ${JSON.stringify(v.alt[i])}`)
    if (v.kind && i < v.kind.length && ln.kind !== v.kind[i]) fail(`kind ${ln.kind} want ${v.kind[i]}`)
  })
}
writeFileSync(out, '')
if (bad) { console.log(`${bad} failed`); process.exit(1) }
console.log(`all ${vectors.length} calculator vectors pass in the browser engine`)
