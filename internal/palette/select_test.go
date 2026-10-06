package palette_test

import (
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

func TestSelectMovesToTheRowShowingAnItem(t *testing.T) {
	p := palette.New("t", palette.List).SetItems(items("a", "b", "c"))

	if got := selected(p.Select(2)); got != "c" {
		t.Errorf("Select(2) = %s, want c", got)
	}
	if got := selected(p.Select(9)); got != "c" {
		t.Errorf("Select past the end = %s, want the last row", got)
	}
	// With "b" filtered out, Select(1) lands on the next shown item.
	p = p.WithQuery("[ac]").WithMatcher(func(_ string, _ []palette.Item) []int { return []int{0, 2} })
	if got := selected(p.Select(1)); got != "c" {
		t.Errorf("Select of a hidden item = %s, want c", got)
	}
	if v := p.Visible(); len(v) != 2 || v[0].Text != "a" || v[1].Text != "c" {
		t.Errorf("Visible = %+v", v)
	}
}

func TestDetailSlotStylesTheDetail(t *testing.T) {
	th := theme.Default()
	p := palette.New("t", palette.List).SetItems([]palette.Item{
		{Text: "sel", Detail: "x.md:1"},
		{Text: "a", Detail: "a.md:2", DetailSlot: theme.TasksSource},
	})
	box, _, _, _ := p.View(th, 80, 20)
	if want := th.Style(theme.TasksSource).Render("a.md:2"); !strings.Contains(box, want) {
		t.Errorf("detail not drawn with tasks.source:\n%q", box)
	}
}
