package calc

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// FuncHelp describes a built-in function (for the help sheet and completion).
type FuncHelp struct {
	Name  string `json:"name"`
	Usage string `json:"usage"`
	About string `json:"about"`
	Group string `json:"group"`
}

// Builtins lists the functions everyone can use.
var Builtins = []FuncHelp{
	{"sqrt", "sqrt(x)", "square root (also √x)", "Powers"},
	{"cbrt", "cbrt(x)", "cube root", "Powers"},
	{"root", "root(x, n)", "n-th root", "Powers"},
	{"exp", "exp(x)", "e to the power x", "Powers"},
	{"ln", "ln(x)", "natural logarithm", "Powers"},
	{"log", "log(x) · log(x, base)", "logarithm, base 10 unless given", "Powers"},
	{"log2", "log2(x)", "logarithm base 2", "Powers"},
	{"abs", "abs(x) · |x|", "distance from zero", "Numbers"},
	{"round", "round(x) · round(x, digits)", "round to the nearest", "Numbers"},
	{"floor", "floor(x)", "round down", "Numbers"},
	{"ceil", "ceil(x)", "round up", "Numbers"},
	{"trunc", "trunc(x)", "drop the decimals", "Numbers"},
	{"sign", "sign(x)", "−1, 0 or 1", "Numbers"},
	{"mod", "mod(a, b) · a mod b", "what is left after dividing", "Numbers"},
	{"gcd", "gcd(a, b, …)", "greatest common divisor", "Numbers"},
	{"lcm", "lcm(a, b, …)", "least common multiple", "Numbers"},
	{"factorial", "factorial(n) · n!", "1 × 2 × … × n", "Numbers"},
	{"nCr", "nCr(n, k)", "ways to choose k of n", "Numbers"},
	{"nPr", "nPr(n, k)", "ordered ways to pick k of n", "Numbers"},
	{"hypot", "hypot(a, b)", "√(a² + b²)", "Numbers"},
	{"sin", "sin(x)", "sine", "Angles"},
	{"cos", "cos(x)", "cosine", "Angles"},
	{"tan", "tan(x)", "tangent", "Angles"},
	{"asin", "asin(x)", "angle with this sine", "Angles"},
	{"acos", "acos(x)", "angle with this cosine", "Angles"},
	{"atan", "atan(x) · atan(y, x)", "angle with this tangent", "Angles"},
	{"sinh", "sinh(x)", "hyperbolic sine", "Angles"},
	{"cosh", "cosh(x)", "hyperbolic cosine", "Angles"},
	{"tanh", "tanh(x)", "hyperbolic tangent", "Angles"},
	{"sec", "sec(x)", "1/cos", "Angles"},
	{"csc", "csc(x)", "1/sin", "Angles"},
	{"cot", "cot(x)", "1/tan", "Angles"},
	{"min", "min(a, b, …)", "the smallest", "Lists"},
	{"max", "max(a, b, …)", "the largest", "Lists"},
	{"sum", "sum(a, b, …) · sum(expr, n, from, to)", "add up; or a series", "Lists"},
	{"mean", "mean(a, b, …)", "average", "Lists"},
	{"median", "median(a, b, …)", "the middle value", "Lists"},
	{"sd", "sd(a, b, …)", "standard deviation (sample)", "Lists"},
	{"var", "var(a, b, …)", "variance (sample)", "Lists"},
	{"count", "count(list)", "how many", "Lists"},
	{"sort", "sort(list)", "smallest first", "Lists"},
	{"product", "product(a, b, …)", "multiply together", "Lists"},
	{"solve", "solve(x^2 = 9) · solve(eq, x)", "every solution of an equation", "Solving"},
	{"deriv", "deriv(expr, x, at)", "slope (derivative) at a point", "Solving"},
	{"integral", "integral(expr, x, a, b)", "area under a curve from a to b", "Solving"},
	{"pmt", "pmt(rate, periods, amount)", "payment per period of a loan", "Money"},
	{"fv", "fv(rate, periods, payment, start)", "what savings grow to", "Money"},
	{"pv", "pv(rate, periods, payment)", "what future payments are worth now", "Money"},
	{"compound", "compound(start, rate, years, times per year)", "start amount with compound interest", "Money"},
	{"days", "days(from, to)", "days between two dates", "Dates"},
	{"weekday", "weekday(date)", "day of the week", "Dates"},
	{"deg", "deg(x)", "radians to degrees", "Angles"},
	{"rad", "rad(x)", "degrees to radians", "Angles"},
}

