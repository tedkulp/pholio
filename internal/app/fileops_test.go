package app_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

// diskTrash is the fake Trash over a real temp Vault: it records each path,
// as seamtest.Trash does, and removes it so the rest of the app sees it gone.
type diskTrash struct{ fake seamtest.Trash }

func (d *diskTrash) Trash(p string) error {
	if err := d.fake.Trash(p); err != nil {
		return err
	}
	return os.RemoveAll(p)
}

func (d *diskTrash) Trashed() []string { return d.fake.Trashed() }

// opsVault is a copy of the links fixture with the app open on file, the
// index scanned and a fake Trash.
func opsVault(t *testing.T, file string) (app.Model, linksVault, *diskTrash) {
	t.Helper()
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	trash := &diskTrash{}
	fsys := seam.OSFS{}
	v.ix = index.New(fsys, v.root)
	s, err := config.Startup(fsys, dirs, "/home/u", v.abs(file))
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Opener: v.opener, Index: v.ix, Trash: trash}, v.abs(file))
	if err != nil {
		t.Fatal(err)
	}
	m = resize(m.WithSession(s), 100, 14)
	return drive(t, m, m.Init()), v, trash
}

func (v linksVault) read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(v.abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (v linksVault) exists(rel string) bool {
	_, err := os.Stat(v.abs(rel))
	return err == nil
}

var ctrlU = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}

// selectInTree focuses the sidebar and moves the selection to rel.
func selectInTree(t *testing.T, m app.Model, v linksVault, rel string) app.Model {
	t.Helper()
	m = typeKeys(keys(m, ctrlH), "g")
	for range 40 {
		if m.SidebarSelected() == v.abs(rel) {
			return m
		}
		m = typeKeys(m, "j")
	}
	t.Fatalf("%s is not in the tree", rel)
	return m
}

func TestRenameAsksThenRewritesEveryLinkForm(t *testing.T) {
	m, v, _ := opsVault(t, "alpha.md")

	m = ex(m, "rename omega.md")
	if got := messageLine(m); got != "Update 3 Links in 2 Notes? [Y/n]" {
		t.Fatalf("message = %q, want the update prompt", got)
	}
	if !v.exists("alpha.md") {
		t.Fatal("renamed before the answer")
	}
	m = press(m, enter)

	if v.exists("alpha.md") || !v.exists("omega.md") {
		t.Fatal("alpha.md was not renamed to omega.md")
	}
	if got := v.rel(t, m); got != "omega.md" {
		t.Errorf("open Note = %q, want omega.md", got)
	}
	want := "# Index\n\nStart at [[omega]] or [[omega#Intro|the intro]].\nThen [[beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n"
	if got := v.read(t, "index.md"); got != want {
		t.Errorf("index.md =\n%s\nwant\n%s", got, want)
	}
	if got := v.read(t, "sub/gamma.md"); got != "# Gamma\n\nBack up to [alpha](../omega.md).\n" {
		t.Errorf("sub/gamma.md = %q", got)
	}
	if got := v.read(t, "alpha.sync-conflict-20261001-101010-ABCDEFG.md"); !strings.Contains(got, "[[beta]]") {
		t.Errorf("Conflict Note changed: %q", got)
	}
	if bl := v.ix.Backlinks("omega.md"); len(bl) != 3 {
		t.Errorf("index has %d Backlinks to omega.md, want 3", len(bl))
	}
}

func TestRenameAnsweredNoLeavesLinksAlone(t *testing.T) {
	m, v, _ := opsVault(t, "alpha.md")
	_ = typeKeys(ex(m, "rename omega"), "n") // .md is added
	if !v.exists("omega.md") {
		t.Fatal("not renamed")
	}
	if got := v.read(t, "index.md"); !strings.Contains(got, "[[alpha]]") {
		t.Errorf("index.md was rewritten: %q", got)
	}
}

