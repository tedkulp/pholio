// Package editor is the editor pane: it feeds keys to a vim engine and
// draws the engine's buffer with soft wrap, scrolloff, a real cursor,
// markdown highlighting and conceal.
package editor

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/theme"
)

// Scrolloff is how many rows of context stay visible above and below the
// cursor.
const Scrolloff = 3

// spot is a screen row of the buffer: row r of line's layout.
type spot struct{ line, row int }

// Model is the editor pane. The engine is shared between copies of a Model,
// so treat a Model as a handle: use the value Update returns.
type Model struct {
	e    *engine.Engine
	name string   // file name for the status line
	tags []string // shown after the name, such as "[deleted]"
	w, h int      // pane size in cells, status line not included
	wrap bool
	// conceal hides markdown syntax on every line but the cursor's.
	conceal bool
	// fence is the fence pass over the buffer: fence[i] is true when line
	// i is a fence or inside a fenced code block.
	fence []bool

	top  spot // first visible row
	left int  // first visible cell column when wrap is off
}

// New makes a pane over e, showing name on the status line. Wrap and
// conceal are on.
func New(e *engine.Engine, name string) Model {
	return Model{e: e, name: name, wrap: true, conceal: true, fence: fences(e.Buf)}
}

// Engine is the engine the pane edits.
func (m Model) Engine() *engine.Engine { return m.e }

// SetTags sets the markers shown after the file name on the status line,
// such as "[deleted]". No tags clears them.
func (m Model) SetTags(tags ...string) Model {
	m.tags = tags
	return m
}

// SetSize sets the text area to w×h cells.
func (m Model) SetSize(w, h int) Model {
	m.w, m.h = max(1, w), max(1, h)
	m.e.PageLines = m.h
	m.scroll()
	return m
}

// SetWrap turns soft wrap on or off. Off, long lines scroll sideways.
func (m Model) SetWrap(on bool) Model {
	m.wrap = on
	m.left = 0
	m.scroll()
	return m
}

// SetConceal turns conceal on or off. On, [[ ]], **, backticks and link
// URLs are hidden on every line except the cursor's.
func (m Model) SetConceal(on bool) Model {
	m.conceal = on
	m.scroll()
	return m
}

// Update handles key presses and pastes.
func (m Model) Update(msg tea.Msg) Model {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.e.Feed(keyName(msg))
	case tea.PasteMsg:
		m.e.Paste(msg.Content)
	default:
		return m
	}
	m.scroll()
	return m
}

// keyName turns a key press into an engine key: the typed text for plain
// printable keys (so space is " " and shift+g is "G"), else its name.
func keyName(msg tea.KeyPressMsg) string {
	k := msg.Key()
	if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return k.Text
	}
	return msg.Keystroke()
}

// lay lays out buffer line i for the pane, highlighted, and concealed
// unless it is the cursor's line.
func (m *Model) lay(i int) layout {
	l := m.e.Buf.Line(i)
	cur := m.e.Cur
	ks, hidden := highlight(l, i < len(m.fence) && m.fence[i])
	if !m.conceal || i == cur.Line {
		hidden = nil
	}
	return layoutLine(l, ks, hidden, m.w, m.wrap, i == cur.Line && cur.Col >= len(l) && l != "")
}

// up moves s up by n rows, stopping at the first row of the buffer, and
// reports how many rows it moved.
func (m *Model) up(s spot, n int) (spot, int) {
	moved := 0
	for ; moved < n; moved++ {
		switch {
		case s.row > 0:
			s.row--
		case s.line > 0:
			s.line--
			s.row = len(m.lay(s.line)) - 1
		default:
			return s, moved
		}
	}
	return s, moved
}

// rowsBelow counts the rows after s, up to limit.
func (m *Model) rowsBelow(s spot, limit int) int {
	n := len(m.lay(s.line)) - s.row - 1
	for i := s.line + 1; i < m.e.Buf.LineCount() && n < limit; i++ {
		n += len(m.lay(i))
	}
	return min(n, limit)
}

// cursorSpot is the screen row of the cursor and its cell x in that row.
func (m *Model) cursorSpot() (spot, int) {
	cur := m.e.Cur
	r, x := m.lay(cur.Line).find(cur.Col)
	return spot{cur.Line, r}, x
}

// scroll moves the view so the cursor is visible with Scrolloff rows of
// context. It walks at most a screenful of rows from the cursor, so it
// costs O(viewport) however far the cursor jumped.
func (m *Model) scroll() {
	m.fence = fences(m.e.Buf)
	if m.h == 0 {
		return
	}
	// Edits and resizes can leave the top past the buffer or its line.
	m.top.line = min(m.top.line, m.e.Buf.LineCount()-1)
	m.top.row = min(m.top.row, len(m.lay(m.top.line))-1)

	so := min(Scrolloff, (m.h-1)/2)
	cur, x := m.cursorSpot()
	dist := m.distance(m.top, cur, m.h)
	if dist < so {
		m.top, _ = m.up(cur, so)
	} else if below := m.rowsBelow(cur, so); dist+1+below > m.h {
		m.top, _ = m.up(cur, m.h-1-below)
	}

	if m.wrap {
		m.left = 0
		return
	}
	if x < m.left {
		m.left = x
	}
	if x >= m.left+m.w {
		m.left = x - m.w + 1
	}
}