var builtinSet = func() map[string]bool {
	m := map[string]bool{}
	for _, f := range Builtins {
		m[f.Name] = true
	}
	for _, a := range []string{"log10", "avg", "average", "stdev", "variance", "choose", "perm", "fact", "integrate", "derivative", "diff", "asinh", "acosh", "atanh", "atan2", "len", "if"} {
		m[a] = true
	}
	return m
}()

func builtinFunc(name string) bool { return builtinSet[name] }

func (e *Env) call(n *Node) (Value, error) {
	name := n.Name
	// numeric work that needs the expression itself
	switch name {
	case "solve":
		return e.solveCall(n)
	case "integral", "integrate":
		return e.integralCall(n)
	case "deriv", "derivative", "diff":
		return e.derivCall(n)
	case "sum", "product":
		if len(n.Kids) == 4 && n.Kids[1].Op == "id" {
			return e.seriesCall(n)
		}
	case "if":
		if len(n.Kids) != 3 {
			return Value{}, errf(n.Pos, "write if(condition, then, otherwise)")
		}
		c, err := e.Eval(n.Kids[0])
		if err != nil {
			return c, err
		}
		if c.N != 0 {
			return e.Eval(n.Kids[1])
		}
		return e.Eval(n.Kids[2])
	}
	args := make([]Value, 0, len(n.Kids))
	for _, k := range n.Kids {
		v, err := e.Eval(k)
		if err != nil {
			return v, err
		}
		args = append(args, v)
	}
	// a function set in the sheet: f(x) = …, and f'(x)
	if f, ok := e.fn(name); ok {
		if len(args) != len(f.Params) {
			return Value{}, errf(n.Pos, fmt.Sprintf("%s needs %d value(s)", name, len(f.Params)))
		}
		if n.Num > 0 {
			if len(args) != 1 || args[0].K != KNum {
				return Value{}, errf(n.Pos, name+"' works with one number")
			}
			return e.numDeriv(f.Body, f.Params[0], args[0].Float(), int(n.Num), n.Pos)
		}
		c := e.child()
		for i, p := range f.Params {
			c.Vars[p] = args[i]
		}
		return c.Eval(f.Body)
	}
	if n.Num > 0 {
		return Value{}, errf(n.Pos, "' works on a function you set, like f(x) = x^2")
	}
	// a variable followed by brackets is multiplication: a(b + c)
	if v, ok := e.lookup(name); ok && len(args) == 1 {
		return e.mul(v, args[0])
	}
	if !builtinFunc(name) {
		if e.Units {
			if _, err := e.unitDef(name); err == nil && len(args) == 1 {
				return e.mul(args[0], Value{N: 1, U: Units{{canonicalCurrency(name), 1}}})
			}
		}
		return Value{}, errf(n.Pos, "I don't know the function \""+name+"\"")
	}
	v, err := e.builtin(name, args)
	if err != nil {
		var pe *Error
		if !errors.As(err, &pe) {
			err = errf(n.Pos, err.Error())
		}
	}
	return v, err
}

func plain(name string, args []Value, n int) ([]float64, error) {
	if n >= 0 && len(args) != n {
		return nil, fmt.Errorf("%s needs %d value(s)", name, n)
	}
	out := make([]float64, len(args))
	for i, a := range args {
		if a.K != KNum {
			return nil, fmt.Errorf("%s needs numbers", name)
		}
		if len(a.U) > 0 {
			return nil, fmt.Errorf("%s needs plain numbers, without units", name)
		}
		out[i] = a.Float()
	}
	return out, nil
}

