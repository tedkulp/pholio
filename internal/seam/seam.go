// Package seam holds the interfaces pholio uses to reach the outside world:
// the clock, the filesystem, the URL opener, the trash and the file watcher.
// Production code receives them injected, and tests pass the fakes from
// package seamtest.
package seam

import (
	"io/fs"
	"os"
	"time"
)

// Clock reports the wall-clock time. Today is derived from it.
type Clock interface {
	Now() time.Time
}

// FS is the filesystem pholio reads and writes Notes through.
// Paths are OS paths.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte) error
	// Create writes data to a new file, failing with fs.ErrExist when
	// name is already there.
	Create(name string, data []byte) error
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	MkdirAll(name string) error
	Rename(from, to string) error
}

// Opener opens a URL in the user's default handler (xdg-open, or open on macOS).
type Opener interface {
	Open(url string) error
}

// Trash moves a file or folder to the user's trash.
type Trash interface {
	Trash(path string) error
}

// Op is the kind of change a WatchEvent reports. Ops can be combined.
type Op uint8

// The changes a Watcher reports.
const (
	OpCreate Op = 1 << iota
	OpWrite
	OpRemove
	OpRename
	// OpOverflow means events were dropped, so a full rescan is needed.
	OpOverflow
)

// WatchEvent is one change seen by a Watcher.
type WatchEvent struct {
	Path string
	Op   Op
}

// Watcher reports changes in watched directories (one watch per directory).
type Watcher interface {
	Add(dir string) error
	Remove(dir string) error
	Events() <-chan WatchEvent
	Errors() <-chan error
	Close() error
}

// SystemClock is the real Clock.
type SystemClock struct{}

// Now returns the current local time.
func (SystemClock) Now() time.Time { return time.Now() }

// OSFS is the real FS, backed by package os.
type OSFS struct{}

// ReadFile reads the named file.
func (OSFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

// WriteFile writes data to the named file, creating it if needed.
func (OSFS) WriteFile(name string, data []byte) error {
	return os.WriteFile(name, data, 0o644)
}

// Create writes data to a new file, failing with fs.ErrExist when name
// is already there.
func (OSFS) Create(name string, data []byte) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) //nolint:gosec // Notes get ordinary 0644 permissions, as WriteFile
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Stat describes the named file.
func (OSFS) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }

// ReadDir lists the named directory, sorted by filename.
func (OSFS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }

// MkdirAll creates a directory and any missing parents.
func (OSFS) MkdirAll(name string) error { return os.MkdirAll(name, 0o755) }

// Rename moves a file or directory.
func (OSFS) Rename(from, to string) error { return os.Rename(from, to) }
