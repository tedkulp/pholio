// Package form is a pop-up form drawn like a palette: a framed box with
// one labelled row per field, used by the Task Editor. Text rows are typed
// into; Choice rows cycle through their choices.
//
// Like a palette, a form never acts on its own. Update turns each key into
// an Event, and the host decides what Saved means, marking rows it
// refuses with WithInvalid.
package form

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

// Kind is how a row is edited.
type Kind int

const (
	// Text rows take typed text.
	Text Kind = iota
	// Choice rows cycle through Choices with space, ← and →.
	Choice
)

// Field is one row.
type Field struct {
	Label string
	Kind  Kind
	// Value is the text, or for a Choice one of Choices.
	Value string
	// Choices are a Choice row's values, in cycling order.
	Choices []string
	// Placeholder is shown in an empty Text row.
	Placeholder string
}

// EventKind is what a key meant.
type EventKind int

// Events.
const (
	None   EventKind = iota // nothing for the host to do
	Saved                   // enter: Event.Values holds every row's value
	Closed                  // esc: the host should close the form
)

// Event is the outcome of one key.
type Event struct {
	Kind   EventKind
	Values []string
}

// Model is a form. It is a value: use the Model that methods return.
type Model struct {
	title   string
	hint    string
	fields  []Field
	focus   int
	pos     int // the cursor in the focused Text row, in runes
	invalid []bool
}

// New returns a form with the given rows, the first focused.
func New(title string, fields ...Field) Model {
	m := Model{title: title, fields: slices.Clone(fields), invalid: make([]bool, len(fields))}
	m.hint = "tab next · enter save · esc close"
	return m.focusRow(0)
}

// WithHint sets the footer line, which lists the keys.
func (m Model) WithHint(s string) Model { m.hint = s; return m }

// WithInvalid marks rows as refused, drawing them in the error style until
// they are edited, and clears the marks on the other rows.
func (m Model) WithInvalid(rows ...int) Model {
	m.invalid = make([]bool, len(m.fields))
	for _, r := range rows {
		if r >= 0 && r < len(m.fields) {
			m.invalid[r] = true
		}
	}
	return m
}

// Invalid reports whether row i is marked refused.
func (m Model) Invalid(i int) bool { return i >= 0 && i < len(m.invalid) && m.invalid[i] }

// Focus is the focused row.
func (m Model) Focus() int { return m.focus }

// Values are the rows' values, in order.
func (m Model) Values() []string {
	out := make([]string, len(m.fields))
	for i, f := range m.fields {
		out[i] = f.Value
	}
	return out
}

// focusRow focuses row i, with the cursor at the end of a Text row.
func (m Model) focusRow(i int) Model {
	if len(m.fields) == 0 {
		return m
	}
	m.focus = (i + len(m.fields)) % len(m.fields)
	m.pos = len([]rune(m.fields[m.focus].Value))
	return m
}

// Update handles one key press.
func (m Model) Update(msg tea.KeyPressMsg) (Model, Event) {
	name := keyName(msg)
	switch name {
	case "tab", "down":
		return m.focusRow(m.focus + 1), Event{}
	case "shift+tab", "up":
		return m.focusRow(m.focus - 1), Event{}
	case "enter":
		return m, Event{Kind: Saved, Values: m.Values()}
	case "esc", "ctrl+c":
		return m, Event{Kind: Closed}
	}
	if len(m.fields) == 0 {
		return m, Event{}
	}
	if m.fields[m.focus].Kind == Choice {
		switch name {
		case " ", "right", "l":
			return m.cycle(1), Event{}
		case "left", "h":
			return m.cycle(-1), Event{}
		}
		return m, Event{}
	}
	r := []rune(m.fields[m.focus].Value)
	switch {
	case name == "left" || name == "ctrl+b":
		m.pos = max(0, m.pos-1)
	case name == "right" || name == "ctrl+f":
		m.pos = min(len(r), m.pos+1)
	case name == "home" || name == "ctrl+a":
		m.pos = 0
	case name == "end" || name == "ctrl+e":
		m.pos = len(r)
	case name == "backspace":
		if m.pos > 0 {
			m = m.set(string(r[:m.pos-1])+string(r[m.pos:]), m.pos-1)
		}
	case name == "delete" || name == "ctrl+d":
		if m.pos < len(r) {
			m = m.set(string(r[:m.pos])+string(r[m.pos+1:]), m.pos)
		}
	case name == "ctrl+u":
		m = m.set(string(r[m.pos:]), 0)
	case msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt) == 0:
		m = m.insert(msg.Text)
	}
	return m, Event{}
}

