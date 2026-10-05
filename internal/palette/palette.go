// Package palette is the top-anchored overlay used by the Task List, find
// Note, search, Backlinks and the Zettel prompt: a framed box with a text
// input at the top and a filtered, selectable list below it.
//
// A palette never acts on its own. Update turns each key into an Event and
// the host decides what Chosen, Closed and unhandled keys mean.
package palette

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/theme"
)

// Mode is how keys are read.
type Mode int

const (
	// Type is ready-to-type: the input has focus from the start, every
	// printable key goes into the query, and esc closes.
	Type Mode = iota
	// List starts with the input unfocused. Letters reach the host as Key
	// events (the Task List binds space, a and D), "/" focuses the input,
	// and esc in the input returns to the list, keeping the query.
	List
)

// Item is one selectable row.
type Item struct {
	Text   string // the row's main text
	Detail string // right-aligned, drawn with overlay.hint (a path:line)
	Group  string // a header row is drawn whenever this changes
	// Slot styles Text when the row is not selected. Zero means overlay.box.
	Slot theme.Slot
	// Value is the host's payload, such as a path or a Task.
	Value any
}

// Matcher returns the indexes of the items that match query, in the order
// to show them. The default is a case-insensitive substring match on Text
// and Detail that keeps the items' order.
type Matcher func(query string, items []Item) []int

// EventKind is what a key meant.
type EventKind int

// Events.
const (
	None    EventKind = iota
	Closed            // esc: the host should close the palette
	Chosen            // enter: Event.Item (if any) and Event.Query
	Changed           // the query changed (debounced search refreshes here)
	Key               // a key the palette does not handle, in List mode
)

// Event is the outcome of one key.
type Event struct {
	Kind  EventKind
	Item  Item
	Index int // index into the items given to SetItems; -1 when none
	OK    bool
	Query string
	Key   string // for Key events: the key name
}

// Model is a palette. It is a value: use the Model that methods return.
type Model struct {
	title       string
	mode        Mode
	typing      bool // the input has focus
	placeholder string
	hint        string
	empty       string
	info        func(query string) []string
	match       Matcher

	query   string
	items   []Item
	visible []int // indexes into items, after filtering
	sel     int   // index into visible
}

// New returns an empty palette with the given title.
func New(title string, mode Mode) Model {
	m := Model{title: title, mode: mode, typing: mode == Type, match: Substring}
	m.hint = "↑/↓ move · enter choose · esc close"
	return m.refilter()
}

// WithPlaceholder sets the text shown in an empty input.
func (m Model) WithPlaceholder(s string) Model { m.placeholder = s; return m }

// WithHint sets the footer line, which lists the keys.
func (m Model) WithHint(s string) Model { m.hint = s; return m }

// WithEmpty sets what to show when no item matches, such as "indexing…".
func (m Model) WithEmpty(s string) Model { m.empty = s; return m }

// WithInfo sets lines drawn under the input, computed from the query.
// The Zettel prompt previews its file name this way.
func (m Model) WithInfo(f func(query string) []string) Model { m.info = f; return m }

// WithMatcher replaces the default substring filter.
func (m Model) WithMatcher(f Matcher) Model { m.match = f; return m.refilter() }

// WithQuery sets the query, as when a palette opens prefilled.
func (m Model) WithQuery(q string) Model { m.query = q; return m.refilter() }

// SetItems replaces the items, keeping the query. The selection moves to
// the first row.
func (m Model) SetItems(items []Item) Model {
	m.items = items
	m.sel = 0
	return m.refilter()
}

// Query is the text typed so far.
func (m Model) Query() string { return m.query }

// Typing reports whether keys go into the input.
func (m Model) Typing() bool { return m.typing }

// Selected is the item under the selection.
func (m Model) Selected() (item Item, index int, ok bool) {
	if m.sel < len(m.visible) {
		i := m.visible[m.sel]
		return m.items[i], i, true
	}
	return Item{}, -1, false
}