// flatten turns arguments (numbers or lists) into one list of values.
func flatten(args []Value) []Value {
	var out []Value
	for _, a := range args {
		if a.K == KList {
			out = append(out, a.List...)
		} else {
			out = append(out, a)
		}
	}
	return out
}

func (e *Env) angle(v Value) (float64, error) {
	if v.K != KNum {
		return 0, errors.New("an angle must be a number")
	}
	if len(v.U) > 0 {
		x, d, err := e.toSI(v)
		if err != nil {
			return 0, err
		}
		if !d.zero() {
			return 0, errors.New("an angle can't have the unit " + unitText(v.U, 2))
		}
		return x, nil
	}
	if e.Deg {
		return v.Float() * math.Pi / 180, nil
	}
	return v.Float(), nil
}

func (e *Env) angleOut(r float64) Value {
	if e.Deg {
		return Value{N: r * 180 / math.Pi, U: Units{{"deg", 1}}}
	}
	return num(r)
}

// cleanTrig snaps sin(180°) to 0 instead of 1.2e-16.
func cleanTrig(x float64) float64 {
	if math.Abs(x) < 1e-15 {
		return 0
	}
	return x
}

func (e *Env) builtin(name string, args []Value) (Value, error) {
	one := func() (float64, error) {
		x, err := plain(name, args, 1)
		if err != nil {
			return 0, err
		}
		return x[0], nil
	}
	keepUnits := func(f func(float64) float64) (Value, error) {
		if len(args) != 1 || args[0].K != KNum {
			return Value{}, fmt.Errorf("%s needs one number", name)
		}
		v := args[0]
		v.N = f(v.N)
		return v, nil
	}
	switch name {
	case "sqrt":
		if len(args) != 1 {
			return Value{}, errors.New("sqrt needs one number")
		}
		if len(args[0].U) == 0 && args[0].Float() < 0 {
			return Value{}, errors.New("a negative number has no real square root")
		}
		return e.pow(args[0], num(0.5))
	case "cbrt":
		if len(args) != 1 {
			return Value{}, errors.New("cbrt needs one number")
		}
		if len(args[0].U) == 0 {
			return num(math.Cbrt(args[0].Float())), nil
		}
		return e.pow(args[0], num(1.0/3))
	case "root":
		x, err := plain(name, args, 2)
		if err != nil {
			return Value{}, err
		}
		if x[0] < 0 && math.Mod(x[1], 2) == 1 {
			return num(-math.Pow(-x[0], 1/x[1])), nil
		}
		return num(math.Pow(x[0], 1/x[1])), nil
	case "exp":
		x, err := one()
		return num(math.Exp(x)), err
	case "ln":
		x, err := one()
		if err == nil && x <= 0 {
			err = errors.New("ln needs a number above 0")
		}
		return num(math.Log(x)), err
	case "log", "log10":
		if len(args) == 2 {
			x, err := plain(name, args, 2)
			if err != nil {
				return Value{}, err
			}
			return num(math.Log(x[0]) / math.Log(x[1])), nil
		}
		x, err := one()
		if err == nil && x <= 0 {
			err = errors.New("log needs a number above 0")
		}
		return num(math.Log10(x)), err
	case "log2":
		x, err := one()
		return num(math.Log2(x)), err
	case "abs":
		return keepUnits(math.Abs)
	case "floor":
		return keepUnits(math.Floor)
	case "ceil":
		return keepUnits(math.Ceil)
	case "trunc":
		return keepUnits(math.Trunc)
	case "sign":
		x, err := one()
		if x > 0 {
			return num(1), err
		} else if x < 0 {
			return num(-1), err
		}
		return num(0), err
	case "round":
		if len(args) == 2 {
			d, err := plain(name, args[1:], 1)
			if err != nil {
				return Value{}, err
			}
			p := math.Pow(10, math.Round(d[0]))
			v := args[0]
			v.N = math.Round(v.N*p) / p
			return v, nil
		}
		return keepUnits(math.Round)
	case "mod":
		if len(args) != 2 {
			return Value{}, errors.New("mod needs two numbers")
		}
		return e.binary(&Node{Op: "mod"}, args[0], args[1])
	case "gcd", "lcm":
		x, err := plain(name, flatten(args), -1)
		if err != nil || len(x) == 0 {
			return Value{}, fmt.Errorf("%s needs whole numbers", name)
		}
		acc := math.Abs(math.Round(x[0]))
		for _, y := range x[1:] {
			y = math.Abs(math.Round(y))
			if name == "gcd" {
				acc = gcd(acc, y)
			} else if acc != 0 && y != 0 {
				acc = acc / gcd(acc, y) * y
			} else {
				acc = 0
			}
		}
		return num(acc), nil
	case "factorial", "fact":
		x, err := one()
		if err != nil || x < 0 || x != math.Floor(x) {
			return Value{}, errors.New("factorial needs a whole number of 0 or more")
		}
		return num(math.Round(math.Gamma(x + 1))), nil
	case "nCr", "choose", "nPr", "perm":
		x, err := plain(name, args, 2)
		if err != nil {
			return Value{}, err
		}
		n, k := x[0], x[1]
		if n < 0 || k < 0 || k > n || n != math.Floor(n) || k != math.Floor(k) {
			return num(0), nil
		}
		r := 1.0
		for i := 0.0; i < k; i++ {
			r *= (n - i)
			if name == "nCr" || name == "choose" {
				r /= (i + 1)
			}
		}
		return num(math.Round(r)), nil
	case "hypot":
		if len(args) != 2 {
			return Value{}, errors.New("hypot needs two numbers")
		}
		a2, err := e.mul(args[0], args[0])
		if err != nil {
			return a2, err
		}
		b2, err := e.mul(args[1], args[1])
		if err != nil {
			return b2, err
		}
		s, err := e.add(a2, b2, 1)
		if err != nil {
			return s, err
		}
		return e.pow(s, num(0.5))
	case "sin", "cos", "tan", "sec", "csc", "cot":
		if len(args) != 1 {
			return Value{}, fmt.Errorf("%s needs one angle", name)
		}
		a, err := e.angle(args[0])
		if err != nil {
			return Value{}, err
		}
		s, c := math.Sincos(a)
		s, c = cleanTrig(s), cleanTrig(c)
		switch name {
		case "sin":
			return num(s), nil
		case "cos":
			return num(c), nil
		case "tan":
			if c == 0 {
				return Value{}, errors.New("tan is endless here")
			}
			return num(s / c), nil
		case "sec":
			return num(1 / c), nil
		case "csc":
			return num(1 / s), nil
		default:
			return num(c / s), nil
		}
	case "asin", "acos":
		x, err := one()
		if err == nil && (x < -1 || x > 1) {
			err = errors.New(name + " needs a number from −1 to 1")
		}
		if name == "asin" {
			return e.angleOut(math.Asin(x)), err
		}
		return e.angleOut(math.Acos(x)), err
	case "atan", "atan2":
		if len(args) == 2 {
			x, err := plain(name, args, 2)
			if err != nil {
				return Value{}, err
			}
			return e.angleOut(math.Atan2(x[0], x[1])), nil
		}
		x, err := one()
		return e.angleOut(math.Atan(x)), err
	case "sinh", "cosh", "tanh", "asinh", "acosh", "atanh":
		x, err := one()
		f := map[string]func(float64) float64{"sinh": math.Sinh, "cosh": math.Cosh, "tanh": math.Tanh, "asinh": math.Asinh, "acosh": math.Acosh, "atanh": math.Atanh}[name]
		return num(f(x)), err
	case "deg":
		x, err := one()
		return Value{N: x * 180 / math.Pi, U: Units{{"deg", 1}}}, err
	case "rad":
		if len(args) == 1 && len(args[0].U) > 0 {
			a, err := e.angle(args[0])
			return num(a), err
		}
		x, err := one()
		return num(x * math.Pi / 180), err
	case "min", "max":
		items := flatten(args)
		if len(items) == 0 {
			return Value{}, fmt.Errorf("%s needs at least one number", name)
		}
		best := items[0]
		for _, v := range items[1:] {
			c, err := e.compare("lt", v, best)
			if err != nil {
				return Value{}, err
			}
			if (c.N == 1) == (name == "min") {
				best = v
			}
		}
		return best, nil
	case "sum", "product":
		items := flatten(args)
		if len(items) == 0 {
			return num(map[string]float64{"sum": 0, "product": 1}[name]), nil
		}
		acc := items[0]
		for _, v := range items[1:] {
			var err error
			if name == "sum" {
				acc, err = e.add(acc, v, 1)
			} else {
				acc, err = e.mul(acc, v)
			}
			if err != nil {
				return acc, err
			}
		}
		return acc, nil
	case "mean", "avg", "average":
		items := flatten(args)
		if len(items) == 0 {
			return Value{}, errors.New("mean needs numbers")
		}
		s, err := e.builtin("sum", items)
		if err != nil {
			return s, err
		}
		s.N /= float64(len(items))
		return s, nil
	case "median", "sd", "stdev", "var", "variance":
		items := flatten(args)
		if len(items) == 0 {
			return Value{}, fmt.Errorf("%s needs numbers", name)
		}
		base := items[0]
		xs := make([]float64, len(items))
		for i, v := range items {
			x, err := e.sameAs(v, base)
			if err != nil {
				return Value{}, err
			}
			xs[i] = x
		}
		out := Value{U: base.U.clone()}
		switch name {
		case "median":
			sort.Float64s(xs)
			m := len(xs) / 2
			out.N = xs[m]
			if len(xs)%2 == 0 {
				out.N = (xs[m-1] + xs[m]) / 2
			}
		default:
			if len(xs) < 2 {
				return Value{}, errors.New("needs at least two numbers")
			}
			mean := 0.0
			for _, x := range xs {
				mean += x
			}
			mean /= float64(len(xs))
			ss := 0.0
			for _, x := range xs {
				ss += (x - mean) * (x - mean)
			}
			v := ss / float64(len(xs)-1)
			if name == "var" || name == "variance" {
				for i := range out.U {
					out.U[i].Pow *= 2
				}
				out.N = v
			} else {
				out.N = math.Sqrt(v)
			}
		}
		return out, nil
	case "count", "len":
		return num(float64(len(flatten(args)))), nil
	case "sort":
		items := flatten(args)
		sort.SliceStable(items, func(i, j int) bool {
			c, _ := e.compare("lt", items[i], items[j])
			return c.N == 1
		})
		return Value{K: KList, List: items}, nil
	case "pmt", "fv", "pv":
		if len(args) < 3 || len(args) > 4 {
			return Value{}, fmt.Errorf("write %s(rate, periods, amount)", name)
		}
		rate := args[0]
		if rate.K != KNum || len(rate.U) > 0 {
			return Value{}, errors.New("the rate is a percentage, like 5%/12")
		}
		r := rate.Float()
		periods, err := plain(name, args[1:2], 1)
		if err != nil {
			return Value{}, err
		}
		n := periods[0]
		money := args[2]
		extra := 0.0
		if len(args) == 4 {
			x, err := e.sameAs(args[3], money)
			if err != nil {
				return Value{}, err
			}
			extra = x
		}
		g := math.Pow(1+r, n)
		out := Value{U: money.U.clone()}
		switch name {
		case "pmt": // amount borrowed → payment
			if r == 0 {
				out.N = (money.N + extra) / n
			} else {
				out.N = (money.N*g + extra) * r / (g - 1)
			}
		case "fv": // payment each period (+ start) → value at the end
			if r == 0 {
				out.N = money.N*n + extra
			} else {
				out.N = money.N*(g-1)/r + extra*g
			}
		case "pv":
			if r == 0 {
				out.N = money.N*n + extra
			} else {
				out.N = money.N*(1-1/g)/r + extra/g
			}
		}
		return out, nil
	case "compound":
		if len(args) < 3 || len(args) > 4 {
			return Value{}, errors.New("write compound(start, rate, years) or compound(start, rate, years, times a year)")
		}
		start := args[0]
		r := args[1].Float()
		yrs := args[2]
		years := yrs.Float()
		if len(yrs.U) > 0 {
			y, err := e.convert(yrs, Units{{"year", 1}})
			if err != nil {
				return Value{}, err
			}
			years = y
		}
		k := 1.0
		if len(args) == 4 {
			k = args[3].Float()
		}
		start.N *= math.Pow(1+r/k, k*years)
		return start, nil
	case "days":
		if len(args) != 2 || args[0].K != KDate || args[1].K != KDate {
			return Value{}, errors.New("write days(from, to) with two dates")
		}
		return Value{N: args[1].N - args[0].N, U: Units{{"day", 1}}}, nil
	case "weekday":
		if len(args) != 1 || args[0].K != KDate {
			return Value{}, errors.New("weekday needs a date")
		}
		return Value{K: KDate, N: args[0].N, Fmt: "weekday"}, nil
	}
	return Value{}, fmt.Errorf("I don't know the function \"%s\"", name)
}

