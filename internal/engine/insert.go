package engine

import (
	"regexp"
	"strconv"
	"strings"
)

// listRe matches a list item prefix: indent, bullet or number, spacing, and
// an optional Task checkbox.
var listRe = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX/-]\](?:\s+|$))?`)

func (e *Engine) startInsert() {
	e.Mode = Insert
	e.Cur.Col = min(e.Cur.Col, len(e.line(e.Cur.Line)))
}

// continuation is the prefix for a new line after l: its indent, plus the
// next bullet or number and an empty checkbox when l is a list item.
func (e *Engine) continuation(l string) string {
	if e.ListContinuation {
		if m := listRe.FindStringSubmatch(l); m != nil {
			return nextItem(m)
		}
	}
	return l[:firstNonBlank(l)]
}

// nextItem builds the prefix of the item after the one listRe matched.
func nextItem(m []string) string {
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

func (e *Engine) insertKey(k string) {
	line := e.line(e.Cur.Line)
	switch k {
	case "esc", "ctrl+c", "ctrl+[":
		e.Mode = Normal
		e.recording = false
		e.Cur.Col = prevG(line, e.Cur.Col)
	case "enter":
		e.insertEnter(line)
	case "backspace", "ctrl+h":
		if p, ok := e.prev(e.Cur); ok {
			e.del(p, e.Cur)
			e.Cur = p
		}
	case "delete":
		if p, ok := e.next(e.Cur); ok {
			e.del(e.Cur, p)
		}
	case "ctrl+w":
		p := e.Cur
		back := func() int { return class(runeAt(line, prevG(line, p.Col)), false) }
		for p.Col > 0 && back() == clsBlank {
			p.Col = prevG(line, p.Col)
		}
		if p.Col > 0 {
			c := back()
			for p.Col > 0 && back() == c {
				p.Col = prevG(line, p.Col)
			}
		}
		e.del(p, e.Cur)
		e.Cur = p
	case "ctrl+u":
		p := Pos{e.Cur.Line, firstNonBlank(line)}
		if p.Col >= e.Cur.Col {
			p.Col = 0
		}
		e.del(p, e.Cur)
		e.Cur = p
	case "tab":
		e.Cur = e.ins(e.Cur, "\t")
	case "left":
		e.Cur.Col = prevG(line, e.Cur.Col)
	case "right":
		e.Cur.Col = nextG(line, e.Cur.Col)
	case "up", "down":
		d := 1
		if k == "up" {
			d = -1
		}
		l := max(0, min(e.Cur.Line+d, e.Buf.LineCount()-1))
		e.Cur = Pos{l, colAtCells(e.line(l), Cells(line, e.Cur.Col))}
	default:
		if isText(k) {
			e.Cur = e.ins(e.Cur, k)
		}
	}
}

// insertEnter splits the line, continuing a list item. Enter on an empty
// item ends the list instead.
func (e *Engine) insertEnter(line string) {
	prefix := line[:firstNonBlank(line)]
	if e.ListContinuation {
		if m := listRe.FindStringSubmatch(line); m != nil && e.Cur.Col >= len(m[0]) {
			if isBlankLine(line[len(m[0]):]) {
				e.del(Pos{e.Cur.Line, 0}, Pos{e.Cur.Line, len(line)})
				e.Cur.Col = 0
				return
			}
			prefix = nextItem(m)
		}
	}
	e.Cur = e.ins(e.Cur, "\n"+prefix)
}

// Paste inserts bracketed-paste text at the cursor. It only works in insert
// mode.
func (e *Engine) Paste(s string) {
	if e.Mode == Insert {
		e.Cur = e.ins(e.Cur, strings.ReplaceAll(s, "\r\n", "\n"))
	}
}
