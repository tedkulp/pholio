// Package app holds pholio's root Bubble Tea model.
package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/editor"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/sidebar"
	"github.com/tedkulp/pholio/internal/theme"
)

// Deps are the injected seams the app reaches the outside world through.
type Deps struct {
	FS    seam.FS
	Clock seam.Clock
}

// focus is the pane that receives keys.
type focus int

const (
	focusEditor focus = iota
	focusSidebar
)

// Model is the root model: the sidebar and the editor pane, the status and
// message lines, and at most one prompt or palette on top.
type Model struct {
	deps  Deps
	vault string
	ed    editor.Model
	wrap  bool
	w, h  int

	side   sidebar.Model
	sideOn bool
	sideW  int // the chosen width; drawn narrower on small screens
	focus  focus
	state  *config.StateStore // nil without a session: width is not saved

	leader  bool     // spc was pressed; the next key is a leader key
	prompt  *prompt  // a question on the message line
	overlay *overlay // an open palette
	exq     *exQueue // ex commands the engine handed to the app

	session *config.Session // nil until WithSession
	looks   looks
	message string // shown on the message line until the next key
	problem bool   // the message reports a problem
}

// New opens the Note at path. A missing file opens as an empty buffer.
// Until WithSession, the Note's folder stands in for the Vault.
func New(deps Deps, path string) (Model, error) {
	m := Model{
		deps: deps, vault: filepath.Dir(path), wrap: true,
		sideOn: true, sideW: config.DefaultState().SidebarWidth,
		exq: &exQueue{}, looks: defaultLooks(),
	}
	e, err := m.openEngine(path)
	if err != nil {
		return Model{}, err
	}
	m.ed = editor.New(e, m.rel(path))
	m.side = sidebar.New(deps.FS, m.vault).Reveal(path)
	return m.relayout(), nil
}

// openEngine loads path into a new engine wired to the app's ex commands.
func (m Model) openEngine(path string) (*engine.Engine, error) {
	e, err := engine.Open(m.deps.FS, path)
	if err != nil {
		return nil, err
	}
	e.Msg = ""
	m.registerEx(e)
	return e, nil
}

// path is the open Note's path.
func (m Model) path() string { return m.ed.Engine().Path }

// rel is path relative to the Vault, for display.
func (m Model) rel(path string) string {
	if r, err := filepath.Rel(m.vault, path); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return filepath.Base(path)
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyPressMsg:
		m, cmd = m.key(msg)
	case tea.PasteMsg:
		m, cmd = m.paste(msg)
	case paletteMsg:
		m = m.showPalette(msg.p, msg.on)
	}
	return m.relayout(), cmd
}

// key routes one key press: the prompt, app keys, the palette, the
// leader, global keys, the sidebar and finally the editor.
func (m Model) key(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	name := keyName(msg)
	if m.prompt != nil {
		return m.answer(name)
	}
	if a, ok := lookup(scopeApp, name); ok {
		return m.run(a, "")
	}
	if m.overlay != nil {
		return m.overlayKey(msg)
	}
	if m.leader {
		m.leader = false
		return m.leaderKey(name)
	}
	if m.free() {
		if a, ok := lookup(scopeGlobal, name); ok {
			return m.run(a, "")
		}
	}
	if m.sidebarFocused() {
		m.message, m.problem = "", false
		if a, ok := lookup(scopeSidebar, name); ok {
			return m.run(a, "")
		}
		return m.sidebarKey(name)
	}
	return m.editorKey(msg)
}

// free reports whether global keys (the leader, ctrl+h/l) may fire: from
// the sidebar, or in normal mode with nothing pending.
func (m Model) free() bool {
	if m.sidebarFocused() {
		return true
	}
	e := m.ed.Engine()
	return e.Mode == engine.Normal && e.PendingKeys() == ""
}

func (m Model) editorKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	m.message, m.problem = "", false
	e := m.ed.Engine()
	if e.Mode == engine.Command && keyName(msg) == "enter" {
		if prompt, text := e.CmdLine(); prompt == ":" {
			switch strings.Fields(text + " ")[0] {
			case "q", "quit":
				e.Feed("esc")
				return m.requestQuit()
			case "q!", "quit!":
				return m, tea.Quit
			}
		}
	}
	m.ed = m.ed.Update(msg)
	m, cmd := m.drainEx()
	if e.Quit { // :wq and :x, once written
		e.Quit = false
		return m.requestQuit()
	}
	return m, cmd
}

