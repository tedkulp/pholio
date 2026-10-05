// Package watch keeps pholio in step with changes made to the Vault outside
// pholio. A Vault turns raw watcher events (one watch per directory) into
// debounced Notices: a Note changed, a Note is gone, or events were lost
// and everything must be rescanned.
package watch

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tedkulp/pholio/internal/seam"
)

// DefaultDebounce is how long a path must be quiet before it is re-read.
// Editors save in bursts (rename-away, create, write, chmod), and the file
// can be briefly missing in the middle of one.
const DefaultDebounce = 100 * time.Millisecond

// Index is what a Vault keeps up to date. *index.Index satisfies it.
type Index interface {
	Update(path string, contents []byte)
	Remove(path string)
	Scan(ctx context.Context) error
}

// Options configure a Vault.
type Options struct {
	// Debounce is the quiet time per path before it is re-read;
	// zero means DefaultDebounce.
	Debounce time.Duration
	// Index, if set, is updated before each Notice is sent and rescanned
	// before each Rescan Notice.
	Index Index
}

// Notice is one change the Vault saw. Either Rescan is set, or Path names
// a Note (an absolute OS path) that now holds Data, or is Gone.
type Notice struct {
	Path   string
	Data   []byte
	Gone   bool
	Rescan bool // events were lost or a directory went away: recheck everything
}

// Vault watches a Vault's directories through a seam.Watcher.
type Vault struct {
	fs    seam.FS
	root  string
	opt   Options
	out   chan Notice
	done  chan struct{}
	close sync.Once

	mu      sync.Mutex
	w       seam.Watcher
	dirs    map[string]bool
	timers  map[string]*time.Timer
	written map[string][sha256.Size]byte
}

// New returns a Vault over root that is not watching yet. Without Watch it
// still serves Rescan and its Notices, which is the focus-only fallback.
func New(fsys seam.FS, root string, opt Options) *Vault {
	if opt.Debounce <= 0 {
		opt.Debounce = DefaultDebounce
	}
	return &Vault{
		fs: fsys, root: filepath.Clean(root), opt: opt,
		out:     make(chan Notice, 256),
		done:    make(chan struct{}),
		dirs:    map[string]bool{},
		timers:  map[string]*time.Timer{},
		written: map[string][sha256.Size]byte{},
	}
}

// Watch adds a watch for every directory in the Vault except dot-dirs and
// starts handling w's events. If any directory can't be watched, w is
// closed and the error returned; the Vault then only rescans on request.
func (v *Vault) Watch(w seam.Watcher) error {
	v.mu.Lock()
	err := v.addTree(w, v.root, false)
	if err == nil {
		v.w = w
	}
	v.mu.Unlock()
	if err != nil {
		_ = w.Close()
		return err
	}
	go v.loop(w)
	return nil
}

// Notices delivers what the Vault saw, in order.
func (v *Vault) Notices() <-chan Notice { return v.out }

// Wrote records that pholio itself just wrote data to path, so the echo of
// that write is ignored. It is matched by content hash.
func (v *Vault) Wrote(path string, data []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.written[filepath.Clean(path)] = sha256.Sum256(data)
}

// Rescan rescans the Index, if any, then sends a Rescan Notice. It returns
// at once; call it when the terminal regains focus.
func (v *Vault) Rescan() { go v.rescan() }

// Close stops watching. Notices is not closed.
func (v *Vault) Close() error {
	var err error
	v.close.Do(func() {
		close(v.done)
		v.mu.Lock()
		defer v.mu.Unlock()
		for _, t := range v.timers {
			t.Stop()
		}
		if v.w != nil {
			err = v.w.Close()
		}
	})
	return err
}

func (v *Vault) loop(w seam.Watcher) {
	events, errs := w.Events(), w.Errors()
	for events != nil || errs != nil {
		select {
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			v.handle(w, ev)
		case _, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			v.rescan()
		case <-v.done:
			return
		}
	}
}

