package calc

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

var constants = map[string]float64{
	"pi": math.Pi, "e": math.E, "tau": 2 * math.Pi, "phi": math.Phi, "inf": math.Inf(1), "∞": math.Inf(1), "infinity": math.Inf(1),
}

// physical constants with their units.
var physConsts = map[string]Value{
	"gravity":    {N: 9.80665, U: Units{{"m", 1}, {"s", -2}}},
	"lightspeed": {N: 299792458, U: Units{{"m", 1}, {"s", -1}}},
	"soundspeed": {N: 343, U: Units{{"m", 1}, {"s", -1}}},
}

const maxDepth = 60

// Eval works out one expression.
func (e *Env) Eval(n *Node) (Value, error) {
	if e.depth > maxDepth {
		return Value{}, errors.New("this goes round in circles (a function calls itself too often)")
	}
	switch n.Op {
	case "num":
		return num(n.Num), nil
	case "qty":
		v := Value{N: n.Num, U: n.Units.clone()}
		if n.Scale != 0 && n.Scale != 1 {
			// 6.4 L/100km: kept per km, shown per 100 km
			v.N /= n.Scale
			v.LScale = 1 / n.Scale
			v.Label = labelOf(Value{N: v.LScale, U: v.U})
		}
		return v, nil
	case "unit":
		return Value{N: 1, U: Units{{n.Name, 1}}}, nil
	case "paren":
		return e.Eval(n.Kids[0])
	case "date":
		t, err := time.Parse("2006-01-02", n.Name)
		if err != nil {
			return Value{}, errf(n.Pos, n.Name+" is not a date")
		}
		return Value{K: KDate, N: dateDays(t)}, nil
	case "id":
		return e.ident(n)
	case "neg":
		v, err := e.Eval(n.Kids[0])
		if err != nil {
			return v, err
		}
		return e.neg(v)
	case "pct":
		v, err := e.Eval(n.Kids[0])
		if err != nil {
			return v, err
		}
		if v.K != KNum || len(v.U) > 0 || v.Pct {
			return v, errf(n.Pos, "only a plain number can be a percentage")
		}
		v.Pct = true
		return v, nil
	case "cur":
		v, err := e.Eval(n.Kids[0])
		if err != nil {
			return v, err
		}
		return e.mul(v, Value{N: 1, U: Units{{n.Name, 1}}})
	case "+", "-", "*", "/", "mod", "^", "imp", "of", "off", "on":
		a, err := e.Eval(n.Kids[0])
		if err != nil {
			return a, err
		}
		b, err := e.Eval(n.Kids[1])
		if err != nil {
			return b, err
		}
		v, err := e.binary(n, a, b)
		if err != nil {
			var pe *Error
			if !errors.As(err, &pe) {
				err = errf(n.Pos, err.Error())
			}
		}
		return v, err
	case "!":
		v, err := e.Eval(n.Kids[0])
		if err != nil {
			return v, err
		}
		if v.K != KNum || len(v.U) > 0 || v.N < 0 || v.N != math.Floor(v.N) {
			return v, errf(n.Pos, "! needs a whole number of 0 or more")
		}
		if v.N > 170 {
			return num(math.Inf(1)), nil
		}
		return num(math.Round(math.Gamma(v.N + 1))), nil
	case "list":
		out := Value{K: KList}
		for _, k := range n.Kids {
			v, err := e.Eval(k)
			if err != nil {
				return v, err
			}
			if v.K == KList {
				out.List = append(out.List, v.List...)
			} else {
				out.List = append(out.List, v)
			}
		}
		return out, nil
	case "tuple":
		out := Value{K: KPoint}
		for _, k := range n.Kids {
			v, err := e.Eval(k)
			if err != nil {
				return v, err
			}
			out.List = append(out.List, v)
		}
		return out, nil
	case "conv":
		v, err := e.Eval(n.Kids[0])
		if err != nil {
			return v, err
		}
		if n.Name != "" {
			return e.special(v, n.Name, n.Pos)
		}
		return e.convertTo(v, n.Kids[1])
	case "call":
		return e.call(n)
	case "eq", "lt", "gt", "le", "ge", "ne":
		return e.equation(n)
	}
	return Value{}, errf(n.Pos, "I can't work this out")
}

