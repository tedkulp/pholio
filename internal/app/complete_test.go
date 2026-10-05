package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

var (
	popDown = tea.KeyPressMsg{Code: tea.KeyDown}
	popBksp = tea.KeyPressMsg{Code: tea.KeyBackspace}
)

// popup is the [[ popup's rows, trimmed, or nil when it is closed.
func popup(m app.Model) []string { return m.Completion() }

// newLine starts insert mode on a fresh last line of the Note.
func newLine(m app.Model) app.Model { return typeKeys(m, "Go") }

func TestBracketsOpenThePopupWhichSaysIndexingUntilTheScan(t *testing.T) {
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	m := typeKeys(newLine(openIn(t, &v, "index.md")), "see [")
	if got := popup(m); got != nil {
		t.Fatalf("one [ opened the popup: %q", got)
	}
	m = typeKeys(m, "[bet")
	if got := popup(m); len(got) != 1 || got[0] != "indexing…" {
		t.Fatalf("popup before the scan = %q, want indexing…", got)
	}
	if !strings.Contains(strings.Join(rows(m), "\n"), "indexing…") {
		t.Errorf("the popup isn't drawn:\n%s", strings.Join(rows(m), "\n"))
	}
	m = drive(t, m, m.Init())
	if got := popup(m); len(got) == 0 || !strings.HasPrefix(got[0], "beta") {
		t.Fatalf("popup after the scan = %q, want beta first", got)
	}
}

func TestAcceptingThePopupInsertsTheFilename(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = typeKeys(newLine(m), "see [[gam")
	m = press(m, enter)
	lines := strings.Split(m.Text(), "\n")
	if got := lines[len(lines)-2]; got != "see [[gamma]]" {
		t.Fatalf("line = %q, want see [[gamma]]", got)
	}
	if popup(m) != nil {
		t.Error("the popup is still open after accepting")
	}
	m = typeKeys(m, " next")
	lines = strings.Split(m.Text(), "\n")
	if got := lines[len(lines)-2]; got != "see [[gamma]] next" {
		t.Errorf("line = %q: still in insert mode after ]]?", got)
	}
}

func TestPopupMatchesTitlesAndMovesWithArrows(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = typeKeys(newLine(m), "[[dup in")
	got := popup(m)
	if len(got) != 2 {
		t.Fatalf("popup = %q, want the two dup Notes", got)
	}
	m = press(press(m, popDown), tab)
	lines := strings.Split(m.Text(), "\n")
	// Both are named dup; the second needs its folder to be unambiguous.
	if got := lines[len(lines)-2]; got != "[[b/c/dup]]" {
		t.Fatalf("line = %q, want [[b/c/dup]]", got)
	}
}

func TestEscClosesThePopupAndStaysInInsertMode(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = typeKeys(newLine(m), "[[al")
	m = press(m, esc)
	if popup(m) != nil {
		t.Fatal("esc left the popup open")
	}
	m = typeKeys(m, "x")
	lines := strings.Split(m.Text(), "\n")
	if got := lines[len(lines)-2]; got != "[[alx" {
		t.Errorf("line = %q, want [[alx (still inserting)", got)
	}
}

func TestPopupClosesWhenTheCursorLeavesTheLink(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = typeKeys(newLine(m), "[[al")
	m = press(press(press(m, popBksp), popBksp), popBksp)
	if popup(m) != nil {
		t.Errorf("popup still open after deleting the [[: %q", popup(m))
	}
}
