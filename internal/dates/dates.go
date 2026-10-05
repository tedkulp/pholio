// Package dates holds pholio's calendar helpers: Today, the relative-date
// parser shared by Daily Notes and Tasks, and moment-style formatting.
package dates

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ISO is the layout of a date in a Daily Note's name and in Task metadata.
const ISO = "2006-01-02"

// Today is the day pholio counts as today at now: the calendar day, or the
// day before when now is earlier than startsAt after midnight. It is
// midnight of that day in now's location.
func Today(now time.Time, startsAt time.Duration) time.Time {
	d := Midnight(now)
	h, m, s := now.Clock()
	sinceMidnight := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
	if sinceMidnight < startsAt {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

// relative is +Nd, +Nw, -Nd or -Nw.
var relative = regexp.MustCompile(`^([+-])(\d{1,4})([dw])$`)

// Parse reads a date relative to today: an ISO date (2026-10-05), today,
// tomorrow, yesterday, a weekday name or its first three letters (the
// next such day, never today), or an offset of days or weeks (+3d, +1w,
// -2d). Case and surrounding space are ignored. The result is midnight in
// today's location.
func Parse(s string, today time.Time) (time.Time, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	today = Midnight(today)
	switch s {
	case "today":
		return today, true
	case "tomorrow":
		return today.AddDate(0, 0, 1), true
	case "yesterday":
		return today.AddDate(0, 0, -1), true
	}
	if d, err := time.ParseInLocation(ISO, s, today.Location()); err == nil {
		return d, true
	}
	if m := relative.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[2])
		if m[1] == "-" {
			n = -n
		}
		if m[3] == "w" {
			n *= 7
		}
		return today.AddDate(0, 0, n), true
	}
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		name := strings.ToLower(wd.String())
		if s == name || s == name[:3] {
			ahead := (int(wd)-int(today.Weekday())+6)%7 + 1
			return today.AddDate(0, 0, ahead), true
		}
	}
	return time.Time{}, false
}

// Midnight is the start of t's calendar day in t's location.
func Midnight(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
}
