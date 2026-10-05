package palette_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+n":
		return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

// feed sends keys (names or single characters) and returns the last event.
func feed(p palette.Model, ks ...string) (palette.Model, palette.Event) {
	var ev palette.Event
	for _, k := range ks {
		p, ev = p.Update(key(k))
	}
	return p, ev
}

func items(texts ...string) []palette.Item {
	out := make([]palette.Item, len(texts))
	for i, t := range texts {
		out[i] = palette.Item{Text: t}
	}
	return out
}

func selected(p palette.Model) string {
	it, _, ok := p.Selected()
	if !ok {
		return "<none>"
	}
	return it.Text
}

func view(p palette.Model, w, h int) string {
	box, _, _, _ := p.View(theme.Default(), w, h)
	return ansi.Strip(box)
}

func TestTypingFiltersCaseInsensitively(t *testing.T) {
	p := palette.New("t", palette.Type).SetItems(items("Apple", "banana", "Cherry"))

	p, ev := feed(p, "A", "n")

	if ev.Kind != palette.Changed || ev.Query != "An" {
		t.Errorf("event = %+v, want Changed with the query", ev)
	}
	if got := selected(p); got != "banana" {
		t.Errorf("selected = %q, want the only match", got)
	}
	p, _ = feed(p, "backspace", "backspace")
	if p.Query() != "" || selected(p) != "Apple" {
		t.Errorf("after backspace: query %q selected %q", p.Query(), selected(p))
	}
}

func TestArrowsAndCtrlNPMove(t *testing.T) {
	p := palette.New("t", palette.Type).SetItems(items("a", "b", "c"))

	p, _ = feed(p, "down", "ctrl+n", "ctrl+n")
	if got := selected(p); got != "c" {
		t.Errorf("after moving down past the end: %q", got)
	}
	p, _ = feed(p, "ctrl+p", "up", "up")
	if got := selected(p); got != "a" {
		t.Errorf("after moving up past the top: %q", got)
	}
}

func TestEnterChoosesAndEscCloses(t *testing.T) {
	p := palette.New("t", palette.Type).SetItems(items("a", "b"))

	_, ev := feed(p, "down", "enter")
	if ev.Kind != palette.Chosen || !ev.OK || ev.Item.Text != "b" || ev.Index != 1 {
		t.Errorf("enter: %+v", ev)
	}
	_, ev = feed(p, "esc")
	if ev.Kind != palette.Closed {
		t.Errorf("esc: %+v", ev)
	}
}

func TestEnterWithNoMatchStillReportsTheQuery(t *testing.T) {
	p := palette.New("New Zettel", palette.Type)

	_, ev := feed(p, "h", "i", "space", "x", "enter")

	if ev.Kind != palette.Chosen || ev.OK || ev.Query != "hi x" {
		t.Errorf("event = %+v, want Chosen with no item and the title", ev)
	}
}

func TestListModeHandsLettersToTheHost(t *testing.T) {
	p := palette.New("Task List", palette.List).SetItems(items("one", "two", "three"))

	p, ev := feed(p, "j")
	if ev.Kind != palette.None || selected(p) != "two" {
		t.Errorf("j: %+v, selected %q", ev, selected(p))
	}
	p, ev = feed(p, "space")
	if ev.Kind != palette.Key || ev.Key != " " || ev.Index != -1 {
		t.Errorf("space: %+v, want a Key event", ev)
	}
	p, _ = feed(p, "k")
	if selected(p) != "one" {
		t.Errorf("k: selected %q", selected(p))
	}
	if p.Typing() {
		t.Error("list mode started typing")
	}
}

func TestSlashFiltersAndEscReturnsToTheList(t *testing.T) {
	p := palette.New("Task List", palette.List).SetItems(items("one", "two", "three"))

	p, _ = feed(p, "/", "t", "h")
	if !p.Typing() || selected(p) != "three" {
		t.Fatalf("filter: typing %v, selected %q", p.Typing(), selected(p))
	}
	p, ev := feed(p, "esc")
	if ev.Kind != palette.None || p.Typing() || p.Query() != "th" {
		t.Fatalf("esc in the filter: %+v typing %v query %q", ev, p.Typing(), p.Query())
	}
	_, ev = feed(p, "esc")
	if ev.Kind != palette.Closed {
		t.Errorf("esc in the list: %+v, want Closed", ev)
	}
}

