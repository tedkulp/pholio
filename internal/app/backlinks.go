package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/palette"
)

// showBacklinks (spc b, :backlinks) opens a filtering list of the Links
// to the open Note, each with its line for context. enter jumps to one.
func (m Model) showBacklinks() (Model, tea.Cmd) {
	if m.index() == nil {
		return m.say("Backlinks need the Vault index", true), nil
	}
	m = m.syncIndex()
	rel := m.rel(m.path())
	p := palette.New("Backlinks to "+rel, palette.Type).
		WithPlaceholder("filter").
		WithEmpty("no Backlinks")
	p = m.backlinkItems(p)
	m = m.showPalette(p, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind != palette.Chosen || !ev.OK {
			return m, nil
		}
		b := ev.Item.Value.(index.Backlink)
		return m.openAt(m.abs(b.Source), func(m Model) Model {
			m.ed.Engine().SetCursor(engine.Pos{Line: b.Link.Line, Col: b.Link.Start})
			return m
		})
	})
	m.overlay.ready = func(m Model) Model {
		m.overlay.p = m.backlinkItems(m.overlay.p)
		return m
	}
	return m, nil
}

// backlinkItems fills p with the open Note's Backlinks, or says the index
// is still being built.
func (m Model) backlinkItems(p palette.Model) palette.Model {
	if !m.indexReady() {
		return p.WithEmpty("indexing…")
	}
	links := m.index().Backlinks(m.rel(m.path()))
	items := make([]palette.Item, len(links))
	for i, b := range links {
		items[i] = palette.Item{
			Text:   strings.TrimSpace(b.Context),
			Detail: fmt.Sprintf("%s:%d", b.Source, b.Link.Line+1),
			Value:  b,
		}
	}
	return p.WithEmpty("no Backlinks").SetItems(items)
}
