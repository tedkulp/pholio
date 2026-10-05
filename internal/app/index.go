package app

import (
	"context"
	"errors"
	"io/fs"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/index"
)

// indexDebounce is how long the open buffer has to stay unchanged before
// the index re-reads it.
const indexDebounce = 300 * time.Millisecond

// indexReadyMsg reports that the startup scan finished.
type indexReadyMsg struct{ err error }

// indexSyncMsg asks for the open buffer to be fed to the index, unless a
// later edit superseded it (gen).
type indexSyncMsg struct{ gen int }

// index is the Vault index, or nil when the app runs without one.
func (m Model) index() *index.Index { return m.deps.Index }

// indexReady reports whether the startup scan has finished. Until then,
// features built on the index show "indexing…".
func (m Model) indexReady() bool { return m.deps.Index != nil && m.deps.Index.Ready() }

// scanIndex is the startup scan, run in the background.
func (m Model) scanIndex() tea.Cmd {
	ix := m.deps.Index
	if ix == nil {
		return nil
	}
	return func() tea.Msg { return indexReadyMsg{err: ix.Scan(context.Background())} }
}

// indexScanned handles the end of the startup scan. The scan read the open
// Note from disk, so a dirty buffer is fed again: it is the source of truth.
func (m Model) indexScanned(msg indexReadyMsg) Model {
	if msg.err != nil {
		return m.say("indexing the Vault: "+msg.err.Error(), true)
	}
	if m.ed.Engine().Dirty {
		m = m.syncIndex()
	}
	return m.refreshPickers()
}

// editedIndex schedules feeding the open buffer to the index when an edit
// changed it. Each edit restarts the wait.
func (m Model) editedIndex() (Model, tea.Cmd) {
	if m.deps.Index == nil || m.ed.Engine().Buf.Version() == m.fed {
		return m, nil
	}
	m.syncGen++
	gen := m.syncGen
	return m, tea.Tick(indexDebounce, func(time.Time) tea.Msg { return indexSyncMsg{gen} })
}

// syncIndex feeds the open buffer to the index now.
func (m Model) syncIndex() Model {
	ix := m.deps.Index
	if ix == nil {
		return m
	}
	e := m.ed.Engine()
	ix.Update(m.path(), []byte(e.Buf.String()))
	m.fed = e.Buf.Version()
	m.syncGen++ // a pending tick has nothing left to do
	return m
}

// unsyncIndex puts the open Note's file back in the index, in place of
// buffer text that is being discarded.
func (m Model) unsyncIndex() Model {
	ix := m.deps.Index
	if ix == nil {
		return m
	}
	m.syncGen++
	data, err := m.deps.FS.ReadFile(m.path())
	switch {
	case err == nil:
		ix.Update(m.path(), data)
	case errors.Is(err, fs.ErrNotExist):
		ix.Remove(m.path())
	}
	return m
}