func TestCtrlUClearsTheQuery(t *testing.T) {
	p, _ := feed(palette.New("t", palette.Type), "a", "b", "ctrl+u")

	if p.Query() != "" {
		t.Errorf("query = %q", p.Query())
	}
}

func TestPasteGoesIntoTheInput(t *testing.T) {
	p, ev := palette.New("t", palette.Type).Paste("two\nlines")

	if ev.Kind != palette.Changed || p.Query() != "two lines" {
		t.Errorf("event %+v query %q", ev, p.Query())
	}
	_, ev = palette.New("t", palette.List).Paste("x")
	if ev.Kind != palette.None {
		t.Errorf("paste into a list without the filter: %+v", ev)
	}
}

func TestCustomMatcher(t *testing.T) {
	prefix := func(q string, its []palette.Item) []int {
		var out []int
		for i := len(its) - 1; i >= 0; i-- { // reversed, to show order is the matcher's
			if strings.HasPrefix(its[i].Text, q) {
				out = append(out, i)
			}
		}
		return out
	}
	p := palette.New("t", palette.Type).WithMatcher(prefix).SetItems(items("ab", "b", "abc"))

	p, _ = feed(p, "a")

	if got := selected(p); got != "abc" {
		t.Errorf("selected %q, want the matcher's first", got)
	}
}

func TestViewDrawsGroupsDetailsAndHint(t *testing.T) {
	p := palette.New("Task List", palette.List).WithHint("j/k move").SetItems([]palette.Item{
		{Text: "pay rent", Detail: "a.md:3", Group: "Overdue (1)"},
		{Text: "call mom", Detail: "b.md:1", Group: "Today (1)"},
	})

	want := strings.Join([]string{
		"╭─ Task List ──────────────╮",
		"│ >                        │",
		"│ ──────────────────────── │",
		"│ Overdue (1)              │",
		"│ pay rent          a.md:3 │",
		"│                          │",
		"│ Today (1)                │",
		"│ call mom          b.md:1 │",
		"│ j/k move                 │",
		"╰──────────────────────────╯",
	}, "\n")
	if got := view(p, 38, 24); got != want {
		t.Errorf("view =\n%s\nwant\n%s", got, want)
	}
}

func TestViewShowsInfoAndPlaceholderForAPrompt(t *testing.T) {
	p := palette.New("New Zettel", palette.Type).WithPlaceholder("title").WithHint("").
		WithInfo(func(q string) []string { return []string{"→ zettel/" + q + ".md"} })

	want := strings.Join([]string{
		"╭─ New Zettel ─────────────╮",
		"│ > title                  │",
		"│ → zettel/.md             │",
		"╰──────────────────────────╯",
	}, "\n")
	if got := view(p, 38, 24); got != want {
		t.Errorf("view =\n%s\nwant\n%s", got, want)
	}
}

func TestViewScrollsToKeepTheSelectionVisible(t *testing.T) {
	var texts []string
	for i := range 40 {
		texts = append(texts, fmt.Sprintf("item %02d", i))
	}
	p := palette.New("t", palette.Type).SetItems(items(texts...))
	for range 30 {
		p, _ = feed(p, "down")
	}

	got := view(p, 40, 30)

	if !strings.Contains(got, "item 30") || strings.Contains(got, "item 00") {
		t.Errorf("view does not follow the selection:\n%s", got)
	}
	if n := strings.Count(got, "\n") + 1; n != 18 {
		t.Errorf("box is %d rows, want the 18-row maximum", n)
	}
}

func TestGeometryIsTopAnchoredAndCentred(t *testing.T) {
	x, y, w, h := palette.Geometry(120, 40)
	if x != 20 || y != 1 || w != 80 || h != 18 {
		t.Errorf("Geometry(120, 40) = %d,%d %dx%d", x, y, w, h)
	}
	x, _, w, h = palette.Geometry(50, 10)
	if x != 5 || w != 40 || h != 8 {
		t.Errorf("Geometry(50, 10) = x %d, %dx%d", x, w, h)
	}
}
