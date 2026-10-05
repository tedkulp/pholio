package editor

import (
	"github.com/clipperhouse/uax29/v2/graphemes"

	"github.com/tedkulp/pholio/internal/engine"
)

// glyph is one grapheme of a buffer line as laid out on screen.
type glyph struct {
	start int // byte offset in the line
	text  string
	w     int // cells
}

// layout is one buffer line broken into screen rows.
type layout [][]glyph

// layoutLine breaks line into rows of at most width cells. With wrap off
// the whole line is one row. With wrap on, a row breaks after its last
// blank, or mid-word when the word alone fills the row. When the cursor
// sits just past a full last row (insert mode at end of line), an empty
// row is added for it.
func layoutLine(line string, width int, wrap, cursorAtEnd bool) layout {
	var gs []glyph
	it := graphemes.FromString(line)
	for it.Next() {
		gs = append(gs, glyph{start: it.Start(), text: it.Value()})
	}
	if !wrap {
		x := 0
		for k := range gs {
			gs[k].w = engine.CellWidth(gs[k].text, x)
			x += gs[k].w
		}
		return layout{gs}
	}
	var lay layout
	var row []glyph
	x, lastBlank := 0, -1
	for _, g := range gs {
		g.w = engine.CellWidth(g.text, x)
		if x+g.w > width && len(row) > 0 {
			if lastBlank >= 0 && lastBlank < len(row)-1 {
				carry := append([]glyph(nil), row[lastBlank+1:]...)
				lay = append(lay, row[:lastBlank+1])
				row = carry
			} else {
				lay = append(lay, row)
				row = nil
			}
			// Re-measure the carried glyphs from the row start (tabs).
			x, lastBlank = 0, -1
			for k := range row {
				row[k].w = engine.CellWidth(row[k].text, x)
				x += row[k].w
				if isBlank(row[k].text) {
					lastBlank = k
				}
			}
			g.w = engine.CellWidth(g.text, x)
		}
		row = append(row, g)
		x += g.w
		if isBlank(g.text) {
			lastBlank = len(row) - 1
		}
	}
	lay = append(lay, row)
	if cursorAtEnd && x >= width {
		lay = append(lay, nil)
	}
	return lay
}

func isBlank(g string) bool { return g == " " || g == "\t" }

// find returns the row and cell x of byte column col. A column past the
// last glyph lands just after the last row's end.
func (lay layout) find(col int) (row, x int) {
	for r, gs := range lay {
		cx := 0
		for _, g := range gs {
			if g.start == col {
				return r, cx
			}
			cx += g.w
		}
		if r == len(lay)-1 {
			return r, cx
		}
	}
	return 0, 0
}
