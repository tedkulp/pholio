// PROTOTYPE — throwaway code for wayfinder ticket "Vim editor prototype".

package engine

import (
	"strconv"
	"strings"
	"unicode"
)

// ---------- parser ----------
//
// Grammar:  ["x] [count] [op [count]] (motion | textobject | op-again)
//       or  ["x] [count] command
// The parser is a pure function over the pending keys; it is re-run on every
// key and answers incomplete / complete / invalid. No hidden state.

const (
	incomplete = iota
	complete
	invalid
)

type parsed struct {
	reg   rune
	count int    // product of both counts; 0 means "none given"
	op    string // "d" "c" "y" or ""
	name  string // motion, text object ("iw"), command, or "line" for dd/cc/yy
	arg   string // literal for f t F T r
	rest  []string
}

var argKeys = map[string]bool{"f": true, "F": true, "t": true, "T": true, "r": true}

func digit(k string) (int, bool) {
	if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
		return int(k[0] - '0'), true
	}
	return 0, false
}

func parse(keys []string, visual bool) (p parsed, st int) {
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
		p.reg = []rune(r)[0]
		if k, ok = next(); !ok {
			return p, incomplete
		}
	}
	c1, c2 := 0, 0
	for d, isD := digit(k); isD && (d != 0 || c1 != 0); d, isD = digit(k) {
		c1 = c1*10 + d
		if k, ok = next(); !ok {
			return p, incomplete
		}
	}
	restStart := i - 1
	if !visual && (k == "d" || k == "c" || k == "y") {
		p.op = k
		if k, ok = next(); !ok {
			return p, incomplete
		}
		for d, isD := digit(k); isD && (d != 0 || c2 != 0); d, isD = digit(k) {
			c2 = c2*10 + d
			if k, ok = next(); !ok {
				return p, incomplete
			}
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
		if _, ok := objects[k2]; !ok {
			return p, invalid
		}
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
	_, isObject := objects[strings.TrimLeft(p.name, "ia")]
	isObject = isObject && len(p.name) == 2 && (p.name[0] == 'i' || p.name[0] == 'a')
	switch {
	case p.op != "":
		if !isMotion && !isObject && p.name != "line" && p.name != "/" && p.name != "?" {
			return p, invalid
		}
	case visual:
		if _, ok := visualCmds[p.name]; !ok && !isMotion && !isObject {
			return p, invalid
		}
	default:
		if _, ok := normalCmds[p.name]; !ok && !isMotion {
			return p, invalid
		}
	}
	return p, complete
}

func (e *Engine) normalKey() {
	visual := e.Mode != Normal
	p, st := parse(e.keys, visual)
	if st == incomplete {
		return
	}
	e.keys = nil
	if st == invalid {
		return
	}
	_, isMotion := motions[p.name]
	switch {
	case p.op != "":
		e.applyOp(p)
		e.setDot(p)
	case visual && len(p.name) == 2 && (p.name[0] == 'i' || p.name[0] == 'a'):
		if a, z, lw, ok := e.object(p.name); ok {
			e.anchor = a
			e.Cur = Pos{z.Line, prevG(e.line(z.Line), z.Col)}
			if lw {
				e.Mode = VisualLine
			}
		}
	case isMotion && !(visual && visualCmds[p.name] != nil):
		e.doMotion(p)
	case visual:
		visualCmds[p.name](e, p)
	default:
		normalCmds[p.name](e, p)
		if changeCmds[p.name] {
			e.setDot(p)
		}
	}
}

func (e *Engine) setDot(p parsed) {
	if e.replaying {
		return
	}
	e.dot = append([]string(nil), p.rest...)
	e.dotCount = p.count
	e.recording = e.Mode == Insert
}

func (e *Engine) line(i int) string { return e.Buf.Line(i) }

// ---------- motions ----------

type kind int

const (
	excl kind = iota
	incl
	lines
)

type motionFn func(e *Engine, n int, arg string) (Pos, kind, bool)

var keepWant = map[string]bool{"j": true, "k": true, "up": true, "down": true}

var motions map[string]motionFn

func init() {
	h := func(e *Engine, n int, _ string) (Pos, kind, bool) {
		l, c := e.line(e.Cur.Line), e.Cur.Col
		for i := 0; i < max(n, 1) && c > 0; i++ {
			c = prevG(l, c)
		}
		return Pos{e.Cur.Line, c}, excl, c != e.Cur.Col
	}
	l := func(e *Engine, n int, _ string) (Pos, kind, bool) {
		ln, c := e.line(e.Cur.Line), e.Cur.Col
		for i := 0; i < max(n, 1) && c < len(ln); i++ {
			c = nextG(ln, c)
		}
		return Pos{e.Cur.Line, c}, excl, c != e.Cur.Col
	}
	vert := func(dir int) motionFn {
		return func(e *Engine, n int, _ string) (Pos, kind, bool) {
			t := max(0, min(e.Cur.Line+dir*max(n, 1), e.Buf.LineCount()-1))
			return e.atWant(t), lines, t != e.Cur.Line
		}
	}
	word := func(f func(e *Engine, p Pos, big bool) Pos, big bool, k kind) motionFn {
		return func(e *Engine, n int, _ string) (Pos, kind, bool) {
			p := e.Cur
			for i := 0; i < max(n, 1); i++ {
				p = f(e, p, big)
			}
			return p, k, true
		}
	}
	find := func(name string) motionFn {
		return func(e *Engine, n int, arg string) (Pos, kind, bool) {
			e.lastFind = name + arg
			return e.find(name, arg, max(n, 1), false)
		}
	}
	page := func(dir, div int) motionFn {
		return func(e *Engine, n int, _ string) (Pos, kind, bool) {
			step := max(e.PageLines/div, 1)
			if div == 1 {
				step = max(e.PageLines-2, 1) // ctrl+f/b keep two lines of overlap
			}
			t := max(0, min(e.Cur.Line+dir*max(n, 1)*step, e.Buf.LineCount()-1))
			return Pos{t, firstNonBlank(e.line(t))}, lines, true
		}
	}
	motions = map[string]motionFn{
		"h": h, "left": h, "backspace": h,
		"l": l, "right": l, " ": l,
		"j": vert(1), "down": vert(1),
		"k": vert(-1), "up": vert(-1),
		"0":    func(e *Engine, _ int, _ string) (Pos, kind, bool) { return Pos{e.Cur.Line, 0}, excl, true },
		"home": func(e *Engine, _ int, _ string) (Pos, kind, bool) { return Pos{e.Cur.Line, 0}, excl, true },
		"^": func(e *Engine, _ int, _ string) (Pos, kind, bool) {
			return Pos{e.Cur.Line, firstNonBlank(e.line(e.Cur.Line))}, excl, true
		},
		"$": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			t := min(e.Cur.Line+max(n, 1)-1, e.Buf.LineCount()-1)
			return Pos{t, lastG(e.line(t))}, incl, true
		},
		"enter": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			t := min(e.Cur.Line+max(n, 1), e.Buf.LineCount()-1)
			return Pos{t, firstNonBlank(e.line(t))}, lines, true
		},
		"w":  word((*Engine).wordFwd, false, excl),
		"W":  word((*Engine).wordFwd, true, excl),
		"b":  word((*Engine).wordBack, false, excl),
		"B":  word((*Engine).wordBack, true, excl),
		"e":  word((*Engine).wordEnd, false, incl),
		"E":  word((*Engine).wordEnd, true, incl),
		"ge": word((*Engine).wordEndBack, false, incl),
		"gE": word((*Engine).wordEndBack, true, incl),
		"gg": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			t := max(0, min(n-1, e.Buf.LineCount()-1))
			return Pos{t, firstNonBlank(e.line(t))}, lines, true
		},
		"G": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			t := e.Buf.LineCount() - 1
			if n > 0 {
				t = min(n-1, t)
			}
			return Pos{t, firstNonBlank(e.line(t))}, lines, true
		},
		"f": find("f"), "F": find("F"), "t": find("t"), "T": find("T"),
		";": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			if e.lastFind == "" {
				return e.Cur, excl, false
			}
			return e.find(e.lastFind[:1], e.lastFind[1:], max(n, 1), true)
		},
		",": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			if e.lastFind == "" {
				return e.Cur, excl, false
			}
			rev := map[string]string{"f": "F", "F": "f", "t": "T", "T": "t"}[e.lastFind[:1]]
			return e.find(rev, e.lastFind[1:], max(n, 1), true)
		},
		"}": func(e *Engine, n int, _ string) (Pos, kind, bool) { return e.para(1, max(n, 1)), excl, true },
		"{": func(e *Engine, n int, _ string) (Pos, kind, bool) { return e.para(-1, max(n, 1)), excl, true },
		"%": func(e *Engine, _ int, _ string) (Pos, kind, bool) {
			p, ok := e.matchBracket()
			return p, incl, ok
		},
		"n": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			p, ok := e.searchFrom(e.Cur, e.searchBack, max(n, 1))
			return p, excl, ok
		},
		"N": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			p, ok := e.searchFrom(e.Cur, !e.searchBack, max(n, 1))
			return p, excl, ok
		},
		"ctrl+d": page(1, 2), "ctrl+u": page(-1, 2),
		"ctrl+f": page(1, 1), "ctrl+b": page(-1, 1), "pgdown": page(1, 1), "pgup": page(-1, 1),
	}
}

