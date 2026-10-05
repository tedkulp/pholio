package watch_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/watch"
)

const debounce = 20 * time.Millisecond

// fakeIndex records what the Vault feeds it.
type fakeIndex struct {
	mu      sync.Mutex
	updated map[string]string
	removed []string
	scans   int
}

func newFakeIndex() *fakeIndex { return &fakeIndex{updated: map[string]string{}} }

func (f *fakeIndex) Update(p string, contents []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated[p] = string(contents)
}

func (f *fakeIndex) Remove(p string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, p)
}

func (f *fakeIndex) Scan(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scans++
	return nil
}

func (f *fakeIndex) get(p string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.updated[p]
	return s, ok
}

type rig struct {
	fs  *seamtest.MemFS
	w   *seamtest.Watcher
	ix  *fakeIndex
	v   *watch.Vault
	err error
}

func newRig(t *testing.T, files map[string]string) *rig {
	t.Helper()
	r := &rig{fs: seamtest.NewMemFS(files), w: seamtest.NewWatcher(), ix: newFakeIndex()}
	r.v = watch.New(r.fs, "/vault", watch.Options{Debounce: debounce, Index: r.ix})
	r.err = r.v.Watch(r.w)
	t.Cleanup(func() { _ = r.v.Close() })
	return r
}

func next(t *testing.T, v *watch.Vault) watch.Notice {
	t.Helper()
	select {
	case n := <-v.Notices():
		return n
	case <-time.After(2 * time.Second):
		t.Fatal("no notice")
		return watch.Notice{}
	}
}

func TestWatchAddsEveryDirectoryButDotDirs(t *testing.T) {
	r := newRig(t, map[string]string{
		"/vault/a.md":           "a",
		"/vault/sub/deep/b.md":  "b",
		"/vault/.git/config":    "x",
		"/vault/.obsidian/x.md": "x",
	})
	if r.err != nil {
		t.Fatal(r.err)
	}
	for _, d := range []string{"/vault", "/vault/sub", "/vault/sub/deep"} {
		if !r.w.Watching(d) {
			t.Errorf("%s not watched", d)
		}
	}
	for _, d := range []string{"/vault/.git", "/vault/.obsidian"} {
		if r.w.Watching(d) {
			t.Errorf("%s watched", d)
		}
	}
}

func TestWatchFailureIsReported(t *testing.T) {
	fs := seamtest.NewMemFS(map[string]string{"/vault/a.md": "a"})
	w := seamtest.NewWatcher()
	w.AddErr = errors.New("too many open files")
	v := watch.New(fs, "/vault", watch.Options{Debounce: debounce})
	defer func() { _ = v.Close() }()
	if err := v.Watch(w); err == nil {
		t.Fatal("Watch succeeded")
	}
}

func TestChangedNoteIsReadAfterDebounce(t *testing.T) {
	r := newRig(t, map[string]string{"/vault/a.md": "old"})
	_ = r.fs.WriteFile("/vault/a.md", []byte("new"))
	r.w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpWrite})
	n := next(t, r.v)
	if n.Path != "/vault/a.md" || string(n.Data) != "new" || n.Gone || n.Rescan {
		t.Fatalf("notice = %+v", n)
	}
	if s, _ := r.ix.get("/vault/a.md"); s != "new" {
		t.Errorf("index has %q", s)
	}
}

func TestBurstOfEventsIsOneNotice(t *testing.T) {
	r := newRig(t, map[string]string{"/vault/a.md": "old", "/vault/b.md": "b"})
	for _, op := range []seam.Op{seam.OpRename, seam.OpCreate, seam.OpWrite, seam.OpWrite} {
		r.w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: op})
	}
	_ = r.fs.WriteFile("/vault/a.md", []byte("new"))
	if n := next(t, r.v); n.Path != "/vault/a.md" || string(n.Data) != "new" {
		t.Fatalf("notice = %+v", n)
	}
	// A later, separate change shows up next: nothing else was queued for a.md.
	r.w.Emit(seam.WatchEvent{Path: "/vault/b.md", Op: seam.OpWrite})
	if n := next(t, r.v); n.Path != "/vault/b.md" {
		t.Fatalf("second notice = %+v", n)
	}
}

