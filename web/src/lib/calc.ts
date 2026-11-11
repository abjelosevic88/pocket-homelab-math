// The calculator, in the browser: the same rules as internal/calc in Go
// (both are checked against internal/calc/testdata/vectors.json), so the
// answers appear as you type and graphs can be drawn fast.

// ---------- units ----------

type Dim = number[] // length, mass, time, current, temperature, amount, light, data, money
const D = (pairs: [number, number][]): Dim => { const d = [0, 0, 0, 0, 0, 0, 0, 0, 0]; for (const [i, p] of pairs) d[i] = p; return d }
const [dL, dM, dT, dI, dK, dN, dJ, dB, dC] = [0, 1, 2, 3, 4, 5, 6, 7, 8]
const dimAdd = (a: Dim, b: Dim, k: number): Dim => a.map((x, i) => x + b[i] * k)
const dimZero = (d: Dim) => d.every(x => x === 0)
const dimEq = (a: Dim, b: Dim) => a.every((x, i) => x === b[i])

interface UnitDef { sym: string; plural?: string; factor: number; dim: Dim; offset?: number; prefix?: boolean; kind: string; cur?: string }

const length = D([[dL, 1]]), mass = D([[dM, 1]]), tm = D([[dT, 1]]), area = D([[dL, 2]]), volume = D([[dL, 3]])
const speed = D([[dL, 1], [dT, -1]]), force = D([[dM, 1], [dL, 1], [dT, -2]]), energy = D([[dM, 1], [dL, 2], [dT, -2]])
const power = D([[dM, 1], [dL, 2], [dT, -3]]), pressure = D([[dM, 1], [dL, -1], [dT, -2]]), current = D([[dI, 1]])
const voltage = D([[dM, 1], [dL, 2], [dT, -3], [dI, -1]]), resist = D([[dM, 1], [dL, 2], [dT, -3], [dI, -2]]), charge = D([[dI, 1], [dT, 1]])
const temp = D([[dK, 1]]), freq = D([[dT, -1]]), data = D([[dB, 1]]), amount = D([[dN, 1]]), light = D([[dJ, 1]]), money = D([[dC, 1]]), none = D([])
const DAY = 86400, YEAR = 365.2425 * DAY, MONTH = YEAR / 12, USGAL = 3.785411784e-3

const unitTable: [string[], UnitDef][] = [
  [['m', 'meter', 'meters', 'metre', 'metres'], { sym: 'm', factor: 1, dim: length, prefix: true, kind: 'length' }],
  [['in', 'inch', 'inches', '″'], { sym: 'in', factor: 0.0254, dim: length, kind: 'length' }],
  [['ft', 'foot', 'feet', '′'], { sym: 'ft', factor: 0.3048, dim: length, kind: 'length' }],
  [['yd', 'yard', 'yards'], { sym: 'yd', factor: 0.9144, dim: length, kind: 'length' }],
  [['mi', 'mile', 'miles'], { sym: 'mi', factor: 1609.344, dim: length, kind: 'length' }],
  [['nmi', 'nauticalmile', 'nauticalmiles'], { sym: 'nmi', factor: 1852, dim: length, kind: 'length' }],
  [['au'], { sym: 'au', factor: 149597870700, dim: length, kind: 'length' }],
  [['ly', 'lightyear', 'lightyears'], { sym: 'ly', factor: 9460730472580800, dim: length, kind: 'length' }],
  [['g', 'gram', 'grams', 'gramme', 'grammes'], { sym: 'g', factor: 1e-3, dim: mass, prefix: true, kind: 'mass' }],
  [['t', 'tonne', 'tonnes', 'ton', 'tons'], { sym: 't', factor: 1000, dim: mass, kind: 'mass' }],
  [['lb', 'lbs', 'pound', 'pounds'], { sym: 'lb', factor: 0.45359237, dim: mass, kind: 'mass' }],
  [['oz', 'ounce', 'ounces'], { sym: 'oz', factor: 0.028349523125, dim: mass, kind: 'mass' }],
  [['st', 'stone', 'stones'], { sym: 'st', factor: 6.35029318, dim: mass, kind: 'mass' }],
  [['ct', 'carat', 'carats'], { sym: 'ct', factor: 0.0002, dim: mass, kind: 'mass' }],
  [['s', 'sec', 'secs', 'second', 'seconds'], { sym: 's', factor: 1, dim: tm, prefix: true, kind: 'time' }],
  [['min', 'mins', 'minute', 'minutes'], { sym: 'min', factor: 60, dim: tm, kind: 'time' }],
  [['h', 'hr', 'hrs', 'hour', 'hours'], { sym: 'h', factor: 3600, dim: tm, kind: 'time' }],
  [['day', 'days'], { sym: 'day', plural: 'days', factor: DAY, dim: tm, kind: 'time' }],
  [['week', 'weeks', 'wk', 'wks'], { sym: 'week', plural: 'weeks', factor: 7 * DAY, dim: tm, kind: 'time' }],
  [['month', 'months', 'mo'], { sym: 'month', plural: 'months', factor: MONTH, dim: tm, kind: 'time' }],
  [['year', 'years', 'yr', 'yrs'], { sym: 'year', plural: 'years', factor: YEAR, dim: tm, kind: 'time' }],
  [['ha', 'hectare', 'hectares'], { sym: 'ha', factor: 1e4, dim: area, kind: 'area' }],
  [['acre', 'acres', 'ac'], { sym: 'acre', plural: 'acres', factor: 4046.8564224, dim: area, kind: 'area' }],
  [['sqm'], { sym: 'm²', factor: 1, dim: area, kind: 'area' }],
  [['sqft'], { sym: 'ft²', factor: 0.09290304, dim: area, kind: 'area' }],
  [['sqkm'], { sym: 'km²', factor: 1e6, dim: area, kind: 'area' }],
  [['dunum', 'dunam', 'dulum'], { sym: 'dunum', factor: 1000, dim: area, kind: 'area' }],
  [['L', 'l', 'liter', 'liters', 'litre', 'litres'], { sym: 'L', factor: 1e-3, dim: volume, prefix: true, kind: 'volume' }],
  [['gal', 'gallon', 'gallons'], { sym: 'gal', factor: USGAL, dim: volume, kind: 'volume' }],
  [['qt', 'quart', 'quarts'], { sym: 'qt', factor: USGAL / 4, dim: volume, kind: 'volume' }],
  [['pt', 'pint', 'pints'], { sym: 'pt', factor: USGAL / 8, dim: volume, kind: 'volume' }],
  [['cup', 'cups'], { sym: 'cup', plural: 'cups', factor: USGAL / 16, dim: volume, kind: 'volume' }],
  [['floz'], { sym: 'fl oz', factor: USGAL / 128, dim: volume, kind: 'volume' }],
  [['tbsp', 'tablespoon', 'tablespoons'], { sym: 'tbsp', factor: 15e-6, dim: volume, kind: 'volume' }],
  [['tsp', 'teaspoon', 'teaspoons'], { sym: 'tsp', factor: 5e-6, dim: volume, kind: 'volume' }],
  [['cc'], { sym: 'cc', factor: 1e-6, dim: volume, kind: 'volume' }],
  [['mph'], { sym: 'mph', factor: 0.44704, dim: speed, kind: 'speed' }],
  [['kph', 'kmh'], { sym: 'km/h', factor: 1 / 3.6, dim: speed, kind: 'speed' }],
  [['kn', 'knot', 'knots'], { sym: 'kn', factor: 1852 / 3600, dim: speed, kind: 'speed' }],
  [['mpg'], { sym: 'mpg', factor: 1609.344 / USGAL, dim: D([[dL, -2]]), kind: 'fuel' }],
  [['N', 'newton', 'newtons'], { sym: 'N', factor: 1, dim: force, prefix: true, kind: 'force' }],
  [['lbf'], { sym: 'lbf', factor: 4.4482216152605, dim: force, kind: 'force' }],
  [['J', 'joule', 'joules'], { sym: 'J', factor: 1, dim: energy, prefix: true, kind: 'energy' }],
  [['cal', 'calorie', 'calories'], { sym: 'cal', factor: 4.184, dim: energy, prefix: true, kind: 'energy' }],
  [['Wh'], { sym: 'Wh', factor: 3600, dim: energy, prefix: true, kind: 'energy' }],
  [['eV'], { sym: 'eV', factor: 1.602176634e-19, dim: energy, prefix: true, kind: 'energy' }],
  [['BTU', 'btu'], { sym: 'BTU', factor: 1055.05585262, dim: energy, kind: 'energy' }],
  [['W', 'watt', 'watts'], { sym: 'W', factor: 1, dim: power, prefix: true, kind: 'power' }],
  [['hp', 'horsepower'], { sym: 'hp', factor: 745.69987158227, dim: power, kind: 'power' }],
  [['PS'], { sym: 'PS', factor: 735.49875, dim: power, kind: 'power' }],
  [['Pa', 'pascal', 'pascals'], { sym: 'Pa', factor: 1, dim: pressure, prefix: true, kind: 'pressure' }],
  [['bar'], { sym: 'bar', factor: 1e5, dim: pressure, prefix: true, kind: 'pressure' }],
  [['atm'], { sym: 'atm', factor: 101325, dim: pressure, kind: 'pressure' }],
  [['psi'], { sym: 'psi', factor: 6894.757293168, dim: pressure, kind: 'pressure' }],
  [['mmHg'], { sym: 'mmHg', factor: 133.322387415, dim: pressure, kind: 'pressure' }],
  [['A', 'amp', 'amps', 'ampere', 'amperes'], { sym: 'A', factor: 1, dim: current, prefix: true, kind: 'current' }],
  [['V', 'volt', 'volts'], { sym: 'V', factor: 1, dim: voltage, prefix: true, kind: 'voltage' }],
  [['Ω', 'ohm', 'ohms'], { sym: 'Ω', factor: 1, dim: resist, prefix: true, kind: 'resistance' }],
  [['Ah'], { sym: 'Ah', factor: 3600, dim: charge, prefix: true, kind: 'charge' }],
  [['Hz', 'hertz'], { sym: 'Hz', factor: 1, dim: freq, prefix: true, kind: 'frequency' }],
  [['rpm'], { sym: 'rpm', factor: 1 / 60, dim: freq, kind: 'frequency' }],
  [['K', 'kelvin'], { sym: 'K', factor: 1, dim: temp, kind: 'temperature' }],
  [['°C', 'degC', 'celsius', 'Celsius'], { sym: '°C', factor: 1, offset: 273.15, dim: temp, kind: 'temperature' }],
  [['°F', 'degF', 'fahrenheit', 'Fahrenheit'], { sym: '°F', factor: 5 / 9, offset: 459.67, dim: temp, kind: 'temperature' }],
  [['bit', 'bits'], { sym: 'bit', plural: 'bits', factor: 1, dim: data, prefix: true, kind: 'data' }],
  [['B', 'byte', 'bytes'], { sym: 'B', factor: 8, dim: data, prefix: true, kind: 'data' }],
  [['KiB'], { sym: 'KiB', factor: 8 * 1024, dim: data, kind: 'data' }],
  [['MiB'], { sym: 'MiB', factor: 8 * 1024 ** 2, dim: data, kind: 'data' }],
  [['GiB'], { sym: 'GiB', factor: 8 * 1024 ** 3, dim: data, kind: 'data' }],
  [['TiB'], { sym: 'TiB', factor: 8 * 1024 ** 4, dim: data, kind: 'data' }],
  [['rad', 'radian', 'radians'], { sym: 'rad', factor: 1, dim: none, kind: 'angle' }],
  [['deg', 'degree', 'degrees', '°'], { sym: '°', factor: Math.PI / 180, dim: none, kind: 'angle' }],
  [['turn', 'turns', 'rev'], { sym: 'turn', plural: 'turns', factor: 2 * Math.PI, dim: none, kind: 'angle' }],
  [['mol'], { sym: 'mol', factor: 1, dim: amount, prefix: true, kind: 'amount' }],
  [['cd', 'candela'], { sym: 'cd', factor: 1, dim: light, kind: 'light' }],
]
const prefixes: [string, number][] = [['da', 1e1], ['Y', 1e24], ['Z', 1e21], ['E', 1e18], ['P', 1e15], ['T', 1e12], ['G', 1e9], ['M', 1e6], ['k', 1e3], ['h', 1e2],
  ['d', 1e-1], ['c', 1e-2], ['m', 1e-3], ['µ', 1e-6], ['μ', 1e-6], ['u', 1e-6], ['n', 1e-9], ['p', 1e-12], ['f', 1e-15]]
