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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"
)

// Meeting is one timed Meeting from a feed. A recurring Meeting from Parse
// stands for its whole series until On expands it into occurrences.
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
	// Original is the occurrence's original start in UTC: its
	// RECURRENCE-ID when it was moved, else its start. UID and Original
	// together are the Meeting's identity.
	Original time.Time

	series *series // set on a recurring Meeting until On expands it
}

// series is a recurring Meeting's occurrences, kept as wall-clock times
// (in UTC, ignoring zones) in its DTSTART zone, so they keep their clock
// time across DST changes.
type series struct {
	set  *rrule.Set
	zone zone
}

// Parse reads the Meetings of an ICS body. All-day Meetings
// (DTSTART;VALUE=DATE) and Meetings whose times can't be read are left
// out. A TZID that can't be resolved is read as local time.
func Parse(body []byte) ([]Meeting, error) {
	cal, err := ics.ParseCalendar(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	color := ""
	for _, p := range cal.CalendarProperties {
		if p.IANAToken == string(ics.PropertyColor) {
			color = strings.TrimSpace(p.Value)
		}
	}
	zones := map[string]vtimezone{}
	for _, tz := range cal.Timezones() {
		if p := tz.GetProperty(ics.ComponentPropertyTzid); p != nil {
			zones[p.Value] = readVTimezone(tz)
		}
	}
	var ms []Meeting
	moved := map[string][]time.Time{} // UID -> RECURRENCE-ID instants
	for _, ev := range cal.Events() {
		sp := ev.GetProperty(ics.ComponentPropertyDtStart)
		if sp == nil || slices.Contains(sp.ICalParameters["VALUE"], "DATE") {
			continue
		}
		walls, z := readTimes(sp, zones)
		if len(walls) != 1 {
			continue
		}
		start := z.at(walls[0])
		end := start
		if p := ev.GetProperty(ics.ComponentPropertyDtEnd); p != nil {
			if ws, ez := readTimes(p, zones); len(ws) == 1 {
				end = ez.at(ws[0])
			}
		} else if p := ev.GetProperty(ics.ComponentPropertyDuration); p != nil {
			if d, ok := duration(p.Value); ok {
				end = start.Add(d)
			}
		}
		m := Meeting{UID: ev.Id(), Start: start, End: end, Original: start.UTC(), CalendarColor: color}
		if p := ev.GetProperty(ics.ComponentPropertySummary); p != nil {
			m.Title = p.Value
		}
		m.details(ev)
		if p := ev.GetProperty(ics.ComponentPropertyRecurrenceId); p != nil {
			if ws, rz := readTimes(p, zones); len(ws) == 1 {
				m.Original = rz.at(ws[0]).UTC()
				moved[m.UID] = append(moved[m.UID], m.Original)
			}
			if p := ev.GetProperty(ics.ComponentPropertyStatus); p != nil && p.Value == "CANCELLED" {
				continue
			}
		} else {
			m.series = readSeries(ev, walls[0], z, zones)
		}
		ms = append(ms, m)
	}
	for _, m := range ms {
		if m.series != nil {
			for _, t := range moved[m.UID] {
				m.series.set.ExDate(m.series.zone.wall(t))
			}
		}
	}
	return ms, nil
}

// DateTime is the layout of an ICS DATE-TIME value, less any trailing Z.
const DateTime = "20060102T150405"

var untilUTC = regexp.MustCompile(`UNTIL=\d{8}T\d{6}Z`)

// readSeries is ev's recurrence starting at wall in z, or nil when ev
// doesn't recur.
func readSeries(ev *ics.VEvent, wall time.Time, z zone, zones map[string]vtimezone) *series {
	rp := ev.GetProperty(ics.ComponentPropertyRrule)
	rdates := ev.GetProperties(ics.ComponentPropertyRdate)
	if rp == nil && len(rdates) == 0 {
		return nil
	}
	s := &series{set: &rrule.Set{}, zone: z}
	s.set.DTStart(wall)
	if rp == nil {
		s.set.RDate(wall)
	} else if r := readRRule(rp.Value, wall, z); r != nil {
		s.set.RRule(r)
	}
	for _, p := range rdates {
		ws, pz := readTimes(p, zones)
		for _, w := range ws {
			s.set.RDate(z.wall(pz.at(w)))
		}
	}
	for _, p := range ev.GetProperties(ics.ComponentPropertyExdate) {
		ws, pz := readTimes(p, zones)
		for _, w := range ws {
			s.set.ExDate(z.wall(pz.at(w)))
		}
	}
	return s
}

// readRRule is the RRULE v starting at wall-clock time start in z, or nil
// when it can't be read.
func readRRule(v string, start time.Time, z zone) *rrule.RRule {
	opt, err := rrule.StrToROptionInLocation(v, time.UTC)
	if err != nil {
		return nil
	}
	if untilUTC.MatchString(v) {
		opt.Until = z.wall(opt.Until)
	}
	opt.Dtstart = start
	r, err := rrule.NewRRule(*opt)
	if err != nil {
		return nil
	}
	return r
}

// between is the series' starts in [from, to).
func (s *series) between(from, to time.Time) []time.Time {
	// A day either side covers any gap between the zone's wall clock and
	// the window's.
	var out []time.Time
	for _, w := range s.set.Between(s.zone.wall(from).AddDate(0, 0, -1), s.zone.wall(to).AddDate(0, 0, 1), true) {
		if t := s.zone.at(w); !t.Before(from) && t.Before(to) {
			out = append(out, t)
		}
	}
	return out
}

// readTimes is a DATE-TIME property's values as wall-clock times (in UTC,
// ignoring zones), and the zone they're in. Unreadable values are
// skipped.
func readTimes(p *ics.IANAProperty, zones map[string]vtimezone) ([]time.Time, zone) {
	z := zone{loc: time.Local}
	if tzid := p.ICalParameters["TZID"]; len(tzid) == 1 {
		z = resolve(tzid[0], zones)
	}
	var out []time.Time
	for v := range strings.SplitSeq(p.Value, ",") {
		utc := strings.HasSuffix(v, "Z")
		w, err := time.Parse(DateTime, strings.TrimSuffix(v, "Z"))
		if err != nil {
			continue
		}
		if utc {
			// A UTC value is converted into the property's zone so that
			// every value shares it.
			w = z.wall(w)
		}
		out = append(out, w)
	}
	return out, z
}

// resolve finds a TZID: Go's tz database, then the Windows names, then the
// feed's own VTIMEZONE, else local time.
func resolve(tzid string, zones map[string]vtimezone) zone {
	if loc, err := time.LoadLocation(tzid); err == nil {
		return zone{loc: loc}
	}
	if loc := ics.WindowsTimezoneToIANA(tzid); loc != nil {
		return zone{loc: loc}
	}
	if vt, ok := zones[tzid]; ok && len(vt) > 0 {
		return zone{vt: vt}
	}
	return zone{loc: time.Local}
}

// zone is a Go location or, when loc is nil, a feed's VTIMEZONE.
type zone struct {
	loc *time.Location
	vt  vtimezone
}

// at is the instant of wall-clock time w in z.
func (z zone) at(w time.Time) time.Time {
	if z.loc != nil {
		return time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), 0, z.loc)
	}
	return w.Add(-z.vt.offset(w))
}

