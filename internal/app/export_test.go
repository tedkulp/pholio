package app

// Text is the editor's buffer.
func (m Model) Text() string { return m.ed.Engine().Buf.String() }
