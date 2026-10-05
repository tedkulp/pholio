// PROTOTYPE — throwaway code for wayfinder ticket "Vim editor prototype".

// Package engine is a vim editing engine with no Bubble Tea dependency.
// Input is a stream of key names (Feed); output is plain state the view reads.
package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Mode int

const (
	Normal Mode = iota
	Insert
	Visual
	VisualLine
	Command // ":" line
	Search  // "/" or "?" line
)

func (m Mode) String() string {
	return [...]string{"NORMAL", "INSERT", "VISUAL", "V-LINE", "COMMAND", "SEARCH"}[m]
}

// FS is the engine's only side-effect; tests pass an in-memory one.
type FS interface {
	ReadFile(path string) (string, error)
	WriteFile(path, data string) error
}

type register struct {
	text     string
	linewise bool
}

// edit is one primitive mutation: at `at`, `del` was removed then `ins` inserted.
type edit struct {
	at       Pos
	del, ins string
}

// change is one undo step: every edit made by one normal command or one
// insert session.
type change struct {
	edits         []edit
	before, after Pos
}

type Engine struct {
	Buf   *Buffer
	Cur   Pos
	Mode  Mode
	Path  string
	Dirty bool
	Quit  bool
	Msg   string
	Opts  map[string]bool // number, relnum, wrap, hl, conceal, hlsearch, listcont
	// PageLines is set by the host so ctrl+d / ctrl+u know the viewport height.
	PageLines int

	fs     FS
	want   int // remembered display column for j/k; -1 = end of line
	anchor Pos // visual mode start

	keys     []string // pending normal-mode keys (the command being parsed)
	cmdline  string
	cmdStart Pos // cursor when the current command began

	regs map[rune]register

	undo, redo []change
	pending    *change
	depth      int // Feed nesting (dot repeat replays through Feed)

	dot       []string // last change, without its leading count
	dotCount  int
	recording bool // still appending insert-mode keys to dot
	replaying bool

	keptWant  bool    // last command was j/k/$: don't recompute want
	opPending *parsed // operator waiting on a "/" search motion
	opFrom    Pos

	lastFind    string // e.g. "f;" — the motion and its char
	search      *regexp.Regexp
	searchBack  bool
	searchInput string
}

func New(fs FS, path string) *Engine {
	e := &Engine{
		fs:        fs,
		Path:      path,
		regs:      map[rune]register{},
		Opts:      map[string]bool{"number": true, "wrap": true, "hl": true, "hlsearch": true, "listcont": true},
		PageLines: 20,
	}
	e.load(path)
	return e
}

func (e *Engine) load(path string) {
	text, err := e.fs.ReadFile(path)
	if err != nil {
		text = ""
		e.Msg = fmt.Sprintf("%q [New]", path)
	} else {
		e.Msg = fmt.Sprintf("%q %dL", path, strings.Count(text, "\n"))
	}
	e.Buf = NewBuffer(text)
	e.Path, e.Cur, e.Mode, e.Dirty = path, Pos{}, Normal, false
	e.undo, e.redo, e.pending = nil, nil, nil
}

// ---------- read-only view API ----------

func (e *Engine) PendingKeys() string { return strings.Join(e.keys, "") }

// OperatorPending is true while waiting for a motion after d/c/y.
func (e *Engine) OperatorPending() bool {
	p, st := parse(e.keys, e.Mode != Normal)
	return st == incomplete && p.op != ""
}

// CmdLine returns the prompt char and text for ":" / "/" / "?".
func (e *Engine) CmdLine() (string, string) {
	if e.Mode == Search {
		if e.searchBack {
			return "?", e.cmdline
		}
		return "/", e.cmdline
	}
	return ":", e.cmdline
}

// Selection returns the visual selection as [a, z) (charwise) or the line
// span (linewise). ok is false outside visual mode.
func (e *Engine) Selection() (a, z Pos, linewise, ok bool) {
	if e.Mode != Visual && e.Mode != VisualLine {
		return
	}
	a, z = order(e.anchor, e.Cur)
	z.Col = nextG(e.Buf.Line(z.Line), z.Col)
	return a, z, e.Mode == VisualLine, true
}

// Matches returns [start,end) byte ranges of the search pattern on line i.
func (e *Engine) Matches(i int) [][]int {
	if e.search == nil || !e.Opts["hlsearch"] {
		return nil
	}
	return e.search.FindAllStringIndex(e.Buf.Line(i), -1)
}

