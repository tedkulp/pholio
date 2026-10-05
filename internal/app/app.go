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
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/sidebar"
	"github.com/tedkulp/pholio/internal/theme"
	"github.com/tedkulp/pholio/internal/watch"
)

// Deps are the injected seams the app reaches the outside world through.
type Deps struct {
	FS    seam.FS
	Clock seam.Clock
	// Watch, if set, reports changes made to the Vault outside pholio.
	Watch *watch.Vault
	// Opener opens http(s) URLs followed from a Note.
	Opener seam.Opener
	// Index, if set, is the Vault index. Init scans it in the background
	// and the open buffer's edits are fed to it.
	Index *index.Index
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
	deps    Deps
	vault   string
	file    *noteFile // the open Note's file; shared by copies, like the engine
	ed      editor.Model
	wrap    bool
	conceal bool
	w, h    int

	side   sidebar.Model
	sideOn bool
	sideW  int // the chosen width; drawn narrower on small screens
	focus  focus
	state  *config.StateStore // nil without a session: width is not saved

	leader  bool             // spc was pressed; the next key is a leader key
	pending *tea.KeyPressMsg // the first key of a sequence such as [d
	prompt  *prompt          // a question on the message line
	overlay *overlay         // an open palette
	exq     *exQueue         // ex commands the engine handed to the app

	session *config.Session // nil until WithSession
	looks   looks
	message string // shown on the message line until the next key
	problem bool   // the message reports a problem

	confirming bool // the overwrite y/N prompt is up (disk.go)

	jumps   jumplist // the session's Note history (jumplist.go)
	fed     uint64   // the buffer version last fed to the index (index.go)
	syncGen int      // the pending index feed; bumped to cancel it
}

// New opens the Note at path. A missing file opens as an empty buffer, and
// path "" opens an empty buffer with no file. Until WithSession, the Note's
// folder stands in for the Vault.
func New(deps Deps, path string) (Model, error) {
	if path != "" {
		path = filepath.Clean(path)
	}
	m := Model{
		deps: deps, vault: filepath.Dir(path), wrap: true, conceal: true,
		sideOn: true, sideW: config.DefaultState().SidebarWidth,
		exq: &exQueue{}, looks: defaultLooks(),
	}
	file, e, err := m.openEngine(path)
	if err != nil {
		return Model{}, err
	}
	m.file = file
	m.ed = m.newEditor(e)
	m.fed = e.Buf.Version()
	m.side = sidebar.New(deps.FS, m.vault).Reveal(path)
	return m.relayout(), nil
}

// openEngine loads path, through a new noteFile, into a new engine wired
// to the app's ex commands.
func (m Model) openEngine(path string) (*noteFile, *engine.Engine, error) {
	file := &noteFile{fs: m.deps.FS, watch: m.deps.Watch, index: m.deps.Index, path: path}
	e, err := engine.Open(file, path)
	if err != nil {
		return nil, nil, err
	}
	e.Msg = "" // the message line is the app's
	m.registerEx(e)
	return file, e, nil
}

// newEditor makes the editor pane over e with the configured settings.
func (m Model) newEditor(e *engine.Engine) editor.Model {
	return editor.New(e, m.rel(e.Path)).SetWrap(m.wrap).SetConceal(m.conceal)
}

// path is the open Note's path.
func (m Model) path() string { return m.file.path }

// rel is path relative to the Vault, for display.
func (m Model) rel(path string) string {
	if path == "" {
		return "[No Name]"
	}
	if r, err := filepath.Rel(m.vault, path); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return filepath.Base(path)
}

// Init implements tea.Model: it starts listening for watcher Notices and
// scanning the Vault index.
func (m Model) Init() tea.Cmd { return tea.Batch(m.listen(), m.scanIndex()) }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case noticeMsg:
		m.side = m.side.Refresh()
		if msg.Rescan || msg.Path == m.path() {
			m = m.checkDisk()
			if m.ed.Engine().Dirty { // the index read the file, not the buffer
				m = m.syncIndex()
			}
		}
		cmd = m.listen()
	case tea.FocusMsg:
		if m.deps.Watch != nil {
			m.deps.Watch.Rescan()
		}
		m.side = m.side.Refresh()
		m = m.checkDisk()
	case tea.KeyPressMsg:
		m, cmd = m.key(msg)
		var sync tea.Cmd
		m, sync = m.editedIndex()
		cmd = tea.Batch(cmd, sync)
	case tea.PasteMsg:
		m, cmd = m.paste(msg)
		var sync tea.Cmd
		m, sync = m.editedIndex()
		cmd = tea.Batch(cmd, sync)
	case paletteMsg:
		m = m.showPalette(msg.p, msg.on)
	case indexReadyMsg:
		m = m.indexScanned(msg)
	case grepTickMsg:
		m = m.grepTick(msg)
	case indexSyncMsg:
		if msg.gen == m.syncGen {
			m = m.syncIndex()
		}
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
	if m.confirming && name != "ctrl+q" {
		return m.answerOverwrite(msg), nil
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
	if m.pending != nil {
		first := *m.pending
		m.pending = nil
		return m.sequenceKey(first, msg)
	}
	if m.free() {
		if a, ok := lookup(scopeGlobal, name); ok {
			return m.run(a, "")
		}
		if startsSequence(name) {
			m.pending = &msg
			return m, nil
		}
	}
	if m.sidebarFocused() {
		m.message, m.problem = "", false
		if a, ok := lookup(scopeSidebar, name); ok {
			return m.run(a, "")
		}
		return m.sidebarKey(name)
	}
	if m.normalIdle() {
		if a, ok := lookup(scopeNormal, name); ok {
			m.message, m.problem = "", false
			return m.run(a, "")
		}
	}
	if e := m.ed.Engine(); e.Mode == engine.Normal && e.PendingKeys() == "g" && name == "d" {
		e.Feed("esc") // drop the pending g
		m.message, m.problem = "", false
		return m.run(actGoToLink, "")
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

// normalIdle reports whether the editor has focus in normal mode with
// nothing pending: where scopeNormal keys fire.
func (m Model) normalIdle() bool {
	e := m.ed.Engine()
	return !m.sidebarFocused() && e.Mode == engine.Normal && e.PendingKeys() == ""
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
	m = m.askedToOverwrite()
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
	v.ReportFocus = true
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
	// An open ":" or "/" line replaces the message line and takes the
	// cursor.
	if line, c, ok := m.ed.CmdLine(th, m.w); ok && m.prompt == nil {
		rows[len(rows)-1] = line
		c.Y = m.h - 1
		cursor = c
	}
	rows = rows[max(0, len(rows)-m.h):]
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
	text, cursor := m.ed.SetTags(m.file.tags()...).View(th)
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

// statusLine is the editor's status line, or the leader hint while spc is
// pending.
func (m Model) statusLine() string {
	th := m.looks.theme
	if !m.leader {
		return m.ed.SetTags(m.file.tags()...).StatusLine(th, m.w)
	}
	left := th.Style(theme.UIModeCommand).Render(" spc ") + th.Style(theme.UIStatusline).Render(" "+leaderHint())
	left = ansi.Truncate(left, m.w, "…")
	return left + th.Style(theme.UIStatusline).Render(strings.Repeat(" ", max(0, m.w-ansi.StringWidth(left))))
}

// messageLine shows the prompt, the app's message or the engine's, in that
// order. The cmdline, when open, is drawn over it in View.
func (m Model) messageLine() string {
	th := m.looks.theme
	e := m.ed.Engine()
	msg, slot := e.Msg, theme.UIMessage
	switch {
	case m.prompt != nil:
		msg = m.prompt.question
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