func (e *Env) ident(n *Node) (Value, error) {
	if v, ok := e.lookup(n.Name); ok {
		return v, nil
	}
	if c, ok := constants[n.Name]; ok {
		return num(c), nil
	}
	if c, ok := physConsts[n.Name]; ok && e.Units {
		return c, nil
	}
	switch n.Name {
	case "ans", "prev", "previous", "last":
		if e.prev != nil {
			return *e.prev, nil
		}
		return Value{}, errf(n.Pos, "there is no line above to use")
	case "total", "sum", "subtotal":
		return e.total(n.Pos, false)
	case "average", "avg", "mean":
		return e.total(n.Pos, true)
	case "today", "now":
		return Value{K: KDate, N: dateDays(e.Today)}, nil
	case "tomorrow":
		return Value{K: KDate, N: dateDays(e.Today) + 1}, nil
	case "yesterday":
		return Value{K: KDate, N: dateDays(e.Today) - 1}, nil
	case "true", "yes":
		return Value{K: KBool, N: 1}, nil
	case "false", "no":
		return Value{K: KBool, N: 0}, nil
	}
	if e.Units {
		if _, err := e.unitDef(n.Name); err == nil {
			return Value{N: 1, U: Units{{canonicalCurrency(n.Name), 1}}}, nil
		}
	}
	if _, ok := e.fn(n.Name); ok || builtinFunc(n.Name) {
		return Value{}, errf(n.Pos, n.Name+" is a function: write "+n.Name+"(…)")
	}
	return Value{}, errf(n.Pos, "I don't know \""+n.Name+"\" yet")
}

// total adds up the results above (back to an empty line).
func (e *Env) total(pos int, avg bool) (Value, error) {
	var acc *Value
	k := 0
	for _, v := range e.above {
		if v.K != KNum || v.Pct {
			continue
		}
		if acc == nil {
			c := v
			acc = &c
			k++
			continue
		}
		s, err := e.add(*acc, v, 1)
		if err != nil {
			continue
		}
		acc = &s
		k++
	}
	if acc == nil {
		return num(0), nil
	}
	if avg {
		acc.N /= float64(k)
	}
	return *acc, nil
}

func (e *Env) neg(v Value) (Value, error) {
	switch v.K {
	case KNum:
		v.N = -v.N
		return v, nil
	case KList:
		out := Value{K: KList}
		for _, x := range v.List {
			y, err := e.neg(x)
			if err != nil {
				return y, err
			}
			out.List = append(out.List, y)
		}
		return out, nil
	}
	return v, errors.New("this can't be made negative")
}

func (e *Env) binary(n *Node, a, b Value) (Value, error) {
	// lists work item by item
	if a.K == KList || b.K == KList {
		return e.listOp(n, a, b)
	}
	switch n.Op {
	case "+":
		return e.add(a, b, 1)
	case "-":
		return e.add(a, b, -1)
	case "*":
		return e.mul(a, b)
	case "imp":
		// 2 h 30 min, 5 ft 3 in: amounts of the same thing written together add up
		if n.Kids[1].Op == "qty" && len(a.U) > 0 && a.K == KNum {
			da, err1 := e.dimOf(a.U)
			db, err2 := e.dimOf(b.U)
			if err1 == nil && err2 == nil && da == db && !da.zero() {
				return e.add(a, b, 1)
			}
		}
		return e.mul(a, b)
	case "/":
		return e.div(a, b)
	case "mod":
		if a.K != KNum || b.K != KNum {
			return Value{}, errors.New("mod needs numbers")
		}
		bn, err := e.sameAs(b, a)
		if err != nil {
			return Value{}, err
		}
		if bn == 0 {
			return Value{}, errors.New("mod by zero")
		}
		r := math.Mod(a.N, bn)
		if r != 0 && (r < 0) != (bn < 0) {
			r += bn
		}
		a.N = r
		return a, nil
	case "^":
		return e.pow(a, b)
	case "of", "off", "on":
		if !a.Pct {
			if n.Op == "of" {
				return e.mul(a, b)
			}
			return Value{}, errors.New("write a percentage before \"" + n.Op + "\", like 20% " + n.Op + " 50")
		}
		p := a.N / 100
		switch n.Op {
		case "off":
			p = 1 - p
		case "on":
			p = 1 + p
		}
		return e.mul(num(p), b)
	}
	return Value{}, errors.New("I can't work this out")
}

