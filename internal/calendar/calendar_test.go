package calendar_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/calendar"
)

func newYork(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// clock is a Meeting as "HH:mm–HH:mm title" in loc.
func clock(ms []calendar.Meeting, loc *time.Location) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Start.In(loc).Format("15:04")+"–"+m.End.In(loc).Format("15:04")+" "+m.Title)
	}
	return out
}

func TestOnListsTheDaysTimedMeetingsByStart(t *testing.T) {
	body, err := os.ReadFile("testdata/day.ics")
	if err != nil {
		t.Fatal(err)
	}
	ms, err := calendar.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	ny := newYork(t)

	got := clock(calendar.On(ms, time.Date(2026, 10, 12, 0, 0, 0, 0, ny)), ny)

	want := []string{"09:30–09:45 Standup", "14:00–15:00 Design review"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Meetings = %q, want %q", got, want)
	}
}

func serve(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s.URL
}

func TestParseGivesEachMeetingTheCalendarLevelColor(t *testing.T) {
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//pholio//test//EN\r\nCOLOR:lightcoral\r\n" +
		"X-APPLE-CALENDAR-COLOR:#0000ff\r\n" +
		"BEGIN:VEVENT\r\nUID:a@example.test\r\nDTSTAMP:20261001T000000Z\r\nDTSTART:20261012T130000Z\r\nSUMMARY:Standup\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	ms, err := calendar.Parse([]byte(body))

	if err != nil || len(ms) != 1 || ms[0].CalendarColor != "lightcoral" {
		t.Errorf("Parse = %+v, %v; want one Meeting with CalendarColor lightcoral", ms, err)
	}
}

func TestFetchReadsTheFeedsMeetings(t *testing.T) {
	body, err := os.ReadFile("testdata/day.ics")
	if err != nil {
		t.Fatal(err)
	}
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })

	ms, err := calendar.Fetch(http.DefaultClient, url, "")

	if err != nil || len(ms) != 3 {
		t.Errorf("Fetch = %d Meetings, %v; want 3, nil", len(ms), err)
	}
}

func TestFetchFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"401", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }, "HTTP 401"},
		{"not ICS", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html><body>Sign in</body></html>\n"))
		}, "not a calendar"},
		{"empty", func(http.ResponseWriter, *http.Request) {}, "not a calendar"},
		{"timeout", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, "timed out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := serve(t, tt.handler)

			_, err := calendar.Fetch(&http.Client{Timeout: 50 * time.Millisecond}, url, "")

			if err == nil || err.Error() != tt.want {
				t.Errorf("Fetch error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFetchCachesTheBodyPrivatelyKeyedByURL(t *testing.T) {
	body, err := os.ReadFile("testdata/day.ics")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "calendars")
	h := func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }
	a, b := serve(t, h), serve(t, h)

	for _, url := range []string{a, b} {
		if _, err := calendar.Fetch(http.DefaultClient, url, dir); err != nil {
			t.Fatal(err)
		}
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.ics"))
	if len(files) != 2 {
		t.Fatalf("cache files = %q, want one per URL", files)
	}
	for _, f := range files {
		if fi, err := os.Stat(f); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s: mode %v, %v; want 0600", f, fi.Mode().Perm(), err)
		}
	}
	ms, at, err := calendar.Cached(dir, a)
	if err != nil || len(ms) != 3 || time.Since(at) > time.Minute {
		t.Errorf("Cached = %d Meetings at %v, %v; want 3 just now", len(ms), at, err)
	}
}

func TestABadFetchKeepsTheCache(t *testing.T) {
	body, err := os.ReadFile("testdata/day.ics")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var bad atomic.Bool
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		if !bad.Load() {
			_, _ = w.Write(body)
			return
		}
		_, _ = w.Write([]byte("<html>Sign in</html>"))
	})
	if _, err := calendar.Fetch(http.DefaultClient, url, dir); err != nil {
		t.Fatal(err)
	}
	bad.Store(true)

	if _, err := calendar.Fetch(http.DefaultClient, url, dir); err == nil {
		t.Fatal("a bad body fetched without error")
	}

	if ms, _, err := calendar.Cached(dir, url); err != nil || len(ms) != 3 {
		t.Errorf("Cached = %d Meetings, %v; want the 3 from the good fetch", len(ms), err)
	}
}

func TestNothingCached(t *testing.T) {
	if _, _, err := calendar.Cached(t.TempDir(), "https://example.test/x.ics"); err == nil {
		t.Error("Cached found a feed never fetched")
	}
}

func recurring(t *testing.T) []calendar.Meeting {
	t.Helper()
	body, err := os.ReadFile("testdata/recurring.ics")
	if err != nil {
		t.Fatal(err)
	}
	ms, err := calendar.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

func TestMovedOccurrenceKeepsItsOriginalStart(t *testing.T) {
	var got []time.Time
	for _, m := range onDay(t, time.November, 10) {
		if m.UID == "standup@example.test" {
			got = append(got, m.Original)
		}
	}

	want := []time.Time{time.Date(2026, 11, 10, 14, 30, 0, 0, time.UTC)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Original = %v, want %v", got, want)
	}
}

func TestOccurrenceIdentityIsItsStartInUTC(t *testing.T) {
	ms := onDay(t, time.November, 3)
	if len(ms) != 2 || !ms[1].Original.Equal(ms[1].Start) || ms[1].Original.Location() != time.UTC {
		t.Errorf("Meetings = %+v, want the Standup's Original to be its start in UTC", ms)
	}
}

func TestUnresolvableTZIDIsLocalTime(t *testing.T) {
	ms := calendar.On(recurring(t), time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local))

	if got := clock(ms, time.Local); !reflect.DeepEqual(got, []string{"16:00–17:00 Floating chat"}) {
		t.Errorf("Meetings = %q, want the floating chat at 16:00 local", got)
	}
}

// onDay is testdata/recurring.ics's Meetings on a New York day.
func onDay(t *testing.T, mo time.Month, d int) []calendar.Meeting {
	t.Helper()
	return calendar.On(recurring(t), time.Date(2026, mo, d, 0, 0, 0, 0, newYork(t)))
}

func TestRecurringMeetingsShowOnEachDayTheyOccur(t *testing.T) {
	tests := []struct {
		name  string
		month time.Month
		day   int
		want  []string
	}{
		{"BYDAY ordinal", time.December, 8, []string{
			"08:00–08:30 Early sync", "09:30–09:45 Standup", "11:00–12:00 Monthly review",
		}},
		{"BYSETPOS: last weekday of the month", time.November, 30, []string{"16:00–16:30 Month-end wrap-up"}},
		{"BYSETPOS: not the last weekday", time.November, 27, nil},
		{"comma EXDATE, first value", time.October, 20, nil},
		{"comma EXDATE, second value; VTIMEZONE in daylight time", time.October, 27, []string{"08:00–08:30 Early sync"}},
		{"across DST: VTIMEZONE in standard time", time.November, 3, []string{
			"08:00–08:30 Early sync", "09:30–09:45 Standup",
		}},
		{"moved occurrence, Windows TZID, DURATION", time.November, 10, []string{
			"08:00–08:30 Early sync", "11:00–12:00 Monthly review", "12:00–12:30 Outlook lunch",
			"14:00–14:15 Standup (moved)", "17:00–17:45 Retro",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clock(onDay(t, tt.month, tt.day), newYork(t))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Meetings = %q, want %q", got, tt.want)
			}
		})
	}
}
