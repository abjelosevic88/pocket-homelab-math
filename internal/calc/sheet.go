package calc

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
)

// Options for working out a sheet.
type Options struct {
	Units  bool
	Deg    bool
	Rates  map[string]float64
	Today  time.Time
	Format Format
}

// Line is the answer to one line of a sheet.
type Line struct {
	Kind     string   `json:"kind"` // value | set | func | empty | text | error
	Label    string   `json:"label,omitempty"`
	Name     string   `json:"name,omitempty"`
	Text     string   `json:"text,omitempty"`
	Alt      string   `json:"alt,omitempty"`
	Error    string   `json:"error,omitempty"`
	ErrorPos int      `json:"error_pos,omitempty"`
	Number   *float64 `json:"number,omitempty"` // the amount in the units shown
	Unit     string   `json:"unit,omitempty"`
}

// New makes an environment from options.
func (o Options) New() *Env {
	e := NewEnv(o.Units, o.Deg, o.Rates)
	if !o.Today.IsZero() {
		e.Today = o.Today
	}
	return e
}

// EvalSheet works out every line in order; variables and functions set on
// a line are known on the lines after it.
func EvalSheet(lines []string, o Options) ([]Line, *Env) {
	e := o.New()
	out := make([]Line, 0, len(lines))
	for _, l := range lines {
		out = append(out, e.RunLine(l, o.Format))
	}
	return out, e
}

// RunLine works out one line in this environment.
func (e *Env) RunLine(line string, f Format) Line {
	trim := strings.TrimSpace(line)
	if trim == "" {
		e.above = nil
		return Line{Kind: "empty"}
	}
	if strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "//") {
		return Line{Kind: "text"}
	}
	st, err := ParseLine(line, e)
	if err != nil {
		return e.lineErr(trim, err, "")
	}
	switch st.Kind {
	case "empty":
		return Line{Kind: "text", Label: st.Label}
	case "func":
		e.Funcs[st.Name] = &Func{Params: st.Params, Body: st.Expr}
		delete(e.Vars, st.Name)
		return Line{Kind: "func", Label: st.Label, Name: st.Name}
	}
	v, err := e.Eval(st.Expr)
	if err != nil {
		return e.lineErr(trim, err, st.Label)
	}
	ln := e.lineOf(v, f)
	ln.Label = st.Label
	if st.Kind == "assign" {
		e.Vars[st.Name] = v
		delete(e.Funcs, st.Name)
		ln.Kind, ln.Name = "set", st.Name
	}
	if v.K == KNum || v.K == KDate {
		c := v
		e.prev = &c
		if v.K == KNum {
			e.above = append(e.above, v)
		}
	}
	return ln
}

func (e *Env) lineOf(v Value, f Format) Line {
	text, alt := e.Show(v, f)
	ln := Line{Kind: "value", Text: text, Alt: alt}
	if v.K == KNum && !math.IsNaN(v.N) && !math.IsInf(v.N, 0) {
		n := v.N
		if v.Label != "" && v.LScale != 0 {
			n /= v.LScale
		}
		ln.Number = &n
		ln.Unit = firstNonEmpty(v.Label, unitText(v.U, 2))
		if v.Pct {
			ln.Unit = "%"
		}
	}
	return ln
}

// lineErr: a line of words only ("Shopping list") is text, not a mistake.
func (e *Env) lineErr(trim string, err error, label string) Line {
	hasDigit := strings.IndexFunc(trim, unicode.IsDigit) >= 0
	hasOp := strings.ContainsAny(trim, "+*/^=()")
	if !hasDigit && !hasOp {
		return Line{Kind: "text", Label: label}
	}
	ln := Line{Kind: "error", Label: label, Error: err.Error()}
	var pe *Error
	if errors.As(err, &pe) {
		ln.ErrorPos = pe.Pos
	}
	return ln
}

// Input is a value given to a saved calculation ("4.2 m", "1200 EUR").
type Input struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Unit    string `json:"unit"`    // the unit the box is in (shown next to it)
	Default string `json:"default"` // what is in the box at first
	Help    string `json:"help"`
	Kind    string `json:"kind"`              // number (default) | percent | date | choice
	Options string `json:"options,omitempty"` // choice: "Male=5|Female=-161"
	From    string `json:"from,omitempty"`    // can be filled in from another app: "rooms:wall", "health:weight"…
}