func gcd(a, b float64) float64 {
	for b != 0 {
		a, b = b, math.Mod(a, b)
	}
	return a
}

// ---- calculus and solving (numbers, not symbols) ----

func (e *Env) varArg(n *Node, i int) (string, bool) {
	if i < len(n.Kids) && n.Kids[i].Op == "id" {
		return n.Kids[i].Name, true
	}
	return "", false
}

func (e *Env) solveCall(n *Node) (Value, error) {
	if len(n.Kids) == 0 || len(n.Kids) > 2 {
		return Value{}, errf(n.Pos, "write solve(x^2 = 9) or solve(equation, x)")
	}
	eq := n.Kids[0]
	l, r := eq, &Node{Op: "num"}
	if eq.Op == "eq" {
		l, r = eq.Kids[0], eq.Kids[1]
	}
	name, ok := e.varArg(n, 1)
	if !ok {
		unk := map[string]bool{}
		e.unknowns(eq, unk)
		if len(unk) != 1 {
			names := []string{}
			for k := range unk {
				names = append(names, k)
			}
			sort.Strings(names)
			if len(names) == 0 {
				return Value{}, errf(n.Pos, "there is nothing unknown to solve for")
			}
			return Value{}, errf(n.Pos, "say which to solve for: solve(…, "+names[0]+")")
		}
		for k := range unk {
			name = k
		}
	}
	return e.solve(l, r, name, n.Pos)
}

