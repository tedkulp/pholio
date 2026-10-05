package editor

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/tedkulp/pholio/internal/theme"
)

// overlay is what a glyph is marked as on top of its markdown kind.
type overlay uint8

const (
	plain    overlay = iota
	match            // a search match
	selected         // in the visual selection
	overlays
)

// palette is the style of every kind under every overlay.
type palette [overlays][kinds]lipgloss.Style

func paletteFor(th theme.Theme) *palette {
	var p palette
	base := th.Style(theme.UIBase)
	for k, slot := range kindSlots {
		st := th.Style(slot)
		if slot != theme.UIBase {
			st = st.Inherit(base)
		}
		p[plain][k] = st
		p[match][k] = th.Style(theme.MarkdownSearch).Inherit(st)
		p[selected][k] = th.Style(theme.MarkdownVisual).Inherit(st)
	}
	return &p
}

// looks gives the overlay at each byte column of line i. The visual
// selection wins over search matches.
func (m *Model) looks(i int) func(col int) overlay {
	matches := m.e.Matches(i)
	a, z, linewise, ok := m.e.Selection()
	sel := ok && a.Line <= i && i <= z.Line
	if !sel && matches == nil {
		return func(int) overlay { return plain }
	}
	from, to := 0, len(m.e.Buf.Line(i))
	if sel && !linewise {
		if i == a.Line {
			from = a.Col
		}
		if i == z.Line {
			to = z.Col
		}
	}
	return func(col int) overlay {
		if sel && from <= col && col < to {
			return selected
		}
		for _, r := range matches {
			if r[0] <= col && col < r[1] {
				return match
			}
		}
		return plain
	}
}

// drawRow draws the glyphs that fall in cells [left, left+w), styled by
// kind and overlay, with one Render per run of equal style. A wide glyph
// cut by either edge shows as blanks.
func drawRow(gs []glyph, left, w int, p *palette, look func(int) overlay) string {
	var out, run strings.Builder
	var o overlay
	var k kind
	flush := func() {
		if run.Len() > 0 {
			out.WriteString(p[o][k].Render(run.String()))
			run.Reset()
		}
	}
	x := 0
	for _, g := range gs {
		gx := x
		x += g.w
		if x <= left {
			continue
		}
		if gx >= left+w {
			break
		}
		if ov := look(g.start); ov != o || g.kind != k {
			flush()
			o, k = ov, g.kind
		}
		if gx < left || x > left+w || g.text == "\t" {
			run.WriteString(strings.Repeat(" ", min(x, left+w)-max(gx, left)))
			continue
		}
		run.WriteString(g.text)
	}
	flush()
	return out.String()
}
