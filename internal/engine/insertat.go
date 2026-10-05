package engine

// InsertAfterCursor inserts s after the character under the cursor, as `a`
// would, or at the start of an empty line. It is one undo step, the engine
// returns to normal mode and the cursor ends on the last inserted
// character. The app uses it to drop a Zettel's Link into its Origin.
func (e *Engine) InsertAfterCursor(s string) {
	e.commit()
	e.Mode, e.keys, e.recording = Normal, nil, false
	e.clampNormal()
	e.cmdStart = e.Cur
	at := Pos{Line: e.Cur.Line, Col: nextG(e.line(e.Cur.Line), e.Cur.Col)}
	end := e.ins(at, s)
	e.SetCursor(Pos{Line: end.Line, Col: prevG(e.line(end.Line), end.Col)})
	e.commit()
}