func (e *Env) solve(l, r *Node, name string, pos int) (Value, error) {
	diff := &Node{Op: "-", Kids: []*Node{l, r}}
	f := e.fnOf(diff, name)
	roots := FindRoots(func(x float64) float64 {
		y, err := f(x)
		if err != nil {
			return math.NaN()
		}
		return y
	}, -1e6, 1e6)
	out := Value{K: KSolved, Name: name}
	for _, x := range roots {
		out.List = append(out.List, num(x))
	}
	if len(out.List) == 0 {
		return out, errf(pos, "I found no solution for "+name+" (between −1,000,000 and 1,000,000)")
	}
	return out, nil
}

// FindRoots finds where f crosses (or touches) zero between lo and hi.
func FindRoots(f func(float64) float64, lo, hi float64) []float64 {
	// sample densely near zero and more sparsely further out
	var xs []float64
	for x := -100.0; x <= 100.0; x += 0.05 {
		xs = append(xs, math.Round(x*1e6)/1e6)
	}
	for p := 2.0; p <= math.Log10(hi); p += 0.02 {
		v := math.Pow(10, p)
		xs = append(xs, v, -v)
	}
	sort.Float64s(xs)
	var roots []float64
	add := func(x float64) {
		x = tidyRoot(x)
		for _, r := range roots {
			if math.Abs(r-x) <= 1e-7*math.Max(1, math.Abs(x)) {
				return
			}
		}
		roots = append(roots, x)
	}
	prevX, prevY := math.NaN(), math.NaN()
	for i, x := range xs {
		y := f(x)
		if y == 0 {
			add(x)
		} else if !math.IsNaN(y) && !math.IsNaN(prevY) && (y > 0) != (prevY > 0) && prevY != 0 {
			a, b, fa := prevX, x, prevY
			for k := 0; k < 200; k++ {
				m := (a + b) / 2
				fm := f(m)
				if math.IsNaN(fm) {
					break
				}
				if (fm > 0) == (fa > 0) {
					a, fa = m, fm
				} else {
					b = m
				}
				if b-a < 1e-14*math.Max(1, math.Abs(m)) {
					break
				}
			}
			m := (a + b) / 2
			// a jump across a pole (1/x) is not a root
			if fm := f(m); !math.IsNaN(fm) && math.Abs(fm) < 1e-6*math.Max(1, math.Max(math.Abs(prevY), math.Abs(y))) {
				add(m)
			}
		} else if i > 0 && i+1 < len(xs) && !math.IsNaN(y) && !math.IsNaN(prevY) {
			// touching zero without crossing: a small local minimum of |f|
			ny := f(xs[i+1])
			if math.Abs(y) < math.Abs(prevY) && math.Abs(y) < math.Abs(ny) && (y > 0) == (ny > 0) && (y > 0) == (prevY > 0) {
				g := func(t float64) float64 { return math.Abs(f(t)) }
				a, b := prevX, xs[i+1]
				for k := 0; k < 200; k++ {
					m1, m2 := a+(b-a)/3, b-(b-a)/3
					if g(m1) < g(m2) {
						b = m2
					} else {
						a = m1
					}
				}
				m := (a + b) / 2
				if g(m) < 1e-9 {
					add(m)
				}
			}
		}
		prevX, prevY = x, y
	}
	sort.Float64s(roots)
	if len(roots) > 20 {
		roots = roots[:20]
	}
	return roots
}