const currencyAliases: Record<string, string> = {
  '€': 'EUR', '$': 'USD', '£': 'GBP', '¥': 'JPY', '₹': 'INR', '₽': 'RUB', '₺': 'TRY', '₩': 'KRW', '₣': 'CHF',
  KM: 'BAM', euro: 'EUR', euros: 'EUR', dollar: 'USD', dollars: 'USD', marka: 'BAM', marke: 'BAM', maraka: 'BAM', dinar: 'RSD', dinara: 'RSD', kn: '',
}
const unitIndex = new Map<string, UnitDef>()
for (const [names, def] of unitTable) for (const n of names) unitIndex.set(n, def)

function lookupUnit(name: string): UnitDef | undefined {
  const u = unitIndex.get(name)
  if (u) return u
  for (const [p, f] of prefixes) {
    if (!name.startsWith(p) || name.length === p.length) continue
    const baseName = name.slice(p.length)
    const base = unitIndex.get(baseName)
    if (!base || !base.prefix || base.sym !== baseName) continue
    return { ...base, sym: name, plural: undefined, factor: base.factor * f }
  }
  return undefined
}

export type UnitPow = { name: string; pow: number }
export type Units = UnitPow[]

export const converterKinds: { kind: string; units: string[] }[] = [
  { kind: 'length', units: ['mm', 'cm', 'm', 'km', 'in', 'ft', 'yd', 'mi', 'nmi'] },
  { kind: 'mass', units: ['mg', 'g', 'kg', 't', 'oz', 'lb', 'st', 'ct'] },
  { kind: 'temperature', units: ['°C', '°F', 'K'] },
  { kind: 'area', units: ['cm²', 'm²', 'km²', 'ha', 'dunum', 'in²', 'ft²', 'acre'] },
  { kind: 'volume', units: ['mL', 'L', 'm³', 'tsp', 'tbsp', 'cup', 'floz', 'pt', 'qt', 'gal'] },
  { kind: 'speed', units: ['m/s', 'km/h', 'mph', 'kn'] },
  { kind: 'time', units: ['ms', 's', 'min', 'h', 'day', 'week', 'month', 'year'] },
  { kind: 'energy', units: ['J', 'kJ', 'cal', 'kcal', 'Wh', 'kWh', 'BTU'] },
  { kind: 'power', units: ['W', 'kW', 'hp', 'PS'] },
  { kind: 'pressure', units: ['Pa', 'hPa', 'kPa', 'bar', 'mbar', 'atm', 'psi', 'mmHg'] },
  { kind: 'data', units: ['bit', 'B', 'kB', 'MB', 'GB', 'TB', 'KiB', 'MiB', 'GiB'] },
  { kind: 'angle', units: ['°', 'rad', 'turn'] },
  { kind: 'fuel', units: ['L/100km', 'km/L', 'mpg'] },
]

export function unitNames(): string[] { return [...unitIndex.keys()].sort() }

// ---------- lexer ----------

type TK = 'eof' | 'num' | 'id' | 'op' | 'date' | 'cur'
interface Tok { kind: TK; text: string; num: number; pos: number }
export class CalcError extends Error { pos: number; constructor(pos: number, msg: string) { super(msg); this.pos = pos } }
const sup: Record<string, string> = { '⁰': '0', '¹': '1', '²': '2', '³': '3', '⁴': '4', '⁵': '5', '⁶': '6', '⁷': '7', '⁸': '8', '⁹': '9', '⁻': '-' }
const isDigit = (c: string) => c >= '0' && c <= '9'
const isLetter = (c: string) => /\p{L}/u.test(c)
const isIdentStart = (c: string) => isLetter(c) || c === '_' || c === '°' || c === 'µ' || c === 'Ω' || c === '∞'
const isIdentPart = (c: string) => isIdentStart(c) || /\p{Nd}/u.test(c)
const isDateAt = (s: string, i: number) => /^\d{4}-\d{2}-\d{2}$/.test(s.slice(i, i + 10)) && !(i + 10 < s.length && isDigit(s[i + 10]))

function lex(src: string): Tok[] {
  const out: Tok[] = []
  // positions are byte offsets in Go; here they are UTF-16 offsets — only used to point at mistakes
  let i = 0
  const chars = src
  while (i < chars.length) {
    const cp = chars.codePointAt(i)!
    const c = String.fromCodePoint(cp)
    const w = c.length
    const start = i
    if (c === ' ' || c === '\t' || c === ' ' || c === ' ' || c === ' ') { i += w; continue }
    if (isDigit(c) || (c === '.' && i + 1 < chars.length && isDigit(chars[i + 1]))) {
      if (isDateAt(chars, i)) { out.push({ kind: 'date', text: chars.slice(i, i + 10), num: 0, pos: start }); i += 10; continue }
      let j = i
      const pre = chars.slice(i, i + 2)
      if (pre === '0x' || pre === '0b' || pre === '0o') {
        const base = pre === '0x' ? 16 : pre === '0b' ? 2 : 8
        j = i + 2
        while (j < chars.length && ((base === 16 && /[0-9a-fA-F]/.test(chars[j])) || /[01]/.test(chars[j]) || (base === 8 && /[0-7]/.test(chars[j])) || chars[j] === '_')) j++
        let n = 0
        for (const d of chars.slice(i + 2, j).replace(/_/g, '')) n = n * base + parseInt(d, 16)
        out.push({ kind: 'num', text: chars.slice(i, j), num: n, pos: start }); i = j; continue
      }
      while (j < chars.length && (isDigit(chars[j]) || chars[j] === '_' || chars[j] === '.')) j++
      if (j < chars.length && (chars[j] === 'e' || chars[j] === 'E')) {
        let k = j + 1
        if (k < chars.length && (chars[k] === '+' || chars[k] === '-')) k++
        if (k < chars.length && isDigit(chars[k])) { while (k < chars.length && isDigit(chars[k])) k++; j = k }
      }
      const txt = chars.slice(i, j).replace(/_/g, '')
      const n = Number(txt)
      if (!isFinite(n) && txt !== 'Infinity' || (txt.split('.').length > 2) || isNaN(n)) throw new CalcError(start, 'this number does not look right: ' + chars.slice(i, j))
      out.push({ kind: 'num', text: txt, num: n, pos: start }); i = j; continue
    }
    if (sup[c]) {
      let j = i, s = ''
      while (j < chars.length && sup[chars[j]]) { s += sup[chars[j]]; j++ }
      out.push({ kind: 'op', text: '^', num: 0, pos: start }, { kind: 'num', text: s, num: Number(s), pos: start }); i = j; continue
    }
    if (c === '°' && i + 1 < chars.length && (chars[i + 1] === 'C' || chars[i + 1] === 'F') && (i + 2 === chars.length || !isIdentPart(chars[i + 2]))) {
      out.push({ kind: 'id', text: chars.slice(i, i + 2), num: 0, pos: start }); i += 2; continue
    }
    if (isIdentStart(c)) {
      let j = i + w
      while (j < chars.length) {
        const c2 = String.fromCodePoint(chars.codePointAt(j)!)
        if (!isIdentPart(c2) || c2 === '°') break
        j += c2.length
      }
      out.push({ kind: 'id', text: chars.slice(i, j), num: 0, pos: start }); i = j; continue
    }
    if ('$€£¥₹₽₺₩'.includes(c)) { out.push({ kind: 'cur', text: c, num: 0, pos: start }); i += w; continue }
    if (c === '#') break
    if (c === '/' && chars[i + 1] === '/') break
    let op = c
    let ww = w
    switch (c) {
      case '×': case '·': case '⋅': op = '*'; break
      case '÷': case '∕': op = '/'; break
      case '−': case '–': op = '-'; break
      case '→': op = 'to'; break
      case '≤': op = '<='; break
      case '≥': op = '>='; break
      case '√': out.push({ kind: 'id', text: 'sqrt', num: 0, pos: start }); i += w; continue
      case 'π': out.push({ kind: 'id', text: 'pi', num: 0, pos: start }); i += w; continue
      case '*': if (chars[i + 1] === '*') { op = '^'; ww = 2 } break
      case '<': case '>': case '=': case '!': if (chars[i + 1] === '=') { op = chars.slice(i, i + 2); ww = 2 } break
    }
    if (op === 'to') { out.push({ kind: 'id', text: 'to', num: 0, pos: start }); i += ww; continue }
    if (!'+-*/^()[],=!%\'|<><=>=!=;{}:'.includes(op)) throw new CalcError(start, `I don't know the sign "${op}"`)
    out.push({ kind: 'op', text: op, num: 0, pos: start }); i += ww
  }
  out.push({ kind: 'eof', text: '', num: 0, pos: chars.length })
  return out
}

// ---------- parser ----------

export interface Node { op: string; num?: number; name?: string; units?: Units; scale?: number; kids?: Node[]; pos: number }
export interface Stmt { kind: 'expr' | 'assign' | 'func' | 'empty'; label: string; name?: string; params?: string[]; expr?: Node }

