package app

// Completion is the [[ popup's rows as plain text, or nil when it is closed.
func (m Model) Completion() []string {
	if m.complete == nil {
		return nil
	}
	return m.complete.lines()
}
