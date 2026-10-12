// Package calendar fetches ICS calendar feeds and reads their Meetings.
// See docs/adr/0002-calendar-source-is-ics-feeds.md.
package calendar

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

// Meeting is one timed Meeting from a feed.
type Meeting struct {
	UID   string
	Title string
	Start time.Time
	End   time.Time
	// CalendarColor is the feed's calendar-level COLOR (RFC 7986, a CSS
	// colour name) as written, "" if none.
	CalendarColor string
}

// Parse reads the Meetings of an ICS body. All-day Meetings
// (DTSTART;VALUE=DATE) and Meetings whose times can't be read are left
// out.
//
// ponytail: recurrence (RRULE/EXDATE/RECURRENCE-ID) and VTIMEZONE-only
// TZIDs are not handled yet; a recurring Meeting shows on its first day only.
func Parse(body []byte) ([]Meeting, error) {
	cal, err := ics.ParseCalendarWithOptions(bytes.NewReader(body), ics.WithWindowsTimezoneMapping())
	if err != nil {
		return nil, err
	}
	color := ""
	for _, p := range cal.CalendarProperties {
		if p.IANAToken == string(ics.PropertyColor) {
			color = strings.TrimSpace(p.Value)
		}
	}
	var ms []Meeting
	for _, ev := range cal.Events() {
		if p := ev.GetProperty(ics.ComponentPropertyDtStart); p == nil || slices.Contains(p.ICalParameters["VALUE"], "DATE") {
			continue
		}
		start, err := ev.GetStartAt()
		if err != nil {
			continue
		}
		end, err := ev.GetEndAt()
		if err != nil {
			end = start
		}
		m := Meeting{UID: ev.Id(), Start: start, End: end, CalendarColor: color}
		if p := ev.GetProperty(ics.ComponentPropertySummary); p != nil {
			m.Title = p.Value
		}
		ms = append(ms, m)
	}
	return ms, nil
}

// Timeout is how long one feed's fetch may take.
const Timeout = 15 * time.Second

// Fetch downloads the feed at url and parses its Meetings. Its errors are
// short enough for the picker: "HTTP 401", "timed out", "not a calendar".
func Fetch(client *http.Client, url string) ([]Meeting, error) {
	resp, err := client.Get(url)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var body []byte
		if body, err = io.ReadAll(resp.Body); err == nil {
			body = bytes.TrimSpace(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")))
			ms, err := Parse(body)
			if err != nil || !bytes.HasPrefix(body, []byte("BEGIN:VCALENDAR")) {
				return nil, errors.New("not a calendar")
			}
			return ms, nil
		}
	}
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return nil, errors.New("timed out")
	}
	return nil, err
}

// On is the Meetings that start on day (midnight to midnight in day's
// location), sorted by start.
func On(ms []Meeting, day time.Time) []Meeting {
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	to := from.AddDate(0, 0, 1)
	var out []Meeting
	for _, m := range ms {
		if !m.Start.Before(from) && m.Start.Before(to) {
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, b Meeting) int { return cmp.Compare(a.Start.UnixNano(), b.Start.UnixNano()) })
	return out
}