// wall is t's wall-clock time in z (in UTC, ignoring zones).
func (z zone) wall(t time.Time) time.Time {
	if z.loc != nil {
		t = t.In(z.loc)
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	}
	// ponytail: the offset is looked up at t's UTC clock, so wall times
	// within a few hours of a transition can be an hour out.
	return t.UTC().Add(z.vt.offset(t.UTC()))
}

// vtimezone is a VTIMEZONE's STANDARD and DAYLIGHT parts.
type vtimezone []tzPart

// tzPart is one STANDARD or DAYLIGHT part: when it begins (wall-clock
// times) and the UTC offsets before and after.
type tzPart struct {
	onsets   *rrule.Set
	from, to time.Duration
}

func readVTimezone(tz *ics.VTimezone) vtimezone {
	var vt vtimezone
	for _, c := range tz.SubComponents() {
		var cb *ics.ComponentBase
		switch c := c.(type) {
		case *ics.Standard:
			cb = &c.ComponentBase
		case *ics.Daylight:
			cb = &c.ComponentBase
		default:
			continue
		}
		from, ok1 := offset(cb.GetProperty(ics.ComponentProperty(ics.PropertyTzoffsetfrom)))
		to, ok2 := offset(cb.GetProperty(ics.ComponentProperty(ics.PropertyTzoffsetto)))
		sp := cb.GetProperty(ics.ComponentPropertyDtStart)
		if !ok1 || !ok2 || sp == nil {
			continue
		}
		start, err := time.Parse(DateTime, sp.Value)
		if err != nil {
			continue
		}
		// rrule-go stops about 290 years after DTSTART, and Outlook starts
		// its parts in 1601; transitions before 1970 don't matter here.
		if start.Year() < 1970 {
			start = start.AddDate(1970-start.Year(), 0, 0)
		}
		set := &rrule.Set{}
		set.DTStart(start)
		set.RDate(start)
		if rp := cb.GetProperty(ics.ComponentPropertyRrule); rp != nil {
			if r := readRRule(rp.Value, start, zone{loc: time.UTC}); r != nil {
				set.RRule(r)
			}
		}
		for _, p := range cb.GetProperties(ics.ComponentPropertyRdate) {
			for v := range strings.SplitSeq(p.Value, ",") {
				if t, err := time.Parse(DateTime, v); err == nil {
					set.RDate(t)
				}
			}
		}
		vt = append(vt, tzPart{set, from, to})
	}
	return vt
}