// choiceValue maps a picked option to its value.
func choiceValue(in Input, raw string) string {
	for _, o := range strings.Split(in.Options, "|") {
		label, val, ok := strings.Cut(o, "=")
		if !ok {
			label, val = o, o
		}
		if strings.EqualFold(strings.TrimSpace(label), raw) || strings.TrimSpace(val) == raw {
			return strings.TrimSpace(val)
		}
	}
	return raw
}

// Output is one answer of a saved calculation.
type Output struct {
	Label    string `json:"label"`
	Expr     string `json:"expr"`
	Unit     string `json:"unit"`     // shown in this unit ("" = as worked out)
	Decimals *int   `json:"decimals"` // round to this many decimals
	Main     bool   `json:"main"`     // the big answer
}

// RunResult is a saved calculation worked out.
type RunResult struct {
	Outputs []OutputResult `json:"outputs"`
	Lines   []Line         `json:"lines"`
	Error   string         `json:"error,omitempty"`
}

// OutputResult is one answer.
type OutputResult struct {
	Label  string   `json:"label"`
	Text   string   `json:"text"`
	Number *float64 `json:"number,omitempty"`
	Unit   string   `json:"unit,omitempty"`
	Error  string   `json:"error,omitempty"`
	Main   bool     `json:"main,omitempty"`
}

// Run works out a saved calculation: inputs first, then its working, then
// each answer.
func Run(code string, inputs []Input, values map[string]string, outputs []Output, o Options) RunResult {
	e := o.New()
	var res RunResult
	for _, in := range inputs {
		raw := strings.TrimSpace(values[in.Name])
		if raw == "" {
			raw = in.Default
		}
		if raw == "" {
			res.Error = "fill in " + firstNonEmpty(in.Label, in.Name)
			return res
		}
		src := raw
		if in.Kind == "choice" {
			src = choiceValue(in, raw)
		} else if in.Kind == "percent" && !strings.HasSuffix(src, "%") {
			src += "%"
		} else if in.Unit != "" && isBareNumber(raw) {
			src = raw + " " + in.Unit
		}
		n, err := ParseExpr(src, e)
		if err != nil {
			res.Error = firstNonEmpty(in.Label, in.Name) + ": " + err.Error()
			return res
		}
		v, err := e.Eval(n)
		if err != nil {
			res.Error = firstNonEmpty(in.Label, in.Name) + ": " + err.Error()
			return res
		}
		e.Vars[in.Name] = v
	}
	for _, l := range strings.Split(code, "\n") {
		ln := e.RunLine(l, o.Format)
		res.Lines = append(res.Lines, ln)
	}
	for _, out := range outputs {
		r := OutputResult{Label: out.Label, Main: out.Main}
		src := out.Expr
		if strings.TrimSpace(out.Unit) != "" {
			src = "(" + src + ") to " + out.Unit
		}
		n, err := ParseExpr(src, e)
		var v Value
		if err == nil {
			v, err = e.Eval(n)
		}
		if err != nil {
			r.Error = err.Error()
			res.Outputs = append(res.Outputs, r)
			continue
		}
		if out.Decimals != nil && v.K == KNum {
			p := math.Pow(10, float64(*out.Decimals))
			sc := 1.0
			if v.Label != "" && v.LScale != 0 {
				sc = v.LScale
			}
			if v.Pct {
				v.N = math.Round(v.N*p) / p
			} else {
				v.N = math.Round(v.N/sc*p) / p * sc
			}
		}
		ln := e.lineOf(v, o.Format)
		r.Text, r.Number, r.Unit = ln.Text, ln.Number, ln.Unit
		res.Outputs = append(res.Outputs, r)
	}
	return res
}

func isBareNumber(s string) bool {
	s = strings.ReplaceAll(strings.TrimSpace(s), "_", "")
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c >= '0' && c <= '9' || c == '.' || (c == '-' || c == '+') && i == 0 || c == 'e' || c == 'E') {
			return false
		}
	}
	return true
}

// Calculate works out one expression on its own (for the API, other apps
// and Pocket AI).
func Calculate(expr string, o Options) Line {
	e := o.New()
	var last Line
	for _, l := range strings.Split(expr, "\n") {
		ln := e.RunLine(l, o.Format)
		if ln.Kind != "empty" && ln.Kind != "text" {
			last = ln
		}
	}
	if last.Kind == "" {
		last = Line{Kind: "error", Error: "there is nothing to work out"}
	}
	return last
}
