// Package calendar fetches ICS calendar feeds and reads their Meetings.
// See docs/adr/0002-calendar-source-is-ics-feeds.md.
package calendar

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

	Attendees   []string // see details.go
	Location    string
	Link        string
	Description string
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
		m.details(ev)
		ms = append(ms, m)
	}
	return ms, nil
}

// Timeout is how long one feed's fetch may take.
const Timeout = 15 * time.Second

// Fetch downloads the feed at url and parses its Meetings. Its errors are
// short enough for the picker: "HTTP 401", "timed out", "not a calendar".
// With a cache dir, a body that parses replaces url's cache file there; a
// bad one leaves it alone.
func Fetch(client *http.Client, url, dir string) ([]Meeting, error) {
	resp, err := client.Get(url)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var body []byte
		if body, err = io.ReadAll(resp.Body); err == nil {
			ms, err := parseFeed(body)
			if err == nil && dir != "" {
				// The cache only speeds up the next start; failing to
				// write it doesn't fail the fetch.
				_ = writeCache(cachePath(dir, url), body)
			}
			return ms, err
		}
	}
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return nil, errors.New("timed out")
	}
	return nil, err
}

// parseFeed is Parse for a body that must be a calendar.
func parseFeed(body []byte) ([]Meeting, error) {
	body = bytes.TrimSpace(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")))
	ms, err := Parse(body)
	if err != nil || !bytes.HasPrefix(body, []byte("BEGIN:VCALENDAR")) {
		return nil, errors.New("not a calendar")
	}
	return ms, nil
}

// Cached is url's Meetings from the cache in dir, and when they were
// fetched.
func Cached(dir, url string) ([]Meeting, time.Time, error) {
	path := cachePath(dir, url)
	fi, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	ms, err := parseFeed(body)
	return ms, fi.ModTime(), err
}

// cachePath is url's cache file in dir. Feed URLs are secrets, so the
// name is a hash.
func cachePath(dir, url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(dir, hex.EncodeToString(sum[:8])+".ics")
}

// writeCache replaces path with body, readable by the user only. The
// file's mtime is the fetch time.
func writeCache(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".fetch-*") // mode 0600
	if err != nil {
		return err
	}
	_, err = f.Write(body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
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