const keywords = new Set(['to', 'as', 'of', 'off', 'on', 'per', 'mod', 'into'])
const specialTargets: Record<string, string> = {
  hex: 'hex', hexadecimal: 'hex', bin: 'bin', binary: 'bin', oct: 'oct', octal: 'oct', sci: 'sci', scientific: 'sci', fraction: 'frac', frac: 'frac', fractions: 'frac',
  percent: 'pct', percentage: 'pct', dec: 'dec', decimal: 'dec', number: 'dec', base: 'base',
}
const prefixFuncs = new Set(['sqrt', 'cbrt', 'sin', 'cos', 'tan', 'ln', 'log', 'exp', 'abs'])
const labelRe = /^\s*(\p{L}[\p{L}\p{N} _'’&/(),.-]*?)\s*:\s*(.*)$/u
const isoCodes = new Set(`EUR USD GBP CHF BAM RSD HRK HUF CZK PLN RON BGN DKK SEK NOK ISK TRY RUB UAH MKD ALL
JPY CNY HKD KRW INR IDR MYR PHP SGD THB AUD NZD CAD MXN BRL ZAR ILS AED SAR BTC`.split(/\s+/))

function isCurrency(name: string, rates?: (c: string) => boolean): boolean {
  const a = currencyAliases[name]
  if (a !== undefined && a !== '') return true
  if (name.length !== 3 || name.toUpperCase() !== name) return false
  return isoCodes.has(name) || !!rates?.(name)
}
const canonicalCurrency = (n: string) => { const a = currencyAliases[n]; return a ? a : n }

export function splitLabel(line: string): [string, string] {
  const m = labelRe.exec(line)
  return m ? [m[1], m[2]] : ['', line]
}

class Parser {
  toks: Tok[]; i = 0; units: boolean; known: (n: string) => boolean; rates: (c: string) => boolean
  constructor(toks: Tok[], units: boolean, known: (n: string) => boolean, rates: (c: string) => boolean) { this.toks = toks; this.units = units; this.known = known; this.rates = rates }
  peek() { return this.toks[this.i] }
  peekN(n: number) { return this.toks[Math.min(this.i + n, this.toks.length - 1)] }
  next() { return this.toks[this.i++] }
  isOp(s: string) { const t = this.peek(); return t.kind === 'op' && t.text === s }
  end() {
    const t = this.peek()
    if (t.kind === 'eof') return
    if (t.kind === 'op' && t.text === ')') throw new CalcError(t.pos, 'there is a ) without its (')
    throw new CalcError(t.pos, `I don't understand "${t.text}" here`)
  }
  equation(): Node {
    const l = this.expr()
    const t = this.peek()
    if (t.kind === 'op') {
      const op = ({ '=': 'eq', '==': 'eq', '<': 'lt', '>': 'gt', '<=': 'le', '>=': 'ge', '!=': 'ne' } as Record<string, string>)[t.text]
      if (op) { this.next(); const r = this.expr(); return { op, kids: [l, r], pos: t.pos } }
    }
    return l
  }
  inIsConversion() {
    const n = this.peekN(1)
    if (n.kind === 'id' && keywords.has(n.text)) return false
    return n.kind === 'id' || n.kind === 'cur' || (n.kind === 'op' && n.text === '%')
  }
  expr(): Node {
    let n = this.sum()
    for (;;) {
      const t = this.peek()
      if (t.kind !== 'id' || !(t.text === 'to' || t.text === 'as' || t.text === 'into' || (t.text === 'in' && this.inIsConversion()))) return n
      this.next()
      if (this.peek().kind === 'id') {
        const sp = specialTargets[this.peek().text]
        if (sp && sp !== 'base') { this.next(); n = { op: 'conv', name: sp, kids: [n], pos: t.pos }; continue }
      }
      if (this.isOp('%')) { this.next(); n = { op: 'conv', name: 'pct', kids: [n], pos: t.pos }; continue }
      const su = this.units, sk = this.known
      this.units = true; this.known = () => false
      let tgt: Node
      try { tgt = this.product(true) } finally { this.units = su; this.known = sk }
      n = { op: 'conv', kids: [n, tgt], pos: t.pos }
    }
  }
  sum(): Node {
    let n = this.product(false)
    for (;;) {
      const t = this.peek()
      if (t.kind === 'op' && (t.text === '+' || t.text === '-')) { this.next(); const r = this.product(false); n = { op: t.text, kids: [n, r], pos: t.pos }; continue }
      return n
    }
  }
  startsOperand() {
    const t = this.peek()
    if (t.kind === 'num' || t.kind === 'cur') return true
    if (t.kind === 'id') {
      if (keywords.has(t.text)) return false
      if (t.text === 'in' && this.inIsConversion()) return false
      return true
    }
    return t.kind === 'op' && (t.text === '(' || t.text === '√')
  }
  product(target: boolean): Node {
    let n = this.unary()
    for (;;) {
      const t = this.peek()
      if (t.kind === 'op' && (t.text === '*' || t.text === '/')) { this.next(); const r = this.unary(); n = { op: t.text, kids: [n, r], pos: t.pos } }
      else if (t.kind === 'id' && (t.text === 'per' || t.text === 'mod')) { this.next(); const r = this.unary(); n = { op: t.text === 'mod' ? 'mod' : '/', kids: [n, r], pos: t.pos } }
      else if (!target && t.kind === 'id' && (t.text === 'of' || t.text === 'off' || t.text === 'on')) { this.next(); const r = this.unary(); n = { op: t.text, kids: [n, r], pos: t.pos } }
      else if (this.startsOperand()) { const r = this.power(); n = { op: 'imp', kids: [n, r], pos: t.pos } }
      else return n
    }
  }
  unary(): Node {
    const t = this.peek()
    if (t.kind === 'op' && t.text === '-') { this.next(); return { op: 'neg', kids: [this.unary()], pos: t.pos } }
    if (t.kind === 'op' && t.text === '+') { this.next(); return this.unary() }
    return this.power()
  }
  power(): Node {
    const b = this.postfix()
    if (this.isOp('^')) { const t = this.next(); const e = this.unary(); return { op: '^', kids: [b, e], pos: t.pos } }
    return b
  }
  postfix(): Node {
    let n = this.primary()
    for (;;) {
      const t = this.peek()
      if (t.kind !== 'op') return n
      if (t.text === '!') { this.next(); n = { op: '!', kids: [n], pos: t.pos } }
      else if (t.text === '%') { this.next(); n = { op: 'pct', kids: [n], pos: t.pos } }
      else return n
    }
  }
  isUnitName(name: string) {
    if (!this.units || this.known(name)) return false
    if (lookupUnit(name)) return true
    return isCurrency(name, this.rates)
  }
  primary(): Node {
    const t = this.next()
    switch (t.kind) {
      case 'num': {
        if (!this.units) return { op: 'num', num: t.num, pos: t.pos }
        const us: Units = []
        for (;;) {
          const u = this.peek()
          if (u.kind !== 'id' || keywords.has(u.text) || !this.isUnitName(u.text)) break
          const nx = this.peekN(1)
          if (nx.kind === 'op' && nx.text === '(') break
          if (u.text === 'in' && this.inIsConversion()) break
          this.next()
          let pow = 1
          if (this.isOp('^') && this.peekN(1).kind === 'num' && Number.isInteger(this.peekN(1).num)) { this.next(); pow = this.next().num }
          else if (this.isOp('^') && this.peekN(1).kind === 'op' && this.peekN(1).text === '-' && this.peekN(2).kind === 'num') { this.next(); this.next(); pow = -this.next().num }
          us.push({ name: canonicalCurrency(u.text), pow })
        }
        if (us.length) {
          const n: Node = { op: 'qty', num: t.num, units: us, pos: t.pos }
          const sl = this.peek(), nm = this.peekN(1), un = this.peekN(2)
          if (sl.kind === 'op' && sl.text === '/' && nm.kind === 'num' && un.kind === 'id' && nm.pos === sl.pos + 1 && un.pos === nm.pos + nm.text.length && nm.num > 0 && this.isUnitName(un.text)) {
            this.i += 3
            n.units!.push({ name: canonicalCurrency(un.text), pow: -1 })
            n.scale = nm.num
          }
          return n
        }
        return { op: 'num', num: t.num, pos: t.pos }
      }
      case 'date': return { op: 'date', name: t.text, pos: t.pos }
      case 'cur': {
        const code = currencyAliases[t.text]
        const v = this.postfix()
        return { op: 'cur', name: code, kids: [v], pos: t.pos }
      }
      case 'id': {
        if (keywords.has(t.text)) throw new CalcError(t.pos, `a number is missing before "${t.text}"`)
        let primes = 0
        while (this.isOp("'")) { this.next(); primes++ }
        if (this.isOp('(')) {
          this.next()
          const args: Node[] = []
          if (!this.isOp(')')) {
            for (;;) {
              args.push(this.equation())
              if (this.isOp(',') || this.isOp(';')) { this.next(); continue }
              break
            }
          }
          if (!this.isOp(')')) throw new CalcError(this.peek().pos, 'a ) is missing')
          this.next()
          return { op: 'call', name: t.text, num: primes, kids: args, pos: t.pos }
        }
        if (primes > 0) throw new CalcError(t.pos, `write ${t.text}'(x) with what to work it out at`)
        if (prefixFuncs.has(t.text) && !this.known(t.text) && this.startsOperand()) {
          const a = this.power()
          return { op: 'call', name: t.text, num: 0, kids: [a], pos: t.pos }
        }
        if (this.isUnitName(t.text)) return { op: 'unit', name: canonicalCurrency(t.text), pos: t.pos }
        return { op: 'id', name: t.text, pos: t.pos }
      }
      case 'op': {
        if (t.text === '(') {
          let n = this.equation()
          if (this.isOp(',')) {
            const kids = [n]
            while (this.isOp(',')) { this.next(); kids.push(this.expr()) }
            n = { op: 'tuple', kids, pos: t.pos }
          }
          if (!this.isOp(')')) throw new CalcError(this.peek().pos, 'a ) is missing')
          this.next()
          return n.op === 'tuple' ? n : { op: 'paren', kids: [n], pos: t.pos }
        }
        if (t.text === '[') {
          const kids: Node[] = []
          if (!this.isOp(']')) {
            for (;;) {
              kids.push(this.expr())
              if (this.isOp(',') || this.isOp(';')) { this.next(); continue }
              break
            }
          }
          if (!this.isOp(']')) throw new CalcError(this.peek().pos, 'a ] is missing')
          this.next()
          return { op: 'list', kids, pos: t.pos }
        }
        if (t.text === '|') {
          const n = this.expr()
          if (!this.isOp('|')) throw new CalcError(this.peek().pos, 'a closing | is missing')
          this.next()
          return { op: 'call', name: 'abs', kids: [n], num: 0, pos: t.pos }
        }
        throw new CalcError(t.pos, `a number is missing before "${t.text}"`)
      }
      case 'eof': throw new CalcError(t.pos, 'something is missing at the end')
    }
    throw new CalcError(t.pos, `I don't understand "${t.text}"`)
  }
}

const reservedName = (n: string) => ['pi', 'e', 'to', 'in', 'as', 'of', 'per', 'mod'].includes(n)

export function parseLine(line: string, env: Env): Stmt {
  const [label, rest] = splitLabel(line)
  const toks = lex(rest)
  if (toks.length === 1) return { kind: 'empty', label }
  const p = new Parser(toks, env.units, n => env.known(n), c => env.hasCurrency(c))
  if (toks[0].kind === 'id' && !keywords.has(toks[0].text)) {
    if (toks[1].kind === 'op' && toks[1].text === '=' && !reservedName(toks[0].text)) {
      p.i = 2
      const expr = p.expr(); p.end()
      return { kind: 'assign', label, name: toks[0].text, expr }
    }
    if (toks[1].kind === 'op' && toks[1].text === '(') {
      let j = 2
      const params: string[] = []
      let ok = true
      for (;;) {
        if (toks[j].kind !== 'id') { ok = false; break }
        params.push(toks[j].text); j++
        if (toks[j].kind === 'op' && toks[j].text === ',') { j++; continue }
        break
      }
      if (ok && toks[j].kind === 'op' && toks[j].text === ')' && toks[j + 1].kind === 'op' && toks[j + 1].text === '=' && !builtinSet.has(toks[0].text)) {
        p.i = j + 2
        const inner = p.known
        p.known = n => params.includes(n) || inner(n)
        const expr = p.expr(); p.end()
        return { kind: 'func', label, name: toks[0].text, params, expr }
      }
    }
  }
  const expr = p.equation(); p.end()
  return { kind: 'expr', label, expr }
}

export function parseExpr(src: string, env: Env): Node {
  const p = new Parser(lex(src), env.units, n => env.known(n), c => env.hasCurrency(c))
  const n = p.equation(); p.end()
  return n
}

// ---------- values ----------

export enum K { Num, List, Date, Bool, Solved, Point }
export interface Value { k: K; n: number; u: Units; pct?: boolean; list?: Value[]; fmt?: string; label?: string; lscale?: number; name?: string }
const num = (n: number): Value => ({ k: K.Num, n, u: [] })
const float = (v: Value) => v.pct ? v.n / 100 : v.n

export interface Func { params: string[]; body: Node }

export class Env {
  units: boolean; deg: boolean; rates: Record<string, number>; today: Date
  vars = new Map<string, Value>(); funcs = new Map<string, Func>()
  parent?: Env; prev?: Value; above: Value[] = []; depth = 0
  constructor(units: boolean, deg: boolean, rates: Record<string, number> = {}, today = new Date()) { this.units = units; this.deg = deg; this.rates = rates; this.today = today }
  child(): Env { const c = new Env(this.units, this.deg, this.rates, this.today); c.parent = this; c.depth = this.depth + 1; return c }
  lookup(n: string): Value | undefined { for (let x: Env | undefined = this; x; x = x.parent) { const v = x.vars.get(n); if (v) return v } return undefined }
  fn(n: string): Func | undefined { for (let x: Env | undefined = this; x; x = x.parent) { const f = x.funcs.get(n); if (f) return f } return undefined }
  known(n: string) { return !!this.lookup(n) || !!this.fn(n) }
  hasCurrency(c: string) { return c in this.rates }
  break() { this.above = [] }

  unitDef(name: string): UnitDef {
    const u = lookupUnit(name)
    if (u) return u
    const code = canonicalCurrency(name)
    if (isCurrency(code, c => this.hasCurrency(c))) {
      let factor = NaN
      if (code === 'EUR') factor = 1
      else if (this.rates[code] > 0) factor = 1 / this.rates[code]
      return { sym: code, dim: money, kind: 'money', cur: code, factor }
    }
    throw new Error(`I don't know the unit ${name}`)
  }
  info(us: Units): { factor: number; dim: Dim; offset: number } {
    const out = { factor: 1, dim: D([]), offset: 0 }
    for (const u of us) {
      const d = this.unitDef(u.name)
      if (isNaN(d.factor)) throw new Error(`there is no exchange rate for ${d.cur} yet`)
      out.factor *= Math.pow(d.factor, u.pow)
      out.dim = dimAdd(out.dim, d.dim, u.pow)
      if (us.length === 1 && u.pow === 1) out.offset = d.offset || 0
    }
    return out
  }
  dimOf(us: Units): Dim { let d = D([]); for (const u of us) d = dimAdd(d, this.unitDef(u.name).dim, u.pow); return d }
  convert(v: Value, to: Units): number {
    if (sameUnits(v.u, to)) return v.n
    const a = this.info(v.u), b = this.info(to)
    if (!dimEq(a.dim, b.dim)) throw new Error(`${unitText(v.u, 2)} and ${unitText(to, 2)} do not measure the same thing`)
    return ((v.n + a.offset) * a.factor) / b.factor - b.offset
  }
  toSI(v: Value): [number, Dim] {
    if (!v.u.length) return [float(v), D([])]
    const i = this.info(v.u)
    return [(v.n + i.offset) * i.factor, i.dim]
  }
  mulUnits(a: Units, b: Units, sign: number): [Units, number] {
    const out = a.map(x => ({ ...x }))
    let scale = 1
    for (const ub0 of b) {
      const ub = { ...ub0, pow: ub0.pow * sign }
      let done = false
      for (const ua of out) if (ua.name === ub.name) { ua.pow += ub.pow; done = true; break }
      if (!done) {
        const db = this.unitDef(ub.name)
        for (const ua of out) {
          const da = this.unitDef(ua.name)
          if (dimEq(da.dim, db.dim) && !dimZero(da.dim) && !da.offset && !db.offset) {
            if (isNaN(da.factor)) throw new Error(`there is no exchange rate for ${da.cur} yet`)
            if (isNaN(db.factor)) throw new Error(`there is no exchange rate for ${db.cur} yet`)
            scale *= Math.pow(db.factor / da.factor, ub.pow)
            ua.pow += ub.pow
            done = true
            break
          }
        }
      }
      if (!done) out.push(ub)
    }
    return [out.filter(u => u.pow !== 0), scale]
  }
}

const sameUnits = (a: Units, b: Units) => a.length === b.length && a.every((u, i) => u.name === b[i].name && u.pow === b[i].pow)

const SUPD = '⁰¹²³⁴⁵⁶⁷⁸⁹'
function superscript(n: number): string { return String(n).split('').map(c => c === '-' ? '⁻' : SUPD[+c]).join('') }

export function unitText(us: Units, n: number): string {
  if (!us.length) return ''
  const pos: string[] = [], neg: string[] = []
  for (const u of us) {
    let name = u.name
    const d = lookupUnit(u.name)
    if (d) { name = d.sym; if (d.plural && us.length === 1 && u.pow === 1 && Math.abs(n) !== 1) name = d.plural }
    const p = Math.abs(u.pow)
    const s = p !== 1 ? name + superscript(p) : name
    if (u.pow > 0) pos.push(s); else neg.push(s)
  }
  let txt = pos.join('·')
  if (us.length === 2 && us[0].pow === 1 && us[1].pow === 1 && us[1].name === 'h' && us[0].name.endsWith('W')) txt = us[0].name + 'h'
  if (neg.length) {
    if (!txt) txt = '1'
    txt += neg.length > 1 ? '/(' + neg.join('·') + ')' : '/' + neg[0]
  }
  return txt
}

// ---------- dates ----------

const dateDays = (t: Date) => Math.floor(Date.UTC(t.getFullYear(), t.getMonth(), t.getDate()) / 86400000)
const daysDate = (n: number) => new Date(Math.floor(n) * 86400000)
function addMonths(days: number, n: number): number {
  const t = daysDate(days)
  const y = t.getUTCFullYear(), m = t.getUTCMonth(), d = t.getUTCDate()
  const first = new Date(Date.UTC(y, m + n, 1))
  const last = new Date(Date.UTC(first.getUTCFullYear(), first.getUTCMonth() + 1, 0)).getUTCDate()
  return Math.floor(Date.UTC(first.getUTCFullYear(), first.getUTCMonth(), Math.min(d, last)) / 86400000)
}
const parseDate = (s: string) => { const [y, m, d] = s.split('-').map(Number); const t = new Date(Date.UTC(y, m - 1, d)); return t.getUTCMonth() === m - 1 ? Math.floor(t.getTime() / 86400000) : NaN }

// ---------- evaluation ----------

const constants: Record<string, number> = { pi: Math.PI, e: Math.E, tau: 2 * Math.PI, phi: (1 + Math.sqrt(5)) / 2, inf: Infinity, '∞': Infinity, infinity: Infinity }
const physConsts: Record<string, Value> = {
  gravity: { k: K.Num, n: 9.80665, u: [{ name: 'm', pow: 1 }, { name: 's', pow: -2 }] },
  lightspeed: { k: K.Num, n: 299792458, u: [{ name: 'm', pow: 1 }, { name: 's', pow: -1 }] },
  soundspeed: { k: K.Num, n: 343, u: [{ name: 'm', pow: 1 }, { name: 's', pow: -1 }] },
}

function wrap(e: unknown, pos: number): never {
  if (e instanceof CalcError) throw e
  throw new CalcError(pos, (e as Error).message)
}

export function evaluate(e: Env, n: Node): Value {
  if (e.depth > 60) throw new Error('this goes round in circles (a function calls itself too often)')
  switch (n.op) {
    case 'num': return num(n.num!)
    case 'qty': {
      const v: Value = { k: K.Num, n: n.num!, u: n.units!.map(u => ({ ...u })) }
      if (n.scale && n.scale !== 1) { v.n /= n.scale; v.lscale = 1 / n.scale; v.label = labelOf({ k: K.Num, n: v.lscale, u: v.u }) }
      return v
    }
    case 'unit': return { k: K.Num, n: 1, u: [{ name: n.name!, pow: 1 }] }
    case 'paren': return evaluate(e, n.kids![0])
    case 'date': { const d = parseDate(n.name!); if (isNaN(d)) throw new CalcError(n.pos, n.name + ' is not a date'); return { k: K.Date, n: d, u: [] } }
    case 'id': return ident(e, n)
    case 'neg': return neg(e, evaluate(e, n.kids![0]))
    case 'pct': {
      const v = evaluate(e, n.kids![0])
      if (v.k !== K.Num || v.u.length || v.pct) throw new CalcError(n.pos, 'only a plain number can be a percentage')
      return { ...v, pct: true }
    }
    case 'cur': return mul(e, evaluate(e, n.kids![0]), { k: K.Num, n: 1, u: [{ name: n.name!, pow: 1 }] })
    case '+': case '-': case '*': case '/': case 'mod': case '^': case 'imp': case 'of': case 'off': case 'on': {
      const a = evaluate(e, n.kids![0]), b = evaluate(e, n.kids![1])
      try { return binary(e, n, a, b) } catch (x) { wrap(x, n.pos) }
    }
    case '!': {
      const v = evaluate(e, n.kids![0])
      if (v.k !== K.Num || v.u.length || v.n < 0 || v.n !== Math.floor(v.n)) throw new CalcError(n.pos, '! needs a whole number of 0 or more')
      return num(v.n > 170 ? Infinity : factorial(v.n))
    }
    case 'list': {
      const out: Value = { k: K.List, n: 0, u: [], list: [] }
      for (const k of n.kids!) { const v = evaluate(e, k); if (v.k === K.List) out.list!.push(...v.list!); else out.list!.push(v) }
      return out
    }
    case 'tuple': return { k: K.Point, n: 0, u: [], list: n.kids!.map(k => evaluate(e, k)) }
    case 'conv': {
      const v = evaluate(e, n.kids![0])
      if (n.name) return special(v, n.name, n.pos)
      return convertTo(e, v, n.kids![1])
    }
    case 'call': return call(e, n)
    case 'eq': case 'lt': case 'gt': case 'le': case 'ge': case 'ne': return equation(e, n)
  }
  throw new CalcError(n.pos, "I can't work this out")
}

function factorial(n: number): number { let r = 1; for (let i = 2; i <= n; i++) r *= i; return r }

export const builtinSet = new Set(['sqrt', 'cbrt', 'root', 'exp', 'ln', 'log', 'log2', 'abs', 'round', 'floor', 'ceil', 'trunc', 'sign', 'mod', 'gcd', 'lcm', 'factorial', 'nCr', 'nPr', 'hypot',
  'sin', 'cos', 'tan', 'asin', 'acos', 'atan', 'sinh', 'cosh', 'tanh', 'sec', 'csc', 'cot', 'min', 'max', 'sum', 'mean', 'median', 'sd', 'var', 'count', 'sort', 'product', 'solve', 'deriv', 'integral',
  'pmt', 'fv', 'pv', 'compound', 'days', 'weekday', 'deg', 'rad',
  'log10', 'avg', 'average', 'stdev', 'variance', 'choose', 'perm', 'fact', 'integrate', 'derivative', 'diff', 'asinh', 'acosh', 'atanh', 'atan2', 'len', 'if'])

function ident(e: Env, n: Node): Value {
  const v = e.lookup(n.name!)
  if (v) return v
  if (n.name! in constants) return num(constants[n.name!])
  if (e.units && physConsts[n.name!]) return physConsts[n.name!]
  switch (n.name) {
    case 'ans': case 'prev': case 'previous': case 'last':
      if (e.prev) return e.prev
      throw new CalcError(n.pos, 'there is no line above to use')
    case 'total': case 'sum': case 'subtotal': return total(e, false)
    case 'average': case 'avg': case 'mean': return total(e, true)
    case 'today': case 'now': return { k: K.Date, n: dateDays(e.today), u: [] }
    case 'tomorrow': return { k: K.Date, n: dateDays(e.today) + 1, u: [] }
    case 'yesterday': return { k: K.Date, n: dateDays(e.today) - 1, u: [] }
    case 'true': case 'yes': return { k: K.Bool, n: 1, u: [] }
    case 'false': case 'no': return { k: K.Bool, n: 0, u: [] }
  }
  if (e.units) { try { e.unitDef(n.name!); return { k: K.Num, n: 1, u: [{ name: canonicalCurrency(n.name!), pow: 1 }] } } catch { /* not a unit */ } }
  if (e.fn(n.name!) || builtinSet.has(n.name!)) throw new CalcError(n.pos, `${n.name} is a function: write ${n.name}(…)`)
  throw new CalcError(n.pos, `I don't know "${n.name}" yet`)
}

function total(e: Env, avg: boolean): Value {
  let acc: Value | undefined
  let k = 0
  for (const v of e.above) {
    if (v.k !== K.Num || v.pct) continue
    if (!acc) { acc = { ...v }; k++; continue }
    try { acc = add(e, acc, v, 1); k++ } catch { /* skip */ }
  }
  if (!acc) return num(0)
  if (avg) acc = { ...acc, n: acc.n / k }
  return acc
}

function neg(e: Env, v: Value): Value {
  if (v.k === K.Num) return { ...v, n: -v.n }
  if (v.k === K.List) return { ...v, list: v.list!.map(x => neg(e, x)) }
  throw new Error("this can't be made negative")
}

function binary(e: Env, n: Node, a: Value, b: Value): Value {
  if (a.k === K.List || b.k === K.List) return listOp(e, n, a, b)
  switch (n.op) {
    case '+': return add(e, a, b, 1)
    case '-': return add(e, a, b, -1)
    case '*': return mul(e, a, b)
    case 'imp':
      if (n.kids![1].op === 'qty' && a.u.length && a.k === K.Num) {
        try {
          const da = e.dimOf(a.u), db = e.dimOf(b.u)
          if (dimEq(da, db) && !dimZero(da)) return add(e, a, b, 1)
        } catch { /* fall through */ }
      }
      return mul(e, a, b)
    case '/': return div(e, a, b)
    case 'mod': {
      if (a.k !== K.Num || b.k !== K.Num) throw new Error('mod needs numbers')
      const bn = sameAs(e, b, a)
      if (bn === 0) throw new Error('mod by zero')
      let r = a.n % bn
      if (r !== 0 && (r < 0) !== (bn < 0)) r += bn
      return { ...a, n: r }
    }
    case '^': return pow(e, a, b)
    case 'of': case 'off': case 'on': {
      if (!a.pct) {
        if (n.op === 'of') return mul(e, a, b)
        throw new Error(`write a percentage before "${n.op}", like 20% ${n.op} 50`)
      }
      let p = a.n / 100
      if (n.op === 'off') p = 1 - p
      if (n.op === 'on') p = 1 + p
      return mul(e, num(p), b)
    }
  }
  throw new Error("I can't work this out")
}

function listOp(e: Env, n: Node, a: Value, b: Value): Value {
  if (a.k === K.List && b.k === K.List) {
    if (a.list!.length !== b.list!.length) throw new Error('the lists are not the same length')
    return { k: K.List, n: 0, u: [], list: a.list!.map((x, i) => binary(e, n, x, b.list![i])) }
  }
  if (a.k === K.List) return { k: K.List, n: 0, u: [], list: a.list!.map(x => binary(e, n, x, b)) }
  return { k: K.List, n: 0, u: [], list: b.list!.map(x => binary(e, n, a, x)) }
}

function sameAs(e: Env, b: Value, a: Value): number {
  if (!a.u.length && !b.u.length) return float(b)
  return e.convert(b, a.u)
}

function add(e: Env, a: Value, b: Value, sign: number): Value {
  if (a.k === K.Date || b.k === K.Date) return addDate(e, a, b, sign)
  if (a.k !== K.Num || b.k !== K.Num) throw new Error('only numbers can be added')
  if (b.pct && !a.pct) return mul(e, a, num(1 + (sign * b.n) / 100))
  if (a.pct && b.pct) return { ...a, n: a.n + sign * b.n }
  if (a.pct && !b.pct) a = num(a.n / 100)
  if (!a.u.length && b.u.length) {
    if (a.n === 0) return { ...b, n: b.n * sign }
    if (dimZero(e.dimOf(b.u))) { try { return { ...a, n: a.n + sign * e.convert(b, []) } } catch { /* */ } }
    throw new Error(`can't add a plain number and ${unitText(b.u, 2)}`)
  }
  if (!b.u.length && a.u.length) {
    if (b.n === 0) return a
    if (dimZero(e.dimOf(a.u))) { try { return num(e.convert(a, []) + sign * b.n) } catch { /* */ } }
    throw new Error(`can't add ${unitText(a.u, 2)} and a plain number`)
  }
  const bn = convertDelta(e, b, a.u)
  return { ...a, n: a.n + sign * bn, label: undefined, lscale: undefined }
}

function convertDelta(e: Env, b: Value, to: Units): number {
  if (sameUnits(b.u, to)) return b.n
  const x = e.info(b.u), y = e.info(to)
  if (!dimEq(x.dim, y.dim)) throw new Error(`can't add ${unitText(to, 2)} and ${unitText(b.u, 2)}: they measure different things`)
  return (b.n * x.factor) / y.factor
}

function addDate(e: Env, a: Value, b: Value, sign: number): Value {
  if (a.k === K.Date && b.k === K.Date) {
    if (sign > 0) throw new Error("two dates can't be added (take one from the other)")
    return { k: K.Num, n: a.n - b.n, u: [{ name: 'day', pow: 1 }] }
  }
  if (b.k === K.Date) { if (sign < 0) throw new Error("a date can't be taken from a number"); [a, b] = [b, a] }
  if (!b.u.length) throw new Error('say what to add to a date: days, weeks, months or years')
  if (!dimEq(e.dimOf(b.u), tm)) throw new Error('only time can be added to a date')
  if (b.u.length === 1 && b.u[0].pow === 1 && b.n === Math.trunc(b.n)) {
    if (b.u[0].name === 'month') return { k: K.Date, n: addMonths(a.n, sign * b.n), u: [] }
    if (b.u[0].name === 'year') return { k: K.Date, n: addMonths(a.n, 12 * sign * b.n), u: [] }
  }
  const [secs] = e.toSI(b)
  return { k: K.Date, n: a.n + sign * Math.round((secs / DAY) * 1e6) / 1e6, u: [] }
}

function mul(e: Env, a: Value, b: Value): Value {
  if (a.k === K.Date || b.k === K.Date) throw new Error("a date can't be multiplied")
  if (a.k !== K.Num || b.k !== K.Num) throw new Error('only numbers can be multiplied')
  const [us, scale] = e.mulUnits(a.u, b.u, 1)
  if (a.pct && b.pct) return { k: K.Num, n: (a.n * b.n) / 100, u: [], pct: true }
  return tidy(e, { k: K.Num, n: float(a) * float(b) * scale, u: us })
}

function div(e: Env, a: Value, b: Value): Value {
  if (a.k === K.Date || b.k === K.Date) throw new Error("a date can't be divided")
  if (a.k !== K.Num || b.k !== K.Num) throw new Error('only numbers can be divided')
  const [us, scale] = e.mulUnits(a.u, b.u, -1)
  const bf = float(b)
  if (bf === 0) throw new Error("can't divide by zero")
  if (a.pct && b.pct) return num(a.n / b.n)
  return tidy(e, { k: K.Num, n: (float(a) / bf) * scale, u: us })
}

function tidy(e: Env, v: Value): Value {
  if (!v.u.length) return v
  let d: Dim | undefined
  try { d = e.dimOf(v.u) } catch { return v }
  if (v.u.length > 1 && !dimZero(d)) {
    for (const u of v.u) {
      let def: UnitDef
      try { def = e.unitDef(u.name) } catch { continue }
      if (u.pow !== 1 || !dimEq(def.dim, d) || def.offset) continue
      try { return { k: K.Num, n: e.convert(v, [u]), u: [{ ...u }] } } catch { /* */ }
    }
  }
  if (dimZero(d)) {
    if (v.u.length === 1 && v.u[0].pow === 1) return v
    try { return num(e.convert(v, [])) } catch { /* */ }
  }
  return v
}

function pow(e: Env, a: Value, b: Value): Value {
  if (a.k !== K.Num || b.k !== K.Num) throw new Error('only numbers can be raised to a power')
  if (b.u.length) throw new Error("a power can't have a unit")
  const p = float(b)
  if (!a.u.length) return num(Math.pow(float(a), p))
  const out: Value = { k: K.Num, n: Math.pow(a.n, p), u: [] }
  for (const u of a.u) {
    const np = u.pow * p
    if (Math.abs(np - Math.round(np)) > 1e-9) throw new Error(`${unitText(a.u, 2)} can't be raised to the power ${p}`)
    if (Math.round(np) !== 0) out.u.push({ name: u.name, pow: Math.round(np) })
  }
  void e
  return out
}

function labelOf(t: Value): string {
  if (t.n === 1) return ''
  const pos: string[] = [], neg: string[] = []
  for (const u of t.u) {
    let s = lookupUnit(u.name)?.sym || u.name
    const p = Math.abs(u.pow)
    if (p !== 1) s += superscript(p)
    if (u.pow > 0) pos.push(s); else neg.push(s)
  }
  if (t.n > 1) return trimFloat(t.n) + ' ' + pos.join('·') + '/' + neg.join('·')
  return (pos.join('·') || '1') + '/' + trimFloat(1 / t.n) + neg.join('·')
}
const trimFloat = (f: number) => f.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')

function convertTo(e: Env, v: Value, tn: Node): Value {
  if (v.k === K.List) return { ...v, list: v.list!.map(x => convertTo(e, x, tn)) }
  if (v.k === K.Date) throw new CalcError(tn.pos, "a date can't be turned into a unit")
  const su = e.units
  e.units = true
  let t: Value
  try { t = evaluate(e, tn) } finally { e.units = su }
  if (t.k !== K.Num || !t.u.length) throw new CalcError(tn.pos, 'write a unit after "to"')
  if (!v.u.length && !v.pct) {
    const out: Value = { k: K.Num, n: v.n, u: t.u.map(u => ({ ...u })) }
    if (t.n !== 1) { out.n = v.n * t.n; out.label = labelOf(t); out.lscale = t.n }
    return out
  }
  if (v.pct) v = num(v.n / 100)
  let x: number, dx: Dim, ti: { factor: number; dim: Dim; offset: number }
  try { [x, dx] = e.toSI(v); ti = e.info(t.u) } catch (err) { throw new CalcError(tn.pos, (err as Error).message) }
  const f = t.n * ti.factor
  if (dimEq(dx, ti.dim)) {
    if (t.n !== 1) return { k: K.Num, n: x / ti.factor, u: t.u.map(u => ({ ...u })), label: labelOf(t), lscale: t.n }
    return { k: K.Num, n: x / f - ti.offset, u: t.u.map(u => ({ ...u })) }
  }
  if (dimEq(dx, dimAdd(D([]), ti.dim, -1)) && x !== 0) {
    const out: Value = { k: K.Num, n: 1 / (x * ti.factor), u: t.u.map(u => ({ ...u })) }
    if (t.n !== 1) { out.label = labelOf(t); out.lscale = t.n }
    return out
  }
  throw new CalcError(tn.pos, `${unitText(v.u, 2) || 'a plain number'} can't be turned into ${unitText(t.u, 2)}: they measure different things`)
}

function special(v: Value, how: string, pos: number): Value {
  switch (how) {
    case 'pct':
      if (v.k !== K.Num || v.u.length) throw new CalcError(pos, 'only a plain number can be shown as a percentage')
      return v.pct ? v : { k: K.Num, n: v.n * 100, u: [], pct: true }
    case 'dec': return v.pct ? num(v.n / 100) : { ...v, fmt: undefined }
    case 'hex': case 'bin': case 'oct':
      if (v.k !== K.Num || v.u.length || v.n !== Math.trunc(v.n) || Math.abs(v.n) > 2 ** 53) throw new CalcError(pos, 'only a whole number can be shown in ' + ({ hex: 'hexadecimal', bin: 'binary', oct: 'octal' } as Record<string, string>)[how])
  }
  return { ...v, fmt: how }
}

// equations

function equation(e: Env, n: Node): Value {
  const unk = new Set<string>()
  unknowns(e, n, unk)
  const names = [...unk].sort()
  if (!names.length) return compare(e, n.op, evaluate(e, n.kids![0]), evaluate(e, n.kids![1]))
  if (n.op !== 'eq') throw new CalcError(n.pos, 'an inequality with an unknown is drawn in a graph, not worked out here')
  if (names.length > 1) throw new CalcError(n.pos, `there is more than one unknown (${names.join(', ')}); set all but one first`)
  return solve(e, n.kids![0], n.kids![1], names[0], n.pos)
}

export function unknowns(e: Env, n: Node | undefined, out: Set<string>) {
  if (!n) return
  if (n.op === 'id') {
    let ok = true
    try { ident(e, n) } catch { ok = false }
    if (!ok && !e.fn(n.name!) && !builtinSet.has(n.name!)) out.add(n.name!)
  }
  if (n.op === 'call' && ['solve', 'integral', 'integrate', 'deriv', 'derivative', 'diff'].includes(n.name!)) return
  for (const k of n.kids || []) unknowns(e, k, out)
}

function cmp(op: string, x: number, y: number, eps: number) {
  switch (op) {
    case 'eq': return Math.abs(x - y) <= eps
    case 'ne': return Math.abs(x - y) > eps
    case 'lt': return x < y - eps
    case 'gt': return x > y + eps
    case 'le': return x <= y + eps
    case 'ge': return x >= y - eps
  }
  return false
}
const boolV = (b: boolean): Value => ({ k: K.Bool, n: b ? 1 : 0, u: [] })

function compare(e: Env, op: string, a: Value, b: Value): Value {
  if (a.k === K.Date && b.k === K.Date) return boolV(cmp(op, a.n, b.n, 1e-12))
  const [x, da] = e.toSI(a), [y, db] = e.toSI(b)
  if (!dimEq(da, db)) throw new Error('these two measure different things')
  return boolV(cmp(op, x, y, 1e-9 * Math.max(1, Math.abs(x), Math.abs(y))))
}

/** fnOf turns an expression into a function of one unknown (SI numbers). */
export function fnOf(e: Env, body: Node, name: string): (t: number) => number {
  const c = e.child()
  return (t: number) => {
    c.vars.set(name, num(t))
    let v: Value
    try { v = evaluate(c, body) } catch { return NaN }
    if (v.k === K.Bool) return v.n
    if (v.k !== K.Num) return NaN
    try { return c.toSI(v)[0] } catch { return NaN }
  }
}

function solve(e: Env, l: Node, r: Node, name: string, pos: number): Value {
  const f = fnOf(e, { op: '-', kids: [l, r], pos: 0 }, name)
  const roots = findRoots(f, -1e6, 1e6)
  if (!roots.length) throw new CalcError(pos, `I found no solution for ${name} (between −1,000,000 and 1,000,000)`)
  return { k: K.Solved, n: 0, u: [], name, list: roots.map(num) }
}

export function findRoots(f: (x: number) => number, _lo: number, hi: number): number[] {
  const xs: number[] = []
  for (let x = -100; x <= 100; x += 0.05) xs.push(Math.round(x * 1e6) / 1e6)
  for (let p = 2; p <= Math.log10(hi); p += 0.02) { const v = Math.pow(10, p); xs.push(v, -v) }
  xs.sort((a, b) => a - b)
  const roots: number[] = []
  const addR = (x: number) => {
    x = tidyRoot(x)
    for (const r of roots) if (Math.abs(r - x) <= 1e-7 * Math.max(1, Math.abs(x))) return
    roots.push(x)
  }
  let prevX = NaN, prevY = NaN
  for (let i = 0; i < xs.length; i++) {
    const x = xs[i]
    const y = f(x)
    if (y === 0) addR(x)
    else if (!isNaN(y) && !isNaN(prevY) && (y > 0) !== (prevY > 0) && prevY !== 0) {
      let a = prevX, b = x, fa = prevY
      for (let k = 0; k < 200; k++) {
        const m = (a + b) / 2
        const fm = f(m)
        if (isNaN(fm)) break
        if ((fm > 0) === (fa > 0)) { a = m; fa = fm } else b = m
        if (b - a < 1e-14 * Math.max(1, Math.abs(m))) break
      }
      const m = (a + b) / 2
      const fm = f(m)
      if (!isNaN(fm) && Math.abs(fm) < 1e-6 * Math.max(1, Math.abs(prevY), Math.abs(y))) addR(m)
    } else if (i > 0 && i + 1 < xs.length && !isNaN(y) && !isNaN(prevY)) {
      const ny = f(xs[i + 1])
      if (Math.abs(y) < Math.abs(prevY) && Math.abs(y) < Math.abs(ny) && (y > 0) === (ny > 0) && (y > 0) === (prevY > 0)) {
        const g = (t: number) => Math.abs(f(t))
        let a = prevX, b = xs[i + 1]
        for (let k = 0; k < 200; k++) {
          const m1 = a + (b - a) / 3, m2 = b - (b - a) / 3
          if (g(m1) < g(m2)) b = m2; else a = m1
        }
        const m = (a + b) / 2
        if (g(m) < 1e-9) addR(m)
      }
    }
    prevX = x; prevY = y
  }
  roots.sort((a, b) => a - b)
  return roots.slice(0, 20)
}

function tidyRoot(x: number): number {
  const r = Math.round(x)
  if (Math.abs(x - r) < 1e-9 * Math.max(1, Math.abs(x))) return r + 0
  return Number(x.toPrecision(12))
}

// ---------- functions ----------

function plain(name: string, args: Value[], n: number): number[] {
  if (n >= 0 && args.length !== n) throw new Error(`${name} needs ${n} value(s)`)
  return args.map(a => {
    if (a.k !== K.Num) throw new Error(`${name} needs numbers`)
    if (a.u.length) throw new Error(`${name} needs plain numbers, without units`)
    return float(a)
  })
}
const flatten = (args: Value[]) => args.flatMap(a => a.k === K.List ? a.list! : [a])

function angleArg(e: Env, v: Value): number {
  if (v.k !== K.Num) throw new Error('an angle must be a number')
  if (v.u.length) {
    const [x, d] = e.toSI(v)
    if (!dimZero(d)) throw new Error("an angle can't have the unit " + unitText(v.u, 2))
    return x
  }
  return e.deg ? (float(v) * Math.PI) / 180 : float(v)
}
const angleOut = (e: Env, r: number): Value => e.deg ? { k: K.Num, n: (r * 180) / Math.PI, u: [{ name: 'deg', pow: 1 }] } : num(r)
const cleanTrig = (x: number) => Math.abs(x) < 1e-15 ? 0 : x
const gcd = (a: number, b: number): number => { while (b !== 0) [a, b] = [b, a % b]; return a }

function call(e: Env, n: Node): Value {
  const name = n.name!
  switch (name) {
    case 'solve': return solveCall(e, n)
    case 'integral': case 'integrate': return integralCall(e, n)
    case 'deriv': case 'derivative': case 'diff': return derivCall(e, n)
    case 'sum': case 'product': if (n.kids!.length === 4 && n.kids![1].op === 'id') return seriesCall(e, n); break
    case 'if': {
      if (n.kids!.length !== 3) throw new CalcError(n.pos, 'write if(condition, then, otherwise)')
      const c = evaluate(e, n.kids![0])
      return evaluate(e, c.n !== 0 ? n.kids![1] : n.kids![2])
    }
  }
  const args = n.kids!.map(k => evaluate(e, k))
  const f = e.fn(name)
  if (f) {
    if (args.length !== f.params.length) throw new CalcError(n.pos, `${name} needs ${f.params.length} value(s)`)
    if (n.num! > 0) {
      if (args.length !== 1 || args[0].k !== K.Num) throw new CalcError(n.pos, `${name}' works with one number`)
      return numDeriv(e, f.body, f.params[0], float(args[0]), n.num!, n.pos)
    }
    const c = e.child()
    f.params.forEach((p, i) => c.vars.set(p, args[i]))
    return evaluate(c, f.body)
  }
  if (n.num! > 0) throw new CalcError(n.pos, "' works on a function you set, like f(x) = x^2")
  const v = e.lookup(name)
  if (v && args.length === 1) return mul(e, v, args[0])
  if (!builtinSet.has(name)) {
    if (e.units && args.length === 1) { try { e.unitDef(name); return mul(e, args[0], { k: K.Num, n: 1, u: [{ name: canonicalCurrency(name), pow: 1 }] }) } catch { /* */ } }
    throw new CalcError(n.pos, `I don't know the function "${name}"`)
  }
  try { return builtin(e, name, args) } catch (x) { wrap(x, n.pos) }
}

function builtin(e: Env, name: string, args: Value[]): Value {
  const one = () => plain(name, args, 1)[0]
  const keep = (f: (x: number) => number): Value => {
    if (args.length !== 1 || args[0].k !== K.Num) throw new Error(`${name} needs one number`)
    return { ...args[0], n: f(args[0].n) }
  }
  switch (name) {
    case 'sqrt':
      if (args.length !== 1) throw new Error('sqrt needs one number')
      if (!args[0].u.length && float(args[0]) < 0) throw new Error('a negative number has no real square root')
      return pow(e, args[0], num(0.5))
    case 'cbrt':
      if (args.length !== 1) throw new Error('cbrt needs one number')
      if (!args[0].u.length) return num(Math.cbrt(float(args[0])))
      return pow(e, args[0], num(1 / 3))
    case 'root': { const [x, k] = plain(name, args, 2); if (x < 0 && k % 2 === 1) return num(-Math.pow(-x, 1 / k)); return num(Math.pow(x, 1 / k)) }
    case 'exp': return num(Math.exp(one()))
    case 'ln': { const x = one(); if (x <= 0) throw new Error('ln needs a number above 0'); return num(Math.log(x)) }
    case 'log': case 'log10': {
      if (args.length === 2) { const [x, b] = plain(name, args, 2); return num(Math.log(x) / Math.log(b)) }
      const x = one(); if (x <= 0) throw new Error('log needs a number above 0'); return num(Math.log10(x))
    }
    case 'log2': return num(Math.log2(one()))
    case 'abs': return keep(Math.abs)
    case 'floor': return keep(Math.floor)
    case 'ceil': return keep(Math.ceil)
    case 'trunc': return keep(Math.trunc)
    case 'sign': return num(Math.sign(one()))
    case 'round': {
      if (args.length === 2) { const [d] = plain(name, args.slice(1), 1); const p = Math.pow(10, Math.round(d)); return { ...args[0], n: Math.round(args[0].n * p) / p } }
      return keep(goRound)
    }
    case 'mod': if (args.length !== 2) throw new Error('mod needs two numbers'); return binary(e, { op: 'mod', pos: 0 }, args[0], args[1])
    case 'gcd': case 'lcm': {
      const x = plain(name, flatten(args), -1)
      if (!x.length) throw new Error(`${name} needs whole numbers`)
      let acc = Math.abs(Math.round(x[0]))
      for (let y of x.slice(1)) { y = Math.abs(Math.round(y)); if (name === 'gcd') acc = gcd(acc, y); else acc = acc && y ? (acc / gcd(acc, y)) * y : 0 }
      return num(acc)
    }
    case 'factorial': case 'fact': { const x = one(); if (x < 0 || x !== Math.floor(x)) throw new Error('factorial needs a whole number of 0 or more'); return num(factorial(x)) }
    case 'nCr': case 'choose': case 'nPr': case 'perm': {
      const [n, k] = plain(name, args, 2)
      if (n < 0 || k < 0 || k > n || n !== Math.floor(n) || k !== Math.floor(k)) return num(0)
      let r = 1
      for (let i = 0; i < k; i++) { r *= n - i; if (name === 'nCr' || name === 'choose') r /= i + 1 }
      return num(Math.round(r))
    }
    case 'hypot': {
      if (args.length !== 2) throw new Error('hypot needs two numbers')
      return pow(e, add(e, mul(e, args[0], args[0]), mul(e, args[1], args[1]), 1), num(0.5))
    }
    case 'sin': case 'cos': case 'tan': case 'sec': case 'csc': case 'cot': {
      if (args.length !== 1) throw new Error(`${name} needs one angle`)
      const a = angleArg(e, args[0])
      const s = cleanTrig(Math.sin(a)), c = cleanTrig(Math.cos(a))
      switch (name) {
        case 'sin': return num(s)
        case 'cos': return num(c)
        case 'tan': if (c === 0) throw new Error('tan is endless here'); return num(s / c)
        case 'sec': return num(1 / c)
        case 'csc': return num(1 / s)
        default: return num(c / s)
      }
    }
    case 'asin': case 'acos': {
      const x = one()
      if (x < -1 || x > 1) throw new Error(`${name} needs a number from −1 to 1`)
      return angleOut(e, name === 'asin' ? Math.asin(x) : Math.acos(x))
    }
    case 'atan': case 'atan2':
      if (args.length === 2) { const [y, x] = plain(name, args, 2); return angleOut(e, Math.atan2(y, x)) }
      return angleOut(e, Math.atan(one()))
    case 'sinh': return num(Math.sinh(one()))
    case 'cosh': return num(Math.cosh(one()))
    case 'tanh': return num(Math.tanh(one()))
    case 'asinh': return num(Math.asinh(one()))
    case 'acosh': return num(Math.acosh(one()))
    case 'atanh': return num(Math.atanh(one()))
    case 'deg': return { k: K.Num, n: (one() * 180) / Math.PI, u: [{ name: 'deg', pow: 1 }] }
    case 'rad': if (args.length === 1 && args[0].u.length) return num(angleArg(e, args[0])); return num((one() * Math.PI) / 180)
    case 'min': case 'max': {
      const items = flatten(args)
      if (!items.length) throw new Error(`${name} needs at least one number`)
      let best = items[0]
      for (const v of items.slice(1)) if ((compare(e, 'lt', v, best).n === 1) === (name === 'min')) best = v
      return best
    }
    case 'sum': case 'product': {
      const items = flatten(args)
      if (!items.length) return num(name === 'sum' ? 0 : 1)
      let acc = items[0]
      for (const v of items.slice(1)) acc = name === 'sum' ? add(e, acc, v, 1) : mul(e, acc, v)
      return acc
    }
    case 'mean': case 'avg': case 'average': {
      const items = flatten(args)
      if (!items.length) throw new Error('mean needs numbers')
      const s = builtin(e, 'sum', items)
      return { ...s, n: s.n / items.length }
    }
    case 'median': case 'sd': case 'stdev': case 'var': case 'variance': {
      const items = flatten(args)
      if (!items.length) throw new Error(`${name} needs numbers`)
      const base = items[0]
      const xs = items.map(v => sameAs(e, v, base))
      const out: Value = { k: K.Num, n: 0, u: base.u.map(u => ({ ...u })) }
      if (name === 'median') {
        xs.sort((a, b) => a - b)
        const m = Math.floor(xs.length / 2)
        out.n = xs.length % 2 === 0 ? (xs[m - 1] + xs[m]) / 2 : xs[m]
      } else {
        if (xs.length < 2) throw new Error('needs at least two numbers')
        const mean = xs.reduce((a, b) => a + b, 0) / xs.length
        const v = xs.reduce((a, b) => a + (b - mean) ** 2, 0) / (xs.length - 1)
        if (name === 'var' || name === 'variance') { out.u.forEach(u => (u.pow *= 2)); out.n = v } else out.n = Math.sqrt(v)
      }
      return out
    }
    case 'count': case 'len': return num(flatten(args).length)
    case 'sort': { const items = flatten(args); items.sort((a, b) => compare(e, 'lt', a, b).n === 1 ? -1 : compare(e, 'lt', b, a).n === 1 ? 1 : 0); return { k: K.List, n: 0, u: [], list: items } }
    case 'pmt': case 'fv': case 'pv': {
      if (args.length < 3 || args.length > 4) throw new Error(`write ${name}(rate, periods, amount)`)
      const rate = args[0]
      if (rate.k !== K.Num || rate.u.length) throw new Error('the rate is a percentage, like 5%/12')
      const r = float(rate)
      const [n] = plain(name, args.slice(1, 2), 1)
      const m = args[2]
      const extra = args.length === 4 ? sameAs(e, args[3], m) : 0
      const g = Math.pow(1 + r, n)
      const out: Value = { k: K.Num, n: 0, u: m.u.map(u => ({ ...u })) }
      if (name === 'pmt') out.n = r === 0 ? (m.n + extra) / n : ((m.n * g + extra) * r) / (g - 1)
      else if (name === 'fv') out.n = r === 0 ? m.n * n + extra : (m.n * (g - 1)) / r + extra * g
      else out.n = r === 0 ? m.n * n + extra : (m.n * (1 - 1 / g)) / r + extra / g
      return out
    }
    case 'compound': {
      if (args.length < 3 || args.length > 4) throw new Error('write compound(start, rate, years) or compound(start, rate, years, times a year)')
      const start = args[0], r = float(args[1]), yrs = args[2]
      const years = yrs.u.length ? e.convert(yrs, [{ name: 'year', pow: 1 }]) : float(yrs)
      const k = args.length === 4 ? float(args[3]) : 1
      return { ...start, n: start.n * Math.pow(1 + r / k, k * years) }
    }
    case 'days':
      if (args.length !== 2 || args[0].k !== K.Date || args[1].k !== K.Date) throw new Error('write days(from, to) with two dates')
      return { k: K.Num, n: args[1].n - args[0].n, u: [{ name: 'day', pow: 1 }] }
    case 'weekday':
      if (args.length !== 1 || args[0].k !== K.Date) throw new Error('weekday needs a date')
      return { k: K.Date, n: args[0].n, u: [], fmt: 'weekday' }
  }
  throw new Error(`I don't know the function "${name}"`)
}

/** Go's math.Round: halves away from zero (JS rounds -2.5 to -2). */
function goRound(x: number): number { return x < 0 ? -Math.round(-x) : Math.round(x) }

function varArg(n: Node, i: number): string | undefined { const k = n.kids![i]; return k && k.op === 'id' ? k.name : undefined }

function solveCall(e: Env, n: Node): Value {
  if (!n.kids!.length || n.kids!.length > 2) throw new CalcError(n.pos, 'write solve(x^2 = 9) or solve(equation, x)')
  const eq = n.kids![0]
  let l = eq, r: Node = { op: 'num', num: 0, pos: 0 }
  if (eq.op === 'eq') { l = eq.kids![0]; r = eq.kids![1] }
  let name = varArg(n, 1)
  if (!name) {
    const unk = new Set<string>()
    unknowns(e, eq, unk)
    if (unk.size !== 1) {
      const names = [...unk].sort()
      if (!names.length) throw new CalcError(n.pos, 'there is nothing unknown to solve for')
      throw new CalcError(n.pos, `say which to solve for: solve(…, ${names[0]})`)
    }
    name = [...unk][0]
  }
  return solve(e, l, r, name, n.pos)
}

function numDeriv(e: Env, body: Node, name: string, at: number, order: number, pos: number): Value {
  const g = fnOf(e, body, name)
  const d = (x: number, k: number): number => {
    if (k === 1) { const h = 1e-4 * Math.max(1, Math.abs(x)); return (-g(x + 2 * h) + 8 * g(x + h) - 8 * g(x - h) + g(x - 2 * h)) / (12 * h) }
    const h = 1e-3 * Math.max(1, Math.abs(x))
    return (d(x + h, k - 1) - d(x - h, k - 1)) / (2 * h)
  }
  const v = d(at, order)
  if (!isFinite(v)) throw new CalcError(pos, 'there is no slope there')
  return num(tidyRoot(Number(v.toPrecision(9))))
}

function derivCall(e: Env, n: Node): Value {
  if (n.kids!.length === 2) return numDeriv(e, n.kids![0], 'x', float(evaluate(e, n.kids![1])), 1, n.pos)
  if (n.kids!.length === 3) {
    const name = varArg(n, 1)
    if (!name) throw new CalcError(n.pos, 'write deriv(expr, x, at)')
    return numDeriv(e, n.kids![0], name, float(evaluate(e, n.kids![2])), 1, n.pos)
  }
  throw new CalcError(n.pos, 'write deriv(x^2, x, 3)')
}

export function integrate(f: (x: number) => number, a: number, b: number): number {
  if (a === b) return 0
  const rec = (a: number, b: number, fa: number, fm: number, fb: number, whole: number, eps: number, depth: number): number => {
    const m = (a + b) / 2, lm = (a + m) / 2, rm = (m + b) / 2
    const flm = f(lm), frm = f(rm)
    const left = ((m - a) / 6) * (fa + 4 * flm + fm), right = ((b - m) / 6) * (fm + 4 * frm + fb)
    if (depth <= 0 || Math.abs(left + right - whole) <= 15 * eps) return left + right + (left + right - whole) / 15
    return rec(a, m, fa, flm, fm, left, eps / 2, depth - 1) + rec(m, b, fm, frm, fb, right, eps / 2, depth - 1)
  }
  const fa = f(a), fb = f(b), fm = f((a + b) / 2)
  return rec(a, b, fa, fm, fb, ((b - a) / 6) * (fa + 4 * fm + fb), 1e-10, 40)
}

function integralCall(e: Env, n: Node): Value {
  let name = 'x', a: Node, b: Node
  if (n.kids!.length === 3) { a = n.kids![1]; b = n.kids![2] }
  else if (n.kids!.length === 4) { const v = varArg(n, 1); if (!v) throw new CalcError(n.pos, 'write integral(expr, x, from, to)'); name = v; a = n.kids![2]; b = n.kids![3] }
  else throw new CalcError(n.pos, 'write integral(x^2, x, 0, 3)')
  const f = fnOf(e, n.kids![0], name)
  const v = integrate(f, float(evaluate(e, a)), float(evaluate(e, b)))
  if (isNaN(v)) throw new CalcError(n.pos, "this area can't be worked out (the curve has a gap)")
  return num(tidyRoot(Number(v.toPrecision(10))))
}

function seriesCall(e: Env, n: Node): Value {
  const name = n.kids![1].name!
  const a = goRound(float(evaluate(e, n.kids![2]))), b = goRound(float(evaluate(e, n.kids![3])))
  if (b - a > 1e6) throw new CalcError(n.pos, 'that is too many steps (at most a million)')
  const c = e.child()
  let acc: Value | undefined
  for (let k = a; k <= b; k++) {
    c.vars.set(name, num(k))
    const v = evaluate(c, n.kids![0])
    acc = !acc ? v : n.name === 'sum' ? add(e, acc, v, 1) : mul(e, acc, v)
  }
  return acc || num(n.name === 'sum' ? 0 : 1)
}

// ---------- formatting ----------

export interface Format { digits?: number; group?: string; point?: string }
function norm(f: Format): Required<Format> {
  let digits = f.digits || 10
  if (digits <= 0 || digits > 17) digits = 10
  const point = f.point || '.'
  let group = f.group ?? ''
  if (group === point) group = ''
  return { digits, group, point }
}

export function formatNumber(x: number, f0: Format = {}): string {
  const f = norm(f0)
  if (isNaN(x)) return 'not a number'
  if (x === Infinity) return '∞'
  if (x === -Infinity) return '-∞'
  if (x === 0) return '0'
  const ax = Math.abs(x)
  if (ax >= 1e15 || ax < 1e-6) return sci(x, f)
  return group(String(Number(x.toPrecision(f.digits))), f)
}

function group(s: string, f: Required<Format>): string {
  const neg = s.startsWith('-')
  if (neg) s = s.slice(1)
  let [ip, fp] = s.split('.')
  if ((f.group && ip.length > 4) || (f.group && ip.length === 4 && f.group !== ' ')) ip = ip.replace(/\B(?=(\d{3})+(?!\d))/g, f.group)
  let out = ip
  if (fp) out += f.point + fp
  return neg ? '-' + out : out
}

function sci(x: number, f: Required<Format>): string {
  const s = x.toExponential(f.digits - 1)
  let [m, e] = s.split('e')
  if (m.includes('.')) m = m.replace(/0+$/, '').replace(/\.$/, '')
  return m.replace('.', f.point) + ' × 10' + superscript(parseInt(e, 10))
}

export function fraction(x: number, maxQ: number): [number, number] | null {
  if (!isFinite(x) || Math.abs(x) > 1e9) return null
  let sign = 1
  if (x < 0) { sign = -1; x = -x }
  let h0 = 0, h1 = 1, k0 = 1, k1 = 0, y = x
  for (let i = 0; i < 40; i++) {
    const a = Math.floor(y)
    ;[h0, h1] = [h1, a * h1 + h0]
    ;[k0, k1] = [k1, a * k1 + k0]
    if (k1 > maxQ) return null
    if (Math.abs(x - h1 / k1) <= 1e-10 * Math.max(1, x)) return [sign * h1, k1]
    const fr = y - a
    if (fr < 1e-15) break
    y = 1 / fr
  }
  return null
}

const WD = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const MO = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const iso = (t: Date) => t.toISOString().slice(0, 10)
export function formatDate(days: number): string { const t = daysDate(days); return `${WD[t.getUTCDay()].slice(0, 3)} ${t.getUTCDate()} ${MO[t.getUTCMonth()]} ${t.getUTCFullYear()}` }

function withUnit(s: string, v: Value): string {
  const u = v.label || unitText(v.u, v.n)
  if (!u) return s
  return u === '°' ? s + u : s + ' ' + u
}

export function show(e: Env, v: Value, f: Format = {}): [string, string] {
  switch (v.k) {
    case K.Bool: return [v.n !== 0 ? 'true' : 'false', '']
    case K.Date: { const t = daysDate(v.n); return v.fmt === 'weekday' ? [WD[t.getUTCDay()], iso(t)] : [formatDate(v.n), iso(t)] }
    case K.List: return ['[' + v.list!.map(x => show(e, x, f)[0]).join(', ') + ']', '']
    case K.Point: return ['(' + v.list!.map(x => show(e, x, f)[0]).join(', ') + ')', '']
    case K.Solved: {
      const parts: string[] = [], alts: string[] = []
      for (const x of v.list!) { const [t, a] = show(e, x, f); parts.push(`${v.name} = ${t}`); if (a) alts.push(`${v.name} ${a}`) }
      return [parts.join(' or '), alts.join(', ')]
    }
  }
  if (v.pct) {
    let n = v.n
    if (Math.abs(n) >= 0.01 && Math.abs(n) < 1e12) n = goRound(n * 100) / 100
    return [formatNumber(n, f) + '%', '']
  }
  if (v.label && v.lscale) v = { ...v, n: v.n / v.lscale }
  switch (v.fmt) {
    case 'hex': case 'bin': case 'oct': {
      const base = { hex: 16, bin: 2, oct: 8 }[v.fmt]
      const pre = { hex: '0x', bin: '0b', oct: '0o' }[v.fmt]
      const s = Math.abs(Math.trunc(v.n)).toString(base).toUpperCase()
      return [(v.n < 0 ? '-' : '') + pre + s, '']
    }
    case 'sci': return [withUnit(sci(v.n, norm(f)), v), '']
    case 'frac': {
      const fr = fraction(v.n, 100000)
      if (fr) return [withUnit(fr[1] === 1 ? String(fr[0]) : `${fr[0]}/${fr[1]}`, v), '']
      return [withUnit('≈ ' + formatNumber(v.n, f), v), '']
    }
  }
  if (v.u.length === 1 && v.u[0].pow === 1 && !v.label) {
    try {
      const d = e.unitDef(v.u[0].name)
      if (d.cur && Math.abs(v.n) >= 0.01 && Math.abs(v.n) < 1e15) return [group(v.n.toFixed(2), norm(f)) + ' ' + d.cur, '']
    } catch { /* */ }
  }
  const text = withUnit(formatNumber(v.n, f), v)
  let alt = ''
  // a fraction beside a decimal when it says more: 1/3 for 0.3333333333, 3/4 for 0.75 — not 1269/50 for 25.38
  if (!v.u.length && v.n !== Math.trunc(v.n)) {
    const point = norm(f).point
    const decimals = text.includes(point) ? text.slice(text.indexOf(point) + 1).length : 0
    const fr = fraction(v.n, 1000)
    if (fr && fr[1] > 1 && (fr[1] <= 12 || decimals >= 6)) alt = `= ${fr[0]}/${fr[1]}`
  }
  return [text, alt]
}

// ---------- sheets ----------

export interface Line { kind: 'value' | 'set' | 'func' | 'empty' | 'text' | 'error'; label?: string; name?: string; text?: string; alt?: string; error?: string; error_pos?: number; number?: number; unit?: string }
export interface Options { units: boolean; deg: boolean; rates?: Record<string, number>; today?: Date; format?: Format }

export function newEnv(o: Options): Env { return new Env(o.units, o.deg, o.rates || {}, o.today || new Date()) }

export function lineOf(e: Env, v: Value, f: Format): Line {
  const [text, alt] = show(e, v, f)
  const ln: Line = { kind: 'value', text, alt: alt || undefined }
  if (v.k === K.Num && isFinite(v.n)) {
    ln.number = v.label && v.lscale ? v.n / v.lscale : v.n
    ln.unit = v.pct ? '%' : (v.label || unitText(v.u, 2) || undefined)
  }
  return ln
}

export function runLine(e: Env, line: string, f: Format = {}): Line {
  const trim = line.trim()
  if (!trim) { e.above = []; return { kind: 'empty' } }
  if (trim.startsWith('#') || trim.startsWith('//')) return { kind: 'text' }
  let st: Stmt
  try { st = parseLine(line, e) } catch (err) { return lineErr(trim, err, '') }
  if (st.kind === 'empty') return { kind: 'text', label: st.label || undefined }
  if (st.kind === 'func') {
    e.funcs.set(st.name!, { params: st.params!, body: st.expr! })
    e.vars.delete(st.name!)
    return { kind: 'func', label: st.label || undefined, name: st.name }
  }
  let v: Value
  try { v = evaluate(e, st.expr!) } catch (err) { return lineErr(trim, err, st.label) }
  const ln = lineOf(e, v, f)
  if (st.label) ln.label = st.label
  if (st.kind === 'assign') { e.vars.set(st.name!, v); e.funcs.delete(st.name!); ln.kind = 'set'; ln.name = st.name }
  if (v.k === K.Num || v.k === K.Date) { e.prev = v; if (v.k === K.Num) e.above.push(v) }
  return ln
}

function lineErr(trim: string, err: unknown, label: string): Line {
  const hasDigit = /\d/.test(trim), hasOp = /[+*/^=()]/.test(trim)
  if (!hasDigit && !hasOp) return { kind: 'text', label: label || undefined }
  const ln: Line = { kind: 'error', label: label || undefined, error: (err as Error).message || String(err) }
  if (err instanceof CalcError) ln.error_pos = err.pos
  return ln
}

export function evalSheet(lines: string[], o: Options): [Line[], Env] {
  const e = newEnv(o)
  return [lines.map(l => runLine(e, l, o.format || {})), e]
}

/** Works out one expression on its own (the quick calculator, converter). */
export function calculate(expr: string, o: Options): Line {
  const e = newEnv(o)
  let last: Line | undefined
  for (const l of expr.split('\n')) { const ln = runLine(e, l, o.format || {}); if (ln.kind !== 'empty' && ln.kind !== 'text') last = ln }
  return last || { kind: 'error', error: 'there is nothing to work out' }
}

/** For graphs and geometry: a plain-number function of named variables. */
export function compileNumeric(node: Node, env: Env, vars: string[]): (...xs: number[]) => number {
  const c = env.child()
  return (...xs: number[]) => {
    vars.forEach((v, i) => c.vars.set(v, num(xs[i])))
    try {
      const v = evaluate(c, node)
      if (v.k === K.Bool) return v.n
      if (v.k !== K.Num) return NaN
      return v.u.length ? c.toSI(v)[0] : float(v)
    } catch { return NaN }
  }
}

export { num as numValue, float as floatOf }

/** A copy of what an environment knows now (later lines do not change it). */
export function snapshot(e: Env): Env {
  const c = new Env(e.units, e.deg, e.rates, e.today)
  c.vars = new Map(e.vars)
  c.funcs = new Map(e.funcs)
  return c
}
