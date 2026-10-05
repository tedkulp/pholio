package app

// Dirty reports whether the open buffer has unsaved changes.
func (m Model) Dirty() bool { return m.ed.Engine().Dirty }
