package app_test

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

// find opens find Note (spc f) and types query.
func find(m app.Model, query string) app.Model {
	return typeKeys(press(m, space), "f"+query)
}

// paletteRows are the palette's list rows, trimmed: the lines between the
// input's rule and the footer.
func paletteRows(m app.Model) []string {
	var out []string
	in := false
	for _, r := range rows(m) {
		start := strings.Index(r, "│ ")
		end := strings.LastIndex(r, " │")
		if start < 0 || end <= start {
			continue
		}
		text := strings.TrimSpace(r[start+len("│ ") : end])
		switch {
		case strings.HasPrefix(text, "───"):
			in = true
		case strings.Contains(text, "esc close"):
			in = false
		case in && text != "":
			out = append(out, text)
		}
	}
	return out
}

func TestFindShowsIndexingUntilTheScanIsReady(t *testing.T) {
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	m := find(openIn(t, &v, "index.md"), "alp")
	if got := paletteRows(m); !slices.Equal(got, []string{"indexing…"}) {
		t.Fatalf("rows before the scan = %q, want indexing…", got)
	}
	m = drive(t, m, m.Init())
	if got := paletteRows(m); len(got) == 0 || !strings.HasPrefix(got[0], "alpha.md") {
		t.Fatalf("rows after the scan = %q, want alpha.md first", got)
	}
}

func TestFindMatchesFilenameAndTitleAndOpensTheNote(t *testing.T) {
	m, v := linked(t, "index.md")
	m = find(m, "dup in b") // only the title says "b"
	if got := paletteRows(m); len(got) == 0 || !strings.HasPrefix(got[0], "b/c/dup.md") {
		t.Fatalf("rows = %q, want b/c/dup.md first", got)
	}
	m = press(m, enter)
	if got := v.rel(t, m); got != "b/c/dup.md" {
		t.Fatalf("open Note = %q, want b/c/dup.md", got)
	}
	m = press(m, ctrlO)
	if got := v.rel(t, m); got != "index.md" {
		t.Errorf("ctrl+o went to %q, want index.md: the jump wasn't recorded", got)
	}
}

func TestFindWithAnEmptyQueryListsNotesOpenedThisSessionNewestFirst(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(find(m, "gamma"), enter)
	m = press(find(m, "beta"), enter)
	m = press(find(m, "alpha"), enter)
	m = press(find(m, "beta"), enter) // opened again: it moves to the front
	if got := v.rel(t, m); got != "deep/er/beta.md" {
		t.Fatalf("open Note = %q, want deep/er/beta.md", got)
	}
	m = find(m, "")
	got := paletteRows(m)
	want := []string{"alpha.md", "sub/gamma.md", "index.md"} // the open Note is left out
	if len(got) != len(want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Fatalf("rows = %q, want %q", got, want)
		}
	}
	m = press(m, enter)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Errorf("enter on the first recent Note opened %q, want alpha.md", got)
	}
}

func TestFindOffersANewNoteWhenNothingMatches(t *testing.T) {
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	if err := os.MkdirAll(v.abs(".pholio"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v.abs(".pholio/config.toml"), []byte("new_note_folder = \"inbox\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := openIn(t, &v, "index.md")
	m = drive(t, m, m.Init())

	m = find(m, "Big idea")
	got := paletteRows(m)
	if len(got) == 0 || got[len(got)-1] != `+ new Note "Big idea"` {
		t.Fatalf("rows = %q, want the last to be + new Note \"Big idea\"", got)
	}
	m = press(m, enter)
	if got := v.rel(t, m); got != "inbox/Big idea.md" {
		t.Fatalf("open Note = %q, want inbox/Big idea.md", got)
	}
	if _, err := os.Stat(v.abs("inbox/Big idea.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the new Note is on disk before :w (err %v)", err)
	}
	if msg := messageLine(m); !strings.Contains(msg, ":w to create it") {
		t.Errorf("message = %q, want a hint to :w", msg)
	}
}

func TestFindCommandOpensThePaletteWithTheQuery(t *testing.T) {
	m, v := linked(t, "index.md")
	m = ex(m, "find gamma")
	if got := paletteRows(m); len(got) == 0 || !strings.HasPrefix(got[0], "sub/gamma.md") {
		t.Fatalf("rows = %q, want sub/gamma.md", got)
	}
	m = press(m, enter)
	if got := v.rel(t, m); got != "sub/gamma.md" {
		t.Errorf("open Note = %q, want sub/gamma.md", got)
	}
}
