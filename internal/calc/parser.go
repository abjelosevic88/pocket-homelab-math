package calc

import (
	"regexp"
	"strings"
)

// Error is a mistake in a line, with where it is.
type Error struct {
	Pos int
	Msg string
}

func (e *Error) Error() string { return e.Msg }

func errf(pos int, msg string) error { return &Error{Pos: pos, Msg: msg} }

// Node is one piece of a parsed expression.
type Node struct {
	Op    string  `json:"op"`
	Num   float64 `json:"num,omitempty"`
	Name  string  `json:"name,omitempty"`
	Units Units   `json:"units,omitempty"`
	Scale float64 `json:"scale,omitempty"` // qty written per a number of a unit: L/100km
	Kids  []*Node `json:"kids,omitempty"`
	Pos   int     `json:"-"`
}

// Stmt is one line: an expression, a variable or a function being set.
type Stmt struct {
	Kind   string   // expr | assign | func | empty
	Label  string   // "Rent:" in front of a line
	Name   string   // what is being set
	Params []string // a function's parameters
	Expr   *Node
}

// keywords that never start an operand.
var keywords = map[string]bool{"to": true, "as": true, "of": true, "off": true, "on": true, "per": true, "mod": true, "into": true}

// specialTargets are what a result can be shown as besides a unit.
var specialTargets = map[string]string{
	"hex": "hex", "hexadecimal": "hex", "bin": "bin", "binary": "bin", "oct": "oct", "octal": "oct",
	"sci": "sci", "scientific": "sci", "fraction": "frac", "frac": "frac", "fractions": "frac",
	"percent": "pct", "percentage": "pct", "dec": "dec", "decimal": "dec", "number": "dec", "base": "base",
}

var labelRe = regexp.MustCompile(`^\s*([\p{L}][\p{L}\p{N} _'’&/(),.-]*?)\s*:\s*(.*)$`)

type parser struct {
	toks  []token
	i     int
	units bool
	known func(string) bool // names set as variables or functions
	rates func(string) bool // currency codes we can convert
}

// SplitLabel returns the label written before a colon ("Rent: 500 EUR").
func SplitLabel(line string) (label, rest string) {
	if m := labelRe.FindStringSubmatch(line); m != nil {
		return m[1], m[2]
	}
	return "", line
}

// ParseLine parses one line of a calculation sheet.
func ParseLine(line string, env *Env) (*Stmt, error) {
	label, rest := SplitLabel(line)
	toks, err := lex(rest)
	if err != nil {
		return nil, err
	}
	if len(toks) == 1 {
		return &Stmt{Kind: "empty", Label: label}, nil
	}
	p := &parser{toks: toks, units: env.Units, known: env.known, rates: env.hasCurrency}
	st := &Stmt{Label: label}
	// x = …   f(x, y) = …
	if toks[0].kind == tIdent && !keywords[toks[0].text] {
		if toks[1].kind == tOp && toks[1].text == "=" {
			if !reservedName(toks[0].text) {
				p.i = 2
				st.Kind, st.Name = "assign", toks[0].text
				// the variable itself is unknown while its value is read, so
				// "x = x + 1" reads the old x
				st.Expr, err = p.parseExpr()
				if err == nil {
					err = p.end()
				}
				return st, err
			}
		}
		if toks[1].kind == tOp && toks[1].text == "(" {
			j := 2
			var params []string
			ok := true
			for {
				if toks[j].kind != tIdent {
					ok = false
					break
				}
				params = append(params, toks[j].text)
				j++
				if toks[j].kind == tOp && toks[j].text == "," {
					j++
					continue
				}
				break
			}
			if ok && toks[j].kind == tOp && toks[j].text == ")" && toks[j+1].kind == tOp && toks[j+1].text == "=" && !builtinFunc(toks[0].text) {
				p.i = j + 2
				st.Kind, st.Name, st.Params = "func", toks[0].text, params
				inner := p.known
				p.known = func(n string) bool {
					for _, q := range params {
						if q == n {
							return true
						}
					}
					return inner(n)
				}
				st.Expr, err = p.parseExpr()
				if err == nil {
					err = p.end()
				}
				return st, err
			}
		}
	}
	st.Kind = "expr"
	st.Expr, err = p.parseEquation()
	if err == nil {
		err = p.end()
	}
	return st, err
}

