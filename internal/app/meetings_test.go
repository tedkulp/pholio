package app_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
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
	return veventOn(12, uid, title, hh, mm, minutes)
}

// veventOn is vevent on day (of October 2026).
func veventOn(day int, uid, title string, hh, mm, minutes int) string {
	start := time.Date(2026, 10, day, hh, mm, 0, 0, time.Local).UTC()
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
// config and an empty calendar cache.
func meetingsApp(t *testing.T, userCfg string) app.Model {
	t.Helper()
	return meetingsAppAt(t, t.TempDir(), seamtest.NewClock(meetingDay), userCfg)
}

// meetingsAppAt is meetingsApp with the cache home and clock given.
func meetingsAppAt(t *testing.T, cacheHome string, clock *seamtest.Clock, userCfg string) app.Model {
	t.Helper()
	fsys := seamtest.NewMemFS(map[string]string{userConfig: userCfg, "/vault/a.md": "hi\n"})
	d := dirs
	d.CacheHome = cacheHome
	s, err := config.Startup(fsys, d, "/home/u", "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Clock: clock}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 100, 20)
}

// liveFeed is a feed whose answer can change, counting its fetches.
type liveFeed struct {
	url    string
	status atomic.Int64
	body   atomic.Value // string
	hits   atomic.Int32
}

func newLiveFeed(t *testing.T, body string) *liveFeed {
	t.Helper()
	f := &liveFeed{}
	f.set(200, body)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.hits.Add(1)
		w.WriteHeader(int(f.status.Load()))
		_, _ = w.Write([]byte(f.body.Load().(string)))
	}))
	t.Cleanup(s.Close)
	f.url = s.URL
	return f
}

func (f *liveFeed) set(status int, body string) {
	f.status.Store(int64(status))
	f.body.Store(body)
}

// setFetched backdates every cached feed under cacheHome to at.
func setFetched(t *testing.T, cacheHome string, at time.Time) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(cacheHome, "pholio", "calendars", "*.ics"))
	if len(files) == 0 {
		t.Fatal("nothing cached")
	}
	for _, f := range files {
		if err := os.Chtimes(f, at, at); err != nil {
			t.Fatal(err)
		}
	}
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

func TestDayKeysMoveThePickerBetweenDays(t *testing.T) {
	m := meetingsApp(t, calendarsConfig("work", feed(t, 200, vcalendar(
		veventOn(11, "sun@example.test", "Brunch", 11, 0, 60),
		veventOn(12, "mon@example.test", "Standup", 9, 30, 15),
		veventOn(13, "tue@example.test", "Retro", 15, 0, 60),
		veventOn(14, "wed@example.test", "Lunch", 12, 0, 60),
	))))
	m = openMeetings(t, m)

	steps := []struct {
		key   tea.KeyPressMsg
		title string
		rows  []string
	}{
		{tea.KeyPressMsg{Code: 'l', Text: "l"}, "Meetings · Tue 13 Oct (tomorrow)", []string{" |   15:00–16:00  Retro | work"}},
		{tea.KeyPressMsg{Code: tea.KeyRight}, "Meetings · Wed 14 Oct ─", []string{" |   12:00–13:00  Lunch | work"}},
		{tea.KeyPressMsg{Code: 't', Text: "t"}, "Meetings · Mon 12 Oct (today)", []string{" |   09:30–09:45  Standup | work"}},
		{tea.KeyPressMsg{Code: 'h', Text: "h"}, "Meetings · Sun 11 Oct (yesterday)", []string{" |   11:00–12:00  Brunch | work"}},
		{tea.KeyPressMsg{Code: tea.KeyLeft}, "Meetings · Sat 10 Oct ─", nil},
	}
	for _, s := range steps {
		m = press(m, s.key)
		viewHas(t, m, s.title)
		if got := m.PaletteRows(); !reflect.DeepEqual(got, s.rows) {
			t.Errorf("after %s: rows = %q, want %q", s.key, got, s.rows)
		}
	}
	viewHas(t, m, "no Meetings")
}

func TestEndedMeetingsAreDimmed(t *testing.T) {
	// meetingDay is 08:00.
	m := meetingsApp(t, "[[calendars]]\nname = \"work\"\ncolor = \"lightcoral\"\nurl = \""+feed(t, 200, vcalendar(
		vevent("early@example.test", "Early call", 7, 0, 30),
		vevent("now@example.test", "Running late", 7, 30, 60),
		vevent("later@example.test", "Standup", 9, 30, 15),
	))+"\"\n")
	m = openMeetings(t, m)

	if got, want := m.PaletteSlots(), []string{"overlay.hint", "", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("slots = %q, want %q", got, want)
	}
	if got, want := m.PaletteDetailStyles(), []string{"overlay.hint", "#f08080", "#f08080"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Detail styles = %q, want %q", got, want)
	}
}

func TestCalendarColourComesFromConfigThenTheFeed(t *testing.T) {
	colored := func(color, uid string, hh int) string {
		return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//pholio//test//EN\r\n" + color +
			vevent(uid, uid, hh, 0, 30) + "END:VCALENDAR\r\n"
	}
	cfg := "[[calendars]]\nname = \"a\"\ncolor = \"#1e90ff\"\nurl = \"" + feed(t, 200, colored("COLOR:steelblue\r\n", "a", 9)) + "\"\n" +
		"[[calendars]]\nname = \"b\"\nurl = \"" + feed(t, 200, colored("COLOR:steelblue\r\n", "b", 10)) + "\"\n" +
		"[[calendars]]\nname = \"c\"\nurl = \"" + feed(t, 200, colored("COLOR:notacolour\r\n", "c", 11)) + "\"\n" +
		"[[calendars]]\nname = \"d\"\nurl = \"" + feed(t, 200, colored("X-APPLE-CALENDAR-COLOR:#ff0000\r\n", "d", 12)) + "\"\n"
	m := openMeetings(t, meetingsApp(t, cfg))

	want := []string{"#1e90ff", "#4682b4", "overlay.hint", "overlay.hint"}
	if got := m.PaletteDetailStyles(); !reflect.DeepEqual(got, want) {
		t.Errorf("Detail styles = %q, want %q", got, want)
	}
}