func (e *Env) listOp(n *Node, a, b Value) (Value, error) {
	out := Value{K: KList}
	switch {
	case a.K == KList && b.K == KList:
		if len(a.List) != len(b.List) {
			return out, errors.New("the lists are not the same length")
		}
		for i := range a.List {
			v, err := e.binary(n, a.List[i], b.List[i])
			if err != nil {
				return v, err
			}
			out.List = append(out.List, v)
		}
	case a.K == KList:
		for _, x := range a.List {
			v, err := e.binary(n, x, b)
			if err != nil {
				return v, err
			}
			out.List = append(out.List, v)
		}
	default:
		for _, x := range b.List {
			v, err := e.binary(n, a, x)
			if err != nil {
				return v, err
			}
			out.List = append(out.List, v)
		}
	}
	return out, nil
}

// sameAs gives b's amount in a's units.
func (e *Env) sameAs(b, a Value) (float64, error) {
	if len(a.U) == 0 && len(b.U) == 0 {
		return b.Float(), nil
	}
	return e.convert(b, a.U)
}

func (e *Env) add(a, b Value, sign float64) (Value, error) {
	// dates
	if a.K == KDate || b.K == KDate {
		return e.addDate(a, b, sign)
	}
	if a.K != KNum || b.K != KNum {
		return Value{}, errors.New("only numbers can be added")
	}
	// 50 + 10% is 55
	if b.Pct && !a.Pct {
		return e.mul(a, num(1+sign*b.N/100))
	}
	if a.Pct && b.Pct {
		a.N += sign * b.N
		return a, nil
	}
	if a.Pct && !b.Pct {
		a = num(a.N / 100)
	}
	if len(a.U) == 0 && len(b.U) > 0 {
		if a.N == 0 {
			b.N *= sign
			return b, nil
		}
		db, _ := e.dimOf(b.U)
		if db.zero() { // 1 + 30° (angles are numbers)
			bn, err := e.convert(b, nil)
			if err == nil {
				a.N += sign * bn
				return a, nil
			}
		}
		return Value{}, fmt.Errorf("can't add a plain number and %s", unitText(b.U, 2))
	}
	if len(b.U) == 0 && len(a.U) > 0 {
		if b.N == 0 {
			return a, nil
		}
		da, _ := e.dimOf(a.U)
		if da.zero() {
			an, err := e.convert(a, nil)
			if err == nil {
				return num(an + sign*b.N), nil
			}
		}
		return Value{}, fmt.Errorf("can't add %s and a plain number", unitText(a.U, 2))
	}
	bn, err := e.convertDelta(b, a.U)
	if err != nil {
		return Value{}, err
	}
	a.N += sign * bn
	a.Label = ""
	return a, nil
}

// convertDelta is convert for amounts being added: 20 °C + 5 K is 25 °C.
func (e *Env) convertDelta(b Value, to Units) (float64, error) {
	if sameUnits(b.U, to) {
		return b.N, nil
	}
	x, err := e.info(b.U)
	if err != nil {
		return 0, err
	}
	y, err := e.info(to)
	if err != nil {
		return 0, err
	}
	if x.dim != y.dim {
		return 0, fmt.Errorf("can't add %s and %s: they measure different things", unitText(to, 2), unitText(b.U, 2))
	}
	return b.N * x.factor / y.factor, nil
}

