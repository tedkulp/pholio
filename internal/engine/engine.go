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
)

func (m Mode) String() string {
	return [...]string{"NORMAL", "INSERT"}[m]
}

// Engine is one editing session over a Buffer.
type Engine struct {
	Buf  *Buffer
	Cur  Pos
	Mode Mode
	Msg  string

	// Dirty is set by any change to the buffer.
	Dirty bool

	// ListContinuation makes enter and o continue list items and Tasks.
	ListContinuation bool

	// PageLines is the viewport height, set by the host so that
	// ctrl+d/u/f/b know how far to scroll.
	PageLines int

	want     int // remembered display column (cells) for j/k; -1 = end of line
	keptWant bool
	lastFind string // last f/t/F/T and its char, such as "f;"

	keys     []string // pending normal-mode keys (the command being parsed)
	cmdStart Pos      // cursor when the current command began
	depth    int      // Feed nesting; dot repeat replays through Feed

	dot       []string // the last change's keys, without register and count
	dotCount  int
	dotReg    rune
	recording bool // still appending insert-mode keys to dot
	replaying bool

	regs map[rune]register

	undo, redo []change
	pending    *change // edits of the command in progress
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

// OperatorPending is true while d, c or y waits for its motion or object.
func (e *Engine) OperatorPending() bool {
	p, st := parse(e.keys)
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
	if e.Mode == Insert {
		e.insertKey(k)
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
	if e.Mode != Normal {
		return
	}
	e.clampNormal()
	if len(e.keys) == 0 && !e.keptWant {
		e.want = Cells(e.line(e.Cur.Line), e.Cur.Col)
	}
	e.keptWant = false
	if len(e.keys) == 0 {
		e.commit()
	}
}

func (e *Engine) clampNormal() {
	e.Cur.Line = max(0, min(e.Cur.Line, e.Buf.LineCount()-1))
	l := e.line(e.Cur.Line)
	e.Cur.Col = max(0, min(e.Cur.Col, lastG(l)))
}

func (e *Engine) line(i int) string { return e.Buf.Line(i) }
