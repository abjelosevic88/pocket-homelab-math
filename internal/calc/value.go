package calc

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Kind of a value.
type Kind int

const (
	KNum Kind = iota
	KList
	KDate
	KBool
	KSolved // the answer to an equation: Name = List
	KPoint
)

// Value is a number with its units (as written), or a list, a date (days
// since 1970-01-01), true/false or the solutions of an equation.
type Value struct {
	K      Kind
	N      float64
	U      Units
	Pct    bool    // a percentage: N is 20 for 20%
	List   []Value // list items, solutions, point coordinates
	Fmt    string  // show as: hex | bin | oct | sci | frac | pct | dec
	Label  string  // a unit as written in "to L/100km"
	LScale float64 // with Label: the number shown is N / LScale
	Name   string  // the unknown an equation was solved for
}

func num(n float64) Value { return Value{N: n} }

// Float is the plain number (a percentage as a fraction).
func (v Value) Float() float64 {
	if v.Pct {
		return v.N / 100
	}
	return v.N
}

// Env is what a sheet knows while it is worked out, line by line.
type Env struct {
	Units  bool               // read units and currencies (off for graphs)
	Deg    bool               // plain numbers in sin/cos/tan are degrees
	Rates  map[string]float64 // currency per 1 EUR
	Today  time.Time
	Vars   map[string]Value
	Funcs  map[string]*Func
	parent *Env
	prev   *Value
	above  []Value
	depth  int
}

// Func is a function set in a sheet: f(x) = x^2 + 1.
type Func struct {
	Params []string
	Body   *Node
}

// NewEnv makes an empty environment.
func NewEnv(units, deg bool, rates map[string]float64) *Env {
	return &Env{Units: units, Deg: deg, Rates: rates, Today: time.Now(), Vars: map[string]Value{}, Funcs: map[string]*Func{}}
}

func (e *Env) child() *Env {
	return &Env{Units: e.Units, Deg: e.Deg, Rates: e.Rates, Today: e.Today, Vars: map[string]Value{}, Funcs: map[string]*Func{}, parent: e, depth: e.depth + 1}
}

func (e *Env) lookup(name string) (Value, bool) {
	for x := e; x != nil; x = x.parent {
		if v, ok := x.Vars[name]; ok {
			return v, true
		}
	}
	return Value{}, false
}

func (e *Env) fn(name string) (*Func, bool) {
	for x := e; x != nil; x = x.parent {
		if f, ok := x.Funcs[name]; ok {
			return f, true
		}
	}
	return nil, false
}

func (e *Env) known(name string) bool {
	if _, ok := e.lookup(name); ok {
		return true
	}
	_, ok := e.fn(name)
	return ok
}

func (e *Env) hasCurrency(code string) bool {
	_, ok := e.Rates[code]
	return ok
}

// ---- units of a value ----

type unitInfo struct {
	factor float64
	dim    Dim
	offset float64 // only for a lone °C / °F
}

func (e *Env) unitDef(name string) (*unitDef, error) {
	if u, ok := lookupUnit(name); ok {
		return u, nil
	}
	code := canonicalCurrency(name)
	if isCurrency(code, e.hasCurrency) {
		d := &unitDef{Sym: code, Dim: money, Kind: "money", Cur: code, Factor: math.NaN()}
		if code == "EUR" {
			d.Factor = 1
		} else if r, ok := e.Rates[code]; ok && r > 0 {
			d.Factor = 1 / r
		}
		return d, nil
	}
	return nil, fmt.Errorf("I don't know the unit %s", name)
}

func (e *Env) info(us Units) (unitInfo, error) {
	in := unitInfo{factor: 1}
	for _, u := range us {
		d, err := e.unitDef(u.Name)
		if err != nil {
			return in, err
		}
		if math.IsNaN(d.Factor) {
			return in, fmt.Errorf("there is no exchange rate for %s yet", d.Cur)
		}
		in.factor *= math.Pow(d.Factor, float64(u.Pow))
		in.dim = in.dim.add(d.Dim, int8(u.Pow))
		if len(us) == 1 && u.Pow == 1 {
			in.offset = d.Offset
		}
	}
	return in, nil
}

// dimOf is the dimension without needing exchange rates.
func (e *Env) dimOf(us Units) (Dim, error) {
	var d Dim
	for _, u := range us {
		def, err := e.unitDef(u.Name)
		if err != nil {
			return d, err
		}
		d = d.add(def.Dim, int8(u.Pow))
	}
	return d, nil
}