// distance counts the rows from top down to s: -1 when s is above top,
// limit+1 when it is further than limit.
func (m *Model) distance(top, s spot, limit int) int {
	if s.line < top.line || s.line == top.line && s.row < top.row {
		return -1
	}
	for d := 0; d <= limit; d++ {
		if s == top {
			return d
		}
		s, _ = m.up(s, 1)
	}
	return limit + 1
}

// View draws the text area, one line per row, each padded to the pane
// width. The cursor is relative to the pane's top-left cell. It is nil
// while a command line is open: the cursor is on that line then.
func (m Model) View(th theme.Theme) (string, *tea.Cursor) {
	if len(m.fence) != m.e.Buf.LineCount() { // changed outside Update
		m.fence = fences(m.e.Buf)
	}
	styles := paletteFor(th)
	eob := th.Style(theme.UIEndOfBuffer)
	curSpot, curX := m.cursorSpot()
	rows := make([]string, 0, m.h)
	var c *tea.Cursor
	n := m.e.Buf.LineCount()
	for s := m.top; s.line < n && len(rows) < m.h; s.line, s.row = s.line+1, 0 {
		lay := m.lay(s.line)
		look := m.looks(s.line)
		for ; s.row < len(lay) && len(rows) < m.h; s.row++ {
			if s == curSpot {
				c = tea.NewCursor(curX-m.left, len(rows))
			}
			rows = append(rows, pad(drawRow(lay[s.row], m.left, m.w, styles, look), m.w))
		}
	}
	for len(rows) < m.h {
		rows = append(rows, pad(eob.Render("~"), m.w))
	}
	if c == nil || m.onCmdline() {
		return strings.Join(rows, "\n"), nil
	}
	c.Blink = false
	switch {
	case m.e.Mode == engine.Insert:
		c.Shape = tea.CursorBar
	case m.e.OperatorPending():
		c.Shape = tea.CursorUnderline
	}
	return strings.Join(rows, "\n"), c
}

// pad fills s with spaces to w cells.
func pad(s string, w int) string {
	if gap := w - ansi.StringWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// StatusLine is the w-cell status line: mode, file, [+] when modified,
// pending keys, and line:col on the right.
func (m Model) StatusLine(th theme.Theme, w int) string {
	bar := th.Style(theme.UIStatusline)
	modeSlot := theme.UIModeNormal
	switch m.e.Mode {
	case engine.Insert:
		modeSlot = theme.UIModeInsert
	case engine.Visual, engine.VisualLine:
		modeSlot = theme.UIModeVisual
	case engine.Command, engine.Search:
		modeSlot = theme.UIModeCommand
	}
	left := th.Style(modeSlot).Render(" "+m.e.Mode.String()+" ") +
		th.Style(theme.UIStatusFile).Render(" "+m.name)
	if m.e.Dirty {
		left += th.Style(theme.UIStatusDirty).Render(" [+]")
	}
	for _, t := range m.tags {
		left += th.Style(theme.UIStatusDirty).Render(" " + t)
	}
	if pk := m.e.PendingKeys(); pk != "" {
		left += bar.Render("  " + pk)
	}
	cur := m.e.Cur
	right := bar.Render(fmt.Sprintf("%d:%d ", cur.Line+1, engine.Cells(m.e.Buf.Line(cur.Line), cur.Col)+1))
	// When space runs short, line:col wins over the left side.
	room := w - ansi.StringWidth(right) - 1
	if room < 0 {
		return ansi.Truncate(right, w, "")
	}
	left = ansi.Truncate(left, room, "")
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	return left + bar.Render(strings.Repeat(" ", gap)) + right
}

// onCmdline reports whether the engine is reading a ":", "/" or "?" line.
func (m Model) onCmdline() bool {
	return m.e.Mode == engine.Command || m.e.Mode == engine.Search
}

// CmdLine is the ":", "/" or "?" line being typed, cut from the left to
// fit w cells so its end stays visible, with a bar cursor after it. The
// cursor is relative to the line's first cell. ok is false when no
// command line is open, and the host shows its message line instead.
func (m Model) CmdLine(th theme.Theme, w int) (line string, cursor *tea.Cursor, ok bool) {
	if !m.onCmdline() {
		return "", nil, false
	}
	prompt, text := m.e.CmdLine()
	s := prompt + text
	if over := ansi.StringWidth(s) - (w - 1); over > 0 {
		s = ansi.TruncateLeft(s, over, "")
	}
	c := tea.NewCursor(ansi.StringWidth(s), 0)
	c.Shape, c.Blink = tea.CursorBar, false
	return th.Style(theme.UIBase).Render(s), c, true
}
