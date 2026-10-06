package index_test

import (
	"testing"

	"github.com/tedkulp/pholio/internal/index"
)

func TestTaskMetadataAccessors(t *testing.T) {
	task, ok := index.ParseTask("- [x] ship due:2026-10-07 pri:HIGH done:2026-10-05")
	if !ok {
		t.Fatal("not a Task")
	}
	if task.Due() != "2026-10-07" || task.Pri() != "high" || task.DoneOn() != "2026-10-05" {
		t.Errorf("Due %q Pri %q DoneOn %q", task.Due(), task.Pri(), task.DoneOn())
	}

	bare, _ := index.ParseTask("- [ ] nothing")
	if bare.Due() != "" || bare.Pri() != "" || bare.DoneOn() != "" {
		t.Errorf("bare Task has metadata: %+v", bare)
	}
}
