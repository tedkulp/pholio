package daily_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/daily"
	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

func day(s string) time.Time {
	d, err := time.ParseInLocation(dates.ISO, s, time.Local)
	if err != nil {
		panic(err)
	}
	return d
}

func fixture(t *testing.T) (daily.Notes, string) {
	t.Helper()
	vault := testutil.CopyVault(t, "daily")
	return daily.Notes{FS: seam.OSFS{}, Vault: vault, Folder: "daily", Template: "templates/daily.md"}, vault
}

func TestPathIsInTheDailyFolder(t *testing.T) {
	n := daily.Notes{Vault: "/vault", Folder: "journal/days"}
	if got, want := n.Path(day("2026-10-05")), filepath.FromSlash("/vault/journal/days/2026-10-05.md"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestDayOfADailyNote(t *testing.T) {
	n := daily.Notes{Vault: "/vault", Folder: "daily"}
	if d, ok := n.Day("/vault/daily/2026-10-05.md"); !ok || !d.Equal(day("2026-10-05")) {
		t.Errorf("Day = %v %v, want 2026-10-05", d, ok)
	}
	for _, p := range []string{"/vault/daily/notes.md", "/vault/2026-10-05.md", "/vault/daily/2026-02-30.md", "/vault/daily/2026-10-05.txt"} {
		if _, ok := n.Day(p); ok {
			t.Errorf("Day(%q) ok, want not a Daily Note", p)
		}
	}
}

func TestPathInADailySubfolder(t *testing.T) {
	n := daily.Notes{Vault: "/vault", Folder: "daily", Subfolder: "YYYY/MM"}
	if got, want := n.Path(day("2026-10-06")), filepath.FromSlash("/vault/daily/2026/10/2026-10-06.md"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestDayOfANestedDailyNote(t *testing.T) {
	for _, sub := range []string{"", "YYYY/MM"} {
		n := daily.Notes{Vault: "/vault", Folder: "daily", Subfolder: sub}
		for p, want := range map[string]string{
			"/vault/daily/2026/10/2026-10-06.md": "2026-10-06",
			"/vault/daily/2026-10-05.md":         "2026-10-05",
			"/vault/daily/old/2026-01-02.md":     "2026-01-02",
		} {
			if d, ok := n.Day(p); !ok || !d.Equal(day(want)) {
				t.Errorf("subfolder %q: Day(%q) = %v %v, want %s", sub, p, d, ok, want)
			}
		}
		for _, p := range []string{"/vault/daily/2026/10/notes.md", "/vault/other/2026-10-06.md", "/vault/dailyx/2026-10-06.md"} {
			if _, ok := n.Day(p); ok {
				t.Errorf("subfolder %q: Day(%q) ok, want not a Daily Note", sub, p)
			}
		}
	}
}

func TestPrevAndNextMixFlatAndNestedNotes(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		"/vault/daily/2026-09-30.md":         "",
		"/vault/daily/2026/10/2026-10-02.md": "",
		"/vault/daily/2026-10-02.md":         "", // the same day, flat
		"/vault/daily/2026/10/notes.md":      "",
		"/vault/daily/2026/10/2026-10-06.md": "",
		"/vault/elsewhere/2026-10-04.md":     "",
	})
	n := daily.Notes{FS: fsys, Vault: "/vault", Folder: "daily", Subfolder: "YYYY/MM"}
	for _, c := range []struct{ from, prev, next string }{
		{"2026-10-06", "/vault/daily/2026/10/2026-10-02.md", ""},
		{"2026-10-02", "/vault/daily/2026-09-30.md", "/vault/daily/2026/10/2026-10-06.md"},
		{"2026-09-30", "", "/vault/daily/2026/10/2026-10-02.md"},
	} {
		got, ok, err := n.Prev(day(c.from))
		if err != nil || got.Path != c.prev {
			t.Errorf("Prev(%s) = %+v %v %v, want %q", c.from, got, ok, err, c.prev)
		}
		got, ok, err = n.Next(day(c.from))
		if err != nil || got.Path != c.next {
			t.Errorf("Next(%s) = %+v %v %v, want %q", c.from, got, ok, err, c.next)
		}
	}
}

func TestEnsureInADailySubfolderMakesItsFolders(t *testing.T) {
	n, vault := fixture(t)
	n.Subfolder = "YYYY/MM"

	path, data, err := n.Ensure(day("2026-10-06"), day("2026-10-06"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(vault, "daily", "2026", "10", "2026-10-06.md"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if want := "# Tuesday, October 6th 2026\n\n[[2026-10-05]] · [[2026-10-07]]\n\n## Tasks\n"; string(data) != want {
		t.Errorf("data = %q, want %q", data, want)
	}
}

func TestPrevAndNextSkipGaps(t *testing.T) {
	n, _ := fixture(t)
	for _, c := range []struct {
		from, prev, next string
	}{
		{"2026-10-05", "2026-10-02", "2026-10-08"}, // a day with no Note
		{"2026-10-02", "2026-10-01", "2026-10-08"},
		{"2026-10-01", "2026-09-28", "2026-10-02"},
	} {
		if got, ok, err := n.Prev(day(c.from)); err != nil || !ok || !got.Day.Equal(day(c.prev)) {
			t.Errorf("Prev(%s) = %s %v %v, want %s", c.from, got.Day.Format(dates.ISO), ok, err, c.prev)
		}
		if got, ok, err := n.Next(day(c.from)); err != nil || !ok || !got.Day.Equal(day(c.next)) {
			t.Errorf("Next(%s) = %s %v %v, want %s", c.from, got.Day.Format(dates.ISO), ok, err, c.next)
		}
	}
}

func TestPrevAndNextStopAtTheEnds(t *testing.T) {
	n, _ := fixture(t)
	if _, ok, err := n.Prev(day("2026-09-28")); ok || err != nil {
		t.Errorf("Prev of the first: ok=%v err=%v, want none", ok, err)
	}
	if _, ok, err := n.Next(day("2026-10-08")); ok || err != nil {
		t.Errorf("Next of the last: ok=%v err=%v, want none", ok, err)
	}
}

func TestPrevWithoutADailyFolderFindsNothing(t *testing.T) {
	n := daily.Notes{FS: seamtest.NewMemFS(map[string]string{"/vault/a.md": ""}), Vault: "/vault", Folder: "daily"}
	if _, ok, err := n.Prev(day("2026-10-05")); ok || err != nil {
		t.Errorf("Prev = ok %v err %v, want nothing and no error", ok, err)
	}
}

func TestEnsureCreatesTheNoteFromTheTemplate(t *testing.T) {
	n, vault := fixture(t)
	now := time.Date(2026, 10, 9, 8, 0, 0, 0, time.Local)

	path, data, err := n.Ensure(day("2026-10-05"), now)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(vault, "daily", "2026-10-05.md"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	want := "# Monday, October 5th 2026\n\n[[2026-10-04]] · [[2026-10-06]]\n\n## Tasks\n"
	if string(data) != want {
		t.Errorf("data = %q, want %q", data, want)
	}
	if got, _ := n.FS.ReadFile(path); string(got) != want {
		t.Errorf("on disk = %q, want %q", got, want)
	}
}

func TestEnsureLeavesAnExistingNoteAlone(t *testing.T) {
	n, vault := fixture(t)

	path, data, err := n.Ensure(day("2026-10-02"), day("2026-10-05"))
	if err != nil || data != nil {
		t.Fatalf("Ensure = %q, %v; want no write", data, err)
	}
	if got, _ := n.FS.ReadFile(filepath.Join(vault, "daily", "2026-10-02.md")); string(got) != "# Friday, October 2nd 2026\n" {
		t.Errorf("existing Note changed: %q (path %s)", got, path)
	}
}

func TestEnsureWithoutATemplateMakesAnEmptyNoteAndItsFolder(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": ""})
	n := daily.Notes{FS: fsys, Vault: "/vault", Folder: "days/2026", Template: "templates/daily.md"}

	path, data, err := n.Ensure(day("2026-10-05"), day("2026-10-05"))
	if err != nil {
		t.Fatal(err)
	}
	if path != "/vault/days/2026/2026-10-05.md" || len(data) != 0 || data == nil {
		t.Errorf("Ensure = %q %q, want an empty new Note", path, data)
	}
	if _, err := fsys.Stat(path); err != nil {
		t.Errorf("not written: %v", err)
	}
}

func TestEnsureReportsATemplateItCannotRead(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/templates/daily.md/x": ""}) // a folder
	n := daily.Notes{FS: fsys, Vault: "/vault", Folder: "daily", Template: "templates/daily.md"}

	if _, _, err := n.Ensure(day("2026-10-05"), day("2026-10-05")); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want a read error", err)
	}
}
