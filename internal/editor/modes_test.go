package editor_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/theme"
)

func TestVisualSelectionIsDrawnThroughTheVisualSlot(t *testing.T) {
	m := feed(open("one two three\n", 30, 3), "wvl")

	view, _ := m.View(th)

	if want := th.Style(theme.MarkdownVisual).Inherit(th.Style(theme.UIBase)).Render("tw"); !strings.Contains(view, want) {
		t.Errorf("selection not drawn as visual:\n%q", view)
	}
	if got := ansi.Strip(m.StatusLine(th, 30)); !strings.HasPrefix(got, " VISUAL ") {
		t.Errorf("status line = %q", got)
	}
	if !strings.Contains(m.StatusLine(th, 30), th.Style(theme.UIModeVisual).Render(" VISUAL ")) {
		t.Error("mode not drawn with ui.mode_visual")
	}
}

func TestVisualLineSelectsWholeLines(t *testing.T) {
	m := feed(open("ab\ncd\nef\n", 10, 4), "jVj")

	view, _ := m.View(th)

	visual := th.Style(theme.MarkdownVisual).Inherit(th.Style(theme.UIBase))
	for _, l := range []string{"cd", "ef"} {
		if !strings.Contains(view, visual.Render(l)) {
			t.Errorf("%q not selected:\n%q", l, view)
		}
	}
	if strings.Contains(view, visual.Render("ab")) {
		t.Error("line above the selection is drawn selected")
	}
}

func TestEmptyLinesInASelectionShowOneSelectedCell(t *testing.T) {
	visual := th.Style(theme.MarkdownVisual).Inherit(th.Style(theme.UIBase))
	for _, keys := range []string{"Vjj", "vjj"} {
		m := feed(open("ab\n\ncd\n\n", 10, 5), keys)

		view, _ := m.View(th)

		rows := strings.Split(view, "\n")
		if !strings.HasPrefix(rows[1], visual.Render(" ")) {
			t.Errorf("%s: empty line inside the selection not drawn selected: %q", keys, rows[1])
		}
		if strings.Contains(rows[3], visual.Render(" ")) {
			t.Errorf("%s: empty line after the selection drawn selected: %q", keys, rows[3])
		}
	}
}

func TestSearchMatchesAreDrawnThroughTheSearchSlot(t *testing.T) {
	m := feed(open("cat dog\nhotdog\n", 20, 3), "/dog<enter>")

	view, _ := m.View(th)

	search := th.Style(theme.MarkdownSearch).Inherit(th.Style(theme.UIBase))
	if n := strings.Count(view, search.Render("dog")); n != 2 {
		t.Errorf("%d matches drawn as search, want 2:\n%q", n, view)
	}
}

func TestCommandLineTakesTheCursor(t *testing.T) {
	m := feed(open("text\n", 20, 3), ":wq")

	_, viewCursor := m.View(th)
	line, c, ok := m.CmdLine(th, 20)

	if !ok || ansi.Strip(line) != ":wq" {
		t.Fatalf("cmdline = %q, %v", ansi.Strip(line), ok)
	}
	if viewCursor != nil {
		t.Errorf("text area still has the cursor: %+v", viewCursor)
	}
	if c == nil || c.X != 3 || c.Y != 0 || c.Shape != tea.CursorBar {
		t.Errorf("cmdline cursor = %+v, want a bar at 3,0", c)
	}
	if !strings.Contains(m.StatusLine(th, 20), th.Style(theme.UIModeCommand).Render(" COMMAND ")) {
		t.Errorf("status line = %q", ansi.Strip(m.StatusLine(th, 20)))
	}
}

func TestSearchLineShowsItsPrompt(t *testing.T) {
	m := feed(open("text\n", 20, 3), "?ab")

	line, _, ok := m.CmdLine(th, 20)

	if !ok || ansi.Strip(line) != "?ab" {
		t.Errorf("cmdline = %q, %v", ansi.Strip(line), ok)
	}
}

func TestLongCommandLineKeepsItsEndVisible(t *testing.T) {
	m := feed(open("text\n", 10, 3), ":abcdefghijkl")

	line, c, _ := m.CmdLine(th, 10)

	if got := ansi.Strip(line); got != "defghijkl" {
		t.Errorf("cmdline = %q, want its tail", got)
	}
	if c.X != 9 {
		t.Errorf("cursor x = %d, want 9, the last cell", c.X)
	}
}

func TestNoCommandLineInNormalMode(t *testing.T) {
	m := open("text\n", 10, 3)

	if _, _, ok := m.CmdLine(th, 10); ok {
		t.Error("cmdline open in normal mode")
	}
}
