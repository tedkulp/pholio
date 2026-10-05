package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
)

// tree is the sidebar's entries as plain text, one per row, without the
// header, padding or blank rows.
func tree(m app.Model) []string {
	var out []string
	r := rows(m)
	for _, row := range r[1 : len(r)-2] {
		left, _, ok := strings.Cut(row, "│")
		if !ok {
			return nil
		}
		if s := strings.TrimRight(left, " "); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func TestTreeListsFoldersFirstCaseInsensitively(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	want := []string{" ▸ daily", " ▸ Projects", " ▸ zettel", "   a", "   Beta", "   notes.txt"}
	if got := tree(m); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tree =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTreeNavigation(t *testing.T) {
	m := keys(shell(t, shellVault(), "/vault/a.md"), ctrlH)

	m = typeKeys(m, "gjl") // expand Projects
	if got := tree(m); len(got) < 4 || got[2] != "   ▸ sub" || got[3] != "     plan" {
		t.Fatalf("after l on Projects, tree = %q", got)
	}

	m = typeKeys(m, "jjh") // on plan: h jumps to Projects and collapses it
	if got := tree(m); got[1] != " ▸ Projects" || got[2] != " ▸ zettel" {
		t.Fatalf("after h on plan, tree = %q", got)
	}

	m = keys(typeKeys(m, "k"), enter) // enter toggles daily open
	if got := tree(m); got[1] != "     2026-10-04" {
		t.Fatalf("after enter on daily, tree = %q", got)
	}
	m = typeKeys(m, "h") // h on an open folder collapses it
	if got := tree(m); got[1] != " ▸ Projects" {
		t.Fatalf("after h on daily, tree = %q", got)
	}

	m = keys(typeKeys(m, "lj"), enter) // open daily/2026-10-04
	if got := rows(m)[0]; !strings.Contains(got, "│# Sunday") {
		t.Errorf("first row = %q, want the Daily Note open", got)
	}
	if m.View().Cursor == nil {
		t.Error("opening a Note did not focus the editor")
	}
}

func TestDotToggleShowsDotfilesButNeverPholio(t *testing.T) {
	m := keys(shell(t, shellVault(), "/vault/a.md"), ctrlH)

	m = typeKeys(m, ".")
	got := strings.Join(tree(m), "\n")
	if !strings.Contains(got, "   .hidden") {
		t.Errorf("dotfiles not shown:\n%s", got)
	}
	if strings.Contains(got, ".pholio") {
		t.Errorf(".pholio shown:\n%s", got)
	}
	if !strings.Contains(rows(m)[0], "vault (all)") {
		t.Errorf("header = %q, want it to say dotfiles are shown", rows(m)[0])
	}

	m = typeKeys(m, ".")
	if strings.Contains(strings.Join(tree(m), "\n"), ".hidden") {
		t.Error("second . did not hide dotfiles again")
	}
}

func TestResizeSavesTheWidth(t *testing.T) {
	fsys := shellVault()
	m := keys(shell(t, fsys, "/vault/a.md"), ctrlH)

	m = typeKeys(m, ">")
	if i := strings.Index(rows(m)[0], "│"); i != 33 {
		t.Errorf("border at %d, want 33 after widening to 34", i)
	}
	data, err := fsys.ReadFile("/home/u/.local/state/pholio/state.toml")
	if err != nil || !strings.Contains(string(data), "sidebar_width = 34") {
		t.Fatalf("state = %q, %v; want sidebar_width = 34", data, err)
	}

	again := shell(t, fsys, "/vault/a.md")
	if i := strings.Index(rows(again)[0], "│"); i != 33 {
		t.Errorf("after restart border at %d, want the saved width", i)
	}

	m = typeKeys(m, "<<<<<<")
	if i := strings.Index(rows(m)[0], "│"); i != 15 {
		t.Errorf("border at %d, want the 16-column minimum", i)
	}
}

func TestOpeningANoteOverADirtyBufferAsks(t *testing.T) {
	open := func(t *testing.T, answer string) (app.Model, map[string]string) {
		t.Helper()
		fsys := shellVault()
		m := shell(t, fsys, "/vault/a.md")
		m = keys(typeKeys(m, "dd"), ctrlH)
		m = typeKeys(m, "G")
		m = keys(typeKeys(m, "k"), enter) // Beta
		if got := messageLine(m); got != "Save changes to a.md? y save · n discard · esc cancel" {
			t.Fatalf("message = %q, want the question", got)
		}
		if answer == "esc" {
			m = keys(m, esc)
		} else {
			m = typeKeys(m, answer)
		}
		a, _ := fsys.ReadFile("/vault/a.md")
		return m, map[string]string{"a.md": string(a), "row": rows(m)[0]}
	}

	m, got := open(t, "y")
	if got["a.md"] != "first note\n" || !strings.Contains(got["row"], "│# Beta") {
		t.Errorf("y: a.md = %q, first row %q", got["a.md"], got["row"])
	}
	if strings.Contains(rows(m)[10], "[+]") {
		t.Error("y: new buffer is dirty")
	}

	_, got = open(t, "n")
	if got["a.md"] != "# Alpha\nfirst note\n" || !strings.Contains(got["row"], "│# Beta") {
		t.Errorf("n: a.md = %q, first row %q", got["a.md"], got["row"])
	}

	m, got = open(t, "esc")
	if !strings.Contains(got["row"], "│first note") {
		t.Errorf("esc: first row = %q, want a.md still open", got["row"])
	}
	if messageLine(m) != "" {
		t.Errorf("esc left the question up: %q", messageLine(m))
	}
}

func TestANoteOpenedFromTheSidebarFollowsDiskChanges(t *testing.T) {
	fsys := shellVault()
	m := keys(shell(t, fsys, "/vault/a.md"), ctrlH)
	m = keys(typeKeys(m, "Gk"), enter) // Beta
	if err := fsys.WriteFile("/vault/Beta.md", []byte("# Beta v2\n")); err != nil {
		t.Fatal(err)
	}

	next, _ := m.Update(tea.FocusMsg{})

	if got := next.(app.Model).Text(); got != "# Beta v2\n" {
		t.Errorf("buffer = %q, want Beta reloaded from disk", got)
	}
}

func TestOpeningTheOpenNoteKeepsItsBuffer(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")
	m = keys(typeKeys(m, "dd"), ctrlH)

	m = keys(typeKeys(m, "Gkk"), enter) // a

	if got := rows(m)[0]; !strings.Contains(got, "│first note") || messageLine(m) != "" {
		t.Errorf("first row = %q, message %q; want the edited buffer, no question", got, messageLine(m))
	}
}

func TestCtrlQOnADirtyBufferAsks(t *testing.T) {
	fsys := shellVault()
	m := typeKeys(shell(t, fsys, "/vault/a.md"), "dd")

	next, cmd := m.Update(ctrlQ)
	if cmd != nil {
		t.Fatal("ctrl+q on a dirty buffer quit without asking")
	}
	m = next.(app.Model)
	if got := messageLine(m); got != "Save changes to a.md before quitting? y save · n discard · esc cancel" {
		t.Fatalf("message = %q", got)
	}

	_, cmd = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if !quits(cmd) {
		t.Fatal("y did not quit")
	}
	if a, _ := fsys.ReadFile("/vault/a.md"); string(a) != "first note\n" {
		t.Errorf("a.md = %q, want it saved", a)
	}
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestColonQ(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")
	_, cmd := typeKeys(m, ":q").Update(enter)
	if !quits(cmd) {
		t.Error(":q on a clean buffer did not quit")
	}

	m = typeKeys(m, "dd:q")
	next, cmd := m.Update(enter)
	if cmd != nil {
		t.Fatal(":q on a dirty buffer quit without asking")
	}
	if got := messageLine(next.(app.Model)); !strings.HasPrefix(got, "Save changes to a.md before quitting?") {
		t.Errorf("message = %q", got)
	}

	_, cmd = typeKeys(m, "dd:q!").Update(enter)
	if !quits(cmd) {
		t.Error(":q! did not quit")
	}
}

func TestColonSidebarTogglesIt(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = keys(typeKeys(m, ":sidebar"), enter)

	if strings.Contains(screen(m), "▸ daily") {
		t.Errorf(":sidebar did not hide the sidebar:\n%s", screen(m))
	}
}

func TestCmdlineShowsOnTheMessageLine(t *testing.T) {
	m := typeKeys(shell(t, shellVault(), "/vault/a.md"), ":side")

	if got := messageLine(m); got != ":side" {
		t.Errorf("message line = %q", got)
	}
	if c := m.View().Cursor; c == nil || c.X != 5 || c.Y != 11 {
		t.Errorf("cursor = %+v, want it after the cmdline text", c)
	}
}
