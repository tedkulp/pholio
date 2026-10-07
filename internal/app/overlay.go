package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/form"
	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

// Layer IDs, for hit testing (mouse) through Model.layers.
const (
	layerBackdrop = "backdrop"
	layerPalette  = "palette"
)

// paletteHandler acts on a palette's events. The app has already closed
// the palette for Closed and Chosen events; a handler may open another.
// For other events the palette stays open and m.overlay.p is current.
type paletteHandler func(m Model, ev palette.Event) (Model, tea.Cmd)

// formHandler acts on a form's Saved and Closed events. The app has
// already closed the form for Closed; for Saved it is still open, with
// m.overlay.form current, so a handler can refuse rows and keep it.
type formHandler func(m Model, ev form.Event) (Model, tea.Cmd)

// overlay is an open palette, or form, and the handler for its events.
type overlay struct {
	p  palette.Model
	on paletteHandler
	// ready, if set, refreshes the items once the Vault index is built,
	// for a palette opened while it said "indexing…".
	ready func(m Model) Model

	// form, when set, is shown instead of p, and onForm handles it.
	form   *form.Model
	onForm formHandler
}

// paletteMsg opens a palette from a tea.Cmd.
type paletteMsg struct {
	p  palette.Model
	on paletteHandler
}

// showPalette opens p over the panes. Pending leader keys are dropped.
func (m Model) showPalette(p palette.Model, on paletteHandler) Model {
	m.overlay = &overlay{p: p, on: on}
	m.leader = false
	return m
}

// showForm opens f over the panes. Pending leader keys are dropped.
func (m Model) showForm(f form.Model, on formHandler) Model {
	m.overlay = &overlay{form: &f, onForm: on}
	m.leader = false
	return m
}

// overlayKey gives a key to the open palette or form and its handler.
func (m Model) overlayKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if f := m.overlay.form; f != nil {
		return m.formEvent(f.Update(msg))
	}
	p, ev := m.overlay.p.Update(msg)
	return m.paletteEvent(p, ev)
}

func (m Model) overlayPaste(s string) (Model, tea.Cmd) {
	if f := m.overlay.form; f != nil {
		return m.formEvent(f.Paste(s))
	}
	p, ev := m.overlay.p.Paste(s)
	return m.paletteEvent(p, ev)
}

// formEvent stores the form a key left and passes Saved and Closed to
// its handler, closing the form first for Closed.
func (m Model) formEvent(f form.Model, ev form.Event) (Model, tea.Cmd) {
	on := m.overlay.onForm
	if ev.Kind == form.Closed {
		m.overlay = nil
	} else {
		m.overlay = &overlay{form: &f, onForm: on}
	}
	if ev.Kind == form.None || on == nil {
		return m, nil
	}
	return on(m, ev)
}

func (m Model) paletteEvent(p palette.Model, ev palette.Event) (Model, tea.Cmd) {
	on := m.overlay.on
	if ev.Kind == palette.Closed || ev.Kind == palette.Chosen {
		m.overlay = nil
	} else {
		m.overlay = &overlay{p: p, on: on, ready: m.overlay.ready}
	}
	if ev.Kind == palette.None || on == nil {
		return m, nil
	}
	return on(m, ev)
}

// layers stacks the screen: the panes dimmed as a backdrop, and the
// palette or form above them.
func (m Model) layers(th theme.Theme, base string) (*lipgloss.Compositor, *tea.Cursor) {
	dim := th.Style(theme.OverlayBackdrop)
	lines := strings.Split(base, "\n")
	for i, l := range lines {
		plain := ansi.Strip(l)
		lines[i] = dim.Render(plain + strings.Repeat(" ", max(0, m.w-ansi.StringWidth(plain))))
	}
	var box string
	var x, y int
	var cursor *tea.Cursor
	if f := m.overlay.form; f != nil {
		box, x, y, cursor = f.View(th, m.w, m.h)
	} else {
		box, x, y, cursor = m.overlay.p.View(th, m.w, m.h)
	}
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(strings.Join(lines, "\n")).ID(layerBackdrop),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1).ID(layerPalette),
	), cursor
}

// composeOverlay draws the palette over the dimmed screen.
func (m Model) composeOverlay(th theme.Theme, base string) (string, *tea.Cursor) {
	c, cursor := m.layers(th, base)
	return c.Render(), cursor
}
