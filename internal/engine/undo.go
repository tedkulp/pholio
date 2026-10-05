package engine

// edit is one primitive mutation: at `at`, del was removed, then ins was
// inserted.
type edit struct {
	at       Pos
	del, ins string
}

// change is one undo step: every edit made by one normal command or one
// insert session, with the cursor before and after it.
type change struct {
	edits         []edit
	before, after Pos
}

func (e *Engine) record(ed edit) {
	if e.pending == nil {
		e.pending = &change{before: e.cmdStart}
	}
	e.pending.edits = append(e.pending.edits, ed)
	e.Dirty = true
}

// commit closes the pending change once the engine is back at rest.
func (e *Engine) commit() {
	if e.pending == nil {
		return
	}
	e.pending.after = e.Cur
	e.undo = append(e.undo, *e.pending)
	e.redo = nil
	e.pending = nil
	e.inserted = false
}

// ins inserts s at p, recording it for undo, and returns the end of s.
func (e *Engine) ins(p Pos, s string) Pos {
	if s == "" {
		return p
	}
	e.record(edit{at: p, ins: s})
	return e.Buf.insert(p, s)
}

// del deletes [a, z), recording it for undo, and returns the removed text.
func (e *Engine) del(a, z Pos) string {
	if !a.Less(z) {
		return ""
	}
	t := e.Buf.delete(a, z)
	e.record(edit{at: a, del: t})
	return t
}

func (e *Engine) undoStep() {
	if len(e.undo) == 0 {
		e.Msg = "Already at oldest change"
		return
	}
	c := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	for i := len(c.edits) - 1; i >= 0; i-- {
		ed := c.edits[i]
		if ed.ins != "" {
			e.Buf.delete(ed.at, endOf(ed.at, ed.ins))
		}
		if ed.del != "" {
			e.Buf.insert(ed.at, ed.del)
		}
	}
	e.redo = append(e.redo, c)
	e.Cur, e.Dirty = c.before, true
}

func (e *Engine) redoStep() {
	if len(e.redo) == 0 {
		e.Msg = "Already at newest change"
		return
	}
	c := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	for _, ed := range c.edits {
		if ed.del != "" {
			e.Buf.delete(ed.at, endOf(ed.at, ed.del))
		}
		if ed.ins != "" {
			e.Buf.insert(ed.at, ed.ins)
		}
	}
	e.undo = append(e.undo, c)
	e.Cur, e.Dirty = c.after, true
}
