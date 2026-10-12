package app_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

// meetingDay is 08:00 local on Monday 2026-10-12.
var meetingDay = time.Date(2026, 10, 12, 8, 0, 0, 0, time.Local)

// vevent is a synthetic Meeting starting at hh:mm local on meetingDay.
func vevent(uid, title string, hh, mm, minutes int) string {
	start := time.Date(2026, 10, 12, hh, mm, 0, 0, time.Local).UTC()
	end := start.Add(time.Duration(minutes) * time.Minute)
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20261001T000000Z\r\n" +
		"DTSTART:" + start.Format("20060102T150405Z") + "\r\nDTEND:" + end.Format("20060102T150405Z") + "\r\n" +
		"SUMMARY:" + title + "\r\nEND:VEVENT\r\n"
}

func vcalendar(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//pholio//test//EN\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

// feed serves body, or status when it isn't 200, and returns its URL.
func feed(t *testing.T, status int, body string) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s.URL
}

// calendarsConfig is a user config with a [[calendars]] entry per name, url
// pair.
func calendarsConfig(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		b.WriteString("[[calendars]]\nname = \"" + pairs[i] + "\"\nurl = \"" + pairs[i+1] + "\"\n")
	}
	return b.String()
}

// meetingsApp opens /vault/a.md at meetingDay with userCfg as the user
// config.
func meetingsApp(t *testing.T, userCfg string) app.Model {
	t.Helper()
	fsys := seamtest.NewMemFS(map[string]string{userConfig: userCfg, "/vault/a.md": "hi\n"})
	s, err := config.Startup(fsys, dirs, "/home/u", "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Clock: seamtest.NewClock(meetingDay)}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 100, 20)
}

// pressing presses k and returns the command it gave, unrun.
func pressing(m app.Model, k tea.KeyPressMsg) (app.Model, tea.Cmd) {
	next, cmd := m.Update(k)
	return next.(app.Model), cmd
}

func viewHas(t *testing.T, m app.Model, s string) {
	t.Helper()
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, s) {
		t.Errorf("view does not show %q:\n%s", s, v)
	}
}

func TestSpcMListsTodaysMeetingsFromEveryCalendar(t *testing.T) {
	work := feed(t, 200, vcalendar(
		vevent("review@example.test", "Design review", 14, 0, 60),
		vevent("standup@example.test", "Standup", 9, 30, 15),
	))
	home := feed(t, 200, vcalendar(
		vevent("dentist@example.test", "Dentist", 11, 0, 45),
		"BEGIN:VEVENT\r\nUID:holiday@example.test\r\nDTSTAMP:20261001T000000Z\r\nDTSTART;VALUE=DATE:20261012\r\nSUMMARY:Holiday\r\nEND:VEVENT\r\n",
	))
	m := meetingsApp(t, calendarsConfig("work", work, "home", home))

	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	viewHas(t, m, "Meetings · Mon 12 Oct (today)")
	viewHas(t, m, "Loading…")
	m = drive(t, m, cmd)

	want := []string{
		" |   09:30–09:45  Standup | work",
		" |   11:00–11:45  Dentist | home",
		" |   14:00–15:00  Design review | work",
	}
	if got := m.PaletteRows(); !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %q\nwant %q", got, want)
	}
	if got := m.PaletteSelected(); got != "  09:30–09:45  Standup" {
		t.Errorf("selected = %q, want the first row", got)
	}
	if m = press(m, esc); m.PaletteRows() != nil {
		t.Error("esc left the picker open")
	}
}

func TestMeetingsExCommandOpensThePicker(t *testing.T) {
	m := meetingsApp(t, calendarsConfig("work", feed(t, 200, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))))

	m, cmd := pressing(typeKeys(m, ":meetings"), enter)
	m = drive(t, m, cmd)

	if got := m.PaletteRows(); len(got) != 1 {
		t.Errorf("rows = %q, want the one Meeting", got)
	}
}

func TestAFailingCalendarFailsAlone(t *testing.T) {
	ok := feed(t, 200, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))
	m := meetingsApp(t, calendarsConfig("secret", feed(t, 401, ""), "work", ok))

	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	m = drive(t, m, cmd)

	if got := m.PaletteRows(); len(got) != 1 {
		t.Errorf("rows = %q, want the working calendar's Meeting", got)
	}
	viewHas(t, m, "secret: HTTP 401")
}

func TestAnEmptyDaySaysNoMeetings(t *testing.T) {
	m := meetingsApp(t, calendarsConfig("work", feed(t, 200, vcalendar())))

	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	m = drive(t, m, cmd)

	viewHas(t, m, "no Meetings")
}

func TestNoCalendarsSaysWhereToAddThem(t *testing.T) {
	m := meetingsApp(t, "")

	m = typeKeys(press(m, space), "m")

	if m.PaletteRows() != nil {
		t.Error("a picker opened with no calendars")
	}
	if got, want := messageLine(m), "no calendars: add [[calendars]] to "+userConfig; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}
