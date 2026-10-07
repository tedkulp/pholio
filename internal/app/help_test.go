package app_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
)

func TestSpcQuestionOpensHelp(t *testing.T) {
	m := typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), space), "?")

	golden.RequireEqual(t, screen(m))
	got := m.PaletteRows()
	for _, want := range []string{
		" | spc d         today | leader",
		" | spc ?         help | leader",
		" | spc           leader key | global",
		" | ctrl+h        focus sidebar | global",
		" | ctrl+shift+h  focus sidebar | global",
		" | ctrl+shift+l  focus editor | global",
		" | [d            previous Daily Note | sequence",
		" | gd            go to Link | normal",
		" | r             rename | sidebar",
		" | F7            cycle theme | app",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("help lacks row %q; rows:\n%s", want, strings.Join(got, "\n"))
		}
	}
}

func TestLeaderHintListsHelp(t *testing.T) {
	m := keys(resize(shell(t, shellVault(), "/vault/a.md"), 120, 12), space)

	golden.RequireEqual(t, rows(m)[10])
}

func TestTypingFiltersHelpAndEscCloses(t *testing.T) {
	m := typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), space), "?daily")

	got := m.PaletteRows()
	if len(got) != 2 || !strings.Contains(got[0], "[d") || !strings.Contains(got[1], "]d") {
		t.Errorf("rows for \"daily\" = %q, want [d and ]d", got)
	}

	m = keys(m, esc)
	if m.PaletteRows() != nil || m.Path() != "/vault/a.md" {
		t.Error("esc did not just close the help popup")
	}
}

func TestEnterOnAHelpRowRunsIt(t *testing.T) {
	m := keys(typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), space), "?spc e"), enter)

	if m.PaletteRows() != nil {
		t.Fatal("help still open after enter")
	}
	if strings.Contains(screen(m), "▸ daily") {
		t.Error("enter on spc e did not hide the sidebar")
	}
}

func TestEnterOnASidebarRowNeedsTheSidebar(t *testing.T) {
	m := keys(typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), space), "?rename"), enter)

	if got := messageLine(m); got != "focus the sidebar first" {
		t.Errorf("message = %q", got)
	}
	if m.PaletteRows() != nil {
		t.Error("help still open")
	}
}

func TestEnterOnASidebarRowFromTheSidebarRunsIt(t *testing.T) {
	m := keys(typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), ctrlH, space), "?narrower"), enter)

	if got := messageLine(m); got == "focus the sidebar first" {
		t.Error("refused a sidebar row opened from the sidebar")
	}
}

func TestEnterOnSpcOrHelpJustCloses(t *testing.T) {
	for _, q := range []string{"?leader key", "?help"} {
		m := keys(typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), space), q), enter)
		if m.PaletteRows() != nil || strings.Contains(rows(m)[10], "e sidebar") {
			t.Errorf("%s: enter left help or the Leader open:\n%s", q, screen(m))
		}
	}
}