func (e *Env) addDate(a, b Value, sign float64) (Value, error) {
	if a.K == KDate && b.K == KDate {
		if sign > 0 {
			return Value{}, errors.New("two dates can't be added (take one from the other)")
		}
		return Value{N: a.N - b.N, U: Units{{"day", 1}}}, nil
	}
	if b.K == KDate {
		if sign < 0 {
			return Value{}, errors.New("a date can't be taken from a number")
		}
		a, b = b, a
	}
	if len(b.U) == 0 {
		return Value{}, errors.New("say what to add to a date: days, weeks, months or years")
	}
	d, err := e.dimOf(b.U)
	if err != nil {
		return Value{}, err
	}
	if d != tm {
		return Value{}, errors.New("only time can be added to a date")
	}
	t := daysDate(a.N)
	// whole months and years move the calendar, the rest counts days
	if len(b.U) == 1 && b.U[0].Pow == 1 && b.N == math.Trunc(b.N) {
		switch b.U[0].Name {
		case "month":
			return Value{K: KDate, N: dateDays(addMonths(t, int(sign*b.N)))}, nil
		case "year":
			return Value{K: KDate, N: dateDays(addMonths(t, 12*int(sign*b.N)))}, nil
		}
	}
	secs, _, err := e.toSI(b)
	if err != nil {
		return Value{}, err
	}
	return Value{K: KDate, N: a.N + sign*math.Round(secs/day*1e6)/1e6}, nil
}

func (e *Env) mul(a, b Value) (Value, error) {
	if a.K == KDate || b.K == KDate {
		return Value{}, errors.New("a date can't be multiplied")
	}
	if a.K != KNum || b.K != KNum {
		return Value{}, errors.New("only numbers can be multiplied")
	}
	us, scale, err := e.mulUnits(a.U, b.U, 1)
	if err != nil {
		return Value{}, err
	}
	out := Value{N: a.Float() * b.Float() * scale, U: us}
	if a.Pct && b.Pct {
		out = Value{N: a.N * b.N / 100, Pct: true}
	}
	return e.tidy(out), nil
}

func (e *Env) div(a, b Value) (Value, error) {
	if a.K == KDate || b.K == KDate {
		return Value{}, errors.New("a date can't be divided")
	}
	if a.K != KNum || b.K != KNum {
		return Value{}, errors.New("only numbers can be divided")
	}
	us, scale, err := e.mulUnits(a.U, b.U, -1)
	if err != nil {
		return Value{}, err
	}
	bf := b.Float()
	if bf == 0 {
		return Value{}, errors.New("can't divide by zero")
	}
	if a.Pct && b.Pct {
		return num(a.N / b.N), nil
	}
	return e.tidy(Value{N: a.Float() / bf * scale, U: us}), nil
}

// tidy turns units that cancel out (m/m, and angles) into a plain number,
// and a mix that comes down to one of its own units into that unit
// (W·h·EUR/kWh is EUR).
func (e *Env) tidy(v Value) Value {
	if len(v.U) == 0 {
		return v
	}
	d, err := e.dimOf(v.U)
	if err == nil && len(v.U) > 1 && !d.zero() {
		for _, u := range v.U {
			def, err := e.unitDef(u.Name)
			if err != nil || u.Pow != 1 || def.Dim != d || def.Offset != 0 {
				continue
			}
			if n, err := e.convert(v, Units{u}); err == nil {
				return Value{N: n, U: Units{u}}
			}
		}
	}
	if err == nil && d.zero() {
		// keep a lone angle unit: 30°
		if len(v.U) == 1 && v.U[0].Pow == 1 {
			return v
		}
		if n, err := e.convert(v, nil); err == nil {
			return num(n)
		}
	}
	return v
}

