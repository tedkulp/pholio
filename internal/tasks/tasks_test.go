package tasks_test

import (
	"slices"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/tasks"
)

// today is Monday 2026-10-05.
var today = time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local)

const dv, emoji = index.Dataview, index.Emoji

func TestExpandRewritesRelativeDueAndDoneToISO(t *testing.T) {
	for in, want := range map[string]string{
		"- [ ] a [due:: today]":            "- [ ] a [due:: 2026-10-05]",
		"- [ ] a [due:: tomorrow]":         "- [ ] a [due:: 2026-10-06]",
		"- [ ] a [due:: friday] #x":        "- [ ] a [due:: 2026-10-09] #x",
		"- [ ] a (due:: fri)":              "- [ ] a (due:: 2026-10-09)",
		"- [ ] a [due::monday]":            "- [ ] a [due::2026-10-12]",
		"- [ ] a [due:: +3d]":              "- [ ] a [due:: 2026-10-08]",
		"- [x] a [completion:: yesterday]": "- [x] a [completion:: 2026-10-04]",
		"- [ ] a 📅 tomorrow":               "- [ ] a 📅 2026-10-06",
		"- [ ] a 📅 +2w ⏫":                  "- [ ] a 📅 2026-10-19 ⏫",
		"- [x] a ✅ yesterday":              "- [x] a ✅ 2026-10-04",
		"  * [ ] [due:: Tomorrow]  📅 fri":  "  * [ ] [due:: 2026-10-06]  📅 2026-10-09",
	} {
		if got := tasks.Expand(in, today); got != want {
			t.Errorf("Expand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExpandLeavesOtherTextAlone(t *testing.T) {
	for _, in := range []string{
		"- [ ] a [due:: 2026-10-05]", // already ISO
		"- [ ] a [due:: someday]",    // not a date
		"- [ ] a [start:: tomorrow]", // not due or done
		"- [ ] a ⏳ tomorrow",         // not due or done
		"- [ ] a due:tomorrow",       // not Task Metadata
		"- a [due:: tomorrow]",       // not a Task
		"📅 tomorrow",                 // not a Task
	} {
		if got := tasks.Expand(in, today); got != in {
			t.Errorf("Expand(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestStampAddsTheDoneDateWhenATaskBecomesDone(t *testing.T) {
	for _, c := range []struct {
		old, new string
		def      index.Format
		want     string
	}{
		{"- [ ] a", "- [x] a", dv, "- [x] a [completion:: 2026-10-05]"},
		{"- [ ] a", "- [x] a", emoji, "- [x] a ✅ 2026-10-05"},
		{"- [/] a #w", "- [X] a #w", dv, "- [X] a #w [completion:: 2026-10-05]"},
		{"- [-] a ", "- [x] a ", dv, "- [x] a [completion:: 2026-10-05]"},
		{"- a", "- [x] a", dv, "- [x] a [completion:: 2026-10-05]"},
		{"- [ ]", "- [x]", dv, "- [x] [completion:: 2026-10-05]"},
		{"- [x]", "- [x]", dv, "- [x]"},
		{"", "- [x]", dv, "- [x] [completion:: 2026-10-05]"},
		{"- [ ] a [completion:: 2026-01-01]", "- [x] a [completion:: 2026-01-01]", dv, "- [x] a [completion:: 2026-10-05]"},
		// the line's first field picks the format
		{"- [ ] X 📅 2026-10-10", "- [x] X 📅 2026-10-10", dv, "- [x] X 📅 2026-10-10 ✅ 2026-10-05"},
		{"- [ ] X [due:: 2026-10-10]", "- [x] X [due:: 2026-10-10]", emoji, "- [x] X [due:: 2026-10-10] [completion:: 2026-10-05]"},
		{"- [ ] X ⏫ [due:: 2026-10-10]", "- [x] X ⏫ [due:: 2026-10-10]", dv, "- [x] X ⏫ [due:: 2026-10-10] ✅ 2026-10-05"},
		// before a trailing block ID
		{"- [ ] X ^abc123", "- [x] X ^abc123", dv, "- [x] X [completion:: 2026-10-05] ^abc123"},
		{"- [ ] X ✅ 2026-01-01 ^ab-1 ", "- [x] X ✅ 2026-01-01 ^ab-1 ", dv, "- [x] X ✅ 2026-10-05 ^ab-1"},
		{"- [ ] X ^abc123 more", "- [x] X ^abc123 more", dv, "- [x] X ^abc123 more [completion:: 2026-10-05]"},
	} {
		if got := tasks.Stamp(c.old, c.new, c.def, today); got != c.want {
			t.Errorf("Stamp(%q, %q, %s) = %q, want %q", c.old, c.new, c.def, got, c.want)
		}
	}
}

func TestStampRemovesTheDoneDateWhenATaskStopsBeingDone(t *testing.T) {
	for _, c := range []struct{ old, new, want string }{
		{"- [x] a [completion:: 2026-10-01]", "- [ ] a [completion:: 2026-10-01]", "- [ ] a"},
		{"- [x] a ✅ 2026-10-01", "- [ ] a ✅ 2026-10-01", "- [ ] a"},
		{"- [x] a (completion:: 2026-10-01) #w", "- [-] a (completion:: 2026-10-01) #w", "- [-] a #w"},
		{"- [X] a  ✅ 2026-10-01 ^id", "- [/] a  ✅ 2026-10-01 ^id", "- [/] a ^id"},
		{"- [x] a [completion:: 2026-10-01]", "- a [completion:: 2026-10-01]", "- a [completion:: 2026-10-01]"}, // no longer a Task
	} {
		if got := tasks.Stamp(c.old, c.new, dv, today); got != c.want {
			t.Errorf("Stamp(%q, %q) = %q, want %q", c.old, c.new, got, c.want)
		}
	}
}

func TestStampLeavesLinesWhoseStatusDidNotChange(t *testing.T) {
	for _, c := range []struct{ old, new string }{
		{"- [x] a", "- [x] ab"},
		{"- [x] a", "- [X] a"},
		{"- [ ] a [completion:: 2026-10-01]", "- [ ] ab [completion:: 2026-10-01]"},
		{"- [ ] a", "- [/] a"},
		{"plain", "plain text"},
	} {
		if got := tasks.Stamp(c.old, c.new, dv, today); got != c.new {
			t.Errorf("Stamp(%q, %q) = %q, want it unchanged", c.old, c.new, got)
		}
	}
}

func TestToggleChecksAnOpenTaskAndUnchecksADoneOne(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"- [ ] a", "- [x] a [completion:: 2026-10-05]"},
		{"  1. [/] a #w", "  1. [x] a #w [completion:: 2026-10-05]"},
		{"* [-] a", "* [x] a [completion:: 2026-10-05]"},
		{"- [x] a [completion:: 2026-10-01]", "- [ ] a"},
		{"- [X] a ✅ 2026-10-01 ⏫", "- [ ] a ⏫"},
		{"- [ж] a", "- [x] a [completion:: 2026-10-05]"},
		{"- [ ]", "- [x] [completion:: 2026-10-05]"},
		{"- [ ] review ✅ checklist", "- [x] review ✅ checklist [completion:: 2026-10-05]"},
		{"- [x] [completion:: 2026-10-01]", "- [ ]"},
	} {
		got, ok := tasks.Toggle(c.in, dv, today)
		if !ok || got != c.want {
			t.Errorf("Toggle(%q) = %q, %v; want %q, true", c.in, got, ok, c.want)
		}
	}
}

func TestToggleWritesTheDefaultFormat(t *testing.T) {
	if got, _ := tasks.Toggle("- [ ] a", emoji, today); got != "- [x] a ✅ 2026-10-05" {
		t.Errorf("Toggle = %q", got)
	}
}

func TestFixupStampsTheTaskWhoseStatusChanged(t *testing.T) {
	before := []string{"# T", "- [x] old", "- [ ] a", "- [ ] b"}
	after := []string{"# T", "- [x] old", "- [x] a", "- [ ] b"}

	got := tasks.Fixup(before, after, false, dv, today)

	want := []tasks.Fix{{Line: 2, Text: "- [x] a [completion:: 2026-10-05]"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupFollowsTasksWhenLinesWereAddedOrRemoved(t *testing.T) {
	before := []string{"- [ ] a", "- [x] b ✅ 2026-10-01", "- [ ] c"}
	after := []string{"- [ ] new", "- [x] a", "- [ ] b ✅ 2026-10-01"}

	got := tasks.Fixup(before, after, false, dv, today)

	want := []tasks.Fix{
		{Line: 1, Text: "- [x] a [completion:: 2026-10-05]"},
		{Line: 2, Text: "- [ ] b"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupExpandsDatesOnChangedLinesOnlyWhenAsked(t *testing.T) {
	before := []string{"- [ ] a [due:: today]", "- [ ] b"}
	after := []string{"- [ ] a [due:: today]", "- [ ] b 📅 tomorrow"}

	if got := tasks.Fixup(before, after, false, dv, today); len(got) != 0 {
		t.Errorf("Fixup without expansion = %v, want nothing", got)
	}
	got := tasks.Fixup(before, after, true, dv, today)
	want := []tasks.Fix{{Line: 1, Text: "- [ ] b 📅 2026-10-06"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupExpandsAndStampsTheSameLine(t *testing.T) {
	before := []string{"- [ ] a"}
	after := []string{"- [x] a [due:: fri]"}

	got := tasks.Fixup(before, after, true, emoji, today)

	want := []tasks.Fix{{Line: 0, Text: "- [x] a [due:: 2026-10-09] [completion:: 2026-10-05]"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestFixupLeavesCheckboxesInFencedCode(t *testing.T) {
	before := []string{"```", "- [ ] a [due:: today]", "```", "- [ ] b"}
	after := []string{"```", "- [x] a [due:: today]", "```", "- [x] b"}

	got := tasks.Fixup(before, after, true, dv, today)

	want := []tasks.Fix{{Line: 3, Text: "- [x] b [completion:: 2026-10-05]"}}
	if !slices.Equal(got, want) {
		t.Errorf("Fixup = %v, want %v", got, want)
	}
}

func TestToggleRefusesALineThatIsNotATask(t *testing.T) {
	if got, ok := tasks.Toggle("- a", dv, today); ok || got != "- a" {
		t.Errorf("Toggle = %q, %v; want the line and false", got, ok)
	}
}
