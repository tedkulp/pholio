package tasks_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/tasks"
)

// editing returns an engine over text with the Task rules hooked in, with
// the cursor on line, column col.
func editing(text string, line, col int) *engine.Engine {
	e := engine.New(text)
	e.Fixup = tasks.Hook(func() time.Time { return today }, func() index.Format { return index.Dataview })
	e.SetCursor(engine.Pos{Line: line, Col: col})
	return e
}

func feed(e *engine.Engine, keys ...string) {
	for _, k := range keys {
		e.Feed(k)
	}
}

func typed(s string) []string { return strings.Split(s, "") }

func text(e *engine.Engine) string { return strings.TrimSuffix(e.Buf.String(), "\n") }

func TestEditorExpandsEachRelativeFormOnLeavingInsertMode(t *testing.T) {
	for word, want := range map[string]string{
		"today":    "2026-10-05",
		"tomorrow": "2026-10-06",
		"thursday": "2026-10-08",
		"thu":      "2026-10-08",
		"monday":   "2026-10-12",
		"+3d":      "2026-10-08",
		"+1w":      "2026-10-12",
	} {
		e := editing("- [ ] call mum", 0, 0)
		feed(e, "A")
		feed(e, typed(" 📅 "+word)...)
		if got := text(e); got != "- [ ] call mum 📅 "+word {
			t.Fatalf("%s: expanded while still typing: %q", word, got)
		}
		feed(e, "esc")
		if got := text(e); got != "- [ ] call mum 📅 "+want {
			t.Errorf("%s: got %q, want 📅 %s", word, got, want)
		}
	}
}

func TestEditorExpandsDataviewDoneToo(t *testing.T) {
	e := editing("- [x] a", 0, 0)
	feed(e, "A")
	feed(e, typed(" [completion:: yesterday]")...)
	feed(e, "esc")

	if got := text(e); got != "- [x] a [completion:: 2026-10-04]" {
		t.Errorf("got %q", got)
	}
}

func TestEditorLeavesRelativeDatesFromNormalCommands(t *testing.T) {
	e := editing("- [ ] a [due:: today]\n- [ ] b", 0, 0)
	feed(e, "y", "y", "j", "p")

	if got := text(e); got != "- [ ] a [due:: today]\n- [ ] b\n- [ ] a [due:: today]" {
		t.Errorf("got %q", got)
	}
}

func TestEditorStampsATaskMarkedDone(t *testing.T) {
	e := editing("# T\n- [ ] a #w", 1, 3)
	feed(e, "r", "x")

	if got := text(e); got != "# T\n- [x] a #w [completion:: 2026-10-05]" {
		t.Errorf("got %q", got)
	}
}

func TestEditorStampsATaskMarkedDoneInInsertMode(t *testing.T) {
	e := editing("- [ ] a", 0, 3)
	feed(e, "s", "X", "esc")

	if got := text(e); got != "- [X] a [completion:: 2026-10-05]" {
		t.Errorf("got %q", got)
	}
}

func TestEditorRemovesTheStampWhenATaskIsReopened(t *testing.T) {
	e := editing("- [x] a [completion:: 2026-10-01] [priority:: high]", 0, 3)
	feed(e, "r", " ")

	if got := text(e); got != "- [ ] a [priority:: high]" {
		t.Errorf("got %q", got)
	}
}

func TestUndoRemovesTheStampWithTheChange(t *testing.T) {
	e := editing("- [ ] a\n- [x] b ✅ 2026-10-01", 0, 3)
	feed(e, "r", "x")
	feed(e, "j", "r", " ")
	if got := text(e); got != "- [x] a [completion:: 2026-10-05]\n- [ ] b" {
		t.Fatalf("after edits: %q", got)
	}

	feed(e, "u")
	if got := text(e); got != "- [x] a [completion:: 2026-10-05]\n- [x] b ✅ 2026-10-01" {
		t.Fatalf("after one u: %q", got)
	}
	feed(e, "u")
	if got := text(e); got != "- [ ] a\n- [x] b ✅ 2026-10-01" {
		t.Errorf("after two u: %q", got)
	}
	feed(e, "ctrl+r")
	if got := text(e); got != "- [x] a [completion:: 2026-10-05]\n- [x] b ✅ 2026-10-01" {
		t.Errorf("after ctrl+r: %q", got)
	}
}

func TestToggleLineInABufferIsOneUndoStep(t *testing.T) {
	e := editing("- [ ] a\n- [x] b ✅ 2026-10-01", 1, 2)

	if !tasks.ToggleLine(e, 0, index.Dataview, today) {
		t.Fatal("ToggleLine refused a Task")
	}
	if got := text(e); got != "- [x] a [completion:: 2026-10-05]\n- [x] b ✅ 2026-10-01" || !e.Dirty {
		t.Fatalf("after toggle: %q dirty %v", got, e.Dirty)
	}
	feed(e, "u")
	if got := text(e); got != "- [ ] a\n- [x] b ✅ 2026-10-01" {
		t.Errorf("after u: %q", got)
	}
	if tasks.ToggleLine(e, 5, index.Dataview, today) {
		t.Error("ToggleLine accepted a line past the end")
	}
}

func TestToggleFileRewritesTheLineOnDisk(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/v/a.md": "# A\r\n- [x] a ✅ 2026-10-01\r\n- [ ] b\r\n"})

	if err := tasks.ToggleFile(fsys, "/v/a.md", 2, index.Dataview, today); err != nil {
		t.Fatal(err)
	}
	if err := tasks.ToggleFile(fsys, "/v/a.md", 1, index.Dataview, today); err != nil {
		t.Fatal(err)
	}

	if got, want := read(t, fsys, "/v/a.md"), "# A\r\n- [ ] a\r\n- [x] b [completion:: 2026-10-05]\r\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

func TestToggleFileRefusesALineThatIsNotATask(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/v/a.md": "# A\n"})

	for _, line := range []int{0, 3} {
		err := tasks.ToggleFile(fsys, "/v/a.md", line, index.Dataview, today)
		if !errors.Is(err, tasks.ErrNotATask) {
			t.Errorf("line %d: err = %v, want ErrNotATask", line, err)
		}
	}
	if err := tasks.ToggleFile(fsys, "/v/missing.md", 0, index.Dataview, today); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: err = %v", err)
	}
	if got := read(t, fsys, "/v/a.md"); got != "# A\n" {
		t.Errorf("file changed: %q", got)
	}
}

func read(t *testing.T, fsys *seamtest.MemFS, name string) string {
	t.Helper()
	b, err := fsys.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
