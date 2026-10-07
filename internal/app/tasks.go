package app

import (
	"fmt"
	"path/filepath"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/tasks"
)

// taskRules is the engine hook for the Task rules: the done date after
// each change and relative-date expansion on leaving insert mode, both
// counted from Today, in the configured Task Format.
func (m Model) taskRules() engine.FixupFunc {
	clock := Model{deps: m.deps, session: m.session} // only what today and config need
	return tasks.Hook(clock.today, func() index.Format { return clock.config().TaskFormat })
}

// toggleTask toggles the Task at line (0-based) of the Note at path (the
// Task List's space). In the open Note it is an undoable edit to the
// buffer, which is not saved; any other Note is rewritten on disk.
func (m Model) toggleTask(path string, line int) (Model, error) {
	path = filepath.Clean(path)
	return m, m.editNote(path, func(e *engine.Engine) error {
		if !tasks.ToggleLine(e, line, m.config().TaskFormat, m.today()) {
			return fmt.Errorf("%s:%d: %w", m.rel(path), line+1, tasks.ErrNotATask)
		}
		return nil
	}, func(f *noteFile) error {
		return tasks.ToggleFile(f, path, line, m.config().TaskFormat, m.today())
	})
}

// editNote changes the Note at path: through buffer when it is the open
// Note, so the edit is undoable and unsaved, else through disk, which
// gets a file to rewrite it with (see otherFile).
func (m Model) editNote(path string, buffer func(*engine.Engine) error, disk func(*noteFile) error) error {
	if path == m.path() {
		return buffer(m.ed.Engine())
	}
	return disk(m.otherFile())
}

// otherFile is a file for writing Notes other than the open one. It
// writes the way the open Note's does (telling the watcher and index)
// without touching the open Note's disk state.
func (m Model) otherFile() *noteFile {
	f := *m.file
	f.e, f.opening = nil, ""
	return &f
}
