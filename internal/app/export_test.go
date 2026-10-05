package app

// Save stands in for :w until the command line lands (#21): the engine
// writes the buffer through the Model's noteFile, and the app reacts.
func Save(m Model) Model {
	e := m.ed.Engine()
	if err := m.file.WriteFile(m.path, []byte(e.Buf.String())); err == nil {
		e.Dirty = false
	}
	return m.askedToOverwrite()
}

// Text is the editor's buffer.
func (m Model) Text() string { return m.ed.Engine().Buf.String() }