// ---------- input ----------

var special = map[string]bool{
	"esc": true, "enter": true, "backspace": true, "tab": true, "delete": true,
	"up": true, "down": true, "left": true, "right": true, "home": true, "end": true,
	"pgup": true, "pgdown": true,
}

func isText(k string) bool {
	return !special[k] && !(utf8.RuneCountInString(k) > 1 && strings.Contains(k, "+"))
}

// Feed processes one key. Keys are printable text ("a", "Ж", "👍🏽") or
// names ("esc", "enter", "ctrl+r").
func (e *Engine) Feed(k string) {
	// A fast "<esc>x" arrives from the terminal as "alt+x". Outside normal
	// mode nothing is bound to alt, so split it back into the two keys.
	if rest, ok := strings.CutPrefix(k, "alt+"); ok && e.Mode != Normal && len(e.keys) == 0 {
		e.Feed("esc")
		e.Feed(rest)
		return
	}
	e.depth++
	defer func() {
		e.depth--
		if e.depth == 0 {
			e.settle()
		}
	}()
	if e.recording && !e.replaying {
		e.dot = append(e.dot, k)
	}
	switch e.Mode {
	case Insert:
		e.insertKey(k)
	case Command, Search:
		e.cmdKey(k)
	default:
		if len(e.keys) == 0 {
			e.cmdStart = e.Cur
			if e.depth == 1 {
				e.Msg = ""
			}
		}
		e.keys = append(e.keys, k)
		e.normalKey()
	}
}

// settle runs after each top-level key: commits the undo step once we are
// back at rest, and keeps the cursor legal for the mode.
func (e *Engine) settle() {
	if e.Mode == Normal || e.Mode == Visual || e.Mode == VisualLine {
		e.clampNormal()
		if len(e.keys) == 0 && !e.keptWant {
			e.want = Cells(e.Buf.Line(e.Cur.Line), e.Cur.Col)
		}
		e.keptWant = false
		if len(e.keys) == 0 && e.pending != nil {
			e.pending.after = e.Cur
			e.undo = append(e.undo, *e.pending)
			e.redo = nil
			e.pending = nil
		}
	}
}

func (e *Engine) clampNormal() {
	n := e.Buf.LineCount()
	e.Cur.Line = max(0, min(e.Cur.Line, n-1))
	l := e.Buf.Line(e.Cur.Line)
	if e.Cur.Col > lastG(l) {
		e.Cur.Col = lastG(l)
	}
}

// ---------- recorded mutations ----------

func (e *Engine) record(ed edit) {
	if e.pending == nil {
		e.pending = &change{before: e.cmdStart}
	}
	e.pending.edits = append(e.pending.edits, ed)
	e.Dirty = true
}

func (e *Engine) ins(p Pos, s string) Pos {
	if s == "" {
		return p
	}
	e.record(edit{at: p, ins: s})
	return e.Buf.insert(p, s)
}

func (e *Engine) del(a, z Pos) string {
	if !a.Less(z) {
		return ""
	}
	t := e.Buf.delete(a, z)
	e.record(edit{at: a, del: t})
	return t
}

func (e *Engine) undoStep() {
	if len(e.undo) == 0 {
		e.Msg = "Already at oldest change"
		return
	}
	c := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	for i := len(c.edits) - 1; i >= 0; i-- {
		ed := c.edits[i]
		if ed.ins != "" {
			e.Buf.delete(ed.at, endOf(ed.at, ed.ins))
		}
		if ed.del != "" {
			e.Buf.insert(ed.at, ed.del)
		}
	}
	e.redo = append(e.redo, c)
	e.Cur, e.Dirty = c.before, true
	e.Msg = fmt.Sprintf("undo: %d edits reverted", len(c.edits))
}

func (e *Engine) redoStep() {
	if len(e.redo) == 0 {
		e.Msg = "Already at newest change"
		return
	}
	c := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	for _, ed := range c.edits {
		if ed.del != "" {
			e.Buf.delete(ed.at, endOf(ed.at, ed.del))
		}
		if ed.ins != "" {
			e.Buf.insert(ed.at, ed.ins)
		}
	}
	e.undo = append(e.undo, c)
	e.Cur, e.Dirty = c.after, true
}

// ---------- insert mode ----------

