package app_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

// taskDay is 10am on Monday 2026-10-05, the day of the tasks fixture's
// Daily Note.
var taskDay = time.Date(2026, 10, 5, 10, 0, 0, 0, time.Local)

// inbox adds overdue, upcoming and undated Tasks to the tasks fixture.
const inbox = `# Inbox
- [ ] renew passport [due:: 2026-10-01]
- [ ] pay rent [due:: 2026-10-01] [priority:: high]
- [ ] book dentist 📅 2026-10-09 🔽
- [ ] fix bike [priority:: medium]
- [x] old chore [completion:: 2026-10-01]
`

// taskList opens file (Vault-relative) in a copy of the tasks fixture with
// inbox.md added, scans the index at taskDay and opens the Task List.
func taskList(t *testing.T, file string) (app.Model, string) {
	t.Helper()
	m, root := tasksVault(t, file)
	return typeKeys(press(m, space), "t"), root
}

// tasksVault is the tasks fixture with inbox.md added, after prep, open on
// file with its index scanned at taskDay.
func tasksVault(t *testing.T, file string, prep ...func(root string)) (app.Model, string) {
	t.Helper()
	m, root := unscanned(t, file, prep...)
	return drive(t, m, m.Init()), root
}

func unscanned(t *testing.T, file string, prep ...func(root string)) (app.Model, string) {
	t.Helper()
	root := testutil.CopyVault(t, "tasks")
	if err := os.WriteFile(filepath.Join(root, "inbox.md"), []byte(inbox), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".pholio"), 0o755); err != nil { // marks the Vault
		t.Fatal(err)
	}
	for _, f := range prep {
		f(root)
	}
	fsys := seam.OSFS{}
	ix := index.New(fsys, root)
	path := filepath.Join(root, filepath.FromSlash(file))
	s, err := config.Startup(fsys, dirs, "/home/u", path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Index: ix, Clock: seamtest.NewClock(taskDay)}, path)
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 100, 30), root
}

func wantRows(t *testing.T, m app.Model, want []string) {
	t.Helper()
	if got := m.PaletteRows(); !reflect.DeepEqual(got, want) {
		t.Errorf("rows:\n%s\nwant:\n%s", lines(got), lines(want))
	}
}

func lines(rows []string) string {
	s := ""
	for _, r := range rows {
		s += "  " + r + "\n"
	}
	return s
}

func TestTaskListGroupsAndSortsTasks(t *testing.T) {
	m, _ := taskList(t, "inbox.md")

	wantRows(t, m, []string{
		"Overdue (2) | [ ] pay rent [due:: 2026-10-01] [priority:: high] | inbox.md:3",
		"Overdue (2) | [ ] renew passport [due:: 2026-10-01] | inbox.md:2",
		"Today (1) | [ ] call the bank [due:: 2026-10-05] [priority:: high] #money | daily/2026-10-05.md:5",
		"Upcoming (2) | [/] write report 📅 2026-10-07 #work | daily/2026-10-05.md:7",
		"Upcoming (2) | [ ] book dentist 📅 2026-10-09 🔽 | inbox.md:4",
		"No date (3) | [ ] fix bike [priority:: medium] | inbox.md:5",
		"No date (3) | [ ] plan beds owner:ted | projects/garden.md:3",
		"No date (3) | [ ] buy seeds #shopping | projects/garden.md:4",
		"Done today (1) | [x] water plants ✅ 2026-10-05 | daily/2026-10-05.md:6",
	})
}

