package watch_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/watch"
)

// startReal watches a fresh t.TempDir() Vault holding note.md with real
// fsnotify.
func startReal(t *testing.T) (*watch.Vault, string) {
	t.Helper()
	root := t.TempDir()
	note := filepath.Join(root, "note.md")
	if err := os.WriteFile(note, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := watch.NewFSNotify()
	if err != nil {
		t.Fatal(err)
	}
	v := watch.New(seam.OSFS{}, root, watch.Options{Debounce: 50 * time.Millisecond})
	if err := v.Watch(w); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v, note
}

// expectOnly waits for a notice about path, then checks nothing else about
// path arrives for a while (the burst was coalesced).
func expectOnly(t *testing.T, v *watch.Vault, path, data string) {
	t.Helper()
	n := next(t, v)
	if n.Path != path || n.Gone || string(n.Data) != data {
		t.Fatalf("notice = {Path:%s Gone:%v Data:%q}, want %s = %q", n.Path, n.Gone, n.Data, path, data)
	}
	select {
	case n := <-v.Notices():
		t.Fatalf("extra notice {Path:%s Gone:%v Data:%q}", n.Path, n.Gone, n.Data)
	case <-time.After(300 * time.Millisecond):
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRealSaveInPlace(t *testing.T) {
	v, note := startReal(t)
	f, err := os.OpenFile(note, os.O_WRONLY|os.O_TRUNC, 0)
	must(t, err)
	_, err = f.WriteString("new\n")
	must(t, err)
	must(t, f.Close())
	expectOnly(t, v, note, "new\n")
}

func TestRealSaveRenameAwayAndCreate(t *testing.T) {
	// vim and nvim defaults: RENAME note.md -> note.md~, CREATE note.md,
	// WRITE, CHMOD, REMOVE note.md~.
	v, note := startReal(t)
	must(t, os.Rename(note, note+"~"))
	must(t, os.WriteFile(note, []byte("new\n"), 0o644))
	must(t, os.Chmod(note, 0o600))
	must(t, os.Remove(note+"~"))
	expectOnly(t, v, note, "new\n")
}

func TestRealSaveTempAndRenameOver(t *testing.T) {
	// Syncthing and sed -i: write a temp file, then rename it over the Note.
	v, note := startReal(t)
	tmp := filepath.Join(filepath.Dir(note), ".syncthing.note.md.tmp")
	must(t, os.WriteFile(tmp, []byte("new\n"), 0o644))
	must(t, os.Rename(tmp, note))
	expectOnly(t, v, note, "new\n")
}

func TestRealDeleteIsGone(t *testing.T) {
	v, note := startReal(t)
	must(t, os.Remove(note))
	n := next(t, v)
	if n.Path != note || !n.Gone {
		t.Fatalf("notice = %+v", n)
	}
}

func TestRealNewDirectoryRace(t *testing.T) {
	v, note := startReal(t)
	deep := filepath.Join(filepath.Dir(note), "a", "b", "c")
	must(t, os.MkdirAll(deep, 0o755))
	p := filepath.Join(deep, "n.md")
	must(t, os.WriteFile(p, []byte("hi\n"), 0o644))
	expectOnly(t, v, p, "hi\n")
	// The new directories are watched from now on.
	must(t, os.WriteFile(p, []byte("again\n"), 0o644))
	expectOnly(t, v, p, "again\n")
}

func TestRealDotDirIgnored(t *testing.T) {
	v, note := startReal(t)
	must(t, os.WriteFile(filepath.Join(filepath.Dir(note), ".git", "x.md"), []byte("x"), 0o644))
	must(t, os.WriteFile(note, []byte("new\n"), 0o644))
	expectOnly(t, v, note, "new\n")
}
