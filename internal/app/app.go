// Package app holds pholio's root Bubble Tea model.
package app

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/editor"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/theme"
)

// Deps are the injected seams the app reaches the outside world through.
type Deps struct {
	FS    seam.FS
	Clock seam.Clock
}

// Model is the root model: for now, one editor pane over one Note.
type Model struct {
	deps Deps
	path string
	ed   editor.Model
	w, h int

	session *config.Session // nil until WithSession
	looks   looks
	message string // shown on the message line until the next key
	problem bool   // the message reports a problem
}

// New opens the Note at path. A missing file opens as an empty buffer.
func New(deps Deps, path string) (Model, error) {
	data, err := deps.FS.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Model{}, err
	}
	ed := editor.New(engine.New(string(data)), filepath.Base(path))
	return Model{deps: deps, path: path, ed: ed, looks: defaultLooks()}, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.ed = m.ed.SetSize(m.w, m.h-2)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+q":
			return m, tea.Quit
		case "f7":
			m = m.cycleTheme()
		case "f8":
			m = m.reload()
		default:
			m.message, m.problem = "", false
			m.ed = m.ed.Update(msg)
			if e := m.ed.Engine(); e.Quit {
				e.Quit = false
				return m, tea.Quit
			}
		}
	case tea.PasteMsg:
		m.ed = m.ed.Update(msg)
	}
	return m, nil
}

// View implements tea.Model: the editor pane fills the screen above the
// status line and the message line.
func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	if m.w <= 0 || m.h <= 0 {
		return v
	}
	th := m.looks.theme
	text, cursor := m.ed.View(th)
	rows := []string{m.ed.StatusLine(th, m.w), m.messageLine()}
	if m.h > 2 {
		rows = append([]string{text}, rows...)
		v.Cursor = cursor
	}
	// An open ":" or "/" line replaces the message line and takes the
	// cursor.
	if line, c, ok := m.ed.CmdLine(th, m.w); ok {
		rows[len(rows)-1] = line
		c.Y = m.h - 1
		v.Cursor = c
	}
	rows = rows[max(0, len(rows)-m.h):]
	v.SetContent(strings.Join(rows, "\n"))
	return v
}

// messageLine shows the app's message, else the engine's.
func (m Model) messageLine() string {
	th := m.looks.theme
	msg, slot := m.ed.Engine().Msg, theme.UIMessage
	if m.message != "" {
		msg = m.message
		if m.problem {
			slot = theme.UIError
		}
	}
	if msg == "" {
		return ""
	}
	return th.Style(slot).Render(ansi.Truncate(msg, m.w, "…"))
}
