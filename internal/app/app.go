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
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/theme"
)

// Deps are the injected seams the app reaches the outside world through.
type Deps struct {
	FS    seam.FS
	Clock seam.Clock
}

// Model is the root model: for now, a single pane showing one Note.
type Model struct {
	deps  Deps
	path  string
	lines []string
	w, h  int

	session *config.Session // nil until WithSession
	looks   looks
	message string // shown on the status line
	problem bool   // the message reports a problem
}

// New opens the Note at path. A missing file opens as an empty buffer.
func New(deps Deps, path string) (Model, error) {
	m := Model{deps: deps, path: path, looks: defaultLooks()}
	data, err := deps.FS.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		m.lines = []string{""}
	case err != nil:
		return Model{}, err
	default:
		m.lines = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}
	return m, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+q":
			return m, tea.Quit
		case "f7":
			m = m.cycleTheme()
		case "f8":
			m = m.reload()
		}
	}
	return m, nil
}

// View implements tea.Model: the Note fills the pane, with a status line
// naming the file on the last row.
func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	if m.w <= 0 || m.h <= 0 {
		return v
	}
	th := m.looks.theme
	base := th.Style(theme.UIBase)
	rows := make([]string, 0, m.h)
	for i := 0; i < m.h-1; i++ {
		line := ""
		if i < len(m.lines) && m.lines[i] != "" {
			line = base.Render(ansi.Truncate(m.lines[i], m.w, ""))
		}
		rows = append(rows, line)
	}
	rows = append(rows, m.statusLine())
	v.SetContent(strings.Join(rows, "\n"))
	return v
}

// statusLine draws the file name and any message across the full width.
func (m Model) statusLine() string {
	th := m.looks.theme
	bar := th.Style(theme.UIStatusline)
	line := th.Style(theme.UIStatusFile).Render(" " + filepath.Base(m.path))
	if m.message != "" {
		slot := theme.UIMessage
		if m.problem {
			slot = theme.UIError
		}
		line += bar.Render("  ") + th.Style(slot).Inherit(bar).Render(m.message)
	}
	line = ansi.Truncate(line, m.w, "")
	if pad := m.w - ansi.StringWidth(line); pad > 0 {
		line += bar.Render(strings.Repeat(" ", pad))
	}
	return line
}