func (e *Engine) atWant(line int) Pos {
	l := e.line(line)
	if e.want < 0 {
		return Pos{line, lastG(l)}
	}
	return Pos{line, colAtCells(l, e.want)}
}

func (e *Engine) doMotion(p parsed) {
	to, _, ok := motions[p.name](e, p.count, p.arg)
	if !ok {
		return
	}
	e.Cur = to
	if p.name == "$" {
		e.want = -1
	}
	e.keptWant = keepWant[p.name] || p.name == "$"
}

// next/prev step one grapheme through the whole buffer; the end of each line
// is a virtual "\n" position.
func (e *Engine) next(p Pos) (Pos, bool) {
	l := e.line(p.Line)
	if p.Col < len(l) {
		return Pos{p.Line, nextG(l, p.Col)}, true
	}
	if p.Line+1 < e.Buf.LineCount() {
		return Pos{p.Line + 1, 0}, true
	}
	return p, false
}

func (e *Engine) prev(p Pos) (Pos, bool) {
	if p.Col > 0 {
		return Pos{p.Line, prevG(e.line(p.Line), p.Col)}, true
	}
	if p.Line > 0 {
		return Pos{p.Line - 1, len(e.line(p.Line - 1))}, true
	}
	return p, false
}

func (e *Engine) cls(p Pos, big bool) int { return class(runeAt(e.line(p.Line), p.Col), big) }
func (e *Engine) emptyLine(p Pos) bool    { return p.Col == 0 && e.line(p.Line) == "" }