func tidyRoot(x float64) float64 {
	if r := math.Round(x); math.Abs(x-r) < 1e-9*math.Max(1, math.Abs(x)) {
		return r + 0 // no -0
	}
	r, _ := fmtSig(x, 12)
	return r
}

func fmtSig(x float64, sig int) (float64, string) {
	s := fmt.Sprintf("%.*g", sig, x)
	var f float64
	fmt.Sscanf(s, "%g", &f)
	return f, s
}

func (e *Env) numDeriv(body *Node, name string, at float64, order int, pos int) (Value, error) {
	f := e.fnOf(body, name)
	g := func(x float64) float64 {
		y, err := f(x)
		if err != nil {
			return math.NaN()
		}
		return y
	}
	var d func(x float64, k int) float64
	d = func(x float64, k int) float64 {
		h := 1e-4 * math.Max(1, math.Abs(x))
		if k == 1 {
			// five-point stencil
			return (-g(x+2*h) + 8*g(x+h) - 8*g(x-h) + g(x-2*h)) / (12 * h)
		}
		h = 1e-3 * math.Max(1, math.Abs(x))
		return (d(x+h, k-1) - d(x-h, k-1)) / (2 * h)
	}
	v := d(at, order)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Value{}, errf(pos, "there is no slope there")
	}
	r, _ := fmtSig(v, 9)
	return num(tidyRoot(r)), nil
}

