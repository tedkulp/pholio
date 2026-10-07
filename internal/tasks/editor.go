package tasks

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
)

// ErrNotATask is returned by ToggleFile when the line isn't a Task (or
// doesn't exist), for example because the file changed since it was read.
var ErrNotATask = errors.New("not a Task")

// Hook is the engine Fixup that applies the Task rules to each change: the
// done date after every change, and relative-date expansion when the
// change included an insert session. today and the default Task Format are
// asked once per change.
func Hook(today func() time.Time, format func() index.Format) engine.FixupFunc {
	return func(e *engine.Engine, before, after []string, inserted bool) {
		for _, f := range Fixup(before, after, inserted, format(), today()) {
			e.SetLine(f.Line, f.Text)
		}
	}
}

// ToggleLine toggles the Task on line (0-based) of e's buffer (see Toggle)
// as one undoable edit. It reports false, changing nothing, when the line
// isn't a Task.
func ToggleLine(e *engine.Engine, line int, def index.Format, today time.Time) bool {
	if line < 0 || line >= e.Buf.LineCount() {
		return false
	}
	text, ok := Toggle(e.Buf.Line(line), def, today)
	if ok {
		e.SetLine(line, text)
	}
	return ok
}

// FS is what ToggleFile needs to read and write a Note.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte) error
}

// ToggleFile toggles the Task on line (0-based) of the file at path (see
// Toggle) and writes the file back, keeping its line endings. It returns
// ErrNotATask, writing nothing, when the line isn't a Task.
func ToggleFile(fsys FS, path string, line int, def index.Format, today time.Time) error {
	data, err := fsys.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	if line < 0 || line >= len(lines) {
		return fmt.Errorf("%s:%d: %w", path, line+1, ErrNotATask)
	}
	l := lines[line]
	body := strings.TrimRight(l, "\r\n")
	text, ok := Toggle(body, def, today)
	if !ok {
		return fmt.Errorf("%s:%d: %w", path, line+1, ErrNotATask)
	}
	lines[line] = text + l[len(body):]
	return fsys.WriteFile(path, []byte(strings.Join(lines, "")))
}
