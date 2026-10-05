package editor

import "github.com/tedkulp/pholio/internal/engine"

// Click moves the cursor to the text under pane cell x, y, as drawn,
// without changing the mode. A cell past the end of a line lands on its
// last character (after it in insert mode), and a row past the end of the
// buffer on the last line.
func (m Model) Click(x, y int) Model {
	m.place(m.PosAt(x, y))
	m.scroll()
	return m
}

// Scroll moves the view n rows down (up when n < 0), as the mouse wheel
// does. The cursor is pulled along, keeping its cell column, when it would
// leave the rows that scrolloff allows.
func (m Model) Scroll(n int) Model {
	if n < 0 {
		m.top, _ = m.up(m.top, -n)
	} else {
		m.top, _ = m.down(m.top, n)
	}
	so := min(Scrolloff, (m.h-1)/2)
	lo := so
	if m.top == (spot{}) {
		lo = 0
	}
	cur, x := m.cursorSpot()
	d := m.distance(m.top, cur, m.h)
	switch {
	case d < lo:
		to, _ := m.down(m.top, lo)
		m.place(m.posIn(to, x))
	case d+1+m.rowsBelow(cur, so) > m.h:
		to, _ := m.down(m.top, m.h-1-so)
		m.place(m.posIn(to, x))
	}
	m.scroll()
	return m
}

// place moves the cursor to p. In insert mode it may sit after the line.
func (m *Model) place(p engine.Pos) {
	m.e.SetCursor(p)
	if m.e.Mode == engine.Insert && p.Col == len(m.e.Buf.Line(p.Line)) {
		m.e.Cur.Col = p.Col // SetCursor clamps as normal mode does
	}
}

// PosAt is the buffer position drawn at pane cell x, y. Past the end of a
// row it is the row's last character, or the end of the line on a line's
// last row; past the end of the buffer it is on the last line.
func (m Model) PosAt(x, y int) engine.Pos {
	s, _ := m.down(m.top, max(0, y)) // the last row when the buffer ends
	return m.posIn(s, x+m.left)
}

// posIn is the buffer position at cell x of screen row s, past the row's
// end as PosAt says.
func (m *Model) posIn(s spot, x int) engine.Pos {
	lay := m.lay(s.line)
	gs := lay[s.row]
	cx := 0
	for _, g := range gs {
		if x < cx+g.w {
			return engine.Pos{Line: s.line, Col: g.start}
		}
		cx += g.w
	}
	if s.row < len(lay)-1 && len(gs) > 0 {
		return engine.Pos{Line: s.line, Col: gs[len(gs)-1].start}
	}
	return engine.Pos{Line: s.line, Col: len(m.e.Buf.Line(s.line))}
}

// down moves s down by n rows. ok is false when the buffer ended first;
// s is then its last row.
func (m *Model) down(s spot, n int) (spot, bool) {
	for ; n > 0; n-- {
		switch {
		case s.row < len(m.lay(s.line))-1:
			s.row++
		case s.line < m.e.Buf.LineCount()-1:
			s.line, s.row = s.line+1, 0
		default:
			return s, false
		}
	}
	return s, true
}
