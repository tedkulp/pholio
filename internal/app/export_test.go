package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/palette"
)

// OpenPalette is a message that opens p, as a feature's tea.Cmd would.
// Each event the palette produces is passed to record.
func OpenPalette(p palette.Model, record func(palette.Event)) tea.Msg {
	return paletteMsg{p: p, on: func(m Model, ev palette.Event) (Model, tea.Cmd) {
		record(ev)
		return m, nil
	}}
}
