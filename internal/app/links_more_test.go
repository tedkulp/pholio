package app_test

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

func TestGdFollowsAnAliasedLinkToItsHeading(t *testing.T) {
	m, v := linked(t, "index.md")
	m = typeKeys(search(m, "the intro"), "gd")
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("open Note = %q, want alpha.md", got)
	}
	if got := m.Cursor(); got != (engine.Pos{Line: 2}) {
		t.Errorf("cursor = %+v, want the ## Intro heading at line 2", got)
	}
	if m.Text() != "# Alpha\n\n## Intro\n\nAlpha points on to [[beta]].\n" {
		t.Errorf("gd changed the buffer: %q", m.Text())
	}
}

func TestEnterFollowsRelativeMarkdownLinks(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(search(m, "gamma]"), enter)
	if got := v.rel(t, m); got != "sub/gamma.md" {
		t.Fatalf("open Note = %q, want sub/gamma.md", got)
	}
	m = press(search(m, "alpha]"), enter)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("from gamma: open Note = %q, want alpha.md", got)
	}
}

func TestFolderPrefixPicksOneOfTwoNotesWithTheSameName(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(search(m, "c/dup"), enter)
	if got := v.rel(t, m); got != "b/c/dup.md" {
		t.Fatalf("open Note = %q, want b/c/dup.md", got)
	}
	if msg := messageLine(m); msg != "" {
		t.Errorf("message = %q, want none", msg)
	}
}

func TestAmbiguousNameOpensTheShortestPathAndWarns(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(search(m, "dup]]"), enter)
	if got := v.rel(t, m); got != "a/dup.md" {
		t.Fatalf("open Note = %q, want a/dup.md", got)
	}
	if msg := messageLine(m); !strings.Contains(msg, "ambiguous") || !strings.Contains(msg, "b/c/dup.md") {
		t.Errorf("message = %q, want an ambiguity warning naming b/c/dup.md", msg)
	}
}

func TestURLGoesToTheOpener(t *testing.T) {
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	web := "See https://example.com/a?b=1, or [docs](http://docs.example.org/x).\n"
	if err := os.WriteFile(v.abs("web.md"), []byte(web), 0o600); err != nil {
		t.Fatal(err)
	}
	m := openIn(t, &v, "web.md")
	m = drive(t, m, m.Init())
	m = press(search(m, "example.com"), enter)
	m = typeKeys(search(m, "docs"), "gd")
	want := []string{"https://example.com/a?b=1", "http://docs.example.org/x"}
	if got := v.opener.Opened(); !slices.Equal(got, want) {
		t.Errorf("opened %q, want %q", got, want)
	}
	if got := v.rel(t, m); got != "web.md" {
		t.Errorf("open Note = %q, want web.md still", got)
	}
}

func TestEnterOffALinkMovesDown(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(m, enter)
	if got := v.rel(t, m); got != "index.md" {
		t.Fatalf("open Note = %q, want index.md", got)
	}
	if got := m.Cursor(); got.Line != 1 {
		t.Errorf("cursor = %+v, want line 1", got)
	}
}

func TestDanglingLinkOpensAnUnsavedBufferUntilW(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(search(m, "missing"), enter)
	if got := v.rel(t, m); got != "missing.md" {
		t.Fatalf("open Note = %q, want missing.md", got)
	}
	if _, err := os.Stat(v.abs("missing.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing.md is on disk before :w (err %v)", err)
	}
	ex(press(typeKeys(m, "inew text"), esc), "w")
	data, err := os.ReadFile(v.abs("missing.md"))
	if err != nil || string(data) != "new text\n" {
		t.Fatalf("after :w missing.md = %q, %v", data, err)
	}
	if _, ok := v.ix.Note("missing.md"); !ok {
		t.Error("the index doesn't know the written Note")
	}
}

