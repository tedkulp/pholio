package app_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
)

var shiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}

// editVault is the tasks fixture open on edit.md, which holds text.
func editVault(t *testing.T, text string) (app.Model, string) {
	t.Helper()
	return tasksVault(t, "edit.md", func(root string) {
		if err := os.WriteFile(filepath.Join(root, "edit.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	})
}

// taskEditor opens the Task Editor on the cursor line with spc T.
func taskEditor(m app.Model) app.Model { return typeKeys(press(m, space), "T") }

func wantForm(t *testing.T, m app.Model, want ...string) {
	t.Helper()
	if got := m.FormValues(); !reflect.DeepEqual(got, want) {
		t.Errorf("form = %q, want %q", got, want)
	}
}

func TestSpcTOpensTheTaskEditorOnTheCursorLine(t *testing.T) {
	m, _ := editVault(t, "- [/] Pay rent [due:: 2026-10-10] #home 📅 2026-10-11 ⏳ 2026-10-08 🛫 2026-10-01 🔽 ^id\n")

	m = taskEditor(m)

	wantForm(t, m, "Pay rent #home", "in progress", "2026-10-10", "2026-10-08", "2026-10-01", "low")
	for _, s := range []string{"Edit Task", "Description", "Scheduled", "‹ in progress ›"} {
		if !strings.Contains(screen(m), s) {
			t.Errorf("screen lacks %q:\n%s", s, screen(m))
		}
	}
}

func TestSpcTAndTaskOnANonTaskLineSaySo(t *testing.T) {
	m, _ := editVault(t, "plain\n- [ ]\n")

	for _, m := range []app.Model{taskEditor(m), ex(m, "task"), taskEditor(typeKeys(m, "j"))} {
		if m.FormValues() != nil {
			t.Error("opened the Task Editor on a non-Task line")
		}
		if got := messageLine(m); got != "not a Task" {
			t.Errorf("message = %q", got)
		}
	}
}

func TestTaskOpensTheTaskEditor(t *testing.T) {
	m, _ := editVault(t, "- [ ] X\n")

	wantForm(t, ex(m, "task"), "X", "open", "", "", "", "none")
}

func TestSavingPriorityIsOneUndoableBufferEdit(t *testing.T) {
	const text = "- [ ] Pay rent [due:: 2026-10-10] #home\n"
	m, root := editVault(t, text)

	m = press(typeKeys(press(taskEditor(m), shiftTab), "  "), enter)

	if m.FormValues() != nil {
		t.Error("form still open after enter")
	}
	if want := "- [ ] Pay rent #home [due:: 2026-10-10] [priority:: high]\n"; m.Text() != want {
		t.Errorf("buffer = %q, want %q", m.Text(), want)
	}
	if got := readFile(t, filepath.Join(root, "edit.md")); got != text {
		t.Errorf("file written: %q", got)
	}
	if m = typeKeys(m, "u"); m.Text() != text {
		t.Errorf("after u = %q", m.Text())
	}
}

func TestScheduledTomorrowGoesBeforeTheBlockID(t *testing.T) {
	m, _ := editVault(t, "- [ ] X 📅 2026-10-10 ^abc\n")

	m = press(typeKeys(keys(taskEditor(m), tab, tab, tab), "tomorrow"), enter)

	if want := "- [ ] X 📅 2026-10-10 ⏳ 2026-10-06 ^abc\n"; m.Text() != want {
		t.Errorf("buffer = %q, want %q", m.Text(), want)
	}
}

func TestDueTakesRelativeDatesClearsAndRefusesNonDates(t *testing.T) {
	m, _ := editVault(t, "- [ ] X [due:: 2026-10-10]\n")
	due := func(m app.Model, s string) app.Model {
		return press(typeKeys(keys(taskEditor(m), tab, tab, ctrlU), s), enter)
	}

	if m = due(m, "fri"); m.Text() != "- [ ] X [due:: 2026-10-09]\n" {
		t.Errorf("fri: buffer = %q", m.Text())
	}
	if m = due(m, ""); m.Text() != "- [ ] X\n" {
		t.Errorf("cleared: buffer = %q", m.Text())
	}
	m = due(m, "someday")
	if m.Text() != "- [ ] X\n" {
		t.Errorf("someday written: %q", m.Text())
	}
	if m.FormValues() == nil || !m.FormInvalid(2) || m.FormInvalid(0) {
		t.Errorf("someday: form %q should stay open with Due marked", m.FormValues())
	}
}

func TestAnEmptyDescriptionIsRefused(t *testing.T) {
	m, _ := editVault(t, "- [ ] X\n")

	m = press(press(taskEditor(m), ctrlU), enter)

	if m.Text() != "- [ ] X\n" || !m.FormInvalid(0) {
		t.Errorf("buffer = %q, description marked = %v", m.Text(), m.FormInvalid(0))
	}
}

func TestStatusFollowsTheDoneDateRule(t *testing.T) {
	m, _ := editVault(t, "- [ ] X 📅 2026-10-10\n")
	status := func(m app.Model, spaces string) app.Model {
		return press(typeKeys(press(taskEditor(m), tab), spaces), enter)
	}

	if m = status(m, "  "); m.Text() != "- [x] X 📅 2026-10-10 ✅ 2026-10-05\n" {
		t.Errorf("done: buffer = %q", m.Text())
	}
	if m = status(m, "  "); m.Text() != "- [ ] X 📅 2026-10-10\n" {
		t.Errorf("open again: buffer = %q", m.Text())
	}
}

func TestFieldsTheFormDoesNotShowSurvive(t *testing.T) {
	m, _ := editVault(t, "- [x] X [created:: 2026-09-01] 🔁 every week [completion:: 2026-10-01]\n")

	m = press(press(taskEditor(m), shiftTab), space)
	m = press(m, enter)

	if want := "- [x] X 🔁 every week [created:: 2026-09-01] [completion:: 2026-10-01] [priority:: highest]\n"; m.Text() != want {
		t.Errorf("buffer = %q, want %q", m.Text(), want)
	}
}

func TestEscClosesTheTaskEditorWritingNothing(t *testing.T) {
	m, root := editVault(t, "- [ ] X\n")

	m = press(typeKeys(taskEditor(m), "zzz"), esc)

	if m.FormValues() != nil || m.Text() != "- [ ] X\n" || readFile(t, filepath.Join(root, "edit.md")) != "- [ ] X\n" {
		t.Errorf("esc: form %q, buffer %q", m.FormValues(), m.Text())
	}
}

func TestEEditsATaskInAnotherNoteOnDiskAndReturnsToTheList(t *testing.T) {
	m, root := tasksVault(t, "inbox.md", func(root string) {
		text := "# C\r\n- [ ] far away [due:: 2026-10-20]\r\n- [ ] far off\r\n"
		if err := os.WriteFile(filepath.Join(root, "crlf.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	m = press(typeKeys(press(m, space), "t/far"), esc)

	m = typeKeys(m, "e")
	wantForm(t, m, "far away", "open", "2026-10-20", "", "", "none")
	m = press(typeKeys(press(m, shiftTab), "  "), enter)

	want := "# C\r\n- [ ] far away [due:: 2026-10-20] [priority:: high]\r\n- [ ] far off\r\n"
	if got := readFile(t, filepath.Join(root, "crlf.md")); got != want {
		t.Errorf("crlf.md = %q, want %q", got, want)
	}
	if m.Text() != inbox {
		t.Errorf("open buffer changed:\n%s", m.Text())
	}
	if got := m.PaletteSelected(); got != "[ ] far away [due:: 2026-10-20] [priority:: high]" {
		t.Errorf("selected %q, want the edited Task", got)
	}
}

func TestEscFromEGoesBackToTheList(t *testing.T) {
	m, _ := taskList(t, "inbox.md")

	m = typeKeys(m, "je")
	m = press(m, esc)

	if m.FormValues() != nil || len(m.PaletteRows()) != 9 {
		t.Errorf("not back on the list: form %q", m.FormValues())
	}
	if got := m.PaletteSelected(); !strings.HasPrefix(got, "[ ] renew passport") {
		t.Errorf("selected %q", got)
	}
}

func TestAOpensAnEmptyTaskEditorAndAddsTheTask(t *testing.T) {
	m, root := taskList(t, "inbox.md")

	m = typeKeys(m, "a")
	wantForm(t, m, "", "open", "", "", "", "none")
	m = press(typeKeys(keys(typeKeys(m, "ring mum"), tab, tab), "tomorrow"), enter)

	want := "- [-] cancelled trip\n- [ ] ring mum [due:: 2026-10-06]\n"
	if got := readFile(t, filepath.Join(root, "daily", "2026-10-05.md")); !strings.Contains(got, want) {
		t.Errorf("daily Note:\n%s\nwant it to contain\n%s", got, want)
	}
	if got := m.PaletteRows(); !slices.Contains(got, "Upcoming (3) | [ ] ring mum [due:: 2026-10-06] | daily/2026-10-05.md:9") {
		t.Errorf("Task List not back with the new Task:\n%s", lines(got))
	}
}

func TestATakesTheFormatOfTheVault(t *testing.T) {
	m, root := tasksVault(t, "inbox.md", func(root string) {
		if err := os.WriteFile(filepath.Join(root, ".pholio", "config.toml"), []byte("task_format = \"emoji\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	m = typeKeys(press(m, space), "ta")

	press(typeKeys(keys(typeKeys(m, "ring mum"), shiftTab), " "), enter)

	if got := readFile(t, filepath.Join(root, "daily", "2026-10-05.md")); !strings.Contains(got, "- [ ] ring mum 🔺\n") {
		t.Errorf("daily Note:\n%s", got)
	}
}

func TestHelpAndTheTaskListHintListTheTaskEditor(t *testing.T) {
	m := typeKeys(keys(shell(t, shellVault(), "/vault/a.md"), space), "?")
	got := m.PaletteRows()
	for _, want := range []string{" | spc T         edit Task | leader", " | :task         edit Task | ex"} {
		if !slices.Contains(got, want) {
			t.Errorf("help lacks %q; rows:\n%s", want, strings.Join(got, "\n"))
		}
	}

	l, _ := taskList(t, "inbox.md")
	if !strings.Contains(screen(l), "e edit") {
		t.Errorf("Task List hint lacks e:\n%s", screen(l))
	}
}

func TestAClickOnTheBackdropClosesTheTaskEditor(t *testing.T) {
	m, _ := editVault(t, "- [ ] X\n")
	m = taskEditor(m)

	if m = send(m, click(50, 4)); m.FormValues() == nil {
		t.Fatal("a click inside the form closed it")
	}
	if m = send(m, click(50, 25)); m.FormValues() != nil || m.Text() != "- [ ] X\n" {
		t.Errorf("backdrop click: form %q, buffer %q", m.FormValues(), m.Text())
	}
}
