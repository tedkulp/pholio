package engine

import (
	"strings"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

// block is a blockwise selection: lines top..bot and the display cells
// [left, right) on each of them. right is -1 after $: the block runs to
// the end of every line.
type block struct{ top, bot, left, right int }

// block is the current blockwise selection, from the anchor to the cursor.
// The cursor's side is the column j and k remember, so a short line the
// cursor is clamped on doesn't narrow or widen the block. The block runs
// to the end of every line after $ until a motion other than j or k,
// which is exactly as long as the remembered column is $'s.
func (e *Engine) block() block {
	a, z := order(e.anchor, e.Cur)
	al, ar := glyphCells(e.line(e.anchor.Line), e.anchor.Col)
	zl, zr := glyphCells(e.line(e.Cur.Line), e.Cur.Col)
	if e.want >= 0 && (e.want < zl || e.want >= zr) {
		zl, zr = e.want, e.want+1
	}
	b := block{a.Line, z.Line, min(al, zl), max(ar, zr)}
	if e.want < 0 {
		b.right = -1
	}
	return b
}

// glyphCells is the cells [from, to) the glyph at byte col covers. The end
// of a line covers one cell.
func glyphCells(l string, col int) (from, to int) {
	from = Cells(l, col)
	if col >= len(l) {
		return from, from + 1
	}
	return from, from + CellWidth(l[col:nextG(l, col)], from)
}

// cols is the byte range of l inside the block. A glyph that straddles
// either edge is wholly inside. from == to when l is too short to reach
// the block.
func (b block) cols(l string) (from, to int) {
	from, to = len(l), len(l)
	w := 0
	it := graphemes.FromString(l)
	for it.Next() {
		if b.right >= 0 && w >= b.right {
			to = it.Start()
			break
		}
		w += CellWidth(it.Value(), w)
		if w > b.left && from == len(l) {
			from = it.Start()
		}
	}
	return from, max(from, to)
}

// SelectedRange is the byte range [from, to) of line i inside the visual
// selection. ok is false when line i has none of it, or outside visual
// mode. An empty line inside a charwise or linewise selection gives 0, 0.
func (e *Engine) SelectedRange(i int) (from, to int, ok bool) {
	if !e.Mode.visual() {
		return 0, 0, false
	}
	l := e.line(i)
	if e.Mode == VisualBlock {
		b := e.block()
		from, to = b.cols(l)
		return from, to, b.top <= i && i <= b.bot && from < to
	}
	a, z, lw := e.selection()
	if i < a.Line || i > z.Line {
		return 0, 0, false
	}
	from, to = 0, len(l)
	if !lw && i == a.Line {
		from = a.Col
	}
	if !lw && i == z.Line {
		to = z.Col
	}
	return from, to, true
}

// blockRows is the text of the block on each of its lines.
func (e *Engine) blockRows(b block) []string {
	var rows []string
	for i := b.top; i <= b.bot; i++ {
		l := e.line(i)
		from, to := b.cols(l)
		rows = append(rows, l[from:to])
	}
	return rows
}

// blockOp applies d, c or y to the block, with register reg.
func (e *Engine) blockOp(op string, reg rune) {
	b := e.block()
	rows := e.blockRows(b)
	e.store(register{text: strings.Join(rows, "\n"), rows: rows}, reg, op == "y")
	from, _ := b.cols(e.line(b.top))
	e.Mode = Normal
	e.cmdStart = Pos{b.top, from}
	if op == "y" {
		e.Cur = e.cmdStart
		return
	}
	var reached []blockRow
	for i := b.top; i <= b.bot; i++ {
		f, t := b.cols(e.line(i))
		if i > b.top && f < t {
			reached = append(reached, blockRow{line: i, col: f})
		}
		e.del(Pos{i, f}, Pos{i, t})
	}
	e.Cur = e.cmdStart
	if op == "c" {
		e.startBlockInsert(reached)
	}
}

// blockToggleCase toggles the case of the text inside the block.
func (e *Engine) blockToggleCase() {
	b := e.block()
	from, _ := b.cols(e.line(b.top))
	e.Mode = Normal
	e.cmdStart = Pos{b.top, from}
	for i := b.top; i <= b.bot; i++ {
		f, t := b.cols(e.line(i))
		e.toggleCase(Pos{i, f}, Pos{i, t})
	}
	e.Cur = e.cmdStart
}

// blockRow is where a block insert's text goes on one more line: at byte
// col, after pad spaces.
type blockRow struct{ line, col, pad int }

// blockInsert is an insert session started from a block. On esc, the text
// typed at `at` is copied to every row.
type blockInsert struct {
	at    Pos
	orig  string // the line at.Line as the session began
	lines int    // the buffer's line count as the session began
	rows  []blockRow
}

// blockI starts inserting before the block's left edge (I), or after its
// right edge (A). A skips no line: it pads lines that are too short.
func (e *Engine) blockI(after bool) {
	b := e.block()
	e.Mode = Normal
	var rows []blockRow
	var at blockRow
	for i := b.top; i <= b.bot; i++ {
		l := e.line(i)
		w := Cells(l, len(l))
		from, to := b.cols(l)
		row := blockRow{line: i, col: from}
		switch {
		case !after && w <= b.left:
			if i > b.top {
				continue // I skips lines that don't reach the block
			}
		case !after:
		case b.right < 0:
			row.col = len(l)
		case w < b.right:
			row.col, row.pad = len(l), b.right-w
		default:
			row.col = to
		}
		if i == b.top {
			at = row
		} else {
			rows = append(rows, row)
		}
	}
	e.cmdStart = e.Cur
	e.ins(Pos{at.line, at.col}, strings.Repeat(" ", at.pad))
	e.Cur = Pos{at.line, at.col + at.pad}
	e.startBlockInsert(rows)
}

// startBlockInsert enters insert mode at the cursor, to copy what is typed
// to rows on esc.
func (e *Engine) startBlockInsert(rows []blockRow) {
	e.startInsert()
	e.blockIns = &blockInsert{at: e.Cur, orig: e.line(e.Cur.Line), lines: e.Buf.LineCount(), rows: rows}
}

// endBlockInsert ends a block insert left with key k, copying the text
// typed to the block's other lines, and reports whether it did. It copies
// nothing, keeping only the first line's edit, when k is ctrl+c, or the
// text took a line break or was not one plain insert at the start point.
func (e *Engine) endBlockInsert(k string) bool {
	bi := e.blockIns
	e.blockIns = nil
	if bi == nil || k == "ctrl+c" || e.Buf.LineCount() != bi.lines || e.Cur.Line != bi.at.Line {
		return false
	}
	l, c := e.line(bi.at.Line), bi.at.Col
	n := len(l) - len(bi.orig)
	if n <= 0 || l[:c] != bi.orig[:c] || l[c+n:] != bi.orig[c:] {
		return false
	}
	text := l[c : c+n]
	for _, r := range bi.rows {
		e.ins(Pos{r.line, r.col}, strings.Repeat(" ", r.pad)+text)
	}
	e.Cur = bi.at
	return true
}

// pasteBlock puts a blockwise register's rows on the lines from the
// cursor down, at the cell column after (p) or at (P) the cursor. Short
// lines are padded with spaces, and lines are added past the buffer's end.
func (e *Engine) pasteBlock(rows []string, count int, after bool) {
	l := e.line(e.Cur.Line)
	col := Cells(l, e.Cur.Col)
	if after && len(l) > 0 {
		col = Cells(l, nextG(l, e.Cur.Col))
	}
	width := 0
	for _, r := range rows {
		width = max(width, Cells(r, len(r)))
	}
	for i, r := range rows {
		ln := e.Cur.Line + i
		if ln == e.Buf.LineCount() {
			e.ins(Pos{ln - 1, len(e.line(ln - 1))}, "\n")
		}
		l := e.line(ln)
		at, lead := colAtCells(l, col), ""
		if w := Cells(l, len(l)); w < col {
			lead = strings.Repeat(" ", col-w)
		}
		text := lead
		for k := range count {
			text += r
			if k < count-1 || at < len(l) {
				text += strings.Repeat(" ", width-Cells(r, len(r)))
			}
		}
		e.ins(Pos{ln, at}, text)
	}
	e.Cur.Col = colAtCells(e.line(e.Cur.Line), col)
}
