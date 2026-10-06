package engine

// InsertLine inserts text as a new line before line i (at the end when i
// is the line count) as one undo step, in normal mode. The cursor stays on
// the text it was on. The Task List's `a` uses it when today's Daily Note
// is open.
func (e *Engine) InsertLine(i int, text string) {
	e.commit()
	e.Mode, e.keys, e.recording = Normal, nil, false
	e.clampNormal()
	cur := e.Cur
	e.cmdStart = cur
	n := e.Buf.LineCount()
	i = max(0, min(i, n))
	if i == n {
		e.ins(Pos{Line: n - 1, Col: len(e.line(n - 1))}, "\n"+text)
	} else {
		e.ins(Pos{Line: i}, text+"\n")
		if i <= cur.Line {
			cur.Line++
		}
	}
	e.SetCursor(cur)
	e.commit()
}
