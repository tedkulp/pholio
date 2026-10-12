package app

import (
	"cmp"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/calendar"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/palette"
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
	tried    time.Time // when the last fetch started
	fetching bool
	err      error // why the last fetch failed, or nil
}

// meetingState is an open Meeting picker.
type meetingState struct {
	open   bool      // the picker is still up
	day    time.Time // the day shown
	failed []string  // one status line per failing calendar
}

// meetingRow is a Meeting and the name of the calendar it came from.
type meetingRow struct {
	calendar.Meeting
	cal string
}

// meetingsMsg is one calendar's fetch result.
type meetingsMsg struct {
	url string
	ms  []calendar.Meeting
	err error
}

// calendarCache is where feed bodies are cached, or "" for no cache.
func (m Model) calendarCache() string {
	if m.session == nil || m.session.Dirs.CacheHome == "" {
		return ""
	}
	return filepath.Join(m.session.Dirs.CacheHome, "pholio", "calendars")
}

// feed is c's feed, read from the cache the first time it is asked for.
func (m Model) feed(c config.Calendar) *feed {
	if f, ok := m.feeds.byURL[c.URL]; ok {
		return f
	}
	f := &feed{}
	if dir := m.calendarCache(); dir != "" {
		if ms, at, err := calendar.Cached(dir, c.URL); err == nil {
			f.ms, f.fetched, f.tried = ms, at, at
		}
	}
	m.feeds.byURL[c.URL] = f
	return f
}

// fetchFeeds fetches every calendar not already being fetched, one
// goroutine each: all of them when force is set, else the stale ones.
func (m Model) fetchFeeds(force bool) tea.Cmd {
	client := &http.Client{Timeout: calendar.Timeout}
	dir := m.calendarCache()
	var cmds []tea.Cmd
	for _, c := range m.config().Calendars {
		f := m.feed(c)
		if f.fetching || !force && m.now().Sub(f.tried) <= staleAfter {
			continue
		}
		f.fetching, f.tried = true, m.now()
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
	st := &meetingState{open: true, day: day}
	m.feeds.picker = st
	cmd := m.fetchFeeds(false)
	p := palette.New("Meetings · "+day.Format("Mon 2 Jan")+" (today)", palette.List).
		WithPlaceholder("/ to filter").
		WithHint("/ filter · esc close").
		WithInfo(func(string) []string { return st.failed })
	m = m.showPalette(m.meetingItems(p, st), func(m Model, ev palette.Event) (Model, tea.Cmd) {
		switch {
		case ev.Kind == palette.Closed || ev.Kind == palette.Chosen:
			st.open = false
		case ev.Kind == palette.Key && ev.Key == "r":
			cmd := m.fetchFeeds(true)
			m.overlay.p = m.meetingItems(m.overlay.p, st)
			return m, cmd
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
	if st := m.feeds.picker; st != nil && st.open && m.overlay != nil {
		m.overlay.p = m.meetingItems(m.overlay.p, st)
	}
	return m
}

// meetingItems fills p with st's day's Meetings from every calendar,
// sorted by start, and notes the failing calendars in st.
func (m Model) meetingItems(p palette.Model, st *meetingState) palette.Model {
	var rows []meetingRow
	st.failed = nil
	loading, cached := false, false
	for _, c := range m.config().Calendars {
		f := m.feed(c)
		loading = loading || f.fetching
		cached = cached || !f.fetched.IsZero()
		for _, mt := range calendar.On(f.ms, st.day) {
			rows = append(rows, meetingRow{mt, c.Name})
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
		items[i] = palette.Item{
			// The two-cell gutter is blank; a later ticket marks Meetings
			// that have a Meeting Note there.
			Text:   "  " + r.Start.In(loc).Format("15:04") + "–" + r.End.In(loc).Format("15:04") + "  " + r.Title,
			Detail: r.cal,
			Value:  r,
		}
	}
	empty := "no Meetings"
	if loading && !cached {
		empty = "Loading…"
	}
	return p.WithEmpty(empty).SetItems(items)
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
