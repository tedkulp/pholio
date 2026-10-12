package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

// reviewUID is the meeting_uid of the Design review Meeting the tests
// pick: its UID and its start in UTC.
var reviewUID = "review@example.test/" + time.Date(2026, 10, 12, 14, 0, 0, 0, time.Local).UTC().Format("20060102T150405Z")

// meetingNotesVault opens /vault/a.md at meetingDay with one calendar
// serving the 14:00 Design review, plus files.
func meetingNotesVault(t *testing.T, files map[string]string) (app.Model, *seamtest.MemFS) {
	t.Helper()
	return meetingNotesVaultWith(t, vevent("review@example.test", "Design review", 14, 0, 60), files)
}

func meetingNotesVaultWith(t *testing.T, event string, files map[string]string) (app.Model, *seamtest.MemFS) {
	t.Helper()
	all := map[string]string{
		userConfig:    calendarsConfig("work", feed(t, 200, vcalendar(event))),
		"/vault/a.md": "hi\n",
	}
	for p, s := range files {
		all[p] = s
	}
	fsys := seamtest.NewMemFS(all)
	s, err := config.Startup(fsys, dirs, "/home/u", "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Clock: seamtest.NewClock(meetingDay)}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 100, 20), fsys
}

// openMeetings opens the Meeting picker and lets every feed arrive.
func openMeetings(t *testing.T, m app.Model) app.Model {
	t.Helper()
	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	return drive(t, m, cmd)
}

func fileText(t *testing.T, fsys *seamtest.MemFS, p string) string {
	t.Helper()
	b, err := fsys.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const reviewNote = "/vault/meetings/2026-10-12-1400-design-review.md"

func TestEnterCreatesTheMeetingNoteFromTheBuiltInTemplate(t *testing.T) {
	m, fsys := meetingNotesVault(t, nil)

	m = press(openMeetings(t, m), enter)

	want := "---\nmeeting_uid: \"" + reviewUID + "\"\n---\n" +
		"# Design review\n\n14:00–15:00 · [[2026-10-12]]\nAttendees: \nLocation: \nLink: \n\n## Notes\n"
	if got := fileText(t, fsys, reviewNote); got != want {
		t.Errorf("Meeting Note =\n%q\nwant\n%q", got, want)
	}
	if m.Path() != reviewNote || m.Text() != want {
		t.Errorf("open = %s %q, want the new Meeting Note", m.Path(), m.Text())
	}
	if got := m.Cursor().Line; got != 10 {
		t.Errorf("cursor line = %d, want 10 (## Notes, the last line)", got)
	}
	if got := messageLine(m); got != "Created meetings/2026-10-12-1400-design-review.md" {
		t.Errorf("message = %q", got)
	}
	viewHas(t, m, "NORMAL")
	if m = press(m, ctrlO); m.Path() != "/vault/a.md" {
		t.Errorf("ctrl-o went to %s, want /vault/a.md", m.Path())
	}
}

func TestMeetingTemplateFrontmatterGetsTheMeetingUID(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{
		"/vault/templates/meeting.md": "---\ntags: [meeting]\n---\n# {{title}} {{date}} {{start:h:mm A}}–{{end}} at {{time}} {{unknown}}\n",
	})

	_ = press(openMeetings(t, m), enter)

	want := "---\nmeeting_uid: \"" + reviewUID + "\"\ntags: [meeting]\n---\n# Design review 2026-10-12 2:00 PM–15:00 at 08:00 {{unknown}}\n"
	if got := fileText(t, fsys, reviewNote); got != want {
		t.Errorf("Meeting Note =\n%q\nwant\n%q", got, want)
	}
}

func TestMeetingTemplateWithoutFrontmatterGetsANewBlock(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{
		"/vault/.pholio/config.toml": "meetings_folder = \"mtg\"\nmeeting_template = \"t/m.md\"\n",
		"/vault/t/m.md":              "# {{title}}\n",
	})

	_ = press(openMeetings(t, m), enter)

	want := "---\nmeeting_uid: \"" + reviewUID + "\"\n---\n# Design review\n"
	if got := fileText(t, fsys, "/vault/mtg/2026-10-12-1400-design-review.md"); got != want {
		t.Errorf("Meeting Note =\n%q\nwant\n%q", got, want)
	}
}

func TestAMeetingNoteNameCollisionCountsUp(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{
		reviewNote: "# another Meeting\n",
		"/vault/meetings/2026-10-12-1400-design-review-2.md": "---\nmeeting_uid: \"other@example.test/20261012T120000Z\"\n---\n",
	})

	m = press(openMeetings(t, m), enter)

	if want := "/vault/meetings/2026-10-12-1400-design-review-3.md"; m.Path() != want {
		t.Errorf("opened %s, want %s", m.Path(), want)
	}
	if got := fileText(t, fsys, reviewNote); got != "# another Meeting\n" {
		t.Errorf("the other Meeting's Note changed: %q", got)
	}
}

func TestAnUntitledMeetingIsNamedMeeting(t *testing.T) {
	event := strings.Replace(vevent("x@example.test", "", 9, 0, 30), "SUMMARY:\r\n", "", 1)
	m, fsys := meetingNotesVaultWith(t, event, map[string]string{"/vault/templates/meeting.md": "# {{title}}\n"})

	_ = press(openMeetings(t, m), enter)

	if got := fileText(t, fsys, "/vault/meetings/2026-10-12-0900-meeting.md"); !strings.HasSuffix(got, "---\n# Untitled meeting\n") {
		t.Errorf("Meeting Note = %q", got)
	}
}

func TestAMovedOccurrenceKeepsItsMeetingNote(t *testing.T) {
	// A daily Design review from 14:00 on meetingDay, whose first
	// occurrence was moved to 16:00.
	series := strings.Replace(vevent("review@example.test", "Design review", 14, 0, 60), "SUMMARY:", "RRULE:FREQ=DAILY\r\nSUMMARY:", 1)
	moved := strings.Replace(vevent("review@example.test", "Design review", 16, 0, 60), "SUMMARY:",
		"RECURRENCE-ID:"+strings.TrimPrefix(reviewUID, "review@example.test/")+"\r\nSUMMARY:", 1)
	existing := "/vault/meetings/notes-from-review.md"
	m, _ := meetingNotesVaultWith(t, series+moved, map[string]string{
		existing: "---\nmeeting_uid: \"" + reviewUID + "\"\n---\n# Review\n",
	})

	m = openMeetings(t, m)
	if got := m.PaletteSelected(); got != "✓ 16:00–17:00  Design review" {
		t.Errorf("row = %q, want the moved occurrence, ticked", got)
	}
	if m = press(m, enter); m.Path() != existing {
		t.Errorf("opened %s, want %s", m.Path(), existing)
	}
}

func TestAMeetingWithAMeetingNoteIsTickedAndReopened(t *testing.T) {
	existing := "/vault/meetings/notes-from-review.md"
	m, _ := meetingNotesVault(t, map[string]string{
		existing: "---\ntitle: x\nmeeting_uid: \"" + reviewUID + "\"\n---\n# Review\n\nnotes\n",
	})

	m = openMeetings(t, m)
	if got := m.PaletteSelected(); got != "✓ 14:00–15:00  Design review" {
		t.Errorf("row = %q, want it ticked", got)
	}
	m = press(m, enter)

	if m.Path() != existing {
		t.Errorf("opened %s, want %s", m.Path(), existing)
	}
	if got := m.Cursor(); got.Line != 0 {
		t.Errorf("cursor = %v, want the usual start", got)
	}
	if got := messageLine(m); got != "" {
		t.Errorf("message = %q, want none", got)
	}
}
