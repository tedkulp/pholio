package app

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/tasks"
	"github.com/tedkulp/pholio/internal/theme"
)

// taskView is what the Task List shows besides the Tasks: the filter, and
// whether D (all done and cancelled Tasks) is on.
type taskView struct {
	query string
	all   bool
}

// taskGroup is one of the Task List's groups, in display order.
type taskGroup struct {
	name string
	slot theme.Slot
}

var (
	groupOverdue  = taskGroup{"Overdue", theme.TasksOverdue}
	groupToday    = taskGroup{"Today", theme.TasksToday}
	groupUpcoming = taskGroup{"Upcoming", theme.TasksUpcoming}
	groupNoDate   = taskGroup{"No date", theme.TasksNoDate}
	groupDone     = taskGroup{"Done today", theme.MarkdownTaskDone}
	groupAllDone  = taskGroup{"Done and cancelled", theme.MarkdownTaskDone}
)

// showTaskList (spc t, :tasks) opens the Task List in list mode.
func (m Model) showTaskList(v taskView) (Model, tea.Cmd) {
	if m.index() == nil {
		return m.say("The Task List needs the Vault index", true), nil
	}
	m = m.syncIndex()
	p := palette.New("Tasks", palette.List).
		WithPlaceholder("/ filter: text, #tag, file").
		WithMatcher(func(_ string, items []palette.Item) []int { // items are filtered already
			all := make([]int, len(items))
			for i := range all {
				all[i] = i
			}
			return all
		}).
		WithQuery(v.query)
	m = m.showPalette(p, nil)
	m = m.taskItems(v, 0)
	m.overlay.ready = func(m Model) Model { return m.taskItems(v, 0) }
	return m, nil
}

// taskItems fills the open Task List for v, selecting row sel, and points
// its handler at v.
func (m Model) taskItems(v taskView, sel int) Model {
	p := m.overlay.p.WithHint(taskHint(v))
	m.overlay.on = func(m Model, ev palette.Event) (Model, tea.Cmd) { return m.taskEvent(v, ev) }
	if !m.indexReady() {
		m.overlay.p = p.WithEmpty("indexing…").SetItems(nil)
		return m
	}
	p = p.WithEmpty("no Tasks").SetItems(m.taskRows(v))
	m.overlay.p = p.Select(sel)
	return m
}

func taskHint(v taskView) string {
	d := "D all done"
	if v.all {
		d = "D done today"
	}
	return "space toggle · enter open · a add · " + d + " · / filter · esc close"
}

// taskEvent handles the Task List's keys.
func (m Model) taskEvent(v taskView, ev palette.Event) (Model, tea.Cmd) {
	switch ev.Kind {
	case palette.Changed:
		v.query = ev.Query
		return m.taskItems(v, 0), nil
	case palette.Chosen:
		if !ev.OK {
			return m, nil
		}
		t := ev.Item.Value.(index.Task)
		return m.openAt(m.abs(t.Path), func(m Model) Model {
			m.ed.Engine().SetCursor(engine.Pos{Line: t.Line, Col: t.Indent})
			return m
		})
	case palette.Key:
		_, sel, _ := m.overlay.p.Selected()
		switch ev.Key {
		case " ":
			return m.toggleListed(v, sel), nil
		case "D":
			v.all = !v.all
			return m.taskItems(v, 0), nil
		case "a":
			return m.promptTask(v), nil
		}
	}
	return m, nil
}

// toggleListed toggles the selected Task, in its buffer when its Note is
// open and on disk otherwise, and refreshes the list keeping the row.
func (m Model) toggleListed(v taskView, sel int) Model {
	it, _, ok := m.overlay.p.Selected()
	if !ok {
		return m
	}
	t := it.Value.(index.Task)
	m, err := m.toggleTask(m.abs(t.Path), t.Line)
	if err != nil {
		m = m.say(err.Error(), true)
	} else {
		m = m.syncIndex()
	}
	return m.taskItems(v, sel)
}

