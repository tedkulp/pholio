package index_test

import (
	"context"
	"io/fs"
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func TestUpdateReparsesABufferIntoTheIndex(t *testing.T) {
	ix, root := scanned(t, "links")

	ix.Update(root+"/sub/gamma.md", []byte("# Gamma v2\n- [ ] new task\n[[beta]]\n"))
	n, _ := ix.Note("sub/gamma.md")
	if n.Title != "Gamma v2" {
		t.Errorf("Title = %q", n.Title)
	}
	if got := taskRows(ix.Tasks()); !reflect.DeepEqual(got, []string{"sub/gamma.md:1 [ ] new task"}) {
		t.Errorf("Tasks = %q", got)
	}
	if bl := ix.Backlinks("alpha.md"); len(bl) != 2 {
		t.Errorf("gamma's old Link to alpha still counted: %+v", bl)
	}

	// A new Note becomes resolvable.
	ix.Update(root+"/new/fresh.md", []byte("# Fresh\n"))
	if r, ok := ix.Resolve("index.md", wiki("fresh")); !ok || r.Path != "new/fresh.md" {
		t.Errorf("Resolve(fresh) = %+v, %v", r, ok)
	}
	// Renaming by Remove + Update keeps the name map in step.
	ix.Remove(root + "/new/fresh.md")
	if _, ok := ix.Resolve("index.md", wiki("fresh")); ok {
		t.Error("removed Note still resolves")
	}
	if _, ok := ix.Note("new/fresh.md"); ok {
		t.Error("removed Note still indexed")
	}
}

func TestUpdateIgnoresPathsOutsideTheIndexedSet(t *testing.T) {
	ix, root := scanned(t, "links")
	before := paths(ix.Notes())
	ix.Update(root+"/notes.txt", []byte("x"))
	ix.Update(root+"/.obsidian/x.md", []byte("x"))
	ix.Update(root+"/../elsewhere.md", []byte("x"))
	ix.Update("relative.md", []byte("x"))
	if got := paths(ix.Notes()); !reflect.DeepEqual(got, before) {
		t.Errorf("Notes changed: %v", got)
	}
}

// gatedFS holds the first ReadDir of the Vault root until release is closed.
type gatedFS struct {
	*seamtest.MemFS
	entered, release chan struct{}
}

func (g gatedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "/vault" {
		close(g.entered)
		<-g.release
	}
	return g.MemFS.ReadDir(name)
}

func TestUpdatesDuringAScanWinOverWhatTheScanRead(t *testing.T) {
	mem := seamtest.NewMemFS(map[string]string{
		"/vault/kept.md":    "- [ ] from disk\n",
		"/vault/deleted.md": "- [ ] deleted while scanning\n",
	})
	g := gatedFS{MemFS: mem, entered: make(chan struct{}), release: make(chan struct{})}
	ix := index.New(g, "/vault")

	errc := make(chan error)
	go func() { errc <- ix.Scan(context.Background()) }()
	<-g.entered
	ix.Update("/vault/kept.md", []byte("- [ ] from buffer\n"))
	ix.Remove("/vault/deleted.md")
	if ix.Ready() {
		t.Error("Ready while scanning")
	}
	close(g.release)
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if got := taskRows(ix.Tasks()); !reflect.DeepEqual(got, []string{"kept.md:0 [ ] from buffer"}) {
		t.Errorf("Tasks = %q", got)
	}
}