// Substring is the default Matcher.
func Substring(query string, items []Item) []int {
	q := strings.ToLower(query)
	var out []int
	for i, it := range items {
		if strings.Contains(strings.ToLower(it.Text+" "+it.Detail), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) refilter() Model {
	if m.query == "" {
		m.visible = make([]int, len(m.items))
		for i := range m.items {
			m.visible[i] = i
		}
	} else {
		m.visible = m.match(m.query, m.items)
	}
	m.sel = max(0, min(m.sel, len(m.visible)-1))
	return m
}

// Update handles one key press.
func (m Model) Update(msg tea.KeyPressMsg) (Model, Event) {
	name := keyName(msg)
	switch name {
	case "up", "ctrl+p":
		m.sel = max(0, m.sel-1)
		return m, Event{}
	case "down", "ctrl+n":
		m.sel = min(len(m.visible)-1, m.sel+1)
		m.sel = max(0, m.sel)
		return m, Event{}
	case "enter":
		it, i, ok := m.Selected()
		return m, Event{Kind: Chosen, Item: it, Index: i, OK: ok, Query: m.query}
	case "esc", "ctrl+c":
		if m.mode == List && m.typing && name == "esc" {
			m.typing = false
			return m, Event{}
		}
		return m, Event{Kind: Closed, Index: -1, Query: m.query}
	}
	if !m.typing {
		switch name {
		case "j":
			return m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		case "k":
			return m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		case "/":
			m.typing = true
			return m, Event{}
		}
		return m, Event{Kind: Key, Key: name, Index: -1, Query: m.query}
	}
	q := m.query
	switch {
	case name == "backspace":
		if r := []rune(q); len(r) > 0 {
			q = string(r[:len(r)-1])
		}
	case name == "ctrl+u":
		q = ""
	case msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt) == 0:
		q += msg.Text
	default:
		return m, Event{}
	}
	return m.typed(q)
}

// Paste adds pasted text to the query when the input has focus.
func (m Model) Paste(s string) (Model, Event) {
	if !m.typing {
		return m, Event{}
	}
	return m.typed(m.query + strings.ReplaceAll(s, "\n", " "))
}

func (m Model) typed(q string) (Model, Event) {
	if q == m.query {
		return m, Event{}
	}
	m.query, m.sel = q, 0
	m = m.refilter()
	return m, Event{Kind: Changed, Index: -1, Query: q}
}

func keyName(msg tea.KeyPressMsg) string {
	k := msg.Key()
	if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return k.Text
	}
	return msg.Keystroke()
}

// Geometry is where a palette sits on a screen of w×h cells: centred, one
// row from the top, at most 80 wide and 18 tall, clear of the bottom row.
func Geometry(w, h int) (x, y, boxW, maxH int) {
	boxW = min(80, max(20, w-10), w)
	return max(0, (w-boxW)/2), min(1, max(0, h-4)), boxW, max(3, min(18, h-2))
}

// View draws the palette for a w×h screen. It returns the box, its screen
// position and the input cursor in screen cells (nil when the input does
// not have focus).
func (m Model) View(th theme.Theme, w, h int) (box string, x, y int, cursor *tea.Cursor) {
	x, y, boxW, _ := Geometry(w, h)
	inner := max(1, boxW-4) // "│ " and " │"
	sh := m.shape(h)

	in := th.Style(theme.OverlayInput).Render(m.query)
	if m.query == "" && m.placeholder != "" {
		in = th.Style(theme.OverlayPlaceholder).Render(m.placeholder)
	}
	lines := []string{"> " + in}
	for _, l := range sh.info {
		lines = append(lines, th.Style(theme.OverlayHint).Render(l))
	}
	if sh.rule {
		lines = append(lines, th.Style(theme.OverlayBorder).Render(strings.Repeat("─", inner)))
	}
	for _, r := range sh.rows {
		lines = append(lines, m.drawRow(th, r, inner))
	}
	if m.hint != "" {
		lines = append(lines, th.Style(theme.OverlayHint).Render(m.hint))
	}
	box = m.frame(th, lines, inner)
	if m.typing {
		cursor = tea.NewCursor(x+2+2+ansi.StringWidth(m.query), y+1)
		cursor.Shape = tea.CursorBar
		cursor.Blink = false
	}
	return box, x, y, cursor
}

// listRow is one row of the list: an item (vi, an index into visible), a
// group header, a gap before a header, or the empty-list text.
type listRow struct {
	vi    int // -1 when the row is not an item
	group string
	empty bool
}

// shape is what the box holds on a screen h rows tall, below its top
// border: the input line, the info lines, the rule under them (absent for
// a pure prompt), the list rows that fit, then the hint.
type shape struct {
	info []string
	rule bool
	rows []listRow
}