// ParseExpr parses a single expression (for a formula's output or a graph).
func ParseExpr(src string, env *Env) (*Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, units: env.Units, known: env.known, rates: env.hasCurrency}
	n, err := p.parseEquation()
	if err == nil {
		err = p.end()
	}
	return n, err
}

// Reserved says whether a name can't be a variable (a keyword or a constant).
func Reserved(n string) bool { return reservedName(n) || keywords[n] || n == "in" }

func reservedName(n string) bool {
	switch n {
	case "pi", "e", "to", "in", "as", "of", "per", "mod":
		return true
	}
	return false
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) peekN(n int) token {
	if p.i+n < len(p.toks) {
		return p.toks[p.i+n]
	}
	return p.toks[len(p.toks)-1]
}
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }
func (p *parser) isOp(s string) bool {
	t := p.peek()
	return t.kind == tOp && t.text == s
}
func (p *parser) isWord(s string) bool {
	t := p.peek()
	return t.kind == tIdent && t.text == s
}

func (p *parser) end() error {
	t := p.peek()
	if t.kind == tEOF {
		return nil
	}
	if t.kind == tOp && t.text == ")" {
		return errf(t.pos, "there is a ) without its (")
	}
	return errf(t.pos, "I don't understand \""+t.text+"\" here")
}

func (p *parser) parseEquation() (*Node, error) {
	l, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	t := p.peek()
	if t.kind == tOp {
		op := ""
		switch t.text {
		case "=", "==":
			op = "eq"
		case "<":
			op = "lt"
		case ">":
			op = "gt"
		case "<=":
			op = "le"
		case ">=":
			op = "ge"
		case "!=":
			op = "ne"
		}
		if op != "" {
			p.next()
			r, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &Node{Op: op, Kids: []*Node{l, r}, Pos: t.pos}, nil
		}
	}
	return l, nil
}

// conversion: expr (to|in|as) target
func (p *parser) parseExpr() (*Node, error) {
	n, err := p.parseSum()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind != tIdent || !(t.text == "to" || t.text == "as" || t.text == "into" || t.text == "in" && p.inIsConversion()) {
			return n, nil
		}
		p.next()
		if p.peek().kind == tIdent {
			if sp, ok := specialTargets[p.peek().text]; ok && sp != "base" {
				p.next()
				n = &Node{Op: "conv", Name: sp, Kids: []*Node{n}, Pos: t.pos}
				continue
			}
		}
		if p.isOp("%") {
			p.next()
			n = &Node{Op: "conv", Name: "pct", Kids: []*Node{n}, Pos: t.pos}
			continue
		}
		save := p.units
		p.units = true
		saveKnown := p.known
		p.known = func(string) bool { return false }
		tgt, err := p.parseProduct(true)
		p.units, p.known = save, saveKnown
		if err != nil {
			return nil, err
		}
		n = &Node{Op: "conv", Kids: []*Node{n, tgt}, Pos: t.pos}
	}
}

// inIsConversion: "in" followed by a unit/currency/word is a conversion;
// at the end of a line (or before an operator) it is inches.
func (p *parser) inIsConversion() bool {
	n := p.peekN(1)
	if n.kind == tIdent && keywords[n.text] {
		return false
	}
	return n.kind == tIdent || n.kind == tCur || n.kind == tOp && n.text == "%"
}

func (p *parser) parseSum() (*Node, error) {
	n, err := p.parseProduct(false)
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind == tOp && (t.text == "+" || t.text == "-") {
			p.next()
			r, err := p.parseProduct(false)
			if err != nil {
				return nil, err
			}
			n = &Node{Op: t.text, Kids: []*Node{n, r}, Pos: t.pos}
			continue
		}
		return n, nil
	}
}

// startsOperand reports whether the next token can begin a value (for
// multiplication written without a sign: 2x, 3(4+5), 5 km).
func (p *parser) startsOperand() bool {
	t := p.peek()
	switch t.kind {
	case tNum, tCur:
		return true
	case tIdent:
		if keywords[t.text] {
			return false
		}
		if t.text == "in" && p.inIsConversion() {
			return false
		}
		return true
	case tOp:
		return t.text == "(" || t.text == "√"
	}
	return false
}

