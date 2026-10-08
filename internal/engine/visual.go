package engine

// Selection is the visual selection: [a, z) charwise, or lines a.Line to
// z.Line when linewise. ok is false outside visual mode.
func (e *Engine) Selection() (a, z Pos, linewise, ok bool) {
	if !e.Mode.visual() {
		return
	}
	a, z, linewise = e.selection()
	return a, z, linewise, true
}

func (e *Engine) selection() (a, z Pos, linewise bool) {
	a, z = order(e.anchor, e.Cur)
	z.Col = nextG(e.line(z.Line), z.Col)
	return a, z, e.Mode == VisualLine
}

// selectObject makes a text object the selection, as viw does.
func (e *Engine) selectObject(name string) {
	a, z, lw, ok := e.object(name)
	if !ok {
		return
	}
	e.anchor = a
	e.Cur = Pos{z.Line, prevG(e.line(z.Line), z.Col)}
	if lw {
		e.Mode = VisualLine
	}
}

// toggleVisual switches to mode m, or back to normal when already in it.
func (e *Engine) toggleVisual(m Mode) {
	if e.Mode == m {
		e.Mode = Normal
	} else {
		e.Mode = m
	}
}

// visualOp applies an operator to the selection.
func visualOp(o string) func(e *Engine, p parsed) {
	return func(e *Engine, p parsed) {
		a, z, lw, _ := e.Selection()
		e.Mode = Normal
		e.cmdStart = a
		e.operate(o, p.reg, a, z, lw)
	}
}

// visualShift indents (>) or outdents (<) every line the selection touches.
func visualShift(right bool) func(e *Engine, p parsed) {
	return func(e *Engine, _ parsed) {
		a, z, _, _ := e.Selection()
		e.Mode = Normal
		e.cmdStart = a
		e.shift(a.Line, z.Line, right)
	}
}

var visualCmds map[string]func(e *Engine, p parsed)

// visualChanges are the visual commands that "." repeats.
var visualChanges = map[string]bool{
	"d": true, "x": true, "delete": true, "c": true, "s": true,
	">": true, "<": true, "~": true, "J": true,
}

func init() {
	visualCmds = map[string]func(e *Engine, p parsed){
		"d": visualOp("d"), "x": visualOp("d"), "delete": visualOp("d"),
		"c": visualOp("c"), "s": visualOp("c"), "y": visualOp("y"),
		">": visualShift(true), "<": visualShift(false),
		"o":   func(e *Engine, _ parsed) { e.anchor, e.Cur = e.Cur, e.anchor },
		"esc": func(e *Engine, _ parsed) { e.Mode = Normal },
		"v":   func(e *Engine, _ parsed) { e.toggleVisual(Visual) },
		"V":   func(e *Engine, _ parsed) { e.toggleVisual(VisualLine) },
		"/":   func(e *Engine, _ parsed) { e.startCmdline(Search, false) },
		"?":   func(e *Engine, _ parsed) { e.startCmdline(Search, true) },
		"J": func(e *Engine, _ parsed) {
			a, z, _, _ := e.Selection()
			e.Mode = Normal
			e.cmdStart = a
			e.join(a.Line, max(z.Line-a.Line+1, 2))
		},
		"~": func(e *Engine, _ parsed) {
			a, z, lw, _ := e.Selection()
			if lw {
				a.Col, z.Col = 0, len(e.line(z.Line))
			}
			e.Mode = Normal
			e.cmdStart = a
			e.toggleCase(a, z)
			e.Cur = a
		},
	}
}
