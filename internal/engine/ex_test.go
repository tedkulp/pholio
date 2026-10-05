package engine

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

// memFS is an in-memory FS. Paths under ro/ refuse writes and paths under
// bad/ refuse reads and writes.
type memFS map[string]string

func (m memFS) ReadFile(path string) ([]byte, error) {
	if strings.HasPrefix(path, "bad/") {
		return nil, errors.New("permission denied")
	}
	s, ok := m[path]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	return []byte(s), nil
}

func (m memFS) WriteFile(path string, data []byte) error {
	if strings.HasPrefix(path, "ro/") || strings.HasPrefix(path, "bad/") {
		return errors.New("permission denied")
	}
	m[path] = string(data)
	return nil
}

// The app's seam.FS (and its test fake) can be passed straight to Open.
var (
	_ FS = seam.OSFS{}
	_ FS = (*seamtest.MemFS)(nil)
)

func TestOpenWithSeamFS(t *testing.T) {
	m := seamtest.NewMemFS(map[string]string{"notes/a.md": "hi\n"})
	e, err := Open(m, "notes/a.md")
	if err != nil {
		t.Fatal(err)
	}
	feed(e, "x:w notes/b.md<enter>:e! notes/c.md<enter>")
	if got, _ := m.ReadFile("notes/b.md"); string(got) != "i\n" {
		t.Errorf("b.md = %q", got)
	}
	if e.Msg != `"notes/c.md" [New]` {
		t.Errorf("msg %q", e.Msg)
	}
}

