package tasks_test

import (
	"testing"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/tasks"
)

func TestReadSplitsALineIntoTheFormsValues(t *testing.T) {
	for _, tc := range []struct {
		line string
		want tasks.Values
	}{
		{"- [ ] Pay rent [due:: 2026-10-10] #home", tasks.Values{Description: "Pay rent #home", Due: "2026-10-10"}},
		{"  * [/] X 📅 2026-10-10 ⏳ 2026-10-08 🛫 2026-10-01 ⏫ ^abc", tasks.Values{
			Description: "X", Status: index.InProgress, Due: "2026-10-10", Scheduled: "2026-10-08", Start: "2026-10-01", Priority: "high"}},
		{"- [x] [priority:: Low] done thing [completion:: 2026-10-01]", tasks.Values{Description: "done thing", Status: index.Done, Priority: "low"}},
		{"- [-] dropped 🔁 every week", tasks.Values{Description: "dropped 🔁 every week", Status: index.Cancelled}},
	} {
		got, ok := tasks.Read(tc.line)
		if !ok || got != tc.want {
			t.Errorf("Read(%q) = %+v, %v; want %+v", tc.line, got, ok, tc.want)
		}
	}
}

func TestReadRefusesNonTasks(t *testing.T) {
	for _, line := range []string{"plain", "- [ ]", "- [ ]   ", "# heading"} {
		if _, ok := tasks.Read(line); ok {
			t.Errorf("Read(%q) ok", line)
		}
	}
}

func TestRewriteRebuildsTheLine(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		edit       func(*tasks.Values)
		def        index.Format
		want       string
	}{
		{"unchanged keeps the line", "- [ ] Pay rent [due:: 2026-10-10] #home", func(*tasks.Values) {}, index.Dataview,
			"- [ ] Pay rent #home [due:: 2026-10-10]"},
		{"new priority appended in the line's format", "- [ ] Pay rent [due:: 2026-10-10] #home",
			func(v *tasks.Values) { v.Priority = "high" }, index.Emoji,
			"- [ ] Pay rent #home [due:: 2026-10-10] [priority:: high]"},
		{"new emoji date before the block ID", "- [ ] X 📅 2026-10-10 ^abc",
			func(v *tasks.Values) { v.Scheduled = "2026-10-06" }, index.Dataview,
			"- [ ] X 📅 2026-10-10 ⏳ 2026-10-06 ^abc"},
		{"no fields uses the default format", "- [ ] X",
			func(v *tasks.Values) { v.Due, v.Start, v.Priority = "2026-10-10", "2026-10-01", "highest" }, index.Emoji,
			"- [ ] X 📅 2026-10-10 🛫 2026-10-01 🔺"},
		{"edited values replaced in place", "- [ ] X [due:: 2026-10-10] [priority:: low] (start:: 2026-10-01)",
			func(v *tasks.Values) { v.Due, v.Priority, v.Start = "2026-11-11", "medium", "2026-10-02" }, index.Dataview,
			"- [ ] X [due:: 2026-11-11] [priority:: medium] (start:: 2026-10-02)"},
		{"emoji priority rewritten as its emoji", "- [ ] X ⏬ 📅 2026-10-10",
			func(v *tasks.Values) { v.Priority = "highest" }, index.Dataview,
			"- [ ] X 🔺 📅 2026-10-10"},
		{"cleared fields dropped", "- [ ] X [due:: 2026-10-10] 🔼 [created:: 2026-10-01]",
			func(v *tasks.Values) { v.Due, v.Priority = "", "" }, index.Dataview,
			"- [ ] X [created:: 2026-10-01]"},
		{"a mid-text field moves after the description", "- [ ] call [due:: 2026-10-10] mum #fam",
			func(*tasks.Values) {}, index.Dataview,
			"- [ ] call mum #fam [due:: 2026-10-10]"},
		{"description replaced, indent and marker kept", "   1. [ ] old 📅 2026-10-10",
			func(v *tasks.Values) { v.Description = "new #tag" }, index.Dataview,
			"   1. [ ] new #tag 📅 2026-10-10"},
		{"in progress", "- [ ] X", func(v *tasks.Values) { v.Status = index.InProgress }, index.Dataview, "- [/] X"},
		{"cancelled", "- [ ] X", func(v *tasks.Values) { v.Status = index.Cancelled }, index.Dataview, "- [-] X"},
		{"done stamps today in the line's format", "- [ ] X 📅 2026-10-10 ^id",
			func(v *tasks.Values) { v.Status = index.Done }, index.Dataview,
			"- [x] X 📅 2026-10-10 ✅ 2026-10-05 ^id"},
		{"done stamps today in the default format", "- [ ] X",
			func(v *tasks.Values) { v.Status = index.Done }, index.Dataview,
			"- [x] X [completion:: 2026-10-05]"},
		{"reopening removes the done date", "- [x] X [completion:: 2026-10-01] [created:: 2026-09-01]",
			func(v *tasks.Values) { v.Status = index.Open }, index.Dataview,
			"- [ ] X [created:: 2026-09-01]"},
		{"a done Task left done keeps its done date", "- [X] X ✅ 2026-10-01 🔁 every week",
			func(v *tasks.Values) { v.Priority = "low" }, index.Dataview,
			"- [X] X 🔁 every week ✅ 2026-10-01 🔽"},
		{"duplicate edited keys collapse to one", "- [ ] X [due:: 2026-10-10] [due:: 2026-10-11]",
			func(v *tasks.Values) { v.Due = "2026-10-12" }, index.Dataview,
			"- [ ] X [due:: 2026-10-12]"},
		{"an empty checkbox becomes a new Task", "- [ ]",
			func(v *tasks.Values) { v.Description, v.Due = "ring mum", "2026-10-06" }, index.Dataview,
			"- [ ] ring mum [due:: 2026-10-06]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := tasks.Read(tc.line)
			if !ok {
				v = tasks.Values{}
			}
			tc.edit(&v)
			got, ok := tasks.Rewrite(tc.line, v, tc.def, today)
			if !ok || got != tc.want {
				t.Errorf("Rewrite(%q) = %q, %v\nwant %q", tc.line, got, ok, tc.want)
			}
		})
	}
}

func TestRewriteRefusesNonTasks(t *testing.T) {
	if got, ok := tasks.Rewrite("not a task", tasks.Values{Description: "x"}, index.Dataview, today); ok || got != "not a task" {
		t.Errorf("Rewrite = %q, %v", got, ok)
	}
}

func TestRewriteLeavesARelativeDoneDateAlone(t *testing.T) {
	line := "- [x] X [completion:: today]"
	v, _ := tasks.Read(line)
	v.Priority = "low"
	if got, _ := tasks.Rewrite(line, v, index.Dataview, today); got != "- [x] X [completion:: today] [priority:: low]" {
		t.Errorf("Rewrite = %q", got)
	}
}
