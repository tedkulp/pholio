package tasks_test

import (
	"slices"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/tasks"
)

// today is Monday 2026-10-05.
var today = time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local)

func TestExpandRewritesRelativeDueAndDoneToISO(t *testing.T) {
	for in, want := range map[string]string{
		"- [ ] a due:today":              "- [ ] a due:2026-10-05",
		"- [ ] a due:tomorrow":           "- [ ] a due:2026-10-06",
		"- [ ] a due:friday #x":          "- [ ] a due:2026-10-09 #x",
		"- [ ] a due:fri":                "- [ ] a due:2026-10-09",
		"- [ ] a due:monday":             "- [ ] a due:2026-10-12",
		"- [ ] a due:+3d":                "- [ ] a due:2026-10-08",
		"- [ ] a due:+2w":                "- [ ] a due:2026-10-19",
		"- [x] a done:yesterday":         "- [x] a done:2026-10-04",
		"  * [ ] due:Tomorrow  pri:high": "  * [ ] due:2026-10-06  pri:high",
	} {
		if got := tasks.Expand(in, today); got != want {
			t.Errorf("Expand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExpandLeavesOtherTextAlone(t *testing.T) {
	for _, in := range []string{
		"- [ ] a due:2026-10-05",   // already ISO
		"- [ ] a due:someday",      // not a date
		"- [ ] a start:tomorrow",   // not due or done
		"- [ ] a overdue:tomorrow", // key is overdue, not due
		"- a due:tomorrow",         // not a Task
		"due:tomorrow",             // not a Task
		"- [ ] see https://x.io/due:today",
	} {
		if got := tasks.Expand(in, today); got != in {
			t.Errorf("Expand(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestStampAddsDoneWhenATaskBecomesDone(t *testing.T) {
	for _, c := range []struct{ old, new, want string }{
		{"- [ ] a", "- [x] a", "- [x] a done:2026-10-05"},
		{"- [/] a #w", "- [X] a #w", "- [X] a #w done:2026-10-05"},
		{"- [-] a ", "- [x] a ", "- [x] a done:2026-10-05"},
		{"- a", "- [x] a", "- [x] a done:2026-10-05"},
		{"- [ ]", "- [x]", "- [x] done:2026-10-05"},
		{"- [x]", "- [x]", "- [x]"},
		{"", "- [x]", "- [x] done:2026-10-05"},
		{"- [ ] a done:2026-01-01", "- [x] a done:2026-01-01", "- [x] a done:2026-10-05"},
	} {
		if got := tasks.Stamp(c.old, c.new, today); got != c.want {
			t.Errorf("Stamp(%q, %q) = %q, want %q", c.old, c.new, got, c.want)
		}
	}
}

func TestStampRemovesDoneWhenATaskStopsBeingDone(t *testing.T) {
	for _, c := range []struct{ old, new, want string }{
		{"- [x] a done:2026-10-01", "- [ ] a done:2026-10-01", "- [ ] a"},
		{"- [x] a done:2026-10-01 #w", "- [-] a done:2026-10-01 #w", "- [-] a #w"},
		{"- [X] a  done:2026-10-01", "- [/] a  done:2026-10-01", "- [/] a"},
		{"- [x] a done:2026-10-01", "- a done:2026-10-01", "- a done:2026-10-01"}, // no longer a Task
	} {
		if got := tasks.Stamp(c.old, c.new, today); got != c.want {
			t.Errorf("Stamp(%q, %q) = %q, want %q", c.old, c.new, got, c.want)
		}
	}
}

func TestStampLeavesLinesWhoseStatusDidNotChange(t *testing.T) {
	for _, c := range []struct{ old, new string }{
		{"- [x] a", "- [x] ab"},
		{"- [x] a", "- [X] a"},
		{"- [ ] a done:2026-10-01", "- [ ] ab done:2026-10-01"},
		{"- [ ] a", "- [/] a"},
		{"plain", "plain text"},
	} {
		if got := tasks.Stamp(c.old, c.new, today); got != c.new {
			t.Errorf("Stamp(%q, %q) = %q, want it unchanged", c.old, c.new, got)
		}
	}
}

func TestToggleChecksAnOpenTaskAndUnchecksADoneOne(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"- [ ] a", "- [x] a done:2026-10-05"},
		{"  1. [/] a #w", "  1. [x] a #w done:2026-10-05"},
		{"* [-] a", "* [x] a done:2026-10-05"},
		{"- [x] a done:2026-10-01", "- [ ] a"},
		{"- [X] a done:2026-10-01 pri:high", "- [ ] a pri:high"},
		{"- [ж] a", "- [x] a done:2026-10-05"},
		{"- [ ]", "- [x] done:2026-10-05"},
		{"- [x] done:2026-10-01", "- [ ]"},
	} {
		got, ok := tasks.Toggle(c.in, today)
		if !ok || got != c.want {
			t.Errorf("Toggle(%q) = %q, %v; want %q, true", c.in, got, ok, c.want)
		}
	}
}

func TestFixupStampsTheTaskWhoseStatusChanged(t *testing.T) {
	before := []string{"# T", "- [x] old", "- [ ] a", "- [ ] b"}
	after := []string{"# T", "- [x] old", "- [x] a", "- [ ] b"}

	got := tasks.Fixup(before, after, false, today)

	want := []tasks.Fix{{Line: 2, Text: "- [x] a done:2026-10-05"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupFollowsTasksWhenLinesWereAddedOrRemoved(t *testing.T) {
	before := []string{"- [ ] a", "- [x] b done:2026-10-01", "- [ ] c"}
	after := []string{"- [ ] new", "- [x] a", "- [ ] b done:2026-10-01"}

	got := tasks.Fixup(before, after, false, today)

	want := []tasks.Fix{
		{Line: 1, Text: "- [x] a done:2026-10-05"},
		{Line: 2, Text: "- [ ] b"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupExpandsDatesOnChangedLinesOnlyWhenAsked(t *testing.T) {
	before := []string{"- [ ] a due:today", "- [ ] b"}
	after := []string{"- [ ] a due:today", "- [ ] b due:tomorrow"}

	if got := tasks.Fixup(before, after, false, today); len(got) != 0 {
		t.Errorf("Fixup without expansion = %v, want nothing", got)
	}
	got := tasks.Fixup(before, after, true, today)
	want := []tasks.Fix{{Line: 1, Text: "- [ ] b due:2026-10-06"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupExpandsAndStampsTheSameLine(t *testing.T) {
	before := []string{"- [ ] a"}
	after := []string{"- [x] a due:fri"}

	got := tasks.Fixup(before, after, true, today)

	want := []tasks.Fix{{Line: 0, Text: "- [x] a due:2026-10-09 done:2026-10-05"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupLeavesCheckboxesInFencedCode(t *testing.T) {
	before := []string{"```", "- [ ] a due:today", "```", "- [ ] b"}
	after := []string{"```", "- [x] a due:today", "```", "- [x] b"}

	got := tasks.Fixup(before, after, true, today)

	want := []tasks.Fix{{Line: 3, Text: "- [x] b done:2026-10-05"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestToggleRefusesALineThatIsNotATask(t *testing.T) {
	if got, ok := tasks.Toggle("- a", today); ok || got != "- a" {
		t.Errorf("Toggle = %q, %v; want the line and false", got, ok)
	}
}
