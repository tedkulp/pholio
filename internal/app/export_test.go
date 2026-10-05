package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/palette"
)

// Text is the editor's buffer.
func (m Model) Text() string { return m.ed.Engine().Buf.String() }

// OpenPalette is a message that opens p, as a feature's tea.Cmd would.
// Each event the palette produces is passed to record.
func OpenPalette(p palette.Model, record func(palette.Event)) tea.Msg {
	return paletteMsg{p: p, on: func(m Model, ev palette.Event) (Model, tea.Cmd) {
		record(ev)
		return m, nil
	}}
}

// ToggleTask toggles the Task at line (0-based) of the Note at path, as the
// Task List's space does.
func (m Model) ToggleTask(path string, line int) (Model, error) { return m.toggleTask(path, line) }

// Path is the open Note's path.
func (m Model) Path() string { return m.path() }

// SidebarSelected is the path selected in the tree, or "".
func (m Model) SidebarSelected() string {
	n, _ := m.side.Selected()
	return n.Path
}

// Cursor is the editor's cursor.
func (m Model) Cursor() engine.Pos { return m.ed.Engine().Cur }
