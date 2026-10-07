package index_test

import (
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
)

func TestParseTaskReadsStatusTextMetadataAndTags(t *testing.T) {
	tests := []struct {
		line string
		want index.Task
	}{
		{"- [ ] buy milk", index.Task{Status: index.Open, Mark: ' ', Text: "buy milk", Summary: "buy milk"}},
		{"  * [/] half done", index.Task{Status: index.InProgress, Mark: '/', Text: "half done", Summary: "half done", Indent: 2}},
		{"+ [x] shipped [completion:: 2026-10-01]", index.Task{Status: index.Done, Mark: 'x', Text: "shipped [completion:: 2026-10-01]", Summary: "shipped",
			Meta: map[string]string{"completion": "2026-10-01"}}},
		{"1. [X] numbered", index.Task{Status: index.Done, Mark: 'X', Text: "numbered", Summary: "numbered"}},
		{"- [-] dropped", index.Task{Status: index.Cancelled, Mark: '-', Text: "dropped", Summary: "dropped"}},
		{"- [?] odd mark", index.Task{Status: index.Open, Mark: '?', Text: "odd mark", Summary: "odd mark"}},
		{"\t- [ ] call bob [due:: 2026-10-10] ⏫ #work #home/ops see https://x.io", index.Task{
			Status: index.Open, Mark: ' ', Indent: 1,
			Text:    "call bob [due:: 2026-10-10] ⏫ #work #home/ops see https://x.io",
			Summary: "call bob see https://x.io",
			Meta:    map[string]string{"due": "2026-10-10", "priority": "high"},
			Tags:    []string{"work", "home/ops"},
		}},
		{"- [ ]", index.Task{Status: index.Open, Mark: ' '}},
	}
	for _, tt := range tests {
		got, ok := index.ParseTask(tt.line)
		if !ok {
			t.Errorf("ParseTask(%q) not a Task", tt.line)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseTask(%q)\n got %+v\nwant %+v", tt.line, got, tt.want)
		}
	}
}

func TestParseTaskRejectsNonTasks(t *testing.T) {
	for _, line := range []string{
		"plain text",
		"- plain item",
		"-[ ] no space",
		"- [ ]no space after",
		"- [xx] two marks",
		"[ ] no marker",
		"# - [ ] heading",
	} {
		if _, ok := index.ParseTask(line); ok {
			t.Errorf("ParseTask(%q) = Task, want not a Task", line)
		}
	}
}
