package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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

// overlay is an open palette and the handler for its events.
type overlay struct {
	p  palette.Model
	on paletteHandler
	// ready, if set, refreshes the items once the Vault index is built,
	// for a palette opened while it said "indexing…".
	ready func(m Model) Model
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

// overlayKey gives a key to the open palette and its handler.
func (m Model) overlayKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	p, ev := m.overlay.p.Update(msg)
	return m.paletteEvent(p, ev)
}

func (m Model) overlayPaste(s string) (Model, tea.Cmd) {
	p, ev := m.overlay.p.Paste(s)
	return m.paletteEvent(p, ev)
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
// palette above them.
func (m Model) layers(th theme.Theme, base string) (*lipgloss.Compositor, *tea.Cursor) {
	dim := th.Style(theme.OverlayBackdrop)
	lines := strings.Split(base, "\n")
	for i, l := range lines {
		plain := ansi.Strip(l)
		lines[i] = dim.Render(plain + strings.Repeat(" ", max(0, m.w-ansi.StringWidth(plain))))
	}
	box, x, y, cursor := m.overlay.p.View(th, m.w, m.h)
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