func (v *Vault) handle(w seam.Watcher, ev seam.WatchEvent) {
	if ev.Op&seam.OpOverflow != 0 {
		v.rescan()
		return
	}
	p := filepath.Clean(ev.Path)
	if !v.inside(p) {
		return
	}
	if isNote(p) {
		v.schedule(p)
		return
	}
	info, err := v.fs.Stat(p)
	v.mu.Lock()
	switch {
	case err == nil && info.IsDir() && !v.dirs[p]:
		// Files made inside a new directory before its watch was added
		// send no events, so index everything already in it.
		_ = v.addTree(w, p, true)
		v.mu.Unlock()
	case err != nil && v.dirs[p]:
		for d := range v.dirs {
			if d == p || strings.HasPrefix(d, p+string(filepath.Separator)) {
				_ = w.Remove(d)
				delete(v.dirs, d)
			}
		}
		v.mu.Unlock()
		v.rescan()
	default:
		v.mu.Unlock()
	}
}

// addTree watches dir and every directory under it, skipping dot-dirs.
// With notes set, it schedules a read of every Note it finds. v.mu is held.
func (v *Vault) addTree(w seam.Watcher, dir string, notes bool) error {
	if err := w.Add(dir); err != nil {
		return err
	}
	v.dirs[dir] = true
	entries, err := v.fs.ReadDir(dir)
	if err != nil {
		return nil //nolint:nilerr // the directory may already be gone; its removal event follows
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		switch {
		case strings.HasPrefix(e.Name(), "."):
		case e.IsDir():
			if err := v.addTree(w, p, notes); err != nil {
				return err
			}
		case notes && isNote(p):
			v.scheduleLocked(p)
		}
	}
	return nil
}

func (v *Vault) schedule(p string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.scheduleLocked(p)
}

func (v *Vault) scheduleLocked(p string) {
	if t, ok := v.timers[p]; ok {
		t.Reset(v.opt.Debounce)
		return
	}
	v.timers[p] = time.AfterFunc(v.opt.Debounce, func() { v.flush(p) })
}

// flush re-reads p once its events have settled. The event types are not
// trusted: whatever is on disk now decides.
func (v *Vault) flush(p string) {
	v.mu.Lock()
	delete(v.timers, p)
	v.mu.Unlock()

	data, err := v.fs.ReadFile(p)
	gone := errors.Is(err, fs.ErrNotExist)
	if err != nil && !gone {
		return
	}
	v.mu.Lock()
	if gone {
		delete(v.written, p)
	} else if h, ok := v.written[p]; ok {
		if h == sha256.Sum256(data) {
			v.mu.Unlock()
			return
		}
		delete(v.written, p)
	}
	v.mu.Unlock()

	if ix := v.opt.Index; ix != nil {
		if gone {
			ix.Remove(p)
		} else {
			ix.Update(p, data)
		}
	}
	v.send(Notice{Path: p, Data: data, Gone: gone})
}

func (v *Vault) rescan() {
	if ix := v.opt.Index; ix != nil {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-v.done:
				cancel()
			case <-ctx.Done():
			}
		}()
		_ = ix.Scan(ctx)
		cancel()
	}
	v.send(Notice{Rescan: true})
}

func (v *Vault) send(n Notice) {
	select {
	case v.out <- n:
	case <-v.done:
	}
}

// inside reports whether p is in the Vault and not under a dot-dir.
func (v *Vault) inside(p string) bool {
	rel, err := filepath.Rel(v.root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	if rel == "." {
		return true
	}
	for seg := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(seg, ".") {
			return false
		}
	}
	return true
}

// isNote reports whether p names a Note. Swap and backup files (.swp, ~,
// 4913, sedXXXX) don't end in ".md" and are skipped with everything else.
func isNote(p string) bool {
	return strings.HasSuffix(p, ".md") && !strings.HasPrefix(filepath.Base(p), ".")
}
