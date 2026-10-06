package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func TestColonLineShowsOnTheMessageLineWithABarCursor(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "hello\n"})
	m := typeKeys(started(t, fsys), ":no")

	if got := messageLine(m); got != ":no" {
		t.Errorf("message line = %q, want the command line", got)
	}
	c := m.View().Cursor
	if c == nil || c.X != 3 || c.Y != 4 || c.Shape != tea.CursorBar {
		t.Errorf("cursor = %+v, want a bar at 3,4 on the bottom row", c)
	}
}

func TestQuitCommandQuitsTheApp(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "hello\n"})
	m := typeKeys(started(t, fsys), ":q")

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal(":q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf(":q command produced %T, want tea.QuitMsg", cmd())
	}
}

func TestConcealConfigOffShowsMarkup(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		userConfig:    "conceal = false\n",
		"/vault/a.md": "top\nsee [[Other]]\n",
	})
	m := started(t, fsys)

	if got := strings.TrimRight(rows(m)[1], " "); got != "see [[Other]]" {
		t.Errorf("second row = %q, want the link markup shown", got)
	}
}

func TestConcealIsOnByDefault(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "top\nsee [[Other]]\n"})
	m := started(t, fsys)

	if got := strings.TrimRight(rows(m)[1], " "); got != "see Other" {
		t.Errorf("second row = %q, want the link markup concealed", got)
	}
}