func (e *Engine) wordFwd(p Pos, big bool) Pos {
	start := p
	ok := true
	if c := e.cls(p, big); c != clsBlank {
		for ok && e.cls(p, big) == c {
			p, ok = e.next(p)
		}
	}
	for ok && !(e.emptyLine(p) && p != start) && e.cls(p, big) == clsBlank {
		var q Pos
		if q, ok = e.next(p); ok {
			p = q
		}
	}
	if !ok {
		return Pos{p.Line, len(e.line(p.Line))}
	}
	return p
}

func (e *Engine) wordEnd(p Pos, big bool) Pos {
	p, ok := e.next(p)
	for ok && e.cls(p, big) == clsBlank {
		p, ok = e.next(p)
	}
	c := e.cls(p, big)
	for {
		q, ok := e.next(p)
		if !ok || q.Line != p.Line || e.cls(q, big) != c {
			return p
		}
		p = q
	}
}

func (e *Engine) wordBack(p Pos, big bool) Pos {
	p, ok := e.prev(p)
	for ok && e.cls(p, big) == clsBlank && !e.emptyLine(p) {
		p, ok = e.prev(p)
	}
	if e.emptyLine(p) {
		return p
	}
	c := e.cls(p, big)
	for {
		q, ok := e.prev(p)
		if !ok || q.Line != p.Line || e.cls(q, big) != c {
			return p
		}
		p = q
	}
}

