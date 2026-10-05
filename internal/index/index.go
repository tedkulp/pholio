package index

import (
	"context"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/tedkulp/pholio/internal/seam"
)

// Index is the in-memory Vault index. It is safe for concurrent use: Scan
// runs in the background while the UI queries and updates it.
type Index struct {
	fsys seam.FS
	root string

	mu sync.RWMutex
	// templatesDir is the folder of daily_template. Its Tasks are left out
	// of Tasks, as are those under templates/; "" when it is templates/ or
	// the Vault root.
	templatesDir string
	notes        map[string]*Note    // by Vault-relative path
	byName       map[string][]string // lowercased Name -> paths, conflicts excluded
	// While a Scan runs, touched records paths changed through Update or
	// Remove, so the Scan's result doesn't overwrite them.
	scanning int
	touched  map[string]bool

	ready     bool
	done      chan struct{}
	closeDone sync.Once
}

// Option configures an Index.
type Option func(*Index)

// templatesFolder is the folder whose Tasks never count, whatever
// daily_template says.
const templatesFolder = "templates"

// WithTemplatesDir sets the Vault-relative folder of daily_template. Its
// Notes' Tasks are left out of Tasks, as are those under templates/.
func WithTemplatesDir(dir string) Option {
	return func(ix *Index) { ix.templatesDir = cleanDir(dir) }
}

// SetTemplatesDir changes the folder WithTemplatesDir set, after the config
// was reloaded.
func (ix *Index) SetTemplatesDir(dir string) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.templatesDir = cleanDir(dir)
}

func cleanDir(dir string) string {
	dir = strings.Trim(path.Clean(filepath.ToSlash(dir)), "/")
	if dir == "." || dir == templatesFolder {
		return ""
	}
	return dir
}

// New returns an empty Index of the Vault at root. Call Scan to fill it.
func New(fsys seam.FS, root string, opts ...Option) *Index {
	ix := &Index{
		fsys:   fsys,
		root:   root,
		notes:  map[string]*Note{},
		byName: map[string][]string{},
		done:   make(chan struct{}),
	}
	for _, o := range opts {
		o(ix)
	}
	return ix
}

// Ready reports whether the first Scan has finished. Until then UIs show
// "indexing…" in place of Task, Backlink and search results.
func (ix *Index) Ready() bool {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.ready
}

// Done is closed when the first Scan finishes.
func (ix *Index) Done() <-chan struct{} { return ix.done }

// Root returns the Vault's root directory.
func (ix *Index) Root() string { return ix.root }

// Scan reads every Note in the Vault in parallel and replaces the index with
// the result. Dot-directories and non-.md files are skipped. Updates made
// while it runs win over what it read. Scan is also the full rescan.
func (ix *Index) Scan(ctx context.Context) error {
	ix.mu.Lock()
	if ix.scanning == 0 {
		ix.touched = map[string]bool{}
	}
	ix.scanning++
	ix.mu.Unlock()

	notes, err := ix.scan(ctx)

	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.scanning--
	if err != nil {
		return err
	}
	for p := range ix.touched {
		if n, ok := ix.notes[p]; ok {
			notes[p] = n
		} else {
			delete(notes, p)
		}
	}
	if ix.scanning == 0 {
		ix.touched = nil
	}
	ix.notes = notes
	ix.byName = map[string][]string{}
	for p, n := range notes {
		ix.addName(p, n)
	}
	ix.ready = true
	ix.closeDone.Do(func() { close(ix.done) })
	return nil
}

// scan walks the Vault on one goroutine and parses files on several.
func (ix *Index) scan(ctx context.Context) (map[string]*Note, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	paths := make(chan string, 256)
	results := make(chan *Note, 256)
	workers := max(4, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for rel := range paths {
				data, err := ix.fsys.ReadFile(ix.abs(rel))
				if err != nil {
					continue // gone since the walk; the watcher will tell us
				}
				n := Parse(rel, data)
				results <- &n
			}
		})
	}

	walkErr := make(chan error, 1)
	go func() {
		defer close(paths)
		walkErr <- ix.walk(ctx, "", paths)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	notes := map[string]*Note{}
	for n := range results {
		notes[n.Path] = n
	}
	if err := <-walkErr; err != nil {
		return nil, err
	}
	return notes, ctx.Err()
}

