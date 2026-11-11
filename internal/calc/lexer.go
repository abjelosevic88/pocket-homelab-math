package calc

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokKind int

const (
	tEOF tokKind = iota
	tNum
	tIdent
	tOp   // + - * / ^ ( ) [ ] , = ! % ' | < > <= >= ;
	tDate // 2026-12-25
	tCur  // a currency symbol written before a number: $ € £
)

type token struct {
	kind tokKind
	text string
	num  float64
	pos  int
}

var superscripts = map[rune]string{'⁰': "0", '¹': "1", '²': "2", '³': "3", '⁴': "4", '⁵': "5", '⁶': "6", '⁷': "7", '⁸': "8", '⁹': "9", '⁻': "-"}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_' || r == '°' || r == 'µ' || r == 'Ω' || r == '∞'
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r)
}

// lex splits a line into tokens. Thousands separators written as "_" or a
// thin space are dropped; the decimal mark is always ".".
func lex(src string) ([]token, error) {
	var out []token
	i := 0
	for i < len(src) {
		r, w := utf8.DecodeRuneInString(src[i:])
		start := i
		switch {
		case r == ' ' || r == '\t' || r == ' ' || r == ' ' || r == ' ':
			i += w
		case r >= '0' && r <= '9' || r == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			// a date?
			if i+10 <= len(src) && isDate(src[i:i+10]) && (i+10 == len(src) || !isDigitByte(src[i+10])) {
				out = append(out, token{kind: tDate, text: src[i : i+10], pos: start})
				i += 10
				continue
			}
			j := i
			if strings.HasPrefix(src[i:], "0x") || strings.HasPrefix(src[i:], "0b") || strings.HasPrefix(src[i:], "0o") {
				base := map[byte]int{'x': 16, 'b': 2, 'o': 8}[src[i+1]]
				j = i + 2
				for j < len(src) && (isHexByte(src[j]) && base == 16 || src[j] >= '0' && src[j] <= '1' || base == 8 && src[j] >= '0' && src[j] <= '7' || src[j] == '_') {
					j++
				}
				var n float64
				for _, c := range strings.ReplaceAll(src[i+2:j], "_", "") {
					n = n*float64(base) + float64(hexVal(byte(c)))
				}
				out = append(out, token{kind: tNum, text: src[i:j], num: n, pos: start})
				i = j
				continue
			}
			for j < len(src) && (isDigitByte(src[j]) || src[j] == '_' || src[j] == '.') {
				j++
			}
			if j < len(src) && (src[j] == 'e' || src[j] == 'E') {
				k := j + 1
				if k < len(src) && (src[k] == '+' || src[k] == '-') {
					k++
				}
				if k < len(src) && isDigitByte(src[k]) {
					for k < len(src) && isDigitByte(src[k]) {
						k++
					}
					j = k
				}
			}
			txt := strings.ReplaceAll(src[i:j], "_", "")
			var n float64
			if _, err := fmt.Sscanf(txt, "%g", &n); err != nil || strings.Count(txt, ".") > 1 {
				return nil, &Error{Pos: start, Msg: "this number does not look right: " + src[i:j]}
			}
			out = append(out, token{kind: tNum, text: txt, num: n, pos: start})
			i = j
		case superscripts[r] != "":
			j := i
			s := ""
			for j < len(src) {
				r2, w2 := utf8.DecodeRuneInString(src[j:])
				if superscripts[r2] == "" {
					break
				}
				s += superscripts[r2]
				j += w2
			}
			var n float64
			fmt.Sscanf(s, "%g", &n)
			out = append(out, token{kind: tOp, text: "^", pos: start}, token{kind: tNum, text: s, num: n, pos: start})
			i = j
		case r == '°' && i+w < len(src) && (src[i+w] == 'C' || src[i+w] == 'F') && (i+w+1 == len(src) || !isIdentPart(rune(src[i+w+1]))):
			out = append(out, token{kind: tIdent, text: src[i : i+w+1], pos: start})
			i += w + 1
		case isIdentStart(r):
			j := i + w
			for j < len(src) {
				r2, w2 := utf8.DecodeRuneInString(src[j:])
				if !isIdentPart(r2) || r2 == '°' {
					break
				}
				j += w2
			}
			out = append(out, token{kind: tIdent, text: src[i:j], pos: start})
			i = j
		case r == '$' || r == '€' || r == '£' || r == '¥' || r == '₹' || r == '₽' || r == '₺' || r == '₩':
			out = append(out, token{kind: tCur, text: string(r), pos: start})
			i += w
		case r == '#':
			i = len(src) // a comment to the end of the line
		case r == '/' && i+1 < len(src) && src[i+1] == '/':
			i = len(src)
		default:
			op := string(r)
			switch r {
			case '×', '·', '⋅':
				op = "*"
			case '÷', '∕':
				op = "/"
			case '−', '–':
				op = "-"
			case '→':
				op = "to"
			case '≤':
				op = "<="
			case '≥':
				op = ">="
			case '√':
				out = append(out, token{kind: tIdent, text: "sqrt", pos: start})
				i += w
				continue
			case 'π':
				out = append(out, token{kind: tIdent, text: "pi", pos: start})
				i += w
				continue
			case '*':
				if i+1 < len(src) && src[i+1] == '*' {
					op = "^"
					w = 2
				}
			case '<', '>', '=', '!':
				if i+1 < len(src) && src[i+1] == '=' {
					op = src[i : i+2]
					w = 2
				}
			}
			if op == "to" {
				out = append(out, token{kind: tIdent, text: "to", pos: start})
				i += w
				continue
			}
			if !strings.Contains("+-*/^()[],=!%'|<><=>=!=;{}:", op) {
				return nil, &Error{Pos: start, Msg: fmt.Sprintf("I don't know the sign %q", op)}
			}
			out = append(out, token{kind: tOp, text: op, pos: start})
			i += w
		}
	}
	out = append(out, token{kind: tEOF, pos: len(src)})
	return out, nil
}

func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }
func isHexByte(b byte) bool {
	return isDigitByte(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
func hexVal(b byte) int {
	switch {
	case isDigitByte(b):
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	default:
		return int(b-'A') + 10
	}
}

func isDate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, c := range []byte(s) {
		if i == 4 || i == 7 {
			continue
		}
		if !isDigitByte(c) {
			return false
		}
	}
	return true
}