func (e *Engine) wordEndBack(p Pos, big bool) Pos {
	c := e.cls(p, big)
	for {
		q, ok := e.prev(p)
		if !ok {
			return p
		}
		p = q
		if c == clsBlank || e.cls(p, big) != c {
			break
		}
	}
	for e.cls(p, big) == clsBlank && !e.emptyLine(p) {
		q, ok := e.prev(p)
		if !ok {
			return p
		}
		p = q
	}
	return p
}

func (e *Engine) find(name, arg string, n int, repeat bool) (Pos, kind, bool) {
	l, c := e.line(e.Cur.Line), e.Cur.Col
	found, cnt := -1, 0
	switch name {
	case "f", "t":
		start := nextG(l, c)
		if name == "t" && repeat {
			start = nextG(l, start)
		}
		for i := start; i < len(l); i = nextG(l, i) {
			if strings.HasPrefix(l[i:nextG(l, i)], arg) { // "f👍" matches "👍🏽"
				if cnt++; cnt == n {
					found = i
					break
				}
			}
		}
		if found < 0 {
			return e.Cur, incl, false
		}
		if name == "t" {
			found = prevG(l, found)
		}
		return Pos{e.Cur.Line, found}, incl, true
	default: // F T
		start := c
		if name == "T" && repeat {
			start = prevG(l, start)
		}
		for i := start; i > 0; {
			i = prevG(l, i)
			if strings.HasPrefix(l[i:nextG(l, i)], arg) { // "f👍" matches "👍🏽"
				if cnt++; cnt == n {
					found = i
					break
				}
			}
		}
		if found < 0 {
			return e.Cur, excl, false
		}
		if name == "T" {
			found = nextG(l, found)
		}
		return Pos{e.Cur.Line, found}, excl, true
	}
}

func (e *Engine) para(dir, n int) Pos {
	l, last := e.Cur.Line, e.Buf.LineCount()-1
	blank := func(i int) bool { return isBlankLine(e.line(i)) }
	for ; n > 0; n-- {
		if dir > 0 {
			for l < last && blank(l) {
				l++
			}
			for l < last && !blank(l) {
				l++
			}
		} else {
			for l > 0 && blank(l) {
				l--
			}
			for l > 0 && !blank(l) {
				l--
			}
		}
	}
	if dir > 0 && l == last && !blank(l) {
		return Pos{l, len(e.line(l))}
	}
	return Pos{l, 0}
}

var bracketPairs = map[byte]byte{'(': ')', '[': ']', '{': '}'}

func (e *Engine) matchBracket() (Pos, bool) {
	l := e.line(e.Cur.Line)
	i := e.Cur.Col
	for i < len(l) && !strings.ContainsRune("()[]{}", rune(l[i])) {
		i++
	}
	if i >= len(l) {
		return e.Cur, false
	}
	text, o := e.Buf.String(), e.Buf.offset(Pos{e.Cur.Line, i})
	ch := text[o]
	if cl, ok := bracketPairs[ch]; ok {
		for j, d := o+1, 0; j < len(text); j++ {
			switch text[j] {
			case ch:
				d++
			case cl:
				if d == 0 {
					return e.Buf.posAt(j), true
				}
				d--
			}
		}
		return e.Cur, false
	}
	var op byte
	for k, v := range bracketPairs {
		if v == ch {
			op = k
		}
	}
	for j, d := o-1, 0; j >= 0; j-- {
		switch text[j] {
		case ch:
			d++
		case op:
			if d == 0 {
				return e.Buf.posAt(j), true
			}
			d--
		}
	}
	return e.Cur, false
}

// ---------- text objects ----------

// objects maps the char after i/a to its delimiter pair (or a marker).
var objects = map[string][2]byte{
	"w": {'w'}, "W": {'W'}, "p": {'p'},
	`"`: {'"', '"'}, "'": {'\'', '\''}, "`": {'`', '`'},
	"(": {'(', ')'}, ")": {'(', ')'}, "b": {'(', ')'},
	"[": {'[', ']'}, "]": {'[', ']'},
	"{": {'{', '}'}, "}": {'{', '}'}, "B": {'{', '}'},
	"<": {'<', '>'}, ">": {'<', '>'},
	"*": {'*', '*'}, "_": {'_', '_'}, // markdown emphasis, same rules as quotes
}

