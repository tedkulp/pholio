package index_test

import (
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
)

func TestBothTaskFormatsParseToTheSameMetadata(t *testing.T) {
	for _, line := range []string{
		"- [ ] Pay rent [due:: 2026-10-10] [priority:: high] #home",
		"- [ ] Pay rent (due:: 2026-10-10) (priority:: high) #home",
		"- [ ] Pay rent 📅 2026-10-10 ⏫ #home",
		"- [ ] Pay rent [due:: 2026-10-10] ⏫ #home",
		"- [ ] Pay rent 📅️ 2026-10-10 ⏫️ #home", // with variation selectors
	} {
		task, ok := index.ParseTask(line)
		if !ok {
			t.Fatalf("%q is not a Task", line)
		}
		if task.Due() != "2026-10-10" || task.Pri() != "high" || !reflect.DeepEqual(task.Tags, []string{"home"}) ||
			task.Summary != "Pay rent" {
			t.Errorf("%q: Due %q Pri %q Tags %q Summary %q", line, task.Due(), task.Pri(), task.Tags, task.Summary)
		}
	}
}

func TestEveryDateFieldIsMetadata(t *testing.T) {
	for _, line := range []string{
		"- [x] a [due:: 2026-10-01] [completion:: 2026-10-02] [start:: 2026-10-03] [scheduled:: 2026-10-04] [created:: 2026-10-05] [cancelled:: 2026-10-06]",
		"- [x] a 📅 2026-10-01 ✅ 2026-10-02 🛫 2026-10-03 ⏳ 2026-10-04 ➕ 2026-10-05 ❌ 2026-10-06",
	} {
		task, _ := index.ParseTask(line)
		want := map[string]string{
			"due": "2026-10-01", "completion": "2026-10-02", "start": "2026-10-03",
			"scheduled": "2026-10-04", "created": "2026-10-05", "cancelled": "2026-10-06",
		}
		if !reflect.DeepEqual(task.Meta, want) || task.Summary != "a" || task.DoneOn() != "2026-10-02" {
			t.Errorf("%q: Meta %v Summary %q", line, task.Meta, task.Summary)
		}
	}
}

func TestEachPriorityMapsToItsLevel(t *testing.T) {
	for line, want := range map[string]string{
		"- [ ] a 🔺":                    "highest",
		"- [ ] a ⏫":                    "high",
		"- [ ] a 🔼":                    "medium",
		"- [ ] a 🔽":                    "low",
		"- [ ] a ⏬":                    "lowest",
		"- [ ] a [priority:: highest]": "highest",
		"- [ ] a [priority:: High]":    "high",
		"- [ ] a [priority:: medium]":  "medium",
		"- [ ] a [priority:: low]":     "low",
		"- [ ] a [priority:: lowest]":  "lowest",
		"- [ ] a [priority:: urgent]":  "",
		"- [ ] a":                      "",
	} {
		task, _ := index.ParseTask(line)
		if task.Pri() != want {
			t.Errorf("%q: Pri %q, want %q", line, task.Pri(), want)
		}
	}
}

func TestKeyValueWordsAreNoLongerMetadata(t *testing.T) {
	task, _ := index.ParseTask("- [ ] Pay rent due:2026-10-10 pri:high done:2026-10-01")
	if task.Due() != "" || task.Pri() != "" || task.DoneOn() != "" || task.Meta != nil {
		t.Errorf("key:value read as metadata: %+v", task)
	}
	if task.Summary != "Pay rent due:2026-10-10 pri:high done:2026-10-01" {
		t.Errorf("Summary %q", task.Summary)
	}
}

func TestOtherBracketsAndEmojiStayText(t *testing.T) {
	for _, line := range []string{
		"- [ ] see [the docs](https://x.io) and [[a link]]",
		"- [ ] [owner:: ted] unknown key",
		"- [ ] [due:: 2026-10-10) mismatched",
		"- [ ] 🔁 every week 🏁 delete 🆔 abc ⛔ def",
		"- [ ] 1 ➕ more ❌ none ⏳ later 🛫 now", // only due and done take relative dates
		"- [ ] 📅 ✅",
		"- [ ] review ✅ checklist with team", // a word that isn't a date
		"- [ ] plan 📅 meeting agenda",
	} {
		task, _ := index.ParseTask(line)
		if task.Meta != nil || task.Pri() != "" {
			t.Errorf("%q: Meta %v Pri %q", line, task.Meta, task.Pri())
		}
	}
}

func TestFieldsReportsEachFieldsSpanAndFormat(t *testing.T) {
	line := "- [ ] a [due:: tomorrow] ⏫ ✅ 2026-10-02 x"
	got := index.Fields(line)
	want := []index.Field{
		{Key: "due", Value: "tomorrow", Format: index.Dataview, Start: 8, End: 24, ValStart: 15, ValEnd: 23},
		{Key: "priority", Value: "high", Format: index.Emoji, Start: 25, End: 28, ValStart: 28, ValEnd: 28},
		{Key: "completion", Value: "2026-10-02", Format: index.Emoji, Start: 29, End: 43, ValStart: 33, ValEnd: 43},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fields\n got %+v\nwant %+v", got, want)
	}
}
