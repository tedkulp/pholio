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
	"github.com/tedkulp/pholio/internal/watch"
)

// Deps are the injected seams the app reaches the outside world through.
type Deps struct {
	FS    seam.FS
	Clock seam.Clock
	// Watch, if set, reports changes made to the Vault outside pholio.
	Watch *watch.Vault
}

// Model is the root model: for now, one editor pane over one Note.
type Model struct {
	deps Deps
	path string
	file *noteFile // shared by copies of the Model, like the engine
	ed   editor.Model
	w, h int

	session *config.Session // nil until WithSession
	looks   looks
	message string // shown on the message line until the next key
	problem bool   // the message reports a problem

	confirming bool // the overwrite y/N prompt is up
}

// New opens the Note at path. A missing file opens as an empty buffer.
func New(deps Deps, path string) (Model, error) {
	path = filepath.Clean(path)
	file := &noteFile{fs: deps.FS, watch: deps.Watch, path: path}
	data, err := file.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Model{}, err
	}
	ed := editor.New(engine.New(string(data)), filepath.Base(path))
	return Model{deps: deps, path: path, file: file, ed: ed, looks: defaultLooks()}, nil
}

// Init implements tea.Model: it starts listening for watcher Notices.
func (m Model) Init() tea.Cmd { return m.listen() }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.ed = m.ed.SetSize(m.w, m.h-2)
	case noticeMsg:
		if msg.Rescan || msg.Path == m.path {
			m = m.checkDisk()
		}
		return m, m.listen()
	case tea.FocusMsg:
		if m.deps.Watch != nil {
			m.deps.Watch.Rescan()
		}
		m = m.checkDisk()
	case tea.KeyPressMsg:
		if m.confirming && msg.String() != "ctrl+q" {
			return m.answerOverwrite(msg), nil
		}
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
			m = m.askedToOverwrite()
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
	v.ReportFocus = true
	if m.w <= 0 || m.h <= 0 {
		return v
	}
	th := m.looks.theme
	ed := m.ed.SetTags(m.file.tags()...)
	text, cursor := ed.View(th)
	rows := []string{ed.StatusLine(th, m.w), m.messageLine()}
	if m.h > 2 {
		rows = append([]string{text}, rows...)
		v.Cursor = cursor
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
