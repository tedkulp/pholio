package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

// monday is 10am on Monday 2026-10-05, a day with no Daily Note in the
// daily fixture (which has 09-28, 10-01, 10-02 and 10-08).
var monday = time.Date(2026, 10, 5, 10, 0, 0, 0, time.Local)

// dailyVault copies the daily fixture and returns it with config dirs that
// hold no user config.
func dailyVault(t *testing.T) (vault string, d config.Dirs) {
	t.Helper()
	home := t.TempDir()
	return testutil.CopyVault(t, "daily"), config.Dirs{ConfigHome: filepath.Join(home, "config"), StateHome: filepath.Join(home, "state")}
}

// start runs pholio's startup on arg ("" for the Vault alone) at now.
func start(t *testing.T, vault string, d config.Dirs, arg string, now time.Time) app.Model {
	t.Helper()
	if arg == "" {
		arg = vault
	}
	s, err := config.Startup(seam.OSFS{}, d, filepath.Dir(d.ConfigHome), arg)
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.Start(app.Deps{FS: seam.OSFS{}, Clock: seamtest.NewClock(now)}, s)
	if err != nil {
		t.Fatal(err)
	}
	return resize(m, 80, 12)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func wantOpen(t *testing.T, m app.Model, rel string) {
	t.Helper()
	if s := statusLine(m); !strings.Contains(s, " "+rel) {
		t.Errorf("status line %q, want %s open", s, rel)
	}
}

const mondayNote = "# Monday, October 5th 2026\n\n[[2026-10-04]] · [[2026-10-06]]\n\n## Tasks\n"

func TestStartupOpensTodaysDailyNoteFromTheTemplate(t *testing.T) {
	vault, d := dailyVault(t)

	m := start(t, vault, d, "", monday)

	wantOpen(t, m, "daily/2026-10-05.md")
	if m.Text() != mondayNote {
		t.Errorf("text = %q, want %q", m.Text(), mondayNote)
	}
	if got, _ := os.ReadFile(filepath.Join(vault, "daily", "2026-10-05.md")); string(got) != mondayNote {
		t.Errorf("on disk = %q, want the new Note written", got)
	}
}

func TestSmokeStartupOpensTodaysDailyNote(t *testing.T) {
	vault, d := dailyVault(t)
	s, err := config.Startup(seam.OSFS{}, d, filepath.Dir(d.ConfigHome), vault)
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.Start(app.Deps{FS: seam.OSFS{}, Clock: seamtest.NewClock(monday)}, s)
	if err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(smokeW, smokeH),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.Ascii)),
	)
	scr := newVTScreen()
	scr.waitFor(t, tm, func(s string) bool {
		return strings.Contains(s, "# Monday, October 5th 2026") && strings.Contains(s, "daily/2026-10-05.md")
	})

	tm.Send(ctrlQ)
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestStartupBeforeTheDayStartsOpensYesterdaysNote(t *testing.T) {
	vault, d := dailyVault(t) // day_starts_at = "04:00"

	m := start(t, vault, d, "", time.Date(2026, 10, 6, 3, 30, 0, 0, time.Local))

	wantOpen(t, m, "daily/2026-10-05.md")
}

func TestStartupWithAFileOpensItAndNoDailyNote(t *testing.T) {
	vault, d := dailyVault(t)

	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	wantOpen(t, m, "README.md")
	if exists(filepath.Join(vault, "daily", "2026-10-05.md")) {
		t.Error("today's Daily Note was created")
	}
}

func TestStartupWithDailyOffOpensNoNote(t *testing.T) {
	vault, d := dailyVault(t)
	if err := os.MkdirAll(filepath.Join(d.ConfigHome, "pholio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(d), []byte("open_daily_on_startup = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := start(t, vault, d, "", monday)

	if exists(filepath.Join(vault, "daily", "2026-10-05.md")) {
		t.Error("today's Daily Note was created")
	}
	if strings.TrimSpace(m.Text()) != "" {
		t.Errorf("text = %q, want an empty buffer", m.Text())
	}
	if s := statusLine(m); !strings.Contains(s, "[No Name]") {
		t.Errorf("status line %q, want an unnamed buffer", s)
	}
}

func TestPrevAndNextDailyNoteSkipGapsAndNeverCreate(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, "", monday)

	for _, want := range []string{"2026-10-02", "2026-10-01", "2026-09-28"} {
		m = typeKeys(m, "[d")
		wantOpen(t, m, "daily/"+want+".md")
	}
	m = typeKeys(m, "[d")
	wantOpen(t, m, "daily/2026-09-28.md")
	if msg := rows(m)[11]; !strings.Contains(msg, "no earlier Daily Note") {
		t.Errorf("message line %q", msg)
	}

	for _, want := range []string{"2026-10-01", "2026-10-02", "2026-10-05", "2026-10-08"} {
		m = typeKeys(m, "]d")
		wantOpen(t, m, "daily/"+want+".md")
	}
	m = typeKeys(m, "]d")
	if msg := rows(m)[11]; !strings.Contains(msg, "no later Daily Note") {
		t.Errorf("message line %q", msg)
	}
	for _, gap := range []string{"2026-09-29", "2026-10-03", "2026-10-04", "2026-10-06"} {
		if exists(filepath.Join(vault, "daily", gap+".md")) {
			t.Errorf("%s was created", gap)
		}
	}
}

func TestDailyStepsAreRecordedInTheJumplist(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, "", monday)

	m = typeKeys(m, "]d")
	wantOpen(t, m, "daily/2026-10-08.md")
	m = press(m, ctrlO)
	wantOpen(t, m, "daily/2026-10-05.md")
	m = press(m, tab)
	wantOpen(t, m, "daily/2026-10-08.md")
}

