package engine

import (
	"strconv"
	"strings"
)

const errNoWrite = "E37: No write since last change (add ! to override)"

// ex runs one ":" command line.
func (e *Engine) ex(line string) {
	line = strings.TrimSpace(line)
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if n, err := strconv.Atoi(name); err == nil {
		l := max(0, min(n-1, e.Buf.LineCount()-1))
		e.Cur = Pos{l, firstNonBlank(e.line(l))}
		return
	}
	bang := strings.HasSuffix(name, "!")
	switch strings.TrimSuffix(name, "!") {
	case "":
	case "w", "write":
		e.write(arg)
	case "wq", "x", "xit":
		if e.write(arg) {
			e.Quit = true
		}
	case "q", "quit":
		if e.Dirty && !bang {
			e.Msg = errNoWrite
			return
		}
		e.Quit = true
	case "e", "edit":
		if e.Dirty && !bang {
			e.Msg = errNoWrite
			return
		}
		if arg == "" {
			arg = e.Path
		}
		if err := e.Load(arg); err != nil {
			e.Msg = "E484: Can't open file " + arg
		}
	case "noh", "nohlsearch":
		e.search.hl = false
	default:
		fn, ok := e.exCmds[strings.TrimSuffix(name, "!")]
		if !ok {
			e.Msg = "E492: Not an editor command: " + line
			return
		}
		if err := fn(e, ExCmd{Name: strings.TrimSuffix(name, "!"), Bang: bang, Arg: arg}); err != nil {
			e.Msg = err.Error()
		}
	}
}

// ExCmd is one ":" command line, split up for an ExFunc.
type ExCmd struct {
	Name string // the command name, without "!"
	Bang bool   // the name ended in "!"
	Arg  string // everything after the name, trimmed
}

// ExFunc runs a registered ":" command. It may use e freely, including Feed
// and SetCursor; any buffer changes it makes are one undo step. A returned
// error is shown as the message.
type ExFunc func(e *Engine, c ExCmd) error

// Register adds a ":" command, such as :tasks or :today. Built-in commands
// (:w, :q, :e and the rest) take precedence over registered ones.
func (e *Engine) Register(name string, fn ExFunc) {
	if e.exCmds == nil {
		e.exCmds = map[string]ExFunc{}
	}
	e.exCmds[name] = fn
}