func (e *Env) pow(a, b Value) (Value, error) {
	if a.K != KNum || b.K != KNum {
		return Value{}, errors.New("only numbers can be raised to a power")
	}
	if len(b.U) > 0 {
		return Value{}, errors.New("a power can't have a unit")
	}
	p := b.Float()
	if len(a.U) == 0 {
		return num(math.Pow(a.Float(), p)), nil
	}
	out := Value{N: math.Pow(a.N, p)}
	for _, u := range a.U {
		np := float64(u.Pow) * p
		if math.Abs(np-math.Round(np)) > 1e-9 {
			return Value{}, fmt.Errorf("%s can't be raised to the power %g", unitText(a.U, 2), p)
		}
		if int(math.Round(np)) != 0 {
			out.U = append(out.U, UnitPow{u.Name, int(math.Round(np))})
		}
	}
	return out, nil
}

// convertTo: "60 mph to km/h", "6.5 L/100km to mpg", "100 EUR in BAM".
func (e *Env) convertTo(v Value, tn *Node) (Value, error) {
	if v.K == KList {
		out := Value{K: KList}
		for _, x := range v.List {
			y, err := e.convertTo(x, tn)
			if err != nil {
				return y, err
			}
			out.List = append(out.List, y)
		}
		return out, nil
	}
	if v.K == KDate {
		return Value{}, errf(tn.Pos, "a date can't be turned into a unit")
	}
	saveU := e.Units
	e.Units = true
	t, err := e.Eval(tn)
	e.Units = saveU
	if err != nil {
		return t, err
	}
	if t.K != KNum || len(t.U) == 0 {
		return Value{}, errf(tn.Pos, "write a unit after \"to\"")
	}
	if len(v.U) == 0 && !v.Pct {
		// 5 to km: a plain number takes the unit
		out := Value{N: v.N, U: t.U.clone()}
		if t.N != 1 {
			out.N, out.Label, out.LScale = v.N*t.N, labelOf(t), t.N
		}
		return out, nil
	}
	if v.Pct {
		v = num(v.N / 100)
	}
	x, dx, err := e.toSI(v)
	if err != nil {
		return Value{}, errf(tn.Pos, err.Error())
	}
	ti, err := e.info(t.U)
	if err != nil {
		return Value{}, errf(tn.Pos, err.Error())
	}
	f := t.N * ti.factor
	switch {
	case dx == ti.dim:
		if t.N != 1 {
			return Value{N: x / ti.factor, U: t.U.clone(), Label: labelOf(t), LScale: t.N}, nil
		}
		return Value{N: x/f - ti.offset, U: t.U.clone()}, nil
	case dx == (Dim{}).add(ti.dim, -1) && x != 0:
		// L/100km ↔ km/L ↔ mpg
		v := Value{N: 1 / (x * ti.factor), U: t.U.clone()}
		if t.N != 1 {
			v.Label, v.LScale = labelOf(t), t.N
		}
		return v, nil
	}
	return Value{}, errf(tn.Pos, fmt.Sprintf("%s can't be turned into %s: they measure different things", firstNonEmpty(unitText(v.U, 2), "a plain number"), unitText(t.U, 2)))
}

// labelOf keeps "L/100km" as written when the target has a number in it.
func labelOf(t Value) string {
	if t.N == 1 {
		return ""
	}
	var pos, neg []string
	for _, u := range t.U {
		s := u.Name
		if d, ok := lookupUnit(u.Name); ok {
			s = d.Sym
		}
		p := u.Pow
		if p < 0 {
			p = -p
		}
		if p != 1 {
			s += superscript(p)
		}
		if u.Pow > 0 {
			pos = append(pos, s)
		} else {
			neg = append(neg, s)
		}
	}
	numTxt := trimFloat(1 / t.N)
	if t.N > 1 {
		return trimFloat(t.N) + " " + strings.Join(pos, "·") + "/" + strings.Join(neg, "·")
	}
	return firstNonEmpty(strings.Join(pos, "·"), "1") + "/" + numTxt + strings.Join(neg, "·")
}

func trimFloat(f float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
}

