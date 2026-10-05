package index_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
)

func TestParseReadsNameTitleHeadingsAndTasks(t *testing.T) {
	src := "intro line\n" +
		"## Before\n" +
		"# The Title\n" +
		"- [ ] first #a\n" +
		"```go\n" +
		"- [ ] not a task in code\n" +
		"# not a heading\n" +
		"```\n" +
		"  - [x] nested done:2026-10-01\n" +
		"~~~~\n" +
		"- [ ] still code\n" +
		"~~~\n" +
		"- [ ] tilde fence needs four\n" +
		"~~~~\n" +
		"### Third ###\n" +
		"# Second H1\n" +
		"#nospace is a tag line\n"
	n := index.Parse("dir/My Note.md", []byte(src))

	if n.Path != "dir/My Note.md" || n.Name != "My Note" || n.Title != "The Title" {
		t.Errorf("Path/Name/Title = %q/%q/%q", n.Path, n.Name, n.Title)
	}
	wantH := []index.Heading{
		{Level: 2, Text: "Before", Line: 1},
		{Level: 1, Text: "The Title", Line: 2},
		{Level: 3, Text: "Third", Line: 14},
		{Level: 1, Text: "Second H1", Line: 15},
	}
	if !reflect.DeepEqual(n.Headings, wantH) {
		t.Errorf("Headings\n got %+v\nwant %+v", n.Headings, wantH)
	}
	var got []string
	for _, tk := range n.Tasks {
		if tk.Path != "dir/My Note.md" {
			t.Errorf("task path %q", tk.Path)
		}
		got = append(got, fmt.Sprintf("%s@%d", tk.Summary, tk.Line))
	}
	want := []string{"first@3", "nested@8"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tasks = %v, want %v", got, want)
	}
	if n.Contents != src {
		t.Errorf("Contents not kept")
	}
}

func TestParseHandlesCRLF(t *testing.T) {
	n := index.Parse("a.md", []byte("# Title\r\n- [ ] task\r\n"))
	if n.Title != "Title" || len(n.Tasks) != 1 || n.Tasks[0].Text != "task" {
		t.Errorf("got title %q tasks %+v", n.Title, n.Tasks)
	}
}

func TestParseTitleIsEmptyWithoutH1(t *testing.T) {
	n := index.Parse("a.md", []byte("## only h2\n"))
	if n.Title != "" || n.Name != "a" {
		t.Errorf("Title/Name = %q/%q", n.Title, n.Name)
	}
}

func TestParseFlagsSyncConflicts(t *testing.T) {
	if !index.Parse("a/note.sync-conflict-20261001-120000-ABCDEF.md", nil).Conflict {
		t.Error("sync-conflict file not flagged")
	}
	if index.Parse("a/note.md", nil).Conflict {
		t.Error("plain note flagged as conflict")
	}
}
