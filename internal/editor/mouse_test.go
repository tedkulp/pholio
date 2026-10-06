package editor_test

import (
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/engine"
)

func TestClickMapsWrappedRowsToTheirText(t *testing.T) {
	m := open("aaaa bbbb cccc\nend", 6, 5) // rows "aaaa ", "bbbb ", "cccc"

	for _, c := range []struct {
		x, y int
		want engine.Pos
	}{
		{1, 1, engine.Pos{Line: 0, Col: 6}},  // the second b
		{5, 0, engine.Pos{Line: 0, Col: 4}},  // past a wrapped row: its last glyph
		{3, 2, engine.Pos{Line: 0, Col: 13}}, // past the line's end: its last character
		{1, 3, engine.Pos{Line: 1, Col: 1}},
		{0, 4, engine.Pos{Line: 1, Col: 0}}, // a ~ row: the last line
	} {
		if got := m.Click(c.x, c.y).Engine().Cur; got != c.want {
			t.Errorf("click %d,%d: cursor = %+v, want %+v", c.x, c.y, got, c.want)
		}
		m = m.Click(0, 0)
	}
}

func TestScrollPastEitherEndKeepsTheCursorOnScreen(t *testing.T) {
	text := strings.Repeat("a long wrapped line of words\n", 30)
	m := open(text, 10, 8)

	for _, n := range []int{100, -100, 7, -3, 500} {
		m = m.Scroll(n)
		if _, c := m.View(th); c == nil {
			t.Fatalf("after Scroll(%d) the cursor is off screen at %+v", n, m.Engine().Cur)
		}
	}
	if got := m.Engine().Cur.Line; got != 29 {
		t.Errorf("after scrolling to the end, cursor line = %d, want 29", got)
	}
}

func TestClickUsesTheConcealedLayout(t *testing.T) {
	m := open("top\nsee [[Link]]", 40, 5) // line 1 shows as "see Link"

	if got := m.Click(4, 1).Engine().Cur; got != (engine.Pos{Line: 1, Col: 6}) {
		t.Errorf("cursor = %+v, want on the L of Link", got)
	}
}