var listRe = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX]\]\s+)?`)

func (e *Engine) startInsert() {
	e.Mode = Insert
	e.Cur.Col = min(e.Cur.Col, len(e.Buf.Line(e.Cur.Line)))
}

func (e *Engine) insertKey(k string) {
	line := e.Buf.Line(e.Cur.Line)
	switch k {
	case "esc", "ctrl+c", "ctrl+[":
		e.Mode = Normal
		e.recording = false
		e.Cur.Col = prevG(line, e.Cur.Col)
	case "enter":
		indent := line[:firstNonBlank(line)]
		if e.Opts["listcont"] {
			if m := listRe.FindStringSubmatch(line); m != nil && e.Cur.Col >= len(m[0]) {
				if strings.TrimSpace(line[len(m[0]):]) == "" {
					// Enter on an empty list item ends the list.
					e.del(Pos{e.Cur.Line, 0}, Pos{e.Cur.Line, len(line)})
					e.Cur.Col = 0
					return
				}
				bullet := m[2]
				if n, err := strconv.Atoi(strings.TrimRight(bullet, ".)")); err == nil {
					bullet = strconv.Itoa(n+1) + bullet[len(bullet)-1:]
				}
				indent = m[1] + bullet + m[3]
				if m[4] != "" {
					indent += "[ ] "
				}
			}
		}
		e.Cur = e.ins(e.Cur, "\n"+indent)
	case "backspace", "ctrl+h":
		if e.Cur.Col > 0 {
			p := Pos{e.Cur.Line, prevG(line, e.Cur.Col)}
			e.del(p, e.Cur)
			e.Cur = p
		} else if e.Cur.Line > 0 {
			p := Pos{e.Cur.Line - 1, len(e.Buf.Line(e.Cur.Line - 1))}
			e.del(p, e.Cur)
			e.Cur = p
		}
	case "delete":
		if e.Cur.Col < len(line) {
			e.del(e.Cur, Pos{e.Cur.Line, nextG(line, e.Cur.Col)})
		} else if e.Cur.Line < e.Buf.LineCount()-1 {
			e.del(e.Cur, Pos{e.Cur.Line + 1, 0})
		}
	case "ctrl+w":
		p := e.Cur
		for p.Col > 0 && class(runeAt(line, prevG(line, p.Col)), false) == clsBlank {
			p.Col = prevG(line, p.Col)
		}
		if p.Col > 0 {
			c := class(runeAt(line, prevG(line, p.Col)), false)
			for p.Col > 0 && class(runeAt(line, prevG(line, p.Col)), false) == c {
				p.Col = prevG(line, p.Col)
			}
		}
		e.del(p, e.Cur)
		e.Cur = p
	case "ctrl+u":
		p := Pos{e.Cur.Line, firstNonBlank(line)}
		if p.Col >= e.Cur.Col {
			p.Col = 0
		}
		e.del(p, e.Cur)
		e.Cur = p
	case "tab":
		e.Cur = e.ins(e.Cur, "\t")
	case "left":
		e.Cur.Col = prevG(line, e.Cur.Col)
	case "right":
		e.Cur.Col = nextG(line, e.Cur.Col)
	case "up", "down":
		d := 1
		if k == "up" {
			d = -1
		}
		l := max(0, min(e.Cur.Line+d, e.Buf.LineCount()-1))
		e.Cur = Pos{l, colAtCells(e.Buf.Line(l), Cells(line, e.Cur.Col))}
	default:
		if isText(k) {
			e.Cur = e.ins(e.Cur, k)
		}
	}
}

// Paste inserts bracketed-paste text (insert mode only).
func (e *Engine) Paste(s string) {
	if e.Mode == Insert {
		e.Cur = e.ins(e.Cur, strings.ReplaceAll(s, "\r\n", "\n"))
	}
}

// ---------- ":" and "/" lines ----------

func (e *Engine) cmdKey(k string) {
	switch k {
	case "esc", "ctrl+c":
		e.Mode, e.cmdline, e.opPending = Normal, "", nil
	case "enter":
		line, mode := e.cmdline, e.Mode
		e.Mode, e.cmdline = Normal, ""
		if mode == Search {
			e.setSearch(line)
			if p := e.opPending; p != nil {
				e.opPending = nil
				if to, ok := e.searchFrom(e.opFrom, e.searchBack, max(p.count, 1)); ok {
					a, z, lw := e.motionRange(e.opFrom, to, excl)
					e.operate(p.op, p.reg, a, z, lw)
				}
				return
			}
			e.jumpSearch(false, 1)
		} else {
			e.ex(line)
		}
	case "backspace":
		if e.cmdline == "" {
			e.Mode = Normal
		} else {
			_, n := utf8.DecodeLastRuneInString(e.cmdline)
			e.cmdline = e.cmdline[:len(e.cmdline)-n]
		}
	default:
		if isText(k) {
			e.cmdline += k
		}
	}
}

func (e *Engine) ex(line string) {
	cmd, arg, _ := strings.Cut(strings.TrimSpace(line), " ")
	arg = strings.TrimSpace(arg)
	if n, err := strconv.Atoi(cmd); err == nil {
		e.Cur = Pos{max(0, min(n-1, e.Buf.LineCount()-1)), 0}
		e.Cur.Col = firstNonBlank(e.Buf.Line(e.Cur.Line))
		return
	}
	switch cmd {
	case "w", "write":
		e.write(arg)
	case "wq", "x":
		if e.write(arg) {
			e.Quit = true
		}
	case "q", "quit":
		if e.Dirty {
			e.Msg = "E37: No write since last change (add ! to override)"
			return
		}
		e.Quit = true
	case "q!":
		e.Quit = true
	case "e", "edit", "e!":
		if e.Dirty && cmd != "e!" {
			e.Msg = "E37: No write since last change (add ! to override)"
			return
		}
		if arg == "" {
			arg = e.Path
		}
		e.load(arg)
	case "set", "se":
		e.set(arg)
	case "noh", "nohlsearch":
		e.search = nil
	case "":
	default:
		e.Msg = "E492: Not an editor command: " + line
	}
}

func (e *Engine) write(path string) bool {
	if path == "" {
		path = e.Path
	}
	if err := e.fs.WriteFile(path, e.Buf.String()); err != nil {
		e.Msg = "E212: " + err.Error()
		return false
	}
	e.Path, e.Dirty = path, false
	e.Msg = fmt.Sprintf("%q %dL written", path, e.Buf.LineCount())
	return true
}

func (e *Engine) set(arg string) {
	for _, o := range strings.Fields(arg) {
		switch {
		case strings.HasSuffix(o, "!"):
			o = strings.TrimSuffix(o, "!")
			e.Opts[o] = !e.Opts[o]
		case strings.HasPrefix(o, "no"):
			o = strings.TrimPrefix(o, "no")
			e.Opts[o] = false
		default:
			e.Opts[o] = true
		}
		e.Msg = fmt.Sprintf("%s=%v", o, e.Opts[o])
	}
}

func (e *Engine) setSearch(pat string) {
	if pat == "" {
		return // reuse last pattern
	}
	e.searchInput = pat
	if strings.ToLower(pat) == pat {
		pat = "(?i)" + pat // smartcase
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		re = regexp.MustCompile(regexp.QuoteMeta(e.searchInput))
	}
	e.search = re
}

// searchFrom returns the nth match start after (or before) p, wrapping.
func (e *Engine) searchFrom(p Pos, back bool, n int) (Pos, bool) {
	if e.search == nil {
		e.Msg = "E35: No previous regular expression"
		return p, false
	}
	text := e.Buf.String()
	ms := e.search.FindAllStringIndex(text, -1)
	if len(ms) == 0 {
		e.Msg = "E486: Pattern not found: " + e.searchInput
		return p, false
	}
	o := e.Buf.offset(p)
	idx := -1
	for k := 0; k < n; k++ {
		idx = -1
		if back {
			for i := len(ms) - 1; i >= 0; i-- {
				if ms[i][0] < o {
					idx = i
					break
				}
			}
			if idx < 0 {
				idx = len(ms) - 1
				e.Msg = "search hit TOP, continuing at BOTTOM"
			}
		} else {
			for i, m := range ms {
				if m[0] > o {
					idx = i
					break
				}
			}
			if idx < 0 {
				idx = 0
				e.Msg = "search hit BOTTOM, continuing at TOP"
			}
		}
		o = ms[idx][0]
	}
	return e.Buf.posAt(o), true
}

func (e *Engine) jumpSearch(reverse bool, n int) {
	if p, ok := e.searchFrom(e.Cur, e.searchBack != reverse, n); ok {
		e.Cur = p
		e.want = Cells(e.Buf.Line(p.Line), p.Col)
	}
}