// object returns [a, z) for a text object, or a line span if linewise.
func (e *Engine) object(name string) (a, z Pos, linewise, ok bool) {
	inner := name[0] == 'i'
	d := objects[name[1:]]
	ln, l, c := e.Cur.Line, e.line(e.Cur.Line), e.Cur.Col
	switch {
	case d[0] == 'w' || d[0] == 'W':
		if l == "" {
			return
		}
		big := d[0] == 'W'
		cl := class(runeAt(l, c), big)
		s, t := c, nextG(l, c)
		for s > 0 && class(runeAt(l, prevG(l, s)), big) == cl {
			s = prevG(l, s)
		}
		for t < len(l) && class(runeAt(l, t), big) == cl {
			t = nextG(l, t)
		}
		if !inner {
			if cl == clsBlank {
				if t < len(l) {
					c2 := class(runeAt(l, t), big)
					for t < len(l) && class(runeAt(l, t), big) == c2 {
						t = nextG(l, t)
					}
				}
			} else {
				t2 := t
				for t2 < len(l) && class(runeAt(l, t2), big) == clsBlank {
					t2 = nextG(l, t2)
				}
				if t2 > t {
					t = t2
				} else {
					for s > 0 && class(runeAt(l, prevG(l, s)), big) == clsBlank {
						s = prevG(l, s)
					}
				}
			}
		}
		return Pos{ln, s}, Pos{ln, t}, false, true
	case d[0] == 'p':
		blank := isBlankLine(l)
		s, t, last := ln, ln, e.Buf.LineCount()-1
		for s > 0 && isBlankLine(e.line(s-1)) == blank {
			s--
		}
		for t < last && isBlankLine(e.line(t+1)) == blank {
			t++
		}
		if !inner && !blank {
			for t < last && isBlankLine(e.line(t+1)) {
				t++
			}
		}
		return Pos{s, 0}, Pos{t, len(e.line(t))}, true, true
	case d[0] == d[1]: // quote-like, single line
		var qs []int
		for i := 0; i < len(l); i++ {
			if l[i] == d[0] && (i == 0 || l[i-1] != '\\') {
				if d[0] == '*' && i+1 < len(l) && l[i+1] == '*' { // treat ** as one delimiter
					qs = append(qs, i)
					i++
					continue
				}
				qs = append(qs, i)
			}
		}
		dl := 1
		if d[0] == '*' && len(qs) > 0 && strings.HasPrefix(l[qs[0]:], "**") {
			dl = 2
		}
		for i := 0; i+1 < len(qs); i += 2 {
			if c <= qs[i+1]+dl-1 && (c >= qs[i] || i == 0 || c > qs[i-1]) {
				s, t := qs[i], qs[i+1]+dl
				if inner {
					s, t = s+dl, t-dl
				} else {
					for t < len(l) && (l[t] == ' ' || l[t] == '\t') {
						t++
					}
				}
				return Pos{ln, s}, Pos{ln, t}, false, true
			}
		}
		return
	default: // bracket pair, may span lines
		text, o := e.Buf.String(), e.Buf.offset(e.Cur)
		start := -1
		for i, dep := o, 0; i >= 0; i-- {
			if text[i] == d[1] && i != o {
				dep++
			} else if text[i] == d[0] {
				if dep == 0 {
					start = i
					break
				}
				dep--
			}
		}
		if start < 0 {
			return
		}
		end := -1
		for i, dep := start+1, 0; i < len(text); i++ {
			if text[i] == d[0] {
				dep++
			} else if text[i] == d[1] {
				if dep == 0 {
					end = i
					break
				}
				dep--
			}
		}
		if end < 0 {
			return
		}
		if inner {
			return e.Buf.posAt(start + 1), e.Buf.posAt(end), false, true
		}
		return e.Buf.posAt(start), e.Buf.posAt(end + 1), false, true
	}
}

// ---------- operators ----------

