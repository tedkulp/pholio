package engine

import (
	"errors"
	"fmt"
	"io/fs"
)

// FS is the engine's only side effect: reading and writing Notes. It is a
// subset of seam.FS, so the app passes its own FS. ReadFile should return an
// error matching fs.ErrNotExist for a missing file, which opens as a new,
// empty buffer.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte) error
}

// Open returns an Engine editing path. A missing file gives an empty buffer
// that is only created on :w; any other read error is returned.
func Open(fsys FS, path string) (*Engine, error) {
	e := New("")
	e.fs = fsys
	if err := e.Load(path); err != nil {
		return nil, err
	}
	return e, nil
}

// Load replaces the buffer with the contents of path, as :e! does. It
// discards unsaved changes and the undo history. A missing file gives an
// empty buffer; on any other error the engine is left unchanged.
func (e *Engine) Load(path string) error {
	if e.fs == nil {
		return errors.New("no file system")
	}
	text, err := e.fs.ReadFile(path)
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	e.Buf = NewBuffer(string(text))
	e.Path, e.Mode = path, Normal
	e.undo, e.redo, e.pending, e.keys, e.recording = nil, nil, nil, nil, false
	e.blockIns = nil
	e.changes++
	e.baseID = e.changes // a fresh base: no older change id matches it
	e.MarkSaved()
	e.SetCursor(Pos{})
	if missing {
		e.Msg = fmt.Sprintf("%q [New]", path)
	} else {
		e.Msg = fmt.Sprintf("%q %dL", path, e.Buf.LineCount())
	}
	return nil
}

// write saves the buffer to path, or to the buffer's own path when path is
// empty. Writing elsewhere leaves the buffer's name and Dirty alone, as in
// vim.
func (e *Engine) write(path string) bool {
	own := path == "" || path == e.Path
	if path == "" {
		path = e.Path
	}
	if path == "" {
		e.Msg = "E32: No file name"
		return false
	}
	if e.fs == nil {
		e.Msg = "E212: Can't open file for writing: no file system"
		return false
	}
	if err := e.fs.WriteFile(path, []byte(e.Buf.String())); err != nil {
		e.Msg = "E212: Can't open file for writing: " + err.Error()
		return false
	}
	if own || e.Path == "" {
		e.Path = path
		e.MarkSaved()
	}
	e.Msg = fmt.Sprintf("%q %dL written", path, e.Buf.LineCount())
	return true
}