func (p *parser) parseProduct(target bool) (*Node, error) {
	n, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		switch {
		case t.kind == tOp && (t.text == "*" || t.text == "/"):
			p.next()
			r, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			n = &Node{Op: t.text, Kids: []*Node{n, r}, Pos: t.pos}
		case t.kind == tIdent && (t.text == "per" || t.text == "mod"):
			p.next()
			r, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			op := "/"
			if t.text == "mod" {
				op = "mod"
			}
			n = &Node{Op: op, Kids: []*Node{n, r}, Pos: t.pos}
		case !target && t.kind == tIdent && (t.text == "of" || t.text == "off" || t.text == "on"):
			p.next()
			r, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			n = &Node{Op: t.text, Kids: []*Node{n, r}, Pos: t.pos}
		case p.startsOperand():
			r, err := p.parsePower()
			if err != nil {
				return nil, err
			}
			n = &Node{Op: "imp", Kids: []*Node{n, r}, Pos: t.pos}
		default:
			return n, nil
		}
	}
}

func (p *parser) parseUnary() (*Node, error) {
	t := p.peek()
	if t.kind == tOp && t.text == "-" {
		p.next()
		n, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{Op: "neg", Kids: []*Node{n}, Pos: t.pos}, nil
	}
	if t.kind == tOp && t.text == "+" {
		p.next()
		return p.parseUnary()
	}
	return p.parsePower()
}

func (p *parser) parsePower() (*Node, error) {
	b, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	if p.isOp("^") {
		t := p.next()
		e, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{Op: "^", Kids: []*Node{b, e}, Pos: t.pos}, nil
	}
	return b, nil
}

func (p *parser) parsePostfix() (*Node, error) {
	n, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind != tOp {
			return n, nil
		}
		switch t.text {
		case "!":
			p.next()
			n = &Node{Op: "!", Kids: []*Node{n}, Pos: t.pos}
		case "%":
			p.next()
			n = &Node{Op: "pct", Kids: []*Node{n}, Pos: t.pos}
		default:
			return n, nil
		}
	}
}

func (p *parser) isUnitNameAt(name string) bool { return p.isUnitName(name) }

// isUnitName: a unit or a currency, unless the name was set as a variable.
func (p *parser) isUnitName(name string) bool {
	if !p.units || p.known(name) {
		return false
	}
	if _, ok := lookupUnit(name); ok {
		return true
	}
	return isCurrency(name, p.rates)
}

