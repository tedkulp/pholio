package index_test

import (
	"context"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func TestNotesWithAnUppercaseExtensionAreNotes(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		"/vault/Foo.MD":   "# Foo\n- [ ] shout\n",
		"/vault/bar.Md":   "[[Foo]]\n",
		"/vault/note.txt": "- [ ] not a Note\n",
	})
	ix := index.New(fsys, "/vault")
	if err := ix.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	n, ok := ix.Note("Foo.MD")
	if !ok || n.Name != "Foo" {
		t.Fatalf("Note(Foo.MD) = %+v, %v; want it indexed as Foo", n, ok)
	}
	if r, ok := ix.Resolve("bar.Md", index.Link{Kind: index.WikiLink, Target: "Foo"}); !ok || r.Path != "Foo.MD" {
		t.Errorf("Resolve(Foo) = %+v, %v", r, ok)
	}
	if bl := ix.Backlinks("Foo.MD"); len(bl) != 1 {
		t.Errorf("Backlinks(Foo.MD) = %v", bl)
	}
	if ts := ix.Tasks(); len(ts) != 1 || ts[0].Path != "Foo.MD" {
		t.Errorf("Tasks() = %+v", ts)
	}

	ix.Update("/vault/Foo.MD", []byte("# Foo\n"))
	if n, _ := ix.Note("Foo.MD"); n.Contents != "# Foo\n" {
		t.Errorf("Update ignored Foo.MD: %q", n.Contents)
	}
}

func TestNoteHelpers(t *testing.T) {
	for p, want := range map[string]bool{"a.md": true, "A.MD": true, "x/b.Md": true, "c.txt": false, "md": false, ".md": true} {
		if got := index.IsNote(p); got != want {
			t.Errorf("IsNote(%q) = %v", p, got)
		}
	}
	for p, want := range map[string]string{"a.md": "a", "x/Foo.MD": "Foo", "c.txt": "c.txt"} {
		if got := index.NoteName(p); got != want {
			t.Errorf("NoteName(%q) = %q, want %q", p, got, want)
		}
	}
	if got := index.TrimNoteExt("x/Foo.MD"); got != "x/Foo" {
		t.Errorf("TrimNoteExt = %q", got)
	}
}
