package engine

import "strings"

// objects maps the key after i/a to its delimiter pair. w, W and p are
// markers rather than delimiters. * and _ are markdown emphasis and follow
// the quote rules; * treats ** as one delimiter.
var objects = map[string][2]byte{
	"w": {'w'}, "W": {'W'}, "p": {'p'},
	`"`: {'"', '"'}, "'": {'\'', '\''}, "`": {'`', '`'},
	"(": {'(', ')'}, ")": {'(', ')'}, "b": {'(', ')'},
	"[": {'[', ']'}, "]": {'[', ']'},
	"{": {'{', '}'}, "}": {'{', '}'}, "B": {'{', '}'},
	"<": {'<', '>'}, ">": {'<', '>'},
	"*": {'*', '*'}, "_": {'_', '_'},
}

// object returns the span [a, z) of a text object such as "iw", or a line
// span when linewise. ok is false when there is no such object here.
func (e *Engine) object(name string) (a, z Pos, linewise, ok bool) {
	inner := name[0] == 'i'
	d := objects[name[1:]]
	switch d[0] {
	case 'w', 'W':
		return e.wordObject(inner, d[0] == 'W')
	case 'p':
		a, z = e.paraObject(inner)
		return a, z, true, true
	}
	if d[0] == d[1] {
		return e.quoteObject(inner, d[0])
	}
	return e.bracketObject(inner, d[0], d[1])
}

func (e *Engine) wordObject(inner, big bool) (a, z Pos, linewise, ok bool) {
	ln, l, c := e.Cur.Line, e.line(e.Cur.Line), e.Cur.Col
	if l == "" {
		// An empty line is an empty word: c then just enters insert mode.
		return Pos{ln, 0}, Pos{ln, 0}, false, true
	}
	clsAt := func(i int) int { return class(runeAt(l, i), big) }
	cl := clsAt(c)
	s, t := c, nextG(l, c)
	for s > 0 && clsAt(prevG(l, s)) == cl {
		s = prevG(l, s)
	}
	for t < len(l) && clsAt(t) == cl {
		t = nextG(l, t)
	}
	if !inner {
		switch cl {
		case clsBlank:
			// aw on blanks takes the blanks and the word after them.
			if t < len(l) {
				c2 := clsAt(t)
				for t < len(l) && clsAt(t) == c2 {
					t = nextG(l, t)
				}
			}
		default:
			// aw takes trailing blanks, or leading ones when there are none.
			t2 := t
			for t2 < len(l) && clsAt(t2) == clsBlank {
				t2 = nextG(l, t2)
			}
			if t2 > t {
				t = t2
			} else {
				for s > 0 && clsAt(prevG(l, s)) == clsBlank {
					s = prevG(l, s)
				}
			}
		}
	}
	return Pos{ln, s}, Pos{ln, t}, false, true
}

func (e *Engine) paraObject(inner bool) (a, z Pos) {
	ln := e.Cur.Line
	blank := isBlankLine(e.line(ln))
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
	return Pos{s, 0}, Pos{t, len(e.line(t))}
}

// quoteObject finds the quote-like pair around (or after) the cursor on the
// cursor line. Backslash-escaped delimiters are skipped.
func (e *Engine) quoteObject(inner bool, q byte) (a, z Pos, linewise, ok bool) {
	ln, l, c := e.Cur.Line, e.line(e.Cur.Line), e.Cur.Col
	var qs []int
	for i := 0; i < len(l); i++ {
		if l[i] != q || i > 0 && l[i-1] == '\\' {
			continue
		}
		qs = append(qs, i)
		if q == '*' && i+1 < len(l) && l[i+1] == '*' {
			i++ // ** is one delimiter
		}
	}
	dl := 1
	if q == '*' && len(qs) > 0 && strings.HasPrefix(l[qs[0]:], "**") {
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
}

// bracketObject finds the open/cl pair around the cursor. It may span lines.
func (e *Engine) bracketObject(inner bool, open, cl byte) (a, z Pos, linewise, ok bool) {
	text, o := e.Buf.String(), e.Buf.offset(e.Cur)
	var start int
	switch text[o] {
	case open:
		start = o
	case cl:
		start = scanPair(text, o-1, -1, cl, open)
	default:
		start = scanPair(text, o, -1, cl, open)
	}
	if start < 0 {
		return
	}
	end := scanPair(text, start+1, 1, open, cl)
	if end < 0 {
		return
	}
	if inner {
		return e.Buf.posAt(start + 1), e.Buf.posAt(end), false, true
	}
	return e.Buf.posAt(start), e.Buf.posAt(end + 1), false, true
}
