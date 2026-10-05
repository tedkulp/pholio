package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/engine"
)

// action names something the user can do. Every binding, leader key and ex
// command goes through the action table, so each action has one handler.
type action string

// Actions.
const (
	actLeader        action = "leader"
	actToggleSidebar action = "toggle-sidebar"
	actFocusSidebar  action = "focus-sidebar"
	actFocusEditor   action = "focus-editor"
	actSidebarNarrow action = "sidebar-narrower"
	actSidebarWiden  action = "sidebar-wider"
	actQuit          action = "quit"
	actCycleTheme    action = "cycle-theme"
	actReload        action = "reload"
	actToday         action = "today"
	actDailyPrev     action = "daily-prev"
	actDailyNext     action = "daily-next"
	actJumpToDate    action = "jump-to-date"
	actFollowLink    action = "follow-link"
	actGoToLink      action = "go-to-link"
	actJumpBack      action = "jump-back"
	actJumpForward   action = "jump-forward"
	actBacklinks     action = "backlinks"
	actGrep          action = "grep"
	actZettel        action = "zettel"
	actFindNote      action = "find-note"
)

// scope is where a binding fires.
type scope int

const (
	// scopeApp keys fire everywhere except in a prompt, even over a palette.
	scopeApp scope = iota
	// scopeGlobal keys fire from the sidebar, or from the editor in normal
	// mode with nothing pending.
	scopeGlobal
	// scopeLeader keys follow spc.
	scopeLeader
	// scopeSidebar keys fire when the sidebar has focus, before the tree's
	// own navigation keys.
	scopeSidebar
	// scopeSequence keys are two-key sequences ("[d") that fire where
	// scopeGlobal keys do. The first key waits for the second; when the
	// pair is not bound, both go on as usual.
	scopeSequence
	// scopeNormal keys fire in the editor in normal mode with nothing
	// pending, before the engine sees them. (gd, two keys, is matched in
	// Model.key.)
	scopeNormal
)

// binding maps a key in a scope to an action. help is the leader hint.
type binding struct {
	scope  scope
	key    string // a key name as keyName gives it: " ", "e", "ctrl+h"
	action action
	help   string
}

// bindings is the action table: action name → key. There is no remapping
// in v1, so this is the whole keymap above the editor and the tree.
var bindings = []binding{
	{scopeApp, "ctrl+q", actQuit, ""},
	{scopeApp, "f7", actCycleTheme, ""},
	{scopeApp, "f8", actReload, ""},

	{scopeGlobal, " ", actLeader, ""},
	{scopeGlobal, "ctrl+h", actFocusSidebar, ""},
	{scopeGlobal, "ctrl+l", actFocusEditor, ""},

	{scopeLeader, "e", actToggleSidebar, "sidebar"},
	{scopeLeader, "d", actToday, "today"},
	{scopeLeader, "D", actJumpToDate, "date"},
	{scopeLeader, "b", actBacklinks, "backlinks"},
	{scopeLeader, "/", actGrep, "search"},
	{scopeLeader, "z", actZettel, "zettel"},
	{scopeLeader, "f", actFindNote, "find"},

	{scopeSequence, "[d", actDailyPrev, ""},
	{scopeSequence, "]d", actDailyNext, ""},

	{scopeSidebar, "tab", actFocusEditor, ""},
	{scopeSidebar, "esc", actFocusEditor, ""},
	{scopeSidebar, "<", actSidebarNarrow, ""},
	{scopeSidebar, ">", actSidebarWiden, ""},

	{scopeNormal, "enter", actFollowLink, ""},
	{scopeNormal, "ctrl+o", actJumpBack, ""},
	{scopeNormal, "tab", actJumpForward, ""},
	{scopeNormal, "ctrl+i", actJumpForward, ""},
}

// exCommands maps ":" commands to actions; the text after the name is the
// action's argument. The engine's built-ins (:w, :q, :e) win over these.
var exCommands = map[string]action{
	"sidebar":   actToggleSidebar,
	"today":     actToday,
	"daily":     actJumpToDate,
	"backlinks": actBacklinks,
	"grep":      actGrep,
	"zettel":    actZettel,
	"find":      actFindNote,
}

// handler runs an action. arg is an ex command's argument, else "".
type handler func(m Model, arg string) (Model, tea.Cmd)

// handlers is filled in init, since handlers reach back into the tables.
var handlers map[action]handler

