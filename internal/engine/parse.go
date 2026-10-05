package engine

import "unicode/utf8"

// The parser is a pure function over the pending keys. It is re-run on every
// key and answers incomplete, complete or invalid, so there is no hidden
// state between keys.
//
// Grammar:  ["x] [count] [op [count]] (motion | textobject | op-again)
//       or  ["x] [count] command

type parseState int

const (
	incomplete parseState = iota
	complete
	invalid
)

// parsed is one complete command.
type parsed struct {
	reg   rune     // register from "x, or 0
	count int      // product of both counts; 0 means none was given
	op    string   // "d", "c", "y" or ""
	name  string   // motion, text object ("iw"), command, or "line" for dd/cc/yy
	arg   string   // the literal after f t F T r
	rest  []string // the keys without the leading register and count, for "."
}

// argKeys take one literal key as their argument.
var argKeys = map[string]bool{"f": true, "F": true, "t": true, "T": true, "r": true}

// searchKeys start a "/" or "?" line; after an operator they are its motion.
var searchKeys = map[string]bool{"/": true, "?": true}

var operators = map[string]bool{"d": true, "c": true, "y": true}

func digit(k string) (int, bool) {
	if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
		return int(k[0] - '0'), true
	}
	return 0, false
}

func parse(keys []string, visual bool) (p parsed, st parseState) {
	i := 0
	next := func() (string, bool) {
		if i < len(keys) {
			i++
			return keys[i-1], true
		}
		return "", false
	}
	k, ok := next()
	if !ok {
		return p, incomplete
	}
	if k == `"` {
		r, ok := next()
		if !ok {
			return p, incomplete
		}
		if !isText(r) || utf8.RuneCountInString(r) != 1 {
			return p, invalid
		}
		p.reg = []rune(r)[0]
		if k, ok = next(); !ok {
			return p, incomplete
		}
	}
	count := func(c *int) bool {
		for d, isD := digit(k); isD && (d != 0 || *c != 0); d, isD = digit(k) {
			*c = *c*10 + d
			if k, ok = next(); !ok {
				return false
			}
		}
		return true
	}
	c1, c2 := 0, 0
	if !count(&c1) {
		return p, incomplete
	}
	restStart := i - 1
	if operators[k] && !visual {
		p.op = k
		if k, ok = next(); !ok {
			return p, incomplete
		}
		if !count(&c2) {
			return p, incomplete
		}
	}
	if k == "g" {
		k2, ok := next()
		if !ok {
			return p, incomplete
		}
		k = "g" + k2
	}
	switch {
	case p.op != "" && k == p.op:
		p.name = "line"
	case (p.op != "" || visual) && (k == "i" || k == "a"):
		k2, ok := next()
		if !ok {
			return p, incomplete
		}
		p.name = k + k2
	case argKeys[k]:
		a, ok := next()
		if !ok {
			return p, incomplete
		}
		if !isText(a) {
			return p, invalid
		}
		p.name, p.arg = k, a
	default:
		p.name = k
	}
	if c1 != 0 || c2 != 0 {
		p.count = max(c1, 1) * max(c2, 1)
	}
	p.rest = keys[restStart:]

	_, isMotion := motions[p.name]
	switch {
	case p.op != "":
		if !isMotion && !isObject(p.name) && p.name != "line" && !searchKeys[p.name] {
			return p, invalid
		}
	case visual:
		if _, ok := visualCmds[p.name]; !ok && !isMotion && !isObject(p.name) {
			return p, invalid
		}
	default:
		if _, ok := normalCmds[p.name]; !ok && !isMotion {
			return p, invalid
		}
	}
	return p, complete
}

func isObject(name string) bool {
	if len(name) < 2 || name[0] != 'i' && name[0] != 'a' {
		return false
	}
	_, ok := objects[name[1:]]
	return ok
}

func (e *Engine) normalKey() {
	visual := e.Mode.visual()
	p, st := parse(e.keys, visual)
	if st == incomplete {
		return
	}
	e.keys = nil
	if st == invalid {
		return
	}
	switch {
	case p.op != "" && searchKeys[p.name]:
		// d/foo: the operator finishes when the search line is entered.
		e.startCmdline(Search, p.name == "?")
		e.cmd.opPending = &p
	case p.op != "":
		e.applyOp(p)
		e.setDot(p)
	case visual && visualCmds[p.name] != nil:
		visualCmds[p.name](e, p)
	case visual && isObject(p.name):
		e.selectObject(p.name)
	case motions[p.name] != nil:
		e.doMotion(p)
	default:
		normalCmds[p.name](e, p)
		if changeCmds[p.name] {
			e.setDot(p)
		}
	}
}
