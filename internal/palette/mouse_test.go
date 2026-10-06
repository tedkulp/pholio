package palette_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

// grouped is a scrolled List palette with group headers and an info line.
func grouped() palette.Model {
	var items []palette.Item
	for i := range 12 {
		items = append(items, palette.Item{Text: fmt.Sprintf("task %02d", i), Group: fmt.Sprintf("Group %d", i/4)})
	}
	return palette.New("Tasks", palette.List).
		WithInfo(func(string) []string { return []string{"info line"} }).
		SetItems(items).Scroll(7)
}

func TestClickChoosesTheItemDrawnOnThatRow(t *testing.T) {
	const w, h = 60, 14
	p := grouped()
	box, _, top, _ := p.View(theme.Default(), w, h)
	lines := strings.Split(ansi.Strip(box), "\n")

	chosen := 0
	for y := 0; y < h; y++ {
		_, ev := p.Click(w, h, y)
		var drawn string
		if r := y - top; r >= 0 && r < len(lines) {
			drawn = lines[r]
		}
		isItem := strings.Contains(drawn, "task ")
		switch {
		case ev.Kind == palette.Chosen:
			chosen++
			if !strings.Contains(drawn, ev.Item.Text) {
				t.Errorf("row %d: chose %q but it shows %q", y, ev.Item.Text, drawn)
			}
		case isItem:
			t.Errorf("row %d shows %q but a click there chose nothing", y, drawn)
		}
	}
	if chosen == 0 {
		t.Error("no row could be clicked")
	}
}

func TestScrollMovesTheSelectionWithinTheList(t *testing.T) {
	p := grouped().Scroll(-100)
	if it, _, _ := p.Selected(); it.Text != "task 00" {
		t.Errorf("selected %q after scrolling up past the start", it.Text)
	}
	p = p.Scroll(100)
	if it, _, _ := p.Selected(); it.Text != "task 11" {
		t.Errorf("selected %q after scrolling down past the end", it.Text)
	}
}
