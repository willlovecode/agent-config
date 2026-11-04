package catalog

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// Rules are small boolean expressions over scan facts and answers:
//
//	always | never
//	use == personal
//	stacks contains java and tools contains claude_code and jdk >= 17
//	not (use == personal) or budget != lean
//
// A bare fact name is true when the fact is set (true, a non-empty string or list, a
// non-zero number). Versions compare by their leading number ("21.0.2" >= 17).

// Facts maps a fact name to a bool, string, []string or float64.
type Facts map[string]any

type Rule struct {
	src  string
	root node
}

func (r *Rule) String() string { return r.src }

func (r *Rule) Eval(f Facts) bool { return r == nil || r.root.eval(f) }

func (r *Rule) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	p, err := ParseRule(s)
	if err != nil {
		return err
	}
	*r = *p
	return nil
}

type node interface{ eval(Facts) bool }

type andNode struct{ l, r node }
type orNode struct{ l, r node }
type notNode struct{ n node }
type constNode bool
type cmpNode struct{ fact, op, value string }

func (n andNode) eval(f Facts) bool { return n.l.eval(f) && n.r.eval(f) }
func (n orNode) eval(f Facts) bool  { return n.l.eval(f) || n.r.eval(f) }
func (n notNode) eval(f Facts) bool { return !n.n.eval(f) }
func (n constNode) eval(Facts) bool { return bool(n) }

func (n cmpNode) eval(f Facts) bool {
	v, ok := f[n.fact]
	if n.op == "" {
		return ok && truthy(v)
	}
	if !ok {
		return n.op == "!="
	}
	switch n.op {
	case "contains":
		if l, ok := v.([]string); ok {
			for _, x := range l {
				if strings.EqualFold(x, n.value) {
					return true
				}
			}
		}
		return false
	case "==":
		return strings.EqualFold(text(v), n.value)
	case "!=":
		return !strings.EqualFold(text(v), n.value)
	}
	a, okA := number(v)
	b, okB := number(n.value)
	if !okA || !okB {
		return false
	}
	switch n.op {
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case "<":
		return a < b
	}
	return false
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x != ""
	case []string:
		return len(x) > 0
	case float64:
		return x != 0
	}
	return v != nil
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// number reads a float or the leading number of a version string.
func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		end := strings.IndexFunc(x, func(r rune) bool { return !unicode.IsDigit(r) && r != '.' })
		if end < 0 {
			end = len(x)
		}
		s := x[:end]
		if i := strings.Index(s, "."); i >= 0 {
			if j := strings.Index(s[i+1:], "."); j >= 0 {
				s = s[:i+1+j] // "21.0.2" -> "21.0"
			}
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
	return 0, false
}

// ParseRule parses a rule; an empty string means never.
func ParseRule(src string) (*Rule, error) {
	if strings.TrimSpace(src) == "" {
		return &Rule{src: "never", root: constNode(false)}, nil
	}
	toks, err := lex(src)
	if err != nil {
		return nil, fmt.Errorf("rule %q: %w", src, err)
	}
	p := &parser{toks: toks}
	n, err := p.or()
	if err != nil {
		return nil, fmt.Errorf("rule %q: %w", src, err)
	}
	if p.i < len(p.toks) {
		return nil, fmt.Errorf("rule %q: unexpected %q", src, p.toks[p.i])
	}
	return &Rule{src: src, root: n}, nil
}

func lex(s string) ([]string, error) {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '(' || c == ')':
			out = append(out, string(c))
			i++
		case strings.ContainsRune("=!<>", rune(c)):
			j := i + 1
			if j < len(s) && s[j] == '=' {
				j++
			}
			out = append(out, s[i:j])
			i = j
		case c == '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				return nil, fmt.Errorf("unterminated quote")
			}
			out = append(out, s[i:i+j+2])
			i += j + 2
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t()=!<>\"", rune(s[j])) {
				j++
			}
			out = append(out, s[i:j])
			i = j
		}
	}
	return out, nil
}

type parser struct {
	toks []string
	i    int
}

func (p *parser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *parser) next() string { t := p.peek(); p.i++; return t }

func (p *parser) or() (node, error) {
	l, err := p.and()
	for err == nil && p.peek() == "or" {
		p.next()
		var r node
		r, err = p.and()
		l = orNode{l, r}
	}
	return l, err
}

func (p *parser) and() (node, error) {
	l, err := p.not()
	for err == nil && p.peek() == "and" {
		p.next()
		var r node
		r, err = p.not()
		l = andNode{l, r}
	}
	return l, err
}

func (p *parser) not() (node, error) {
	if p.peek() == "not" {
		p.next()
		n, err := p.not()
		return notNode{n}, err
	}
	return p.atom()
}

var ops = map[string]bool{"==": true, "!=": true, ">=": true, "<=": true, ">": true, "<": true, "contains": true}

func (p *parser) atom() (node, error) {
	t := p.next()
	switch t {
	case "":
		return nil, fmt.Errorf("unexpected end")
	case "(":
		n, err := p.or()
		if err == nil && p.next() != ")" {
			err = fmt.Errorf("missing )")
		}
		return n, err
	case "always":
		return constNode(true), nil
	case "never":
		return constNode(false), nil
	case ")", "and", "or":
		return nil, fmt.Errorf("unexpected %q", t)
	}
	if !ops[p.peek()] {
		return cmpNode{fact: t}, nil
	}
	op := p.next()
	v := p.next()
	if v == "" || v == ")" || ops[v] {
		return nil, fmt.Errorf("missing value after %s", op)
	}
	return cmpNode{fact: t, op: op, value: strings.Trim(v, `"`)}, nil
}
