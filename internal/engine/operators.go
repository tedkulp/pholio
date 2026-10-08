package engine

import (
	"strconv"
	"strings"
	"unicode"
)

// register holds yanked or deleted text.
type register struct {
	text     string
	linewise bool
}

func (e *Engine) applyOp(p parsed) {
	from := e.Cur
	var a, z Pos
	var lw bool
	switch {
	case p.name == "line":
		last := min(from.Line+max(p.count, 1)-1, e.Buf.LineCount()-1)
		a, z, lw = Pos{from.Line, 0}, Pos{last, len(e.line(last))}, true
	case isObject(p.name):
		var ok bool
		if a, z, lw, ok = e.object(p.name); !ok {
			return
		}
	case p.op == "c" && (p.name == "w" || p.name == "W") && e.cls(from, p.name == "W") != clsBlank:
		// cw is ce, except that it stays put on the last char of a word.
		big := p.name == "W"
		to := e.wordEndStop(from, big)
		for i := 1; i < max(p.count, 1); i++ {
			to = e.wordEnd(to, big)
		}
		a, z, lw = e.motionRange(from, to, incl)
	default:
		to, k, ok := motions[p.name](e, p.count, p.arg)
		if !ok {
			return
		}
		if (p.name == "w" || p.name == "W") && to.Line > from.Line {
			// The last word on a line: stop at its end, not at the next word.
			to, k = Pos{to.Line - 1, len(e.line(to.Line - 1))}, excl
			if to.Less(from) {
				to = from
			}
		}
		a, z, lw = e.motionRange(from, to, k)
	}
	if p.op == ">" || p.op == "<" {
		e.shift(a.Line, z.Line, p.op == ">")
		return
	}
	e.operate(p.op, p.reg, a, z, lw)
}

// shift indents (>) or outdents (<) lines a..z by one tab, vim style: >
// skips blank lines, and < takes one leading tab, or else up to TabStop
// leading spaces. The cursor goes to the first non-blank of line a.
func (e *Engine) shift(a, z int, right bool) {
	for i := a; i <= z; i++ {
		l := e.line(i)
		switch {
		case right && !isBlankLine(l):
			e.ins(Pos{i, 0}, "\t")
		case !right && strings.HasPrefix(l, "\t"):
			e.del(Pos{i, 0}, Pos{i, 1})
		case !right:
			n := len(l) - len(strings.TrimLeft(l, " "))
			e.del(Pos{i, 0}, Pos{i, min(n, TabStop)})
		}
	}
	e.Cur = Pos{a, firstNonBlank(e.line(a))}
}

// wordEndStop is the target of a single e, except that it does not move when
// p is already on the last char of a word.
func (e *Engine) wordEndStop(p Pos, big bool) Pos {
	if q, ok := e.next(p); !ok || q.Line != p.Line || e.cls(q, big) != e.cls(p, big) {
		return p
	}
	return e.wordEnd(p, big)
}

// motionRange turns a motion from..to into the span an operator acts on.
func (e *Engine) motionRange(from, to Pos, k kind) (a, z Pos, linewise bool) {
	a, z = order(from, to)
	switch k {
	case lines:
		return Pos{a.Line, 0}, Pos{z.Line, len(e.line(z.Line))}, true
	case incl:
		z.Col = nextG(e.line(z.Line), z.Col)
	case excl:
		// An exclusive motion that ends at column 0 stops at the end of the
		// line before, and becomes linewise if it started at the indent.
		if z.Line > a.Line && z.Col == 0 {
			z = Pos{z.Line - 1, len(e.line(z.Line - 1))}
			if a.Col <= firstNonBlank(e.line(a.Line)) {
				return Pos{a.Line, 0}, z, true
			}
		}
	}
	return a, z, false
}

// operate applies d, c or y to [a, z), or to lines a.Line..z.Line.
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
		if n := strings.Count(text, "\n"); linewise && n > 2 {
			e.Msg = strconv.Itoa(n) + " lines yanked"
		}
		if !linewise {
			e.Cur = a
		} else if e.Cur.Line != a.Line {
			e.Cur = Pos{a.Line, firstNonBlank(e.line(a.Line))}
		}
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
			a.Col = firstNonBlank(e.line(a.Line))
		}
		e.del(a, z)
		e.Cur = a
		e.startInsert()
	}
}

// deleteLines removes lines a..z, including their line breaks.
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

// paste puts register p.reg after (p) or before (P) the cursor.
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
	if after {
		at.Col = nextG(e.line(at.Line), at.Col)
	}
	end := e.ins(at, text)
	if strings.Contains(text, "\n") {
		e.Cur = at
	} else {
		e.Cur = Pos{end.Line, prevG(e.line(end.Line), end.Col)}
	}
}

// join joins n lines starting at line, vim style.
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

func (e *Engine) toggleCase(a, z Pos) {
	out := []rune(e.Buf.slice(a, z))
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
