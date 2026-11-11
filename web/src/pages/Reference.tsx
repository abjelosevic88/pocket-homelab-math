// What can be written: examples, units, functions.
import { Link } from 'react-router-dom'
import { builtinSet, converterKinds } from '../lib/calc'

const EXAMPLES: [string, string][] = [
  ['2 + 3 × 4', 'normal order: 14'], ['(2 + 3) × 4', 'brackets first: 20'], ['2^10 or 2**10', 'powers'], ['5!', 'factorial'], ['√2, sqrt(2)', 'square root'],
  ['5 km + 300 m', 'units: 5.3 km'], ['60 mph to km/h', 'conversion'], ['3 m × 4 m', '12 m²'], ['20 °C to °F', 'temperature'], ['2 h 30 min to min', 'mixed units: 150 min'],
  ['6.5 L/100km to mpg', 'fuel use'], ['100 EUR in BAM', 'money (European Central Bank rates)'], ['$20 + €15', 'symbols work too'],
  ['20% of 80', '16'], ['50 + 10%', '55'], ['10% off 80', '72'], ['0.25 to %', '25%'],
  ['price = 12.5 EUR', 'a name for a number, used below'], ['f(x) = x^2 + 1', 'a function; then f(3), and its slope f\'(3)'],
  ['Rent: 450 EUR', 'words before a colon are a label'], ['total', 'adds up the lines above (back to an empty line)'], ['ans', 'the line above'],
  ['2x + 3 = 11', 'solved: x = 4'], ['solve(x^2 = 9)', 'every solution'], ['integral(x^2, x, 0, 3)', 'area under a curve'], ['deriv(x^3, x, 2)', 'slope at a point'],
  ['sum(k^2, k, 1, 10)', 'a series: 385'], ['mean(2, 4, 9)', 'and median, sd, min, max'], ['pmt(4%/12, 240, 150000 EUR)', 'a loan payment'],
  ['2026-12-25 - today', 'days between'], ['today + 3 weeks', 'a date'], ['255 to hex', 'and bin, oct, sci, fraction'], ['# a note', 'a comment'],
]

export default function Reference() {
  return (
    <div className="stack loose ref">
      <div className="page-head"><div className="grow"><h1 className="page-title">What you can write</h1><p className="muted">In a calculation, write each sum on its own line; the answer shows beside it. Try them on the <Link to="/">start page</Link>.</p></div></div>
      <section className="card"><table className="dtable ref-table"><tbody>{EXAMPLES.map(([a, b]) => <tr key={a}><td><code>{a}</code></td><td className="muted">{b}</td></tr>)}</tbody></table></section>
      <section className="card pad-card"><h3>Graphs</h3><p className="muted"><code>y = 2x + 1</code> · <code>f(x) = sin(x)</code> · <code>x² + y² = 25</code> · <code>y &lt; x^2</code> (shaded) · <code>x = 3</code> · <code>r = 1 + cos(θ)</code> · <code>(cos(t), sin(t))</code> · <code>(2, 3)</code> · <code>a = 2</code> (a slider). A function set in a calculation above can be drawn too.</p></section>
      <section className="card pad-card"><h3>Units</h3>{converterKinds.map(k => <p key={k.kind}><b>{k.kind}</b> <span className="muted">{k.units.join(', ')}</span></p>)}<p className="muted small">SI prefixes work on SI units: mm, km, mg, kWh, MB, µs… Currencies: ISO codes (EUR, USD, BAM, KM…) and €, $, £.</p></section>
      <section className="card pad-card"><h3>Functions</h3><p className="muted">{[...builtinSet].sort().join(', ')}</p></section>
    </div>
  )
}