func (e *Engine) applyOp(p parsed) {
	from := e.Cur
	var a, z Pos
	var lw bool
	switch {
	case p.name == "/" || p.name == "?":
		// d/foo: finish the motion when the search line is entered (cmdKey)
		e.opPending = &p
		e.opFrom = from
		e.Mode, e.cmdline, e.searchBack = Search, "", p.name == "?"
		return
	case p.name == "line":
		last := min(from.Line+max(p.count, 1)-1, e.Buf.LineCount()-1)
		a, z, lw = Pos{from.Line, 0}, Pos{last, len(e.line(last))}, true
	case len(p.name) == 2 && (p.name[0] == 'i' || p.name[0] == 'a') && motions[p.name] == nil:
		var ok bool
		if a, z, lw, ok = e.object(p.name); !ok {
			return
		}
	default:
		name := p.name
		if p.op == "c" && (name == "w" || name == "W") && e.cls(from, name == "W") != clsBlank {
			name = map[string]string{"w": "e", "W": "E"}[name]
			// cw on the last char of a word changes just that char
			l := e.line(from.Line)
			if nx := nextG(l, from.Col); max(p.count, 1) == 1 && (nx >= len(l) || class(runeAt(l, nx), name == "E") != e.cls(from, name == "E")) {
				a, z = from, Pos{from.Line, nx}
				break
			}
		}
		to, k, ok := motions[name](e, p.count, p.arg)
		if !ok {
			return
		}
		if (name == "w" || name == "W") && to.Line > from.Line {
			// last word on a line: stop at end of that line, not the next word
			to, k = Pos{to.Line - 1, len(e.line(to.Line - 1))}, excl
			if to.Less(from) {
				to = from
			}
		}
		a, z, lw = e.motionRange(from, to, k)
	}
	e.operate(p.op, p.reg, a, z, lw)
}

func (e *Engine) motionRange(from, to Pos, k kind) (a, z Pos, linewise bool) {
	a, z = order(from, to)
	switch k {
	case lines:
		return Pos{a.Line, 0}, Pos{z.Line, len(e.line(z.Line))}, true
	case incl:
		z.Col = nextG(e.line(z.Line), z.Col)
	case excl:
		if z.Line > a.Line && z.Col == 0 {
			z = Pos{z.Line - 1, len(e.line(z.Line - 1))}
			if a.Col <= firstNonBlank(e.line(a.Line)) {
				return Pos{a.Line, 0}, z, true
			}
		}
	}
	return a, z, false
}

func (e *Engine) operate(op string, reg rune, a, z Pos, linewise bool) {
	var text string
	if linewise {
		a.Col, z.Col = 0, len(e.line(z.Line))
		text = e.Buf.slice(a, z) + "\n"
	} else {
		text = e.Buf.slice(a, z)
	}
	r := register{text, linewise}
	e.regs['"'] = r
	if reg != 0 {
		e.regs[reg] = r
	}
	switch op {
	case "y":
		e.regs['0'] = r
		if linewise {
			if e.Cur.Line != a.Line {
				e.Cur = Pos{a.Line, firstNonBlank(e.line(a.Line))}
			}
		} else {
			e.Cur = a
		}
		e.Msg = "yanked " + strconv.Itoa(strings.Count(text, "\n")+1) + " lines"
	case "d":
		if linewise {
			e.deleteLines(a.Line, z.Line)
			l := min(a.Line, e.Buf.LineCount()-1)
			e.Cur = Pos{l, firstNonBlank(e.line(l))}
		} else {
			e.del(a, z)
			e.Cur = a
		}
	case "c":
		if linewise {
			ind := firstNonBlank(e.line(a.Line))
			e.del(Pos{a.Line, ind}, z)
			e.Cur = Pos{a.Line, ind}
		} else {
			e.del(a, z)
			e.Cur = a
		}
		e.startInsert()
	}
}

func (e *Engine) deleteLines(a, z int) {
	switch {
	case z+1 < e.Buf.LineCount():
		e.del(Pos{a, 0}, Pos{z + 1, 0})
	case a > 0:
		e.del(Pos{a - 1, len(e.line(a - 1))}, Pos{z, len(e.line(z))})
	default:
		e.del(Pos{0, 0}, Pos{z, len(e.line(z))})
	}
}

// ---------- commands ----------

var changeCmds = map[string]bool{
	"x": true, "X": true, "s": true, "S": true, "D": true, "C": true, "r": true, "J": true,
	"p": true, "P": true, "~": true, "i": true, "a": true, "I": true, "A": true, "o": true, "O": true,
	"delete": true,
}

