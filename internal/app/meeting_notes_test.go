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

func meetingNotesVaultWith(t *testing.T, meeting string, files map[string]string) (app.Model, *seamtest.MemFS) {
	t.Helper()
	all := map[string]string{
		userConfig:    calendarsConfig("work", feed(t, 200, vcalendar(meeting))),
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
	if got := messageLine(m); got != "Created meetings/2026-10-12-1400-design-review.md (no Daily Note for 2026-10-12)" {
		t.Errorf("message = %q", got)
	}
	if _, err := fsys.Stat(reviewDaily); err == nil {
		t.Error("the back-link created the Daily Note")
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

func TestTheMeetingNoteHasTheMeetingsDetails(t *testing.T) {
	meeting := strings.Replace(vevent("review@example.test", "Design review", 14, 0, 60), "END:VEVENT",
		"ATTENDEE;CN=Ada Example:mailto:ada@example.test\r\nATTENDEE:mailto:bob@example.test\r\n"+
			"LOCATION:Room 4\\, Floor 2\r\nDESCRIPTION:Join https://acme.zoom.us/j/1\\nBring notes\r\nEND:VEVENT", 1)
	m, fsys := meetingNotesVaultWith(t, meeting, map[string]string{
		"/vault/templates/meeting.md": "{{attendees}}|{{location}}|{{link}}|{{description}}\n",
	})

	_ = press(openMeetings(t, m), enter)

	want := "---\nmeeting_uid: \"" + reviewUID + "\"\n---\n" +
		"Ada Example, bob@example.test|Room 4, Floor 2|https://acme.zoom.us/j/1|Join https://acme.zoom.us/j/1\nBring notes\n"
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
	meeting := strings.Replace(vevent("x@example.test", "", 9, 0, 30), "SUMMARY:\r\n", "", 1)
	m, fsys := meetingNotesVaultWith(t, meeting, map[string]string{"/vault/templates/meeting.md": "# {{title}}\n"})

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

const reviewDaily = "/vault/daily/2026-10-12.md"

func TestCreatingAMeetingNoteLinksItFromTheDailyNote(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{
		reviewDaily: "# Mon\n\n## Meetings\n- 09:00 [[standup]]\n\n## Tasks\n- [ ] x\n",
	})

	m = press(openMeetings(t, m), enter)

	want := "# Mon\n\n## Meetings\n- 09:00 [[standup]]\n- 14:00 [[2026-10-12-1400-design-review]]\n\n## Tasks\n- [ ] x\n"
	if got := fileText(t, fsys, reviewDaily); got != want {
		t.Errorf("Daily Note =\n%q\nwant\n%q", got, want)
	}
	if got := messageLine(m); got != "Created meetings/2026-10-12-1400-design-review.md" {
		t.Errorf("message = %q", got)
	}
}

func TestTheMeetingsHeadingIsAddedWhenMissing(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{
		"/vault/.pholio/config.toml": "meetings_heading = \"### Calls\"\n",
		reviewDaily:                  "# Mon\n\n## Meetings\n",
	})

	_ = press(openMeetings(t, m), enter)

	want := "# Mon\n\n## Meetings\n\n### Calls\n- 14:00 [[2026-10-12-1400-design-review]]\n"
	if got := fileText(t, fsys, reviewDaily); got != want {
		t.Errorf("Daily Note =\n%q\nwant\n%q", got, want)
	}
}

func TestTheBackLinkReachesTheOpenDailyNote(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{reviewDaily: "## Meetings\n"})
	m = ex(m, "e "+reviewDaily)

	m = press(openMeetings(t, m), enter)

	want := "## Meetings\n- 14:00 [[2026-10-12-1400-design-review]]\n"
	if got := fileText(t, fsys, reviewDaily); got != want {
		t.Errorf("Daily Note =\n%q\nwant\n%q", got, want)
	}
	if m = press(m, ctrlO); m.Path() != reviewDaily || m.Text() != want {
		t.Errorf("ctrl-o = %s %q, want the Daily Note with the back-link", m.Path(), m.Text())
	}
}

func TestTheBackLinkReachesADirtyOpenDailyNote(t *testing.T) {
	for _, tc := range []struct{ answer, want string }{
		{"y", "## Meetings\nmine\n- 14:00 [[2026-10-12-1400-design-review]]\n"},
		{"n", "## Meetings\n- 14:00 [[2026-10-12-1400-design-review]]\n"},
	} {
		t.Run(tc.answer, func(t *testing.T) {
			m, fsys := meetingNotesVault(t, map[string]string{reviewDaily: "## Meetings\n"})
			m = press(typeKeys(ex(m, "e "+reviewDaily), "omine"), esc)

			m = press(openMeetings(t, m), enter)
			m = typeKeys(m, tc.answer)

			if got := fileText(t, fsys, reviewDaily); got != tc.want {
				t.Errorf("Daily Note =\n%q\nwant\n%q", got, tc.want)
			}
			if m.Path() != reviewNote {
				t.Errorf("opened %s, want the Meeting Note", m.Path())
			}
		})
	}
}

func TestAMeetingNoteInASubfolderIsFound(t *testing.T) {
	existing := "/vault/meetings/2026/10/notes-from-review.md"
	m, _ := meetingNotesVault(t, map[string]string{
		existing: "---\nmeeting_uid: \"" + reviewUID + "\"\n---\n# Review\n",
	})

	m = openMeetings(t, m)
	if got := m.PaletteSelected(); got != "✓ 14:00–15:00  Design review" {
		t.Errorf("row = %q, want it ticked", got)
	}
	if m = press(m, enter); m.Path() != existing {
		t.Errorf("opened %s, want %s", m.Path(), existing)
	}
}

func TestANoteAppearingWhileAskedIsNotOverwritten(t *testing.T) {
	m, fsys := meetingNotesVault(t, nil)
	m = press(typeKeys(m, "omine"), esc)

	m = press(openMeetings(t, m), enter) // asks to save a.md first
	if err := fsys.WriteFile(reviewNote, []byte("# someone else's\n")); err != nil {
		t.Fatal(err)
	}
	m = typeKeys(m, "y")

	if got := fileText(t, fsys, reviewNote); got != "# someone else's\n" {
		t.Errorf("the Note that appeared was overwritten: %q", got)
	}
	if want := "/vault/meetings/2026-10-12-1400-design-review-2.md"; m.Path() != want {
		t.Errorf("opened %s, want %s", m.Path(), want)
	}
}

func TestACRLFTemplateFrontmatterGetsTheMeetingUID(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{
		"/vault/templates/meeting.md": "---\r\ntags: [meeting]\r\n---\r\n# {{title}}\r\n",
	})

	_ = press(openMeetings(t, m), enter)

	want := "---\r\nmeeting_uid: \"" + reviewUID + "\"\r\ntags: [meeting]\r\n---\r\n# Design review\r\n"
	if got := fileText(t, fsys, reviewNote); got != want {
		t.Errorf("Meeting Note =\n%q\nwant\n%q", got, want)
	}
}

