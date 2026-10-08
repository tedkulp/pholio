package engine

import "strconv"

// setDot remembers p as the change "." repeats. When p entered insert mode,
// the keys typed until esc are appended as they arrive.
func (e *Engine) setDot(p parsed) {
	if e.replaying {
		return
	}
	e.dot = append([]string(nil), p.rest...)
	e.dotCount, e.dotReg, e.dotVis = p.count, p.reg, nil
	e.recording = e.Mode == Insert
}

// visualDot is a change made from visual mode. "." repeats it on a
// selection of the same size that starts at the cursor, since replaying
// the motions that built the selection would not reproduce its size.
type visualDot struct {
	name     string // the visual command, such as "d" or ">"
	linewise bool
	lines    int // lines the selection spans
	size     int // graphemes on a one-line selection, else the cell column it ends at
}

// visualShape measures the selection for a change command run from
// charwise or linewise visual mode. ok is false for anything else.
func (e *Engine) visualShape(name string) (v visualDot, ok bool) {
	if !visualChanges[name] {
		return v, false
	}
	a, z, lw := e.selection()
	v = visualDot{name: name, linewise: lw, lines: z.Line - a.Line + 1}
	l := e.line(z.Line)
	if v.lines == 1 {
		for c := a.Col; c < z.Col; c = nextG(l, c) {
			v.size++
		}
	} else {
		v.size = Cells(l, prevG(l, z.Col))
	}
	return v, true
}

// setVisualDot remembers v, run with register p.reg, as the change "."
// repeats. Keys typed in an insert session it started are appended to dot.
func (e *Engine) setVisualDot(p parsed, v visualDot) {
	if e.replaying {
		return
	}
	e.dot, e.dotCount, e.dotReg, e.dotVis = []string{}, 0, p.reg, &v
	e.recording = e.Mode == Insert
}

// repeatVisual selects the remembered selection's size from the cursor,
// clamped to the buffer and its lines, and runs the visual change again.
func (e *Engine) repeatVisual() {
	v := e.dotVis
	a, z := e.Cur, e.Cur
	z.Line = min(a.Line+v.lines-1, e.Buf.LineCount()-1)
	l := e.line(z.Line)
	switch {
	case v.linewise:
	case v.lines == 1:
		for i := 1; i < v.size && nextG(l, z.Col) < len(l); i++ {
			z.Col = nextG(l, z.Col)
		}
	default:
		z.Col = min(colAtCells(l, v.size), lastG(l))
	}
	e.Mode, e.anchor, e.Cur = Visual, a, z
	if v.linewise {
		e.Mode = VisualLine
	}
	e.replaying = true
	defer func() { e.replaying = false }()
	visualCmds[v.name](e, parsed{reg: e.dotReg})
	for _, k := range e.dot {
		e.Feed(k)
	}
	if e.Mode == Insert {
		e.Feed("esc")
	}
}

// repeatDot replays the last change. A count replaces the original one,
// except on a visual change, where it is ignored.
func (e *Engine) repeatDot(count int) {
	if e.dot == nil {
		return
	}
	if e.dotVis != nil {
		e.repeatVisual()
		return
	}
	if count == 0 {
		count = e.dotCount
	}
	var keys []string
	if e.dotReg != 0 {
		keys = append(keys, `"`, string(e.dotReg))
	}
	if count > 0 {
		for _, r := range strconv.Itoa(count) {
			keys = append(keys, string(r))
		}
	}
	keys = append(keys, e.dot...)
	e.replaying = true
	defer func() { e.replaying = false }()
	for _, k := range keys {
		e.Feed(k)
	}
	if e.Mode == Insert {
		e.Feed("esc")
	}
}