// offset is the UTC offset in force at wall-clock time w: the TZOFFSETTO
// of the part that began last, else the first part's TZOFFSETFROM.
func (vt vtimezone) offset(w time.Time) time.Duration {
	var last time.Time
	off := vt[0].from
	for _, p := range vt {
		if t := p.onsets.Before(w, true); !t.IsZero() && t.After(last) {
			last, off = t, p.to
		}
	}
	return off
}

var offsetRE = regexp.MustCompile(`^([+-])(\d\d)(\d\d)(\d\d)?$`)

// offset reads a TZOFFSETFROM/TZOFFSETTO value such as -0500.
func offset(p *ics.IANAProperty) (time.Duration, bool) {
	if p == nil {
		return 0, false
	}
	m := offsetRE.FindStringSubmatch(p.Value)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[2])
	mi, _ := strconv.Atoi(m[3])
	s, _ := strconv.Atoi(m[4] + "0")
	d := time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute + time.Duration(s/10)*time.Second
	if m[1] == "-" {
		d = -d
	}
	return d, true
}

var durationRE = regexp.MustCompile(`^([+-])?P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

// duration reads a DURATION value such as PT45M or P1DT2H.
func duration(v string) (time.Duration, bool) {
	m := durationRE.FindStringSubmatch(v)
	if m == nil {
		return 0, false
	}
	var d time.Duration
	for i, unit := range []time.Duration{7 * 24 * time.Hour, 24 * time.Hour, time.Hour, time.Minute, time.Second} {
		n, _ := strconv.Atoi(m[i+2])
		d += time.Duration(n) * unit
	}
	if m[1] == "-" {
		d = -d
	}
	return d, true
}

// Timeout is how long one feed's fetch may take.
const Timeout = 15 * time.Second

// Fetch downloads the feed at url and parses its Meetings. Its errors are
// short enough for the picker: "HTTP 401", "timed out", "not a calendar",
// "offline". They never hold the URL, which is a secret.
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
	return nil, errors.New("offline")
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
		if m.series == nil {
			if !m.Start.Before(from) && m.Start.Before(to) {
				out = append(out, m)
			}
			continue
		}
		for _, start := range m.series.between(from, to) {
			o := m
			o.series = nil
			o.Start, o.End, o.Original = start, start.Add(m.End.Sub(m.Start)), start.UTC()
			out = append(out, o)
		}
	}
	slices.SortStableFunc(out, func(a, b Meeting) int { return cmp.Compare(a.Start.UnixNano(), b.Start.UnixNano()) })
	return out
}