func TestMissingAfterDebounceIsGone(t *testing.T) {
	r := newRig(t, map[string]string{"/vault/a.md": "a"})
	_ = r.fs.Remove("/vault/a.md")
	r.w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpRemove})
	n := next(t, r.v)
	if n.Path != "/vault/a.md" || !n.Gone {
		t.Fatalf("notice = %+v", n)
	}
	if !slices.Contains(r.ix.removed, "/vault/a.md") {
		t.Errorf("index not told: %v", r.ix.removed)
	}
}

func TestIgnoredPaths(t *testing.T) {
	r := newRig(t, map[string]string{
		"/vault/a.md": "a", "/vault/a.md~": "x", "/vault/.git/HEAD": "x",
		"/vault/.obsidian/n.md": "x", "/vault/4913": "",
	})
	for _, p := range []string{"/vault/a.md~", "/vault/.git/HEAD", "/vault/.obsidian/n.md", "/vault/4913", "/elsewhere/n.md"} {
		r.w.Emit(seam.WatchEvent{Path: p, Op: seam.OpWrite})
	}
	r.w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpWrite})
	if n := next(t, r.v); n.Path != "/vault/a.md" {
		t.Fatalf("first notice = %+v", n)
	}
}

func TestNewDirectoryIsWatchedAndItsNotesIndexed(t *testing.T) {
	r := newRig(t, map[string]string{"/vault/a.md": "a"})
	// mkdir -p x/y && echo > x/y/n.md: only the Create of x is reported.
	_ = r.fs.WriteFile("/vault/x/y/n.md", []byte("hello"))
	r.w.Emit(seam.WatchEvent{Path: "/vault/x", Op: seam.OpCreate})
	n := next(t, r.v)
	if n.Path != "/vault/x/y/n.md" || string(n.Data) != "hello" {
		t.Fatalf("notice = %+v", n)
	}
	for _, d := range []string{"/vault/x", "/vault/x/y"} {
		if !r.w.Watching(d) {
			t.Errorf("%s not watched", d)
		}
	}
}

func TestOverflowAndErrorsRescan(t *testing.T) {
	r := newRig(t, map[string]string{"/vault/a.md": "a"})
	r.w.Emit(seam.WatchEvent{Op: seam.OpOverflow})
	if n := next(t, r.v); !n.Rescan {
		t.Fatalf("notice = %+v", n)
	}
	r.w.Fail(errors.New("boom"))
	if n := next(t, r.v); !n.Rescan {
		t.Fatalf("notice = %+v", n)
	}
	r.ix.mu.Lock()
	defer r.ix.mu.Unlock()
	if r.ix.scans != 2 {
		t.Errorf("scans = %d, want 2", r.ix.scans)
	}
}

func TestRemovedDirectoryRescans(t *testing.T) {
	r := newRig(t, map[string]string{"/vault/a.md": "a", "/vault/sub/b.md": "b"})
	_ = r.fs.Remove("/vault/sub")
	r.w.Emit(seam.WatchEvent{Path: "/vault/sub", Op: seam.OpRename})
	if n := next(t, r.v); !n.Rescan {
		t.Fatalf("notice = %+v", n)
	}
	if r.w.Watching("/vault/sub") {
		t.Error("removed directory still watched")
	}
}

func TestOwnWritesAreIgnored(t *testing.T) {
	r := newRig(t, map[string]string{"/vault/a.md": "old"})
	_ = r.fs.WriteFile("/vault/a.md", []byte("mine"))
	r.v.Wrote("/vault/a.md", []byte("mine"))
	r.w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpWrite})
	// Someone else then changes it.
	time.Sleep(2 * debounce)
	_ = r.fs.WriteFile("/vault/a.md", []byte("theirs"))
	r.w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpWrite})
	if n := next(t, r.v); string(n.Data) != "theirs" {
		t.Fatalf("notice = %+v, want only the external change", n)
	}
}

func TestRescanWithoutWatcher(t *testing.T) {
	ix := newFakeIndex()
	v := watch.New(seamtest.NewMemFS(nil), "/vault", watch.Options{Index: ix})
	defer func() { _ = v.Close() }()
	v.Rescan()
	if n := next(t, v); !n.Rescan {
		t.Fatalf("notice = %+v", n)
	}
}
