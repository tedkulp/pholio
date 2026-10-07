package index_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/testutil"
)

func taskRows(tasks []index.Task) []string {
	var rows []string
	for _, t := range tasks {
		rows = append(rows, fmt.Sprintf("%s:%d [%c] %s", t.Path, t.Line, t.Mark, t.Summary))
	}
	return rows
}

func TestTasksGathersEveryTaskInTheVault(t *testing.T) {
	ix, _ := scanned(t, "tasks")
	want := []string{
		"daily/2026-10-05.md:4 [ ] call the bank",
		"daily/2026-10-05.md:5 [x] water plants",
		"daily/2026-10-05.md:6 [/] write report",
		"daily/2026-10-05.md:7 [-] cancelled trip",
		"projects/garden.md:2 [ ] plan beds owner:ted",
		"projects/garden.md:3 [ ] buy seeds",
		"projects/garden.md:4 [X] pick tomato variety",
	}
	got := ix.Tasks()
	if !reflect.DeepEqual(taskRows(got), want) {
		t.Errorf("Tasks()\n got %q\nwant %q", taskRows(got), want)
	}
	first := got[0]
	if first.Status != index.Open || first.Meta["due"] != "2026-10-05" || first.Pri() != "high" ||
		!reflect.DeepEqual(first.Tags, []string{"money"}) {
		t.Errorf("first Task = %+v", first)
	}
	if got[2].Status != index.InProgress || !got[2].Status.IsOpen() || got[3].Status != index.Cancelled {
		t.Errorf("statuses = %v, %v", got[2].Status, got[3].Status)
	}
}

func TestTasksHonoursTheTemplatesDirOption(t *testing.T) {
	root := testutil.CopyVault(t, "tasks")
	ix := index.New(seam.OSFS{}, root, index.WithTemplatesDir("projects"))
	if err := ix.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, task := range ix.Tasks() {
		if task.Path == "projects/garden.md" {
			t.Fatalf("Task from templates dir listed: %+v", task)
		}
	}
	if len(ix.Tasks()) != 4 {
		t.Errorf("Tasks() = %q, want templates/ left out too", taskRows(ix.Tasks()))
	}
}

func TestSetTemplatesDirChangesTheExtraExcludedFolder(t *testing.T) {
	ix, _ := scanned(t, "tasks")
	ix.SetTemplatesDir("projects")
	for _, task := range ix.Tasks() {
		if task.Path == "projects/garden.md" || task.Path == "templates/daily.md" {
			t.Fatalf("excluded Task listed: %+v", task)
		}
	}
	ix.SetTemplatesDir(".") // a daily_template at the Vault root
	if got := len(ix.Tasks()); got != 7 {
		t.Errorf("Tasks() = %q, want all but templates/", taskRows(ix.Tasks()))
	}
}
