// Package tasks holds the editing rules for Task lines: relative-date
// expansion of due:/done: and the done: stamp that follows a change of Task
// Status. It works on plain lines, so the editor and the Task List share it.
package tasks

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/index"
)

// dateMeta matches a due: or done: token: group 1 is the key with its colon,
// group 2 the value.
var dateMeta = regexp.MustCompile(`(?:^|[ \t])((?:due|done):)(\S+)`)

// Expand rewrites relative due: and done: values on a Task line (today,
// tomorrow, a weekday, +3d, +1w, ...) to ISO dates counted from today. Any
// other line, and any value that isn't a date, is returned as it is.
func Expand(line string, today time.Time) string {
	if _, ok := index.ParseTask(line); !ok {
		return line
	}
	ms := dateMeta.FindAllStringSubmatchIndex(line, -1)
	for i := len(ms) - 1; i >= 0; i-- {
		vs, ve := ms[i][4], ms[i][5]
		v := line[vs:ve]
		d, ok := dates.Parse(v, today)
		if !ok {
			continue
		}
		if iso := d.Format(dates.ISO); iso != v {
			line = line[:vs] + iso + line[ve:]
		}
	}
	return line
}

// doneMeta matches a done: token and the blanks before it.
var doneMeta = regexp.MustCompile(`[ \t]+done:\S+`)

// isDone reports whether line is a Task whose status is done.
func isDone(line string) bool {
	t, ok := index.ParseTask(line)
	return ok && t.Status == index.Done
}

// Stamp applies the done: rule to a line that changed from old to changed.
// When changed is a done Task and old wasn't, any done: value is replaced by
// done:<today> at the end of the line. When old was a done Task and changed is
// a Task that isn't done, its done: values are removed. Otherwise changed is
// returned as it is.
func Stamp(old, changed string, today time.Time) string {
	was, is := isDone(old), isDone(changed)
	switch {
	case is && !was:
		return MarkDone(changed, today)
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
// the line rewrites that the Task rules ask for: the done: stamp (see Stamp)
// on each Task whose status changed and, when expand is set, relative-date
// expansion (see Expand) on each changed line. Lines outside the changed
// region, and lines in fenced code, are never touched.
//
// A changed line is matched with its earlier self by its text with the
// checkbox mark and done: values ignored, or, failing that, by position
// when the change kept the line count. A line with no earlier self (such as
// a newly typed Task) gets no stamp.
func Fixup(before, after []string, expand bool, today time.Time) []Fix {
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
			text = Stamp(olds[j], text, today)
		}
		if text != n {
			fixes = append(fixes, Fix{Line: p + i, Text: text})
		}
	}
	return fixes
}

// key is a Task line with its mark and done: values blanked out, so a Task
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
// "[ ]" and loses its done: stamp, and any other Task becomes "[x]" with
// done:<today>. ok is false, and line is returned, when it isn't a Task.
func Toggle(line string, today time.Time) (string, bool) {
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
	return Stamp(line, toggled, today), true
}

// MarkDone puts done:<today> at the end of line, replacing any done: value
// already there. It doesn't change the checkbox.
func MarkDone(line string, today time.Time) string {
	return strings.TrimRight(Unmark(line), " \t") + " done:" + today.Format(dates.ISO)
}

// Unmark removes the done: values from line. It doesn't change the
// checkbox.
func Unmark(line string) string {
	return doneMeta.ReplaceAllString(line, "")
}