func TestLongTitlesTruncateBeforeTheCalendar(t *testing.T) {
	long := strings.Repeat("Quarterly planning ", 8)
	m := openMeetings(t, meetingsApp(t, calendarsConfig("work", feed(t, 200, vcalendar(
		vevent("a@example.test", "Standup", 9, 0, 15),
		vevent("b@example.test", long, 10, 0, 60),
	)))))

	viewHas(t, m, "10:00–11:00  Quarterly planning")
	viewHas(t, m, "… work")
}

func TestThePickerHintListsItsKeys(t *testing.T) {
	m := openMeetings(t, meetingsApp(t, calendarsConfig("work", feed(t, 200, vcalendar()))))

	viewHas(t, m, "h/l day · t today · r refresh · enter open/create · esc close")
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

func TestStartupFetchesSoThePickerOpensWithMeetings(t *testing.T) {
	work := newLiveFeed(t, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))
	m := meetingsApp(t, calendarsConfig("work", work.url))

	m = drive(t, m, m.Init())
	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})

	if got := m.PaletteRows(); len(got) != 1 {
		t.Errorf("rows = %q, want the Meeting fetched at startup", got)
	}
	drive(t, m, cmd)
	if n := work.hits.Load(); n != 1 {
		t.Errorf("fetched %d times, want once at startup only", n)
	}
}

func TestThePickerShowsTheCacheAndSaysWhenOffline(t *testing.T) {
	cache := t.TempDir()
	work := newLiveFeed(t, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))
	cfg := calendarsConfig("work", work.url)
	m := meetingsAppAt(t, cache, seamtest.NewClock(meetingDay), cfg)
	drive(t, m, m.Init())
	setFetched(t, cache, meetingDay.Add(-2*time.Hour))
	work.set(401, "")

	m = meetingsAppAt(t, cache, seamtest.NewClock(meetingDay), cfg)
	m = drive(t, m, m.Init())
	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	m = drive(t, m, cmd)
	if got := m.PaletteRows(); len(got) != 1 {
		t.Errorf("rows = %q, want the cached Meeting kept after a failed fetch", got)
	}
	viewHas(t, m, "work: offline, showing data from 2h ago")
}

func TestOpeningRefreshesStaleMeetingsInPlace(t *testing.T) {
	work := newLiveFeed(t, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))
	clock := seamtest.NewClock(meetingDay)
	m := meetingsAppAt(t, t.TempDir(), clock, calendarsConfig("work", work.url))
	m = drive(t, m, m.Init())
	work.set(200, vcalendar(
		vevent("a@example.test", "Standup", 9, 30, 15),
		vevent("b@example.test", "Review", 14, 0, 60),
	))
	clock.Advance(16 * time.Minute)

	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	if got := m.PaletteRows(); len(got) != 1 {
		t.Errorf("rows = %q, want the old list at once", got)
	}
	m = drive(t, m, cmd)

	if got := m.PaletteRows(); len(got) != 2 {
		t.Errorf("rows = %q, want the refreshed list", got)
	}
}

func TestRRefreshesEvenWhenFresh(t *testing.T) {
	work := newLiveFeed(t, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))
	m := meetingsApp(t, calendarsConfig("work", work.url))
	m = drive(t, m, m.Init())
	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	m = drive(t, m, cmd)
	work.set(200, vcalendar())

	m, cmd = pressing(m, tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = drive(t, m, cmd)

	viewHas(t, m, "no Meetings")
}

func TestStartupFailuresWaitForThePicker(t *testing.T) {
	m := meetingsApp(t, calendarsConfig("work", feed(t, 401, "")))

	m = drive(t, m, m.Init())

	if got := messageLine(m); got != "" {
		t.Errorf("message = %q, want nothing until the picker opens", got)
	}
	m, _ = pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	viewHas(t, m, "work: HTTP 401")
}

func TestStartupFetchesEvenAFreshCache(t *testing.T) {
	cache := t.TempDir()
	work := newLiveFeed(t, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))
	cfg := calendarsConfig("work", work.url)
	m := meetingsAppAt(t, cache, seamtest.NewClock(meetingDay), cfg)
	drive(t, m, m.Init())
	setFetched(t, cache, meetingDay.Add(-time.Minute))

	m = meetingsAppAt(t, cache, seamtest.NewClock(meetingDay), cfg)
	drive(t, m, m.Init())

	if n := work.hits.Load(); n != 2 {
		t.Errorf("fetched %d times, want once per start", n)
	}
}

func TestOpeningRetriesAFailedFetch(t *testing.T) {
	work := newLiveFeed(t, "")
	work.set(503, "")
	m := meetingsApp(t, calendarsConfig("work", work.url))
	m = drive(t, m, m.Init())
	work.set(200, vcalendar(vevent("a@example.test", "Standup", 9, 30, 15)))

	m, cmd := pressing(press(m, space), tea.KeyPressMsg{Code: 'm', Text: "m"})
	m = drive(t, m, cmd)

	if got := m.PaletteRows(); len(got) != 1 {
		t.Errorf("rows = %q, want the Meeting from the retried fetch", got)
	}
}
