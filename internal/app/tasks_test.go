package app_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/app"
)

// beforeDawn is 3:30am on Tuesday 2026-10-06. The daily fixture's day starts
// at 04:00, so Today is still Monday 2026-10-05.
var beforeDawn = time.Date(2026, 10, 6, 3, 30, 0, 0, time.Local)

// taskVault is the daily fixture with todo.md and other.md added.
func taskVault(t *testing.T) (app.Model, string) {
	t.Helper()
	vault, d := dailyVault(t)
	for name, text := range map[string]string{
		"todo.md":  "- [ ] a\n- [x] b done:2026-10-01\n",
		"other.md": "# Other\n- [ ] c\n",
	} {
		if err := os.WriteFile(filepath.Join(vault, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return start(t, vault, d, filepath.Join(vault, "todo.md"), beforeDawn), vault
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMarkingATaskDoneStampsItWithToday(t *testing.T) {
	m, _ := taskVault(t)

	m = typeKeys(m, "3lrx")

	if want := "- [x] a done:2026-10-05\n- [x] b done:2026-10-01\n"; m.Text() != want {
		t.Errorf("buffer %q, want %q", m.Text(), want)
	}
	m = typeKeys(m, "u")
	if want := "- [ ] a\n- [x] b done:2026-10-01\n"; m.Text() != want {
		t.Errorf("after u: %q, want %q", m.Text(), want)
	}
}

func TestReopeningATaskRemovesItsStamp(t *testing.T) {
	m, _ := taskVault(t)

	m = typeKeys(m, "j3lr ")

	if want := "- [ ] a\n- [ ] b\n"; m.Text() != want {
		t.Errorf("buffer %q, want %q", m.Text(), want)
	}
}

func TestLeavingInsertModeExpandsRelativeDates(t *testing.T) {
	m, _ := taskVault(t)

	m = typeKeys(m, "A due:tomorrow")
	m = press(m, esc)

	if want := "- [ ] a due:2026-10-06\n- [x] b done:2026-10-01\n"; m.Text() != want {
		t.Errorf("buffer %q, want %q", m.Text(), want)
	}
}

func TestToggleTaskInTheOpenNoteEditsTheBufferOnly(t *testing.T) {
	m, vault := taskVault(t)
	path := filepath.Join(vault, "todo.md")

	m, err := m.ToggleTask(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	if want := "- [x] a done:2026-10-05\n- [x] b done:2026-10-01\n"; m.Text() != want {
		t.Errorf("buffer %q, want %q", m.Text(), want)
	}
	if got := readFile(t, path); got != "- [ ] a\n- [x] b done:2026-10-01\n" {
		t.Errorf("file written: %q", got)
	}
	m = typeKeys(m, "u")
	if want := "- [ ] a\n- [x] b done:2026-10-01\n"; m.Text() != want {
		t.Errorf("after u: %q, want %q", m.Text(), want)
	}
}

func TestToggleTaskInAnotherNoteWritesItsFile(t *testing.T) {
	m, vault := taskVault(t)
	path := filepath.Join(vault, "other.md")

	m, err := m.ToggleTask(path, 1)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := readFile(t, path), "# Other\n- [x] c done:2026-10-05\n"; got != want {
		t.Errorf("file %q, want %q", got, want)
	}
	if want := "- [ ] a\n- [x] b done:2026-10-01\n"; m.Text() != want {
		t.Errorf("open buffer changed: %q", m.Text())
	}
	if _, err := m.ToggleTask(path, 0); err == nil {
		t.Error("toggling a heading succeeded")
	}
}
