package engine

import "strings"

// changeCmds are the commands that "." repeats.
var changeCmds = map[string]bool{
	"x": true, "X": true, "s": true, "S": true, "D": true, "C": true, "r": true, "J": true,
	"p": true, "P": true, "~": true, "i": true, "a": true, "I": true, "A": true, "o": true, "O": true,
	"delete": true,
}

// op is a command that is shorthand for an operator and motion, such as x
// for dl.
func op(o, name string) func(e *Engine, p parsed) {
	return func(e *Engine, p parsed) {
		e.applyOp(parsed{reg: p.reg, count: p.count, op: o, name: name})
	}
}

var normalCmds map[string]func(e *Engine, p parsed)

func init() {
	normalCmds = map[string]func(e *Engine, p parsed){
		"i": func(e *Engine, _ parsed) { e.startInsert() },
		"a": func(e *Engine, _ parsed) {
			e.Cur.Col = nextG(e.line(e.Cur.Line), e.Cur.Col)
			e.startInsert()
		},
		"I": func(e *Engine, _ parsed) {
			e.Cur.Col = firstNonBlank(e.line(e.Cur.Line))
			e.startInsert()
		},
		"A": func(e *Engine, _ parsed) {
			e.Cur.Col = len(e.line(e.Cur.Line))
			e.startInsert()
		},
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
			n := max(p.count, 1)
			l, z := e.line(e.Cur.Line), e.Cur.Col
			for range n {
				if z >= len(l) {
					return
				}
				z = nextG(l, z)
			}
			e.del(e.Cur, Pos{e.Cur.Line, z})
			e.ins(e.Cur, strings.Repeat(p.arg, n))
			e.Cur.Col += len(p.arg) * (n - 1)
		},
		"J": func(e *Engine, p parsed) { e.join(e.Cur.Line, max(p.count, 2)) },
		"p": func(e *Engine, p parsed) { e.paste(p, true) },
		"P": func(e *Engine, p parsed) { e.paste(p, false) },
		"~": func(e *Engine, p parsed) {
			l, z := e.line(e.Cur.Line), e.Cur.Col
			for i := 0; i < max(p.count, 1) && z < len(l); i++ {
				z = nextG(l, z)
			}
			e.toggleCase(e.Cur, Pos{e.Cur.Line, z})
			e.Cur.Col = z
		},
		"u":      func(e *Engine, _ parsed) { e.undoStep() },
		"ctrl+r": func(e *Engine, _ parsed) { e.redoStep() },
		".":      func(e *Engine, p parsed) { e.repeatDot(p.count) },
		"esc":    func(*Engine, parsed) {},
		"v":      func(e *Engine, _ parsed) { e.Mode, e.anchor = Visual, e.Cur },
		"V":      func(e *Engine, _ parsed) { e.Mode, e.anchor = VisualLine, e.Cur },
		"ctrl+v": func(e *Engine, _ parsed) { e.Mode, e.anchor = VisualBlock, e.Cur },
		":":      func(e *Engine, _ parsed) { e.startCmdline(Command, false) },
		"/":      func(e *Engine, _ parsed) { e.startCmdline(Search, false) },
		"?":      func(e *Engine, _ parsed) { e.startCmdline(Search, true) },
	}
}
