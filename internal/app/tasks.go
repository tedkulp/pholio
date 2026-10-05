package app

import (
	"fmt"
	"path/filepath"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/tasks"
)

// taskRules is the engine hook for the Task rules: the done: stamp after
// each change and relative-date expansion on leaving insert mode, both
// counted from Today.
func (m Model) taskRules() engine.FixupFunc {
	clock := Model{deps: m.deps, session: m.session} // only what today needs
	return tasks.Hook(clock.today)
}

// toggleTask toggles the Task at line (0-based) of the Note at path (the
// Task List's space). In the open Note it is an undoable edit to the
// buffer, which is not saved; any other Note is rewritten on disk.
func (m Model) toggleTask(path string, line int) (Model, error) {
	path = filepath.Clean(path)
	if path == m.path() {
		if !tasks.ToggleLine(m.ed.Engine(), line, m.today()) {
			return m, fmt.Errorf("%s:%d: %w", m.rel(path), line+1, tasks.ErrNotATask)
		}
		return m, nil
	}
	// A copy of the open Note's file writes the same way (reporting the
	// write to the watcher) without touching the open Note's disk state.
	other := *m.file
	other.path = ""
	return m, tasks.ToggleFile(&other, path, line, m.today())
}
