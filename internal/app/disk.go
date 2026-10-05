package app

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/watch"
)

// errChangedOnDisk refuses a write over a file that changed outside pholio
// while the buffer was being edited, until the user confirms.
var errChangedOnDisk = errors.New("changed on disk")

const overwritePrompt = "Changed on disk. Overwrite? y/N"

// noteFile is the open Note's file. The engine reads and writes the Note
// through it (it satisfies the engine's FS), so it knows what is on disk:
// the hash of the contents pholio last read or wrote. A change on disk is
// any content with a different hash.
type noteFile struct {
	fs    seam.FS
	watch *watch.Vault // nil when not watching
	path  string

	disk    [sha256.Size]byte
	exists  bool // the file was on disk when last read or written
	stale   bool // it changed on disk while the buffer was dirty
	deleted bool // it was deleted on disk while open
	refused bool // a write was refused because of stale; the app asks
}

// ReadFile reads name. Reading the Note itself (opening it, or :e!)
// records what is on disk and clears every warning.
func (f *noteFile) ReadFile(name string) ([]byte, error) {
	data, err := f.fs.ReadFile(name)
	if filepath.Clean(name) == f.path && (err == nil || errors.Is(err, fs.ErrNotExist)) {
		f.disk, f.exists = sha256.Sum256(data), err == nil
		f.stale, f.deleted = false, false
	}
	return data, err
}

// WriteFile writes name. A write over the Note while it is stale is
// refused with errChangedOnDisk until the user confirms.
func (f *noteFile) WriteFile(name string, data []byte) error {
	own := filepath.Clean(name) == f.path
	if own && f.stale {
		f.refused = true
		return errChangedOnDisk
	}
	if err := f.fs.WriteFile(name, data); err != nil {
		return err
	}
	if f.watch != nil {
		f.watch.Wrote(name, data)
	}
	if own {
		f.disk, f.exists = sha256.Sum256(data), true
		f.stale, f.deleted = false, false
	}
	return nil
}

// tags are the status-line markers for the file's state.
func (f *noteFile) tags() []string {
	switch {
	case f.deleted:
		return []string{"[deleted]"}
	case f.stale:
		return []string{"[changed on disk]"}
	}
	return nil
}

// noticeMsg carries one watcher Notice into Update.
type noticeMsg watch.Notice

// listen waits for the next watcher Notice.
func (m Model) listen() tea.Cmd {
	if m.deps.Watch == nil {
		return nil
	}
	ch := m.deps.Watch.Notices()
	return func() tea.Msg { return noticeMsg(<-ch) }
}

// WithWatchError reports that the Vault could not be watched, so changes
// made outside pholio are only picked up when the terminal regains focus.
func (m Model) WithWatchError(err error) Model {
	m.message = joinMessages(m.message,
		fmt.Sprintf("File watching unavailable (%v); rescanning on focus only", err))
	m.problem = true
	return m
}

// checkDisk compares the Note on disk with what pholio last saw there and
// applies the reload rules: a clean buffer reloads silently, a dirty one
// gets a warning, and a vanished file marks the buffer [deleted].
func (m Model) checkDisk() Model {
	f, e := m.file, m.ed.Engine()
	data, err := f.fs.ReadFile(f.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if f.exists && !f.deleted {
			f.deleted = true
		}
		return m
	case err != nil:
		return m
	}
	if f.exists && !f.deleted && sha256.Sum256(data) == f.disk {
		return m
	}
	if !e.Dirty || string(data) == e.Buf.String() {
		e.Reload(string(data))
		f.disk, f.exists = sha256.Sum256(data), true
		f.stale, f.deleted = false, false
		if m.w > 0 {
			m = m.relayout() // re-scroll over the new text
		}
		return m
	}
	if !f.stale {
		f.stale, f.deleted = true, false
		m.message, m.problem = "File changed on disk: :e! to reload, :w to overwrite", true
	}
	return m
}

// askedToOverwrite turns a refused write into the y/N prompt.
func (m Model) askedToOverwrite() Model {
	if !m.file.refused {
		return m
	}
	m.file.refused = false
	m.confirming = true
	m.ed.Engine().Msg = ""
	m.message, m.problem = overwritePrompt, false
	return m
}

// answerOverwrite handles the key typed at the y/N prompt.
func (m Model) answerOverwrite(k tea.KeyPressMsg) Model {
	m.confirming = false
	m.message, m.problem = "", false
	if k.String() != "y" {
		return m
	}
	e := m.ed.Engine()
	m.file.stale = false
	if err := m.file.WriteFile(m.path(), []byte(e.Buf.String())); err != nil {
		m.message, m.problem = "E212: Can't open file for writing: "+err.Error(), true
		return m
	}
	e.Dirty = false
	e.Msg = fmt.Sprintf("%q %dL written", m.path(), e.Buf.LineCount())
	return m
}