// taskRows are the index's Tasks that v shows, grouped and sorted.
func (m Model) taskRows(v taskView) []palette.Item {
	today := m.today()
	byGroup := map[taskGroup][]index.Task{}
	for _, t := range m.index().Tasks() {
		if g, ok := groupOf(t, today, v.all); ok && taskMatches(t, v.query) {
			byGroup[g] = append(byGroup[g], t)
		}
	}
	last := groupDone
	if v.all {
		last = groupAllDone
	}
	var items []palette.Item
	for _, g := range []taskGroup{groupOverdue, groupToday, groupUpcoming, groupNoDate, last} {
		ts := byGroup[g]
		slices.SortFunc(ts, compareTasks)
		for _, t := range ts {
			slot := g.slot
			if t.Meta["pri"] == "high" && t.Status.IsOpen() && g != groupOverdue {
				slot = theme.TasksPriHigh
			}
			items = append(items, palette.Item{
				Text:       fmt.Sprintf("[%c] %s", t.Mark, t.Text),
				Detail:     fmt.Sprintf("%s:%d", t.Path, t.Line+1),
				DetailSlot: theme.TasksSource,
				Group:      fmt.Sprintf("%s (%d)", g.name, len(ts)),
				Slot:       slot,
				Value:      t,
			})
		}
	}
	return items
}

// groupOf is the group t is listed in. ok is false for a Task the list
// leaves out: done before today or cancelled, unless all is on.
func groupOf(t index.Task, today time.Time, all bool) (taskGroup, bool) {
	if !t.Status.IsOpen() {
		switch {
		case all:
			return groupAllDone, true
		case t.Status == index.Done && t.Meta["done"] == today.Format(dates.ISO):
			return groupDone, true
		}
		return taskGroup{}, false
	}
	due, err := time.ParseInLocation(dates.ISO, t.Meta["due"], today.Location())
	switch {
	case err != nil:
		return groupNoDate, true
	case due.Before(today):
		return groupOverdue, true
	case due.Equal(today):
		return groupToday, true
	}
	return groupUpcoming, true
}

// compareTasks orders Tasks by due date (undated last), then pri (high,
// med, low, none), then file and line.
func compareTasks(a, b index.Task) int {
	da, db := a.Meta["due"], b.Meta["due"]
	if (da == "") != (db == "") {
		if da == "" {
			return 1
		}
		return -1
	}
	return cmp.Or(
		strings.Compare(da, db),
		cmp.Compare(priRank(a), priRank(b)),
		strings.Compare(a.Path, b.Path),
		cmp.Compare(a.Line, b.Line),
	)
}

func priRank(t index.Task) int {
	switch strings.ToLower(t.Meta["pri"]) {
	case "high":
		return 0
	case "med":
		return 1
	case "low":
		return 2
	}
	return 3
}

// taskMatches is the Task List filter. Each word of query must match:
// "#tag" a tag (by prefix), anything else the Task's text or file
// (ignoring case).
func taskMatches(t index.Task, query string) bool {
	text := strings.ToLower(t.Text + " " + t.Path)
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if tag, ok := strings.CutPrefix(w, "#"); ok && tag != "" {
			if !slices.ContainsFunc(t.Tags, func(s string) bool { return strings.HasPrefix(strings.ToLower(s), tag) }) {
				return false
			}
			continue
		}
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// promptTask (a) asks for a new Task's text. Either way it returns to the
// Task List.
func (m Model) promptTask(v taskView) Model {
	p := palette.New("Add Task to today's Daily Note", palette.Type).
		WithPlaceholder("text  due:tomorrow pri:high #tag").
		WithHint("enter add · esc back")
	return m.showPalette(p, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind == palette.Chosen && strings.TrimSpace(ev.Query) != "" {
			m = m.addTask(ev.Query)
		}
		if ev.Kind == palette.Chosen || ev.Kind == palette.Closed {
			return m.showTaskList(v)
		}
		return m, nil
	})
}

// addTask puts "- [ ] text" under tasks_heading in today's Daily Note
// (see tasks.AddAt), creating the Note if needed. Relative dates are
// expanded. When the Note is open the line goes into its buffer.
func (m Model) addTask(text string) Model {
	day := m.today()
	line := tasks.Expand("- [ ] "+strings.TrimSpace(text), day)
	heading := m.config().TasksHeading
	path := m.dailyNotes().Path(day)
	err := m.editNote(path, func(e *engine.Engine) error {
		lines := strings.Split(strings.TrimSuffix(e.Buf.String(), "\n"), "\n")
		e.InsertLine(tasks.AddAt(lines, heading), line)
		return nil
	}, func(f *noteFile) error {
		if _, err := m.ensureDaily(day); err != nil {
			return err
		}
		return tasks.AddFile(f, path, heading, line)
	})
	if err != nil {
		return m.say("adding a Task: "+err.Error(), true)
	}
	if path == m.path() {
		m = m.syncIndex()
	}
	return m.say("Added a Task to "+m.rel(path), false)
}