func (ix *Index) walk(ctx context.Context, rel string, out chan<- string) error {
	entries, err := ix.fsys.ReadDir(ix.abs(rel))
	if err != nil {
		if rel == "" {
			return err
		}
		return nil // a subfolder vanished mid-walk
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		child := path.Join(rel, name)
		if e.IsDir() {
			if err := ix.walk(ctx, child, out); err != nil {
				return err
			}
			continue
		}
		if !IsNote(name) {
			continue
		}
		select {
		case out <- child:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (ix *Index) abs(rel string) string {
	return filepath.Join(ix.root, filepath.FromSlash(rel))
}

// addName records n under its name for resolution. Callers hold mu.
func (ix *Index) addName(p string, n *Note) {
	if n.Conflict {
		return
	}
	k := strings.ToLower(n.Name)
	ix.byName[k] = append(ix.byName[k], p)
}

// removeName undoes addName. Callers hold mu.
func (ix *Index) removeName(p string, n *Note) {
	k := strings.ToLower(n.Name)
	ps := slices.DeleteFunc(ix.byName[k], func(q string) bool { return q == p })
	if len(ps) == 0 {
		delete(ix.byName, k)
	} else {
		ix.byName[k] = ps
	}
}

// Note returns the Note at the Vault-relative path p.
func (ix *Index) Note(p string) (Note, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	n, ok := ix.notes[p]
	if !ok {
		return Note{}, false
	}
	return *n, true
}

// Update re-parses the Note at the OS path p from contents, so an open
// buffer feeds the index. The caller debounces. Paths outside the Vault, in
// dot-directories or not ending in ".md" are ignored.
func (ix *Index) Update(p string, contents []byte) {
	rel, ok := ix.rel(p)
	if !ok {
		return
	}
	n := Parse(rel, contents)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if old, ok := ix.notes[rel]; ok {
		ix.removeName(rel, old)
	}
	ix.notes[rel] = &n
	ix.addName(rel, &n)
	ix.touch(rel)
}

// Remove drops the Note at the OS path p, e.g. after it was deleted on disk.
func (ix *Index) Remove(p string) {
	rel, ok := ix.rel(p)
	if !ok {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if old, ok := ix.notes[rel]; ok {
		ix.removeName(rel, old)
		delete(ix.notes, rel)
	}
	ix.touch(rel)
}

func (ix *Index) touch(rel string) {
	if ix.scanning > 0 {
		ix.touched[rel] = true
	}
}

// rel turns an OS path into a Vault-relative slash path, if it names a file
// the index covers.
func (ix *Index) rel(p string) (string, bool) {
	if !filepath.IsAbs(p) && !path.IsAbs(filepath.ToSlash(p)) {
		return "", false
	}
	r, err := filepath.Rel(ix.root, p)
	if err != nil {
		return "", false
	}
	r = filepath.ToSlash(r)
	if r == ".." || strings.HasPrefix(r, "../") || !IsNote(r) {
		return "", false
	}
	for seg := range strings.SplitSeq(r, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", false
		}
	}
	return r, true
}

// Tasks returns every Task in the Vault, sorted by path and line. Tasks in
// Conflict Notes and under the templates folder are left out.
func (ix *Index) Tasks() []Task {
	ix.mu.RLock()
	var out []Task
	for p, n := range ix.notes {
		if n.Conflict || ix.inTemplates(p) {
			continue
		}
		out = append(out, n.Tasks...)
	}
	ix.mu.RUnlock()
	slices.SortFunc(out, func(a, b Task) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	return out
}

// inTemplates reports whether p is under templates/ or daily_template's
// folder. Callers hold mu.
func (ix *Index) inTemplates(p string) bool {
	return strings.HasPrefix(p, templatesFolder+"/") ||
		ix.templatesDir != "" && strings.HasPrefix(p, ix.templatesDir+"/")
}

// Notes returns every Note, sorted by path. Conflict Notes are included.
func (ix *Index) Notes() []Note {
	ix.mu.RLock()
	out := make([]Note, 0, len(ix.notes))
	for _, n := range ix.notes {
		out = append(out, *n)
	}
	ix.mu.RUnlock()
	slices.SortFunc(out, func(a, b Note) int { return strings.Compare(a.Path, b.Path) })
	return out
}
