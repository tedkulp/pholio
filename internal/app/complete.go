package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/theme"
)

// Popup limits.
const (
	completeLimit = 50 // matches kept
	completeRows  = 8  // rows shown
	completeWidth = 48 // most cells wide
)

// completion is the [[ popup: Notes fuzzy-matched against the text typed
// after the [[, which stays in the buffer as it is typed.
type completion struct {
	start engine.Pos // just after the [[
	query string
	ready bool // the index had scanned when notes was computed
	notes []index.Note
	sel   int
}

// lines is the popup's rows as plain text.
func (c *completion) lines() []string {
	switch {
	case !c.ready:
		return []string{indexing}
	case len(c.notes) == 0:
		return []string{"no matches"}
	}
	out := make([]string, len(c.notes))
	for i, n := range c.notes {
		out[i] = strings.TrimSpace(n.Name + "  " + n.Title)
	}
	return out
}

// maybeComplete opens the popup when the key just typed in insert mode
// finished a [[. It takes editorKey's results.
func maybeComplete(m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	e := m.ed.Engine()
	if m.complete != nil || e.Mode != engine.Insert || e.Cur.Col < 2 {
		return m, cmd
	}
	before := e.Buf.Line(e.Cur.Line)[:e.Cur.Col]
	if !strings.HasSuffix(before, "[[") || strings.HasSuffix(before, "[[[") {
		return m, cmd
	}
	m.complete = &completion{start: e.Cur}
	return m.syncCompletion(true), cmd
}

// completeKey handles a key while the popup is open: arrows and ctrl+n/p
// move, enter and tab accept, esc closes it, and everything else is typed.
func (m Model) completeKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	c := *m.complete
	switch keyName(msg) {
	case "up", "ctrl+p":
		c.sel = max(0, c.sel-1)
	case "down", "ctrl+n":
		c.sel = max(0, min(len(c.notes)-1, c.sel+1))
	case "esc":
		m.complete = nil
		return m, nil
	case "enter", "tab":
		if c.sel < len(c.notes) {
			return m.acceptCompletion(c.notes[c.sel]), nil
		}
		m.complete = nil
		return m.editorKey(msg)
	default:
		var cmd tea.Cmd
		m, cmd = m.editorKey(msg)
		return m.syncCompletion(false), cmd
	}
	m.complete = &c
	return m, nil
}

// syncCompletion follows the buffer: the query is the text from the [[ to
// the cursor. The popup closes when the cursor leaves it, insert mode ends
// or the Link is closed. force recomputes the matches.
func (m Model) syncCompletion(force bool) Model {
	if m.complete == nil {
		return m
	}
	c := *m.complete
	e := m.ed.Engine()
	line := ""
	if e.Cur.Line < e.Buf.LineCount() {
		line = e.Buf.Line(e.Cur.Line)
	}
	if e.Mode != engine.Insert || e.Cur.Line != c.start.Line || e.Cur.Col < c.start.Col ||
		c.start.Col > len(line) || line[c.start.Col-2:c.start.Col] != "[[" {
		m.complete = nil
		return m
	}
	query := line[c.start.Col:e.Cur.Col]
	if strings.ContainsAny(query, "]|#") {
		m.complete = nil
		return m
	}
	if force || query != c.query || c.ready != m.indexReady() {
		c.query, c.ready, c.notes, c.sel = query, m.indexReady(), nil, 0
		if c.ready {
			notes := slices.DeleteFunc(m.index().Notes(), func(n index.Note) bool { return n.Conflict })
			c.notes = rankNotes(query, notes, completeLimit)
		}
	}
	m.complete = &c
	return m
}

// acceptCompletion replaces the query with the Note's filename and closes
// the Link. When another Note of the same name would win, the path is
// used instead, so the Link goes where it was meant to.
func (m Model) acceptCompletion(n index.Note) Model {
	e := m.ed.Engine()
	target := n.Name
	if r, ok := m.index().Resolve(m.rel(m.path()), index.Link{Kind: index.WikiLink, Target: n.Name}); !ok || r.Path != n.Path {
		target = index.TrimNoteExt(n.Path)
	}
	for e.Cur.Col > m.complete.start.Col {
		e.Feed("backspace")
	}
	e.Paste(target)
	if strings.HasPrefix(e.Buf.Line(e.Cur.Line)[e.Cur.Col:], "]]") {
		e.Feed("right")
		e.Feed("right")
	} else {
		e.Paste("]]")
	}
	m.complete = nil
	return m
}

// drawCompletion layers the popup over the screen, under the cursor and
// aligned with the text typed after the [[ (above it near the bottom).
func (m Model) drawCompletion(th theme.Theme, content string, cursor *tea.Cursor) string {
	c := m.complete
	lines := c.lines()
	top := max(0, min(c.sel-completeRows/2, len(lines)-completeRows))
	lines = lines[top:min(len(lines), top+completeRows)]
	w := 0
	for _, l := range lines {
		w = max(w, ansi.StringWidth(l))
	}
	w = min(w+2, completeWidth, m.w)
	var rows []string
	for i, l := range lines {
		text := " " + l
		if c.ready && len(c.notes) > 0 {
			n := c.notes[top+i]
			text = " " + n.Name
			if n.Title != "" {
				text += "  " + th.Style(theme.OverlayHint).Render(n.Title)
			}
		}
		text = ansi.Truncate(text, w, "…")
		text += strings.Repeat(" ", max(0, w-ansi.StringWidth(text)))
		slot := theme.OverlayBox
		switch {
		case !c.ready || len(c.notes) == 0:
			slot = theme.OverlayHint
		case top+i == c.sel:
			slot = theme.OverlaySelected
			text = ansi.Strip(text)
		}
		rows = append(rows, th.Style(slot).Render(text))
	}
	x := max(0, min(cursor.X-ansi.StringWidth(c.query), m.w-w))
	y := cursor.Y + 1
	if y+len(rows) > m.h-2 { // no room below: above the cursor
		y = max(0, cursor.Y-len(rows))
	}
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(content),
		lipgloss.NewLayer(strings.Join(rows, "\n")).X(x).Y(y).Z(1),
	).Render()
}