func TestSpcDIsRecordedInTheJumplist(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = keys(m, space)
	m = typeKeys(m, "d")
	wantOpen(t, m, "daily/2026-10-05.md")
	m = press(m, ctrlO)
	wantOpen(t, m, "README.md")
}

func TestPrevDailyNoteFromAnotherNoteStartsAtToday(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = typeKeys(m, "]d")

	wantOpen(t, m, "daily/2026-10-08.md")
}

func TestBracketWithAnotherKeyStillReachesTheEditor(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = typeKeys(m, "[x")

	if !strings.HasPrefix(m.Text(), " The daily Vault") {
		t.Errorf("text = %q, want x to delete the #", m.Text())
	}
}

func TestSpcDOpensToday(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = keys(m, space)
	m = typeKeys(m, "d")

	wantOpen(t, m, "daily/2026-10-05.md")
	if m.Text() != mondayNote {
		t.Errorf("text = %q", m.Text())
	}
}

func TestColonTodayOpensToday(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = ex(m, "today")

	wantOpen(t, m, "daily/2026-10-05.md")
}

func TestColonDailyCreatesTheNoteForADate(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = ex(m, "daily friday")

	wantOpen(t, m, "daily/2026-10-09.md")
	if !strings.HasPrefix(m.Text(), "# Friday, October 9th 2026\n\n[[2026-10-08]] · [[2026-10-10]]") {
		t.Errorf("text = %q", m.Text())
	}
	if !exists(filepath.Join(vault, "daily", "2026-10-09.md")) {
		t.Error("not written")
	}
}

func TestColonDailyOpensAnExistingNoteAsIs(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = ex(m, "daily 2026-10-01")

	wantOpen(t, m, "daily/2026-10-01.md")
	if m.Text() != "# Thursday, October 1st 2026\n" {
		t.Errorf("text = %q", m.Text())
	}
}

func TestColonDailyRejectsANonDate(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = ex(m, "daily someday")

	wantOpen(t, m, "README.md")
	if msg := rows(m)[11]; !strings.Contains(msg, `not a date: "someday"`) {
		t.Errorf("message line %q", msg)
	}
}

func TestSpcShiftDJumpsToATypedDate(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)

	m = keys(m, space)
	m = typeKeys(m, "D")
	if s := screen(m); !strings.Contains(s, "Jump to date") {
		t.Fatalf("no date prompt:\n%s", s)
	}
	m = typeKeys(m, "-1w")
	if s := screen(m); !strings.Contains(s, "Monday 2026-09-28") {
		t.Errorf("prompt does not preview the date:\n%s", s)
	}
	m = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	wantOpen(t, m, "daily/2026-09-28.md")
}

func TestDailyOverADirtyBufferAsksAndCancelCreatesNothing(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)
	m = typeKeys(m, "x")

	m = ex(m, "daily tomorrow")
	if msg := rows(m)[11]; !strings.Contains(msg, "Save changes to README.md?") {
		t.Fatalf("message line %q, want a question", msg)
	}
	m = keys(m, esc)

	wantOpen(t, m, "README.md")
	if exists(filepath.Join(vault, "daily", "2026-10-06.md")) {
		t.Error("tomorrow's Note was created though the switch was cancelled")
	}
}

func TestTodayUsesTheConfiguredDayStart(t *testing.T) {
	vault, d := dailyVault(t)
	clock := seamtest.NewClock(time.Date(2026, 10, 6, 3, 59, 0, 0, time.Local))
	s, err := config.Startup(seam.OSFS{}, d, filepath.Dir(d.ConfigHome), filepath.Join(vault, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.Start(app.Deps{FS: seam.OSFS{}, Clock: clock}, s)
	if err != nil {
		t.Fatal(err)
	}
	m = resize(m, 80, 12)

	m = ex(m, "today")
	wantOpen(t, m, "daily/2026-10-05.md")

	clock.Set(time.Date(2026, 10, 6, 4, 0, 0, 0, time.Local))
	m = ex(m, "today")
	wantOpen(t, m, "daily/2026-10-06.md")
}

func TestDailySubfolderNestsNewNotesAndMixesWithFlatOnes(t *testing.T) {
	vault, d := dailyVault(t)
	f, err := os.OpenFile(config.VaultConfigPath(vault), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("daily_subfolder = \"YYYY/MM\"\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}

	m := start(t, vault, d, "", time.Date(2026, 10, 6, 10, 0, 0, 0, time.Local))

	wantOpen(t, m, "daily/2026/10/2026-10-06.md")
	if !strings.HasPrefix(m.Text(), "# Tuesday, October 6th 2026\n") {
		t.Errorf("text = %q, want the daily Template", m.Text())
	}
	m = typeKeys(m, "[d")
	wantOpen(t, m, "daily/2026-10-02.md")
	m = typeKeys(m, "]d")
	wantOpen(t, m, "daily/2026/10/2026-10-06.md")
	m = typeKeys(m, "]d")
	wantOpen(t, m, "daily/2026-10-08.md")
}