func TestRenameEscCancels(t *testing.T) {
	m, v, _ := opsVault(t, "alpha.md")
	m = press(ex(m, "rename omega.md"), esc)
	if !v.exists("alpha.md") || v.exists("omega.md") {
		t.Error("esc did not cancel the rename")
	}
	if got := v.rel(t, m); got != "alpha.md" {
		t.Errorf("open Note = %q", got)
	}
}

func TestRenameUsesTheShortestUniqueName(t *testing.T) {
	m, v, _ := opsVault(t, "b/c/dup.md")
	_ = press(ex(m, "rename b/d/dup.md"), enter)
	if got := v.read(t, "index.md"); !strings.Contains(got, "[[dup]], [[d/dup]] and") {
		t.Errorf("index.md = %q, want [[c/dup]] → [[d/dup]] and [[dup]] kept", got)
	}
}

func TestMovingANoteBetweenFoldersRewritesItsOwnRelativeLinks(t *testing.T) {
	m, v, _ := opsVault(t, "sub/gamma.md")
	m = ex(m, "rename gamma.md")
	if got := messageLine(m); got != "Update 2 Links in 2 Notes? [Y/n]" {
		t.Fatalf("message = %q", got)
	}
	m = press(m, enter)
	if got := v.read(t, "gamma.md"); got != "# Gamma\n\nBack up to [alpha](alpha.md).\n" {
		t.Errorf("gamma.md = %q", got)
	}
	if m.Text() != "# Gamma\n\nBack up to [alpha](alpha.md).\n" {
		t.Errorf("open buffer = %q, want the rewrite", m.Text())
	}
	if m.Dirty() {
		t.Error("a clean buffer was left dirty")
	}
	if got := v.read(t, "index.md"); !strings.Contains(got, "[gamma](gamma.md)") {
		t.Errorf("index.md = %q", got)
	}
}

func TestRenameFromTheSidebarEditsTheOpenDirtyBufferNotItsFile(t *testing.T) {
	m, v, _ := opsVault(t, "index.md")
	m = press(typeKeys(m, "Go[[alpha]] again"), esc)
	onDisk := v.read(t, "index.md")

	m = selectInTree(t, m, v, "alpha.md")
	m = typeKeys(m, "r")
	if q := paletteInput(m); !strings.Contains(q, "alpha.md") {
		t.Fatalf("rename prompt %q is not prefilled with alpha.md", q)
	}
	m = typeKeys(keys(m, ctrlU), "deep/omega.md")
	m = press(m, enter)
	if got := messageLine(m); got != "Update 4 Links in 2 Notes? [Y/n]" {
		t.Fatalf("message = %q", got)
	}
	m = typeKeys(m, "y")

	if !v.exists("deep/omega.md") {
		t.Fatal("not moved")
	}
	if got := v.read(t, "index.md"); got != onDisk {
		t.Errorf("the dirty Note was written to disk:\n%s", got)
	}
	if !m.Dirty() {
		t.Error("the buffer is no longer dirty")
	}
	if got := m.Text(); !strings.Contains(got, "Start at [[omega]] or [[omega#Intro|the intro]].") || !strings.Contains(got, "[[omega]] again") {
		t.Errorf("buffer = %q, want both the saved and the typed Links rewritten", got)
	}
}

func TestRenamingAFileThatIsNotANoteRewritesNothing(t *testing.T) {
	m, v, _ := opsVault(t, "index.md")
	m = selectInTree(t, m, v, "notes.txt")
	m = typeKeys(keys(typeKeys(m, "r"), ctrlU), "more/notes.txt")
	m = press(m, enter)
	if !v.exists("more/notes.txt") || v.exists("notes.txt") {
		t.Fatal("notes.txt was not moved")
	}
	if got := messageLine(m); strings.Contains(got, "Update") {
		t.Errorf("asked to update Links: %q", got)
	}
}

func TestRenameRefusesAnExistingTarget(t *testing.T) {
	m, v, _ := opsVault(t, "alpha.md")
	m = ex(m, "rename index.md")
	if got := messageLine(m); !strings.Contains(got, "index.md already exists") {
		t.Errorf("message = %q", got)
	}
	if !v.exists("alpha.md") {
		t.Error("alpha.md is gone")
	}
}

