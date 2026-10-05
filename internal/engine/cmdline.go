package engine

import "unicode/utf8"

// cmdlineState is the ":" or "/" line being typed.
type cmdlineState struct {
	text      string
	prev      Mode    // the mode to return to
	back      bool    // "?" rather than "/"
	opPending *parsed // an operator waiting for a "/" or "?" motion, as in d/foo
}

// CmdLine returns the prompt (":", "/" or "?") and the text typed after it.
func (e *Engine) CmdLine() (prompt, text string) {
	switch {
	case e.Mode == Command:
		return ":", e.cmd.text
	case e.cmd.back:
		return "?", e.cmd.text
	default:
		return "/", e.cmd.text
	}
}

// startCmdline enters the ":" line (Command) or the "/" or "?" line (Search).
func (e *Engine) startCmdline(m Mode, back bool) {
	e.cmd = cmdlineState{prev: e.Mode, back: back}
	e.Mode = m
}

func (e *Engine) cmdKey(k string) {
	switch k {
	case "esc", "ctrl+c":
		e.Mode = e.cmd.prev
		e.cmd.opPending = nil
	case "enter":
		mode, text := e.Mode, e.cmd.text
		e.Mode = e.cmd.prev
		if mode == Command {
			e.ex(text)
		} else {
			e.searchEnter(text)
		}
	case "backspace", "ctrl+h":
		if e.cmd.text == "" {
			e.cmdKey("esc")
			return
		}
		_, n := utf8.DecodeLastRuneInString(e.cmd.text)
		e.cmd.text = e.cmd.text[:len(e.cmd.text)-n]
	case "ctrl+u":
		e.cmd.text = ""
	default:
		if isText(k) {
			e.cmd.text += k
		}
	}
}

// searchEnter runs the "/" or "?" line: a jump, or the motion of a pending
// operator.
func (e *Engine) searchEnter(pat string) {
	e.compileSearch(pat)
	e.search.back = e.cmd.back
	p := e.cmd.opPending
	e.cmd.opPending = nil
	to, ok := e.searchFrom(e.Cur, e.search.back, 1)
	if !ok {
		return
	}
	if p == nil {
		e.Cur = to
		return
	}
	a, z, lw := e.motionRange(e.Cur, to, excl)
	e.operate(p.op, p.reg, a, z, lw)
	// "." repeats the whole command, pattern included.
	dot := *p
	dot.rest = append(append([]string(nil), p.rest...), splitKeys(pat)...)
	dot.rest = append(dot.rest, "enter")
	e.setDot(dot)
}

// splitKeys splits typed text back into one key per rune.
func splitKeys(s string) []string {
	var out []string
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}