func (e *Env) derivCall(n *Node) (Value, error) {
	// deriv(expr, at) or deriv(expr, x, at)
	switch len(n.Kids) {
	case 2:
		at, err := e.Eval(n.Kids[1])
		if err != nil {
			return at, err
		}
		return e.numDeriv(n.Kids[0], "x", at.Float(), 1, n.Pos)
	case 3:
		name, ok := e.varArg(n, 1)
		if !ok {
			return Value{}, errf(n.Pos, "write deriv(expr, x, at)")
		}
		at, err := e.Eval(n.Kids[2])
		if err != nil {
			return at, err
		}
		return e.numDeriv(n.Kids[0], name, at.Float(), 1, n.Pos)
	}
	return Value{}, errf(n.Pos, "write deriv(x^2, x, 3)")
}

func (e *Env) integralCall(n *Node) (Value, error) {
	name := "x"
	var a, b *Node
	switch len(n.Kids) {
	case 3:
		a, b = n.Kids[1], n.Kids[2]
	case 4:
		v, ok := e.varArg(n, 1)
		if !ok {
			return Value{}, errf(n.Pos, "write integral(expr, x, from, to)")
		}
		name, a, b = v, n.Kids[2], n.Kids[3]
	default:
		return Value{}, errf(n.Pos, "write integral(x^2, x, 0, 3)")
	}
	av, err := e.Eval(a)
	if err != nil {
		return av, err
	}
	bv, err := e.Eval(b)
	if err != nil {
		return bv, err
	}
	f := e.fnOf(n.Kids[0], name)
	g := func(x float64) float64 {
		y, err := f(x)
		if err != nil {
			return math.NaN()
		}
		return y
	}
	v := Integrate(g, av.Float(), bv.Float())
	if math.IsNaN(v) {
		return Value{}, errf(n.Pos, "this area can't be worked out (the curve has a gap)")
	}
	r, _ := fmtSig(v, 10)
	return num(tidyRoot(r)), nil
}

