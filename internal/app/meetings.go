package app

import (
	"cmp"
	"image/color"
	"net/http"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/calendar"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

// meetingState is shared by an open Meeting picker and its feed fetches.
type meetingState struct {
	open    bool         // the picker is still up
	day     time.Time    // the day shown
	pending int          // feeds still being fetched
	rows    []meetingRow // every fetched Meeting, on any day
	failed  []string     // "name: why", one per failing calendar
}

// meetingRow is a Meeting, the name of the calendar it came from and that
// calendar's colour (nil for overlay.hint).
type meetingRow struct {
	calendar.Meeting
	cal   string
	color color.Color
}

// meetingsMsg is one calendar's fetch result.
type meetingsMsg struct {
	st    *meetingState
	cal   string
	color string // the calendar's configured colour, "" if none
	ms    []calendar.Meeting
	err   error
}

// showMeetings (spc m, :meetings) opens the Meeting picker on Today and
// fetches every calendar, one goroutine each.
func (m Model) showMeetings() (Model, tea.Cmd) {
	cals := m.config().Calendars
	if len(cals) == 0 {
		where := "the user config"
		if m.session != nil {
			where = config.UserConfigPath(m.session.Dirs)
		}
		return m.say("no calendars: add [[calendars]] to "+where, true), nil
	}
	day := m.today()
	st := &meetingState{open: true, day: day, pending: len(cals)}
	p := palette.New("", palette.List).
		WithPlaceholder("/ to filter").
		WithHint("h/l day · t today · r refresh · enter open/create · esc close").
		WithInfo(func(string) []string { return st.failed })
	m = m.showPalette(m.meetingItems(p, st), func(m Model, ev palette.Event) (Model, tea.Cmd) {
		switch ev.Kind {
		case palette.Closed, palette.Chosen:
			st.open = false
		case palette.Key:
			switch ev.Key {
			case "h", "left":
				st.day = st.day.AddDate(0, 0, -1)
			case "l", "right":
				st.day = st.day.AddDate(0, 0, 1)
			case "t":
				st.day = m.today()
			default:
				return m, nil
			}
			m.overlay.p = m.meetingItems(m.overlay.p, st)
		}
		return m, nil
	})
	client := &http.Client{Timeout: calendar.Timeout}
	cmds := make([]tea.Cmd, len(cals))
	for i, c := range cals {
		cmds[i] = func() tea.Msg {
			ms, err := calendar.Fetch(client, c.URL)
			return meetingsMsg{st: st, cal: c.Name, color: c.Color, ms: ms, err: err}
		}
	}
	return m, tea.Batch(cmds...)
}

// meetingsArrived merges one calendar's Meetings into the open picker.
func (m Model) meetingsArrived(msg meetingsMsg) Model {
	st := msg.st
	if !st.open || m.overlay == nil {
		return m
	}
	st.pending--
	if msg.err != nil {
		st.failed = append(st.failed, msg.cal+": "+msg.err.Error())
	}
	for _, mt := range msg.ms {
		st.rows = append(st.rows, meetingRow{mt, msg.cal, calendarColor(msg.color, mt.CalendarColor)})
	}
	m.overlay.p = m.meetingItems(m.overlay.p, st)
	return m
}

// calendarColor is the configured colour, else the feed's COLOR, else nil
// (overlay.hint). An unreadable feed COLOR is ignored.
func calendarColor(configured, feed string) color.Color {
	for _, s := range []string{configured, feed} {
		if c, err := theme.ParseCSSColor(s); err == nil {
			return c
		}
	}
	return nil
}

// meetingItems titles p with st's day and fills it with that day's
// Meetings, sorted by start. Meetings that have ended are dimmed.
func (m Model) meetingItems(p palette.Model, st *meetingState) palette.Model {
	slices.SortStableFunc(st.rows, func(a, b meetingRow) int { return cmp.Compare(a.Start.UnixNano(), b.Start.UnixNano()) })
	loc := st.day.Location()
	from := st.day
	to := from.AddDate(0, 0, 1)
	now := m.now()
	var items []palette.Item
	for _, r := range st.rows {
		if r.Start.Before(from) || !r.Start.Before(to) {
			continue
		}
		it := palette.Item{
			// The two-cell gutter is blank; a later ticket marks Meetings
			// that have a Meeting Note there.
			Text:        "  " + r.Start.In(loc).Format("15:04") + "–" + r.End.In(loc).Format("15:04") + "  " + r.Title,
			Detail:      r.cal,
			DetailColor: r.color,
			Value:       r,
		}
		if !r.End.After(now) {
			it.Slot, it.DetailColor = theme.OverlayHint, nil
		}
		items = append(items, it)
	}
	empty := "no Meetings"
	if st.pending > 0 {
		empty = "Loading…"
	}
	return p.WithTitle(meetingsTitle(st.day, m.today())).WithEmpty(empty).SetItems(items)
}

// meetingsTitle is "Meetings · Mon 12 Oct", with " (today)",
// " (yesterday)" or " (tomorrow)" when day is one of those.
func meetingsTitle(day, today time.Time) string {
	title := "Meetings · " + day.Format("Mon 2 Jan")
	switch day.Format(time.DateOnly) {
	case today.Format(time.DateOnly):
		title += " (today)"
	case today.AddDate(0, 0, -1).Format(time.DateOnly):
		title += " (yesterday)"
	case today.AddDate(0, 0, 1).Format(time.DateOnly):
		title += " (tomorrow)"
	}
	return title
}