func TestDeleteAsksWithTheBacklinkCountAndDefaultsToNo(t *testing.T) {
	m, v, trash := opsVault(t, "alpha.md")
	m = ex(m, "delete")
	if got := messageLine(m); got != "Delete alpha.md (3 Backlinks)? y/N" {
		t.Fatalf("message = %q", got)
	}
	m = press(m, enter)
	if len(trash.Trashed()) != 0 || !v.exists("alpha.md") {
		t.Fatal("enter deleted the Note")
	}

	m = typeKeys(ex(m, "delete"), "y")
	if got := trash.Trashed(); !slices.Equal(got, []string{v.abs("alpha.md")}) {
		t.Errorf("trashed %q, want alpha.md", got)
	}
	if m.Path() != "" {
		t.Errorf("the deleted Note is still open: %q", m.Path())
	}
	if bl := v.ix.Backlinks("alpha.md"); len(bl) != 0 {
		t.Errorf("index still resolves %d Links to alpha.md", len(bl))
	}
}

func TestDeleteAFolderFromTheSidebarCountsItsFiles(t *testing.T) {
	m, v, trash := opsVault(t, "index.md")
	m = selectInTree(t, m, v, "deep")
	m = typeKeys(m, "d")
	if got := messageLine(m); got != "Delete deep/ (1 file)? y/N" {
		t.Fatalf("message = %q", got)
	}
	m = typeKeys(m, "y")
	if got := trash.Trashed(); !slices.Equal(got, []string{v.abs("deep")}) {
		t.Errorf("trashed %q, want deep", got)
	}
	if got := v.rel(t, m); got != "index.md" {
		t.Errorf("open Note = %q, want index.md kept open", got)
	}
	for _, row := range tree(m) {
		if strings.Contains(row, "deep") {
			t.Errorf("tree still shows deep: %q", tree(m))
		}
	}
}

func TestNewOpensAnUnsavedBuffer(t *testing.T) {
	m, v, _ := opsVault(t, "index.md")
	m = ex(m, "new ideas/first")
	if got := v.rel(t, m); got != "ideas/first.md" {
		t.Fatalf("open Note = %q", got)
	}
	if v.exists("ideas/first.md") {
		t.Error("written before :w")
	}
}

func TestSpcNPromptsForTheName(t *testing.T) {
	m, v, _ := opsVault(t, "index.md")
	m = typeKeys(keys(m, space), "n")
	m = press(typeKeys(m, "plan"), enter)
	if got := v.rel(t, m); got != "plan.md" {
		t.Fatalf("open Note = %q", got)
	}
}

func TestSidebarAddCreatesANoteInTheSelectedFolderOrAFolder(t *testing.T) {
	m, v, _ := opsVault(t, "index.md")
	m = selectInTree(t, m, v, "sub")
	m = typeKeys(m, "a")
	if q := paletteInput(m); !strings.Contains(q, "sub/") {
		t.Fatalf("add prompt %q is not prefilled with sub/", q)
	}
	m = press(typeKeys(m, "fresh"), enter)
	if !v.exists("sub/fresh.md") {
		t.Fatal("sub/fresh.md not created")
	}
	if got := v.rel(t, m); got != "sub/fresh.md" {
		t.Errorf("open Note = %q", got)
	}

	m = selectInTree(t, m, v, "sub")
	_ = press(typeKeys(keys(typeKeys(m, "a"), ctrlU), "box/"), enter)
	if info, err := os.Stat(v.abs("box")); err != nil || !info.IsDir() {
		t.Errorf("folder box not created: %v", err)
	}
}

// paletteInput is the palette's input row, found by its "> " prompt, or
// the whole screen when none matches.
func paletteInput(m app.Model) string {
	for _, r := range rows(m) {
		if strings.Contains(r, "│ > ") {
			return r
		}
	}
	return strings.Join(rows(m), "\n")
}
