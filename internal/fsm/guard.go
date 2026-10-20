package fsm

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

type guardOp int

const (
	opName guardOp = iota
	opNot
	opAnd
	opOr
)

type GuardExpr struct {
	op    guardOp
	name  string
	left  *GuardExpr
	right *GuardExpr
}

func ParseGuard(source string) (*GuardExpr, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, nil
	}
	tokens, err := tokenizeGuard(source)
	if err != nil {
		return nil, err
	}
	p := guardParser{tokens: tokens, source: source}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.tokens) {
		return nil, fmt.Errorf("%w: %q: sobra %q", ErrMalformedGuard, source, strings.Join(p.tokens[p.pos:], " "))
	}
	return expr, nil
}

func (g *GuardExpr) Eval(truth func(name string) bool) bool {
	switch g.op {
	case opNot:
		return !g.left.Eval(truth)
	case opAnd:
		return g.left.Eval(truth) && g.right.Eval(truth)
	case opOr:
		return g.left.Eval(truth) || g.right.Eval(truth)
	default:
		return truth(g.name)
	}
}

func (g *GuardExpr) Names() []string {
	var names []string
	var walk func(*GuardExpr)
	walk = func(n *GuardExpr) {
		if n == nil {
			return
		}
		if n.op == opName && !slices.Contains(names, n.name) {
			names = append(names, n.name)
		}
		walk(n.left)
		walk(n.right)
	}
	walk(g)
	return names
}

func (g *GuardExpr) String() string {
	switch g.op {
	case opNot:
		return "!" + g.left.grouped(opNot)
	case opAnd:
		return g.left.grouped(opAnd) + " && " + g.right.grouped(opAnd)
	case opOr:
		return g.left.grouped(opOr) + " || " + g.right.grouped(opOr)
	default:
		return g.name
	}
}

func (g *GuardExpr) grouped(parent guardOp) string {
	if precedence(g.op) >= precedence(parent) {
		return g.String()
	}
	return "(" + g.String() + ")"
}

func precedence(op guardOp) int {
	switch op {
	case opOr:
		return 1
	case opAnd:
		return 2
	default:
		return 3
	}
}

func tokenizeGuard(source string) ([]string, error) {
	var tokens []string
	runes := []rune(source)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '(' || r == ')' || r == '!':
			tokens = append(tokens, string(r))
			i++
		case r == '&' || r == '|':
			if i+1 >= len(runes) || runes[i+1] != r {
				return nil, fmt.Errorf("%w: %q: operador incompleto na posição %d", ErrMalformedGuard, source, i)
			}
			tokens = append(tokens, string([]rune{r, r}))
			i += 2
		case isIdentStart(r):
			start := i
			for i < len(runes) && isIdentPart(runes[i]) {
				i++
			}
			tokens = append(tokens, string(runes[start:i]))
		default:
			return nil, fmt.Errorf("%w: %q: caractere %q inesperado", ErrMalformedGuard, source, r)
		}
	}
	return tokens, nil
}

func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
}

type guardParser struct {
	tokens []string
	pos    int
	source string
}

func (p *guardParser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *guardParser) parseOr() (*GuardExpr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() == "||" {
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &GuardExpr{op: opOr, left: left, right: right}
	}
	return left, nil
}

func (p *guardParser) parseAnd() (*GuardExpr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek() == "&&" {
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &GuardExpr{op: opAnd, left: left, right: right}
	}
	return left, nil
}

func (p *guardParser) parseUnary() (*GuardExpr, error) {
	switch token := p.peek(); {
	case token == "!":
		p.pos++
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &GuardExpr{op: opNot, left: inner}, nil
	case token == "(":
		p.pos++
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("%w: %q: parêntese não fechado", ErrMalformedGuard, p.source)
		}
		p.pos++
		return inner, nil
	case token != "" && isIdentStart([]rune(token)[0]):
		p.pos++
		return &GuardExpr{op: opName, name: token}, nil
	default:
		return nil, fmt.Errorf("%w: %q: esperado nome de guarda na posição %d", ErrMalformedGuard, p.source, p.pos)
	}
}

type ActionRef struct {
	Name string
	Arg  string
}

func ParseAction(source string) (ActionRef, error) {
	source = strings.TrimSpace(source)
	name, rest, hasArg := strings.Cut(source, "(")
	name = strings.TrimSpace(name)
	if name == "" || !isIdentStart([]rune(name)[0]) || strings.ContainsFunc(name, func(r rune) bool { return !isIdentPart(r) }) {
		return ActionRef{}, fmt.Errorf("%w: %q", ErrMalformedAction, source)
	}
	if !hasArg {
		return ActionRef{Name: name}, nil
	}
	arg, ok := strings.CutSuffix(rest, ")")
	if !ok || strings.ContainsAny(arg, "()") {
		return ActionRef{}, fmt.Errorf("%w: %q", ErrMalformedAction, source)
	}
	return ActionRef{Name: name, Arg: strings.TrimSpace(arg)}, nil
}

func (a ActionRef) String() string {
	if a.Arg == "" {
		return a.Name
	}
	return a.Name + "(" + a.Arg + ")"
}
