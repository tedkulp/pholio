package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/form"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/tasks"
)

// The Task Editor's rows, in order.
const (
	rowDescription = iota
	rowStatus
	rowDue
	rowScheduled
	rowStart
	rowPriority
)

// statusNames are the Status row's choices, indexed by index.Status, so
// they follow its order.
var statusNames = []string{"open", "in progress", "done", "cancelled"}

// priorityNames are the Priority row's choices; "none" means no field.
var priorityNames = []string{"none", "highest", "high", "medium", "low", "lowest"}

// taskEditorHint is the Task Editor's footer line.
const taskEditorHint = "tab next · spc ←/→ change · enter save · esc cancel"

// taskForm is the Task Editor's form, filled from v.
func taskForm(title string, v tasks.Values) form.Model {
	pri := v.Priority
	if pri == "" {
		pri = "none"
	}
	date := "date: 2026-10-10, tomorrow, fri, +3d"
	return form.New(title,
		form.Field{Label: "Description", Value: v.Description, Placeholder: "text #tag"},
		form.Field{Label: "Status", Kind: form.Choice, Choices: statusNames, Value: statusNames[v.Status]},
		form.Field{Label: "Due", Value: v.Due, Placeholder: date},
		form.Field{Label: "Scheduled", Value: v.Scheduled, Placeholder: date},
		form.Field{Label: "Start", Value: v.Start, Placeholder: date},
		form.Field{Label: "Priority", Kind: form.Choice, Choices: priorityNames, Value: pri},
	).WithHint(taskEditorHint)
}

// taskValues reads the form's rows back, with relative dates made ISO
// dates counted from Today. bad lists the rows that are refused: an empty
// Description and dates that aren't dates.
func (m Model) taskValues(rows []string) (v tasks.Values, bad []int) {
	v.Description = strings.TrimSpace(rows[rowDescription])
	if v.Description == "" {
		bad = append(bad, rowDescription)
	}
	for s := index.Open; s <= index.Cancelled; s++ {
		if statusNames[s] == rows[rowStatus] {
			v.Status = s
		}
	}
	for row, to := range map[int]*string{rowDue: &v.Due, rowScheduled: &v.Scheduled, rowStart: &v.Start} {
		s := strings.TrimSpace(rows[row])
		if s == "" {
			continue
		}
		d, ok := dates.Parse(s, m.today())
		if !ok {
			bad = append(bad, row)
			continue
		}
		*to = d.Format(dates.ISO)
	}
	if p := rows[rowPriority]; p != "none" {
		v.Priority = p
	}
	slices.Sort(bad)
	return v, bad
}

// showTaskEditor opens the Task Editor on v. On enter with valid rows it
// closes and calls save; then, as after esc, it calls after.
func (m Model) showTaskEditor(title string, v tasks.Values, save func(Model, tasks.Values) Model, after func(Model) (Model, tea.Cmd)) Model {
	return m.showForm(taskForm(title, v), func(m Model, ev form.Event) (Model, tea.Cmd) {
		if ev.Kind == form.Saved {
			v, bad := m.taskValues(ev.Values)
			if len(bad) > 0 {
				f := m.overlay.form.WithInvalid(bad...)
				m.overlay.form = &f
				return m, nil
			}
			m.overlay = nil
			m = save(m, v)
		}
		return after(m)
	})
}

// editTaskAtCursor (spc T, :task) opens the Task Editor on the Task on the
// cursor line of the open Note. It saves into the buffer.
func (m Model) editTaskAtCursor() (Model, tea.Cmd) {
	e := m.ed.Engine()
	line := e.Cur.Line
	v, ok := tasks.Read(e.Buf.Line(line))
	if !ok || inCode(e, line) {
		return m.say("not a Task", true), nil
	}
	path := m.path()
	return m.showTaskEditor("Edit Task", v, func(m Model, v tasks.Values) Model {
		return m.saveTask(path, line, v)
	}, func(m Model) (Model, tea.Cmd) { return m, nil }), nil
}

// inCode reports whether line of e's buffer is in fenced code, where a
// checkbox isn't a Task.
func inCode(e *engine.Engine, line int) bool {
	var f index.Fences
	for i := 0; i < line; i++ {
		f.Code(e.Buf.Line(i))
	}
	return f.Code(e.Buf.Line(line))
}

// saveTask rebuilds the Task at line (0-based) of the Note at path from v
// (see tasks.Rewrite). In the open Note it is one undoable edit to the
// buffer, which is not saved; any other Note is rewritten on disk. An
// error is reported on the message line.
func (m Model) saveTask(path string, line int, v tasks.Values) Model {
	path = filepath.Clean(path)
	def, today := m.config().TaskFormat, m.today()
	err := m.editNote(path, func(e *engine.Engine) error {
		if !tasks.RewriteLine(e, line, v, def, today) {
			return fmt.Errorf("%s:%d: %w", m.rel(path), line+1, tasks.ErrNotATask)
		}
		return nil
	}, func(f *noteFile) error {
		return tasks.RewriteFile(f, path, line, v, def, today)
	})
	if err != nil {
		return m.say("saving the Task: "+err.Error(), true)
	}
	return m.syncIndex()
}

// editListed (e) opens the Task Editor on the Task List's selected Task,
// then returns to the list with that Task still selected.
func (m Model) editListed(v taskView) (Model, tea.Cmd) {
	it, _, ok := m.overlay.p.Selected()
	if !ok {
		return m, nil
	}
	t := it.Value.(index.Task)
	vals, ok := tasks.Read(fmt.Sprintf("- [%c] %s", t.Mark, t.Text))
	if !ok {
		return m, nil
	}
	return m.showTaskEditor("Edit Task", vals, func(m Model, vals tasks.Values) Model {
		return m.saveTask(m.abs(t.Path), t.Line, vals)
	}, func(m Model) (Model, tea.Cmd) { return m.showTaskListAt(v, t.Path, t.Line) }), nil
}

// addListed (a) opens an empty Task Editor and adds the Task it makes to
// today's Daily Note (see addTask). Either way it returns to the Task List.
func (m Model) addListed(v taskView) Model {
	return m.showTaskEditor("Add Task to today's Daily Note", tasks.Values{}, func(m Model, vals tasks.Values) Model {
		// Relative dates typed into the Description are expanded, as the
		// old one-line prompt did.
		line, _ := tasks.Rewrite("- [ ]", vals, m.config().TaskFormat, m.today())
		return m.addTask(tasks.Expand(line, m.today()))
	}, func(m Model) (Model, tea.Cmd) { return m.showTaskList(v) })
}
