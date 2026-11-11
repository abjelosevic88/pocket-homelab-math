package calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Format is how numbers are written.
type Format struct {
	Digits int    // significant digits, 10 when 0
	Group  string // thousands separator: "," "." " " or ""
	Point  string // decimal mark: "." or ","
}

func (f Format) norm() Format {
	if f.Digits <= 0 || f.Digits > 17 {
		f.Digits = 10
	}
	if f.Point == "" {
		f.Point = "."
	}
	if f.Group == f.Point {
		f.Group = ""
	}
	return f
}

// FormatNumber writes a plain number.
func FormatNumber(x float64, f Format) string {
	f = f.norm()
	switch {
	case math.IsNaN(x):
		return "not a number"
	case math.IsInf(x, 1):
		return "∞"
	case math.IsInf(x, -1):
		return "-∞"
	case x == 0:
		return "0"
	}
	ax := math.Abs(x)
	if ax >= 1e15 || ax < 1e-6 {
		return sci(x, f)
	}
	r, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'g', f.Digits, 64), 64)
	return group(strconv.FormatFloat(r, 'f', -1, 64), f)
}

func group(s string, f Format) string {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	ip, fp, _ := strings.Cut(s, ".")
	if f.Group != "" && len(ip) > 4 || f.Group != "" && len(ip) == 4 && f.Group != " " {
		var b strings.Builder
		for i, c := range ip {
			if i > 0 && (len(ip)-i)%3 == 0 {
				b.WriteString(f.Group)
			}
			b.WriteRune(c)
		}
		ip = b.String()
	}
	out := ip
	if fp != "" {
		out += f.Point + fp
	}
	if neg {
		out = "-" + out
	}
	return out
}

func sci(x float64, f Format) string {
	s := strconv.FormatFloat(x, 'e', f.Digits-1, 64)
	m, e, _ := strings.Cut(s, "e")
	if strings.Contains(m, ".") {
		m = strings.TrimRight(strings.TrimRight(m, "0"), ".")
	}
	ex, _ := strconv.Atoi(e)
	return strings.Replace(m, ".", f.Point, 1) + " × 10" + superscript(ex)
}

// Fraction finds p/q close to x with q ≤ maxQ.
func Fraction(x float64, maxQ int) (int64, int64, bool) {
	if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e9 {
		return 0, 0, false
	}
	sign := int64(1)
	if x < 0 {
		sign, x = -1, -x
	}
	// continued fractions
	h0, h1 := int64(0), int64(1)
	k0, k1 := int64(1), int64(0)
	y := x
	for i := 0; i < 40; i++ {
		a := int64(math.Floor(y))
		h0, h1 = h1, a*h1+h0
		k0, k1 = k1, a*k1+k0
		if k1 > int64(maxQ) {
			return 0, 0, false
		}
		if math.Abs(x-float64(h1)/float64(k1)) <= 1e-10*math.Max(1, x) {
			return sign * h1, k1, true
		}
		frac := y - float64(a)
		if frac < 1e-15 {
			break
		}
		y = 1 / frac
	}
	return 0, 0, false
}

var weekdays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// FormatDate writes a date the way people read it: Fri 25 Dec 2026.
func FormatDate(days float64) string {
	t := daysDate(days)
	return t.Format("Mon 2 Jan 2006")
}

// Show writes a value; alt is a second way of reading it (a fraction, the
// date as numbers) or "".
func (e *Env) Show(v Value, f Format) (text, alt string) {
	switch v.K {
	case KBool:
		if v.N != 0 {
			return "true", ""
		}
		return "false", ""
	case KDate:
		t := daysDate(v.N)
		if v.Fmt == "weekday" {
			return weekdays[t.Weekday()], t.Format("2006-01-02")
		}
		return FormatDate(v.N), t.Format(time.DateOnly)
	case KList:
		parts := make([]string, len(v.List))
		for i, x := range v.List {
			parts[i], _ = e.Show(x, f)
		}
		return "[" + strings.Join(parts, ", ") + "]", ""
	case KPoint:
		parts := make([]string, len(v.List))
		for i, x := range v.List {
			parts[i], _ = e.Show(x, f)
		}
		return "(" + strings.Join(parts, ", ") + ")", ""
	case KSolved:
		parts := make([]string, len(v.List))
		var alts []string
		for i, x := range v.List {
			t, a := e.Show(x, f)
			parts[i] = v.Name + " = " + t
			if a != "" {
				alts = append(alts, v.Name+" "+a)
			}
		}
		return strings.Join(parts, " or "), strings.Join(alts, ", ")
	}
	if v.Pct {
		n := v.N
		if math.Abs(n) >= 0.01 && math.Abs(n) < 1e12 {
			n = math.Round(n*100) / 100
		}
		return FormatNumber(n, f) + "%", ""
	}
	if v.Label != "" && v.LScale != 0 {
		v.N /= v.LScale
	}
	switch v.Fmt {
	case "hex", "bin", "oct":
		base := map[string]int{"hex": 16, "bin": 2, "oct": 8}[v.Fmt]
		pre := map[string]string{"hex": "0x", "bin": "0b", "oct": "0o"}[v.Fmt]
		n := int64(v.N)
		s := strings.ToUpper(strconv.FormatInt(abs64(n), base))
		if n < 0 {
			return "-" + pre + s, ""
		}
		return pre + s, ""
	case "sci":
		return withUnit(sci(v.N, f.norm()), v), ""
	case "frac":
		if p, q, ok := Fraction(v.N, 100000); ok {
			if q == 1 {
				return withUnit(fmt.Sprint(p), v), ""
			}
			return withUnit(fmt.Sprintf("%d/%d", p, q), v), ""
		}
		return withUnit("≈ "+FormatNumber(v.N, f), v), ""
	}
	// money: two decimals
	if len(v.U) == 1 && v.U[0].Pow == 1 && v.Label == "" {
		if d, err := e.unitDef(v.U[0].Name); err == nil && d.Cur != "" && math.Abs(v.N) >= 0.01 && math.Abs(v.N) < 1e15 {
			s := group(strconv.FormatFloat(v.N, 'f', 2, 64), f.norm())
			return s + " " + d.Cur, ""
		}
	}
	text = withUnit(FormatNumber(v.N, f), v)
	// a fraction beside a decimal when it says more: 1/3 for 0.3333333333,
	// 3/4 for 0.75 — not 1269/50 for 25.38
	if len(v.U) == 0 && v.N != math.Trunc(v.N) {
		decimals := 0
		if _, fp, ok := strings.Cut(text, f.norm().Point); ok {
			decimals = len(fp)
		}
		if p, q, ok := Fraction(v.N, 1000); ok && q > 1 && (q <= 12 || decimals >= 6) {
			alt = fmt.Sprintf("= %d/%d", p, q)
		}
	}
	return text, alt
}

func withUnit(s string, v Value) string {
	u := v.Label
	if u == "" {
		u = unitText(v.U, v.N)
	}
	if u == "" {
		return s
	}
	if u == "°" {
		return s + u
	}
	return s + " " + u
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
