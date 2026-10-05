package engine

import "slices"

// FixupFunc rewrites lines after a change. before and after are the
// buffer's lines before and after the change; after is the live buffer, so
// read it but don't keep it. inserted is set when the change included an
// insert session. The hook edits through SetLine, and those edits join the
// change's undo step.
type FixupFunc func(e *Engine, before, after []string, inserted bool)

// fixup runs the Fixup hook on the pending change, just before it is
// committed: after each normal command that changed the buffer, and on
// leaving insert mode.
func (e *Engine) fixup() {
	if e.Fixup == nil || e.pending == nil || e.fixing {
		return
	}
	old := &Buffer{lines: slices.Clone(e.Buf.lines)}
	for i := len(e.pending.edits) - 1; i >= 0; i-- {
		ed := e.pending.edits[i]
		if ed.ins != "" {
			old.delete(ed.at, endOf(ed.at, ed.ins))
		}
		if ed.del != "" {
			old.insert(ed.at, ed.del)
		}
	}
	e.fixing = true
	defer func() { e.fixing = false }()
	e.Fixup(e, old.lines, e.Buf.lines, e.inserted)
}

// SetLine replaces line i with text. Inside a command or a Fixup hook it
// joins that change's undo step; called from outside (the Task List
// toggling a Task in the open Note) it is an undo step of its own. The
// cursor stays on its line, clamped to the new text. An i outside the
// buffer is ignored.
func (e *Engine) SetLine(i int, text string) {
	if i < 0 || i >= e.Buf.LineCount() || e.line(i) == text {
		return
	}
	alone := e.depth == 0 && !e.fixing && e.Mode == Normal
	if alone {
		e.commit()
		e.cmdStart = e.Cur
	}
	e.del(Pos{i, 0}, Pos{i, len(e.line(i))})
	e.ins(Pos{i, 0}, text)
	if e.Cur.Line == i {
		e.Cur.Col = min(e.Cur.Col, len(text))
	}
	if e.Mode == Normal {
		e.clampNormal()
	}
	if alone {
		e.commit()
	}
}