func op(o, name string) func(e *Engine, p parsed) {
	return func(e *Engine, p parsed) {
		e.applyOp(parsed{reg: p.reg, count: p.count, op: o, name: name})
	}
}

var normalCmds, visualCmds map[string]func(e *Engine, p parsed)

func init() {
	normalCmds = map[string]func(e *Engine, p parsed){
		"i": func(e *Engine, _ parsed) { e.startInsert() },
		"a": func(e *Engine, _ parsed) {
			e.Cur.Col = nextG(e.line(e.Cur.Line), e.Cur.Col)
			e.startInsert()
		},
		"I": func(e *Engine, _ parsed) { e.Cur.Col = firstNonBlank(e.line(e.Cur.Line)); e.startInsert() },
		"A": func(e *Engine, _ parsed) { e.Cur.Col = len(e.line(e.Cur.Line)); e.startInsert() },
		"o": func(e *Engine, _ parsed) {
			l := e.line(e.Cur.Line)
			e.Cur = e.ins(Pos{e.Cur.Line, len(l)}, "\n"+e.continuation(l))
			e.startInsert()
		},
		"O": func(e *Engine, _ parsed) {
			l := e.line(e.Cur.Line)
			ind := l[:firstNonBlank(l)]
			e.ins(Pos{e.Cur.Line, 0}, ind+"\n")
			e.Cur.Col = len(ind)
			e.startInsert()
		},
		"x": op("d", "l"), "delete": op("d", "l"), "X": op("d", "h"),
		"s": op("c", "l"), "S": op("c", "line"),
		"D": op("d", "$"), "C": op("c", "$"), "Y": op("y", "line"),
		"r": func(e *Engine, p parsed) {
			l, z := e.line(e.Cur.Line), e.Cur.Col
			for i := 0; i < max(p.count, 1); i++ {
				if z >= len(l) {
					return
				}
				z = nextG(l, z)
			}
			e.del(e.Cur, Pos{e.Cur.Line, z})
			e.ins(e.Cur, strings.Repeat(p.arg, max(p.count, 1)))
			e.Cur.Col += len(p.arg) * (max(p.count, 1) - 1)
		},
		"J":      func(e *Engine, p parsed) { e.join(e.Cur.Line, max(p.count, 2)) },
		"p":      func(e *Engine, p parsed) { e.paste(p, true) },
		"P":      func(e *Engine, p parsed) { e.paste(p, false) },
		"u":      func(e *Engine, _ parsed) { e.undoStep() },
		"ctrl+r": func(e *Engine, _ parsed) { e.redoStep() },
		".":      func(e *Engine, p parsed) { e.repeatDot(p.count) },
		"~": func(e *Engine, p parsed) {
			l, z := e.line(e.Cur.Line), e.Cur.Col
			for i := 0; i < max(p.count, 1) && z < len(l); i++ {
				z = nextG(l, z)
			}
			e.toggleCase(e.Cur, Pos{e.Cur.Line, z})
			e.Cur.Col = z
		},
		"v":   func(e *Engine, _ parsed) { e.Mode, e.anchor = Visual, e.Cur },
		"V":   func(e *Engine, _ parsed) { e.Mode, e.anchor = VisualLine, e.Cur },
		":":   func(e *Engine, _ parsed) { e.Mode, e.cmdline = Command, "" },
		"/":   func(e *Engine, _ parsed) { e.Mode, e.cmdline, e.searchBack = Search, "", false },
		"?":   func(e *Engine, _ parsed) { e.Mode, e.cmdline, e.searchBack = Search, "", true },
		"esc": func(e *Engine, _ parsed) {},
	}
	vop := func(o string) func(e *Engine, p parsed) {
		return func(e *Engine, p parsed) {
			a, z, lw, _ := e.Selection()
			e.Mode = Normal
			e.operate(o, p.reg, a, z, lw)
		}
	}
	visualCmds = map[string]func(e *Engine, p parsed){
		"d": vop("d"), "x": vop("d"), "delete": vop("d"),
		"c": vop("c"), "s": vop("c"), "y": vop("y"),
		"o":   func(e *Engine, _ parsed) { e.anchor, e.Cur = e.Cur, e.anchor },
		"esc": func(e *Engine, _ parsed) { e.Mode = Normal },
		"v": func(e *Engine, _ parsed) {
			if e.Mode == Visual {
				e.Mode = Normal
			} else {
				e.Mode = Visual
			}
		},
		"V": func(e *Engine, _ parsed) {
			if e.Mode == VisualLine {
				e.Mode = Normal
			} else {
				e.Mode = VisualLine
			}
		},
		"J": func(e *Engine, _ parsed) {
			a, z, _, _ := e.Selection()
			e.Mode = Normal
			e.join(a.Line, max(z.Line-a.Line+1, 2))
		},
		"~": func(e *Engine, _ parsed) {
			a, z, lw, _ := e.Selection()
			if lw {
				a.Col, z.Col = 0, len(e.line(z.Line))
			}
			e.Mode = Normal
			e.toggleCase(a, z)
			e.Cur = a
		},
	}
}

