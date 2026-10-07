// Package tasks holds the editing rules for Task lines: relative-date
// expansion of due and done dates, and the done date stamped when a Task's
// Status changes. It works on plain lines, so the editor and the Task List
// share it. Rules that write Task Metadata take the Vault's default Task
// Format.
package tasks

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/index"
)

// Expand rewrites relative due and done dates on a Task line (today,
// tomorrow, a weekday, +3d, +1w, ...), in either Task Format, to ISO dates
// counted from today. Any other line, and any value that isn't a date, is
// returned as it is.
func Expand(line string, today time.Time) string {
	if _, ok := index.ParseTask(line); !ok {
		return line
	}
	fs := index.Fields(line)
	for i := len(fs) - 1; i >= 0; i-- {
		f := fs[i]
		if !index.TakesRelativeDate(f.Key) {
			continue
		}
		d, ok := dates.Parse(f.Value, today)
		if !ok {
			continue
		}
		if iso := d.Format(dates.ISO); iso != f.Value {
			line = line[:f.ValStart] + iso + line[f.ValEnd:]
		}
	}
	return line
}

// isDone reports whether line is a Task whose status is done.
func isDone(line string) bool {
	t, ok := index.ParseTask(line)
	return ok && t.Status == index.Done
}

// Stamp applies the done-date rule to a line that changed from old to
// changed. When changed is a done Task and old wasn't, it gets today's done
// date (see MarkDone). When old was a done Task and changed is a Task that
// isn't done, its done dates are removed. Otherwise changed is returned as
// it is.
func Stamp(old, changed string, def index.Format, today time.Time) string {
	was, is := isDone(old), isDone(changed)
	switch {
	case is && !was:
		return MarkDone(changed, def, today)
	case was && !is:
		if _, ok := index.ParseTask(changed); ok {
			return Unmark(changed)
		}
	}
	return changed
}

// Fix replaces line Line (0-based) with Text.
type Fix struct {
	Line int
	Text string
}

// Fixup compares a buffer's lines before and after one change and returns
// the line rewrites that the Task rules ask for: the done date (see Stamp)
// on each Task whose status changed and, when expand is set, relative-date
// expansion (see Expand) on each changed line. Lines outside the changed
// region, and lines in fenced code, are never touched.
//
// A changed line is matched with its earlier self by its text with the
// checkbox mark and done dates ignored, or, failing that, by position
// when the change kept the line count. A line with no earlier self (such as
// a newly typed Task) gets no stamp.
func Fixup(before, after []string, expand bool, def index.Format, today time.Time) []Fix {
	p := 0
	for p < len(before) && p < len(after) && before[p] == after[p] {
		p++
	}
	s := 0
	for s < len(before)-p && s < len(after)-p && before[len(before)-1-s] == after[len(after)-1-s] {
		s++
	}
	olds, news := before[p:len(before)-s], after[p:len(after)-s]

	byKey := map[string][]int{}
	for j, o := range olds {
		if k, ok := key(o); ok {
			byKey[k] = append(byKey[k], j)
		}
	}
	used := make([]bool, len(olds))
	paired := make([]int, len(news))
	for i, n := range news {
		paired[i] = -1
		k, ok := key(n)
		if js := byKey[k]; ok && len(js) > 0 {
			used[js[0]], paired[i] = true, js[0]
			byKey[k] = js[1:]
		}
	}
	if len(olds) == len(news) {
		for i := range news {
			if paired[i] < 0 && !used[i] {
				used[i], paired[i] = true, i
			}
		}
	}

	var fences index.Fences
	for _, l := range after[:p] {
		fences.Code(l)
	}
	var fixes []Fix
	for i, n := range news {
		if fences.Code(n) {
			continue
		}
		text := n
		if expand {
			text = Expand(text, today)
		}
		if j := paired[i]; j >= 0 {
			text = Stamp(olds[j], text, def, today)
		}
		if text != n {
			fixes = append(fixes, Fix{Line: p + i, Text: text})
		}
	}
	return fixes
}

// key is a Task line with its mark and done dates blanked out, so a Task
// matches itself across a change of status. ok is false for other lines.
func key(line string) (string, bool) {
	t, ok := index.ParseTask(line)
	if !ok {
		return line, false
	}
	at := checkbox(line, t)
	return line[:at] + "?" + Unmark(line[at+utf8.RuneLen(t.Mark):]), true
}

// checkbox returns the byte offset of t's mark in line. The first '[' after
// the indent opens the checkbox: list markers are "-", "*", "+" or digits
// followed by "." or ")".
func checkbox(line string, t index.Task) int {
	return t.Indent + strings.IndexByte(line[t.Indent:], '[') + 1
}

// Toggle flips a Task line between done and open: a done Task becomes
// "[ ]" and loses its done date, and any other Task becomes "[x]" with
// today's (see MarkDone). ok is false, and line is returned, when it isn't
// a Task.
func Toggle(line string, def index.Format, today time.Time) (string, bool) {
	t, ok := index.ParseTask(line)
	if !ok {
		return line, false
	}
	at := checkbox(line, t)
	mark, rest := "x", line[at+utf8.RuneLen(t.Mark):]
	if t.Status == index.Done {
		mark = " "
	}
	toggled := line[:at] + mark + rest
	return Stamp(line, toggled, def, today), true
}

// blockID matches a trailing block ID ("^abc123") and the blanks around it.
var blockID = regexp.MustCompile(`[ \t]+\^[A-Za-z0-9-]+[ \t]*$`)

// MarkDone puts today's done date at the end of line, before a trailing
// block ID, replacing any done date already there. It is written in the
// format of the line's first Task Metadata field, or def when there is
// none. It doesn't change the checkbox.
func MarkDone(line string, def index.Format, today time.Time) string {
	if fs := index.Fields(line); len(fs) > 0 {
		def = fs[0].Format
	}
	date := today.Format(dates.ISO)
	field := "[completion:: " + date + "]"
	if def == index.Emoji {
		field = "✅ " + date
	}
	line = Unmark(line)
	id := ""
	if loc := blockID.FindStringIndex(line); loc != nil {
		line, id = line[:loc[0]], strings.TrimRight(line[loc[0]:], " \t")
	}
	return strings.TrimRight(line, " \t") + " " + field + id
}

// Unmark removes the done dates, in either Task Format, from line, with the
// blanks before them. It doesn't change the checkbox.
func Unmark(line string) string {
	fs := index.Fields(line)
	for i := len(fs) - 1; i >= 0; i-- {
		if f := fs[i]; f.Key == "completion" {
			start := len(strings.TrimRight(line[:f.Start], " \t"))
			line = line[:start] + line[f.End:]
		}
	}
	return line
}
