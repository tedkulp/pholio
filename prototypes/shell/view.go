package main

// PROTOTYPE: rendering. Overlays are spliced over the base frame line by line.

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"pholio/prototypes/vim/engine"
)

const scrolloff = 3

// pill is deliberately not themed: it's prototype chrome, not part of the design.
var pill = lipgloss.NewStyle().Background(lipgloss.Color("#ff5fd7")).Foreground(lipgloss.Color("#000000"))

func fit(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

func (m *model) View() tea.View {
	if m.w == 0 {
		return tea.NewView("")
	}
	H := m.bodyH()
	var body []string
	var cx, cy = -1, -1

	sideVisible := m.sidebarOn && m.variant != vDrawer
	edX, edW := 0, m.w
	if sideVisible {
		edX, edW = m.sideW, max(10, m.w-m.sideW)
	}
	ed, ecx, ecy := m.editorLines(edW, H)
	if sideVisible {
		side := m.sidebarLines(m.sideW, H, true)
		for i := range H {
			body = append(body, side[i]+ed[i])
		}
	} else {
		body = ed
	}
	if m.focus == focusEditor && ecy >= 0 {
		cx, cy = edX+ecx, ecy
	}
	if m.variant == vDrawer && m.sidebarOn {
		w := min(m.sideW+4, m.w-10)
		body = place(body, m.sidebarLines(w, H, true), 0, 0)
		cx, cy = -1, -1
	}

	status, cmd := m.statusLine(), m.cmdLine()
	var cmdCursor = -1
	if m.overlay != ovNone {
		cx, cy = -1, -1
		if m.variant != vDrawer {
			dim := m.th.s("overlay.backdrop")
			for i, l := range body {
				body[i] = dim.Render(ansi.Strip(l))
			}
		}
		var box []string
		var bx, by int
		var inX, inY = -1, -1
		switch {
		case m.overlay == ovZettel && m.variant == vDrawer:
			p := "Zettel title: "
			cmd = m.th.s("overlay.title").Render(p) + m.th.s("overlay.input").Render(m.input) +
				m.th.s("overlay.hint").Render("   → zettel/"+zettelName(m.inputOr())+".md   enter create · esc cancel")
			cmdCursor = ansi.StringWidth(p + m.input)
		case m.overlay == ovZettel:
			box, inX, inY = m.zettelBox()
		default:
			box, inX, inY = m.tasksBox(H)
		}
		if box != nil {
			bw := ansi.StringWidth(box[0])
			switch m.variant {
			case vSplit:
				bx, by = (m.w-bw)/2, max(0, (H-len(box))/2)
			case vPalette:
				bx, by = (m.w-bw)/2, 1
			case vDrawer:
				bx, by = m.w-bw, 0
			}
			body = place(body, box, bx, by)
			if inX >= 0 {
				cx, cy = bx+inX, by+inY
			}
		}
	}

	out := append(body, status, fit(cmd, m.w), m.pillLine())
	if cmdCursor < 0 && (m.e.Mode == engine.Command || m.e.Mode == engine.Search) && m.focus == focusEditor && m.overlay == ovNone {
		p, t := m.e.CmdLine()
		cmdCursor = ansi.StringWidth(p + t)
	}
	var c *tea.Cursor
	switch {
	case cmdCursor >= 0:
		c = tea.NewCursor(cmdCursor, H+1)
		c.Shape = tea.CursorBar
	case cx >= 0:
		c = tea.NewCursor(cx, cy)
		if m.overlay != ovNone || m.e.Mode == engine.Insert {
			c.Shape = tea.CursorBar
		} else if m.e.OperatorPending() {
			c.Shape = tea.CursorUnderline
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

func (m *model) inputOr() string {
	if strings.TrimSpace(m.input) == "" {
		return "title"
	}
	return m.input
}

// place splices box lines over base at column x, row y.
func place(base, box []string, x, y int) []string {
	out := append([]string(nil), base...)
	for i, b := range box {
		r := y + i
		if r < 0 || r >= len(out) {
			continue
		}
		l := out[r]
		left := ansi.Truncate(l, x, "")
		left += strings.Repeat(" ", max(0, x-ansi.StringWidth(left)))
		right := ansi.TruncateLeft(l, x+ansi.StringWidth(b), "")
		out[r] = left + "\x1b[m" + b + "\x1b[m" + right
	}
	return out
}

// frame draws a rounded border with a title in the top edge. sides picks which edges to draw.
func (m *model) frame(title string, lines []string, innerW int, left, right bool) []string {
	bs, ts := m.th.s("overlay.border"), m.th.s("overlay.title")
	l, r := "", ""
	if left {
		l = bs.Render("│ ")
	}
	if right {
		r = bs.Render(" │")
	}
	corner := func(a, b string) (string, string) {
		if !left {
			a = "─"
		}
		if !right {
			b = "─"
		}
		return a, b
	}
	tl, tr := corner("╭", "╮")
	bl, br := corner("╰", "╯")
	pad := 0
	if left {
		pad++
	}
	if right {
		pad++
	}
	full := innerW + 2*pad
	t := " " + title + " "
	top := bs.Render(tl+"─") + ts.Render(t) + bs.Render(strings.Repeat("─", max(0, full-2-ansi.StringWidth(t)))+tr)
	out := []string{top}
	for _, s := range lines {
		out = append(out, l+fit(s, innerW)+r)
	}
	out = append(out, bs.Render(bl+strings.Repeat("─", max(0, full-2))+br))
	return out
}

func (m *model) zettelBox() ([]string, int, int) {
	w := min(64, m.w-6)
	in := m.th.s("overlay.input").Render(m.input)
	if m.input == "" {
		in = m.th.s("overlay.placeholder").Render("title of the new idea")
	}
	lines := []string{
		"Title: " + in,
		"",
		m.th.s("overlay.hint").Render("→ zettel/" + zettelName(m.inputOr()) + ".md"),
		m.th.s("overlay.hint").Render("link inserted at cursor · Origin: [[" + strings.TrimSuffix(filepath.Base(m.e.Path), ".md") + "]]"),
		m.th.s("overlay.hint").Render("enter create · esc cancel"),
	}
	if m.variant == vPalette {
		lines = append([]string{"> " + in, m.th.s("overlay.border").Render(strings.Repeat("─", w))}, lines[2:]...)
		return m.frame("New Zettel", lines, w, true, true), 2 + 2 + ansi.StringWidth(m.input), 1
	}
	return m.frame("New Zettel", lines, w, true, true), 2 + 7 + ansi.StringWidth(m.input), 1
}

func (m *model) tasksBox(H int) ([]string, int, int) {
	var w, h int
	switch m.variant {
	case vSplit:
		w, h = min(90, m.w-8), min(H-4, 24)
	case vDrawer:
		w, h = max(40, m.w*45/100)-3, H-2
	case vPalette:
		w, h = min(80, m.w-10), min(H-4, 18)
	}
	ts := m.filteredTasks()
	var rows []string
	selRow := 0
	group := -1
	for i, t := range ts {
		if t.group != group && m.variant != vPalette {
			group = t.group
			gs := m.th.s("tasks." + []string{"overdue", "today", "upcoming", "no_date"}[group])
			if len(rows) > 0 {
				rows = append(rows, "")
			}
			n := 0
			for _, u := range ts {
				if u.group == group {
					n++
				}
			}
			rows = append(rows, gs.Render(fmt.Sprintf("%s (%d)", groupNames[group], n)))
		}
		if i == m.taskSel {
			selRow = len(rows)
		}
		rows = append(rows, m.taskRow(t, w, i == m.taskSel))
	}
	if len(ts) == 0 {
		rows = append(rows, m.th.s("overlay.hint").Render("no matching tasks"))
	}
	var head []string
	inX, inY := -1, -1
	if m.variant == vPalette {
		in := m.th.s("overlay.input").Render(m.filter)
		if m.filter == "" {
			in = m.th.s("overlay.placeholder").Render("type to filter tasks")
		}
		head = []string{"> " + in, m.th.s("overlay.border").Render(strings.Repeat("─", w))}
		inX, inY = 2+2+ansi.StringWidth(m.filter), 1
	}
	foot := []string{"", m.th.s("overlay.hint").Render(map[int]string{
		vSplit:   "j/k move · enter open · esc close",
		vDrawer:  "j/k move · enter open · esc close",
		vPalette: "↑/↓ ctrl+n/p move · enter open · esc close",
	}[m.variant])}
	avail := max(1, h-len(head)-len(foot))
	start := max(0, min(selRow-avail/2, len(rows)-avail))
	rows = rows[start:min(len(rows), start+avail)]
	for len(rows) < avail && m.variant != vPalette {
		rows = append(rows, "")
	}
	lines := append(append(head, rows...), foot...)
	title := fmt.Sprintf("Task List · %d open", len(m.tasks))
	return m.frame(title, lines, w, true, m.variant != vDrawer), inX, inY
}

func (m *model) taskRow(t task, w int, sel bool) string {
	src := fmt.Sprintf("%s:%d", t.file, t.line+1)
	text := t.text
	if t.pri == "high" {
		text = "! " + text
	}
	gap := max(1, w-2-ansi.StringWidth(text)-ansi.StringWidth(src))
	if gap == 1 {
		text = ansi.Truncate(text, max(4, w-3-ansi.StringWidth(src)), "…")
		gap = max(1, w-2-ansi.StringWidth(text)-ansi.StringWidth(src))
	}
	if sel {
		return m.th.s("overlay.selected").Render(fit("☐ "+text+strings.Repeat(" ", gap)+src, w))
	}
	return "☐ " + mdInline(text, m.th) + strings.Repeat(" ", gap) + m.th.s("tasks.source").Render(src)
}

func (m *model) sidebarLines(w, H int, _ bool) []string {
	focused := m.focus == focusSidebar
	border := m.th.s("ui.pane_border")
	if focused {
		border = m.th.s("ui.pane_border_focus")
	}
	inner := w - 1
	nodes := m.visibleNodes()
	m.sel = max(0, min(m.sel, len(nodes)-1))
	rows := H - 1
	if m.sel < m.sideTop {
		m.sideTop = m.sel
	}
	if m.sel >= m.sideTop+rows {
		m.sideTop = m.sel - rows + 1
	}
	head := " " + filepath.Base(m.vault)
	if m.showHidden {
		head += " (all)"
	}
	out := []string{m.th.s("sidebar.header").Render(fit(head, inner)) + border.Render("│")}
	for i := m.sideTop; i < m.sideTop+rows; i++ {
		var line string
		if i < len(nodes) {
			n := nodes[i]
			icon := "  "
			if n.dir {
				icon = "▸ "
				if m.expanded[n.path] {
					icon = "▾ "
				}
			}
			label := n.name
			if !n.dir {
				label = strings.TrimSuffix(label, ".md")
			}
			txt := fit(" "+strings.Repeat("  ", n.depth)+icon+label, inner)
			st := m.th.s("sidebar.file")
			switch {
			case i == m.sel && focused:
				st = m.th.s("sidebar.selected")
			case i == m.sel:
				st = m.th.s("sidebar.selected_blur")
			case n.path == m.e.Path:
				st = m.th.s("sidebar.open_file")
			case n.hidden:
				st = m.th.s("sidebar.hidden")
			case n.dir:
				st = m.th.s("sidebar.dir")
			case !strings.HasSuffix(n.name, ".md"):
				st = m.th.s("sidebar.other_file")
			}
			line = st.Render(txt)
		} else {
			line = strings.Repeat(" ", inner)
		}
		out = append(out, line+border.Render("│"))
	}
	return out
}

func (m *model) editorLines(w, H int) ([]string, int, int) {
	e := m.e
	n := e.Buf.LineCount()
	cur := e.Cur.Line
	so := min(scrolloff, (H-1)/2)
	if cur < m.top+so {
		m.top = max(0, cur-so)
	}
	if cur > m.top+H-1-so {
		m.top = cur - H + 1 + so
	}
	var out []string
	cx, cy := -1, -1
	for r := range H {
		i := m.top + r
		if i >= n {
			out = append(out, fit(m.th.s("ui.pane_border").Render(" ~"), w))
			continue
		}
		l := e.Buf.Line(i)
		out = append(out, fit(" "+mdLine(l, m.th), w))
		if i == cur {
			cx, cy = min(w-1, 1+ansi.StringWidth(l[:min(e.Cur.Col, len(l))])), r
		}
	}
	return out, cx, cy
}

func (m *model) statusLine() string {
	st := m.th.s("ui.statusline")
	var mode string
	var ms lipgloss.Style
	if m.focus == focusSidebar {
		mode, ms = " TREE ", m.th.s("ui.mode_command")
	} else {
		mode = " " + strings.ToUpper(m.e.Mode.String()) + " "
		ms = map[engine.Mode]lipgloss.Style{
			engine.Normal: m.th.s("ui.mode_normal"), engine.Insert: m.th.s("ui.mode_insert"),
			engine.Visual: m.th.s("ui.mode_visual"), engine.VisualLine: m.th.s("ui.mode_visual"),
		}[m.e.Mode]
		if m.e.Mode == engine.Command || m.e.Mode == engine.Search {
			ms = m.th.s("ui.mode_command")
		}
	}
	rel, _ := filepath.Rel(m.vault, m.e.Path)
	left := ms.Render(mode) + st.Render(" ") + m.th.s("ui.status_file").Render(rel)
	if m.e.Dirty {
		left += m.th.s("ui.status_dirty").Render(" [+]")
	}
	pend := m.e.PendingKeys()
	if m.prefix != "" {
		pend = m.prefix + "-"
	}
	right := fmt.Sprintf("%s  %d:%d ", pend, m.e.Cur.Line+1, m.e.Cur.Col+1)
	gap := max(1, m.w-ansi.StringWidth(left)-ansi.StringWidth(right))
	return ansi.Truncate(left+st.Render(strings.Repeat(" ", gap)+right), m.w, "")
}

func (m *model) cmdLine() string {
	if m.focus == focusEditor && (m.e.Mode == engine.Command || m.e.Mode == engine.Search) {
		p, t := m.e.CmdLine()
		return p + t
	}
	msg := m.msg
	if msg == "" {
		msg = m.e.Msg
	}
	if strings.Contains(msg, "unknown") {
		return m.th.s("ui.error").Render(msg)
	}
	return m.th.s("ui.message").Render(msg)
}

func (m *model) pillLine() string {
	focus := "EDITOR"
	if m.focus == focusSidebar {
		focus = "SIDEBAR"
	}
	s := fmt.Sprintf(" F5◀ %s ▶F6 │ %s │ F7 theme:%s · focus:%s · w%d ",
		variantNames[m.variant], variantHints[m.variant], m.th.name, focus, m.sideW)
	return pill.Render(fit(s, m.w))
}

// ---------- markdown, deliberately cheap ----------

var (
	headRe  = regexp.MustCompile(`^#{1,6}\s`)
	tboxRe  = regexp.MustCompile(`^(\s*)([-*] \[( |x|X)\])(.*)$`)
	bullRe  = regexp.MustCompile(`^(\s*)([-*+]|\d+\.)(\s.*)$`)
	linkRe  = regexp.MustCompile(`\[\[[^\]]+\]\]`)
	tagRe   = regexp.MustCompile(`(^|\s)(#[\w/-]+)`)
	metaRe  = regexp.MustCompile(`(^|\s)([a-z]+:[^\s\]]+)`)
	codeRe  = regexp.MustCompile("`[^`]+`")
	boldRe  = regexp.MustCompile(`\*\*[^*]+\*\*`)
)

func mdLine(l string, th *theme) string {
	switch {
	case headRe.MatchString(l):
		return th.s("markdown.heading").Render(l)
	case strings.HasPrefix(l, ">"):
		return th.s("markdown.quote").Render(l)
	}
	if mt := tboxRe.FindStringSubmatch(l); mt != nil {
		if mt[3] != " " {
			return mt[1] + th.s("markdown.task_box").Render(mt[2]) + th.s("markdown.task_done").Render(mt[4])
		}
		return mt[1] + th.s("markdown.task_box").Render(mt[2]) + mdInline(mt[4], th)
	}
	if mt := bullRe.FindStringSubmatch(l); mt != nil {
		return mt[1] + th.s("markdown.bullet").Render(mt[2]) + mdInline(mt[3], th)
	}
	return mdInline(l, th)
}

func mdInline(s string, th *theme) string {
	s = codeRe.ReplaceAllStringFunc(s, func(x string) string { return th.s("markdown.code").Render(x) })
	s = boldRe.ReplaceAllStringFunc(s, func(x string) string { return th.s("markdown.bold").Render(x) })
	s = linkRe.ReplaceAllStringFunc(s, func(x string) string { return th.s("markdown.link").Render(x) })
	s = tagRe.ReplaceAllString(s, "$1"+th.s("markdown.tag").Render("$2"))
	s = metaRe.ReplaceAllString(s, "$1"+th.s("markdown.meta").Render("$2"))
	return s
}