func (m Model) paste(msg tea.PasteMsg) (Model, tea.Cmd) {
	switch {
	case m.prompt != nil:
	case m.overlay != nil:
		return m.overlayPaste(msg.Content)
	case !m.sidebarFocused():
		m.ed = m.ed.Update(msg)
	}
	return m, nil
}

// keyName turns a key press into a key name: the typed text for plain
// printable keys (so space is " "), else its name ("ctrl+h", "enter").
func keyName(msg tea.KeyPressMsg) string {
	k := msg.Key()
	if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return k.Text
	}
	return msg.Keystroke()
}

// relayout sizes the panes for the screen and the sidebar's width.
func (m Model) relayout() Model {
	if m.w <= 0 || m.h <= 0 {
		return m
	}
	m.ed = m.ed.SetSize(m.w-m.sidebarWidth(), max(1, m.h-2))
	m.side = m.side.SetHeight(max(1, m.h-2))
	return m
}

// View implements tea.Model: the sidebar and the editor above the status
// line and the message line, with any palette layered on top.
func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	if m.w <= 0 || m.h <= 0 {
		return v
	}
	th := m.looks.theme
	rows := []string{m.statusLine(), m.messageLine()}
	var cursor *tea.Cursor
	if m.h > 2 {
		body, c := m.body()
		rows = append([]string{body}, rows...)
		cursor = c
	}
	rows = rows[max(0, len(rows)-m.h):]
	if m.prompt == nil && !m.sidebarFocused() && m.cmdline() {
		p, t := m.ed.Engine().CmdLine()
		cursor = tea.NewCursor(min(m.w-1, ansi.StringWidth(p+t)), m.h-1)
		cursor.Shape, cursor.Blink = tea.CursorBar, false
	}
	content := strings.Join(rows, "\n")
	if m.overlay != nil {
		content, cursor = m.composeOverlay(th, content)
	}
	v.SetContent(content)
	v.Cursor = cursor
	return v
}

// body is the pane area: sidebar and editor side by side.
func (m Model) body() (string, *tea.Cursor) {
	th := m.looks.theme
	text, cursor := m.ed.View(th)
	sw := m.sidebarWidth()
	if m.sidebarFocused() {
		cursor = nil
	} else if cursor != nil {
		cursor.X += sw
	}
	if sw == 0 {
		return text, cursor
	}
	side := strings.Split(m.side.View(th, sw, m.h-2, m.sidebarFocused(), m.path()), "\n")
	lines := strings.Split(text, "\n")
	for i := range lines {
		if i < len(side) {
			lines[i] = side[i] + lines[i]
		}
	}
	return strings.Join(lines, "\n"), cursor
}

// cmdline reports whether the editor is reading a ":" or "/" line.
func (m Model) cmdline() bool {
	mode := m.ed.Engine().Mode
	return mode == engine.Command || mode == engine.Search
}

// statusLine is the editor's status line, or the leader hint while spc is
// pending.
func (m Model) statusLine() string {
	th := m.looks.theme
	if !m.leader {
		return m.ed.StatusLine(th, m.w)
	}
	left := th.Style(theme.UIModeCommand).Render(" spc ") + th.Style(theme.UIStatusline).Render(" "+leaderHint())
	left = ansi.Truncate(left, m.w, "…")
	return left + th.Style(theme.UIStatusline).Render(strings.Repeat(" ", max(0, m.w-ansi.StringWidth(left))))
}

// messageLine shows the prompt, the cmdline, the app's message or the
// engine's, in that order.
func (m Model) messageLine() string {
	th := m.looks.theme
	e := m.ed.Engine()
	msg, slot := e.Msg, theme.UIMessage
	switch {
	case m.prompt != nil:
		msg = m.prompt.question
	case m.cmdline() && !m.sidebarFocused():
		p, t := e.CmdLine()
		msg = p + t
	case m.message != "":
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

// say sets the app message; problem picks the error style.
func (m Model) say(msg string, problem bool) Model {
	m.message, m.problem = msg, problem
	return m
}
