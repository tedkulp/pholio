// PROTOTYPE — throwaway code for wayfinder ticket "Vim editor prototype".
// Run: go run . [file.md]      (no file: a scratch copy of sample.md in $TMPDIR)
//      go run . -big 200        (sample repeated 200x, for highlighting perf)

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"

	"pholio/prototypes/vim/engine"
)

//go:embed sample.md
var sample string

type osFS struct{}

func (osFS) ReadFile(p string) (string, error) { b, err := os.ReadFile(p); return string(b), err }
func (osFS) WriteFile(p, d string) error       { return os.WriteFile(p, []byte(d), 0o644) }

const scrolloff = 3

var styles [sCount]lipgloss.Style

func init() {
	s := lipgloss.NewStyle
	styles[sHeading] = s().Bold(true).Foreground(lipgloss.Color("12"))
	styles[sMarker] = s().Foreground(lipgloss.Color("8"))
	styles[sBold] = s().Bold(true)
	styles[sItalic] = s().Italic(true)
	styles[sCode] = s().Foreground(lipgloss.Color("3"))
	styles[sLink] = s().Foreground(lipgloss.Color("6")).Underline(true)
	styles[sTaskBox] = s().Foreground(lipgloss.Color("5")).Bold(true)
	styles[sTaskDone] = s().Foreground(lipgloss.Color("8")).Strikethrough(true)
	styles[sTag] = s().Foreground(lipgloss.Color("13"))
	styles[sMeta] = s().Foreground(lipgloss.Color("2"))
	styles[sQuote] = s().Italic(true).Foreground(lipgloss.Color("7"))
	styles[sBullet] = s().Foreground(lipgloss.Color("5"))
	styles[sSearch] = s().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
	styles[sVisual] = s().Reverse(true)
}

var (
	gutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	curNrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	statusStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	modeStyles  = map[engine.Mode]lipgloss.Style{
		engine.Normal:     lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("4")).Foreground(lipgloss.Color("0")),
		engine.Insert:     lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0")),
		engine.Visual:     lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("5")).Foreground(lipgloss.Color("0")),
		engine.VisualLine: lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("5")).Foreground(lipgloss.Color("0")),
		engine.Command:    lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")),
		engine.Search:     lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")),
	}
)

type model struct {
	e          *engine.Engine
	w, h       int
	top, left  int
	renderTime time.Duration
	hlTime     time.Duration
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.e.PageLines = m.textRows()
	case tea.PasteMsg:
		m.e.Paste(msg.Content)
	case tea.KeyPressMsg:
		k := msg.Key()
		name := msg.Keystroke()
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			name = k.Text
		}
		m.e.Feed(name)
		if m.e.Quit {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *model) textRows() int { return max(1, m.h-2) }

// ---------- layout: buffer line -> screen rows ----------

type glyph struct {
	start, end int // byte range in the line
	text       string
	w          int
}

type layout struct {
	rows [][]glyph
	st   []uint8
}

func (m *model) gutterW() int {
	if !m.e.Opts["number"] && !m.e.Opts["relnum"] {
		return 0
	}
	return max(3, len(strconv.Itoa(m.e.Buf.LineCount()))) + 1
}