func sameUnits(a, b Units) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// convert gives v's amount in other units (same dimension).
func (e *Env) convert(v Value, to Units) (float64, error) {
	if sameUnits(v.U, to) {
		return v.N, nil
	}
	a, err := e.info(v.U)
	if err != nil {
		return 0, err
	}
	b, err := e.info(to)
	if err != nil {
		return 0, err
	}
	if a.dim != b.dim {
		return 0, fmt.Errorf("%s and %s do not measure the same thing", unitText(v.U, 2), unitText(to, 2))
	}
	si := (v.N + a.offset) * a.factor
	return si/b.factor - b.offset, nil
}

// toSI is the amount in SI base units (euros for money).
func (e *Env) toSI(v Value) (float64, Dim, error) {
	if len(v.U) == 0 {
		return v.Float(), Dim{}, nil
	}
	in, err := e.info(v.U)
	if err != nil {
		return 0, Dim{}, err
	}
	return (v.N + in.offset) * in.factor, in.dim, nil
}

// mulUnits multiplies two unit lists: the same unit adds its powers; a unit
// of the same single quantity (m and cm) is turned into the left one, so
// "2 m × 50 cm" is 1 m².
func (e *Env) mulUnits(a, b Units, sign int) (Units, float64, error) {
	out := a.clone()
	scale := 1.0
	for _, ub := range b {
		ub.Pow *= sign
		done := false
		for i, ua := range out {
			if ua.Name == ub.Name {
				out[i].Pow += ub.Pow
				done = true
				break
			}
		}
		if !done {
			db, err := e.unitDef(ub.Name)
			if err != nil {
				return nil, 0, err
			}
			for i, ua := range out {
				da, err := e.unitDef(ua.Name)
				if err != nil {
					return nil, 0, err
				}
				if da.Dim == db.Dim && !da.Dim.zero() && da.Offset == 0 && db.Offset == 0 {
					if math.IsNaN(da.Factor) {
						return nil, 0, fmt.Errorf("there is no exchange rate for %s yet", da.Cur)
					}
					if math.IsNaN(db.Factor) {
						return nil, 0, fmt.Errorf("there is no exchange rate for %s yet", db.Cur)
					}
					scale *= math.Pow(db.Factor/da.Factor, float64(ub.Pow))
					out[i].Pow += ub.Pow
					done = true
					break
				}
			}
		}
		if !done {
			out = append(out, ub)
		}
	}
	clean := out[:0]
	for _, u := range out {
		if u.Pow != 0 {
			clean = append(clean, u)
		}
	}
	return clean, scale, nil
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

// ---- dates ----

func dateDays(t time.Time) float64 {
	y, m, d := t.Date()
	return float64(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

func daysDate(n float64) time.Time {
	return time.Unix(int64(math.Floor(n))*86400, 0).UTC()
}

func unitText(us Units, n float64) string {
	if len(us) == 0 {
		return ""
	}
	var pos, neg []string
	for _, u := range us {
		name := u.Name
		if d, ok := lookupUnit(u.Name); ok {
			name = d.Sym
			if d.Plural != "" && len(us) == 1 && u.Pow == 1 && math.Abs(n) != 1 {
				name = d.Plural
			}
		}
		p := u.Pow
		if p < 0 {
			p = -p
		}
		s := name
		if p != 1 {
			s += superscript(p)
		}
		if u.Pow > 0 {
			pos = append(pos, s)
		} else {
			neg = append(neg, s)
		}
	}
	txt := strings.Join(pos, "·")
	// kW·h reads as kWh
	if len(us) == 2 && us[0].Pow == 1 && us[1].Pow == 1 && us[1].Name == "h" && strings.HasSuffix(us[0].Name, "W") {
		txt = us[0].Name + "h"
	}
	if len(neg) > 0 {
		if txt == "" {
			txt = "1"
		}
		if len(neg) > 1 {
			txt += "/(" + strings.Join(neg, "·") + ")"
		} else {
			txt += "/" + neg[0]
		}
	}
	return txt
}

func superscript(n int) string {
	const digits = "⁰¹²³⁴⁵⁶⁷⁸⁹"
	s := fmt.Sprint(n)
	var b strings.Builder
	for _, c := range s {
		if c == '-' {
			b.WriteString("⁻")
			continue
		}
		b.WriteString(string([]rune(digits)[c-'0']))
	}
	return b.String()
}

// Break starts a new block: "total" adds up only what comes after it.
func (e *Env) Break() { e.above = nil }
