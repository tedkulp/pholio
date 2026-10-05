package dates_test

import (
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/dates"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func day(s string) time.Time { return at(s + " 00:00") }

func TestTodayBeforeTheDayStartsIsStillYesterday(t *testing.T) {
	got := dates.Today(at("2026-10-05 00:30"), time.Hour)

	if want := day("2026-10-04"); !got.Equal(want) {
		t.Errorf("Today = %v, want %v", got, want)
	}
}

func TestTodayFromTheDayStartOnIsTheCalendarDay(t *testing.T) {
	for _, now := range []string{"2026-10-05 01:00", "2026-10-05 23:59"} {
		got := dates.Today(at(now), time.Hour)
		if want := day("2026-10-05"); !got.Equal(want) {
			t.Errorf("Today(%s) = %v, want %v", now, got, want)
		}
	}
}

func TestParseRelativeAndISODates(t *testing.T) {
	today := day("2026-10-05") // a Monday
	for in, want := range map[string]string{
		"today":      "2026-10-05",
		"Today":      "2026-10-05",
		" tomorrow ": "2026-10-06",
		"yesterday":  "2026-10-04",
		"tuesday":    "2026-10-06",
		"friday":     "2026-10-09",
		"fri":        "2026-10-09",
		"sunday":     "2026-10-11",
		"monday":     "2026-10-12", // the next one, never today
		"+3d":        "2026-10-08",
		"+1w":        "2026-10-12",
		"+0d":        "2026-10-05",
		"-2d":        "2026-10-03",
		"-1w":        "2026-09-28",
		"+30d":       "2026-11-04",
		"2026-02-28": "2026-02-28",
	} {
		got, ok := dates.Parse(in, today)
		if !ok {
			t.Errorf("Parse(%q) failed, want %s", in, want)
			continue
		}
		if !got.Equal(day(want)) {
			t.Errorf("Parse(%q) = %s, want %s", in, got.Format(dates.ISO), want)
		}
	}
}

func TestParseRejectsOtherText(t *testing.T) {
	today := day("2026-10-05")
	for _, in := range []string{"", "soon", "2026-02-30", "2026-2-3", "+d", "+3", "+3m", "3d", "frid", "+-3d"} {
		if got, ok := dates.Parse(in, today); ok {
			t.Errorf("Parse(%q) = %s, want failure", in, got.Format(dates.ISO))
		}
	}
}

func TestFormatMomentTokens(t *testing.T) {
	afternoon := time.Date(2026, 10, 5, 14, 7, 9, 0, time.UTC) // a Monday
	morning := time.Date(2026, 3, 1, 0, 5, 3, 0, time.UTC)     // a Sunday
	for _, c := range []struct {
		format    string
		afternoon string
		morning   string
	}{
		{"YYYY", "2026", "2026"},
		{"YY", "26", "26"},
		{"Q", "4", "1"},
		{"MMMM", "October", "March"},
		{"MMM", "Oct", "Mar"},
		{"MM", "10", "03"},
		{"M", "10", "3"},
		{"DDDD", "278", "060"},
		{"DDD", "278", "60"},
		{"DD", "05", "01"},
		{"D", "5", "1"},
		{"Do", "5th", "1st"},
		{"dddd", "Monday", "Sunday"},
		{"ddd", "Mon", "Sun"},
		{"dd", "Mo", "Su"},
		{"d", "1", "0"},
		{"E", "1", "7"},
		{"WW", "41", "09"},
		{"W", "41", "9"},
		{"GGGG", "2026", "2026"},
		{"HH", "14", "00"},
		{"H", "14", "0"},
		{"hh", "02", "12"},
		{"h", "2", "12"},
		{"mm", "07", "05"},
		{"m", "7", "5"},
		{"ss", "09", "03"},
		{"s", "9", "3"},
		{"A", "PM", "AM"},
		{"a", "pm", "am"},
		{"X", "1791209229", "1772323503"},
		{"[Week] W, YYYY", "Week 41, 2026", "Week 9, 2026"},
		{"dddd, MMMM Do YYYY", "Monday, October 5th 2026", "Sunday, March 1st 2026"},
		{"YYYY-MM-DD HH:mm", "2026-10-05 14:07", "2026-03-01 00:05"},
		{"[at] ,./-:", "at ,./-:", "at ,./-:"},
	} {
		if got := dates.Format(afternoon, c.format); got != c.afternoon {
			t.Errorf("Format(afternoon, %q) = %q, want %q", c.format, got, c.afternoon)
		}
		if got := dates.Format(morning, c.format); got != c.morning {
			t.Errorf("Format(morning, %q) = %q, want %q", c.format, got, c.morning)
		}
	}
}

func TestFormatOrdinals(t *testing.T) {
	for d, want := range map[int]string{1: "1st", 2: "2nd", 3: "3rd", 4: "4th", 11: "11th", 12: "12th", 13: "13th", 21: "21st", 22: "22nd", 23: "23rd", 31: "31st"} {
		if got := dates.Format(time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC), "Do"); got != want {
			t.Errorf("Do for day %d = %q, want %q", d, got, want)
		}
	}
}

func TestTodayWithMidnightStart(t *testing.T) {
	got := dates.Today(at("2026-10-05 00:00"), 0)
	if want := day("2026-10-05"); !got.Equal(want) {
		t.Errorf("Today = %v, want %v", got, want)
	}
}
