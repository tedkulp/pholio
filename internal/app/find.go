package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/fuzzy"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/palette"
)

// findLimit is how many matches find Note lists.
const findLimit = 50

// indexing is shown wherever a feature waits for the startup scan.
const indexing = "indexing…"

// newNote is the "+ new Note" row's value: the query to name it after.
type newNote string

// rankNotes fuzzy-matches query against notes' filenames and titles (and
// paths, so "c/dup" picks one of two dup.md), best first, at most limit.
// Ties go to the shorter path.
func rankNotes(query string, notes []index.Note, limit int) []index.Note {
	notes = slices.Clone(notes)
	slices.SortStableFunc(notes, func(a, b index.Note) int { return len(a.Path) - len(b.Path) })
	ranked := fuzzy.Rank(query, len(notes), func(i int) []string {
		n := notes[i]
		return []string{n.Name, n.Title, index.TrimNoteExt(n.Path)}
	})
	out := make([]index.Note, 0, min(limit, len(ranked)))
	for _, r := range ranked[:min(limit, len(ranked))] {
		out = append(out, notes[r.Index])
	}
	return out
}

// remember puts path at the front of the session's recent Notes.
func remember(recent []string, path string) []string {
	if path == "" {
		return recent
	}
	recent = slices.DeleteFunc(slices.Clone(recent), func(p string) bool { return p == path })
	return append([]string{path}, recent...)
}

// findNote (spc f, :find) opens the find Note palette, prefilled with arg.
func (m Model) findNote(arg string) (Model, tea.Cmd) {
	query := strings.TrimSpace(arg)
	p := palette.New("Find Note", palette.Type).
		WithPlaceholder("Note name or title").
		WithMatcher(func(_ string, items []palette.Item) []int { // the app ranks
			out := make([]int, len(items))
			for i := range out {
				out[i] = i
			}
			return out
		}).
		WithQuery(query)
	m = m.showPalette(p, onFind)
	m.overlay.ready = refreshFind
	return refreshFind(m), nil
}

// refreshFind recomputes the open find palette's rows for its query.
func refreshFind(m Model) Model {
	items, empty := m.findItems(m.overlay.p.Query())
	m.overlay.p = m.overlay.p.SetItems(items).WithEmpty(empty)
	return m
}

func onFind(m Model, ev palette.Event) (Model, tea.Cmd) {
	switch ev.Kind {
	case palette.Changed:
		return refreshFind(m), nil
	case palette.Chosen:
		if !ev.OK {
			return m, nil
		}
		switch v := ev.Item.Value.(type) {
		case newNote:
			return m.dangling(string(v))
		case string:
			return m.openNote(v)
		}
	}
	return m, nil
}

// findItems lists the Notes for query. An empty query lists the Notes
// opened this session, newest first, leaving out the open one. When
// nothing matches, the only row offers a new Note named after the query.
// empty is what to show when there are no rows.
func (m Model) findItems(query string) (items []palette.Item, empty string) {
	query = strings.TrimSpace(query)
	if query == "" {
		for _, p := range m.recent {
			it, ok := m.noteItem(p)
			if ok && p != m.path() {
				items = append(items, it)
			}
		}
		if len(items) == 0 && !m.indexReady() {
			return nil, indexing
		}
		return items, "no Notes opened yet"
	}
	if !m.indexReady() {
		return nil, indexing
	}
	for _, n := range rankNotes(query, m.index().Notes(), findLimit) {
		items = append(items, palette.Item{Text: n.Path, Detail: n.Title, Value: m.abs(n.Path)})
	}
	if len(items) == 0 {
		items = []palette.Item{{Text: `+ new Note "` + query + `"`, Value: newNote(query)}}
	}
	return items, ""
}

// noteItem is a find row for the Note at the OS path p. ok is false once
// the index is ready and doesn't know the Note: it was never written, or it
// has gone.
func (m Model) noteItem(p string) (it palette.Item, ok bool) {
	rel := m.rel(p)
	it = palette.Item{Text: rel, Value: p}
	if !m.indexReady() {
		return it, true
	}
	n, ok := m.index().Note(rel)
	it.Detail = n.Title
	return it, ok
}
