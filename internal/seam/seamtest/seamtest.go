// Package seamtest provides in-memory fakes for the interfaces in package seam.
// All fakes are safe for concurrent use.
package seamtest

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing/fstest"
	"time"

	"github.com/tedkulp/pholio/internal/seam"
)

// Clock is a fake seam.Clock that only moves when told to.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

var _ seam.Clock = (*Clock)(nil)

// NewClock returns a Clock stopped at now.
func NewClock(now time.Time) *Clock { return &Clock{now: now} }

// Now returns the fake time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set moves the clock to t.
func (c *Clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Advance moves the clock forward by d.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// MemFS is a fake seam.FS held in memory. Paths are slash-separated and
// absolute ("/vault/a.md"); parent directories exist implicitly.
type MemFS struct {
	mu    sync.Mutex
	files fstest.MapFS
}

var _ seam.FS = (*MemFS)(nil)

// NewMemFS returns a MemFS holding files, a map of absolute path to contents.
func NewMemFS(files map[string]string) *MemFS {
	m := &MemFS{files: fstest.MapFS{}}
	for p, s := range files {
		m.files[key(p)] = &fstest.MapFile{Data: []byte(s), Mode: 0o644}
	}
	return m
}

func key(name string) string {
	k := strings.TrimPrefix(path.Clean(filepath.ToSlash(name)), "/")
	if k == "" {
		return "."
	}
	return k
}

func pathErr(op, name string, err error) error {
	return &fs.PathError{Op: op, Path: name, Err: err}
}

// ReadFile returns the contents of the named file.
func (m *MemFS) ReadFile(name string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := m.files.ReadFile(key(name))
	if err != nil {
		return nil, pathErr("read", name, unwrap(err))
	}
	return b, nil
}

// WriteFile stores data at name, replacing any existing file.
func (m *MemFS) WriteFile(name string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, err := m.files.Stat(key(name)); err == nil && info.IsDir() {
		return pathErr("write", name, fs.ErrInvalid)
	}
	m.files[key(name)] = &fstest.MapFile{Data: append([]byte(nil), data...), Mode: 0o644, ModTime: time.Now()}
	return nil
}

// Stat describes the named file or directory.
func (m *MemFS) Stat(name string) (fs.FileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, err := m.files.Stat(key(name))
	if err != nil {
		return nil, pathErr("stat", name, unwrap(err))
	}
	return info, nil
}

// ReadDir lists the named directory, sorted by filename.
func (m *MemFS) ReadDir(name string) ([]fs.DirEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries, err := m.files.ReadDir(key(name))
	if err != nil {
		return nil, pathErr("readdir", name, unwrap(err))
	}
	return entries, nil
}

// MkdirAll creates an explicit, empty directory entry.
func (m *MemFS) MkdirAll(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, err := m.files.Stat(key(name)); err == nil {
		if info.IsDir() {
			return nil
		}
		return pathErr("mkdir", name, fs.ErrExist)
	}
	m.files[key(name)] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
	return nil
}

// Rename moves a file, or a directory and everything under it.
func (m *MemFS) Rename(from, to string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	src, dst := key(from), key(to)
	if _, err := m.files.Stat(src); err != nil {
		return pathErr("rename", from, unwrap(err))
	}
	for k, f := range m.files {
		switch {
		case k == src:
			delete(m.files, k)
			m.files[dst] = f
		case strings.HasPrefix(k, src+"/"):
			delete(m.files, k)
			m.files[dst+strings.TrimPrefix(k, src)] = f
		}
	}
	return nil
}

// Remove deletes a file, or a directory and everything under it.
// It is not part of seam.FS; the fake Trash uses it.
func (m *MemFS) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(name)
	if _, err := m.files.Stat(k); err != nil {
		return pathErr("remove", name, unwrap(err))
	}
	for p := range m.files {
		if p == k || strings.HasPrefix(p, k+"/") {
			delete(m.files, p)
		}
	}
	return nil
}

// unwrap strips the PathError fstest adds so ours carries the caller's path.
func unwrap(err error) error {
	if pe, ok := err.(*fs.PathError); ok { //nolint:errorlint // fstest returns *PathError directly
		return pe.Err
	}
	return err
}

// Opener is a fake seam.Opener that records every URL it is asked to open.
type Opener struct {
	mu     sync.Mutex
	opened []string
	// Err, if set, is returned from Open.
	Err error
}

var _ seam.Opener = (*Opener)(nil)

// Open records url.
func (o *Opener) Open(url string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.opened = append(o.opened, url)
	return o.Err
}

// Opened returns the URLs opened so far, oldest first.
func (o *Opener) Opened() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.opened...)
}

// Trash is a fake seam.Trash. It records each trashed path and, when FS is
// set, removes the path from it.
type Trash struct {
	FS *MemFS

	mu      sync.Mutex
	trashed []string
}

var _ seam.Trash = (*Trash)(nil)

// Trash records path and removes it from FS, if set.
func (t *Trash) Trash(path string) error {
	if t.FS != nil {
		if err := t.FS.Remove(path); err != nil {
			return err
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.trashed = append(t.trashed, path)
	return nil
}

// Trashed returns the paths trashed so far, oldest first.
func (t *Trash) Trashed() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.trashed...)
}

// Watcher is a fake seam.Watcher. Tests push events with Emit and errors with
// Fail; Add and Remove only track which directories are watched.
type Watcher struct {
	mu     sync.Mutex
	dirs   map[string]bool
	events chan seam.WatchEvent
	errs   chan error
	// AddErr, if set, is returned from Add (for example to simulate running
	// out of file descriptors).
	AddErr error
}

var _ seam.Watcher = (*Watcher)(nil)

// NewWatcher returns a Watcher whose channels buffer up to 64 items.
func NewWatcher() *Watcher {
	return &Watcher{
		dirs:   map[string]bool{},
		events: make(chan seam.WatchEvent, 64),
		errs:   make(chan error, 64),
	}
}

// Add starts watching dir.
func (w *Watcher) Add(dir string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.AddErr != nil {
		return w.AddErr
	}
	w.dirs[dir] = true
	return nil
}

// Remove stops watching dir.
func (w *Watcher) Remove(dir string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.dirs, dir)
	return nil
}

// Watching reports whether dir is being watched.
func (w *Watcher) Watching(dir string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dirs[dir]
}

// Events returns the event channel.
func (w *Watcher) Events() <-chan seam.WatchEvent { return w.events }

// Errors returns the error channel.
func (w *Watcher) Errors() <-chan error { return w.errs }

// Emit delivers an event to the watcher's consumer.
func (w *Watcher) Emit(ev seam.WatchEvent) { w.events <- ev }

// Fail delivers an error to the watcher's consumer.
func (w *Watcher) Fail(err error) { w.errs <- err }

// Close closes both channels.
func (w *Watcher) Close() error {
	close(w.events)
	close(w.errs)
	return nil
}