func (m Model) shape(h int) shape {
	_, _, _, maxH := Geometry(0, h)
	var sh shape
	if m.info != nil {
		sh.info = m.info(m.query)
	}
	if len(m.items) == 0 && m.empty == "" && m.mode == Type {
		return sh // a pure prompt, like the Zettel title
	}
	sh.rule = true
	foot := 0
	if m.hint != "" {
		foot = 1
	}
	rows, selRow := m.list()
	avail := max(1, maxH-2-(2+len(sh.info))-foot)
	if len(rows) > avail {
		start := max(0, min(selRow-avail/2, len(rows)-avail))
		rows = rows[start : start+avail]
	}
	sh.rows = rows
	return sh
}

// list is every row of the list, group headers included, and the
// selected row.
func (m Model) list() (rows []listRow, selRow int) {
	group := ""
	for vi, i := range m.visible {
		it := m.items[i]
		if it.Group != "" && (vi == 0 || it.Group != group) {
			if len(rows) > 0 {
				rows = append(rows, listRow{vi: -1})
			}
			rows = append(rows, listRow{vi: -1, group: it.Group})
		}
		group = it.Group
		if vi == m.sel {
			selRow = len(rows)
		}
		rows = append(rows, listRow{vi: vi})
	}
	if len(m.visible) == 0 {
		rows = append(rows, listRow{vi: -1, empty: true})
	}
	return rows, selRow
}

// drawRow draws one list row w cells wide.
func (m Model) drawRow(th theme.Theme, r listRow, w int) string {
	switch {
	case r.vi >= 0:
		return row(th, m.items[m.visible[r.vi]], w, r.vi == m.sel)
	case r.group != "":
		return th.Style(theme.OverlayGroup).Render(fit(r.group, w))
	case r.empty:
		text := m.empty
		if text == "" {
			text = "no matches"
		}
		return th.Style(theme.OverlayHint).Render(fit(text, w))
	}
	return ""
}

// Click acts on a click on screen row y, inside the box, on a w×h screen:
// an item's row is chosen, as enter chooses it. Other rows do nothing.
func (m Model) Click(w, h, y int) (Model, Event) {
	_, top, _, _ := Geometry(w, h)
	sh := m.shape(h)
	r := y - top - 2 - len(sh.info) - 1 // the border, input, info and rule
	if !sh.rule || r < 0 || r >= len(sh.rows) || sh.rows[r].vi < 0 {
		return m, Event{}
	}
	m.sel = sh.rows[r].vi
	it, i, ok := m.Selected()
	return m, Event{Kind: Chosen, Item: it, Index: i, OK: ok, Query: m.query}
}

// Scroll moves the selection n rows down (up when n < 0), as the mouse
// wheel does; the list scrolls to keep it in view.
func (m Model) Scroll(n int) Model {
	m.sel = max(0, min(m.sel+n, len(m.visible)-1))
	return m
}

func row(th theme.Theme, it Item, w int, selected bool) string {
	dw := ansi.StringWidth(it.Detail)
	if dw > w/2 {
		dw = w / 2
	}
	text := it.Text
	gap := 0
	if dw > 0 {
		gap = 1
	}
	text = fit(text, w-dw-gap)
	detail := ansi.Truncate(it.Detail, dw, "…")
	if selected {
		return th.Style(theme.OverlaySelected).Render(fit(text+strings.Repeat(" ", gap)+detail, w))
	}
	slot := it.Slot
	if slot == "" {
		slot = theme.OverlayBox
	}
	return th.Style(slot).Render(text) + strings.Repeat(" ", gap) + th.Style(theme.OverlayHint).Render(detail)
}

// frame draws a rounded border around lines with the title in the top edge.
func (m Model) frame(th theme.Theme, lines []string, inner int) string {
	bs, box := th.Style(theme.OverlayBorder), th.Style(theme.OverlayBox)
	full := inner + 2
	title := " " + m.title + " "
	if m.title == "" {
		title = ""
	}
	title = ansi.Truncate(title, max(0, full-1), "…")
	out := []string{bs.Render("╭─") + th.Style(theme.OverlayTitle).Render(title) +
		bs.Render(strings.Repeat("─", max(0, full-1-ansi.StringWidth(title)))+"╮")}
	for _, l := range lines {
		l = ansi.Truncate(l, inner, "…")
		pad := box.Render(" " + strings.Repeat(" ", max(0, inner-ansi.StringWidth(l))))
		out = append(out, bs.Render("│")+box.Render(" ")+l+pad+bs.Render("│"))
	}
	out = append(out, bs.Render("╰"+strings.Repeat("─", full)+"╯"))
	return strings.Join(out, "\n")
}

// fit truncates s to w cells with an ellipsis and pads it to w.
func fit(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