func TestDanglingLinkGoesInTheNewNoteFolder(t *testing.T) {
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	if err := os.MkdirAll(v.abs(".pholio"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v.abs(".pholio/config.toml"), []byte("new_note_folder = \"inbox\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := openIn(t, &v, "index.md")
	m = drive(t, m, m.Init())
	m = press(search(m, "missing"), enter)
	if got := v.rel(t, m); got != "inbox/missing.md" {
		t.Fatalf("open Note = %q, want inbox/missing.md", got)
	}
	if _, err := os.Stat(v.abs("inbox")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("inbox/ exists before :w (err %v)", err)
	}
	ex(m, "w")
	if _, err := os.Stat(v.abs("inbox/missing.md")); err != nil {
		t.Fatalf(":w didn't create inbox/missing.md: %v", err)
	}
}

func TestWikiLinksWaitForTheIndex(t *testing.T) {
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	m := openIn(t, &v, "index.md")
	m = press(search(m, "alpha]]"), enter)
	if got := v.rel(t, m); got != "index.md" {
		t.Fatalf("followed to %q before the index was ready", got)
	}
	if msg := messageLine(m); msg != "indexing…" {
		t.Errorf("message = %q, want indexing…", msg)
	}
	m = drive(t, m, m.Init())
	m = press(m, enter)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("after the scan: open Note = %q, want alpha.md", got)
	}
}

var (
	ctrlO = tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}
	ctrlI = tea.KeyPressMsg{Code: 'i', Mod: tea.ModCtrl}
)

func TestJumplistGoesBackAndForward(t *testing.T) {
	m, v := linked(t, "index.md")
	m = search(m, "alpha]]")
	start := m.Cursor()
	m = press(m, enter) // index → alpha
	m = search(m, "beta")
	inAlpha := m.Cursor()
	m = press(m, enter) // alpha → beta
	if got := v.rel(t, m); got != "deep/er/beta.md" {
		t.Fatalf("open Note = %q, want deep/er/beta.md", got)
	}
	m = typeKeys(m, "jj")
	inBeta := m.Cursor()

	steps := []struct {
		key  tea.KeyPressMsg
		path string
		pos  engine.Pos
	}{
		{ctrlO, "alpha.md", inAlpha},
		{ctrlO, "index.md", start},
		{ctrlO, "index.md", start}, // the oldest entry: stays
		{tab, "alpha.md", inAlpha},
		{ctrlI, "deep/er/beta.md", inBeta},
		{tab, "deep/er/beta.md", inBeta}, // the newest: stays
	}
	for i, s := range steps {
		m = press(m, s.key)
		if got := v.rel(t, m); got != s.path || m.Cursor() != s.pos {
			t.Fatalf("step %d (%s): at %s %+v, want %s %+v", i, s.key, got, m.Cursor(), s.path, s.pos)
		}
	}
}

func TestANewJumpDropsTheForwardHistory(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(search(m, "alpha]]"), enter)
	m = press(m, ctrlO)
	m = press(search(m, "gamma]"), enter)
	m = press(m, tab)
	if got := v.rel(t, m); got != "sub/gamma.md" {
		t.Fatalf("tab after a new jump went to %q", got)
	}
	m = press(m, ctrlO)
	if got := v.rel(t, m); got != "index.md" {
		t.Fatalf("ctrl+o = %q, want index.md", got)
	}
}

func TestOpeningFromTheSidebarIsRecorded(t *testing.T) {
	m, v := linked(t, "deep/er/beta.md")
	m = keys(m, ctrlH)
	m = typeKeys(m, "gg") // a/ b/ deep/ (open) er/ beta.md sub/ alpha.md
	for !strings.HasSuffix(m.SidebarSelected(), "alpha.md") {
		m = typeKeys(m, "j")
	}
	m = press(m, enter)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("sidebar opened %q, want alpha.md", got)
	}
	m = press(m, ctrlO)
	if got := v.rel(t, m); got != "deep/er/beta.md" {
		t.Errorf("ctrl+o after a sidebar open = %q, want deep/er/beta.md", got)
	}
}

func TestJumpBackAsksAboutADirtyBuffer(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(search(m, "alpha]]"), enter)
	m = press(typeKeys(m, "ihi"), esc)
	m = press(m, ctrlO)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("switched to %q without asking", got)
	}
	m = press(m, esc) // cancel
	m = press(m, tab)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("cancelled ctrl+o moved the jumplist: tab went to %q", got)
	}
	m = typeKeys(press(m, ctrlO), "n")
	if got := v.rel(t, m); got != "index.md" {
		t.Fatalf("after n: %q, want index.md", got)
	}
	m = press(m, tab)
	if got := v.rel(t, m); got != "alpha.md" || strings.HasPrefix(m.Text(), "hi") {
		t.Fatalf("forward: %q %q, want alpha.md as on disk", got, m.Text())
	}
}

func TestOpenBufferEditsReachTheIndex(t *testing.T) {
	m, v := linked(t, "index.md")
	m = typeKeys(m, "Go")
	var cmd tea.Cmd
	for _, r := range "[[zeta]]" {
		var next tea.Model
		next, cmd = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(app.Model)
	}
	if hasLinkTo(v.ix, "zeta") {
		t.Fatal("the index was fed before the debounce")
	}
	m = drive(t, m, cmd)
	if !hasLinkTo(v.ix, "zeta") {
		t.Fatal("the index never saw the edit")
	}
	// Discarding the edit puts the file back.
	m = press(m, esc)
	m = typeKeys(press(search(m, "alpha]]"), enter), "n")
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("open Note = %q, want alpha.md", got)
	}
	if hasLinkTo(v.ix, "zeta") {
		t.Error("the discarded edit is still in the index")
	}
}

func hasLinkTo(ix *index.Index, target string) bool {
	n, _ := ix.Note("index.md")
	for _, l := range n.Links {
		if l.Target == target {
			return true
		}
	}
	return false
}