// continuation is the prefix for a new line after l: indent, plus the list
// bullet / task checkbox when listcont is on.
func (e *Engine) continuation(l string) string {
	if e.Opts["listcont"] {
		if m := listRe.FindStringSubmatch(l); m != nil {
			bullet := m[2]
			if n, err := strconv.Atoi(strings.TrimRight(bullet, ".)")); err == nil {
				bullet = strconv.Itoa(n+1) + bullet[len(bullet)-1:]
			}
			s := m[1] + bullet + m[3]
			if m[4] != "" {
				s += "[ ] "
			}
			return s
		}
	}
	return l[:firstNonBlank(l)]
}

func (e *Engine) join(line, n int) {
	for i := 1; i < n && line+1 < e.Buf.LineCount(); i++ {
		a, b := e.line(line), e.line(line+1)
		bt := strings.TrimLeft(b, " \t")
		sep := " "
		if a == "" || strings.HasSuffix(a, " ") || bt == "" || strings.HasPrefix(bt, ")") {
			sep = ""
		}
		e.del(Pos{line, len(a)}, Pos{line + 1, len(b) - len(bt)})
		e.ins(Pos{line, len(a)}, sep)
		e.Cur = Pos{line, len(a)}
	}
}

func (e *Engine) paste(p parsed, after bool) {
	reg := p.reg
	if reg == 0 {
		reg = '"'
	}
	r, ok := e.regs[reg]
	if !ok {
		e.Msg = "E353: Nothing in register " + string(reg)
		return
	}
	text := strings.Repeat(r.text, max(p.count, 1))
	if r.linewise {
		t := e.Cur.Line
		switch {
		case !after:
			e.ins(Pos{t, 0}, text)
		case t+1 < e.Buf.LineCount():
			t++
			e.ins(Pos{t, 0}, text)
		default:
			e.ins(Pos{t, len(e.line(t))}, "\n"+strings.TrimSuffix(text, "\n"))
			t++
		}
		e.Cur = Pos{t, firstNonBlank(e.line(t))}
		return
	}
	at := e.Cur
	if after && e.line(at.Line) != "" {
		at.Col = nextG(e.line(at.Line), at.Col)
	}
	end := e.ins(at, text)
	e.Cur = Pos{end.Line, prevG(e.line(end.Line), end.Col)}
}

func (e *Engine) toggleCase(a, z Pos) {
	t := e.Buf.slice(a, z)
	out := []rune(t)
	for i, r := range out {
		if unicode.IsUpper(r) {
			out[i] = unicode.ToLower(r)
		} else {
			out[i] = unicode.ToUpper(r)
		}
	}
	e.del(a, z)
	e.ins(a, string(out))
}

func (e *Engine) repeatDot(count int) {
	if e.dot == nil {
		return
	}
	if count == 0 {
		count = e.dotCount
	}
	var keys []string
	if count > 0 {
		for _, r := range strconv.Itoa(count) {
			keys = append(keys, string(r))
		}
	}
	keys = append(keys, e.dot...)
	e.replaying = true
	for _, k := range keys {
		e.Feed(k)
	}
	if e.Mode == Insert { // dot recorded mid-insert (shouldn't happen) — leave insert
		e.Feed("esc")
	}
	e.replaying = false
}