func open(t *testing.T, m memFS, path string) *Engine {
	t.Helper()
	e, err := Open(m, path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	return e
}

func TestOpen(t *testing.T) {
	m := memFS{"a.md": "one\ntwo\n"}
	e := open(t, m, "a.md")
	if e.Path != "a.md" || e.Buf.String() != "one\ntwo\n" || e.Dirty || e.Msg != `"a.md" 2L` {
		t.Errorf("Open: path %q buf %q dirty %v msg %q", e.Path, e.Buf.String(), e.Dirty, e.Msg)
	}

	e = open(t, m, "new.md")
	if e.Path != "new.md" || e.Buf.String() != "\n" || e.Msg != `"new.md" [New]` {
		t.Errorf("Open new: path %q buf %q msg %q", e.Path, e.Buf.String(), e.Msg)
	}
	if _, ok := m["new.md"]; ok {
		t.Error("opening a missing file must not create it")
	}

	if _, err := Open(memFS{}, "bad/x.md"); err == nil {
		t.Error("Open of an unreadable file should fail")
	}
}

// exCase runs keys on a.md (or text when no file) and checks the result.
type exCase struct {
	name     string
	files    memFS
	keys     string
	buf      string // want buffer, without the trailing newline
	path     string
	msg      string
	dirty    bool
	quit     bool
	wantFile map[string]string // files that must have this content afterwards
}

func TestExCommands(t *testing.T) {
	cases := []exCase{
		{name: ":w", keys: "xx:w<enter>", buf: "e", path: "a.md", msg: `"a.md" 1L written`,
			wantFile: map[string]string{"a.md": "e\n"}},
		{name: ":write", keys: "x:write<enter>", buf: "ne", path: "a.md", msg: `"a.md" 1L written`},
		{name: ":w other keeps name", keys: "x:w b.md<enter>", buf: "ne", path: "a.md", dirty: true,
			msg: `"b.md" 1L written`, wantFile: map[string]string{"a.md": "one", "b.md": "ne\n"}},
		{name: ":w fails", keys: "x:w ro/b.md<enter>", buf: "ne",
			path: "a.md", dirty: true, msg: "E212: Can't open file for writing: permission denied"},
		{name: ":wq", keys: "x:wq<enter>", buf: "ne", path: "a.md", msg: `"a.md" 1L written`, quit: true,
			wantFile: map[string]string{"a.md": "ne\n"}},
		{name: ":x", keys: "x:x<enter>", buf: "ne", path: "a.md", msg: `"a.md" 1L written`, quit: true},
		{name: ":wq fails", keys: "x:wq ro/b.md<enter>", buf: "ne", path: "a.md",
			dirty: true, msg: "E212: Can't open file for writing: permission denied"},
		{name: ":q clean", keys: ":q<enter>", buf: "one", path: "a.md", quit: true},
		{name: ":quit", keys: ":quit<enter>", buf: "one", path: "a.md", quit: true},
		{name: ":q dirty", keys: "x:q<enter>", buf: "ne", path: "a.md", dirty: true,
			msg: "E37: No write since last change (add ! to override)"},
		{name: ":q!", keys: "x:q!<enter>", buf: "ne", path: "a.md", dirty: true, quit: true},
		{name: ":e other", keys: ":e b.md<enter>", buf: "bee", path: "b.md", msg: `"b.md" 1L`},
		{name: ":edit other", keys: ":edit b.md<enter>", buf: "bee", path: "b.md", msg: `"b.md" 1L`},
		{name: ":e new file", keys: ":e c.md<enter>", buf: "", path: "c.md", msg: `"c.md" [New]`},
		{name: ":e dirty", keys: "x:e b.md<enter>", buf: "ne", path: "a.md", dirty: true,
			msg: "E37: No write since last change (add ! to override)"},
		{name: ":e! other", keys: "x:e! b.md<enter>", buf: "bee", path: "b.md", msg: `"b.md" 1L`},
		{name: ":e! reloads", keys: "x:e!<enter>", buf: "one", path: "a.md", msg: `"a.md" 1L`},
		{name: ":e reloads clean", keys: ":e<enter>", buf: "one", path: "a.md", msg: `"a.md" 1L`},
		{name: ":e unreadable", keys: ":e bad/b.md<enter>", buf: "one", path: "a.md",
			msg: `E484: Can't open file bad/b.md`},
		{name: "unknown", keys: ":frob it<enter>", buf: "one", path: "a.md", msg: "E492: Not an editor command: frob it"},
		{name: "empty", keys: ":<enter>", buf: "one", path: "a.md"},
		{name: "cancelled", keys: ":q<esc>", buf: "one", path: "a.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := c.files
			if files == nil {
				files = memFS{"a.md": "one", "b.md": "bee\n"}
			}
			e := open(t, files, "a.md")
			feed(e, c.keys)
			if got := e.Buf.String(); got != c.buf+"\n" {
				t.Errorf("buf = %q, want %q", got, c.buf+"\n")
			}
			if e.Path != c.path || e.Msg != c.msg || e.Dirty != c.dirty || e.Quit != c.quit {
				t.Errorf("path %q msg %q dirty %v quit %v\nwant %q %q %v %v",
					e.Path, e.Msg, e.Dirty, e.Quit, c.path, c.msg, c.dirty, c.quit)
			}
			for p, want := range c.wantFile {
				if files[p] != want {
					t.Errorf("file %s = %q, want %q", p, files[p], want)
				}
			}
			if e.Mode != Normal {
				t.Errorf("mode %v after ex", e.Mode)
			}
		})
	}
}

func TestExWithoutFile(t *testing.T) {
	e := load("|abc")
	feed(e, "x:w<enter>")
	if e.Msg != "E32: No file name" || !e.Dirty {
		t.Errorf("msg %q dirty %v", e.Msg, e.Dirty)
	}
}

func TestEditResetsUndo(t *testing.T) {
	e := open(t, memFS{"a.md": "one", "b.md": "bee"}, "a.md")
	feed(e, "x:w<enter>:e b.md<enter>u")
	if e.Buf.String() != "bee\n" || e.Msg != "Already at oldest change" {
		t.Errorf("buf %q msg %q", e.Buf.String(), e.Msg)
	}
}

func TestExLineNumber(t *testing.T) {
	runTable(t, []tcase{
		{":2", "|a\n  b\nc", ":2<enter>", "a\n  |b\nc"},
		{":99 clamps", "|a\nb", ":99<enter>", "a\n|b"},
		{":0", "a\n|b", ":0<enter>", "|a\nb"},
	})
}
