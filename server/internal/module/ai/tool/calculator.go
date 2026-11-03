package tool

import (
	"errors"
	"math/big"
	"strings"
	"unicode"
)

type arithmetic struct {
	source string
	pos    int
	depth  int
	ops    int
}

func calculate(source string) (string, error) {
	if len(source) == 0 || len(source) > 2048 {
		return "", errors.New("expression length invalid")
	}
	p := arithmetic{source: source}
	v, e := p.expression()
	if e != nil {
		return "", e
	}
	p.space()
	if p.pos != len(p.source) {
		return "", errors.New("unexpected arithmetic input")
	}
	if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
		return "", errors.New("arithmetic range exceeded")
	}
	out := v.FloatString(16)
	out = strings.TrimRight(strings.TrimRight(out, "0"), ".")
	if out == "-0" {
		out = "0"
	}
	return out, nil
}
func (p *arithmetic) space() {
	for p.pos < len(p.source) && unicode.IsSpace(rune(p.source[p.pos])) {
		p.pos++
	}
}
func (p *arithmetic) take(c byte) bool {
	p.space()
	if p.pos < len(p.source) && p.source[p.pos] == c {
		p.pos++
		return true
	}
	return false
}
func (p *arithmetic) expression() (*big.Rat, error) {
	v, e := p.term()
	if e != nil {
		return nil, e
	}
	for {
		if p.take('+') {
			r, e := p.term()
			if e != nil {
				return nil, e
			}
			v.Add(v, r)
		} else if p.take('-') {
			r, e := p.term()
			if e != nil {
				return nil, e
			}
			v.Sub(v, r)
		} else {
			return v, nil
		}
		p.ops++
		if p.ops > 256 {
			return nil, errors.New("operation limit exceeded")
		}
	}
}
func (p *arithmetic) term() (*big.Rat, error) {
	v, e := p.factor()
	if e != nil {
		return nil, e
	}
	for {
		if p.take('*') {
			r, e := p.factor()
			if e != nil {
				return nil, e
			}
			v.Mul(v, r)
		} else if p.take('/') {
			r, e := p.factor()
			if e != nil {
				return nil, e
			}
			if r.Sign() == 0 {
				return nil, errors.New("division by zero")
			}
			v.Quo(v, r)
		} else {
			return v, nil
		}
		p.ops++
		if p.ops > 256 || v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return nil, errors.New("arithmetic limit exceeded")
		}
	}
}
func (p *arithmetic) factor() (*big.Rat, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 64 {
		return nil, errors.New("expression depth exceeded")
	}
	if p.take('+') {
		return p.factor()
	}
	if p.take('-') {
		v, e := p.factor()
		if e != nil {
			return nil, e
		}
		return v.Neg(v), nil
	}
	if p.take('(') {
		v, e := p.expression()
		if e != nil {
			return nil, e
		}
		if !p.take(')') {
			return nil, errors.New("unclosed parenthesis")
		}
		return v, nil
	}
	p.space()
	start := p.pos
	dot := false
	digits := 0
	for p.pos < len(p.source) {
		c := p.source[p.pos]
		if c >= '0' && c <= '9' {
			p.pos++
			digits++
			continue
		}
		if c == '.' && !dot {
			dot = true
			p.pos++
			continue
		}
		break
	}
	if digits == 0 || p.pos-start > 128 {
		return nil, errors.New("number invalid")
	}
	v, ok := new(big.Rat).SetString(p.source[start:p.pos])
	if !ok {
		return nil, errors.New("number invalid")
	}
	return v, nil
}