func (p *parser) parsePrimary() (*Node, error) {
	t := p.next()
	switch t.kind {
	case tNum:
		n := &Node{Op: "num", Num: t.num, Pos: t.pos}
		if !p.units {
			return n, nil
		}
		// 5 km, 3 m², 20 °C, 60 km/h (the /h is division, done above)
		var us Units
		for {
			u := p.peek()
			if u.kind != tIdent || keywords[u.text] || !p.isUnitName(u.text) {
				break
			}
			if nx := p.peekN(1); nx.kind == tOp && nx.text == "(" {
				break
			}
			if u.text == "in" && p.inIsConversion() {
				break
			}
			p.next()
			pow := 1
			if p.isOp("^") && p.peekN(1).kind == tNum && p.peekN(1).num == float64(int(p.peekN(1).num)) {
				p.next()
				pow = int(p.next().num)
			} else if p.isOp("^") && p.peekN(1).kind == tOp && p.peekN(1).text == "-" && p.peekN(2).kind == tNum {
				p.next()
				p.next()
				pow = -int(p.next().num)
			}
			us = append(us, UnitPow{Name: canonicalCurrency(u.text), Pow: pow})
		}
		if len(us) > 0 {
			n := &Node{Op: "qty", Num: t.num, Units: us, Pos: t.pos}
			// L/100km written together: per 100 km
			sl, nm, un := p.peek(), p.peekN(1), p.peekN(2)
			if sl.kind == tOp && sl.text == "/" && nm.kind == tNum && un.kind == tIdent && nm.pos == sl.pos+1 && un.pos == nm.pos+len(nm.text) && nm.num > 0 && p.isUnitNameAt(un.text) {
				p.i += 3
				n.Units = append(n.Units, UnitPow{Name: canonicalCurrency(un.text), Pow: -1})
				n.Scale = nm.num
			}
			return n, nil
		}
		return n, nil
	case tDate:
		return &Node{Op: "date", Name: t.text, Pos: t.pos}, nil
	case tCur:
		// $50, €20.5
		code := currencyAliases[t.text]
		v, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		return &Node{Op: "cur", Name: code, Kids: []*Node{v}, Pos: t.pos}, nil
	case tIdent:
		if keywords[t.text] {
			return nil, errf(t.pos, "a number is missing before \""+t.text+"\"")
		}
		primes := 0
		for p.isOp("'") {
			p.next()
			primes++
		}
		if p.isOp("(") {
			p.next()
			var args []*Node
			if !p.isOp(")") {
				for {
					a, err := p.parseEquation()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.isOp(",") || p.isOp(";") {
						p.next()
						continue
					}
					break
				}
			}
			if !p.isOp(")") {
				return nil, errf(p.peek().pos, "a ) is missing")
			}
			p.next()
			return &Node{Op: "call", Name: t.text, Num: float64(primes), Kids: args, Pos: t.pos}, nil
		}
		if primes > 0 {
			return nil, errf(t.pos, "write "+t.text+"'(x) with what to work it out at")
		}
		// √2, sin 30, ln 5: a one-value function without brackets
		if prefixFuncs[t.text] && !p.known(t.text) && p.startsOperand() {
			a, err := p.parsePower()
			if err != nil {
				return nil, err
			}
			return &Node{Op: "call", Name: t.text, Kids: []*Node{a}, Pos: t.pos}, nil
		}
		if p.isUnitName(t.text) {
			return &Node{Op: "unit", Name: canonicalCurrency(t.text), Pos: t.pos}, nil
		}
		return &Node{Op: "id", Name: t.text, Pos: t.pos}, nil
	case tOp:
		switch t.text {
		case "(":
			n, err := p.parseEquation()
			if err != nil {
				return nil, err
			}
			if p.isOp(",") {
				kids := []*Node{n}
				for p.isOp(",") {
					p.next()
					k, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					kids = append(kids, k)
				}
				n = &Node{Op: "tuple", Kids: kids, Pos: t.pos}
			}
			if !p.isOp(")") {
				return nil, errf(p.peek().pos, "a ) is missing")
			}
			p.next()
			if n.Op != "tuple" {
				n = &Node{Op: "paren", Kids: []*Node{n}, Pos: t.pos}
			}
			return n, nil
		case "[":
			var kids []*Node
			if !p.isOp("]") {
				for {
					k, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					kids = append(kids, k)
					if p.isOp(",") || p.isOp(";") {
						p.next()
						continue
					}
					break
				}
			}
			if !p.isOp("]") {
				return nil, errf(p.peek().pos, "a ] is missing")
			}
			p.next()
			return &Node{Op: "list", Kids: kids, Pos: t.pos}, nil
		case "|":
			n, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if !p.isOp("|") {
				return nil, errf(p.peek().pos, "a closing | is missing")
			}
			p.next()
			return &Node{Op: "call", Name: "abs", Kids: []*Node{n}, Pos: t.pos}, nil
		}
		return nil, errf(t.pos, "a number is missing before \""+t.text+"\"")
	case tEOF:
		return nil, errf(t.pos, "something is missing at the end")
	}
	return nil, errf(t.pos, "I don't understand \""+t.text+"\"")
}

var prefixFuncs = map[string]bool{"sqrt": true, "cbrt": true, "sin": true, "cos": true, "tan": true, "ln": true, "log": true, "exp": true, "abs": true}

// currency codes known without rates (so "50 USD" reads even offline).
var isoCodes = strings.Fields(`EUR USD GBP CHF BAM RSD HRK HUF CZK PLN RON BGN DKK SEK NOK ISK TRY RUB UAH MKD ALL
JPY CNY HKD KRW INR IDR MYR PHP SGD THB AUD NZD CAD MXN BRL ZAR ILS AED SAR BTC`)

func isCurrency(name string, rates func(string) bool) bool {
	if c, ok := currencyAliases[name]; ok && c != "" {
		return true
	}
	if len(name) != 3 || strings.ToUpper(name) != name {
		return false
	}
	for _, c := range isoCodes {
		if c == name {
			return true
		}
	}
	return rates != nil && rates(name)
}

func canonicalCurrency(name string) string {
	if c, ok := currencyAliases[name]; ok && c != "" {
		return c
	}
	return name
}