func (m *model) layoutLine(i int, inFence bool) layout {
	l := m.e.Buf.Line(i)
	t0 := time.Now()
	var st []uint8
	var conceal []bool
	if m.e.Opts["hl"] {
		st, conceal = highlight(l, inFence)
	} else {
		st, conceal = make([]uint8, len(l)), make([]bool, len(l))
	}
	m.hlTime += time.Since(t0)
	for _, mt := range m.e.Matches(i) {
		for j := mt[0]; j < mt[1]; j++ {
			st[j] = sSearch
		}
	}
	if a, z, lw, ok := m.e.Selection(); ok && i >= a.Line && i <= z.Line {
		s, t := 0, len(l)
		if !lw {
			if i == a.Line {
				s = a.Col
			}
			if i == z.Line {
				t = z.Col
			}
		}
		for j := s; j < t; j++ {
			st[j] = sVisual
		}
	}
	cur := m.e.Cur
	doConceal := m.e.Opts["conceal"] && i != cur.Line

	var gs []glyph
	it := graphemes.FromString(l)
	for it.Next() {
		if doConceal && conceal[it.Start()] && st[it.Start()] != sVisual {
			continue
		}
		gs = append(gs, glyph{start: it.Start(), end: it.End(), text: it.Value()})
	}
	W := max(1, m.w-m.gutterW())
	lay := layout{st: st}
	if !m.e.Opts["wrap"] {
		x := 0
		for k := range gs {
			gs[k].w = engine.CellWidth(gs[k].text, x)
			x += gs[k].w
		}
		lay.rows = [][]glyph{gs}
		return lay
	}
	var row []glyph
	x, lastSpace := 0, -1
	for _, g := range gs {
		g.w = engine.CellWidth(g.text, x)
		if x+g.w > W && len(row) > 0 {
			if lastSpace >= 0 && lastSpace < len(row)-1 { // word wrap
				carry := append([]glyph(nil), row[lastSpace+1:]...)
				lay.rows = append(lay.rows, row[:lastSpace+1])
				row = carry
			} else {
				lay.rows = append(lay.rows, row)
				row = nil
			}
			x, lastSpace = 0, -1
			for k, c := range row {
				x += c.w
				if c.text == " " {
					lastSpace = k
				}
			}
			g.w = engine.CellWidth(g.text, x)
		}
		row = append(row, g)
		x += g.w
		if g.text == " " || g.text == "\t" {
			lastSpace = len(row) - 1
		}
	}
	lay.rows = append(lay.rows, row)
	// Insert-mode cursor sitting just past a full last row gets its own row.
	if i == cur.Line && cur.Col == len(l) && x >= W {
		lay.rows = append(lay.rows, nil)
	}
	return lay
}

// cursorRow returns the row index and cell x of byte col within a layout.
func (lay layout) cursorRow(col int) (row, x int) {
	for r, gs := range lay.rows {
		cx := 0
		for _, g := range gs {
			if g.start == col {
				return r, cx
			}
			cx += g.w
		}
		if r == len(lay.rows)-1 {
			return r, cx // past end of line (insert mode)
		}
	}
	return 0, 0
}

// ---------- view ----------