func TestTaskListSortsAndColoursByObsidianPriority(t *testing.T) {
	m, _ := tasksVault(t, "inbox.md", func(root string) {
		text := "- [ ] p-low [due:: 2026-10-20] 🔽\n" +
			"- [ ] p-none [due:: 2026-10-20]\n" +
			"- [ ] p-highest 📅 2026-10-20 🔺\n" +
			"- [ ] p-lowest [due:: 2026-10-20] [priority:: lowest]\n" +
			"- [ ] p-medium 📅 2026-10-20 🔼\n" +
			"- [ ] p-high [due:: 2026-10-20] [priority:: high]\n"
		if err := os.WriteFile(filepath.Join(root, "prio.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	m = typeKeys(press(m, space), "t/p-")

	var got []string
	for _, r := range m.PaletteRows() {
		got = append(got, strings.Fields(strings.Split(r, " | ")[1])[2])
	}
	if want := []string{"p-highest", "p-high", "p-medium", "p-none", "p-low", "p-lowest"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order = %q, want %q", got, want)
	}
	want := []string{"tasks.pri_high", "tasks.pri_high", "tasks.upcoming", "tasks.upcoming", "tasks.upcoming", "tasks.upcoming"}
	if got := m.PaletteSlots(); !reflect.DeepEqual(got, want) {
		t.Errorf("slots = %q, want %q", got, want)
	}
}

func TestSpaceTogglesATaskInTheOpenNotesBuffer(t *testing.T) {
	m, root := taskList(t, "inbox.md")

	m = typeKeys(m, " ")

	if got := m.PaletteRows(); len(got) != 9 ||
		got[0] != "Overdue (1) | [ ] renew passport [due:: 2026-10-01] | inbox.md:2" ||
		got[7] != "Done today (2) | [x] pay rent [due:: 2026-10-01] [priority:: high] [completion:: 2026-10-05] | inbox.md:3" {
		t.Errorf("rows after toggle:\n%s", lines(got))
	}
	if got := readFile(t, filepath.Join(root, "inbox.md")); got != inbox {
		t.Errorf("inbox.md written:\n%s", got)
	}
	m = typeKeys(press(m, esc), "u")
	if m.Text() != inbox {
		t.Errorf("after esc u, buffer:\n%s", m.Text())
	}
}

func TestSpaceTogglesATaskInAnotherNoteOnDisk(t *testing.T) {
	m, root := taskList(t, "inbox.md")

	m = typeKeys(m, "jj ")

	daily := readFile(t, filepath.Join(root, "daily", "2026-10-05.md"))
	if !strings.Contains(daily, "- [x] call the bank [due:: 2026-10-05] [priority:: high] #money [completion:: 2026-10-05]\n") {
		t.Errorf("daily Note:\n%s", daily)
	}
	if m.Text() != inbox {
		t.Errorf("open buffer changed:\n%s", m.Text())
	}
	if got := m.PaletteSelected(); !strings.HasPrefix(got, "[/] write report") {
		t.Errorf("selected %q, want the next Task in the toggled one's row", got)
	}
	// Toggling it back, from Done today, reopens it.
	m = typeKeys(m, "jjjjj")
	if rows := m.PaletteRows(); !strings.Contains(rows[7], "call the bank") {
		t.Fatalf("row 7 should be the done Task:\n%s", lines(rows))
	}
	typeKeys(m, " ")
	daily = readFile(t, filepath.Join(root, "daily", "2026-10-05.md"))
	if !strings.Contains(daily, "- [ ] call the bank [due:: 2026-10-05] [priority:: high] #money\n") {
		t.Errorf("daily Note after toggling back:\n%s", daily)
	}
}

func TestAAddsATaskUnderTheHeadingInTodaysDailyNote(t *testing.T) {
	m, root := taskList(t, "inbox.md")

	m = press(typeKeys(m, "aring mum [due:: tomorrow]"), enter)

	want := "- [-] cancelled trip\n- [ ] ring mum [due:: 2026-10-06]\n\n```markdown"
	if got := readFile(t, filepath.Join(root, "daily", "2026-10-05.md")); !strings.Contains(got, want) {
		t.Errorf("daily Note:\n%s\nwant it to contain\n%s", got, want)
	}
	if got := m.PaletteRows(); !slices.Contains(got, "Upcoming (3) | [ ] ring mum [due:: 2026-10-06] | daily/2026-10-05.md:9") {
		t.Errorf("Task List not back with the new Task:\n%s", lines(got))
	}
}

func TestAAddsToTheOpenDailyNotesBuffer(t *testing.T) {
	m, root := taskList(t, "daily/2026-10-05.md")

	m = press(typeKeys(m, "aring mum"), enter)

	if !strings.Contains(m.Text(), "- [-] cancelled trip\n- [ ] ring mum\n") {
		t.Errorf("buffer:\n%s", m.Text())
	}
	if got := readFile(t, filepath.Join(root, "daily", "2026-10-05.md")); strings.Contains(got, "ring mum") {
		t.Errorf("file written:\n%s", got)
	}
	if got := m.PaletteRows(); !slices.Contains(got, "No date (4) | [ ] ring mum | daily/2026-10-05.md:9") {
		t.Errorf("Task List lacks the new Task:\n%s", lines(got))
	}
}

func TestACreatesTodaysDailyNoteFromTheTemplate(t *testing.T) {
	m, root := tasksVault(t, "inbox.md", func(root string) {
		if err := os.Remove(filepath.Join(root, "daily", "2026-10-05.md")); err != nil {
			t.Fatal(err)
		}
	})
	m = typeKeys(press(m, space), "t")

	press(typeKeys(m, "aring mum"), enter)

	want := "# 2026-10-05\n\n## Tasks\n\n- [ ] template Task, never listed\n- [ ] ring mum\n"
	if got := readFile(t, filepath.Join(root, "daily", "2026-10-05.md")); got != want {
		t.Errorf("daily Note = %q, want %q", got, want)
	}
}

func TestEscInTheAddPromptGoesBackToTheList(t *testing.T) {
	m, _ := taskList(t, "inbox.md")

	m = press(typeKeys(m, "ax"), esc)

	if got := m.PaletteRows(); len(got) != 9 {
		t.Errorf("Task List not back:\n%s", lines(got))
	}
}

func TestDShowsAllDoneAndCancelledTasks(t *testing.T) {
	m, _ := taskList(t, "inbox.md")

	m = typeKeys(m, "D")

	got := m.PaletteRows()
	want := []string{
		"Done and cancelled (4) | [x] water plants ✅ 2026-10-05 | daily/2026-10-05.md:6",
		"Done and cancelled (4) | [-] cancelled trip | daily/2026-10-05.md:8",
		"Done and cancelled (4) | [x] old chore [completion:: 2026-10-01] | inbox.md:6",
		"Done and cancelled (4) | [X] pick tomato variety | projects/garden.md:5",
	}
	if len(got) != 12 || !reflect.DeepEqual(got[8:], want) {
		t.Errorf("rows:\n%s\nwant to end with:\n%s", lines(got), lines(want))
	}
	if got := typeKeys(m, "D").PaletteRows(); len(got) != 9 {
		t.Errorf("D again: rows\n%s", lines(got))
	}
}

func TestSlashFiltersOnTextTagAndFile(t *testing.T) {
	for query, want := range map[string][]string{
		"#mon": {"Today (1) | [ ] call the bank [due:: 2026-10-05] [priority:: high] #money | daily/2026-10-05.md:5"},
		"garden": {
			"No date (2) | [ ] plan beds owner:ted | projects/garden.md:3",
			"No date (2) | [ ] buy seeds #shopping | projects/garden.md:4",
		},
		"PAY rent":     {"Overdue (1) | [ ] pay rent [due:: 2026-10-01] [priority:: high] | inbox.md:3"},
		"#work report": {"Upcoming (1) | [/] write report 📅 2026-10-07 #work | daily/2026-10-05.md:7"},
	} {
		m, _ := taskList(t, "inbox.md")
		m = typeKeys(m, "/"+query)
		wantRows(t, m, want)
	}
}

func TestEscInTheFilterReturnsToTheList(t *testing.T) {
	m, _ := taskList(t, "inbox.md")

	m = typeKeys(press(typeKeys(m, "/i"), esc), "j")

	if got := m.PaletteSelected(); !strings.HasPrefix(got, "[ ] renew passport") {
		t.Errorf("selected %q: j should move in list mode, keeping the filter", got)
	}
	if m = press(m, esc); m.PaletteRows() != nil {
		t.Error("esc in the list did not close it")
	}
}

func TestTaskListSaysIndexingUntilTheScanIsDone(t *testing.T) {
	m, _ := unscanned(t, "inbox.md")

	m = ex(m, "tasks")

	if s := screen(m); !strings.Contains(s, "indexing…") {
		t.Errorf("screen lacks indexing…:\n%s", s)
	}
	m = drive(t, m, m.Init())
	if got := m.PaletteRows(); len(got) != 9 {
		t.Errorf("rows after the scan:\n%s", lines(got))
	}
}

func TestTaskListGolden(t *testing.T) {
	m, _ := taskList(t, "inbox.md")
	golden.RequireEqual(t, screen(m))
}

func TestEnterJumpsToTheTasksLine(t *testing.T) {
	m, root := taskList(t, "inbox.md")

	m = press(typeKeys(m, "jjjjjjj"), enter)

	if got := m.Path(); got != filepath.Join(root, "projects", "garden.md") {
		t.Fatalf("open Note = %s", got)
	}
	if got := m.Cursor(); got != (engine.Pos{Line: 3, Col: 3}) {
		t.Errorf("cursor = %+v, want 3:3", got)
	}
	if m.PaletteRows() != nil {
		t.Error("Task List still open")
	}
	m = press(m, ctrlO)
	if got := m.Path(); got != filepath.Join(root, "inbox.md") {
		t.Errorf("ctrl+o went to %s", got)
	}
}