func init() {
	handlers = map[action]handler{
		actLeader:        func(m Model, _ string) (Model, tea.Cmd) { m.leader = true; return m, nil },
		actToggleSidebar: func(m Model, _ string) (Model, tea.Cmd) { return m.toggleSidebar(), nil },
		actFocusSidebar:  func(m Model, _ string) (Model, tea.Cmd) { return m.focusSidebar(), nil },
		actFocusEditor:   func(m Model, _ string) (Model, tea.Cmd) { m.focus = focusEditor; return m, nil },
		actSidebarNarrow: func(m Model, _ string) (Model, tea.Cmd) { return m.resizeSidebar(-sidebarStep), nil },
		actSidebarWiden:  func(m Model, _ string) (Model, tea.Cmd) { return m.resizeSidebar(sidebarStep), nil },
		actQuit:          func(m Model, _ string) (Model, tea.Cmd) { return m.requestQuit() },
		actCycleTheme:    func(m Model, _ string) (Model, tea.Cmd) { return m.cycleTheme(), nil },
		actReload:        func(m Model, _ string) (Model, tea.Cmd) { return m.reload(), nil },
		actToday:         func(m Model, _ string) (Model, tea.Cmd) { return m.openDaily(m.today()) },
		actDailyPrev:     func(m Model, _ string) (Model, tea.Cmd) { return m.stepDaily(-1) },
		actDailyNext:     func(m Model, _ string) (Model, tea.Cmd) { return m.stepDaily(1) },
		actJumpToDate:    func(m Model, arg string) (Model, tea.Cmd) { return m.jumpToDate(arg) },
		actFollowLink:    func(m Model, _ string) (Model, tea.Cmd) { return m.followOrMove() },
		actGoToLink:      func(m Model, _ string) (Model, tea.Cmd) { return m.goToLink() },
		actJumpBack:      func(m Model, _ string) (Model, tea.Cmd) { return m.jumpBack() },
		actJumpForward:   func(m Model, _ string) (Model, tea.Cmd) { return m.jumpForward() },
		actBacklinks:     func(m Model, _ string) (Model, tea.Cmd) { return m.showBacklinks() },
		actGrep:          func(m Model, arg string) (Model, tea.Cmd) { return m.showGrep(arg) },
		actZettel:        func(m Model, arg string) (Model, tea.Cmd) { return m.newZettel(arg) },
		actFindNote:      func(m Model, arg string) (Model, tea.Cmd) { return m.findNote(arg) },
	}
}

// lookup finds the action bound to key in a scope.
func lookup(s scope, key string) (action, bool) {
	for _, b := range bindings {
		if b.scope == s && b.key == key {
			return b.action, true
		}
	}
	return "", false
}

// run performs an action.
func (m Model) run(a action, arg string) (Model, tea.Cmd) {
	h, ok := handlers[a]
	if !ok {
		return m.say(string(a)+": not available yet", true), nil
	}
	return h(m, arg)
}

// startsSequence reports whether key is the first key of a bound
// sequence.
func startsSequence(key string) bool {
	for _, b := range bindings {
		if b.scope == scopeSequence && len(b.key) > len(key) && strings.HasPrefix(b.key, key) {
			return true
		}
	}
	return false
}

// sequenceKey runs the key after the first key of a sequence. When the
// pair is not bound, the first key goes to the editor and the second is
// routed as usual.
func (m Model) sequenceKey(first, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if a, ok := lookup(scopeSequence, keyName(first)+keyName(msg)); ok {
		return m.run(a, "")
	}
	if !m.sidebarFocused() {
		m, _ = m.editorKey(first)
	}
	return m.key(msg)
}

// leaderHint lists the leader keys: "e sidebar  t tasks".
func leaderHint() string {
	var parts []string
	for _, b := range bindings {
		if b.scope == scopeLeader {
			parts = append(parts, b.key+" "+b.help)
		}
	}
	return strings.Join(parts, "  ")
}

// leaderKey runs the key after spc.
func (m Model) leaderKey(name string) (Model, tea.Cmd) {
	if name == "esc" {
		return m, nil
	}
	a, ok := lookup(scopeLeader, name)
	if !ok {
		return m.say("spc "+name+": not bound", true), nil
	}
	return m.run(a, "")
}

// exRequest is an ex command the engine passed up to the app.
type exRequest struct {
	action action
	arg    string
}

// exQueue collects ex commands while the engine runs them inside Feed. It
// is shared by every copy of the Model and drained right after each key.
type exQueue struct{ reqs []exRequest }

// registerEx teaches an engine the app's ex commands.
func (m Model) registerEx(e *engine.Engine) {
	q := m.exq
	for name, a := range exCommands {
		e.Register(name, func(_ *engine.Engine, c engine.ExCmd) error {
			q.reqs = append(q.reqs, exRequest{a, c.Arg})
			return nil
		})
	}
}

// drainEx runs the ex commands queued by the last key.
func (m Model) drainEx() (Model, tea.Cmd) {
	reqs := m.exq.reqs
	m.exq.reqs = nil
	var cmds []tea.Cmd
	for _, r := range reqs {
		var cmd tea.Cmd
		m, cmd = m.run(r.action, r.arg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}
