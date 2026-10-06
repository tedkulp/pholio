package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/sidebar"
)

// mouseMode is the mouse mode the view asks for: cell motion (clicks,
// the wheel and drags) unless the config says mouse = false.
func (m Model) mouseMode() tea.MouseMode {
	if m.noMouse {
		return tea.MouseModeNone
	}
	return tea.MouseModeCellMotion
}

// region is the part of the screen a mouse event lands on.
type region int

const (
	regionNone    region = iota // the status and message lines
	regionSidebar               // the tree, header included
	regionBorder                // the sidebar's right border: a drag handle
	regionEditor
)

// regionAt says what is at screen cell x, y.
func (m Model) regionAt(x, y int) region {
	if y < 0 || y >= m.h-2 {
		return regionNone
	}
	sw := m.sidebarWidth()
	switch {
	case x < sw-1:
		return regionSidebar
	case x == sw-1:
		return regionBorder
	}
	return regionEditor
}

// mouse routes one mouse event. While a question is on the message line
// the mouse does nothing.
func (m Model) mouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if _, ok := msg.(tea.MouseReleaseMsg); ok && m.drag {
		m.drag = false
		return m.resizeSidebar(0), nil // saves the dragged width
	}
	if m.prompt != nil || m.confirming {
		return m, nil
	}
	if m.overlay != nil {
		return m.overlayMouse(msg)
	}
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return m.click(msg.Mouse())
	case tea.MouseWheelMsg:
		return m.wheel(msg.Mouse()), nil
	case tea.MouseMotionMsg:
		if m.drag {
			m.sideW = max(sidebarMin, min(msg.X+1, sidebarMax, m.w-minEditor))
		}
	}
	return m, nil
}

// wheelRows is how far one wheel notch scrolls.
const wheelRows = 3

// wheel scrolls the pane under the pointer; focus stays where it is.
func (m Model) wheel(e tea.Mouse) Model {
	n := 0
	switch e.Button {
	case tea.MouseWheelUp:
		n = -wheelRows
	case tea.MouseWheelDown:
		n = wheelRows
	default:
		return m
	}
	switch m.regionAt(e.X, e.Y) {
	case regionSidebar:
		m.side = m.side.Scroll(n)
	case regionEditor:
		m.ed = m.ed.Scroll(n)
		m = m.syncCompletion(false) // closes when the cursor was pulled away
	}
	return m
}

// click handles a button press. A click drops a pending leader or key
// sequence, as another key would.
func (m Model) click(e tea.Mouse) (Model, tea.Cmd) {
	if e.Button != tea.MouseLeft {
		return m, nil
	}
	r := m.regionAt(e.X, e.Y)
	if r == regionNone {
		return m, nil
	}
	m.leader, m.pending, m.complete = false, nil, nil
	m.message, m.problem = "", false
	switch r {
	case regionSidebar:
		m.focus = focusSidebar
		return m.sidebarClick(e.Y)
	case regionBorder:
		m.drag = true
	case regionEditor:
		m.focus = focusEditor
		ed := m.ed.Engine()
		switch {
		case ed.Mode == engine.Command || ed.Mode == engine.Search:
			return m, nil // the cursor is on the command line
		case ed.Mode == engine.Normal && ed.PendingKeys() != "":
			ed.Feed("esc") // drop a half-typed command such as d
		}
		m.ed = m.ed.Click(e.X-m.sidebarWidth(), e.Y)
		if e.Mod.Contains(tea.ModCtrl) {
			return m.goToLink()
		}
	}
	return m, nil
}

// overlayMouse handles the mouse while a palette is open: a click on a
// row chooses it, a click on the backdrop closes the palette and the wheel
// over the palette moves its selection. Nothing reaches the panes.
func (m Model) overlayMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	e := msg.Mouse()
	blank := strings.Repeat("\n", max(0, m.h-1))
	c, _ := m.layers(m.appearance.theme, blank)
	hit := c.Hit(e.X, e.Y).ID()
	p := m.overlay.p
	switch msg.(type) {
	case tea.MouseClickMsg:
		if e.Button != tea.MouseLeft {
			return m, nil
		}
		switch hit {
		case layerBackdrop:
			return m.paletteEvent(p, palette.Event{Kind: palette.Closed, Index: -1, Query: p.Query()})
		case layerPalette:
			return m.paletteEvent(p.Click(m.w, m.h, e.Y))
		}
	case tea.MouseWheelMsg:
		if hit != layerPalette {
			return m, nil
		}
		o := *m.overlay
		switch e.Button {
		case tea.MouseWheelUp:
			o.p = p.Scroll(-1)
		case tea.MouseWheelDown:
			o.p = p.Scroll(1)
		}
		m.overlay = &o
	}
	return m, nil
}

// sidebarClick acts on the tree row at screen row y.
func (m Model) sidebarClick(y int) (Model, tea.Cmd) {
	var ev sidebar.Event
	m.side, ev = m.side.Click(y)
	return m.sidebarEvent(ev)
}