func (m *model) View() tea.View {
	t0 := time.Now()
	m.hlTime = 0
	if m.w == 0 {
		return tea.NewView("")
	}
	e := m.e
	H, W := m.textRows(), max(1, m.w-m.gutterW())
	n := e.Buf.LineCount()
	cur := e.Cur

	fence := fenceStates(e.Buf.Line, min(n, max(cur.Line, m.top)+H+1))
	inFence := func(i int) bool { return i < len(fence) && fence[i] }
	lays := map[int]layout{}
	lay := func(i int) layout {
		if l, ok := lays[i]; ok {
			return l
		}
		l := m.layoutLine(i, inFence(i))
		lays[i] = l
		return l
	}

	// Vertical scroll: keep `scrolloff` rows of context around the cursor.
	if cur.Line < m.top+scrolloff {
		m.top = max(0, cur.Line-scrolloff)
	}
	if m.top > cur.Line {
		m.top = cur.Line
	}
	curRow, curX := lay(cur.Line).cursorRow(cur.Col)
	// Walk up from the cursor until the screen is full: O(viewport), never
	// O(distance jumped). (A first version stepped m.top forward one line at a
	// time and took 1.4s for G on a 19.5k-line file.)
	below := len(lay(cur.Line).rows) - curRow - 1
	for i := cur.Line + 1; i < n && below < scrolloff; i++ {
		below += len(lay(i).rows)
	}
	used, first := curRow+1+min(below, scrolloff), cur.Line
	for first > m.top && used+len(lay(first-1).rows) <= H {
		first--
		used += len(lay(first).rows)
	}
	m.top = first
	// Horizontal scroll when nowrap.
	if !e.Opts["wrap"] {
		if curX < m.left {
			m.left = curX
		}
		if curX >= m.left+W {
			m.left = curX - W + 1
		}
	} else {
		m.left = 0
	}

	var out []string
	cursorY, cursorX := -1, 0
	gw := m.gutterW()
	for i := m.top; i < n && len(out) < H; i++ {
		ly := lay(i)
		for r, gs := range ly.rows {
			if len(out) >= H {
				break
			}
			var sb strings.Builder
			if gw > 0 {
				num := ""
				if r == 0 {
					v := i + 1
					if e.Opts["relnum"] && i != cur.Line {
						v = abs(i - cur.Line)
					}
					num = strconv.Itoa(v)
				}
				st := gutterStyle
				if i == cur.Line {
					st = curNrStyle
				}
				sb.WriteString(st.Render(fmt.Sprintf("%*s ", gw-1, num)))
			}
			sb.WriteString(renderRow(gs, ly.st, m.left, W))
			if i == cur.Line && r == curRow {
				cursorY, cursorX = len(out), gw+curX-m.left
			}
			out = append(out, sb.String())
		}
	}
	for len(out) < H {
		out = append(out, gutterStyle.Render("~"))
	}

	// Status line.
	ms := modeStyles[e.Mode]
	left := ms.Render(" "+e.Mode.String()+" ") + " " + filepath.Base(e.Path)
	if e.Dirty {
		left += " [+]"
	}
	if pk := e.PendingKeys(); pk != "" {
		left += "  " + pk
	}
	m.renderTime = time.Since(t0)
	right := fmt.Sprintf("hl:%s wrap:%s  %d:%d  render %s (hl %s) ",
		onoff(e.Opts["hl"]), onoff(e.Opts["wrap"]), cur.Line+1, engine.Cells(e.Buf.Line(cur.Line), cur.Col)+1,
		m.renderTime.Round(10*time.Microsecond), m.hlTime.Round(10*time.Microsecond))
	gap := max(1, m.w-lipgloss.Width(left)-lipgloss.Width(right))
	out = append(out, statusStyle.Render(left+strings.Repeat(" ", gap)+right))

	// Command / message line.
	var c *tea.Cursor
	if e.Mode == engine.Command || e.Mode == engine.Search {
		p, t := e.CmdLine()
		out = append(out, p+t)
		c = tea.NewCursor(ansi.StringWidth(p+t), len(out)-1)
		c.Shape = tea.CursorBar
	} else {
		out = append(out, ansi.Truncate(e.Msg, m.w, "…"))
		if cursorY >= 0 {
			c = tea.NewCursor(cursorX, cursorY)
			switch {
			case e.Mode == engine.Insert:
				c.Shape = tea.CursorBar
			case e.OperatorPending():
				c.Shape = tea.CursorUnderline
			}
		}
	}
	if c != nil {
		c.Blink = false
	}
	v := tea.NewView(strings.Join(out, "\n"))
	v.AltScreen = true
	v.Cursor = c
	return v
}

// renderRow styles one screen row, grouping runs of the same style id.
func renderRow(gs []glyph, st []uint8, left, W int) string {
	var sb strings.Builder
	x := 0
	var run strings.Builder
	runStyle := uint8(255)
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if runStyle == sNone {
			sb.WriteString(run.String())
		} else {
			sb.WriteString(styles[runStyle].Render(run.String()))
		}
		run.Reset()
	}
	for _, g := range gs {
		gx := x
		x += g.w
		if gx < left {
			continue
		}
		if x-left > W {
			break
		}
		s := st[g.start]
		if s != runStyle {
			flush()
			runStyle = s
		}
		if g.text == "\t" {
			run.WriteString(strings.Repeat(" ", g.w))
		} else {
			run.WriteString(g.text)
		}
	}
	flush()
	return sb.String()
}

func onoff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func main() {
	big := flag.Int("big", 0, "repeat sample.md N times into the scratch file (perf test)")
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		path = filepath.Join(os.TempDir(), "pholio-vim-proto.md")
		if _, err := os.Stat(path); err != nil || *big > 0 {
			os.WriteFile(path, []byte(strings.Repeat(sample, max(*big, 1))), 0o644)
		}
	}
	e := engine.New(osFS{}, path)
	if _, err := tea.NewProgram(&model{e: e}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
