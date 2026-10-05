package engine

// kind says how an operator treats the span up to a motion's target.
type kind int

const (
	excl  kind = iota // the target character is not included
	incl              // the target character is included
	lines             // whole lines
)

// motionFn returns the target for count n (0 = none given) and the literal
// arg (for f t F T). ok is false when the motion cannot move.
type motionFn func(e *Engine, n int, arg string) (to Pos, k kind, ok bool)

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
	bol := func(e *Engine, _ int, _ string) (Pos, kind, bool) { return Pos{e.Cur.Line, 0}, excl, true }
	eol := func(e *Engine, n int, _ string) (Pos, kind, bool) {
		t := min(e.Cur.Line+max(n, 1)-1, e.Buf.LineCount()-1)
		return Pos{t, lastG(e.line(t))}, incl, true
	}
	motions = map[string]motionFn{
		"h": h, "left": h, "backspace": h,
		"l": l, "right": l, " ": l,
		"j": vert(1), "down": vert(1),
		"k": vert(-1), "up": vert(-1),
		"0": bol, "home": bol,
		"^": func(e *Engine, _ int, _ string) (Pos, kind, bool) {
			return Pos{e.Cur.Line, firstNonBlank(e.line(e.Cur.Line))}, excl, true
		},
		"$": eol, "end": eol,
		"enter": func(e *Engine, n int, _ string) (Pos, kind, bool) {
			t := min(e.Cur.Line+max(n, 1), e.Buf.LineCount()-1)
			return Pos{t, firstNonBlank(e.line(t))}, lines, true
		},
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
		"w":  word((*Engine).wordFwd, false, excl),
		"W":  word((*Engine).wordFwd, true, excl),
		"b":  word((*Engine).wordBack, false, excl),
		"B":  word((*Engine).wordBack, true, excl),
		"e":  word((*Engine).wordEnd, false, incl),
		"E":  word((*Engine).wordEnd, true, incl),
		"ge": word((*Engine).wordEndBack, false, incl),
		"gE": word((*Engine).wordEndBack, true, incl),
		"f":  find("f"), "F": find("F"), "t": find("t"), "T": find("T"),
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
		"ctrl+d": page(1, 2), "ctrl+u": page(-1, 2),
		"ctrl+f": page(1, 1), "ctrl+b": page(-1, 1),
		"pgdown": page(1, 1), "pgup": page(-1, 1),
		"n": searchMotion(false), "N": searchMotion(true),
	}
}

// keepWant motions move vertically and keep the remembered column.
var keepWant = map[string]bool{"j": true, "k": true, "up": true, "down": true}

// atWant is the position on line that is closest to the remembered column.
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
	if p.name == "$" || p.name == "end" {
		e.want = -1
		e.keptWant = true
	}
	e.keptWant = e.keptWant || keepWant[p.name]
}
