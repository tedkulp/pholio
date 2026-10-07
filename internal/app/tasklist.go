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

// showTaskListAt opens the Task List with the Task at line (0-based) of
// the Note at path (Vault-relative) selected, or the first row when it
// isn't listed.
func (m Model) showTaskListAt(v taskView, path string, line int) (Model, tea.Cmd) {
	m, cmd := m.showTaskList(v)
	if m.overlay == nil || !m.indexReady() {
		return m, cmd
	}
	sel := slices.IndexFunc(m.overlay.p.Visible(), func(it palette.Item) bool {
		t := it.Value.(index.Task)
		return t.Path == path && t.Line == line
	})
	return m.taskItems(v, max(0, sel)), cmd
}

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
	return "spc toggle · enter open · e edit · a add · " + d + " · / filter · esc close"
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
			return m.addListed(v), nil
		case "e":
			return m.editListed(v)
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
			if p := t.Pri(); (p == "highest" || p == "high") && t.Status.IsOpen() && g != groupOverdue {
				slot = theme.TasksPriHigh
			}
			items = append(items, palette.Item{
				Text:       fmt.Sprintf("[%c] %s", t.Mark, t.Text),
				Detail:     lineRef(t.Path, t.Line),
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
		case t.Status == index.Done && t.DoneOn() == today.Format(dates.ISO):
			return groupDone, true
		}
		return taskGroup{}, false
	}
	due, err := time.ParseInLocation(dates.ISO, t.Due(), today.Location())
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

// compareTasks orders Tasks by due date (undated last), then priority
// (see priRank), then file and line.
func compareTasks(a, b index.Task) int {
	da, db := a.Due(), b.Due()
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

// priRank is t's place in Obsidian's priority order: highest, high,
// medium, none, low, lowest.
func priRank(t index.Task) int {
	switch t.Pri() {
	case "highest":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 4
	case "lowest":
		return 5
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

// addTask puts a Task line under tasks_heading in today's Daily Note (see
// tasks.AddAt), creating the Note if needed. When the Note is open the
// line goes into its buffer.
func (m Model) addTask(line string) Model {
	day := m.today()
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
