// Package engine is pholio's vim editing engine. It has no Bubble Tea
// dependency: input is a stream of key names (Feed) and output is plain
// state (buffer, cursor, mode, pending keys and message) that a view reads.
package engine

import (
	"strings"
	"unicode/utf8"
)

// Mode is the editing mode.
type Mode int

// Modes.
const (
	Normal Mode = iota
	Insert
	Visual
	VisualLine
	VisualBlock
	Command // the ":" line
	Search  // the "/" or "?" line
)

func (m Mode) String() string {
	return [...]string{"NORMAL", "INSERT", "VISUAL", "V-LINE", "V-BLOCK", "COMMAND", "SEARCH"}[m]
}

// visual reports whether m is one of the visual modes.
func (m Mode) visual() bool { return m == Visual || m == VisualLine || m == VisualBlock }

// Engine is one editing session over a Buffer.
type Engine struct {
	Buf  *Buffer
	Cur  Pos
	Mode Mode
	Msg  string

	// Path is the file being edited; "" for a buffer with no file.
	Path string

	// Dirty is set by any change to the buffer and cleared by :w.
	Dirty bool

	// Quit is set by :q, :wq and :q!. The host closes the editor.
	Quit bool

	// ListContinuation makes enter and o continue list items and Tasks.
	ListContinuation bool

	// PageLines is the viewport height, set by the host so that
	// ctrl+d/u/f/b know how far to scroll.
	PageLines int

	// Fixup, when set, runs as each change completes (see FixupFunc).
	Fixup FixupFunc

	fs     FS
	exCmds map[string]ExFunc

	want     int // remembered display column (cells) for j/k; -1 = end of line
	keptWant bool
	anchor   Pos          // the fixed end of a visual selection
	blockIns *blockInsert // the insert session started from a block, if any
	cmd      cmdlineState
	search   searchState
	lastFind string // last f/t/F/T and its char, such as "f;"

	keys     []string // pending normal-mode keys (the command being parsed)
	cmdStart Pos      // cursor when the current command began
	depth    int      // Feed nesting; dot repeat replays through Feed

	dot       []string // the last change's keys, without register and count
	dotCount  int
	dotReg    rune
	dotVis    *visualDot // set when the last change was made from visual mode
	recording bool       // still appending insert-mode keys to dot
	replaying bool

	regs map[rune]register

	undo, redo []change
	pending    *change // edits of the command in progress
	changes    uint64  // the last change id handed out
	baseID     uint64  // the id of the text as loaded, before any change
	saved      uint64  // the id of the text last saved
	inserted   bool    // the pending change included an insert session
	fixing     bool    // the Fixup hook is running
}

// New returns an Engine in normal mode with the cursor at the start of text.
func New(text string) *Engine {
	return &Engine{
		Buf:              NewBuffer(text),
		regs:             map[rune]register{},
		ListContinuation: true,
		PageLines:        20,
	}
}

// SetCursor moves the cursor to p, clamped to the buffer, and makes its
// column the one j/k remember.
func (e *Engine) SetCursor(p Pos) {
	e.Cur = p
	e.clampNormal()
	e.want = Cells(e.line(e.Cur.Line), e.Cur.Col)
}

// PendingKeys is the normal-mode command typed so far, such as `"a2d`.
func (e *Engine) PendingKeys() string { return strings.Join(e.keys, "") }

// OperatorPending is true while an operator (d, c, y, >, <) waits for its
// motion or object.
func (e *Engine) OperatorPending() bool {
	p, st := parse(e.keys, e.Mode.visual())
	return st == incomplete && p.op != ""
}

var special = map[string]bool{
	"esc": true, "enter": true, "backspace": true, "tab": true, "delete": true,
	"up": true, "down": true, "left": true, "right": true, "home": true, "end": true,
	"pgup": true, "pgdown": true,
}

// isText reports whether k is literal text rather than a key name.
func isText(k string) bool {
	return k != "" && !special[k] && (utf8.RuneCountInString(k) == 1 || !strings.Contains(k, "+"))
}

// Feed processes one key. A key is printable text ("a", "Ж", "👍🏽") or a
// name ("esc", "enter", "ctrl+r", "alt+u").
func (e *Engine) Feed(k string) {
	// A fast "<esc>x" reaches the terminal as "alt+x". Outside normal mode
	// nothing is bound to alt, so split it back into the two keys.
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
		return
	case Command, Search:
		e.cmdKey(k)
		return
	}
	if len(e.keys) == 0 {
		e.cmdStart = e.Cur
		if e.depth == 1 {
			e.Msg = ""
		}
	}
	e.keys = append(e.keys, k)
	e.normalKey()
}

// settle runs after each top-level key. It keeps the cursor legal and, once
// the engine is back at rest in normal mode, commits the undo step.
func (e *Engine) settle() {
	if e.Mode != Normal && !e.Mode.visual() {
		return
	}
	e.clampNormal()
	if len(e.keys) == 0 && !e.keptWant {
		e.want = Cells(e.line(e.Cur.Line), e.Cur.Col)
	}
	e.keptWant = false
	if len(e.keys) == 0 {
		e.fixup()
		e.commit()
	}
}

func (e *Engine) clampNormal() {
	e.Cur.Line = max(0, min(e.Cur.Line, e.Buf.LineCount()-1))
	l := e.line(e.Cur.Line)
	e.Cur.Col = max(0, min(e.Cur.Col, lastG(l)))
}

func (e *Engine) line(i int) string { return e.Buf.Line(i) }