// Integrate uses adaptive Simpson's rule.
func Integrate(f func(float64) float64, a, b float64) float64 {
	if a == b {
		return 0
	}
	var rec func(a, b, fa, fm, fb, whole, eps float64, depth int) float64
	rec = func(a, b, fa, fm, fb, whole, eps float64, depth int) float64 {
		m := (a + b) / 2
		lm, rm := (a+m)/2, (m+b)/2
		flm, frm := f(lm), f(rm)
		left := (m - a) / 6 * (fa + 4*flm + fm)
		right := (b - m) / 6 * (fm + 4*frm + fb)
		if depth <= 0 || math.Abs(left+right-whole) <= 15*eps {
			return left + right + (left+right-whole)/15
		}
		return rec(a, m, fa, flm, fm, left, eps/2, depth-1) + rec(m, b, fm, frm, fb, right, eps/2, depth-1)
	}
	fa, fb, fm := f(a), f(b), f((a+b)/2)
	whole := (b - a) / 6 * (fa + 4*fm + fb)
	return rec(a, b, fa, fm, fb, whole, 1e-10, 40)
}

func (e *Env) seriesCall(n *Node) (Value, error) {
	name := n.Kids[1].Name
	from, err := e.Eval(n.Kids[2])
	if err != nil {
		return from, err
	}
	to, err := e.Eval(n.Kids[3])
	if err != nil {
		return to, err
	}
	a, b := math.Round(from.Float()), math.Round(to.Float())
	if b-a > 1e6 {
		return Value{}, errf(n.Pos, "that is too many steps (at most a million)")
	}
	c := e.child()
	var acc *Value
	for k := a; k <= b; k++ {
		c.Vars[name] = num(k)
		v, err := c.Eval(n.Kids[0])
		if err != nil {
			return v, err
		}
		if acc == nil {
			acc = &v
			continue
		}
		var r Value
		if n.Name == "sum" {
			r, err = e.add(*acc, v, 1)
		} else {
			r, err = e.mul(*acc, v)
		}
		if err != nil {
			return r, err
		}
		acc = &r
	}
	if acc == nil {
		return num(map[string]float64{"sum": 0, "product": 1}[n.Name]), nil
	}
	return *acc, nil
}

var _ = strings.TrimSpace