// Paste types s into a focused Text row, on one line.
func (m Model) Paste(s string) (Model, Event) {
	if len(m.fields) == 0 || m.fields[m.focus].Kind != Text {
		return m, Event{}
	}
	return m.insert(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)), Event{}
}

// insert puts s at the cursor of the focused Text row.
func (m Model) insert(s string) Model {
	r := []rune(m.fields[m.focus].Value)
	return m.set(string(r[:m.pos])+s+string(r[m.pos:]), m.pos+len([]rune(s)))
}

// set changes the focused row's value, moving the cursor to pos, and
// clears the row's invalid mark.
func (m Model) set(v string, pos int) Model {
	m.fields = slices.Clone(m.fields)
	m.fields[m.focus].Value = v
	m.invalid = slices.Clone(m.invalid)
	m.invalid[m.focus] = false
	m.pos = pos
	return m
}

// cycle moves the focused Choice row n choices along, wrapping.
func (m Model) cycle(n int) Model {
	f := m.fields[m.focus]
	if len(f.Choices) == 0 {
		return m
	}
	i := max(0, slices.Index(f.Choices, f.Value))
	i = ((i+n)%len(f.Choices) + len(f.Choices)) % len(f.Choices)
	return m.set(f.Choices[i], 0)
}

// keyName is a key's name: the typed text for plain printable keys,
// else its keystroke ("tab", "ctrl+u").
func keyName(msg tea.KeyPressMsg) string {
	k := msg.Key()
	if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return k.Text
	}
	return msg.Keystroke()
}

// labelW is the width of the label column: the longest label and a gap.
func (m Model) labelW() int {
	w := 0
	for _, f := range m.fields {
		w = max(w, ansi.StringWidth(f.Label))
	}
	return w + 2
}

// View draws the form for a w×h screen where a palette would sit (see
// palette.Geometry). It returns the box, its screen position and the
// cursor in screen cells (nil unless a Text row has focus).
func (m Model) View(th theme.Theme, w, h int) (box string, x, y int, cursor *tea.Cursor) {
	x, y, boxW, _ := palette.Geometry(w, h)
	inner := max(1, boxW-4)
	lw := m.labelW()
	var lines []string
	for i, f := range m.fields {
		label := th.Style(theme.OverlayHint).Render(fitW(f.Label, lw))
		if i == m.focus {
			label = th.Style(theme.OverlayTitle).Render(fitW(f.Label, lw))
		}
		lines = append(lines, label+m.drawValue(th, i))
	}
	if m.hint != "" {
		lines = append(lines, th.Style(theme.OverlayBorder).Render(strings.Repeat("─", inner)),
			th.Style(theme.OverlayHint).Render(m.hint))
	}
	box = palette.Frame(th, m.title, lines, inner)
	if len(m.fields) > 0 && m.fields[m.focus].Kind == Text {
		before := string([]rune(m.fields[m.focus].Value)[:m.pos])
		cursor = tea.NewCursor(x+2+lw+ansi.StringWidth(before), y+1+m.focus)
		cursor.Shape = tea.CursorBar
		cursor.Blink = false
	}
	return box, x, y, cursor
}

// drawValue draws row i's value: a Text row's text (or placeholder), or a
// Choice row's value between arrows, selected when focused.
func (m Model) drawValue(th theme.Theme, i int) string {
	f := m.fields[i]
	slot := theme.OverlayInput
	if m.invalid[i] {
		slot = theme.UIError
	}
	if f.Kind == Choice {
		if i == m.focus {
			slot = theme.OverlaySelected
		}
		return th.Style(slot).Render("‹ " + f.Value + " ›")
	}
	if f.Value == "" && f.Placeholder != "" {
		return th.Style(theme.OverlayPlaceholder).Render(f.Placeholder)
	}
	return th.Style(slot).Render(f.Value)
}

// fitW pads s to w cells.
func fitW(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