const monthSubfolderConfig = "meetings_subfolder = \"YYYY/MM\"\n"

func TestMeetingsSubfolderNestsANewMeetingNote(t *testing.T) {
	m, fsys := meetingNotesVault(t, map[string]string{
		"/vault/.pholio/config.toml": monthSubfolderConfig,
		reviewDaily:                  "## Meetings\n",
	})

	m = press(openMeetings(t, m), enter)

	if want := "/vault/meetings/2026/10/2026-10-12-1400-design-review.md"; m.Path() != want {
		t.Errorf("opened %s, want %s", m.Path(), want)
	}
	if got, want := fileText(t, fsys, reviewDaily), "## Meetings\n- 14:00 [[2026-10-12-1400-design-review]]\n"; got != want {
		t.Errorf("Daily Note = %q, want %q", got, want)
	}
}

func TestMeetingsSubfolderFilesByTheMeetingsOwnDate(t *testing.T) {
	m, _ := meetingNotesVaultWith(t, vevent("late@example.test", "Late call", 0, 30, 30), map[string]string{
		"/vault/.pholio/config.toml": "day_starts_at = \"04:00\"\nmeetings_subfolder = \"YYYY/MM/DD\"\n",
	})

	m = press(openMeetings(t, m), enter)

	if want := "/vault/meetings/2026/10/12/2026-10-12-0030-late-call.md"; m.Path() != want {
		t.Errorf("opened %s, want %s", m.Path(), want)
	}
}

func TestMeetingsSubfolderNamesCollideAcrossTheMeetingsFolder(t *testing.T) {
	m, _ := meetingNotesVault(t, map[string]string{
		"/vault/.pholio/config.toml": monthSubfolderConfig,
		reviewNote:                   "# another Meeting\n",
	})

	m = press(openMeetings(t, m), enter)

	if want := "/vault/meetings/2026/10/2026-10-12-1400-design-review-2.md"; m.Path() != want {
		t.Errorf("opened %s, want %s", m.Path(), want)
	}
}

func TestMeetingsSubfolderReopensAMeetingNoteInAnotherFolder(t *testing.T) {
	existing := "/vault/meetings/notes-from-review.md"
	m, _ := meetingNotesVault(t, map[string]string{
		"/vault/.pholio/config.toml": monthSubfolderConfig,
		existing:                     "---\nmeeting_uid: \"" + reviewUID + "\"\n---\n# Review\n",
	})

	if m = press(openMeetings(t, m), enter); m.Path() != existing {
		t.Errorf("opened %s, want %s", m.Path(), existing)
	}
}

func TestMeetingsSubfolderIgnoresHiddenFolders(t *testing.T) {
	m, _ := meetingNotesVault(t, map[string]string{
		"/vault/.pholio/config.toml":                              monthSubfolderConfig,
		"/vault/meetings/.trash/2026-10-12-1400-design-review.md": "---\nmeeting_uid: \"" + reviewUID + "\"\n---\n",
	})

	m = press(openMeetings(t, m), enter)

	if want := "/vault/meetings/2026/10/2026-10-12-1400-design-review.md"; m.Path() != want {
		t.Errorf("opened %s, want %s", m.Path(), want)
	}
}
