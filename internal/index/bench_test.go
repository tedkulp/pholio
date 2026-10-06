package index_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
)

// genVault writes notes Notes of about 1.5KB each, spread over folders of
// about 80 Notes, each with a few Links and Tasks. No CI gate: the target is
// a warm 50k scan in roughly 50-220ms.
func genVault(b *testing.B, notes int) string {
	b.Helper()
	root := b.TempDir()
	filler := strings.Repeat("Some ordinary prose that is neither a Link nor a Task. ", 20)
	for i := range notes {
		dir := filepath.Join(root, fmt.Sprintf("d%02d", i%8), fmt.Sprintf("f%03d", i/640))
		if i%640 < 8 { // first Note in this folder
			if err := os.MkdirAll(dir, 0o755); err != nil {
				b.Fatal(err)
			}
		}
		body := fmt.Sprintf("# Note %d\n\n## Section\n\n%s\nSee [[note-%d]] and [[note-%d#Section|this]].\n\n"+
			"- [ ] task %d due:2026-10-%02d #tag%d\n- [x] done %d done:2026-10-01\n\n```\n- [ ] code\n```\n%s\n",
			i, filler, (i+1)%notes, (i*7)%notes, i, i%28+1, i%10, i, filler)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("note-%d.md", i)), []byte(body), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func BenchmarkScan50k(b *testing.B) {
	root := genVault(b, 50_000)
	for b.Loop() {
		ix := index.New(seam.OSFS{}, root)
		if err := ix.Scan(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBacklinks50k(b *testing.B) {
	root := genVault(b, 50_000)
	ix := index.New(seam.OSFS{}, root)
	if err := ix.Scan(context.Background()); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if len(ix.Backlinks("d01/f000/note-1.md")) == 0 {
			b.Fatal("no Backlinks")
		}
	}
}
