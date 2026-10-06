package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func typeKeys(m app.Model, keys string) app.Model {
	for _, r := range keys {
		m = press(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

func rows(m app.Model) []string {
	return strings.Split(ansi.Strip(m.View().Content), "\n")
}

func TestKeysEditTheNote(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "hello world\n"})
	m := started(t, fsys)

	m = typeKeys(m, "dw")

	r := rows(m)
	if got := strings.TrimRight(r[0], " "); got != "world" {
		t.Errorf("first row = %q, want the word deleted", got)
	}
	if got := strings.TrimRight(r[len(r)-2], " "); !strings.HasPrefix(got, " NORMAL  a.md [+]") {
		t.Errorf("status line = %q", got)
	}
}

func TestEditorLayoutWithStatusAndMessageLines(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		"/vault/a.md": "# Title\nA line long enough to wrap in a forty column terminal window.\n",
	})
	m := resize(started(t, fsys), 40, 6)

	m = typeKeys(m, "jA")

	golden.RequireEqual(t, ansi.Strip(m.View().Content))
	c := m.View().Cursor
	if c == nil || c.X != 23 || c.Y != 2 || c.Shape != tea.CursorBar {
		t.Errorf("cursor = %+v, want a bar at 23,2", c)
	}
}

func TestWrapConfigOffScrollsSideways(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		userConfig:    "wrap = false\n",
		"/vault/a.md": "short\nA line long enough to scroll in a forty column terminal window.\n",
	})
	m := resize(started(t, fsys), 40, 6)

	m = typeKeys(m, "j$")

	r := rows(m)
	if got := strings.TrimRight(r[1], " "); got != "croll in a forty column terminal window." {
		t.Errorf("second row = %q, want it scrolled to the line's end", got)
	}
	if got := strings.TrimRight(r[0], " "); got != "" {
		t.Errorf("first row = %q, want it scrolled out of view", got)
	}
}

func TestF8AppliesAChangedWrapSetting(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		"/vault/a.md": "A line long enough to wrap in a forty column terminal window.\n",
	})
	m := resize(started(t, fsys), 40, 6)
	if err := fsys.WriteFile(userConfig, []byte("wrap = false\n")); err != nil {
		t.Fatal(err)
	}

	m = press(m, f8)

	if got := strings.TrimRight(rows(m)[1], " "); got != "~" {
		t.Errorf("second row = %q, want the line no longer wrapped", got)
	}
}

func TestAKeyClearsTheMessage(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "hi\n"})
	m := press(started(t, fsys), f7)

	m = typeKeys(m, "l")

	if got := messageLine(m); got != "" {
		t.Errorf("message line = %q, want it cleared", got)
	}
}

func TestPasteReachesTheEditor(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "\n"})
	m := typeKeys(started(t, fsys), "i")

	next, _ := m.Update(tea.PasteMsg{Content: "pasted"})

	if got := strings.TrimRight(rows(next.(app.Model))[0], " "); got != "pasted" {
		t.Errorf("first row = %q", got)
	}
}
