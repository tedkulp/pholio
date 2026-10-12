package app

import (
	"cmp"
	"fmt"
	"image/color"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/calendar"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

// staleAfter is how old a calendar's last fetch may be before opening the
// picker fetches it again.
const staleAfter = 15 * time.Minute

// feeds is every calendar's Meetings as last fetched, shared by copies of
// the Model, so the startup fetch fills the picker opened later.
type feeds struct {
	byURL  map[string]*feed
	picker *meetingState // the picker last opened
}

// feed is one calendar's Meetings and how its fetches went.
type feed struct {
	ms       []calendar.Meeting
	fetched  time.Time // when ms were fetched; zero when there are none
	fetching bool
	err      error // why the last fetch failed, or nil
}

// meetingState is an open Meeting picker.
type meetingState struct {
	open   bool              // the picker is still up
	day    time.Time         // the day shown
	failed []string          // one status line per failing calendar
	notes  map[string]string // meeting_uid → Meeting Note path
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
	url string
	ms  []calendar.Meeting
	err error
}

// cachedMsg is one calendar's Meetings read from the cache, fetched then.
type cachedMsg struct {
	url     string
	ms      []calendar.Meeting
	fetched time.Time
}

// calendarCache is where feed bodies are cached, or "" for no cache.
func (m Model) calendarCache() string {
	if m.session == nil || m.session.Dirs.CacheHome == "" {
		return ""
	}
	return filepath.Join(m.session.Dirs.CacheHome, "pholio", "calendars")
}

// feed is c's feed.
func (m Model) feed(c config.Calendar) *feed {
	f, ok := m.feeds.byURL[c.URL]
	if !ok {
		f = &feed{}
		m.feeds.byURL[c.URL] = f
	}
	return f
}

// startFeeds (at startup) reads every calendar's cache and fetches every
// calendar, all in the background.
func (m Model) startFeeds() tea.Cmd {
	cmds := []tea.Cmd{m.fetchFeeds(true)}
	if dir := m.calendarCache(); dir != "" {
		for _, c := range m.config().Calendars {
			cmds = append(cmds, func() tea.Msg {
				ms, at, err := calendar.Cached(dir, c.URL)
				if err != nil {
					return nil // nothing cached
				}
				return cachedMsg{url: c.URL, ms: ms, fetched: at}
			})
		}
	}
	return tea.Batch(cmds...)
}

// fetchFeeds fetches every calendar not already being fetched, one
// goroutine each: all of them when force is set, else those not fetched
// successfully in the last staleAfter.
func (m Model) fetchFeeds(force bool) tea.Cmd {
	client := &http.Client{Timeout: calendar.Timeout}
	dir := m.calendarCache()
	var cmds []tea.Cmd
	for _, c := range m.config().Calendars {
		f := m.feed(c)
		if f.fetching || !force && m.now().Sub(f.fetched) <= staleAfter {
			continue
		}
		f.fetching = true
		cmds = append(cmds, func() tea.Msg {
			ms, err := calendar.Fetch(client, c.URL, dir)
			return meetingsMsg{url: c.URL, ms: ms, err: err}
		})
	}
	return tea.Batch(cmds...)
}

// showMeetings (spc m, :meetings) opens the Meeting picker on Today with
// what is known, and fetches the calendars whose data is stale.
func (m Model) showMeetings() (Model, tea.Cmd) {
	if len(m.config().Calendars) == 0 {
		where := "the user config"
		if m.session != nil {
			where = config.UserConfigPath(m.session.Dirs)
		}
		return m.say("no calendars: add [[calendars]] to "+where, true), nil
	}
	day := m.today()
	st := &meetingState{open: true, day: day, notes: m.meetingNotes()}
	m.feeds.picker = st
	cmd := m.fetchFeeds(false)
	p := palette.New("", palette.List).
		WithPlaceholder("/ to filter").
		WithHint("h/l day · t today · r refresh · enter open/create · esc close").
		WithInfo(func(string) []string { return st.failed })
	m = m.showPalette(m.meetingItems(p, st), func(m Model, ev palette.Event) (Model, tea.Cmd) {
		switch {
		case ev.Kind == palette.Closed || ev.Kind == palette.Chosen:
			st.open = false
			if r, ok := ev.Item.Value.(meetingRow); ok && ev.Kind == palette.Chosen {
				return m.openMeeting(r.Meeting)
			}
		case ev.Kind == palette.Key && ev.Key == "r":
			cmd := m.fetchFeeds(true)
			m.overlay.p = m.meetingItems(m.overlay.p, st)
			return m, cmd
		case ev.Kind == palette.Key:
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
	return m, cmd
}

// meetingsArrived stores one calendar's fetch and redraws an open picker.
func (m Model) meetingsArrived(msg meetingsMsg) Model {
	f, ok := m.feeds.byURL[msg.url]
	if !ok {
		return m
	}
	f.fetching, f.err = false, msg.err
	if msg.err == nil {
		f.ms, f.fetched = msg.ms, m.now()
	}
	return m.redrawMeetings()
}

// cacheArrived stores one calendar's cached Meetings, unless a fetch has
// already brought newer ones, and redraws an open picker.
func (m Model) cacheArrived(msg cachedMsg) Model {
	f, ok := m.feeds.byURL[msg.url]
	if !ok || !f.fetched.IsZero() {
		return m
	}
	f.ms, f.fetched = msg.ms, msg.fetched
	return m.redrawMeetings()
}

// redrawMeetings refills the picker if it is open.
func (m Model) redrawMeetings() Model {
	if st := m.feeds.picker; st != nil && st.open && m.overlay != nil {
		m.overlay.p = m.meetingItems(m.overlay.p, st)
	}
	return m
}

// meetingItems titles p with st's day and fills it with that day's
// Meetings from every calendar, sorted by start, and notes the failing
// calendars in st. Meetings that have ended are dimmed.
func (m Model) meetingItems(p palette.Model, st *meetingState) palette.Model {
	var rows []meetingRow
	st.failed = nil
	loading, cached := false, false
	for _, c := range m.config().Calendars {
		f := m.feed(c)
		loading = loading || f.fetching
		cached = cached || !f.fetched.IsZero()
		for _, mt := range calendar.On(f.ms, st.day) {
			rows = append(rows, meetingRow{mt, c.Name, calendarColor(c.Color, mt.CalendarColor)})
		}
		switch {
		case f.err == nil:
		case f.fetched.IsZero():
			st.failed = append(st.failed, c.Name+": "+f.err.Error())
		default:
			st.failed = append(st.failed, c.Name+": offline, showing data from "+ago(m.now().Sub(f.fetched)))
		}
	}
	slices.SortStableFunc(rows, func(a, b meetingRow) int { return cmp.Compare(a.Start.UnixNano(), b.Start.UnixNano()) })
	loc := st.day.Location()
	items := make([]palette.Item, len(rows))
	for i, r := range rows {
		gutter := "  "
		if _, ok := st.notes[meetingUID(r.Meeting)]; ok {
			gutter = "✓ "
		}
		items[i] = palette.Item{
			Text:        gutter + r.Start.In(loc).Format("15:04") + "–" + r.End.In(loc).Format("15:04") + "  " + r.Title,
			Detail:      r.cal,
			DetailColor: r.color,
			Value:       r,
		}
		if !r.End.After(m.now()) {
			items[i].Slot, items[i].DetailColor = theme.OverlayHint, nil
		}
	}
	empty := "no Meetings"
	if loading && !cached {
		empty = "Loading…"
	}
	return p.WithTitle(meetingsTitle(st.day, m.today())).WithEmpty(empty).SetItems(items)
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

// ago is d as "5m ago", "2h ago" or "3d ago".
func ago(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
