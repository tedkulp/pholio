package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

// shellVault is a small Vault with folders, non-Markdown files and dotfiles.
func shellVault() *seamtest.MemFS {
	return seamtest.NewMemFS(map[string]string{
		"/vault/a.md":                 "# Alpha\nfirst note\n",
		"/vault/Beta.md":              "# Beta\n",
		"/vault/notes.txt":            "plain\n",
		"/vault/.hidden.md":           "secret\n",
		"/vault/.pholio/config.toml":  "",
		"/vault/daily/2026-10-05.md":  "# Monday\n",
		"/vault/daily/2026-10-04.md":  "# Sunday\n",
		"/vault/zettel/idea.md":       "# Idea\n",
		"/vault/Projects/plan.md":     "# Plan\n",
		"/vault/Projects/sub/deep.md": "# Deep\n",
	})
}

// shell starts the app on file in the shell Vault at 80×12.
func shell(t *testing.T, fsys *seamtest.MemFS, file string) app.Model {
	t.Helper()
	s, err := config.Startup(fsys, dirs, "/home/u", file)
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys}, file)
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 80, 12)
}

func keys(m app.Model, ks ...tea.KeyPressMsg) app.Model {
	for _, k := range ks {
		m = press(m, k)
	}
	return m
}

var (
	ctrlH = tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl}
	ctrlL = tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}
	// ctrlShiftH and ctrlShiftL are what multiplexers that swallow ctrl+h/l
	// (herdr) pass through instead.
	ctrlShiftH = tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl | tea.ModShift}
	ctrlShiftL = tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl | tea.ModShift}
	ctrlQ      = tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl}
	space      = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	tab        = tea.KeyPressMsg{Code: tea.KeyTab}
	esc        = tea.KeyPressMsg{Code: tea.KeyEsc}
	enter      = tea.KeyPressMsg{Code: tea.KeyEnter}
)

// screen is the stripped view.
func screen(m app.Model) string { return ansi.Strip(m.View().Content) }

func TestSidebarBesideTheEditor(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	golden.RequireEqual(t, screen(m))
}

func TestSidebarRevealsTheOpenNote(t *testing.T) {
	m := shell(t, shellVault(), "/vault/Projects/sub/deep.md")

	s := screen(m)
	for _, want := range []string{"▾ Projects", "▾ sub", "deep"} {
		if !strings.Contains(s, want) {
			t.Errorf("screen lacks %q:\n%s", want, s)
		}
	}
}

func TestCtrlHFocusesTheSidebarAndCtrlLReturns(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = keys(m, ctrlH)
	if m.View().Cursor != nil {
		t.Error("terminal cursor shown while the sidebar has focus")
	}
	m = typeKeys(m, "Gl") // the last entry, notes.txt, is not a Note
	if got := messageLine(m); got != "notes.txt: not a Note" {
		t.Errorf("message = %q, want the sidebar to have handled G and l", got)
	}

	m = keys(m, ctrlL)
	if c := m.View().Cursor; c == nil || c.X != 30 || c.Y != 0 {
		t.Errorf("cursor = %+v, want the editor's at 30,0", c)
	}
}

func TestCtrlShiftHAndLAlsoMoveFocus(t *testing.T) {
	m := keys(shell(t, shellVault(), "/vault/a.md"), ctrlShiftH)
	if m.View().Cursor != nil {
		t.Error("ctrl+shift+h did not focus the sidebar")
	}

	m = keys(m, ctrlShiftL)
	if m.View().Cursor == nil {
		t.Error("ctrl+shift+l did not focus the editor")
	}
}

func TestCtrlShiftHWaitsForNormalModeWithNothingPending(t *testing.T) {
	for _, prefix := range []string{"i", "d"} {
		m := keys(typeKeys(shell(t, shellVault(), "/vault/a.md"), prefix), ctrlShiftH)
		if m.View().Cursor == nil {
			t.Errorf("after %q: ctrl+shift+h focused the sidebar", prefix)
		}
	}
}

func TestTabAndEscInTheSidebarReturnToTheEditor(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{tab, esc} {
		m := keys(shell(t, shellVault(), "/vault/a.md"), ctrlH, k)
		if m.View().Cursor == nil {
			t.Errorf("%s: focus stayed in the sidebar", k)
		}
	}
}

func TestGlobalKeysWaitForNormalModeWithNothingPending(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = keys(typeKeys(m, "i"), space, ctrlL, esc) // insert mode: all reach the editor
	if got := rows(m)[0]; !strings.Contains(got, "│ # Alpha") {
		t.Errorf("first row = %q, want the space typed into the Note", got)
	}
	if m.View().Cursor == nil {
		t.Fatal("focus left the editor")
	}

	m = keys(typeKeys(m, "d"), space) // d<space> deletes a character
	if got := rows(m)[0]; !strings.Contains(got, "│# Alpha") {
		t.Errorf("first row = %q, want d<space> to delete the space", got)
	}
}

func TestLeaderShowsTheNextKeys(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = keys(m, space)

	status := rows(m)[10]
	if !strings.Contains(status, "spc") || !strings.Contains(status, "e sidebar") {
		t.Errorf("status line = %q, want the leader keys", status)
	}
	m = keys(m, esc)
	if strings.Contains(rows(m)[10], "e sidebar") {
		t.Error("esc left the leader hint up")
	}
}

func TestLeaderFiresFromTheSidebar(t *testing.T) {
	m := keys(shell(t, shellVault(), "/vault/a.md"), ctrlH, space)

	m = typeKeys(m, "e")

	if strings.Contains(screen(m), "▸ daily") {
		t.Error("spc e from the sidebar did not hide it")
	}
}

func TestSpcEHidesAndShowsTheSidebar(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = typeKeys(keys(m, space), "e")
	if strings.Contains(screen(m), "▸ daily") || !strings.HasPrefix(rows(m)[0], "# Alpha") {
		t.Fatalf("sidebar still shown:\n%s", screen(m))
	}
	if m.View().Cursor == nil {
		t.Error("hiding the sidebar did not leave focus in the editor")
	}

	m = typeKeys(keys(m, space), "e")
	if !strings.Contains(screen(m), "▸ daily") {
		t.Fatalf("sidebar not shown again:\n%s", screen(m))
	}
	if m.View().Cursor != nil {
		t.Error("showing the sidebar did not focus it")
	}
}

func TestUnboundLeaderKeySaysSo(t *testing.T) {
	m := typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), space), "Q")

	if got := messageLine(m); got != "spc Q: not bound" {
		t.Errorf("message = %q", got)
	}
}

func TestSidebarShowsUppercaseExtensionNotesAsNotes(t *testing.T) {
	fsys := shellVault()
	_ = fsys.WriteFile("/vault/Shout.MD", []byte("# Shout\n"))
	m := shell(t, fsys, "/vault/a.md")

	if got := screen(m); !strings.Contains(got, "Shout") || strings.Contains(got, "Shout.MD") {
		t.Errorf("sidebar does not show Shout.MD as the Note Shout:\n%s", got)
	}
}
