package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/sidebar"
)

// Sidebar width limits. The editor keeps at least minEditor columns, and a
// sidebar squeezed below minSidebar is not drawn.
const (
	sidebarMin  = 16
	sidebarMax  = 80
	sidebarStep = 4
	minEditor   = 20
	minSidebar  = 12
)

// sidebarWidth is the drawn width of the sidebar, border included; 0 when
// it is hidden or the screen is too narrow for it.
func (m Model) sidebarWidth() int {
	if !m.sideOn {
		return 0
	}
	sw := min(m.sideW, m.w-minEditor)
	if sw < minSidebar {
		return 0
	}
	return sw
}

func (m Model) sidebarFocused() bool {
	return m.focus == focusSidebar && m.sidebarWidth() > 0
}

// toggleSidebar (spc e, :sidebar): showing it focuses it, hiding it
// returns focus to the editor.
func (m Model) toggleSidebar() Model {
	m.sideOn = !m.sideOn
	m.focus = focusEditor
	if m.sideOn {
		m.focus = focusSidebar
	}
	return m
}

// focusSidebar (ctrl+h) shows the sidebar if needed and focuses it.
func (m Model) focusSidebar() Model {
	m.sideOn, m.focus = true, focusSidebar
	return m
}

// resizeSidebar (< and >) changes the width and saves it to state.
func (m Model) resizeSidebar(delta int) Model {
	m.sideW = max(sidebarMin, min(sidebarMax, m.sideW+delta))
	if m.state == nil {
		return m
	}
	if err := m.state.Save(config.State{SidebarWidth: m.sideW}); err != nil {
		return m.say("saving sidebar width: "+err.Error(), true)
	}
	return m
}

// sidebarKey gives a key to the tree.
func (m Model) sidebarKey(name string) (Model, tea.Cmd) {
	var ev sidebar.Event
	m.side, ev = m.side.Update(name)
	switch ev.Kind {
	case sidebar.Open:
		return m.openNote(ev.Path)
	case sidebar.Message:
		return m.say(ev.Text, false), nil
	}
	return m, nil
}

// openNote shows the Note at path in the editor and focuses it. The open
// Note is the only buffer, so a dirty one is saved or discarded first,
// after asking. Opening the Note that is already open just focuses it.
func (m Model) openNote(path string) (Model, tea.Cmd) {
	if path == m.path() {
		m.focus = focusEditor
		return m, nil
	}
	return m.unlessDirty(saveBeforeSwitch, func(m Model) (Model, tea.Cmd) {
		return m.switchTo(path), nil
	})
}

// saveBeforeSwitch asks about a dirty buffer before another Note replaces
// it.
const saveBeforeSwitch = "Save changes to %s? y save · n discard · esc cancel"

// switchTo replaces the editor's buffer with the Note at path.
func (m Model) switchTo(path string) Model {
	file, e, err := m.openEngine(path)
	if err != nil {
		return m.say(err.Error(), true)
	}
	m.file, m.confirming = file, false
	m.ed = m.newEditor(e)
	m.side = m.side.Reveal(path)
	m.focus = focusEditor
	return m.relayout()
}

// requestQuit (ctrl+q, :q) quits, asking first when the buffer is dirty.
func (m Model) requestQuit() (Model, tea.Cmd) {
	return m.unlessDirty("Save changes to %s before quitting? y save · n discard · esc cancel", func(m Model) (Model, tea.Cmd) {
		return m, tea.Quit
	})
}

// unlessDirty runs then now when the buffer is clean. Otherwise it asks
// question (with %s for the Note): y saves and runs then, n runs then
// without saving, esc cancels.
func (m Model) unlessDirty(question string, then func(Model) (Model, tea.Cmd)) (Model, tea.Cmd) {
	e := m.ed.Engine()
	if !e.Dirty {
		return then(m)
	}
	return m.ask(fmt.Sprintf(question, m.rel(e.Path)), map[string]func(Model) (Model, tea.Cmd){
		"y": func(m Model) (Model, tea.Cmd) {
			m, ok := m.save()
			if !ok {
				return m, nil
			}
			return then(m)
		},
		"n": then,
	}), nil
}

// save writes the buffer to its Note. It refuses, like :w, when the Note
// changed on disk; the message says how to resolve that.
func (m Model) save() (Model, bool) {
	e := m.ed.Engine()
	if err := m.file.WriteFile(m.path(), []byte(e.Buf.String())); err != nil {
		m.file.refused = false
		return m.say("saving "+m.rel(m.path())+": "+err.Error()+" (:e! to reload, :w to overwrite)", true), false
	}
	e.Dirty = false
	return m, true
}

// prompt is a question on the message line. Keys other than its answers
// and esc are ignored while it is up.
type prompt struct {
	question string
	answers  map[string]func(Model) (Model, tea.Cmd)
}

// ask puts a question on the message line.
func (m Model) ask(question string, answers map[string]func(Model) (Model, tea.Cmd)) Model {
	m.prompt = &prompt{question: question, answers: answers}
	m.leader = false
	return m
}

func (m Model) answer(name string) (Model, tea.Cmd) {
	if name == "esc" || name == "ctrl+c" {
		m.prompt = nil
		return m, nil
	}
	f, ok := m.prompt.answers[name]
	if !ok {
		return m, nil
	}
	m.prompt = nil
	return f(m)
}