func (e *Env) special(v Value, how string, pos int) (Value, error) {
	switch how {
	case "pct":
		if v.K != KNum || len(v.U) > 0 {
			return v, errf(pos, "only a plain number can be shown as a percentage")
		}
		if v.Pct {
			return v, nil
		}
		return Value{N: v.N * 100, Pct: true}, nil
	case "dec":
		if v.Pct {
			return num(v.N / 100), nil
		}
		v.Fmt = ""
		return v, nil
	case "hex", "bin", "oct":
		if v.K != KNum || len(v.U) > 0 || v.N != math.Trunc(v.N) || math.Abs(v.N) > 1<<53 {
			return v, errf(pos, "only a whole number can be shown in "+map[string]string{"hex": "hexadecimal", "bin": "binary", "oct": "octal"}[how])
		}
	}
	v.Fmt = how
	return v, nil
}

// ---- equations ----

// equation: with one unknown it is solved ("2x + 3 = 11" → x = 4); with
// none it is checked.
func (e *Env) equation(n *Node) (Value, error) {
	unknown := map[string]bool{}
	e.unknowns(n, unknown)
	names := make([]string, 0, len(unknown))
	for k := range unknown {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(names) == 0 {
		a, err := e.Eval(n.Kids[0])
		if err != nil {
			return a, err
		}
		b, err := e.Eval(n.Kids[1])
		if err != nil {
			return b, err
		}
		return e.compare(n.Op, a, b)
	}
	if n.Op != "eq" {
		return Value{}, errf(n.Pos, "an inequality with an unknown is drawn in a graph, not worked out here")
	}
	if len(names) > 1 {
		return Value{}, errf(n.Pos, "there is more than one unknown ("+strings.Join(names, ", ")+"); set all but one first")
	}
	return e.solve(n.Kids[0], n.Kids[1], names[0], n.Pos)
}

func (e *Env) unknowns(n *Node, out map[string]bool) {
	if n == nil {
		return
	}
	if n.Op == "id" {
		if _, ok := e.ident(n); ok != nil {
			if _, isFn := e.fn(n.Name); !isFn && !builtinFunc(n.Name) {
				out[n.Name] = true
			}
		}
	}
	if n.Op == "call" && (n.Name == "solve" || n.Name == "integral" || n.Name == "integrate" || n.Name == "deriv" || n.Name == "derivative" || n.Name == "diff") {
		return
	}
	for _, k := range n.Kids {
		e.unknowns(k, out)
	}
}

func (e *Env) compare(op string, a, b Value) (Value, error) {
	if a.K == KDate && b.K == KDate {
		return boolV(cmp(op, a.N, b.N, 1e-12)), nil
	}
	x, da, err := e.toSI(a)
	if err != nil {
		return Value{}, err
	}
	y, db, err := e.toSI(b)
	if err != nil {
		return Value{}, err
	}
	if da != db {
		return Value{}, errors.New("these two measure different things")
	}
	return boolV(cmp(op, x, y, 1e-9*math.Max(1, math.Max(math.Abs(x), math.Abs(y))))), nil
}

func cmp(op string, x, y, eps float64) bool {
	switch op {
	case "eq":
		return math.Abs(x-y) <= eps
	case "ne":
		return math.Abs(x-y) > eps
	case "lt":
		return x < y-eps
	case "gt":
		return x > y+eps
	case "le":
		return x <= y+eps
	case "ge":
		return x >= y-eps
	}
	return false
}

func boolV(b bool) Value {
	if b {
		return Value{K: KBool, N: 1}
	}
	return Value{K: KBool}
}

// fnOf turns an expression into a function of one unknown (plain numbers;
// an amount with units is taken in SI units).
func (e *Env) fnOf(body *Node, name string) func(float64) (float64, error) {
	c := e.child()
	return func(t float64) (float64, error) {
		c.Vars[name] = num(t)
		v, err := c.Eval(body)
		if err != nil {
			return 0, err
		}
		if v.K == KBool {
			return v.N, nil
		}
		if v.K != KNum {
			return 0, errors.New("this does not give a number")
		}
		x, _, err := c.toSI(v)
		return x, err
	}
}

// addMonths moves a date by whole months; the 31st becomes the month's last
// day when the month is shorter (31 Jan + 1 month = 28 Feb).
func addMonths(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	if d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.UTC)
}
