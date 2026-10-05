package engine

import "strings"

// next and prev step one grapheme through the whole buffer. The end of each
// line is a position of its own, standing for the "\n".
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
	for ok && (!e.emptyLine(p) || p == start) && e.cls(p, big) == clsBlank {
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

// find implements f t F T. repeat is set for ; and , so that t/T skip the
// character they stopped next to.
func (e *Engine) find(name, arg string, n int, repeat bool) (Pos, kind, bool) {
	l, c := e.line(e.Cur.Line), e.Cur.Col
	found, cnt := -1, 0
	// A grapheme matches when it starts with arg, so "f👍" finds "👍🏽".
	match := func(i int) bool { return strings.HasPrefix(l[i:nextG(l, i)], arg) }
	if name == "f" || name == "t" {
		start := nextG(l, c)
		if name == "t" && repeat {
			start = nextG(l, start)
		}
		for i := start; i < len(l); i = nextG(l, i) {
			if match(i) {
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
	}
	start := c
	if name == "T" && repeat {
		start = prevG(l, start)
	}
	for i := start; i > 0; {
		i = prevG(l, i)
		if match(i) {
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

// para moves n paragraphs in direction dir, stopping on blank lines.
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

// matchBracket finds the partner of the first bracket at or after the cursor.
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
		if j := scanPair(text, o+1, 1, ch, cl); j >= 0 {
			return e.Buf.posAt(j), true
		}
		return e.Cur, false
	}
	var op byte
	for k, v := range bracketPairs {
		if v == ch {
			op = k
		}
	}
	if j := scanPair(text, o-1, -1, ch, op); j >= 0 {
		return e.Buf.posAt(j), true
	}
	return e.Cur, false
}

// scanPair walks text from i in direction dir and returns the index of the
// `want` that balances, skipping nested `same`/`want` pairs. -1 if none.
func scanPair(text string, i, dir int, same, want byte) int {
	for d := 0; i >= 0 && i < len(text); i += dir {
		switch text[i] {
		case same:
			d++
		case want:
			if d == 0 {
				return i
			}
			d--
		}
	}
	return -1
}
